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

// panicRecoveryTestService is backupTestService that also returns the store, so
// the panic tests can inspect the runs table.
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

// The engine panics after BackupContainer has opened its run, and
// BackupContainer does not defer Runs.Finish.
func TestStartBackupPanicRecordsFailedRunAndReleasesGuard(t *testing.T) {
	svc, st, eng := panicRecoveryTestService(t)
	eng.backupPanic = true

	started, err := svc.StartBackup(context.Background(), "plex")
	if err != nil || !started {
		t.Fatalf("backup should start: started=%v err=%v", started, err)
	}

	// An unrecovered panic would have crashed the test binary before this point.
	waitForBackupDone(t, svc)

	tg, err := st.GetTargetByContainer("plex")
	if err != nil {
		t.Fatalf("target row should exist (StartRun needs it): %v", err)
	}
	// recoverOperation is the outermost defer, so it closes the run after
	// batchActive is cleared. Poll the run instead of trusting the guard.
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

	if svc.BackupInProgress() {
		t.Fatal("the shared guard must be released after a recovered panic, not left stuck")
	}
	if started, _ := svc.StartBackup(context.Background(), "radarr"); !started {
		t.Fatal("a later backup must be able to start; the guard must not be stuck from the earlier panic")
	}
	waitForBackupDone(t, svc) // before t.Cleanup closes the store
}

// waitForRunTerminal polls until targetID's run leaves "running" and returns
// it. recoverOperation can close the run a moment after the guard is released.
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

func TestStartBackupAllContinuesPastPanickingItem(t *testing.T) {
	svc, st, eng := panicRecoveryTestService(t)
	// Only plex panics, matched by the tag BackupContainer sets. Arming it
	// before the batch starts keeps the fake free of races.
	eng.backupPanicTag = "container:plex"

	started, err := svc.StartBackupAll(context.Background(), []string{"plex", "radarr", "sonarr"})
	if err != nil || !started {
		t.Fatalf("batch should start: started=%v err=%v", started, err)
	}

	waitForBackupDone(t, svc)

	// backupOneForBatch recovers per item, so plex's own run is closed as failed.
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
	// sonarr comes after radarr, so the batch ran the rest of the queue and did
	// not stop one item past the panic.
	if !successFor("sonarr") {
		t.Fatalf("sonarr, queued AFTER radarr, must also have been backed up normally, got runs=%+v", runs)
	}
}

// The files domain closes its own runID through finishRestoreRun, which
// containers and vms cannot: they only know a target id and go through
// failStuckRun.
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

	// The config marker lets the snapshot listing reach the engine, as in
	// TestForeignRestoreRoute.
	if err := os.MkdirAll(filepath.Join(dir, "backups", "other"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "backups", "other", "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	sessionID, _, err := svc.OpenForeign(context.Background(), location, strings.Repeat("ab", 32), nil)
	if err != nil {
		t.Fatalf("OpenForeign: %v", err)
	}

	// The snapshot has no Paths, so runRestoreFileSet falls back to
	// RestoreInclude("/"), where restorePanic fires.
	eng.restorePanic = true
	started, err := svc.StartForeignRestore(context.Background(), sessionID, "files", "docs", "latest", true, "restore-here/docs", nil, false, "")
	if err != nil || !started {
		t.Fatalf("foreign restore should start: started=%v err=%v", started, err)
	}

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

func TestStartRestoreStackMemberPanicRecordsFailedRunAndContinues(t *testing.T) {
	d := &fakeServiceDocker{liveName: ""} // absent, so the fresh restore path runs
	eng := &fakeResticEngine{
		// Only web's snapshot panics, armed before the goroutine starts.
		restorePanicSnapshot: "aaaa1111",
		snaps: []restic.Snapshot{
			// Paths as a real backup records them.
			{ID: "aaaa1111", Tags: []string{"container:web"}, Paths: []string{"/host/user/appdata/web"}},
			{ID: "bbbb2222", Tags: []string{"container:worker"}, Paths: []string{"/host/user/appdata/worker"}},
		},
	}
	// A remote containers repo skips the local-existence probe, so the restore
	// reaches the engine instead of taking the recreate-only path. The mount
	// root stays Linux-absolute for paths.Within.
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
	settings.ContainersPath = "rest:http://127.0.0.1:8000/containers"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)
	seedStackTarget(t, st, mountRoot, "web", "app", "web", "")
	seedStackTarget(t, st, mountRoot, "worker", "app", "worker", "")

	started, err := svc.StartRestoreStack(context.Background(), "app", "local", "", true, true)
	if err != nil || !started {
		t.Fatalf("stack restore should start: started=%v err=%v", started, err)
	}

	// Without restoreStackMember's recovery the outer recoverOperation would
	// still catch the panic, but the loop would stop at web and leave its run
	// "running". The assertions below catch that.
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
		t.Fatal("a later operation must be able to start; the guard must not be stuck from the earlier panic")
	}
	waitForBackupDone(t, svc) // before t.Cleanup closes the store
}

// StartRestoreConfig runs on the caller's goroutine and RestoreConfig opens the
// run itself, so the panic path closes it through ConfigTargetID. It releases
// batchActive too, because a successful restore can keep that held.
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
	eng.restorePanic = false
	if started, _, err := svc.StartRestoreConfig(context.Background(), "", "local"); err != nil || !started {
		t.Fatalf("a later config restore must be able to start; the guard must not be stuck from the earlier panic: started=%v err=%v", started, err)
	}
}

// panickingHostShell panics instead of running the command. The test uses it
// as the post-hook, which runs while BackupEverything's parent run is open, so
// only failStuckRun can close that run.
type panickingHostShell struct{}

var _ api.HostShell = panickingHostShell{}

func (panickingHostShell) Run(_ context.Context, cmd string) error {
	panic("boom: host shell exploded on " + cmd)
}

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
	svc.SetHostShell(&everythingFakeHostShell{})
	if started, err := svc.StartBackupEverything(context.Background()); err != nil || !started {
		t.Fatalf("a later pass must be able to start; the guard must not be stuck from the earlier panic: started=%v err=%v", started, err)
	}
	waitForEverythingDone(t, svc)
}
