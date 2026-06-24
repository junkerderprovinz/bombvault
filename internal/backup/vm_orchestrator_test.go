package backup_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
)

// fakeVM satisfies backup.VM for unit tests — no real virsh needed.
type fakeVM struct {
	log []string

	// snapshotSkip records the skipDevs passed to SnapshotCreateDiskOnly.
	snapshotSkip []string

	// active is returned by IsActive.
	active    bool
	stateVal  string
	stateErr  error
	activeErr error

	shutdownErr  error
	destroyErr   error
	startErr     error
	defineErr    error
	undefineErr  error
	autostartErr error
	dumpXMLVal   string
	dumpXMLErr   error

	snapshotErr     error
	blockcommitErr  error
	guestAgent      bool
	freezeOnQuiesce bool   // fail a quiesced snapshot with a freeze error (then a no-quiesce retry succeeds)
	snapshotQuiesce []bool // records the quiesce arg of each SnapshotCreateDiskOnly call
}

func (f *fakeVM) State(_ context.Context, name string) (string, error) {
	f.log = append(f.log, "state:"+name)
	return f.stateVal, f.stateErr
}

func (f *fakeVM) IsActive(_ context.Context, name string) (bool, error) {
	f.log = append(f.log, "isActive:"+name)
	return f.active, f.activeErr
}

func (f *fakeVM) DumpXML(_ context.Context, name string) (string, error) {
	f.log = append(f.log, "dumpxml:"+name)
	return f.dumpXMLVal, f.dumpXMLErr
}

func (f *fakeVM) Shutdown(_ context.Context, name string) error {
	f.log = append(f.log, "shutdown:"+name)
	return f.shutdownErr
}

func (f *fakeVM) Destroy(_ context.Context, name string) error {
	f.log = append(f.log, "destroy:"+name)
	return f.destroyErr
}

func (f *fakeVM) Start(_ context.Context, name string) error {
	f.log = append(f.log, "start:"+name)
	return f.startErr
}

func (f *fakeVM) Define(_ context.Context, xmlPath string) error {
	f.log = append(f.log, "define:"+xmlPath)
	return f.defineErr
}

func (f *fakeVM) Undefine(_ context.Context, name string) error {
	f.log = append(f.log, "undefine:"+name)
	return f.undefineErr
}

func (f *fakeVM) Autostart(_ context.Context, name string, on bool) error {
	v := "on"
	if !on {
		v = "off"
	}
	f.log = append(f.log, "autostart:"+name+":"+v)
	return f.autostartErr
}

func (f *fakeVM) SnapshotCreateDiskOnly(_ context.Context, name, _ string, quiesce bool, skipDevs []string) error {
	f.log = append(f.log, "snapshot:"+name)
	f.snapshotSkip = skipDevs
	f.snapshotQuiesce = append(f.snapshotQuiesce, quiesce)
	if f.freezeOnQuiesce && quiesce {
		return errors.New("guest agent command failed: fsfreeze hook failed")
	}
	return f.snapshotErr
}

func (f *fakeVM) BlockCommitActivePivot(_ context.Context, name, device string) error {
	f.log = append(f.log, "blockcommit:"+name+":"+device)
	return f.blockcommitErr
}

func (f *fakeVM) GuestAgentPing(_ context.Context, _ string) bool {
	f.log = append(f.log, "guestping")
	return f.guestAgent
}

// vmContains reports whether any log entry has the given prefix.
func vmContains(log []string, prefix string) bool {
	for _, e := range log {
		if strings.HasPrefix(e, prefix) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// BackupVMGraceful tests
// ---------------------------------------------------------------------------

func sampleVMBackupDeps(t *testing.T, vm *fakeVM, r *fakeRestic, runs *fakeRuns) backup.VMBackupDeps {
	t.Helper()
	return backup.VMBackupDeps{
		Name:      "win10",
		DiskPaths: []string{"/host/domains/win10/win10.qcow2"},
		NVRAMPath: "/host/domains/win10/win10_VARS.fd",
		RepoPath:  "/repo/vms",
		TargetID:  "vmtarget-1",
		DataDir:   t.TempDir(),
		VM:        vm,
		Restic:    r,
		Runs:      runs,
	}
}

func TestBackupVMGracefulHappyPath(t *testing.T) {
	// VM is running; state transitions to "shut off" after shutdown.
	vm := &fakeVM{active: true, stateVal: "shut off"}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678", Bytes: 4096}}
	runs := &fakeRuns{}

	sum, err := backup.BackupVMGraceful(t.Context(), sampleVMBackupDeps(t, vm, r, runs))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sum.SnapshotID != "deadbeef12345678" {
		t.Fatalf("snapshot id = %q", sum.SnapshotID)
	}
	// Graceful order: isActive → shutdown → (poll) → restic backup → start
	if !vmContains(vm.log, "isActive:") {
		t.Fatal("isActive must be called")
	}
	if !vmContains(vm.log, "shutdown:win10") {
		t.Fatal("shutdown must be called")
	}
	if !vmContains(vm.log, "start:win10") {
		t.Fatal("start must be called (ALWAYS restart)")
	}
	if !vmContains(r.log, "backup:/repo/vms") {
		t.Fatalf("restic backup not called: %v", r.log)
	}
	// Tags must include vm:win10 and p2.
	if !strings.Contains(r.log[0], "vm:win10") {
		t.Fatalf("tag vm:win10 missing in %v", r.log)
	}
	if !strings.Contains(r.log[0], "p2") {
		t.Fatalf("tag p2 missing in %v", r.log)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "success" {
		t.Fatalf("run finishes = %v, want [success]", runs.finishes)
	}
}

func TestBackupVMGracefulAlwaysStartsWhenWasRunning(t *testing.T) {
	// VM was running; restic fails → VM must still be started.
	vm := &fakeVM{active: true, stateVal: "shut off"}
	r := &fakeRestic{backupErr: errors.New("restic boom")}
	runs := &fakeRuns{}

	_, err := backup.BackupVMGraceful(t.Context(), sampleVMBackupDeps(t, vm, r, runs))
	if err == nil {
		t.Fatal("expected error to be re-thrown")
	}
	if !vmContains(vm.log, "start:win10") {
		t.Fatal("VM must be restarted even when backup fails and VM was running")
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "failed" {
		t.Fatalf("run finishes = %v, want [failed]", runs.finishes)
	}
}

func TestBackupVMGracefulDoesNotStartWhenWasNotRunning(t *testing.T) {
	// VM was already stopped — must NOT be started after backup.
	vm := &fakeVM{active: false, stateVal: "shut off"}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "abcd1234"}}
	runs := &fakeRuns{}

	if _, err := backup.BackupVMGraceful(t.Context(), sampleVMBackupDeps(t, vm, r, runs)); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if vmContains(vm.log, "start:win10") {
		t.Fatal("VM must NOT be started when it was already stopped before backup")
	}
}

func TestBackupVMGracefulDestroyOnShutdownTimeout(t *testing.T) {
	// State never transitions to "shut off"; the poll loop gives up and calls Destroy.
	// ShutdownTimeout=1 means 1 poll cycle before giving up.
	vm := &fakeVM{active: true, stateVal: "running"} // never transitions
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "abcd1234"}}
	runs := &fakeRuns{}

	deps := sampleVMBackupDeps(t, vm, r, runs)
	deps.ShutdownTimeout = 1 // instant timeout in tests
	_, _ = backup.BackupVMGraceful(t.Context(), deps)
	// Destroy must have been called after the timeout.
	if !vmContains(vm.log, "destroy:win10") {
		t.Fatal("destroy must be called when graceful shutdown times out")
	}
}

// ---------------------------------------------------------------------------
// RestoreVM tests
// ---------------------------------------------------------------------------

func sampleVMRestoreDeps(t *testing.T, vm *fakeVM, r *fakeRestic, runs *fakeRuns) backup.VMRestoreDeps {
	t.Helper()
	return backup.VMRestoreDeps{
		Confirmed:    true,
		Name:         "win10",
		SnapshotID:   "deadbeef12345678",
		DiskPaths:    []string{"/host/domains/win10/win10.qcow2"},
		NVRAMPath:    "/host/domains/win10/win10_VARS.fd",
		DomainXML:    "<domain><name>win10</name></domain>",
		WasAutostart: true,
		StartAfter:   true,
		RepoPath:     "/repo/vms",
		TargetID:     "vmtarget-1",
		DataDir:      t.TempDir(),
		VM:           vm,
		Restic:       r,
		Runs:         runs,
	}
}

func TestRestoreVMAbortsWhenNotConfirmed(t *testing.T) {
	vm := &fakeVM{stateVal: ""}
	r := &fakeRestic{}
	runs := &fakeRuns{}

	deps := sampleVMRestoreDeps(t, vm, r, runs)
	deps.Confirmed = false

	err := backup.RestoreVM(t.Context(), deps)
	if err == nil || !errors.Is(err, backup.ErrNotConfirmed) {
		t.Fatalf("expected ErrNotConfirmed, got %v", err)
	}
	if vmContains(runs.log, "runStart:") {
		t.Fatal("runStart must NOT be called when not confirmed")
	}
}

func TestRestoreVMRejectsBadSnapshotID(t *testing.T) {
	vm := &fakeVM{stateVal: ""}
	r := &fakeRestic{}
	runs := &fakeRuns{}

	deps := sampleVMRestoreDeps(t, vm, r, runs)
	deps.SnapshotID = "not-hex!"

	err := backup.RestoreVM(t.Context(), deps)
	if err == nil || !errors.Is(err, backup.ErrInvalidSnapshotID) {
		t.Fatalf("expected ErrInvalidSnapshotID, got %v", err)
	}
}

// TestRestoreVMAbortsWhenSnapshotMissing pins the restore preflight: a snapshot
// that can't be read must abort BEFORE destroy/undefine, so a running VM is
// never torn down for a doomed restore.
func TestRestoreVMAbortsWhenSnapshotMissing(t *testing.T) {
	vm := &fakeVM{stateVal: "running"}
	r := &fakeRestic{verifyErr: errors.New("snapshot not found")}
	runs := &fakeRuns{}

	err := backup.RestoreVM(t.Context(), sampleVMRestoreDeps(t, vm, r, runs))
	if err == nil || !strings.Contains(err.Error(), "preflight") {
		t.Fatalf("expected snapshot-preflight abort, got %v", err)
	}
	if vmContains(vm.log, "destroy:") || vmContains(vm.log, "undefine:") || vmContains(r.log, "restore:") {
		t.Fatalf("nothing destructive allowed when the preflight fails: vm=%v restic=%v", vm.log, r.log)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "failed" {
		t.Fatalf("run finishes = %v, want [failed]", runs.finishes)
	}
}

func TestRestoreVMRejectsUnsafePath(t *testing.T) {
	vm := &fakeVM{stateVal: ""}
	r := &fakeRestic{}
	runs := &fakeRuns{}

	deps := sampleVMRestoreDeps(t, vm, r, runs)
	deps.DiskPaths = []string{"/host/domains/../../../etc/passwd"}

	err := backup.RestoreVM(t.Context(), deps)
	if err == nil || !strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("expected unsafe path rejection, got %v", err)
	}
}

func TestRestoreVMHappyPath(t *testing.T) {
	// VM is running when restore is called → destroy + undefine before restore.
	vm := &fakeVM{stateVal: "running"}
	r := &fakeRestic{}
	runs := &fakeRuns{}

	if err := backup.RestoreVM(t.Context(), sampleVMRestoreDeps(t, vm, r, runs)); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Order: state → destroy → undefine → (restic restore) → define → autostart → start
	order := vm.log
	idxDestroy := -1
	idxUndefine := -1
	idxDefine := -1
	idxAutostart := -1
	idxStart := -1
	for i, e := range order {
		switch {
		case strings.HasPrefix(e, "destroy:"):
			idxDestroy = i
		case strings.HasPrefix(e, "undefine:"):
			idxUndefine = i
		case strings.HasPrefix(e, "define:"):
			idxDefine = i
		case strings.HasPrefix(e, "autostart:"):
			idxAutostart = i
		case strings.HasPrefix(e, "start:"):
			idxStart = i
		}
	}
	if idxDestroy < 0 {
		t.Fatal("destroy not called for running VM")
	}
	if idxUndefine < 0 {
		t.Fatal("undefine not called")
	}
	if idxDefine < 0 {
		t.Fatal("define not called")
	}
	if idxAutostart < 0 {
		t.Fatal("autostart not called")
	}
	if idxStart < 0 {
		t.Fatal("start not called when StartAfter=true")
	}
	if idxDestroy > idxUndefine {
		t.Fatal("destroy must precede undefine")
	}
	if idxUndefine > idxDefine {
		t.Fatal("undefine must precede define")
	}
	if idxDefine > idxStart {
		t.Fatal("define must precede start")
	}

	// Restic restore must have been called.
	if !vmContains(r.log, "restore:/repo/vms:deadbeef12345678") {
		t.Fatalf("restic restore not called: %v", r.log)
	}
	// Autostart with on=true (WasAutostart=true).
	found := false
	for _, e := range vm.log {
		if e == "autostart:win10:on" {
			found = true
		}
	}
	if !found {
		t.Fatal("autostart:win10:on not called")
	}
	// Run recorded success.
	if len(runs.finishes) != 1 || runs.finishes[0] != "success" {
		t.Fatalf("run finishes = %v, want [success]", runs.finishes)
	}
	// define was called with a file that exists (temp xml file was written).
	for _, e := range vm.log {
		if strings.HasPrefix(e, "define:") {
			xmlPath := strings.TrimPrefix(e, "define:")
			if _, statErr := os.Stat(xmlPath); statErr != nil {
				t.Fatalf("define xml file does not exist: %v", statErr)
			}
		}
	}
}

func TestRestoreVMDoesNotDestroyWhenAbsent(t *testing.T) {
	// VM does not exist on host → destroy/undefine must NOT be called.
	vm := &fakeVM{stateVal: ""} // empty state = not found
	r := &fakeRestic{}
	runs := &fakeRuns{}

	if err := backup.RestoreVM(t.Context(), sampleVMRestoreDeps(t, vm, r, runs)); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if vmContains(vm.log, "destroy:") {
		t.Fatal("destroy must NOT be called when VM is absent")
	}
	if vmContains(vm.log, "undefine:") {
		t.Fatal("undefine must NOT be called when VM is absent")
	}
	if !vmContains(r.log, "restore:") {
		t.Fatal("restic restore must still run")
	}
}

func TestRestoreVMRecordsFailedOnResticError(t *testing.T) {
	vm := &fakeVM{stateVal: ""}
	r := &fakeRestic{restoreErr: errors.New("restic failed")}
	runs := &fakeRuns{}

	err := backup.RestoreVM(t.Context(), sampleVMRestoreDeps(t, vm, r, runs))
	if err == nil {
		t.Fatal("expected error")
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "failed" {
		t.Fatalf("run finishes = %v, want [failed]", runs.finishes)
	}
}

// ---------------------------------------------------------------------------
// BackupVMLive tests (safety-critical)
// ---------------------------------------------------------------------------

func liveDeps(t *testing.T, vm *fakeVM, r *fakeRestic, runs *fakeRuns) backup.VMBackupDeps {
	t.Helper()
	d := sampleVMBackupDeps(t, vm, r, runs)
	d.DiskDevice = "vda"
	return d
}

func TestBackupVMLiveHappyPath(t *testing.T) {
	vm := &fakeVM{guestAgent: true}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678", Bytes: 4096}}
	runs := &fakeRuns{}

	sum, err := backup.BackupVMLive(t.Context(), liveDeps(t, vm, r, runs))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if sum.SnapshotID != "deadbeef12345678" {
		t.Fatalf("snapshot id = %q", sum.SnapshotID)
	}
	if !vmContains(vm.log, "snapshot:win10") {
		t.Fatalf("snapshot not created: %v", vm.log)
	}
	if !vmContains(vm.log, "blockcommit:win10:vda") {
		t.Fatalf("blockcommit not called: %v", vm.log)
	}
	if vmContains(vm.log, "shutdown:") || vmContains(vm.log, "destroy:") {
		t.Fatalf("live backup must NOT shut down the VM: %v", vm.log)
	}
	if !vmContains(r.log, "backup:/repo/vms") {
		t.Fatalf("restic backup not called: %v", r.log)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "success" {
		t.Fatalf("run finishes = %v, want [success]", runs.finishes)
	}
}

// TestBackupVMLiveCommitsAllWritableDisks: a multi-disk VM must have EVERY
// overlay committed back, not just the first — otherwise disks 2..N keep
// diverging on an uncommitted overlay.
func TestBackupVMLiveCommitsAllWritableDisks(t *testing.T) {
	vm := &fakeVM{active: true}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678"}}
	runs := &fakeRuns{}
	d := liveDeps(t, vm, r, runs)
	d.CommitDevs = []string{"vda", "vdb"}

	if _, err := backup.BackupVMLive(t.Context(), d); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !vmContains(vm.log, "blockcommit:win10:vda") || !vmContains(vm.log, "blockcommit:win10:vdb") {
		t.Fatalf("every writable overlay must be committed, got %v", vm.log)
	}
}

// The core safety guarantee: if blockcommit fails, the VM is left RUNNING and is
// never destroyed/undefined, and the error reassures the user.
func TestBackupVMLiveCommitFailsLeavesVMRunning(t *testing.T) {
	vm := &fakeVM{blockcommitErr: errors.New("commit boom")}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678"}}
	runs := &fakeRuns{}

	_, err := backup.BackupVMLive(t.Context(), liveDeps(t, vm, r, runs))
	if err == nil {
		t.Fatal("expected error")
	}
	if vmContains(vm.log, "destroy:") || vmContains(vm.log, "undefine:") {
		t.Fatalf("must never tear down the VM on commit failure: %v", vm.log)
	}
	if !strings.Contains(err.Error(), "STILL RUNNING") {
		t.Fatalf("error must reassure the VM is usable: %v", err)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "failed" {
		t.Fatalf("run finishes = %v, want [failed]", runs.finishes)
	}
}

func TestBackupVMLiveNoDiskDeviceFailsClearly(t *testing.T) {
	// No writable disk to commit ⇒ live can't work. With NO graceful fallback it
	// fails clearly and NEVER shuts the VM down (use the graceful method instead).
	vm := &fakeVM{active: true}
	r := &fakeRestic{}
	runs := &fakeRuns{}
	d := sampleVMBackupDeps(t, vm, r, runs) // DiskDevice empty

	_, err := backup.BackupVMLive(t.Context(), d)
	if err == nil {
		t.Fatal("expected a clear error for a live backup with no writable disk")
	}
	if vmContains(vm.log, "snapshot:") {
		t.Fatalf("must not snapshot without a commit target: %v", vm.log)
	}
	if vmContains(vm.log, "shutdown:") || vmContains(vm.log, "destroy:") {
		t.Fatalf("live must NEVER shut down the VM: %v", vm.log)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "failed" {
		t.Fatalf("run finishes = %v, want [failed]", runs.finishes)
	}
}

// TestBackupVMLiveSnapshotFailureNeverShutsDown: when the live snapshot can't be
// created, live fails clearly and the VM is left RUNNING — never shut down (no
// graceful fallback). Reliability comes from the leftover-overlay recovery in the
// service layer, not from shutting the VM down.
func TestBackupVMLiveSnapshotFailureNeverShutsDown(t *testing.T) {
	vm := &fakeVM{active: true, snapshotErr: errors.New("snapshot device busy")}
	r := &fakeRestic{}
	runs := &fakeRuns{}

	_, err := backup.BackupVMLive(t.Context(), liveDeps(t, vm, r, runs))
	if err == nil {
		t.Fatal("expected snapshot failure to surface as an error")
	}
	if !vmContains(vm.log, "snapshot:win10") {
		t.Fatalf("live snapshot must have been attempted: %v", vm.log)
	}
	if vmContains(vm.log, "shutdown:") || vmContains(vm.log, "destroy:") {
		t.Fatalf("live must NEVER shut down the VM on snapshot failure: %v", vm.log)
	}
	if vmContains(r.log, "backup:") {
		t.Fatalf("restic must not run when the snapshot failed: %v", r.log)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "failed" {
		t.Fatalf("run finishes = %v, want [failed]", runs.finishes)
	}
}

// PreDefine (NVRAM write-back) runs after restic restore and before define.
func TestRestoreVMRunsPreDefineBeforeDefine(t *testing.T) {
	vm := &fakeVM{stateVal: ""}
	r := &fakeRestic{}
	runs := &fakeRuns{}
	d := sampleVMRestoreDeps(t, vm, r, runs)
	called := false
	d.PreDefine = func(_ context.Context) error { called = true; return nil }

	if err := backup.RestoreVM(t.Context(), d); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !called {
		t.Fatal("PreDefine was not called")
	}
}

// TestBackupVMLiveFallsBackWithoutQuiesceOnFreeze: a guest with the agent up but
// a failing fsfreeze hook (e.g. Home Assistant during startup) must not fail the
// whole live backup — the snapshot retries crash-consistent without --quiesce.
func TestBackupVMLiveFallsBackWithoutQuiesceOnFreeze(t *testing.T) {
	vm := &fakeVM{active: true, guestAgent: true, freezeOnQuiesce: true}
	r := &fakeRestic{}
	runs := &fakeRuns{}

	if _, err := backup.BackupVMLive(t.Context(), liveDeps(t, vm, r, runs)); err != nil {
		t.Fatalf("expected fsfreeze fallback to succeed, got %v", err)
	}
	if len(vm.snapshotQuiesce) != 2 || !vm.snapshotQuiesce[0] || vm.snapshotQuiesce[1] {
		t.Fatalf("expected a quiesced attempt then a crash-consistent retry, got %v", vm.snapshotQuiesce)
	}
	if !vmContains(r.log, "backup:") {
		t.Fatalf("restic backup must run after the fallback snapshot: %v", r.log)
	}
}
