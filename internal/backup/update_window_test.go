package backup_test

import (
	"context"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/model"
)

// With "update after successful backup" on, the container is recreated after
// the backup through the WhileDependentsStopped hook, so its dependents stay
// down until the recreate is done. Without the hook they restart right after
// the backup, and a failing hook still leaves no dependent stopped.

// runBackupWithHook backs up a running target that stops deps, with an
// optional WhileDependentsStopped hook and the given health-wait settings. The
// Docker calls land in d.log.
func runBackupWithHook(t *testing.T, d *fakeDocker, deps []backup.StopContainer, hook func(), healthWait bool, timeout time.Duration) {
	t.Helper()
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678", Bytes: 1024}}
	tpl := &fakeTemplates{readXML: "<xml/>", readOK: true}
	runs := &fakeRuns{}
	_, err := backup.BackupContainer(t.Context(), backup.BackupDeps{
		ContainerRef:           "backuptarget",
		ContainerName:          "BackupTarget",
		RepoPath:               "/repo",
		AppdataPaths:           []string{"/host/user/appdata/backuptarget"},
		StopTimeout:            30 * time.Second,
		TargetID:               "target-1",
		WasRunning:             true,
		StopContainers:         deps,
		HealthWait:             healthWait,
		HealthTimeout:          timeout,
		WhileDependentsStopped: hook,
		SnapshotTemplatesDir:   "/data/templates",
		FlashTemplatesDir:      "/boot/templates",
		Docker:                 d,
		Restic:                 r,
		Templates:              tpl,
		Runs:                   runs,
	})
	if err != nil {
		t.Fatalf("unexpected backup error: %v", err)
	}
}

// recreateTargetHook stops, removes and recreates the target the way the update
// does, logging to the same fakeDocker.
func recreateTargetHook(d *fakeDocker) func() {
	return func() {
		ctx := context.Background()
		_ = d.Stop(ctx, "backuptarget", 0)
		_ = d.Remove(ctx, "backuptarget")
		_ = d.CreateAndStart(ctx, model.Inspect{Name: "/backuptarget"}, true)
	}
}

func TestUpdateHookRecreatesBeforeDependentRestart(t *testing.T) {
	defer backup.SetHealthTimingForTest(time.Millisecond, time.Millisecond)()
	d := &fakeDocker{}
	deps := []backup.StopContainer{
		{Name: "app", WasRunning: true, Service: "app", DependsOn: []string{"db"}},
		{Name: "db", WasRunning: true, Service: "db"},
	}
	runBackupWithHook(t, d, deps, recreateTargetHook(d), true, 5*time.Second)

	initialStart := idxOf(d.log, "start:backuptarget")
	recreate := idxOf(d.log, "createAndStart:/backuptarget")
	startDB, startApp := idxOf(d.log, "start:db"), idxOf(d.log, "start:app")
	if initialStart < 0 || recreate < 0 || startDB < 0 || startApp < 0 {
		t.Fatalf("target must be restarted then recreated, and both deps restarted: %v", d.log)
	}
	if initialStart >= recreate {
		t.Fatalf("recreate must run after the target's initial restart: %v", d.log)
	}
	if recreate >= startDB || recreate >= startApp {
		t.Fatalf("dependents must restart only after the recreate: %v", d.log)
	}
	if startDB >= startApp {
		t.Fatalf("restart order must follow depends_on (db < app): %v", d.log)
	}
	if countOf(d.log, "health:db") < 1 {
		t.Fatalf("health wait must still gate the dependents: %v", d.log)
	}
	// The target is waited for twice, after the first restart and after the
	// recreate, before dependents sharing its network namespace start.
	if countOf(d.log, "waitRunning:backuptarget") != 2 {
		t.Fatalf("target must be re-waited running after the recreate: %v", d.log)
	}
}

func TestNoUpdateHookRestartsDependentsRightAfterBackup(t *testing.T) {
	d := &fakeDocker{}
	deps := []backup.StopContainer{
		{Name: "app", WasRunning: true, Service: "app", DependsOn: []string{"db"}},
		{Name: "db", WasRunning: true, Service: "db"},
	}
	runBackupWithHook(t, d, deps, nil, false, 0)

	if idxOf(d.log, "createAndStart:/backuptarget") >= 0 {
		t.Fatalf("no hook must mean no recreate: %v", d.log)
	}
	if idxOf(d.log, "start:db") < 0 || idxOf(d.log, "start:app") < 0 {
		t.Fatalf("both dependents must be restarted: %v", d.log)
	}
	if idxOf(d.log, "start:db") >= idxOf(d.log, "start:app") {
		t.Fatalf("restart order must follow depends_on (db < app): %v", d.log)
	}
	if countOf(d.log, "waitRunning:backuptarget") != 1 {
		t.Fatalf("without a hook the target is waited once: %v", d.log)
	}
}

// TestUpdateHookFailureStillRestartsDependents uses a hook that removes the
// target without recreating it.
func TestUpdateHookFailureStillRestartsDependents(t *testing.T) {
	d := &fakeDocker{}
	deps := []backup.StopContainer{
		{Name: "app", WasRunning: true, Service: "app", DependsOn: []string{"db"}},
		{Name: "db", WasRunning: true, Service: "db"},
	}
	failingUpdate := func() {
		ctx := context.Background()
		_ = d.Stop(ctx, "backuptarget", 0)
		_ = d.Remove(ctx, "backuptarget")
	}
	runBackupWithHook(t, d, deps, failingUpdate, true, 5*time.Second)

	if idxOf(d.log, "start:db") < 0 || idxOf(d.log, "start:app") < 0 {
		t.Fatalf("a failed update must never leave the dependents stopped: %v", d.log)
	}
	if idxOf(d.log, "start:db") >= idxOf(d.log, "start:app") {
		t.Fatalf("restart order must still follow depends_on (db < app): %v", d.log)
	}
}

// TestUpdateHookHealthTimeoutStillProceeds checks that the health timeout still
// applies with the hook: app starts after the recreate although db never turns
// healthy.
func TestUpdateHookHealthTimeoutStillProceeds(t *testing.T) {
	defer backup.SetHealthTimingForTest(time.Millisecond, time.Millisecond)()
	d := &fakeDocker{
		healthSeq: map[string][]model.Health{
			"db": {{HasHealthcheck: true, Healthy: false, Running: true}}, // stuck unhealthy
		},
	}
	deps := []backup.StopContainer{
		{Name: "app", WasRunning: true, Service: "app", DependsOn: []string{"db"}},
		{Name: "db", WasRunning: true, Service: "db"},
	}
	runBackupWithHook(t, d, deps, recreateTargetHook(d), true, 20*time.Millisecond)

	recreate := idxOf(d.log, "createAndStart:/backuptarget")
	startApp := idxOf(d.log, "start:app")
	if recreate < 0 || startApp < 0 {
		t.Fatalf("recreate must run and the dependent must still start after the timeout: %v", d.log)
	}
	if recreate >= startApp {
		t.Fatalf("the dependent must start only after the recreate: %v", d.log)
	}
	if countOf(d.log, "health:db") < 1 {
		t.Fatalf("db must have been polled before the timeout: %v", d.log)
	}
}
