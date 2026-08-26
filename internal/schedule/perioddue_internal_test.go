package schedule

// ---------------------------------------------------------------------------
// PeriodDue — the due-gate for cadences evaluated by a SWEEP of their own
// rather than by their own cron entry (today: the received-repo integrity
// check, which the fixed daily receiver watch walks).
//
// The defect these pin: `now - last < period` in elapsed seconds. `last` is
// stamped when the previous pass FINISHED and `now` is the sweep's fixed daily
// fire, so the day a daily cadence comes due always measures a few minutes
// SHORT of 86400 — the previous check's own runtime — the gate closes, and the
// next chance is a whole day later. Every such repo ran at half its configured
// frequency, silently.
// ---------------------------------------------------------------------------

import (
	"testing"
	"time"
)

// These reuse `at` from everyn_interval_internal_test.go (March 2026, local
// zone), so both halves of the same rule are pinned against the same grid.

// TestPeriodDueSurvivesTheChecksOwnRuntime is the regression proper. A daily
// check is evaluated by a daily sweep; yesterday's check finished ten minutes
// after the sweep fired. Elapsed seconds say 85800 < 86400 ("not due"); the
// calendar says it is a new day, and the check is due.
func TestPeriodDueSurvivesTheChecksOwnRuntime(t *testing.T) {
	const day = int64(86400)
	sweepDay0 := at(10, 9, 15)
	finished := sweepDay0.Add(10 * time.Minute) // a restic check on a large repo
	sweepDay1 := at(11, 9, 15)

	if elapsed := int64(sweepDay1.Sub(finished).Seconds()); elapsed >= day {
		t.Fatalf("test premise broken: elapsed %ds is not short of a day", elapsed)
	}
	if !PeriodDue(finished, sweepDay1, day) {
		t.Fatal("a daily check whose previous run finished after the sweep fired must be due on the NEXT day's sweep, " +
			"not the one after it — this is the every-other-day skip")
	}
}

// TestPeriodDueWeeklySurvivesTheChecksOwnRuntime is the same slip at the weekly
// cadence, where the cost is a check every 14 days instead of every 7.
func TestPeriodDueWeeklySurvivesTheChecksOwnRuntime(t *testing.T) {
	const week = int64(7 * 86400)
	finished := at(10, 9, 15).Add(42 * time.Minute)
	sweepDay7 := at(17, 9, 15)

	if !PeriodDue(finished, sweepDay7, week) {
		t.Fatal("a weekly check must be due on the seventh day's sweep, not the fourteenth")
	}
}

// TestPeriodDueStillHoldsInsideTheInterval pins the other half: the gate must
// still CLOSE, or the cadence would mean nothing and every sweep would run a
// full restic check on every received repo.
func TestPeriodDueStillHoldsInsideTheInterval(t *testing.T) {
	const day = int64(86400)
	sameDayEarlier := at(11, 4, 0)
	sweep := at(11, 9, 15)
	if PeriodDue(sameDayEarlier, sweep, day) {
		t.Fatal("a daily check that already ran TODAY must not run again on today's sweep")
	}

	const week = int64(7 * 86400)
	threeDaysAgo := at(8, 9, 25)
	if PeriodDue(threeDaysAgo, sweep, week) {
		t.Fatal("a weekly check three days old must not be due")
	}
}

// TestPeriodDueNeverCheckedRuns pins the fresh-repo case: no check has ever run,
// so the first sweep runs one rather than deferring by a whole interval.
func TestPeriodDueNeverCheckedRuns(t *testing.T) {
	if !PeriodDue(time.Time{}, at(11, 9, 15), 7*86400) {
		t.Fatal("a repo that has never been checked must be due on the first sweep")
	}
}

// TestPeriodDueSubDailyRunsEverySweep pins the deliberate choice for a cadence
// finer than the sweep that evaluates it: the sweep's own daily granularity is
// the binding constraint, and measuring a 6-hourly cadence in elapsed seconds
// against a daily sweep would reproduce the very skip this gate removes.
func TestPeriodDueSubDailyRunsEverySweep(t *testing.T) {
	const sixHours = int64(6 * 3600)
	finished := at(10, 9, 25)
	if !PeriodDue(finished, at(11, 9, 15), sixHours) {
		t.Fatal("a sub-daily cadence must be due on every sweep")
	}
}

// TestEveryNDueFutureStampIsNotAFreeze covers the wrong-clock case at the one
// place every everyN caller passes through. A stamp written while the clock was
// years ahead makes the elapsed count NEGATIVE, which is below any interval, so
// the plain comparison skips every fire from then on — forever, because the job
// that would re-stamp the record is the one being skipped.
func TestEveryNDueFutureStampIsNotAFreeze(t *testing.T) {
	now := at(11, 3, 0)
	fromABrokenClock := time.Date(2035, time.March, 11, 3, 0, 0, 0, time.Local)

	if calendarDaysBetween(fromABrokenClock, now) >= 0 {
		t.Fatal("test premise broken: the stamp is not in the future")
	}
	if !EveryNDue(fromABrokenClock, now, 7) {
		t.Fatal("a last-run stamp from the FUTURE must read as \"never ran\" and let the pass through — " +
			"otherwise the schedule is frozen permanently with nothing but a \"last run -78840h ago\" log line")
	}
	// Both gates share that guard, so the receiver check has the same protection.
	if !PeriodDue(fromABrokenClock, now, 86400) {
		t.Fatal("PeriodDue must inherit EveryNDue's future-stamp reading")
	}
}
