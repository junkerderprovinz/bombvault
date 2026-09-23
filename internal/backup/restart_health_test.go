package backup_test

import (
	"errors"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/model"
)

// These tests cover the restart after a backup that stopped other containers:
// the stopped containers come back in compose depends_on order, and with the
// health wait on, each must be healthy before its dependents start.

// runHealthRestartBackup backs up a running target that stops deps, with the
// given health-wait settings. The Docker calls land in d.log.
func runHealthRestartBackup(t *testing.T, d *fakeDocker, deps []backup.StopContainer, healthWait bool, timeout time.Duration) {
	t.Helper()
	r := &fakeRestic{summary: backup.Summary{SnapshotID: "deadbeef12345678", Bytes: 1024}}
	tpl := &fakeTemplates{readXML: "<xml/>", readOK: true}
	runs := &fakeRuns{}
	_, err := backup.BackupContainer(t.Context(), backup.BackupDeps{
		ContainerRef:         "backuptarget",
		ContainerName:        "BackupTarget",
		RepoPath:             "/repo",
		AppdataPaths:         []string{"/host/user/appdata/backuptarget"},
		StopTimeout:          30 * time.Second,
		TargetID:             "target-1",
		WasRunning:           true,
		StopContainers:       deps,
		HealthWait:           healthWait,
		HealthTimeout:        timeout,
		SnapshotTemplatesDir: "/data/templates",
		FlashTemplatesDir:    "/boot/templates",
		Docker:               d,
		Restic:               r,
		Templates:            tpl,
		Runs:                 runs,
	})
	if err != nil {
		t.Fatalf("unexpected backup error: %v", err)
	}
}

// idxOf returns the index of the first entry equal to s in log, or -1.
func idxOf(log []string, s string) int {
	for i, e := range log {
		if e == s {
			return i
		}
	}
	return -1
}

// countOf returns how many entries in log equal s.
func countOf(log []string, s string) int {
	n := 0
	for _, e := range log {
		if e == s {
			n++
		}
	}
	return n
}

func TestRestartOrdersByDependsOnWithoutWaitingWhenDisabled(t *testing.T) {
	d := &fakeDocker{}
	deps := []backup.StopContainer{
		{Name: "web", WasRunning: true, Service: "web", DependsOn: []string{"app"}},
		{Name: "app", WasRunning: true, Service: "app", DependsOn: []string{"db"}},
		{Name: "db", WasRunning: true, Service: "db"},
	}
	runHealthRestartBackup(t, d, deps, false, 0)

	db, app, web := idxOf(d.log, "start:db"), idxOf(d.log, "start:app"), idxOf(d.log, "start:web")
	if db < 0 || app < 0 || web < 0 {
		t.Fatalf("every stopped dependency must be restarted: %v", d.log)
	}
	if db >= app || app >= web {
		t.Fatalf("restart order must follow depends_on (db < app < web): %v", d.log)
	}
	if countOf(d.log, "health:db")+countOf(d.log, "health:app")+countOf(d.log, "health:web") != 0 {
		t.Fatalf("health wait disabled must not poll health: %v", d.log)
	}
}

// TestRestartWaitsForHealthyBeforeDependents has db turn healthy on the second
// poll; app depends on db and must not start before that.
func TestRestartWaitsForHealthyBeforeDependents(t *testing.T) {
	defer backup.SetHealthTimingForTest(time.Millisecond, time.Millisecond)()
	d := &fakeDocker{
		healthSeq: map[string][]model.Health{
			// unhealthy on the first poll, healthy from the second on
			"db": {
				{HasHealthcheck: true, Healthy: false, Running: true},
				{HasHealthcheck: true, Healthy: true, Running: true},
			},
		},
	}
	deps := []backup.StopContainer{
		{Name: "app", WasRunning: true, Service: "app", DependsOn: []string{"db"}},
		{Name: "db", WasRunning: true, Service: "db"},
	}
	runHealthRestartBackup(t, d, deps, true, 5*time.Second)

	startDB, startApp := idxOf(d.log, "start:db"), idxOf(d.log, "start:app")
	if startDB < 0 || startApp < 0 {
		t.Fatalf("both dependencies must be restarted: %v", d.log)
	}
	if startDB > startApp {
		t.Fatalf("dependency db must start before its dependent app: %v", d.log)
	}
	// A second poll shows the wait blocked on the unhealthy first answer.
	if countOf(d.log, "health:db") < 2 {
		t.Fatalf("db must be polled until healthy (>=2 polls): %v", d.log)
	}
	firstHealth := idxOf(d.log, "health:db")
	if startDB >= firstHealth || firstHealth >= startApp {
		t.Fatalf("order must be start:db -> health:db -> start:app: %v", d.log)
	}
}

// TestRestartWaitsRunningPlusGraceWhenNoHealthcheck checks that a dependency
// without a healthcheck must be running, plus a short grace, before its
// dependents start.
func TestRestartWaitsRunningPlusGraceWhenNoHealthcheck(t *testing.T) {
	defer backup.SetHealthTimingForTest(time.Millisecond, time.Millisecond)()
	d := &fakeDocker{
		healthSeq: map[string][]model.Health{
			// not running on the first poll, running on the second
			"db": {
				{HasHealthcheck: false, Running: false},
				{HasHealthcheck: false, Running: true},
			},
		},
	}
	deps := []backup.StopContainer{
		{Name: "app", WasRunning: true, Service: "app", DependsOn: []string{"db"}},
		{Name: "db", WasRunning: true, Service: "db"},
	}
	runHealthRestartBackup(t, d, deps, true, 5*time.Second)

	startDB, startApp := idxOf(d.log, "start:db"), idxOf(d.log, "start:app")
	if startDB < 0 || startApp < 0 {
		t.Fatalf("both dependencies must be restarted: %v", d.log)
	}
	if startDB >= startApp {
		t.Fatalf("db must start before its dependent app: %v", d.log)
	}
	if countOf(d.log, "health:db") < 2 {
		t.Fatalf("a no-healthcheck dep must be polled until Running (>=2 polls): %v", d.log)
	}
}

// TestRestartHealthTimeoutProceeds checks that a dependency that never turns
// healthy does not hang the backup: after the timeout its dependents start
// anyway.
func TestRestartHealthTimeoutProceeds(t *testing.T) {
	defer backup.SetHealthTimingForTest(time.Millisecond, time.Millisecond)()
	d := &fakeDocker{
		healthSeq: map[string][]model.Health{
			// a single entry answers every poll
			"db": {{HasHealthcheck: true, Healthy: false, Running: true}},
		},
	}
	deps := []backup.StopContainer{
		{Name: "app", WasRunning: true, Service: "app", DependsOn: []string{"db"}},
		{Name: "db", WasRunning: true, Service: "db"},
	}
	runHealthRestartBackup(t, d, deps, true, 20*time.Millisecond)

	if idxOf(d.log, "start:app") < 0 {
		t.Fatalf("dependent must still start after the wait times out (never hang): %v", d.log)
	}
	if idxOf(d.log, "start:db") < 0 {
		t.Fatalf("db must still be restarted: %v", d.log)
	}
	if countOf(d.log, "health:db") < 1 {
		t.Fatalf("db must have been polled at least once before the timeout: %v", d.log)
	}
}

// TestRestartHealthInspectErrorDegrades checks that when inspecting db fails,
// the restart stops waiting for it and goes on.
func TestRestartHealthInspectErrorDegrades(t *testing.T) {
	defer backup.SetHealthTimingForTest(time.Millisecond, time.Millisecond)()
	d := &fakeDocker{
		healthErr: map[string]error{"db": errors.New("no such container")},
	}
	deps := []backup.StopContainer{
		{Name: "app", WasRunning: true, Service: "app", DependsOn: []string{"db"}},
		{Name: "db", WasRunning: true, Service: "db"},
	}
	runHealthRestartBackup(t, d, deps, true, 5*time.Second)

	if idxOf(d.log, "start:app") < 0 || idxOf(d.log, "start:db") < 0 {
		t.Fatalf("an inspect error must not stop the restart: both must start: %v", d.log)
	}
	if countOf(d.log, "health:db") != 1 {
		t.Fatalf("an inspect error must abandon the wait after one poll, got %d: %v", countOf(d.log, "health:db"), d.log)
	}
}

// TestRestartSkipsDependencyWeDidNotStop checks that a dependency that was
// already stopped at backup time is neither started nor waited on, even when
// a dependent names it.
func TestRestartSkipsDependencyWeDidNotStop(t *testing.T) {
	defer backup.SetHealthTimingForTest(time.Millisecond, time.Millisecond)()
	d := &fakeDocker{}
	deps := []backup.StopContainer{
		{Name: "app", WasRunning: true, Service: "app", DependsOn: []string{"db"}},
		{Name: "db", WasRunning: false, Service: "db"},
	}
	runHealthRestartBackup(t, d, deps, true, 5*time.Second)

	if idxOf(d.log, "start:app") < 0 {
		t.Fatalf("the running dependent must still be restarted: %v", d.log)
	}
	if idxOf(d.log, "stop:db") >= 0 || idxOf(d.log, "start:db") >= 0 {
		t.Fatalf("an already-stopped dependency must never be stopped or started: %v", d.log)
	}
	if countOf(d.log, "health:db") != 0 {
		t.Fatalf("a dependency we did not stop must not be health-polled: %v", d.log)
	}
}
