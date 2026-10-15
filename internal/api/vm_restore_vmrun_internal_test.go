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
	"strings"
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
			Paths: []string{"/vm-disks/tank/vms/mixedvm/disk1.img"}, Summary: &restic.SnapshotSummary{TotalBytesProcessed: 1 << 30}},
		{ID: vdcID, Tags: []string{"vm:mixedvm:zvol:vdc", "p2", runTag},
			Paths: []string{"/vm-disks/tank/vms/mixedvm/disk2.img"}, Summary: &restic.SnapshotSummary{TotalBytesProcessed: 2 << 30}},
	}
}

func blockDisksByDataset(bds []backup.VMRestoreBlockDisk) map[string]backup.VMRestoreBlockDisk {
	out := make(map[string]backup.VMRestoreBlockDisk, len(bds))
	for _, bd := range bds {
		out[bd.Dataset] = bd
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
	if !ok || vdb.SnapshotID != "1111111111111111" || vdb.StdinPath != "/vm-disks/tank/vms/mixedvm/disk1.img" {
		t.Fatalf("vdb disk resolution = %+v, want snapshot 1111111111111111 with its recorded stdin path", vdb)
	}
	vdc, ok := byDataset["tank/vms/mixedvm/disk2"]
	if !ok || vdc.SnapshotID != "2222222222222222" || vdc.StdinPath != "/vm-disks/tank/vms/mixedvm/disk2.img" {
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

// A run that holds no image of a zvol disk, because it has no run tag or its
// zvol backup failed, is refused before anything is touched rather than
// restored without that disk.
func TestPrepareRestoreVMRefusesARunWithoutAZvolImage(t *testing.T) {
	for name, snaps := range map[string][]restic.Snapshot{
		"no run tag": {
			{ID: "deadbeef12345678", Tags: []string{"vm:mixedvm", "p2"}, Paths: []string{mixedvmDisk}},
		},
		"zvol backups missing": {
			{ID: "deadbeef12345678", Tags: []string{"vm:mixedvm", "p2", "vmrun:run-partial"}, Paths: []string{mixedvmDisk}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			s, ref, tg := vmrunRestoreTarget(t, &foreignRecordingEngine{snaps: snaps})
			_, err := s.prepareRestoreVMForTarget(context.Background(), ref, "mixedvm", "latest", tg, tagIdentity("vm:mixedvm"), "", "")
			if err == nil || !strings.Contains(err.Error(), "holds no image of the zvol disk") {
				t.Fatalf("err = %v, want the missing image named", err)
			}
		})
	}
}

// Each zvol is given back the size it had, read from its snapshot.
func TestPrepareRestoreVMTakesEachZvolSizeFromItsImage(t *testing.T) {
	eng := &foreignRecordingEngine{snaps: vmrunGroupSnaps("vmrun:r", "deadbeef12345678", "1111111111111111", "2222222222222222")}
	s, ref, tg := vmrunRestoreTarget(t, eng)

	plan, err := s.prepareRestoreVMForTarget(context.Background(), ref, "mixedvm", "latest", tg, tagIdentity("vm:mixedvm"), "", "")
	if err != nil {
		t.Fatal(err)
	}
	byDataset := blockDisksByDataset(plan.blockDisks)
	if byDataset["tank/vms/mixedvm/disk1"].Size != 1<<30 || byDataset["tank/vms/mixedvm/disk2"].Size != 2<<30 {
		t.Fatalf("block disks = %+v, want each at the size of its image", plan.blockDisks)
	}
}
