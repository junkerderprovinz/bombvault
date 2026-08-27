package schedule

// ---------------------------------------------------------------------------
// NextRuns against an everyN cadence.
//
// An everyN cadence is a DAILY cron trigger plus a due-gate (ParseCadence
// compiles "everyN N HH:MM" to a bare daily spec and carries N on the side), so
// the cron entry's own Next is tomorrow on every one of the N-1 nights the gate
// is going to close. NextRuns feeds the dashboard activity log's "up next" line,
// which therefore promised a drill tomorrow when the real one was days out.
//
// These fire through the SAME registration path the scheduler uses in
// production (ReloadWithDueChecks + the job-run store), so what is pinned is the
// real chain, not a hand-built entry.
// ---------------------------------------------------------------------------

import (
	"errors"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// everyNDrillScheduler registers the drills schedule on "everyN 7 03:00" with a
// job-run store answering `last`, starts the cron runner (Entry.Next is only
// computed once it is running) and returns the scheduler plus the drill's
// reported next run.
func everyNDrillScheduler(t *testing.T, jr JobRunStore) (sc *Scheduler, drill NextRun, found bool) {
	t.Helper()
	noTargets := func() ([]store.Target, error) { return nil, nil }
	sc = New(func(string) error { return nil }, noTargets)
	sc.SetJobRunStore(jr)
	sc.SetDrillJob(func(_, _, _ string) error { return nil })

	settings := store.Settings{
		DrillsEnabled:     true,
		ContainersEnabled: true, // gives drillTasks a non-empty task list
		DrillsSchedule:    "everyN 7 03:00",
	}
	if err := sc.ReloadWithDueChecks(settings, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks: %v", err)
	}
	sc.Start()
	t.Cleanup(sc.Stop)

	for _, r := range sc.NextRuns() {
		if r.Job == "drill" {
			return sc, r, true
		}
	}
	return sc, NextRun{}, false
}

// TestNextRunsSkipsFiresTheEveryNGateWillClose is the regression. The drill pass
// ran three days ago on a 7-day interval, so the next four nightly triggers are
// all going to be skipped by the gate. "Up next" must name the fire that will
// actually run, not tomorrow's trigger.
func TestNextRunsSkipsFiresTheEveryNGateWillClose(t *testing.T) {
	jr := newFakeJobRuns()
	last := time.Now().Add(-3 * 24 * time.Hour)
	jr.set(store.ScheduleJobDrills, last)

	_, drill, found := everyNDrillScheduler(t, jr)
	if !found {
		t.Fatal("expected a job=drill entry in NextRuns")
	}

	// The gate itself is the oracle: whatever NextRuns reports must be a fire the
	// gate would let through, and every earlier daily trigger must be one it would
	// not. Asserting against EveryNDue rather than a hand-computed date means the
	// two can never drift apart.
	if !EveryNDue(last, drill.Next, 7) {
		t.Fatalf("NextRuns reported %v, which the due-gate would SKIP — the dashboard is promising a run that will not happen", drill.Next)
	}
	tomorrow := time.Now().Add(24 * time.Hour)
	if drill.Next.Before(tomorrow) {
		t.Fatalf("NextRuns reported %v, sooner than tomorrow: with a 3-day-old pass and a 7-day interval the next real run is 4 days out", drill.Next)
	}
	if got := calendarDaysBetween(last, drill.Next); got != 7 {
		t.Fatalf("the reported run is %d calendar days after the last pass, want the first one at 7", got)
	}
}

// TestNextRunsReportsTomorrowWhenTheGateWillOpen pins the other half: when the
// interval HAS elapsed, NextRuns must not push the date out — the walk stops at
// the first due fire.
func TestNextRunsReportsTomorrowWhenTheGateWillOpen(t *testing.T) {
	jr := newFakeJobRuns()
	jr.set(store.ScheduleJobDrills, time.Now().Add(-30*24*time.Hour))

	_, drill, found := everyNDrillScheduler(t, jr)
	if !found {
		t.Fatal("expected a job=drill entry in NextRuns")
	}
	if drill.Next.After(time.Now().Add(25 * time.Hour)) {
		t.Fatalf("a long-overdue everyN pass must report its NEXT daily trigger, got %v", drill.Next)
	}
}

// TestNextRunsFallsBackToTheCronFireWhenTheQueryFails pins the conservative
// answer for "cannot tell": report the raw cron fire rather than inventing a
// later date. The gate is conservative in the other direction anyway (it skips),
// so the earliest time this could run is the honest one.
func TestNextRunsFallsBackToTheCronFireWhenTheQueryFails(t *testing.T) {
	jr := newFakeJobRuns()
	jr.queryErr = errors.New("database is locked")

	_, drill, found := everyNDrillScheduler(t, jr)
	if !found {
		t.Fatal("expected a job=drill entry in NextRuns")
	}
	if drill.Next.After(time.Now().Add(25 * time.Hour)) {
		t.Fatalf("a failed last-run query must fall back to the raw cron fire, got %v", drill.Next)
	}
}

// TestNextRunsLeavesPlainCadencesAlone pins that a non-everyN entry is reported
// exactly as cron computed it — the walk must be inert for the cadences whose
// trigger IS their run.
func TestNextRunsLeavesPlainCadencesAlone(t *testing.T) {
	noTargets := func() ([]store.Target, error) { return nil, nil }
	sc := New(func(string) error { return nil }, noTargets)
	if err := sc.Reload(store.Settings{ContainersEnabled: true, ContainersSchedule: "daily 03:00"}); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	sc.Start()
	t.Cleanup(sc.Stop)

	for _, e := range sc.entries {
		if e.domain != "containers" {
			continue
		}
		raw := sc.c.Entry(e.id).Next
		for _, r := range sc.NextRuns() {
			if r.Domain == "containers" && !r.Next.Equal(raw) {
				t.Fatalf("a plain daily cadence must be reported as cron computed it: got %v, cron says %v", r.Next, raw)
			}
		}
		return
	}
	t.Fatal("expected a registered containers entry")
}
