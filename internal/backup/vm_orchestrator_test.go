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

// TestRestoreVMRemapsDisksAndLeavesStopped pins the cross-instance (#122) shape:
// when RestoreDirs is set the restore uses RestoreSubtreeTo (source subtree →
// chosen destination dir) rather than restoring each path back to its own
// location, and with WasAutostart=false + StartAfter=false the VM is defined,
// autostart is cleared, and it is NEVER started.
func TestRestoreVMRemapsDisksAndLeavesStopped(t *testing.T) {
	vm := &fakeVM{stateVal: ""} // absent on the destination host
	r := &fakeRestic{}
	runs := &fakeRuns{}

	deps := sampleVMRestoreDeps(t, vm, r, runs)
	deps.WasAutostart = false
	deps.StartAfter = false
	// The disks END UP under /host/user/user/domains/win10 on the destination; the
	// snapshot's own subtree is the SOURCE dir.
	deps.DiskPaths = []string{"/host/user/user/domains/win10/win10.qcow2"}
	deps.NVRAMPath = ""
	deps.RestoreDirs = []backup.VMRestoreDir{
		{Subtree: "/host/user/zfs/domains/win10", Target: "/host/user/user/domains/win10"},
	}

	if err := backup.RestoreVM(t.Context(), deps); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Remapped restore: RestoreSubtreeTo(source -> dest), NOT the restore-in-place path.
	wantSubtree := "restoreSubtree:/repo/vms:deadbeef12345678:/host/user/zfs/domains/win10->/host/user/user/domains/win10"
	if !vmContains(r.log, wantSubtree) {
		t.Fatalf("expected remapped RestoreSubtreeTo, got %v", r.log)
	}
	for _, e := range r.log {
		if strings.HasPrefix(e, "restore:") {
			t.Fatalf("remapped restore must NOT use restore-in-place, got %v", r.log)
		}
	}
	// Autostart cleared, VM never started.
	if !vmContains(vm.log, "autostart:win10:off") {
		t.Fatalf("autostart must be cleared for a cross-instance restore, got %v", vm.log)
	}
	for _, e := range vm.log {
		if strings.HasPrefix(e, "start:") {
			t.Fatalf("a left-stopped restore must NOT start the VM, got %v", vm.log)
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

// ---------------------------------------------------------------------------
// TPM (vTPM state) tests — Task 11. Mirror the NVRAM path-list tests above:
// TPMPath is given the EXACT SAME treatment as NVRAMPath (appended to the
// restic path list when non-empty, nothing extra when empty), proven by
// asserting the actual paths restic receives — not just that no error occurs.
// ---------------------------------------------------------------------------

const sampleTPMPath = "/var/db/system/vm/tpm/1_win10_tpm_state"

// TestBackupVMGracefulIncludesTPMPathWhenPresent proves TPMPath is wired into
// the SAME real call site NVRAMPath already uses in runVMGraceful: when set,
// it rides along in the same restic Backup call as the disk(s) and NVRAM.
func TestBackupVMGracefulIncludesTPMPathWhenPresent(t *testing.T) {
	vm := &fakeVM{active: true, stateVal: "shut off"}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678"}}
	runs := &fakeRuns{}
	d := sampleVMBackupDeps(t, vm, r, runs)
	d.TPMPath = sampleTPMPath

	if _, err := backup.BackupVMGraceful(t.Context(), d); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !vmContains(r.log, "backup:") || !strings.Contains(r.log[0], sampleTPMPath) {
		t.Fatalf("expected TPM path %q in the restic backup call, got %v", sampleTPMPath, r.log)
	}
}

// TestBackupVMGracefulOmitsTPMPathWhenAbsent is the explicit regression pin:
// a VM with no vTPM (TPMPath empty, every VM in production today) must
// produce EXACTLY the same restic backup call as before TPM support existed
// — not merely "no error", an exact match on what restic receives.
func TestBackupVMGracefulOmitsTPMPathWhenAbsent(t *testing.T) {
	vm := &fakeVM{active: true, stateVal: "shut off"}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678"}}
	runs := &fakeRuns{}
	d := sampleVMBackupDeps(t, vm, r, runs) // TPMPath left unset (zero value)

	if _, err := backup.BackupVMGraceful(t.Context(), d); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	want := "backup:/repo/vms:/host/domains/win10/win10.qcow2,/host/domains/win10/win10_VARS.fd:vm:win10,p2"
	if len(r.log) != 1 || r.log[0] != want {
		t.Fatalf("restic backup call = %v, want [%q] (byte-identical to pre-TPM behavior)", r.log, want)
	}
}

// TestBackupVMLiveIncludesTPMPathWhenPresent mirrors the graceful-path test
// above for BackupVMLive's own separate restic Backup call.
func TestBackupVMLiveIncludesTPMPathWhenPresent(t *testing.T) {
	vm := &fakeVM{guestAgent: true}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678"}}
	runs := &fakeRuns{}
	d := liveDeps(t, vm, r, runs)
	d.TPMPath = sampleTPMPath

	if _, err := backup.BackupVMLive(t.Context(), d); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !vmContains(r.log, "backup:") || !strings.Contains(r.log[0], sampleTPMPath) {
		t.Fatalf("expected TPM path %q in the live restic backup call, got %v", sampleTPMPath, r.log)
	}
}

// TestBackupVMLiveOmitsTPMPathWhenAbsent is BackupVMLive's regression pin,
// mirroring TestBackupVMGracefulOmitsTPMPathWhenAbsent.
func TestBackupVMLiveOmitsTPMPathWhenAbsent(t *testing.T) {
	vm := &fakeVM{guestAgent: true}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678"}}
	runs := &fakeRuns{}
	d := liveDeps(t, vm, r, runs) // TPMPath left unset

	if _, err := backup.BackupVMLive(t.Context(), d); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	want := "backup:/repo/vms:/host/domains/win10/win10.qcow2,/host/domains/win10/win10_VARS.fd:vm:win10,p2,live"
	if len(r.log) != 1 || r.log[0] != want {
		t.Fatalf("restic backup call = %v, want [%q] (byte-identical to pre-TPM behavior)", r.log, want)
	}
}

// TestRestoreVMIncludesTPMPathWhenPresent proves TPMPath is wired into
// runVMRestore's SAME real call site NVRAMPath already uses: it is validated
// by the same path-safety guard and its parent directory is included in the
// restic restore-directory list alongside the disk(s)' and NVRAM's.
func TestRestoreVMIncludesTPMPathWhenPresent(t *testing.T) {
	vm := &fakeVM{stateVal: ""}
	r := &fakeRestic{}
	runs := &fakeRuns{}
	d := sampleVMRestoreDeps(t, vm, r, runs)
	d.TPMPath = sampleTPMPath

	if err := backup.RestoreVM(t.Context(), d); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	found := false
	for _, p := range r.capturedPaths {
		if p == "/var/db/system/vm/tpm" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected the TPM path's parent dir in the restic restore call, got %v", r.capturedPaths)
	}
}

// TestRestoreVMOmitsTPMPathWhenAbsent is the restore-side regression pin.
func TestRestoreVMOmitsTPMPathWhenAbsent(t *testing.T) {
	vm := &fakeVM{stateVal: ""}
	r := &fakeRestic{}
	runs := &fakeRuns{}
	d := sampleVMRestoreDeps(t, vm, r, runs) // TPMPath left unset

	if err := backup.RestoreVM(t.Context(), d); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	want := []string{"/host/domains/win10"}
	if len(r.capturedPaths) != len(want) || r.capturedPaths[0] != want[0] {
		t.Fatalf("restic restore dirs = %v, want %v (byte-identical to pre-TPM behavior)", r.capturedPaths, want)
	}
}

// TestRestoreVMRejectsUnsafeTPMPath proves TPMPath goes through the SAME
// path-safety validation as DiskPaths/NVRAMPath (allPaths in runVMRestore) —
// not a separately-trusted field.
func TestRestoreVMRejectsUnsafeTPMPath(t *testing.T) {
	vm := &fakeVM{stateVal: ""}
	r := &fakeRestic{}
	runs := &fakeRuns{}
	d := sampleVMRestoreDeps(t, vm, r, runs)
	d.TPMPath = "../../../etc/passwd"

	err := backup.RestoreVM(t.Context(), d)
	if err == nil || !strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("expected unsafe path rejection for TPMPath, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// RunTag (per-run correlation tag) tests — Task 1 of
// the design notes. See
// VMBackupDeps.RunTag's doc comment for the full design context: when set,
// it is appended ADDITIONALLY to every restic backup call this file makes
// for one VM backup (the main file-backed backup + each zvol disk's own
// backup via backupBlockDisksAndLog), alongside — never replacing — that
// call's own identity tag. Empty (the default, and what every OTHER test in
// this file leaves it at) must be a byte-identical no-op, pinned explicitly
// below by exact-match assertions on the fakes' recorded call arguments —
// not merely "no error returned".
//
// The restore-side tests below establish a DIFFERENT, equally real finding
// from reading the actual code (see VMRestoreDeps.RunTag's doc comment):
// none of restic's restore-side methods (VerifySnapshot/RestorePaths/
// RestoreSubtreeTo/DumpTo/StreamReceive) take a tags parameter at all, so
// RunTag has NO restic call site to attach to on the restore path — setting
// it is proven to leave every restore call's recorded arguments unchanged.
// ---------------------------------------------------------------------------

// TestRunTagEmptyIsByteIdenticalGraceful is case (1) for runVMGraceful: with
// RunTag left at its zero value, the restic backup call must be an EXACT
// match for what it was before RunTag existed.
func TestRunTagEmptyIsByteIdenticalGraceful(t *testing.T) {
	vm := &fakeVM{active: true, stateVal: "shut off"}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678"}}
	runs := &fakeRuns{}
	d := sampleVMBackupDeps(t, vm, r, runs) // RunTag left unset (zero value)

	if _, err := backup.BackupVMGraceful(t.Context(), d); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	want := "backup:/repo/vms:/host/domains/win10/win10.qcow2,/host/domains/win10/win10_VARS.fd:vm:win10,p2"
	if len(r.log) != 1 || r.log[0] != want {
		t.Fatalf("restic backup call = %v, want [%q] (byte-identical to pre-RunTag behavior)", r.log, want)
	}
}

// TestRunTagEmptyIsByteIdenticalLive mirrors the above for runVMLive's own
// separate restic Backup call.
func TestRunTagEmptyIsByteIdenticalLive(t *testing.T) {
	vm := &fakeVM{guestAgent: true}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678"}}
	runs := &fakeRuns{}
	d := liveDeps(t, vm, r, runs) // RunTag left unset

	if _, err := backup.BackupVMLive(t.Context(), d); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	want := "backup:/repo/vms:/host/domains/win10/win10.qcow2,/host/domains/win10/win10_VARS.fd:vm:win10,p2,live"
	if len(r.log) != 1 || r.log[0] != want {
		t.Fatalf("restic backup call = %v, want [%q] (byte-identical to pre-RunTag behavior)", r.log, want)
	}
}

// TestRunTagEmptyIsByteIdenticalZvolBackup pins backupBlockDisksAndLog's
// per-zvol-disk tags exactly as they were before RunTag existed.
func TestRunTagEmptyIsByteIdenticalZvolBackup(t *testing.T) {
	vm := &fakeVM{active: true, stateVal: "shut off"}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "fileSnap1234"}}
	runs := &fakeRuns{}
	host := &fakeZFSHost{streamSendData: []byte("zvol stream bytes")}
	zr := &fakeZvolRestic{backupSummary: backup.Summary{SnapshotID: "zvolSnap5678"}}

	d := sampleVMBackupDeps(t, vm, r, runs) // RunTag left unset
	d.BlockDisks = []backup.VMBlockDisk{{Dataset: "tank/vms/win10/disk1"}}
	d.ZFSHost = host
	d.ZvolRestic = zr

	if _, err := backup.BackupVMGraceful(t.Context(), d); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(zr.log) != 1 {
		t.Fatalf("expected exactly one zvol backup call, got %v", zr.log)
	}
	if !strings.HasSuffix(zr.log[0], ":vm:win10,p2") {
		t.Fatalf("zvol backup call = %q, want tags suffix %q (byte-identical to pre-RunTag behavior)", zr.log[0], ":vm:win10,p2")
	}
}

// TestRunTagEmptyIsByteIdenticalRestore proves RunTag has no effect on the
// restore side at all when left empty — the regression pin that matters
// most given every existing VMRestoreDeps-constructing test in this package
// leaves the field unset.
func TestRunTagEmptyIsByteIdenticalRestore(t *testing.T) {
	vm := &fakeVM{stateVal: ""}
	r := &fakeRestic{}
	runs := &fakeRuns{}
	d := sampleVMRestoreDeps(t, vm, r, runs) // RunTag left unset

	if err := backup.RestoreVM(t.Context(), d); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	want := "restore:/repo/vms:deadbeef12345678:/host/domains/win10"
	if !vmContains(r.log, want) {
		t.Fatalf("restic restore call = %v, want a call starting %q (byte-identical to pre-RunTag behavior)", r.log, want)
	}
}

// TestRunTagSetAppearsOnAllBackupCalls is case (2): RunTag set on a VM with
// 1 file disk + 2 zvol disks — proves ALL 3 resulting restic backup calls
// (the main file-backed backup + each zvol disk's own backup) carry BOTH
// their own identity tag AND the shared RunTag, verified by inspecting the
// fakes' actual recorded call arguments, not merely "no error returned".
func TestRunTagSetAppearsOnAllBackupCalls(t *testing.T) {
	vm := &fakeVM{active: true, stateVal: "shut off"}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "fileSnap1234"}}
	runs := &fakeRuns{}
	host := &fakeZFSHost{streamSendData: []byte("zvol stream bytes")}
	zr := &fakeZvolRestic{backupSummary: backup.Summary{SnapshotID: "zvolSnap"}}

	d := sampleVMBackupDeps(t, vm, r, runs)
	d.RunTag = "vmrun:run-42"
	d.BlockDisks = []backup.VMBlockDisk{
		{Dataset: "tank/vms/win10/disk1"},
		{Dataset: "tank/vms/win10/disk2"},
	}
	d.ZFSHost = host
	d.ZvolRestic = zr

	if _, err := backup.BackupVMGraceful(t.Context(), d); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// The main file-backed backup: its own "vm:win10" identity tag PLUS RunTag.
	if len(r.log) != 1 {
		t.Fatalf("expected exactly one file-backed restic backup call, got %v", r.log)
	}
	want := "backup:/repo/vms:/host/domains/win10/win10.qcow2,/host/domains/win10/win10_VARS.fd:vm:win10,p2,vmrun:run-42"
	if r.log[0] != want {
		t.Fatalf("file-backed backup call = %q, want %q", r.log[0], want)
	}

	// Each of the 2 zvol disk backups: same "vm:win10" identity tag PLUS RunTag.
	if len(zr.log) != 2 {
		t.Fatalf("expected exactly 2 zvol backup calls (1 per disk), got %v", zr.log)
	}
	for _, entry := range zr.log {
		if !strings.HasSuffix(entry, ":vm:win10,p2,vmrun:run-42") {
			t.Fatalf("zvol backup call %q missing identity tag + RunTag suffix %q", entry, ":vm:win10,p2,vmrun:run-42")
		}
	}
}

// TestVMBlockDiskDevGivesDistinctIdentityTag proves that when a BlockDisks
// entry carries a Dev (v8.0.0 VM service-layer integration, Task 2 — see
// VMBlockDisk.Dev's doc comment), its restic backup call is tagged with its
// OWN "vm:<name>:zvol:<dev>" identity — never the file-backed backup's
// "vm:<name>" tag — alongside RunTag, so a caller (internal/api/service.go)
// can apply retention to each disk's history as its own group.
func TestVMBlockDiskDevGivesDistinctIdentityTag(t *testing.T) {
	vm := &fakeVM{active: true, stateVal: "shut off"}
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "fileSnap1234"}}
	runs := &fakeRuns{}
	host := &fakeZFSHost{streamSendData: []byte("zvol stream bytes")}
	zr := &fakeZvolRestic{backupSummary: backup.Summary{SnapshotID: "zvolSnap"}}

	d := sampleVMBackupDeps(t, vm, r, runs)
	d.RunTag = "vmrun:run-42"
	d.BlockDisks = []backup.VMBlockDisk{
		{Dataset: "tank/vms/win10/disk1", Dev: "vdb"},
		{Dataset: "tank/vms/win10/disk2", Dev: "vdc"},
	}
	d.ZFSHost = host
	d.ZvolRestic = zr

	if _, err := backup.BackupVMGraceful(t.Context(), d); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(zr.log) != 2 {
		t.Fatalf("expected exactly 2 zvol backup calls (1 per disk), got %v", zr.log)
	}
	if !strings.HasSuffix(zr.log[0], ":vm:win10:zvol:vdb,p2,vmrun:run-42") {
		t.Fatalf("disk1 backup call %q missing its own zvol identity tag", zr.log[0])
	}
	if !strings.HasSuffix(zr.log[1], ":vm:win10:zvol:vdc,p2,vmrun:run-42") {
		t.Fatalf("disk2 backup call %q missing its own zvol identity tag", zr.log[1])
	}
	// Never the shared "vm:win10" tag the file-backed backup uses — each disk
	// is its own retention group once it carries a Dev.
	for _, entry := range zr.log {
		if strings.Contains(entry, ":vm:win10,") {
			t.Fatalf("zvol backup call %q wrongly carries the shared vm:win10 tag instead of its own zvol:<dev> tag", entry)
		}
	}
}

// TestRunTagSetHasNoEffectOnRestoreCalls is case (3): RunTag set on
// VMRestoreDeps alongside a matching multi-disk BlockDisks set (1 file disk
// + 2 zvol disks) restores all 3 correctly, and — since restic's
// restore-side methods take a snapshot id, never tags (confirmed by reading
// their real signatures; see VMRestoreDeps.RunTag's doc comment) — RunTag
// has NO effect on any restic call's recorded arguments, verified directly
// against the fakes rather than merely checking for no error.
func TestRunTagSetHasNoEffectOnRestoreCalls(t *testing.T) {
	vm := &fakeVM{stateVal: "running"}
	r := &fakeRestic{}
	runs := &fakeRuns{}
	host := &fakeZFSHost{}
	zr := &fakeZvolRestic{dumpData: []byte("restored zvol bytes")}

	d := sampleVMRestoreDeps(t, vm, r, runs)
	d.RunTag = "vmrun:run-42"
	d.BlockDisks = []backup.VMRestoreBlockDisk{
		{
			SourceDataset: "tank/vms/win10/disk1",
			SnapshotID:    "zvolsnapid1",
			StdinPath:     "/vm-disks/tank/vms/win10/disk1@bombvault-20260816120000",
		},
		{
			SourceDataset: "tank/vms/win10/disk2",
			SnapshotID:    "zvolsnapid2",
			StdinPath:     "/vm-disks/tank/vms/win10/disk2@bombvault-20260816120000",
		},
	}
	d.ZFSHost = host
	d.ZvolRestic = zr

	if err := backup.RestoreVM(t.Context(), d); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// The file-based restic restore call — untouched by RunTag (RestorePaths
	// takes no tags parameter).
	want := "restore:/repo/vms:deadbeef12345678:/host/domains/win10"
	if !vmContains(r.log, want) {
		t.Fatalf("restic restore call = %v, want a call starting %q", r.log, want)
	}

	// Both zvol disk restores must have happened, each dumping ITS OWN
	// snapshot id/path — untouched by RunTag (DumpTo takes no tags either).
	if !vmContains(zr.log, "dumpTo:/repo/vms:zvolsnapid1:/vm-disks/tank/vms/win10/disk1@bombvault-20260816120000") {
		t.Fatalf("disk1 restic dump not invoked: %v", zr.log)
	}
	if !vmContains(zr.log, "dumpTo:/repo/vms:zvolsnapid2:/vm-disks/tank/vms/win10/disk2@bombvault-20260816120000") {
		t.Fatalf("disk2 restic dump not invoked: %v", zr.log)
	}
	if len(runs.finishes) != 1 || runs.finishes[0] != "success" {
		t.Fatalf("run finishes = %v, want [success]", runs.finishes)
	}
}

// TestRestoreVMBlockDiskRestoreBaseDatasetReachesZFSReceiveTarget is the
// end-to-end wiring proof for the cross-instance zvol restore fix: a
// VMRestoreBlockDisk carrying a RestoreBaseDataset (what
// internal/api/service.go's prepareRestoreVMForTarget sets after rebasing the
// source dataset's pool onto an explicit destination pool via
// virshcli.RebaseZvolDatasetPool) must have that value — not SourceDataset —
// actually reach the fake ZFS host's StreamReceive target through the full
// RestoreVM -> restoreBlockDisksAndLog -> RestoreZvolDisk call chain.
// Complements vm_zvol_test.go's RestoreZvolDisk-level tests by proving the
// value is actually threaded from VMRestoreDeps.BlockDisks all the way down.
func TestRestoreVMBlockDiskRestoreBaseDatasetReachesZFSReceiveTarget(t *testing.T) {
	vm := &fakeVM{stateVal: "running"}
	r := &fakeRestic{}
	runs := &fakeRuns{}
	host := &fakeZFSHost{}
	zr := &fakeZvolRestic{dumpData: []byte("restored zvol bytes")}

	d := sampleVMRestoreDeps(t, vm, r, runs)
	d.BlockDisks = []backup.VMRestoreBlockDisk{
		{
			SourceDataset:      "tank/vms/win10/disk1",
			RestoreBaseDataset: "flashpool/vms/win10/disk1", // rebased onto the DESTINATION pool
			SnapshotID:         "zvolsnapid1",
			StdinPath:          "/vm-disks/tank/vms/win10/disk1@bombvault-20260816120000",
		},
	}
	d.ZFSHost = host
	d.ZvolRestic = zr

	if err := backup.RestoreVM(t.Context(), d); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	if !strings.HasPrefix(host.streamReceiveTarget, "flashpool/vms/win10/disk1-bombvault-restore-") {
		t.Fatalf("StreamReceive target = %q, want it derived from RestoreBaseDataset (flashpool/vms/win10/disk1), not SourceDataset", host.streamReceiveTarget)
	}
	if strings.HasPrefix(host.streamReceiveTarget, "tank/") {
		t.Fatalf("StreamReceive target = %q targeted the SOURCE pool — the exact wrong-pool bug this fix closes", host.streamReceiveTarget)
	}
}
