package schedule

// ParseCadence compiles "everyN N HH:MM" to a daily cron spec and carries N on
// the side, so the cron entry's own Next is tomorrow even on the N-1 nights the
// due gate will close. NextRuns feeds the dashboard's "up next" line and has to
// name the fire that will actually run. These tests register through
// ReloadWithDueChecks and the job-run store, as the scheduler does.

import (
	"errors"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// everyNDrillScheduler registers the drills schedule on "everyN 7 03:00" with jr
// as its job-run store, starts cron (Entry.Next is only computed while it runs)
// and returns the drill's reported next run.
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

// TestNextRunsSkipsFiresTheEveryNGateWillClose covers a drill pass that ran
// three days ago on a 7-day interval. The gate will skip the next nightly
// triggers, so "up next" must name the first fire it lets through.
func TestNextRunsSkipsFiresTheEveryNGateWillClose(t *testing.T) {
	jr := newFakeJobRuns()
	last := time.Now().Add(-3 * 24 * time.Hour)
	jr.set(store.ScheduleJobDrills, last)

	_, drill, found := everyNDrillScheduler(t, jr)
	if !found {
		t.Fatal("expected a job=drill entry in NextRuns")
	}

	// Checked against the gate itself rather than a hand-computed date, so the
	// two cannot drift apart.
	if !EveryNDue(last, drill.Next, 7) {
		t.Fatalf("NextRuns reported %v, which the due-gate would skip; the dashboard is promising a run that will not happen", drill.Next)
	}
	tomorrow := time.Now().Add(24 * time.Hour)
	if drill.Next.Before(tomorrow) {
		t.Fatalf("NextRuns reported %v, sooner than tomorrow: with a 3-day-old pass and a 7-day interval the next real run is 4 days out", drill.Next)
	}
	// Count with the gate's own measure. calendarDaysSinceFire anchors on the
	// fire that produced last, and last carries now's clock time, so between
	// midnight and the 03:00 fire calendarDaysBetween would be off by one.
	//
	// Exactly 7: the walk has to stop at the first fire the gate lets through.
	if got := calendarDaysSinceFire(last, drill.Next); got != 7 {
		t.Fatalf("the reported run is %d calendar days after the last pass's fire, want the first one at 7", got)
	}
}

// TestNextRunsReportsTomorrowWhenTheGateWillOpen checks that once the interval
// has passed, NextRuns reports the next daily trigger.
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

// TestNextRunsFallsBackToTheCronFireWhenTheQueryFails checks that without a
// last run to go on, NextRuns reports the raw cron fire, the earliest time the
// job could run, rather than inventing a later date.
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

// TestNextRunsLeavesPlainCadencesAlone checks that an entry without everyN is
// reported exactly as cron computed it.
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
