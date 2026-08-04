package backup_test

import (
	"errors"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/model"
)

// The health-gated ordered restart (#119) governs ONLY the restart-after-backup
// phase of the "stop other containers during backup" feature: the containers we
// stopped are brought back in compose depends_on order, and — when the health
// wait is on — each is waited-for-healthy before the containers that depend on it
// are started. These tests drive BackupContainer end to end with the DI fakes
// (no docker.sock) and assert on the fakeDocker call log.

// runHealthRestartBackup runs a backup of a running target whose stopped-set is
// deps, with the given health-wait config, and returns the fakeDocker call log.
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

// A three-tier stack (db <- app <- web) whose containers were stopped in a
// SCRAMBLED input order must be restarted in compose depends_on order:
// dependencies first (db, then app, then web). With the health wait DISABLED the
// ordering is STILL applied (it is strictly safer than unordered) but NO health
// poll happens — the "orders but does not wait" contract.
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
	// Health wait is off: not a single health poll may happen.
	if countOf(d.log, "health:db")+countOf(d.log, "health:app")+countOf(d.log, "health:web") != 0 {
		t.Fatalf("health wait disabled must not poll health: %v", d.log)
	}
}

// With the health wait ENABLED, a dependency with a Docker healthcheck must be
// waited-for-healthy BEFORE the container that depends on it is started. db turns
// healthy only after one unhealthy poll; app depends_on db. Required order:
// start:db -> health:db(...) -> start:app, with app never started before db is
// healthy.
func TestRestartWaitsForHealthyBeforeDependents(t *testing.T) {
	defer backup.SetHealthTimingForTest(time.Millisecond, time.Millisecond)()
	d := &fakeDocker{
		healthSeq: map[string][]model.Health{
			// unhealthy on the first poll, healthy on the second (and thereafter).
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
	// db must have been polled for health between its start and app's start, and it
	// took more than one poll (it was unhealthy first) — proving the wait blocked.
	if countOf(d.log, "health:db") < 2 {
		t.Fatalf("db must be polled until healthy (>=2 polls): %v", d.log)
	}
	firstHealth := idxOf(d.log, "health:db")
	if startDB >= firstHealth || firstHealth >= startApp {
		t.Fatalf("order must be start:db -> health:db -> start:app: %v", d.log)
	}
}

// A dependency with NO healthcheck is waited on differently: the restart waits
// for it to report Running and then a short grace passes, before its dependents
// start. db has no healthcheck and is not Running on the first poll; app
// depends_on db.
func TestRestartWaitsRunningPlusGraceWhenNoHealthcheck(t *testing.T) {
	defer backup.SetHealthTimingForTest(time.Millisecond, time.Millisecond)()
	d := &fakeDocker{
		healthSeq: map[string][]model.Health{
			// no healthcheck: not running on the first poll, running on the second.
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
	// db must have been polled until Running (>=2 polls: not-running then running).
	if countOf(d.log, "health:db") < 2 {
		t.Fatalf("a no-healthcheck dep must be polled until Running (>=2 polls): %v", d.log)
	}
}

// The per-container wait must be bounded: a dependency that never becomes healthy
// must NOT hang the backup flow. After the timeout the restart logs a warning and
// starts the dependents anyway. db is stuck unhealthy forever; app depends_on db.
func TestRestartHealthTimeoutProceeds(t *testing.T) {
	defer backup.SetHealthTimingForTest(time.Millisecond, time.Millisecond)()
	d := &fakeDocker{
		healthSeq: map[string][]model.Health{
			// permanently unhealthy (a single entry sticks for every poll).
			"db": {{HasHealthcheck: true, Healthy: false, Running: true}},
		},
	}
	deps := []backup.StopContainer{
		{Name: "app", WasRunning: true, Service: "app", DependsOn: []string{"db"}},
		{Name: "db", WasRunning: true, Service: "db"},
	}
	// Tiny timeout so the test proves "proceeds after timeout" quickly.
	runHealthRestartBackup(t, d, deps, true, 20*time.Millisecond)

	// Despite db never turning healthy, its dependent must STILL be restarted — the
	// containers were running before the backup and must come back (best-effort).
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

// An inspect error during the health wait must degrade gracefully: the restart
// stops waiting on the container it cannot see and proceeds, rather than blocking
// the whole run. db's Health inspect always errors; app depends_on db.
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
	// The wait must have given up after the error (a single poll), not looped.
	if countOf(d.log, "health:db") != 1 {
		t.Fatalf("an inspect error must abandon the wait after one poll, got %d: %v", countOf(d.log, "health:db"), d.log)
	}
}

// A dependency that we did NOT actually stop (it was already stopped at backup
// time, #33) must be neither restarted nor waited on, even with a dependent that
// names it — the restart must be safe when a named dependency was not part of the
// set we stopped.
func TestRestartSkipsDependencyWeDidNotStop(t *testing.T) {
	defer backup.SetHealthTimingForTest(time.Millisecond, time.Millisecond)()
	d := &fakeDocker{}
	deps := []backup.StopContainer{
		{Name: "app", WasRunning: true, Service: "app", DependsOn: []string{"db"}},
		{Name: "db", WasRunning: false, Service: "db"}, // already stopped → never touched
	}
	runHealthRestartBackup(t, d, deps, true, 5*time.Second)

	if idxOf(d.log, "start:app") < 0 {
		t.Fatalf("the running dependent must still be restarted: %v", d.log)
	}
	if idxOf(d.log, "stop:db") >= 0 || idxOf(d.log, "start:db") >= 0 {
		t.Fatalf("an already-stopped dependency must never be stopped or started: %v", d.log)
	}
	// db was not in the set we stopped, so it must not be health-polled either.
	if countOf(d.log, "health:db") != 0 {
		t.Fatalf("a dependency we did not stop must not be health-polled: %v", d.log)
	}
}
