package backup_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
)

// These tests check that BackupVMGraceful and RestoreVM run the zvol path for
// a domain's block disks, in addition to the file-backed disks, and leave it
// alone for a domain without any.

func TestBackupVMGracefulInvokesZvolPathForBlockDisks(t *testing.T) {
	vm := &fakeVM{active: true, stateVal: "shut off"}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "fileSnap1234", Bytes: 1024}}
	runs := &fakeRuns{}
	host := &fakeZFSHost{readData: []byte("raw disk image")}
	zr := &fakeZvolRestic{backupSummary: backup.Summary{SnapshotID: "zvolSnap5678", Bytes: 2048}}

	d := sampleVMBackupDeps(t, vm, r, runs)
	d.BlockDisks = []backup.VMBlockDisk{{Dataset: "tank/vms/win10/disk1"}}
	d.ZFSHost = host
	d.ZvolRestic = zr

	sum, err := backup.BackupVMGraceful(t.Context(), d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The returned summary is the file backup's.
	if sum.SnapshotID != "fileSnap1234" {
		t.Fatalf("summary.SnapshotID = %q, want the file-backup snapshot id unchanged", sum.SnapshotID)
	}

	for _, want := range []string{"snapshot:tank/vms/win10/disk1@", "read:tank/vms/win10/disk1-", "destroy:tank/vms/win10/disk1@"} {
		if !vmContains(host.log, want) {
			t.Fatalf("host calls %v, want one starting %q", host.log, want)
		}
	}
	if string(zr.capturedStdin) != "raw disk image" {
		t.Fatalf("restic read %q, want the disk image", zr.capturedStdin)
	}

	if !vmContains(r.log, "backup:/repo/vms") {
		t.Fatalf("file-backed restic backup not called: %v", r.log)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "success" {
		t.Fatalf("run finishes = %v, want [success]", runs.finishes)
	}
}

// TestBackupVMGracefulFileOnlyNeedsNoZFSHost leaves BlockDisks, ZFSHost and
// ZvolRestic nil, so any use of the zvol path would panic.
func TestBackupVMGracefulFileOnlyNeedsNoZFSHost(t *testing.T) {
	vm := &fakeVM{active: true, stateVal: "shut off"}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678", Bytes: 4096}}
	runs := &fakeRuns{}

	d := sampleVMBackupDeps(t, vm, r, runs)

	sum, err := backup.BackupVMGraceful(t.Context(), d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sum.SnapshotID != "deadbeef12345678" || sum.Bytes != 4096 {
		t.Fatalf("summary = %+v, want the restic summary unchanged", sum)
	}
	if !vmContains(vm.log, "isActive:") || !vmContains(vm.log, "shutdown:win10") || !vmContains(vm.log, "start:win10") {
		t.Fatalf("graceful shutdown/restart sequence changed: %v", vm.log)
	}
	if !vmContains(r.log, "backup:/repo/vms") {
		t.Fatalf("restic backup not called: %v", r.log)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "success" {
		t.Fatalf("run finishes = %v, want [success]", runs.finishes)
	}
}

// TestBackupVMGracefulZvolFailureFailsWholeRun checks that a failed block disk
// fails the whole run and the VM is still restarted.
func TestBackupVMGracefulZvolFailureFailsWholeRun(t *testing.T) {
	vm := &fakeVM{active: true, stateVal: "shut off"}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "fileSnap1234"}}
	runs := &fakeRuns{}
	host := &fakeZFSHost{snapshotCreateErr: errors.New("dataset does not exist")}
	zr := &fakeZvolRestic{}

	d := sampleVMBackupDeps(t, vm, r, runs)
	d.BlockDisks = []backup.VMBlockDisk{{Dataset: "tank/vms/win10/disk1"}}
	d.ZFSHost = host
	d.ZvolRestic = zr

	_, err := backup.BackupVMGraceful(t.Context(), d)
	if err == nil {
		t.Fatal("expected an error when the zvol disk backup fails")
	}
	if !vmContains(vm.log, "start:win10") {
		t.Fatal("VM must still be restarted even when the zvol portion fails")
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "failed" {
		t.Fatalf("run finishes = %v, want [failed]", runs.finishes)
	}
}

// TestBackupVMGracefulRecordsZvolDisksInRunSummary pins that a VM's block
// disks count towards its run, so a zvol VM's history is not the file disks
// alone.
func TestBackupVMGracefulRecordsZvolDisksInRunSummary(t *testing.T) {
	file := backup.Summary{
		SnapshotID: "fileSnap1234", Bytes: 1024,
		Measured: true, SourceBytes: 4096, SourceFiles: 12, FilesNew: 3, ResticMS: 900,
	}
	zvol := backup.Summary{
		SnapshotID: "zvolSnap5678", Bytes: 2048,
		Measured: true, SourceBytes: 8192, SourceFiles: 1, FilesNew: 1, ResticMS: 600,
	}

	t.Run("the disks sum into the run", func(t *testing.T) {
		vm := &fakeVM{active: true, stateVal: "shut off"}
		r := &fakeRestic{summary: file}
		runs := &fakeRuns{}
		host := &fakeZFSHost{readData: []byte("zvol stream bytes")}
		zr := &fakeZvolRestic{backupSummary: zvol}

		d := sampleVMBackupDeps(t, vm, r, runs)
		d.BlockDisks = []backup.VMBlockDisk{{Dataset: "tank/vms/win10/disk1"}}
		d.ZFSHost = host
		d.ZvolRestic = zr

		if _, err := backup.BackupVMGraceful(t.Context(), d); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := runs.finishOf(t, "run-1").sum
		if got.SnapshotID != "fileSnap1234" {
			t.Fatalf("SnapshotID = %q, want the file backup's snapshot", got.SnapshotID)
		}
		if got.Bytes != 3072 || got.SourceBytes != 12288 || got.SourceFiles != 13 || got.ResticMS != 1500 {
			t.Fatalf("summary = %+v, want the file disks and the zvol disk summed", got)
		}
		if !got.Measured {
			t.Fatalf("Measured = false, want a measured run when every part was measured")
		}
	})

	t.Run("a failing disk records no metrics", func(t *testing.T) {
		vm := &fakeVM{active: true, stateVal: "shut off"}
		r := &fakeRestic{summary: file}
		runs := &fakeRuns{}
		host := &fakeZFSHost{snapshotCreateErr: errors.New("dataset does not exist")}

		d := sampleVMBackupDeps(t, vm, r, runs)
		d.BlockDisks = []backup.VMBlockDisk{{Dataset: "tank/vms/win10/disk1"}}
		d.ZFSHost = host
		d.ZvolRestic = &fakeZvolRestic{}

		if _, err := backup.BackupVMGraceful(t.Context(), d); err == nil {
			t.Fatal("expected an error when the zvol disk backup fails")
		}
		got := runs.finishOf(t, "run-1")
		if got.status != "failed" || got.sum != (backup.Summary{}) {
			t.Fatalf("finish = %+v, want an empty summary on a failure", got)
		}
	})
}

// TestBackupVMGracefulZvolAttemptsAllDisksAfterOneFails checks that the other
// disks are still backed up after one fails, so as much data as possible
// reaches the repository.
func TestBackupVMGracefulZvolAttemptsAllDisksAfterOneFails(t *testing.T) {
	vm := &fakeVM{active: true, stateVal: "shut off"}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "fileSnap1234"}}
	runs := &fakeRuns{}
	host := &fakeSelectiveZFSHost{failDataset: "tank/vms/win10/disk1"}
	zr := &fakeZvolRestic{backupSummary: backup.Summary{SnapshotID: "zvolSnap"}}

	d := sampleVMBackupDeps(t, vm, r, runs)
	d.BlockDisks = []backup.VMBlockDisk{
		{Dataset: "tank/vms/win10/disk1"},
		{Dataset: "tank/vms/win10/disk2"},
	}
	d.ZFSHost = host
	d.ZvolRestic = zr

	_, err := backup.BackupVMGraceful(t.Context(), d)
	if err == nil {
		t.Fatal("expected an error (disk1's zvol backup failed)")
	}
	if !host.disk2Attempted {
		t.Fatal("disk2's zvol backup must still be attempted after disk1 failed")
	}
}

// fakeSelectiveZFSHost fails SnapshotCreate for failDataset and notes any
// attempt on another dataset.
type fakeSelectiveZFSHost struct {
	failDataset    string
	disk2Attempted bool
}

func (f *fakeSelectiveZFSHost) SnapshotCreate(_ context.Context, dataset, _ string) error {
	if dataset != f.failDataset {
		f.disk2Attempted = true
	}
	if dataset == f.failDataset {
		return errors.New("dataset does not exist")
	}
	return nil
}
func (f *fakeSelectiveZFSHost) Snapshots(context.Context, string) ([]backup.ZFSSnapshot, error) {
	return nil, nil
}
func (f *fakeSelectiveZFSHost) Clone(context.Context, string, string) error { return nil }
func (f *fakeSelectiveZFSHost) Destroy(context.Context, string) error       { return nil }
func (f *fakeSelectiveZFSHost) VolSize(context.Context, string) (int64, bool, error) {
	return 0, false, nil
}
func (f *fakeSelectiveZFSHost) CreateVolume(context.Context, string, int64) error { return nil }
func (f *fakeSelectiveZFSHost) SetVolSize(context.Context, string, int64) error   { return nil }
func (f *fakeSelectiveZFSHost) ReadVolume(context.Context, string) (io.ReadCloser, func() error, error) {
	return io.NopCloser(bytes.NewReader(nil)), func() error { return nil }, nil
}
func (f *fakeSelectiveZFSHost) WriteVolume(_ context.Context, rd io.Reader, _ string) error {
	_, _ = io.ReadAll(rd)
	return nil
}

func TestRestoreVMInvokesZvolPathForBlockDisks(t *testing.T) {
	vm := &fakeVM{stateVal: "running"}
	r := &fakeRestic{}
	runs := &fakeRuns{}
	host := &fakeZFSHost{}
	zr := &fakeZvolRestic{dumpData: []byte("restored zvol bytes")}

	d := sampleVMRestoreDeps(t, vm, r, runs)
	d.BlockDisks = []backup.VMRestoreBlockDisk{{
		Dataset:    "tank/vms/win10/disk1",
		Size:       1 << 30,
		SnapshotID: "zvolsnapid1234",
		StdinPath:  "/vm-disks/tank/vms/win10/disk1.img",
	}}
	d.ZFSHost = host
	d.ZvolRestic = zr

	if err := backup.RestoreVM(t.Context(), d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !vmContains(zr.log, "dumpTo:/repo/vms:zvolsnapid1234:/vm-disks/tank/vms/win10/disk1.img") {
		t.Fatalf("restic dump was never invoked for the block disk; zr.log = %v", zr.log)
	}
	if got := string(host.written["tank/vms/win10/disk1"]); got != "restored zvol bytes" {
		t.Fatalf("wrote %q onto the zvol, want the restic dump", got)
	}

	if !vmContains(r.log, "restore:/repo/vms") {
		t.Fatalf("file-based restic restore not called: %v", r.log)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "success" {
		t.Fatalf("run finishes = %v, want [success]", runs.finishes)
	}
}

// TestRestoreVMFileOnlyNeedsNoZFSHost leaves BlockDisks, ZFSHost and
// ZvolRestic nil, so any use of the zvol path would panic.
func TestRestoreVMFileOnlyNeedsNoZFSHost(t *testing.T) {
	vm := &fakeVM{stateVal: "running"}
	r := &fakeRestic{}
	runs := &fakeRuns{}

	d := sampleVMRestoreDeps(t, vm, r, runs)

	if err := backup.RestoreVM(t.Context(), d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !vmContains(vm.log, "destroy:win10") || !vmContains(vm.log, "undefine:win10") || !vmContains(vm.log, "define:") {
		t.Fatalf("restore sequence changed: %v", vm.log)
	}
	if !vmContains(r.log, "restore:/repo/vms") {
		t.Fatalf("restic restore not called: %v", r.log)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "success" {
		t.Fatalf("run finishes = %v, want [success]", runs.finishes)
	}
}

// TestRestoreVMZvolFailureAbortsBeforeDefine checks that a VM with a disk that
// failed to restore is never defined.
func TestRestoreVMZvolFailureAbortsBeforeDefine(t *testing.T) {
	vm := &fakeVM{stateVal: "running"}
	r := &fakeRestic{}
	runs := &fakeRuns{}
	host := &fakeZFSHost{writeErr: errors.New("dd: no space left on device")}
	zr := &fakeZvolRestic{dumpData: []byte("data")}

	d := sampleVMRestoreDeps(t, vm, r, runs)
	d.BlockDisks = []backup.VMRestoreBlockDisk{{
		Dataset:    "tank/vms/win10/disk1",
		Size:       1 << 30,
		SnapshotID: "zvolsnapid1234",
		StdinPath:  "/vm-disks/tank/vms/win10/disk1.img",
	}}
	d.ZFSHost = host
	d.ZvolRestic = zr

	err := backup.RestoreVM(t.Context(), d)
	if err == nil {
		t.Fatal("expected an error when the zvol disk restore fails")
	}
	if vmContains(vm.log, "define:") {
		t.Fatalf("domain must not be defined when a block disk's restore failed: %v", vm.log)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "failed" {
		t.Fatalf("run finishes = %v, want [failed]", runs.finishes)
	}
}
