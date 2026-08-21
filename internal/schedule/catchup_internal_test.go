package schedule

import (
	"errors"
	"sync"
	"testing"
	"time"
	_ "time/tzdata" // deterministic DST cases (CRON_TZ=Europe/Berlin) on hosts without a system tz database

	"github.com/robfig/cron/v3"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// mustCadence parses a user-facing cadence string or fails the test.
func mustCadence(t *testing.T, s string) Cadence {
	t.Helper()
	cad, err := ParseCadence(s)
	if err != nil {
		t.Fatalf("ParseCadence(%q): %v", s, err)
	}
	return cad
}

// TestCadenceLastFire pins the Prev semantics LastFire builds on top of
// robfig's Next(): the returned time is the most recent fire at or before now.
// Exact expectations are asserted in time.Local (the zone plain cadence specs
// run in), with dates far from any DST transition.
func TestCadenceLastFire(t *testing.T) {
	cases := []struct {
		name    string
		cadence string
		now     time.Time
		want    time.Time
	}{
		{
			name:    "daily, after today's fire",
			cadence: "daily 03:00",
			now:     time.Date(2026, 7, 20, 12, 0, 0, 0, time.Local),
			want:    time.Date(2026, 7, 20, 3, 0, 0, 0, time.Local),
		},
		{
			name:    "daily, before today's fire → yesterday",
			cadence: "daily 03:00",
			now:     time.Date(2026, 7, 20, 2, 59, 0, 0, time.Local),
			want:    time.Date(2026, 7, 19, 3, 0, 0, 0, time.Local),
		},
		{
			name:    "daily, exactly at the fire counts as fired",
			cadence: "daily 03:00",
			now:     time.Date(2026, 7, 20, 3, 0, 0, 0, time.Local),
			want:    time.Date(2026, 7, 20, 3, 0, 0, 0, time.Local),
		},
		{
			// 2026-07-20 is a Monday; the previous Sunday fire is 2026-07-19.
			name:    "weekly, mid-week → previous Sunday",
			cadence: "weekly Sun 04:30",
			now:     time.Date(2026, 7, 22, 12, 0, 0, 0, time.Local),
			want:    time.Date(2026, 7, 19, 4, 30, 0, 0, time.Local),
		},
		{
			// everyN uses a plain daily trigger spec; LastFire reflects the TRIGGER,
			// the interval-days gate lives in missedRun / the wrapped job.
			name:    "everyN trigger is daily",
			cadence: "everyN 3 05:15",
			now:     time.Date(2026, 1, 10, 22, 0, 0, 0, time.Local),
			want:    time.Date(2026, 1, 10, 5, 15, 0, 0, time.Local),
		},
		{
			name:    "raw cron hourly",
			cadence: "30 * * * *",
			now:     time.Date(2026, 7, 20, 14, 45, 0, 0, time.Local),
			want:    time.Date(2026, 7, 20, 14, 30, 0, 0, time.Local),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := mustCadence(t, c.cadence).LastFire(c.now)
			if !ok {
				t.Fatalf("LastFire(%q, %s): no fire found", c.cadence, c.now)
			}
			if !got.Equal(c.want) {
				t.Fatalf("LastFire(%q, %s) = %s, want %s", c.cadence, c.now, got, c.want)
			}
		})
	}
}

// TestCadenceLastFireDisabledAndInvalid pins the no-claim paths: off/blank
// cadences and an unparseable spec yield (zero, false).
func TestCadenceLastFireDisabledAndInvalid(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	if _, ok := (Cadence{}).LastFire(now); ok {
		t.Fatal("a disabled cadence must have no last fire")
	}
	if _, ok := (Cadence{Enabled: true, Spec: "not a cron"}).LastFire(now); ok {
		t.Fatal("an unparseable spec must have no last fire")
	}
}

// TestCadenceLastFireDSTEdges pins the definition of "most recent past fire"
// across both Europe/Berlin DST transitions of 2026 via the invariants that
// hold regardless of how robfig resolves a skipped/repeated wall-clock time:
// LastFire(now) ≤ now, and Next(LastFire(now)) > now (nothing fired between).
// The specs pin their zone with CRON_TZ so the test is deterministic on UTC CI
// runners and any developer machine alike.
func TestCadenceLastFireDSTEdges(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	cases := []struct {
		name string
		spec string
		now  time.Time
	}{
		{
			// Spring forward 2026-03-29: 02:00→03:00, 02:30 does not exist that day.
			name: "spring-forward skipped time",
			spec: "CRON_TZ=Europe/Berlin 30 2 * * *",
			now:  time.Date(2026, 3, 29, 12, 0, 0, 0, berlin),
		},
		{
			name: "spring-forward, now inside the lost hour's morning",
			spec: "CRON_TZ=Europe/Berlin 30 2 * * *",
			now:  time.Date(2026, 3, 29, 3, 5, 0, 0, berlin),
		},
		{
			// Fall back 2026-10-25: 03:00→02:00, 02:30 occurs twice.
			name: "fall-back repeated time",
			spec: "CRON_TZ=Europe/Berlin 30 2 * * *",
			now:  time.Date(2026, 10, 25, 12, 0, 0, 0, berlin),
		},
		{
			name: "weekly across the fall-back weekend",
			spec: "CRON_TZ=Europe/Berlin 15 4 * * 0",
			now:  time.Date(2026, 10, 26, 9, 0, 0, 0, berlin),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cad := Cadence{Enabled: true, Spec: c.spec}
			got, ok := cad.LastFire(c.now)
			if !ok {
				t.Fatalf("LastFire(%q, %s): no fire found", c.spec, c.now)
			}
			if got.After(c.now) {
				t.Fatalf("LastFire = %s is after now %s", got, c.now)
			}
			sched, sErr := cron.ParseStandard(c.spec)
			if sErr != nil {
				t.Fatalf("ParseStandard(%q): %v", c.spec, sErr)
			}
			if next := sched.Next(got); !next.After(c.now) {
				t.Fatalf("LastFire = %s is not the LAST fire: Next(it) = %s is still ≤ now %s", got, next, c.now)
			}
		})
	}
}

// TestMissedRun pins the catch-up gate: missed vs covered vs never-ran vs the
// everyN not-yet-due case, plus the grace window around the fire.
func TestMissedRun(t *testing.T) {
	// A fixed "now" comfortably after today's 03:00 fire, away from DST edges.
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.Local)
	fire := time.Date(2026, 7, 20, 3, 0, 0, 0, time.Local)
	daily := "daily 03:00"

	cases := []struct {
		name        string
		cadence     string
		lastSuccess time.Time
		wantMissed  bool
	}{
		{"missed: success before yesterday's fire", daily, fire.Add(-30 * time.Hour), true},
		{"missed: success just before today's fire (beyond grace)", daily, fire.Add(-11 * time.Minute), true},
		{"not missed: success within grace before the fire", daily, fire.Add(-9 * time.Minute), false},
		{"not missed: success after the fire", daily, fire.Add(30 * time.Minute), false},
		{"never ran: no catch-up", daily, time.Time{}, false},
		// everyN 3: the daily trigger fired at 03:00 today, but the domain is only
		// 2 days past its last success — the due-gate would skip, so no miss.
		{"everyN not yet due", "everyN 3 03:00", now.Add(-48 * time.Hour), false},
		// 4 days past the last success with everyN 3 → due AND fire missed.
		{"everyN due and missed", "everyN 3 03:00", now.Add(-96 * time.Hour), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, missed := missedRun(mustCadence(t, c.cadence), c.lastSuccess, now)
			if missed != c.wantMissed {
				t.Fatalf("missedRun(%q, last=%s) = %v, want %v", c.cadence, c.lastSuccess, missed, c.wantMissed)
			}
		})
	}

	// A disabled cadence never registers a miss, however stale the success.
	if _, missed := missedRun(Cadence{}, now.Add(-1000*time.Hour), now); missed {
		t.Fatal("a disabled cadence must never be missed")
	}
}

// TestCatchUpMissedTriggersBackupJob pins the scheduler-level catch-up seam: a
// domain whose last success predates its last scheduled fire is re-run through
// the SAME registered job the cron entry would fire (here: the containers loop,
// observed via the injected backup fn), while a current domain is left alone.
// Mirrors schedule_offsite_afterbulk_test.go's synchronous-entry style.
func TestCatchUpMissedTriggersBackupJob(t *testing.T) {
	var mu sync.Mutex
	var backups []string
	sc := New(
		func(name string) error {
			mu.Lock()
			backups = append(backups, name)
			mu.Unlock()
			return nil
		},
		func() ([]store.Target, error) {
			return []store.Target{{ContainerName: "plex", IncludeInSchedule: true}}, nil
		},
	)
	sc.SetFlashJob(func() error {
		mu.Lock()
		backups = append(backups, "flash")
		mu.Unlock()
		return nil
	})

	// Containers: last success two days ago → yesterday's/today's daily fire was
	// missed. Flash: fresh success → covered.
	staleLastRun := func() (time.Time, error) { return time.Now().Add(-48 * time.Hour), nil }
	freshLastRun := func() (time.Time, error) { return time.Now(), nil }
	settings := store.Settings{
		ContainersSchedule: "daily 03:00",
		FlashSchedule:      "daily 03:00",
	}
	if err := sc.ReloadWithDueChecks(settings, staleLastRun, nil, freshLastRun, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks: %v", err)
	}

	ran := sc.CatchUpMissed(time.Now())
	if len(ran) != 1 || ran[0] != "containers" {
		t.Fatalf("caught-up domains = %v, want [containers]", ran)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(backups) != 1 || backups[0] != "plex" {
		t.Fatalf("backups = %v, want the containers job to have run exactly its items", backups)
	}
}

// TestCatchUpMissedSkipsNeverRanAndErrors pins two quiet paths: a domain with
// no success EVER is not caught up (no surprise first backup at boot), and a
// failing last-run query skips the domain instead of running or panicking.
func TestCatchUpMissedSkipsNeverRanAndErrors(t *testing.T) {
	var mu sync.Mutex
	backups := 0
	sc := New(
		func(string) error { mu.Lock(); backups++; mu.Unlock(); return nil },
		func() ([]store.Target, error) {
			return []store.Target{{ContainerName: "plex", IncludeInSchedule: true}}, nil
		},
	)
	sc.SetVMJob(func(string) error { mu.Lock(); backups++; mu.Unlock(); return nil },
		func() ([]store.VMTarget, error) {
			return []store.VMTarget{{Name: "vm1", IncludeInSchedule: true}}, nil
		})

	neverRan := func() (time.Time, error) { return time.Time{}, nil }
	failing := func() (time.Time, error) { return time.Time{}, errors.New("db locked") }
	settings := store.Settings{
		ContainersSchedule: "daily 03:00",
		VMsSchedule:        "daily 03:00",
	}
	if err := sc.ReloadWithDueChecks(settings, neverRan, failing, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks: %v", err)
	}

	if ran := sc.CatchUpMissed(time.Now()); len(ran) != 0 {
		t.Fatalf("caught-up domains = %v, want none (never-ran + query error)", ran)
	}
	mu.Lock()
	defer mu.Unlock()
	if backups != 0 {
		t.Fatalf("no backup may run for a never-ran or query-failing domain, got %d", backups)
	}
}

// TestWatchdogEntryRegisteredAndFires pins the watchdog domainSpec: with
// WatchdogEnabled the scheduler registers a "watchdog" entry on the fixed
// cadence, and firing it invokes the wired watchdog fn; without the setting no
// entry exists (existing zero-value Settings tests stay watchdog-free).
func TestWatchdogEntryRegisteredAndFires(t *testing.T) {
	noTargets := func() ([]store.Target, error) { return nil, nil }
	sc := New(func(string) error { return nil }, noTargets)
	fired := 0
	sc.SetWatchdogJob(func() error { fired++; return nil })

	if err := sc.ReloadWithDueChecks(store.Settings{WatchdogEnabled: true}, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks: %v", err)
	}
	found := false
	for _, e := range sc.entries {
		if e.job == "watchdog" {
			found = true
			sc.c.Entry(e.id).WrappedJob.Run()
		}
	}
	if !found {
		t.Fatal("WatchdogEnabled must register a watchdog entry")
	}
	if fired != 1 {
		t.Fatalf("watchdog fn fired %d times, want 1", fired)
	}

	// Disabled → the reload drops the entry again.
	if err := sc.ReloadWithDueChecks(store.Settings{}, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks(off): %v", err)
	}
	for _, e := range sc.entries {
		if e.job == "watchdog" {
			t.Fatal("watchdog entry must be gone when WatchdogEnabled is off")
		}
	}
}
