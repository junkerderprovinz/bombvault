package backup_test

import (
	"context"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/model"
)

// These tests cover the #119 follow-up (bostafari): when "update after successful
// backup" is enabled, the container is RECREATED after the backup. That recreate
// is handed to the orchestrator as the WhileDependentsStopped hook, so it runs
// while the stopped dependents are STILL down — the dependents are restarted only
// AFTER the recreate. With no hook, the dependents restart right after the backup
// (Part B, unchanged). The hook is best-effort: even if it "fails", the dependents
// are never left stopped.

// runBackupWithHook drives BackupContainer of a running target whose stopped-set
// is deps, with an optional WhileDependentsStopped hook and health-wait config,
// and returns the fakeDocker call log.
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

// recreateTargetHook returns a hook that mimics the update-after-backup recreate:
// stop, remove and recreate+start the target, exactly what breaks a dependent that
// came back too early. It records those calls in the same fakeDocker log so the
// test can assert the dependents restart only afterwards.
func recreateTargetHook(d *fakeDocker) func() {
	return func() {
		ctx := context.Background()
		_ = d.Stop(ctx, "backuptarget", 0)
		_ = d.Remove(ctx, "backuptarget")
		_ = d.CreateAndStart(ctx, model.Inspect{Name: "/backuptarget"}, true)
	}
}

// With the update hook set, the target's recreate must happen while the dependents
// are still down, and the dependents must be restarted (health-gated, in
// depends_on order) only AFTER the recreate completes.
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
	// The recreate runs after the target's initial post-backup restart ...
	if initialStart >= recreate {
		t.Fatalf("recreate must run after the target's initial restart: %v", d.log)
	}
	// ... and BOTH dependents come back only AFTER the recreate (the bug: they used
	// to be up already and broke against the torn-down target).
	if recreate >= startDB || recreate >= startApp {
		t.Fatalf("dependents must restart only after the recreate: %v", d.log)
	}
	// depends_on order still holds, health-gated.
	if startDB >= startApp {
		t.Fatalf("restart order must follow depends_on (db < app): %v", d.log)
	}
	if countOf(d.log, "health:db") < 1 {
		t.Fatalf("health wait must still gate the dependents: %v", d.log)
	}
	// The target must be re-waited for Running after the recreate, before its netns
	// dependents start: two waitRunning on the target (initial + post-recreate).
	if countOf(d.log, "waitRunning:backuptarget") != 2 {
		t.Fatalf("target must be re-waited running after the recreate: %v", d.log)
	}
}

// With NO update hook, the dependents restart right after the backup, exactly as
// Part B does today: the target is restarted and waited-for once (no recreate) and
// the dependents come back in order.
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
	// The target is waited-for-running exactly once (no post-recreate re-wait).
	if countOf(d.log, "waitRunning:backuptarget") != 1 {
		t.Fatalf("without a hook the target is waited once: %v", d.log)
	}
}

// If the update "fails" (its recreate leaves the target stopped: stop+remove but no
// successful create), the dependents must STILL be restarted — never orphaned
// stopped. The hook here does not recreate the target.
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
		// recreate failed: the target is left down, but the dependents must still come back.
	}
	runBackupWithHook(t, d, deps, failingUpdate, true, 5*time.Second)

	if idxOf(d.log, "start:db") < 0 || idxOf(d.log, "start:app") < 0 {
		t.Fatalf("a failed update must never leave the dependents stopped: %v", d.log)
	}
	if idxOf(d.log, "start:db") >= idxOf(d.log, "start:app") {
		t.Fatalf("restart order must still follow depends_on (db < app): %v", d.log)
	}
}

// The per-container health timeout must still bound the wait when the update hook
// is present: a dependency that never turns healthy must not hang the flow, and its
// dependent must still be restarted after the timeout — and after the recreate.
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
