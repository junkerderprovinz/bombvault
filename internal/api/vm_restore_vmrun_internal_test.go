package api

// One backup of a VM with file and zvol disks writes a main snapshot plus one
// per zvol disk, all sharing a "vmrun:<runID>" tag. These tests check that
// prepareRestoreVMForTarget resolves that group, and that a snapshot without
// the tag (a file-only VM, or an older backup) resolves from the plain
// "vm:<name>" tag alone. They assert the plan, which is what RestoreZvolDisk
// and RestorePaths consume; vm_restore_vmrun_wiring_test.go runs the full
// backup and restore round trip.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// vmrunDomainXML is vmDomainXML with two zvol disks added, so one backup run
// produces three snapshots sharing a "vmrun:<runID>" tag.
const vmrunDomainXML = `<domain type='kvm'><name>mixedvm</name>` +
	`<devices><disk type='file' device='disk'><source file='/mnt/pool/domains/mixedvm/mixedvm.qcow2'/><target dev='vda'/></disk>` +
	`<disk type='block' device='disk'><source dev='/dev/zvol/tank/vms/mixedvm/disk1'/><target dev='vdb'/></disk>` +
	`<disk type='block' device='disk'><source dev='/dev/zvol/tank/vms/mixedvm/disk2'/><target dev='vdc'/></disk></devices></domain>`

// mixedvmDisk is the file-backed disk of vmrunDomainXML as the backup reads it.
const mixedvmDisk = "/host/user/pool/domains/mixedvm/mixedvm.qcow2"

// vmrunGroupSnaps builds the listing one backup run of vmrunDomainXML
// produces, with the tags internal/backup gives each snapshot.
func vmrunGroupSnaps(runTag, mainID, vdbID, vdcID string) []restic.Snapshot {
	return []restic.Snapshot{
		{ID: mainID, Tags: []string{"vm:mixedvm", "p2", runTag}, Paths: []string{mixedvmDisk}},
		{ID: vdbID, Tags: []string{"vm:mixedvm:zvol:vdb", "p2", runTag},
			Paths: []string{"/vm-disks/tank/vms/mixedvm/disk1@bombvault-20260101000000"}},
		{ID: vdcID, Tags: []string{"vm:mixedvm:zvol:vdc", "p2", runTag},
			Paths: []string{"/vm-disks/tank/vms/mixedvm/disk2@bombvault-20260101000000"}},
	}
}

func blockDisksByDataset(bds []backup.VMRestoreBlockDisk) map[string]backup.VMRestoreBlockDisk {
	out := make(map[string]backup.VMRestoreBlockDisk, len(bds))
	for _, bd := range bds {
		out[bd.SourceDataset] = bd
	}
	return out
}

// vmrunRestoreTarget builds the target and repo the tests below share. SSH is
// wired because prepareRestoreVMForTarget refuses a zvol restore without it.
func vmrunRestoreTarget(t *testing.T, eng ResticEngine) (*Service, repoRef, store.VMTarget) {
	t.Helper()
	s := vmRestoreSvc(t, eng)
	s.ssh = &fakeHostSSH{}
	repoDir := filepath.Join(t.TempDir(), "repo")
	seedResticRepoDir(t, repoDir)
	tg := vmTargetJSON(t, "mixedvm", vmrunDomainXML, []string{mixedvmDisk}, "")
	return s, repoRef{repo: repoDir}, tg
}

// A "latest" restore resolves the main snapshot and each zvol disk's own
// snapshot and stdin path from the group, none swapped or dropped.
func TestPrepareRestoreVMResolvesVmrunGroupForAllThreeDisks(t *testing.T) {
	const runTag = "vmrun:run-abc"
	snaps := vmrunGroupSnaps(runTag, "deadbeef12345678", "1111111111111111", "2222222222222222")
	eng := &foreignRecordingEngine{snaps: snaps}
	s, ref, tg := vmrunRestoreTarget(t, eng)

	plan, err := s.prepareRestoreVMForTarget(context.Background(), ref, "mixedvm", "latest", tg, tagIdentity("vm:mixedvm"), "", "")
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

// An explicit snapshot id resolves the group just as "latest" does.
func TestPrepareRestoreVMExplicitSnapshotIDResolvesVmrunGroup(t *testing.T) {
	const runTag = "vmrun:run-explicit"
	snaps := vmrunGroupSnaps(runTag, "aaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbb", "cccccccccccccccc")
	eng := &foreignRecordingEngine{snaps: snaps}
	s, ref, tg := vmrunRestoreTarget(t, eng)

	plan, err := s.prepareRestoreVMForTarget(context.Background(), ref, "mixedvm", "aaaaaaaaaaaaaaaa", tg, tagIdentity("vm:mixedvm"), "", "")
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

// With two runs in the repo, "latest" resolves every disk from the newer run
// and never mixes in the older one.
func TestPrepareRestoreVMLatestPicksNewestRunsGroupNotOlder(t *testing.T) {
	older := vmrunGroupSnaps("vmrun:run-older", "1000000000000000", "1000000000000001", "1000000000000002")
	newer := vmrunGroupSnaps("vmrun:run-newer", "2000000000000000", "2000000000000001", "2000000000000002")
	// "latest" is the last "vm:"+name snapshot in listing order, so the newer
	// run goes second.
	all := append(append([]restic.Snapshot{}, older...), newer...)
	eng := &foreignRecordingEngine{snaps: all}
	s, ref, tg := vmrunRestoreTarget(t, eng)

	plan, err := s.prepareRestoreVMForTarget(context.Background(), ref, "mixedvm", "latest", tg, tagIdentity("vm:mixedvm"), "", "")
	if err != nil {
		t.Fatalf("prepareRestoreVMForTarget: %v", err)
	}
	if plan.snapshotID != "2000000000000000" {
		t.Fatalf("main snapshotID = %q, want the newer run's file-backed snapshot", plan.snapshotID)
	}
	byDataset := blockDisksByDataset(plan.blockDisks)
	vdb := byDataset["tank/vms/mixedvm/disk1"]
	if vdb.SnapshotID != "2000000000000001" {
		t.Fatalf("vdb disk resolution = %+v, want the newer run's snapshot 2000000000000001 (never the older run's 1000000000000001)", vdb)
	}
	vdc := byDataset["tank/vms/mixedvm/disk2"]
	if vdc.SnapshotID != "2000000000000002" {
		t.Fatalf("vdc disk resolution = %+v, want the newer run's snapshot 2000000000000002 (never the older run's 1000000000000002)", vdc)
	}
}

// Without a "vmrun:" tag the main snapshot comes from the plain "vm:"+name
// tag, and each zvol disk keeps only its SourceDataset: no snapshot is guessed.
func TestPrepareRestoreVMFallsBackWhenNoVmrunTag(t *testing.T) {
	eng := &foreignRecordingEngine{snaps: []restic.Snapshot{
		{ID: "deadbeef12345678", Tags: []string{"vm:mixedvm", "p2"}, Paths: []string{mixedvmDisk}}, // no vmrun: tag
	}}
	s, ref, tg := vmrunRestoreTarget(t, eng)

	plan, err := s.prepareRestoreVMForTarget(context.Background(), ref, "mixedvm", "latest", tg, tagIdentity("vm:mixedvm"), "", "")
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
			t.Fatalf("blockDisk %+v: SnapshotID/StdinPath must stay at zero value with no vmrun: group", bd)
		}
	}
}

// The main snapshot carries a "vmrun:" tag but the zvol backups of that run all
// failed, so the group holds only the main snapshot. The zvol disks then
// resolve as if there were no tag, and RestoreZvolDisk fails instead of
// restoring a guessed snapshot.
func TestPrepareRestoreVMSingleSnapshotVmrunGroupFallsBackForZvolDisks(t *testing.T) {
	const runTag = "vmrun:run-partial"
	eng := &foreignRecordingEngine{snaps: []restic.Snapshot{
		{ID: "deadbeef12345678", Tags: []string{"vm:mixedvm", "p2", runTag}, Paths: []string{mixedvmDisk}},
	}}
	s, ref, tg := vmrunRestoreTarget(t, eng)

	plan, err := s.prepareRestoreVMForTarget(context.Background(), ref, "mixedvm", "latest", tg, tagIdentity("vm:mixedvm"), "", "")
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
			t.Fatalf("blockDisk %+v: SnapshotID/StdinPath must stay at zero value when the vmrun: group has no matching zvol member, as without a tag", bd)
		}
	}
}
