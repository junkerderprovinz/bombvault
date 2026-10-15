package backup_test

import (
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
)

func zvolOnlyDeps(t *testing.T, vm *fakeVM, r *fakeRestic, runs *fakeRuns, zr *fakeZvolRestic) backup.VMBackupDeps {
	t.Helper()
	d := sampleVMBackupDeps(t, vm, r, runs)
	d.DiskPaths = nil
	d.NVRAMPath = ""
	d.BlockDisks = []backup.VMBlockDisk{{Dataset: "tank/vms/win10", Dev: "vda"}}
	d.ZFSHost = &fakeZFSHost{streamSendData: []byte("zvol stream bytes")}
	d.ZvolRestic = zr
	return d
}

func TestGracefulBackupOfAZvolOnlyVMSendsTheZvolAndRestartsTheVM(t *testing.T) {
	vm := &fakeVM{active: true, stateVal: "shut off"}
	r := &fakeRestic{}
	zr := &fakeZvolRestic{backupSummary: backup.Summary{SnapshotID: "zvolSnap", Bytes: 10, Measured: true}}

	sum, err := backup.BackupVMGraceful(t.Context(), zvolOnlyDeps(t, vm, r, &fakeRuns{}, zr))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(r.log) != 0 {
		t.Fatalf("restic was called for a VM without file disks: %v", r.log)
	}
	if len(zr.log) != 1 || !strings.Contains(zr.log[0], "vm:win10:zvol:vda") {
		t.Fatalf("zvol backups = %v", zr.log)
	}
	if sum.SnapshotID != "zvolSnap" || !sum.Measured {
		t.Fatalf("summary = %+v, want the zvol snapshot, measured", sum)
	}
	if !vmContains(vm.log, "shutdown:win10") || !vmContains(vm.log, "start:win10") {
		t.Fatalf("vm calls = %v, want shutdown and start", vm.log)
	}
}

func TestLiveBackupOfAZvolOnlyVMNeedsNoOverlay(t *testing.T) {
	vm := &fakeVM{guestAgent: true}
	r := &fakeRestic{}
	zr := &fakeZvolRestic{backupSummary: backup.Summary{SnapshotID: "zvolSnap"}}

	sum, err := backup.BackupVMLive(t.Context(), zvolOnlyDeps(t, vm, r, &fakeRuns{}, zr))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(r.log) != 0 || len(zr.log) != 1 {
		t.Fatalf("restic calls %v, zvol calls %v; want none and one", r.log, zr.log)
	}
	if vmContains(vm.log, "snapshot:win10") {
		t.Fatalf("a snapshot overlay was created: %v", vm.log)
	}
	if sum.SnapshotID != "zvolSnap" {
		t.Fatalf("summary = %+v", sum)
	}
}
