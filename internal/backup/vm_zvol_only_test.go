package backup_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
)

const zvolOnlyDomainXML = "<domain><name>win10</name></domain>"

func zvolOnlyDeps(t *testing.T, vm *fakeVM, r *fakeRestic, runs *fakeRuns, zr *fakeZvolRestic) backup.VMBackupDeps {
	t.Helper()
	d := sampleVMBackupDeps(t, vm, r, runs)
	d.DiskPaths = nil
	d.NVRAMPath = ""
	d.BlockDisks = []backup.VMBlockDisk{{Dataset: "tank/vms/win10", Dev: "vda"}}
	d.ZFSHost = &fakeZFSHost{streamSendData: []byte("zvol stream bytes")}
	d.ZvolRestic = zr
	d.DomainXML = zvolOnlyDomainXML
	d.RunTag = "vmrun:run-1"
	return d
}

// zvolOnlyRestic answers the domain XML and the zvol stream with summaries of
// their own, so a test can tell which one the run was recorded under.
func zvolOnlyRestic() *fakeZvolRestic {
	return &fakeZvolRestic{
		backupSummary: backup.Summary{SnapshotID: "zvolSnap", Bytes: 1000, Measured: true, SourceBytes: 1000, SourceFiles: 1, FilesNew: 1},
		summaryByPath: map[string]backup.Summary{
			backup.VMDomainStdinPath("win10"): {SnapshotID: "domainSnap", Bytes: 30, Measured: true, SourceBytes: 30, SourceFiles: 1},
		},
	}
}

func TestGracefulBackupOfAZvolOnlyVMSendsTheZvolAndRestartsTheVM(t *testing.T) {
	vm := &fakeVM{active: true, stateVal: "shut off"}
	r := &fakeRestic{}
	zr := zvolOnlyRestic()

	sum, err := backup.BackupVMGraceful(t.Context(), zvolOnlyDeps(t, vm, r, &fakeRuns{}, zr))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(r.log) != 0 {
		t.Fatalf("restic read files for a VM without file disks: %v", r.log)
	}
	if len(zr.log) != 2 || !strings.Contains(zr.log[0], "vm:win10:zvol:vda") {
		t.Fatalf("stdin backups = %v, want the zvol and then the domain", zr.log)
	}
	if !vmContains(vm.log, "shutdown:win10") || !vmContains(vm.log, "start:win10") {
		t.Fatalf("vm calls = %v, want shutdown and start", vm.log)
	}
	if sum.SnapshotID != "domainSnap" || sum.Bytes != 1030 || sum.SourceBytes != 1030 || !sum.Measured {
		t.Fatalf("summary = %+v, want the domain snapshot with the zvol counted in", sum)
	}
}

func TestAZvolOnlyVMRunIsRecordedUnderTheVMsOwnTag(t *testing.T) {
	for _, method := range []string{"graceful", "live"} {
		t.Run(method, func(t *testing.T) {
			vm := &fakeVM{active: true, stateVal: "shut off", guestAgent: true}
			runs := &fakeRuns{}
			zr := zvolOnlyRestic()
			d := zvolOnlyDeps(t, vm, &fakeRestic{}, runs, zr)
			d.FormerNames = []string{"win7"}

			backupVM := backup.BackupVMGraceful
			if method == "live" {
				backupVM = backup.BackupVMLive
			}
			if _, err := backupVM(t.Context(), d); err != nil {
				t.Fatalf("unexpected: %v", err)
			}

			domain := backup.VMDomainStdinPath("win10")
			if got := string(zr.stdinByPath[domain]); got != zvolOnlyDomainXML {
				t.Fatalf("stored domain = %q, want the domain XML", got)
			}
			if zr.capturedPath != domain {
				t.Fatalf("last stdin backup went to %q, want the domain after the disks", zr.capturedPath)
			}
			tags := strings.Join(zr.capturedTags, ",")
			for _, want := range []string{"vm:win10", "p2", "formerly:win7", "vmrun:run-1"} {
				if !strings.Contains(","+tags+",", ","+want+",") {
					t.Fatalf("domain tags = %v, want %s among them", zr.capturedTags, want)
				}
			}
			if got := runs.finishOf(t, "run-1").sum.SnapshotID; got != "domainSnap" {
				t.Fatalf("run recorded snapshot %q, want the domain snapshot", got)
			}
		})
	}
}

func TestLiveBackupOfAZvolOnlyVMNeedsNoOverlay(t *testing.T) {
	vm := &fakeVM{guestAgent: true}
	zr := zvolOnlyRestic()

	if _, err := backup.BackupVMLive(t.Context(), zvolOnlyDeps(t, vm, &fakeRestic{}, &fakeRuns{}, zr)); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if vmContains(vm.log, "snapshot:win10") {
		t.Fatalf("a snapshot overlay was created: %v", vm.log)
	}
	if !slices.Contains(zr.capturedTags, "live") {
		t.Fatalf("domain tags = %v, want live", zr.capturedTags)
	}
}

func TestAZvolOnlyVMWhoseDiskFailedLeavesNoRunSnapshot(t *testing.T) {
	vm := &fakeVM{active: true, stateVal: "shut off"}
	runs := &fakeRuns{}
	zr := zvolOnlyRestic()
	d := zvolOnlyDeps(t, vm, &fakeRestic{}, runs, zr)
	d.ZFSHost = &fakeZFSHost{snapshotCreateErr: errors.New("dataset is busy")}

	if _, err := backup.BackupVMGraceful(t.Context(), d); err == nil {
		t.Fatal("want an error when the zvol could not be backed up")
	}
	if _, stored := zr.stdinByPath[backup.VMDomainStdinPath("win10")]; stored {
		t.Fatalf("the domain was stored for a run whose disk failed: %v", zr.log)
	}
	if got := runs.finishOf(t, "run-1"); got.status != "failed" {
		t.Fatalf("run = %+v, want failed", got)
	}
}

func TestRestoreOfAZvolOnlyVMReceivesTheZvolAndDefinesTheVM(t *testing.T) {
	vm := &fakeVM{stateVal: "running"}
	r := &fakeRestic{}
	runs := &fakeRuns{}
	host := &fakeZFSHost{}
	zr := &fakeZvolRestic{dumpData: []byte("zvol stream bytes")}

	d := sampleVMRestoreDeps(t, vm, r, runs)
	d.DiskPaths = nil
	d.NVRAMPath = ""
	d.BlockDisks = []backup.VMRestoreBlockDisk{{
		SourceDataset: "tank/vms/win10",
		SnapshotID:    "zvolsnapid1234",
		StdinPath:     "/vm-disks/tank/vms/win10@bombvault-20261015120000",
	}}
	d.ZFSHost = host
	d.ZvolRestic = zr

	if err := backup.RestoreVM(t.Context(), d); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if vmContains(r.log, "restore:") || vmContains(r.log, "restoreSubtree:") {
		t.Fatalf("a file restore ran for a VM without file disks: %v", r.log)
	}
	if !vmContains(r.log, "verify:/repo/vms:deadbeef12345678") {
		t.Fatalf("the run snapshot was not checked before the VM was touched: %v", r.log)
	}
	if string(host.streamReceiveData) != "zvol stream bytes" || !strings.HasPrefix(host.streamReceiveTarget, "tank/vms/win10-bombvault-restore-") {
		t.Fatalf("received %q into %q, want the zvol stream in a fresh dataset", host.streamReceiveData, host.streamReceiveTarget)
	}
	if !vmContains(vm.log, "define:") || !vmContains(vm.log, "start:win10") {
		t.Fatalf("vm calls = %v, want the VM defined and started", vm.log)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "success" {
		t.Fatalf("run finishes = %v, want [success]", runs.finishes)
	}
}

func TestRestoreWithNeitherFileNorZvolDisksIsRefused(t *testing.T) {
	vm := &fakeVM{stateVal: "running"}
	d := sampleVMRestoreDeps(t, vm, &fakeRestic{}, &fakeRuns{})
	d.DiskPaths = nil
	d.NVRAMPath = ""

	if err := backup.RestoreVM(t.Context(), d); err == nil || !strings.Contains(err.Error(), "no paths to restore") {
		t.Fatalf("err = %v, want the no paths refusal", err)
	}
	if vmContains(vm.log, "undefine:win10") {
		t.Fatalf("the VM was undefined: %v", vm.log)
	}
}
