package backup_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
)

// ---------------------------------------------------------------------------
// Tests pinning Task 10 Step 5's wiring: BackupVMGraceful/BackupVMLive/
// RestoreVM must actually invoke BackupZvolDisk/RestoreZvolDisk for a
// domain's block-device-backed (zvol) disks — via VMBackupDeps.BlockDisks /
// VMRestoreDeps.BlockDisks — while a domain with ONLY file-backed disks must
// behave completely unchanged (the regression pin). Reuses fakeZFSHost/
// fakeZvolRestic from vm_zvol_test.go and fakeVM/fakeRestic/fakeRuns/
// sampleVMBackupDeps/sampleVMRestoreDeps/vmContains from vm_orchestrator_test.go
// — no real SSH/ZFS/restic system is touched by any test in this file.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// BackupVMGraceful + BlockDisks
// ---------------------------------------------------------------------------

func TestBackupVMGracefulInvokesZvolPathForBlockDisks(t *testing.T) {
	vm := &fakeVM{active: true, stateVal: "shut off"}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "fileSnap1234", Bytes: 1024}}
	runs := &fakeRuns{}
	host := &fakeZFSHost{streamSendData: []byte("zvol stream bytes")}
	zr := &fakeZvolRestic{backupSummary: backup.Summary{SnapshotID: "zvolSnap5678", Bytes: 2048}}

	d := sampleVMBackupDeps(t, vm, r, runs)
	d.BlockDisks = []backup.VMBlockDisk{{Dataset: "tank/vms/win10/disk1"}}
	d.ZFSHost = host
	d.ZvolRestic = zr

	sum, err := backup.BackupVMGraceful(t.Context(), d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The FILE-backed portion's summary is still what gets returned/recorded
	// (unchanged contract) — the zvol backup is a separate mechanism.
	if sum.SnapshotID != "fileSnap1234" {
		t.Fatalf("summary.SnapshotID = %q, want the file-backup snapshot id unchanged", sum.SnapshotID)
	}

	// The zvol path must actually have run: snapshot create → stream send →
	// snapshot destroy, exactly like a direct BackupZvolDisk call.
	if !vmContains(host.log, "snapshotCreate:tank/vms/win10/disk1@") {
		t.Fatalf("zfs snapshot was never created for the block disk; host.log = %v", host.log)
	}
	if !vmContains(host.log, "streamSend:tank/vms/win10/disk1@") {
		t.Fatalf("zfs send was never started for the block disk; host.log = %v", host.log)
	}
	if !vmContains(host.log, "snapshotDestroy:tank/vms/win10/disk1@") {
		t.Fatalf("zfs snapshot was never cleaned up for the block disk; host.log = %v", host.log)
	}
	if len(zr.log) == 0 {
		t.Fatalf("restic BackupStdin was never invoked for the block disk")
	}
	if string(zr.capturedStdin) != "zvol stream bytes" {
		t.Fatalf("restic BackupStdin received %q, want the zfs send stream bytes", zr.capturedStdin)
	}

	// The FILE-backed restic.Backup call must still have happened too — the
	// block-disk path is ADDITIVE, not a replacement.
	if !vmContains(r.log, "backup:/repo/vms") {
		t.Fatalf("file-backed restic backup not called: %v", r.log)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "success" {
		t.Fatalf("run finishes = %v, want [success]", runs.finishes)
	}
}

// TestBackupVMGracefulFileOnlyDiskUnchanged is the regression pin: a domain
// with ONLY file-backed disks (BlockDisks empty, the zero value every
// existing caller uses) must behave EXACTLY as before this wiring existed —
// no zvol machinery is ever touched.
func TestBackupVMGracefulFileOnlyDiskUnchanged(t *testing.T) {
	vm := &fakeVM{active: true, stateVal: "shut off"}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678", Bytes: 4096}}
	runs := &fakeRuns{}

	d := sampleVMBackupDeps(t, vm, r, runs)
	// d.BlockDisks, d.ZFSHost, d.ZvolRestic are all left at their zero value
	// (nil) — exactly what every real caller in internal/api/service.go does
	// today.

	sum, err := backup.BackupVMGraceful(t.Context(), d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sum.SnapshotID != "deadbeef12345678" || sum.Bytes != 4096 {
		t.Fatalf("summary = %+v, unchanged from before this wiring existed", sum)
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
	// Passing nil ZFSHost/ZvolRestic must never be dereferenced — proves the
	// zvol loop is a true no-op when BlockDisks is empty.
}

// TestBackupVMGracefulZvolFailureFailsWholeRun: a block disk's backup
// failure must fail the whole VM backup run (never silently report success
// while a disk's data never reached the repo) — and, since the VM was
// running, it must STILL be restarted (the ALWAYS-restart guarantee is
// unconditional).
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

// TestBackupVMGracefulZvolAttemptsAllDisksAfterOneFails: a multi-block-disk
// VM must still attempt every remaining disk after an earlier one fails, so
// as much data as possible reaches the repo (mirrors BackupVMLive's
// "commit every overlay" pattern for the same reason).
func TestBackupVMGracefulZvolAttemptsAllDisksAfterOneFails(t *testing.T) {
	vm := &fakeVM{active: true, stateVal: "shut off"}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "fileSnap1234"}}
	runs := &fakeRuns{}
	// A single fakeZFSHost/fakeZvolRestic pair is shared across both disk
	// calls (BackupZvolDisk doesn't care), so both attempts show up in one
	// log — snapshotCreateErr only fires on disk1, disk2 must still be tried.
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

// fakeSelectiveZFSHost fails SnapshotCreate only for one specific dataset,
// so a test can prove a LATER disk is still attempted after an earlier one
// fails.
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
func (f *fakeSelectiveZFSHost) SnapshotDestroy(context.Context, string, string) error { return nil }
func (f *fakeSelectiveZFSHost) StreamSend(_ context.Context, _, _ string) (io.ReadCloser, func() error, error) {
	return io.NopCloser(bytes.NewReader(nil)), func() error { return nil }, nil
}
func (f *fakeSelectiveZFSHost) StreamReceive(_ context.Context, rd io.Reader, _ string) error {
	_, _ = io.ReadAll(rd)
	return nil
}

// ---------------------------------------------------------------------------
// RestoreVM + BlockDisks
// ---------------------------------------------------------------------------

func TestRestoreVMInvokesZvolPathForBlockDisks(t *testing.T) {
	vm := &fakeVM{stateVal: "running"}
	r := &fakeRestic{}
	runs := &fakeRuns{}
	host := &fakeZFSHost{}
	zr := &fakeZvolRestic{dumpData: []byte("restored zvol bytes")}

	d := sampleVMRestoreDeps(t, vm, r, runs)
	d.BlockDisks = []backup.VMRestoreBlockDisk{{
		SourceDataset: "tank/vms/win10/disk1",
		SnapshotID:    "zvolsnapid1234",
		StdinPath:     "/vm-disks/tank/vms/win10/disk1@bombvault-20260816120000",
	}}
	d.ZFSHost = host
	d.ZvolRestic = zr

	if err := backup.RestoreVM(t.Context(), d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !vmContains(zr.log, "dumpTo:/repo/vms:zvolsnapid1234:/vm-disks/tank/vms/win10/disk1@bombvault-20260816120000") {
		t.Fatalf("restic dump was never invoked for the block disk; zr.log = %v", zr.log)
	}
	if host.streamReceiveTarget == "" {
		t.Fatal("zfs receive was never invoked for the block disk")
	}
	if host.streamReceiveTarget == d.BlockDisks[0].SourceDataset {
		t.Fatalf("zfs receive targeted the LIVE source dataset %q — data-destroying bug", host.streamReceiveTarget)
	}
	if !strings.HasPrefix(host.streamReceiveTarget, d.BlockDisks[0].SourceDataset+"-bombvault-restore-") {
		t.Fatalf("zfs receive target %q does not carry the expected fresh-dataset marker", host.streamReceiveTarget)
	}
	if string(host.streamReceiveData) != "restored zvol bytes" {
		t.Fatalf("zfs receive stdin = %q, want the restic dump bytes", host.streamReceiveData)
	}

	// The file-based restic restore must ALSO still have happened — additive,
	// not a replacement.
	if !vmContains(r.log, "restore:/repo/vms") {
		t.Fatalf("file-based restic restore not called: %v", r.log)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "success" {
		t.Fatalf("run finishes = %v, want [success]", runs.finishes)
	}
}

// TestRestoreVMFileOnlyDiskUnchanged is the regression pin for restore: a
// domain with ONLY file-backed disks (BlockDisks empty, the zero value every
// existing caller uses today) must behave EXACTLY as before this wiring
// existed.
func TestRestoreVMFileOnlyDiskUnchanged(t *testing.T) {
	vm := &fakeVM{stateVal: "running"}
	r := &fakeRestic{}
	runs := &fakeRuns{}

	d := sampleVMRestoreDeps(t, vm, r, runs)
	// d.BlockDisks, d.ZFSHost, d.ZvolRestic all left at zero value.

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

// TestRestoreVMZvolFailureAbortsBeforeDefine: a block disk's restore failure
// must abort the WHOLE restore BEFORE the domain is (re)defined — a VM with
// an incomplete/missing disk restore must never be defined/started.
func TestRestoreVMZvolFailureAbortsBeforeDefine(t *testing.T) {
	vm := &fakeVM{stateVal: "running"}
	r := &fakeRestic{}
	runs := &fakeRuns{}
	host := &fakeZFSHost{streamReceiveErr: errors.New("zfs: receive failed")}
	zr := &fakeZvolRestic{dumpData: []byte("data")}

	d := sampleVMRestoreDeps(t, vm, r, runs)
	d.BlockDisks = []backup.VMRestoreBlockDisk{{
		SourceDataset: "tank/vms/win10/disk1",
		SnapshotID:    "zvolsnapid1234",
		StdinPath:     "/vm-disks/tank/vms/win10/disk1@bombvault-20260816120000",
	}}
	d.ZFSHost = host
	d.ZvolRestic = zr

	err := backup.RestoreVM(t.Context(), d)
	if err == nil {
		t.Fatal("expected an error when the zvol disk restore fails")
	}
	if vmContains(vm.log, "define:") {
		t.Fatalf("domain must NOT be defined when a block disk's restore failed: %v", vm.log)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "failed" {
		t.Fatalf("run finishes = %v, want [failed]", runs.finishes)
	}
}
