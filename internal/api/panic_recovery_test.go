package api_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// panicRecoveryTestService is backupTestService plus the underlying *store.Repo
// (backupTestService's 4-value signature is used positionally by many existing
// tests, so it isn't extended here — this is a separate, otherwise-identical
// setup for the panic-recovery tests below, which need to inspect the runs
// table directly).
func panicRecoveryTestService(t *testing.T) (*api.Service, *store.Repo, *fakeResticEngine) {
	t.Helper()
	dir := t.TempDir()
	root := strings.ReplaceAll(dir, "\\", "/")
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: root}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"plex", "radarr", "sonarr"} {
		if err := os.MkdirAll(root+"/appdata/"+n, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	d := &fakeServiceDocker{inspect: model.Inspect{
		Name: "/app", Image: "app:latest", Running: true,
	}}
	eng := &fakeResticEngine{}
	prog := progress.NewStore()
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)
	svc.SetProgress(prog)
	return svc, st, eng
}

// TestStartBackupPanicRecordsFailedRunAndReleasesGuard pins the fix for the
// most severe finding of the code-review audit: every StartBackup/StartRestore/
// StartReplicateOffsite goroutine in internal/api/service.go used to run with
// zero panic recovery, so a panic anywhere in the shared backup/restore
// orchestrator (an unusual Docker inspect shape, a nil dereference, ...) took
// down the ENTIRE bombvault process — the HTTP server, the SSE progress
// stream, and every other domain's concurrently in-flight backup/restore with
// it. This test makes the underlying restic engine panic mid-backup (the
// panic is injected AFTER backup.BackupContainer already called
// Runs.Start, but its matching Runs.Finish is a plain, non-deferred call — so
// without the fix the run would also be left stuck "running" forever, even on
// a build where the process itself somehow survived).
//
// It proves BOTH halves of the fix:
//  1. The goroutine's panic is recovered — this test function returning at all
//     (instead of the whole `go test` binary crashing) already proves that,
//     and waitForBackupDone additionally proves the shared single-flight guard
//     was correctly released afterwards, not left stuck.
//  2. The run record is closed out as "failed" (store.FailRunningRun via
//     failStuckRun), not left stuck "running" — the story internal/store's
//     TestListRunsWithRunningRun (package store, not this file's package)
//     exists to warn about.
func TestStartBackupPanicRecordsFailedRunAndReleasesGuard(t *testing.T) {
	svc, st, eng := panicRecoveryTestService(t)
	eng.backupPanic = true

	started, err := svc.StartBackup(context.Background(), "plex")
	if err != nil || !started {
		t.Fatalf("backup should start: started=%v err=%v", started, err)
	}

	// If the panic were NOT recovered, this whole test binary would have
	// already crashed by now. Reaching this line at all is part of the proof.
	waitForBackupDone(t, svc)

	tg, err := st.GetTargetByContainer("plex")
	if err != nil {
		t.Fatalf("target row should exist (StartRun needs it): %v", err)
	}
	// recoverOperation is deferred FIRST in the goroutine (so it runs LAST,
	// after batchActive.Store(false) — see its doc comment for why: it must be
	// the outermost defer to catch a panic from anywhere, including from other
	// cleanup defers). That means the run can still be transitioning to
	// "failed" for a moment AFTER waitForBackupDone (which only watches
	// batchActive) already returned — so the run's OWN terminal state is
	// polled here directly, rather than assumed from the guard alone.
	run := waitForRunTerminal(t, st, tg.ID)
	if run.Status != "failed" {
		t.Fatalf("a panicked backup must record a FAILED run, not left stuck, got status=%q run=%+v", run.Status, run)
	}
	if run.FinishedAt == nil {
		t.Fatalf("a panicked backup's run must be closed (finished_at set), got %+v", run)
	}
	if !strings.Contains(run.Error, "recovered panic") {
		t.Fatalf("the failed run should carry the panic detail, got error=%q", run.Error)
	}

	// The shared single-flight guard must be released — the classic symptom of
	// an unrecovered goroutine crash (before the process itself dies) is every
	// subsequent backup/restore permanently refused as "busy".
	if svc.BackupInProgress() {
		t.Fatal("the shared guard must be released after a recovered panic, not left stuck")
	}
	if started, _ := svc.StartBackup(context.Background(), "radarr"); !started {
		t.Fatal("a later backup must be able to start — the guard must not be stuck from the earlier panic")
	}
	waitForBackupDone(t, svc) // drain it before the test's t.Cleanup closes the store out from under it
}

// waitForRunTerminal polls the runs table until targetID's run leaves
// "running" (or fails the test after a timeout), returning it. Needed
// because recoverOperation's failStuckRun/finishRestoreRun call can complete
// a moment after batchActive.Store(false) already fired (see
// TestStartBackupPanicRecordsFailedRunAndReleasesGuard for why) — asserting
// on the run's own terminal state must not rely on the guard's timing alone.
func waitForRunTerminal(t *testing.T, st *store.Repo, targetID string) store.Run {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		runs, err := st.ListRuns(20)
		if err != nil {
			t.Fatalf("ListRuns: %v", err)
		}
		for _, r := range runs {
			if r.TargetID == targetID && r.Status != "running" {
				return r
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for target %q's run to leave 'running'", targetID)
	return store.Run{}
}

// TestStartBackupAllContinuesPastPanickingItem pins the batch-specific half of
// the fix: StartBackupAll already treats one container's ordinary ERROR as
// "log it, count it, keep going" (the loop never aborted for a normal error
// before this fix, and must not start doing so now that a PANIC is also
// contained). Without per-item recovery, one container's panic would abort
// the whole batch's goroutine, silently skipping every container still queued
// behind it — a regression this test would catch.
func TestStartBackupAllContinuesPastPanickingItem(t *testing.T) {
	svc, st, eng := panicRecoveryTestService(t)
	// Only "plex" panics (matched by its "container:plex" restic tag, set by
	// backup.BackupContainer) — armed BEFORE the batch goroutine launches, so
	// there's no race on the fake between the test goroutine and the batch's.
	eng.backupPanicTag = "container:plex"

	started, err := svc.StartBackupAll(context.Background(), []string{"plex", "radarr", "sonarr"})
	if err != nil || !started {
		t.Fatalf("batch should start: started=%v err=%v", started, err)
	}

	waitForBackupDone(t, svc)

	// plex (the panicking item) must itself be recorded FAILED with a
	// recognizable "recovered panic" marker — backupOneForBatch's own recovery,
	// not just "the batch as a whole survived". Without per-item recovery this
	// run would be left stuck "running" forever (see failStuckRun's doc comment).
	tgPlex, err := st.GetTargetByContainer("plex")
	if err != nil {
		t.Fatalf("plex target should exist: %v", err)
	}
	plexRun := waitForRunTerminal(t, st, tgPlex.ID)
	if plexRun.Status != "failed" {
		t.Fatalf("plex (the panicking item) must record a FAILED run, not left stuck, got status=%q run=%+v", plexRun.Status, plexRun)
	}
	if !strings.Contains(plexRun.Error, "recovered panic") {
		t.Fatalf("plex's failed run should carry the panic detail, got error=%q", plexRun.Error)
	}

	runs, err := st.ListRuns(20)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	successFor := func(containerName string) bool {
		tg, tErr := st.GetTargetByContainer(containerName)
		if tErr != nil {
			t.Fatalf("%s target should exist: %v", containerName, tErr)
		}
		for _, r := range runs {
			if r.TargetID == tg.ID && r.Status == "success" {
				return true
			}
		}
		return false
	}
	if !successFor("radarr") {
		t.Fatalf("radarr, queued AFTER the panicking container, must still have been backed up (batch must continue past a panic like it already does past an error), got runs=%+v", runs)
	}
	// sonarr is queued AFTER radarr — proving the batch didn't just survive one
	// item past the panic and then quietly stop, but ran the WHOLE remaining queue.
	if !successFor("sonarr") {
		t.Fatalf("sonarr, queued AFTER radarr, must also have been backed up normally, got runs=%+v", runs)
	}
}

// TestStartForeignRestorePanicRecordsFailedRunAndReleasesGuard extends the
// panic-recovery proof to StartForeignRestore (internal/api/foreign.go) — one
// of the two goroutines a spec-compliance review found the f5b3286 sweep
// missed (its "go func" grep covered only service.go). The files domain is
// used here because it exercises the run-tracking half of
// prepareForeignRestore's two onPanic strategies: it drives its own local
// runID (finishRestoreRun), unlike the containers/vms branches which only
// know a target id (failStuckRun) — see prepareForeignRestore's own doc
// comment for why the two differ.
func TestStartForeignRestorePanicRecordsFailedRunAndReleasesGuard(t *testing.T) {
	enc := true
	location := "backups/other"
	eng := &fakeResticEngine{
		existingMode: &enc, // the foreign repo "exists" and opens with the encrypted probe
		snaps: []restic.Snapshot{
			{ID: "eeeeeeee55555555", Time: "2026-07-05T10:00:00Z", Tags: []string{"fileset:docs"}},
		},
	}
	_, st, svc, dir := newTestRouterSvcDir(t, &fakeServiceDocker{}, eng)

	// Seed the foreign repo's config marker so the session's snapshot listing
	// reaches the engine — the identical setup TestForeignRestoreRoute uses.
	if err := os.MkdirAll(filepath.Join(dir, "backups", "other"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "backups", "other", "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	sessionID, _, err := svc.OpenForeign(context.Background(), location, strings.Repeat("ab", 32))
	if err != nil {
		t.Fatalf("OpenForeign: %v", err)
	}

	// The snapshot above carries no Paths, so runRestoreFileSet's degenerate
	// fallback calls RestoreInclude("/") — exactly the call shape
	// TestForeignRestoreRoute already pins — so restorePanic reaches it.
	eng.restorePanic = true
	started, err := svc.StartForeignRestore(context.Background(), sessionID, "files", "docs", "latest", true, "restore-here/docs", nil, false, "")
	if err != nil || !started {
		t.Fatalf("foreign restore should start: started=%v err=%v", started, err)
	}

	// If the panic were NOT recovered, this whole test binary would have already
	// crashed by now. Reaching this line at all is part of the proof.
	waitForBackupDone(t, svc)

	set, err := st.GetFileSetByName("docs")
	if err != nil {
		t.Fatalf("the foreign file set should have been adopted locally: %v", err)
	}
	run := waitForRunTerminal(t, st, set.ID)
	if run.Status != "failed" {
		t.Fatalf("a panicked foreign restore must record a FAILED run, not left stuck, got status=%q run=%+v", run.Status, run)
	}
	if run.FinishedAt == nil {
		t.Fatalf("a panicked foreign restore's run must be closed (finished_at set), got %+v", run)
	}
	if !strings.Contains(run.Error, "recovered panic") {
		t.Fatalf("the failed run should carry the panic detail, got error=%q", run.Error)
	}

	if svc.BackupInProgress() {
		t.Fatal("the shared guard must be released after a recovered panic, not left stuck")
	}
}

// TestStartRestoreStackMemberPanicRecordsFailedRunAndContinues extends the
// panic-recovery proof to StartRestoreStack (internal/api/stacks.go) — the
// other goroutine the f5b3286 sweep missed. Unlike the single-target starters,
// a stack restore's per-member loop needed its OWN per-item recovery
// (restoreStackMember), mirroring backupOneForBatch/backupFileSetOneForBatch:
// one member's panic must count as that member failing — exactly like a
// normal per-member error already does — and must not abort the members
// still queued behind it.
func TestStartRestoreStackMemberPanicRecordsFailedRunAndContinues(t *testing.T) {
	d := &fakeServiceDocker{liveName: ""} // absent → fresh restore path
	eng := &fakeResticEngine{
		// Only "web"'s snapshot panics — armed BEFORE the goroutine launches, so
		// there's no race on the fake between the test goroutine and the stack's
		// (mirrors backupPanicTag's own discipline).
		restorePanicSnapshot: "aaaa1111",
		snaps: []restic.Snapshot{
			{ID: "aaaa1111", Tags: []string{"container:web"}},
			{ID: "bbbb2222", Tags: []string{"container:worker"}},
		},
	}
	// A REMOTE containers repo skips the local-existence probe so the restore
	// actually reaches the engine (and restorePanicSnapshot) instead of silently
	// taking the recreate-only path — the same reason
	// TestStartRestoreStackSingleFlight/TestRestoreStackCancelledMemberAbortsLoop
	// use a rest: URL. The mount root stays Linux-absolute for paths.Within.
	dir := t.TempDir()
	const mountRoot = "/host/user"
	cfg := config.Config{
		AppKey:            strings.Repeat("a", 64),
		DataDir:           dir,
		HostMountRoot:     mountRoot,
		FlashTemplatesDir: filepath.Join(dir, "flash"),
	}
	st := newMemStore(t)
	settings := mustSettings(t, st)
	settings.EncryptionEnabled = false
	settings.ContainersPath = "rest:http://127.0.0.1:8000/containers" // remote → no local-repo probe
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)
	seedStackTarget(t, st, mountRoot, "web", "app", "web", "")
	seedStackTarget(t, st, mountRoot, "worker", "app", "worker", "")

	started, err := svc.StartRestoreStack(context.Background(), "app", "local", true, true)
	if err != nil || !started {
		t.Fatalf("stack restore should start: started=%v err=%v", started, err)
	}

	// Unlike the foreign-restore test above, removing restoreStackMember's own
	// per-member recovery would NOT crash this test binary: the outer
	// StartRestoreStack goroutine's own recoverOperation (deferred with a nil
	// onPanic, see its "go func()" above) would still catch the panic one level
	// up, since recover works for any panic still unwinding within the same
	// goroutine, not just the closest defer. What it would actually break is the
	// batch behaviour asserted below — the member loop would abort at "web"
	// instead of continuing to "worker", and web's run would be left stuck
	// "running" forever (the outer recovery has no onPanic to close it out)
	// instead of recorded failed.
	waitForBackupDone(t, svc)

	webTg, err := st.GetTargetByContainer("web")
	if err != nil {
		t.Fatalf("web target should exist: %v", err)
	}
	webRun := waitForRunTerminal(t, st, webTg.ID)
	if webRun.Status != "failed" {
		t.Fatalf("web (the panicking member) must record a FAILED run, not left stuck, got status=%q run=%+v", webRun.Status, webRun)
	}
	if !strings.Contains(webRun.Error, "recovered panic") {
		t.Fatalf("web's failed run should carry the panic detail, got error=%q", webRun.Error)
	}

	workerTg, err := st.GetTargetByContainer("worker")
	if err != nil {
		t.Fatalf("worker target should exist: %v", err)
	}
	runs, err := st.ListRuns(20)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	found := false
	for _, r := range runs {
		if r.TargetID == workerTg.ID && r.Status == "success" {
			found = true
		}
	}
	if !found {
		t.Fatalf("worker, restored alongside the panicking member, must still have completed normally (one member's panic must not abort the rest of the stack), got runs=%+v", runs)
	}

	if svc.BackupInProgress() {
		t.Fatal("the shared guard must be released after a recovered panic, not left stuck")
	}
	if started, _ := svc.StartBackup(context.Background(), "web"); !started {
		t.Fatal("a later operation must be able to start — the guard must not be stuck from the earlier panic")
	}
	waitForBackupDone(t, svc) // drain it before the test's t.Cleanup closes the store out from under it
}

// TestStartRestoreConfigPanicRecordsFailedRunAndReleasesGuard extends the
// panic-recovery proof to StartRestoreConfig (the finding this branch's second
// commit fixes: it ran synchronously against the raw request ctx with NO panic
// recovery at all, unlike every sibling Start* restore). Unlike those siblings,
// StartRestoreConfig does not hand off to a background goroutine (see its own
// doc comment for why — Recovery.tsx needs the real outcome synchronously), so
// a panic here unwinds on the SAME goroutine as this test: reaching the
// assertions below at all (instead of the whole `go test` binary crashing) is
// already half the proof. The other half is that RestoreConfig's own run
// record — started via store.StartRun(store.ConfigTargetID, "restore") deep
// inside RestoreConfig, whose local runID never reaches StartRestoreConfig —
// is still closed out as "failed" via the FailRunningRun/ConfigTargetID
// fallback, not left stuck "running" forever, and that the single-flight
// batchActive guard is released too (StartRestoreConfig's own special case:
// unlike its siblings it sometimes deliberately KEEPS that guard held on
// success, so the panic path must release it explicitly).
func TestStartRestoreConfigPanicRecordsFailedRunAndReleasesGuard(t *testing.T) {
	t.Setenv("BOMBVAULT_SELF_CONTAINER", "")
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: filepath.ToSlash(dir)}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.ConfigEnabled = true
	s.ConfigPath = "backups/config"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	eng := &fakeResticEngine{snaps: []restic.Snapshot{{ID: "aaaa1111bbbb2222"}}, restorePanic: true}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	started, auto, err := svc.StartRestoreConfig(context.Background(), "", "local")
	// Reaching this line at all (instead of the test binary crashing) already
	// proves the panic was recovered.
	if err == nil || !strings.Contains(err.Error(), "recovered panic") {
		t.Fatalf("expected a recovered-panic error, got started=%v auto=%v err=%v", started, auto, err)
	}
	if started {
		t.Fatal("a panicked config restore must not report started=true")
	}

	run := waitForRunTerminal(t, st, store.ConfigTargetID)
	if run.Status != "failed" {
		t.Fatalf("a panicked config restore must record a FAILED run, not left stuck, got status=%q run=%+v", run.Status, run)
	}
	if run.FinishedAt == nil {
		t.Fatalf("a panicked config restore's run must be closed (finished_at set), got %+v", run)
	}
	if !strings.Contains(run.Error, "recovered panic") {
		t.Fatalf("the failed run should carry the panic detail, got error=%q", run.Error)
	}

	if svc.BackupInProgress() {
		t.Fatal("the shared batchActive guard must be released after a recovered panic, not left stuck")
	}
	eng.restorePanic = false // disarm — this call must succeed normally
	if started, _, err := svc.StartRestoreConfig(context.Background(), "", "local"); err != nil || !started {
		t.Fatalf("a later config restore must be able to start — the guard must not be stuck from the earlier panic: started=%v err=%v", started, err)
	}
}

// panickingHostShell is a HostShell (hostshell.go) that panics instead of
// running the command — the cheapest way to raise a panic INSIDE a "Backup
// Everything" pass at a point where the parent run row is already open. The
// post-hook is deliberately the trigger used below: it fires after
// BackupEverything's store.StartRun but before its matching FinishRun, so a
// panic there is exactly the "nothing left alive will ever close this run"
// case failStuckRun exists for.
type panickingHostShell struct{}

var _ api.HostShell = panickingHostShell{}

func (panickingHostShell) Run(_ context.Context, cmd string) error {
	panic("boom: host shell exploded on " + cmd)
}

// TestStartBackupEverythingPanicRecordsFailedRunAndReleasesGuard extends this
// file's panic-recovery guarantee to StartBackupEverything's own detached
// goroutine. It is the newest of the Start* goroutines and therefore the
// easiest one to leave out of the recoverOperation convention every sibling
// follows: without it, a panic anywhere in a pass takes down the WHOLE
// process — the HTTP server, the SSE progress stream, and every other
// domain's concurrently in-flight work — which is precisely what a
// "back up everything, then ping the dead-man's switch" job must never do.
// Reaching the assertions below at all (instead of the test binary crashing)
// is the first half of the proof; the second is that the pass's PARENT run
// row is closed out as "failed" via the failStuckRun/EverythingTargetID
// fallback rather than left stuck "running" forever, and that the
// everythingActive single-flight guard is released so a later pass can start.
func TestStartBackupEverythingPanicRecordsFailedRunAndReleasesGuard(t *testing.T) {
	svc, st, _, _ := everythingTestService(t, &fakeResticEngine{})
	s := mustSettings(t, st)
	s.EverythingPostHook = "ping-the-switch"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	svc.SetHostShell(panickingHostShell{})

	started, err := svc.StartBackupEverything(context.Background())
	if err != nil || !started {
		t.Fatalf("StartBackupEverything should have launched the pass: started=%v err=%v", started, err)
	}
	waitForEverythingDone(t, svc)

	run := waitForRunTerminal(t, st, store.EverythingTargetID)
	if run.Status != "failed" {
		t.Fatalf("a panicked pass must record a FAILED parent run, not leave it stuck, got status=%q run=%+v", run.Status, run)
	}
	if run.FinishedAt == nil {
		t.Fatalf("a panicked pass's parent run must be closed (finished_at set), got %+v", run)
	}
	if !strings.Contains(run.Error, "recovered panic") {
		t.Fatalf("the failed parent run should carry the panic detail, got error=%q", run.Error)
	}

	if svc.EverythingInProgress() {
		t.Fatal("the everythingActive guard must be released after a recovered panic, not left stuck")
	}
	// Disarm and prove the guard really is reusable: a second pass must start.
	svc.SetHostShell(&everythingFakeHostShell{})
	if started, err := svc.StartBackupEverything(context.Background()); err != nil || !started {
		t.Fatalf("a later pass must be able to start — the guard must not be stuck from the earlier panic: started=%v err=%v", started, err)
	}
	waitForEverythingDone(t, svc)
}
