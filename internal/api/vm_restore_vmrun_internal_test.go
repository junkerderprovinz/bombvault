package api

// Tests for restore-side "vmrun:<runID>" correlation-tag resolution — v8.0.0
// VM service-layer integration, Task 3 (the design notes). Task 1/2 tag every
// snapshot ONE
// mixed file+zvol VM backup produces (the main file-backed snapshot plus one
// per zvol disk) with a shared "vmrun:<runID>" tag; this file proves
// prepareRestoreVMForTarget resolves that group correctly — for an explicit
// snapshot id AND for "latest" — and, the critical regression case, falls
// back to EXACTLY the pre-Task-3 single-"vm:<name>"-tag resolution when the
// resolved snapshot carries no "vmrun:" tag at all (a Run predating this
// plan, or a file-only VM's snapshot, which never gets one — a real,
// permanent case, not a transitional one).
//
// These tests operate at the prepareRestoreVMForTarget PLAN level (same style
// as foreign_vm_restore_internal_test.go's requirement pins) rather than
// driving the full executeRestoreVM/virsh dance — Task 3's job is resolution,
// and the plan's blockDisks/snapshotID fields are exactly what
// RestoreZvolDisk/RestorePaths consume (see VMRestoreBlockDisk's own doc
// comment) — so asserting them directly is the precise unit under test. A
// separate end-to-end test (vm_restore_vmrun_wiring_test.go, package
// api_test) drives the real BackupVM->RestoreVM round trip through the
// zvol-restore fakes for extra assurance that the resolved values actually
// reach RestoreZvolDisk correctly.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// vmrunDomainXML mirrors vmDomainXML (foreign_vm_restore_internal_test.go)
// plus TWO block-device (zvol) disks, so one backup run produces 3 restic
// snapshots correlated by a shared "vmrun:<runID>" tag — 1 file-backed + 2
// zvol.
const vmrunDomainXML = `<domain type='kvm'><name>mixedvm</name>` +
	`<devices><disk type='file' device='disk'><source file='/mnt/pool/domains/mixedvm/mixedvm.qcow2'/><target dev='vda'/></disk>` +
	`<disk type='block' device='disk'><source dev='/dev/zvol/tank/vms/mixedvm/disk1'/><target dev='vdb'/></disk>` +
	`<disk type='block' device='disk'><source dev='/dev/zvol/tank/vms/mixedvm/disk2'/><target dev='vdc'/></disk></devices></domain>`

// vmrunGroupSnaps builds the 3-snapshot restic listing one mixed-VM backup
// run under runTag produces: the main file-backed snapshot (mainID) plus one
// per zvol disk (vdb -> vdbID, vdc -> vdcID), each carrying its own identity
// tag AND the shared runTag — mirroring backupBlockDisksAndLog/runVMGraceful's
// real tag shape (internal/backup/vm_orchestrator.go).
func vmrunGroupSnaps(runTag, mainID, vdbID, vdcID string) []restic.Snapshot {
	return []restic.Snapshot{
		{ID: mainID, Tags: []string{"vm:mixedvm", "p2", runTag}},
		{ID: vdbID, Tags: []string{"vm:mixedvm:zvol:vdb", "p2", runTag},
			Paths: []string{"/vm-disks/tank/vms/mixedvm/disk1@bombvault-20260101000000"}},
		{ID: vdcID, Tags: []string{"vm:mixedvm:zvol:vdc", "p2", runTag},
			Paths: []string{"/vm-disks/tank/vms/mixedvm/disk2@bombvault-20260101000000"}},
	}
}

// blockDisksByDataset indexes plan.blockDisks by SourceDataset for
// order-independent assertions.
func blockDisksByDataset(bds []backup.VMRestoreBlockDisk) map[string]backup.VMRestoreBlockDisk {
	out := make(map[string]backup.VMRestoreBlockDisk, len(bds))
	for _, bd := range bds {
		out[bd.SourceDataset] = bd
	}
	return out
}

// vmrunRestoreTarget builds the store.VMTarget + repo fixture shared by every
// test below: the mixed-disk domain XML, a seeded local repo dir, and SSH
// wired (fakeHostSSH, notify_internal_test.go) so the zvol-disk guard in
// prepareRestoreVMForTarget doesn't refuse the restore before resolution even
// runs.
func vmrunRestoreTarget(t *testing.T, eng ResticEngine) (*Service, repoRef, store.VMTarget) {
	t.Helper()
	s := vmRestoreSvc(t, eng)
	s.ssh = &fakeHostSSH{}
	repoDir := filepath.Join(t.TempDir(), "repo")
	seedResticRepoDir(t, repoDir)
	disks := []string{"/host/user/pool/domains/mixedvm/mixedvm.qcow2"}
	tg := vmTargetJSON(t, "mixedvm", vmrunDomainXML, disks, "")
	return s, repoRef{repo: repoDir}, tg
}

// TestPrepareRestoreVMResolvesVmrunGroupForAllThreeDisks pins requirement (a):
// a "latest" restore of a Run with a 3-snapshot vmrun: group resolves the
// main snapshot AND both zvol disks' own snapshot id + stdin path from that
// group — each disk keyed to the CORRECT snapshot, not swapped or dropped.
func TestPrepareRestoreVMResolvesVmrunGroupForAllThreeDisks(t *testing.T) {
	const runTag = "vmrun:run-abc"
	snaps := vmrunGroupSnaps(runTag, "deadbeef12345678", "1111111111111111", "2222222222222222")
	eng := &foreignRecordingEngine{snaps: snaps}
	s, ref, tg := vmrunRestoreTarget(t, eng)

	plan, err := s.prepareRestoreVMForTarget(context.Background(), ref, "mixedvm", "latest", tg, "", "")
	if err != nil {
		t.Fatalf("prepareRestoreVMForTarget: %v", err)
	}
	if plan.snapshotID != "deadbeef12345678" {
		t.Fatalf("main snapshotID = %q, want the file-backed snapshot's id", plan.snapshotID)
	}
	if len(plan.blockDisks) != 2 {
		t.Fatalf("blockDisks = %+v, want 2 entries", plan.blockDisks)
	}
	byDataset := blockDisksByDataset(plan.blockDisks)
	vdb, ok := byDataset["tank/vms/mixedvm/disk1"]
	if !ok || vdb.SnapshotID != "1111111111111111" || vdb.StdinPath != "/vm-disks/tank/vms/mixedvm/disk1@bombvault-20260101000000" {
		t.Fatalf("vdb disk resolution = %+v, want snapshot 1111111111111111 with its recorded stdin path", vdb)
	}
	vdc, ok := byDataset["tank/vms/mixedvm/disk2"]
	if !ok || vdc.SnapshotID != "2222222222222222" || vdc.StdinPath != "/vm-disks/tank/vms/mixedvm/disk2@bombvault-20260101000000" {
		t.Fatalf("vdc disk resolution = %+v, want snapshot 2222222222222222 with its recorded stdin path", vdc)
	}
}

// TestPrepareRestoreVMExplicitSnapshotIDResolvesVmrunGroup covers the OTHER
// half of requirement (a): an explicit (non-"latest") snapshot id from the
// request must resolve the SAME vmrun: group the "latest" path does — the
// group lookup is not something that only kicks in for "latest".
func TestPrepareRestoreVMExplicitSnapshotIDResolvesVmrunGroup(t *testing.T) {
	const runTag = "vmrun:run-explicit"
	snaps := vmrunGroupSnaps(runTag, "aaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbb", "cccccccccccccccc")
	eng := &foreignRecordingEngine{snaps: snaps}
	s, ref, tg := vmrunRestoreTarget(t, eng)

	plan, err := s.prepareRestoreVMForTarget(context.Background(), ref, "mixedvm", "aaaaaaaaaaaaaaaa", tg, "", "")
	if err != nil {
		t.Fatalf("prepareRestoreVMForTarget: %v", err)
	}
	byDataset := blockDisksByDataset(plan.blockDisks)
	vdb, ok := byDataset["tank/vms/mixedvm/disk1"]
	if !ok || vdb.SnapshotID != "bbbbbbbbbbbbbbbb" {
		t.Fatalf("vdb disk resolution = %+v, want snapshot bbbbbbbbbbbbbbbb", vdb)
	}
	vdc, ok := byDataset["tank/vms/mixedvm/disk2"]
	if !ok || vdc.SnapshotID != "cccccccccccccccc" {
		t.Fatalf("vdc disk resolution = %+v, want snapshot cccccccccccccccc", vdc)
	}
}

// TestPrepareRestoreVMLatestPicksNewestRunsGroupNotOlder pins requirement (c):
// with TWO backup runs' snapshots present (an older and a newer, each its own
// vmrun: group), a "latest" restore must resolve the NEWER run's group — not
// silently mix disks from the two runs, and not resolve the older one's.
func TestPrepareRestoreVMLatestPicksNewestRunsGroupNotOlder(t *testing.T) {
	older := vmrunGroupSnaps("vmrun:run-older", "1000000000000000", "1000000000000001", "1000000000000002")
	newer := vmrunGroupSnaps("vmrun:run-newer", "2000000000000000", "2000000000000001", "2000000000000002")
	// snapshotsForTag preserves listSnapshots' own ordering; "latest" picks the
	// LAST element of the "vm:"+name-tagged list, so the newer run's main
	// snapshot must sort after the older run's.
	all := append(append([]restic.Snapshot{}, older...), newer...)
	eng := &foreignRecordingEngine{snaps: all}
	s, ref, tg := vmrunRestoreTarget(t, eng)

	plan, err := s.prepareRestoreVMForTarget(context.Background(), ref, "mixedvm", "latest", tg, "", "")
	if err != nil {
		t.Fatalf("prepareRestoreVMForTarget: %v", err)
	}
	if plan.snapshotID != "2000000000000000" {
		t.Fatalf("main snapshotID = %q, want the NEWER run's file-backed snapshot", plan.snapshotID)
	}
	byDataset := blockDisksByDataset(plan.blockDisks)
	vdb := byDataset["tank/vms/mixedvm/disk1"]
	if vdb.SnapshotID != "2000000000000001" {
		t.Fatalf("vdb disk resolution = %+v, want the NEWER run's snapshot 2000000000000001 (never the older run's 1000000000000001)", vdb)
	}
	vdc := byDataset["tank/vms/mixedvm/disk2"]
	if vdc.SnapshotID != "2000000000000002" {
		t.Fatalf("vdc disk resolution = %+v, want the NEWER run's snapshot 2000000000000002 (never the older run's 1000000000000002)", vdc)
	}
}

// TestPrepareRestoreVMFallsBackWhenNoVmrunTag pins requirement (b), the
// CRITICAL regression case: a Run whose resolved snapshot carries no
// "vmrun:" tag at all (predating this plan, or any backup that for some
// reason never got one) must resolve EXACTLY as it did before Task 3 — the
// main snapshot from the plain "vm:"+name tag scan alone, and every zvol
// disk's SnapshotID/StdinPath left at ZERO VALUE (only SourceDataset set),
// never a group lookup invented from thin air.
func TestPrepareRestoreVMFallsBackWhenNoVmrunTag(t *testing.T) {
	eng := &foreignRecordingEngine{snaps: []restic.Snapshot{
		{ID: "deadbeef12345678", Tags: []string{"vm:mixedvm", "p2"}}, // no vmrun: tag
	}}
	s, ref, tg := vmrunRestoreTarget(t, eng)

	plan, err := s.prepareRestoreVMForTarget(context.Background(), ref, "mixedvm", "latest", tg, "", "")
	if err != nil {
		t.Fatalf("prepareRestoreVMForTarget: %v", err)
	}
	if plan.snapshotID != "deadbeef12345678" {
		t.Fatalf("main snapshotID = %q, want the plain vm:mixedvm tag's snapshot (unchanged fallback)", plan.snapshotID)
	}
	if len(plan.blockDisks) != 2 {
		t.Fatalf("blockDisks = %+v, want 2 entries (SourceDataset still resolved from the domain XML)", plan.blockDisks)
	}
	for _, bd := range plan.blockDisks {
		if bd.SourceDataset == "" {
			t.Fatalf("blockDisk %+v: SourceDataset must still be resolved from the domain XML in the fallback", bd)
		}
		if bd.SnapshotID != "" || bd.StdinPath != "" {
			t.Fatalf("blockDisk %+v: SnapshotID/StdinPath must stay at zero value with no vmrun: group (byte-identical to before Task 3)", bd)
		}
	}
}

// TestPrepareRestoreVMSingleSnapshotVmrunGroupFallsBackForZvolDisks pins a
// DIFFERENT shape of requirement (b) than TestPrepareRestoreVMFallsBackWhenNoVmrunTag
// above: here the resolved snapshot DOES carry a real "vmrun:" tag (this was
// a mixed-disk run, not one predating the tag), but the group that tag
// resolves to contains ONLY the main snapshot itself — e.g. every zvol disk's
// own backup failed AFTER the main file-backed restic Backup call had already
// succeeded and been tagged (backupBlockDisksAndLog, internal/backup/
// vm_orchestrator.go, continues past each failed disk and only surfaces the
// first error once every disk has been attempted, by which point the main
// snapshot already exists with the "vmrun:" tag — deps.RunTag is set purely
// from the domain XML's block-disk COUNT, before any zvol backup runs).
//
// A single-snapshot vmrun: group must resolve IDENTICALLY to having no
// vmrun: tag at all: every disk's SnapshotID/StdinPath stays at zero value
// (RestoreZvolDisk fails loudly, nothing is invented). The presence of A
// "vmrun:" tag/group must never be conflated with the presence of a MATCHING
// group member for a given disk — vmrunGroupSnapshot's per-tag lookup must
// still correctly report "not found" for a group that only contains the
// main snapshot's own "vm:<name>" tag.
func TestPrepareRestoreVMSingleSnapshotVmrunGroupFallsBackForZvolDisks(t *testing.T) {
	const runTag = "vmrun:run-partial"
	eng := &foreignRecordingEngine{snaps: []restic.Snapshot{
		// Only the main snapshot exists in the whole repo — it carries the
		// real runTag (so vmRunTag finds it and a group lookup DOES fire),
		// but neither zvol disk's own snapshot was ever created.
		{ID: "deadbeef12345678", Tags: []string{"vm:mixedvm", "p2", runTag}},
	}}
	s, ref, tg := vmrunRestoreTarget(t, eng)

	plan, err := s.prepareRestoreVMForTarget(context.Background(), ref, "mixedvm", "latest", tg, "", "")
	if err != nil {
		t.Fatalf("prepareRestoreVMForTarget: %v", err)
	}
	if plan.snapshotID != "deadbeef12345678" {
		t.Fatalf("main snapshotID = %q, want the file-backed snapshot's id", plan.snapshotID)
	}
	if len(plan.blockDisks) != 2 {
		t.Fatalf("blockDisks = %+v, want 2 entries (SourceDataset still resolved from the domain XML)", plan.blockDisks)
	}
	for _, bd := range plan.blockDisks {
		if bd.SourceDataset == "" {
			t.Fatalf("blockDisk %+v: SourceDataset must still be resolved from the domain XML", bd)
		}
		if bd.SnapshotID != "" || bd.StdinPath != "" {
			t.Fatalf("blockDisk %+v: SnapshotID/StdinPath must stay at zero value when the vmrun: group has no matching zvol member (single-snapshot group) — behaving EXACTLY like the no-tag-at-all fallback, not silently inventing or misattributing a snapshot", bd)
		}
	}
}
