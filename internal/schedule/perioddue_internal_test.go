package schedule

// PeriodDue gates cadences that a sweep evaluates instead of their own cron
// entry, such as the received-repo integrity check that the daily receiver
// watch walks. last is stamped when the previous pass finished, a few minutes
// after the sweep fired, so comparing elapsed seconds against the period would
// fall short on the day the check comes due and halve its frequency.

import (
	"testing"
	"time"
)

// TestPeriodDueSurvivesTheChecksOwnRuntime covers a daily check whose last run
// finished ten minutes after the sweep fired. Only 85800 seconds pass until
// the next sweep, but it is a new day and the check is due.
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
			"not the one after it, which is the every-other-day skip")
	}
}

// TestPeriodDueWeeklySurvivesTheChecksOwnRuntime covers the same case for a
// weekly check, which would otherwise run every 14 days.
func TestPeriodDueWeeklySurvivesTheChecksOwnRuntime(t *testing.T) {
	const week = int64(7 * 86400)
	finished := at(10, 9, 15).Add(42 * time.Minute)
	sweepDay7 := at(17, 9, 15)

	if !PeriodDue(finished, sweepDay7, week) {
		t.Fatal("a weekly check must be due on the seventh day's sweep, not the fourteenth")
	}
}

// TestPeriodDueStillHoldsInsideTheInterval checks that the gate still closes;
// otherwise every sweep would run a full restic check on every received repo.
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

func TestPeriodDueNeverCheckedRuns(t *testing.T) {
	if !PeriodDue(time.Time{}, at(11, 9, 15), 7*86400) {
		t.Fatal("a repo that has never been checked must be due on the first sweep")
	}
}

// TestPeriodDueSubDailyRunsEverySweep checks that a cadence finer than the
// daily sweep is due on every sweep, which is as often as it can run.
func TestPeriodDueSubDailyRunsEverySweep(t *testing.T) {
	const sixHours = int64(6 * 3600)
	finished := at(10, 9, 25)
	if !PeriodDue(finished, at(11, 9, 15), sixHours) {
		t.Fatal("a sub-daily cadence must be due on every sweep")
	}
}

// TestEveryNDueFutureStampIsNotAFreeze covers a stamp written while the clock
// was years ahead. Its day count is negative, below any interval, and since the
// skipped job is the one that would rewrite the stamp, the schedule would
// never run again.
func TestEveryNDueFutureStampIsNotAFreeze(t *testing.T) {
	now := at(11, 3, 0)
	fromABrokenClock := time.Date(2035, time.March, 11, 3, 0, 0, 0, time.Local)

	if calendarDaysBetween(fromABrokenClock, now) >= 0 {
		t.Fatal("test premise broken: the stamp is not in the future")
	}
	if !EveryNDue(fromABrokenClock, now, 7) {
		t.Fatal("a last-run stamp from the future must read as \"never ran\" and let the pass through; " +
			"otherwise the schedule is frozen permanently with nothing but a \"last run -78840h ago\" log line")
	}
	// PeriodDue shares the guard.
	if !PeriodDue(fromABrokenClock, now, 86400) {
		t.Fatal("PeriodDue must inherit EveryNDue's future-stamp reading")
	}
}
