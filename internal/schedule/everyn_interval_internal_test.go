package schedule

// An everyN cadence is a daily cron trigger plus a due gate (ParseCadence), so
// the gate is evaluated once per wall-clock day. Its other operand is when the
// last pass finished, which is later than that pass's fire by its runtime.
// Measured against an exact N*24h, the Nth fire would always fall short and
// every schedule would slip to N+1 days, so the gate counts calendar days from
// the fire instead. These tests check both sides: the Nth fire runs, the fires
// before it do not.

import (
	"testing"
	"time"
)

// at returns a local time in March 2026. Cron fires and stored last runs are
// both local wall-clock times.
func at(day int, hour, min int) time.Time {
	return time.Date(2026, time.March, day, hour, min, 0, 0, time.Local)
}

// TestEveryNFiresOnTheNthDayDespiteRunDuration covers a weekly pass that fires
// at 03:00 and takes four minutes. The next Monday's trigger comes 6d23h56m
// later, short of 168h, and still has to run.
func TestEveryNFiresOnTheNthDayDespiteRunDuration(t *testing.T) {
	finished := at(2, 3, 4) // Monday 03:00 fire, pass ended 03:04
	nextFire := at(9, 3, 0) // the following Monday's 03:00 trigger

	if elapsed := nextFire.Sub(finished); elapsed >= 7*24*time.Hour {
		t.Fatalf("precondition: the gap must be short of 7x24h (%v), which is the case under test", elapsed)
	}
	if !EveryNDue(finished, nextFire, 7) {
		t.Fatal("a 7-day schedule must fire on the 7th day, not the 8th")
	}
}

// TestEveryNOneRunsEveryDay covers "everyN 1", which CadenceBuilder shows as
// "daily at HH:MM". A pass of any length must not push it to every second day.
func TestEveryNOneRunsEveryDay(t *testing.T) {
	finished := at(2, 3, 12) // last night's pass, twelve minutes long
	nextFire := at(3, 3, 0)

	if !EveryNDue(finished, nextFire, 1) {
		t.Fatal("everyN 1 is the daily cadence and must run on every daily trigger")
	}
}

// TestEveryNHoldsBeforeTheNthDay checks that the fires on days 1 to N-1 after
// a run are skipped.
func TestEveryNHoldsBeforeTheNthDay(t *testing.T) {
	finished := at(2, 3, 4)
	for day := 3; day <= 8; day++ {
		fire := at(day, 3, 0)
		if EveryNDue(finished, fire, 7) {
			t.Fatalf("day %d fire ran: a 7-day schedule must skip every trigger before the 7th day", day)
		}
	}
	if !EveryNDue(finished, at(9, 3, 0), 7) {
		t.Fatal("the 7th day must run")
	}
}

// TestEveryNSurvivesAPassThatCrossesMidnight covers a 23:30 fire whose pass runs
// forty minutes and is stamped 00:10 the next day. Counting days from the stamp
// would come up one short and skip the 7th day's fire.
func TestEveryNSurvivesAPassThatCrossesMidnight(t *testing.T) {
	finished := at(3, 0, 10)  // the 2nd's 23:30 fire, forty minutes long
	nextFire := at(9, 23, 30) // the 7th day's fire

	if days := calendarDaysBetween(finished, nextFire); days >= 7 {
		t.Fatalf("precondition: the stamp must be one calendar day short of the interval (%d), which is the case under test", days)
	}
	if !EveryNDue(finished, nextFire, 7) {
		t.Fatal("a pass that ran past midnight must not cost its schedule a day: the 7th day's fire must run, not the 8th's")
	}
	// "everyN 1", shown as "daily at 23:30", has to run the next night.
	if !EveryNDue(finished, at(3, 23, 30), 1) {
		t.Fatal("everyN 1 at 23:30 must run on the next night's trigger, not the one after it")
	}
	for day := 3; day <= 8; day++ {
		if EveryNDue(finished, at(day, 23, 30), 7) {
			t.Fatalf("day %d fire ran: a 7-day 23:30 schedule must skip every trigger before the 7th day", day)
		}
	}
}

// TestMissedRunAsksTheGateAboutTheFire checks that catch-up asks the gate about
// the last fire, not the boot time. The pass fired on the 2nd at 23:30 and the
// box boots on the 9th at 08:00, so the last fire is the 8th's, six days on and
// not due. Asking at boot would count seven days and run a pass early.
func TestMissedRunAsksTheGateAboutTheFire(t *testing.T) {
	cad := mustCadence(t, "everyN 7 23:30")
	finished := at(3, 0, 10)

	if lastFire, missed := missedRun(cad, finished, at(9, 8, 0)); missed {
		t.Fatalf("the last fire (%s) is six days after the pass's own fire, so nothing was missed yet",
			lastFire.Format(time.RFC3339))
	}
	if _, missed := missedRun(cad, finished, at(10, 8, 0)); !missed {
		t.Fatal("the 7th day's 23:30 fire was slept through and must be caught up")
	}
}

// TestEveryNSurvivesSpringForward covers a week containing Europe/Berlin's
// 23-hour spring-forward day. It is 167 hours long, short of 7x24h even for a
// pass that takes no time.
func TestEveryNSurvivesSpringForward(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	// 2026-03-29 02:00 CET -> 03:00 CEST.
	finished := time.Date(2026, time.March, 26, 3, 0, 0, 0, berlin)
	nextFire := time.Date(2026, time.April, 2, 3, 0, 0, 0, berlin)

	if elapsed := nextFire.Sub(finished); elapsed >= 7*24*time.Hour {
		t.Fatalf("precondition: the DST week must be SHORT of 7x24h, got %v", elapsed)
	}
	if !EveryNDue(finished, nextFire, 7) {
		t.Fatal("a schedule must not lose a day because the clocks moved")
	}
}

func TestEveryNNeverRanIsDue(t *testing.T) {
	if !EveryNDue(time.Time{}, at(9, 3, 0), 30) {
		t.Fatal("a never-run schedule must fire on its first trigger, not 30 days later")
	}
}

// TestMissedRunUsesTheSameRule checks that the catch-up counts days the same
// way as the gate.
func TestMissedRunUsesTheSameRule(t *testing.T) {
	cad, err := ParseCadence("everyN 7 03:00")
	if err != nil {
		t.Fatalf("ParseCadence: %v", err)
	}
	// The last pass finished at 03:04 a week ago; the box boots two minutes
	// after the 03:00 fire it slept through.
	lastSuccess := at(2, 3, 4)
	now := at(9, 3, 2)

	lastFire, missed := missedRun(cad, lastSuccess, now)
	if !missed {
		t.Fatalf("the 7th day's missed fire (%s) must be caught up, not deferred to day 8", lastFire.Format(time.RFC3339))
	}
	if _, tooSoon := missedRun(cad, at(8, 3, 4), now); tooSoon {
		t.Fatal("a domain backed up yesterday must not be flagged as a missed everyN run")
	}
}

// TestCalendarDaysBetweenCountsDays includes the two DST days that are not 24
// hours long.
func TestCalendarDaysBetweenCountsDays(t *testing.T) {
	if got := calendarDaysBetween(at(2, 23, 59), at(3, 0, 1)); got != 1 {
		t.Fatalf("two minutes across midnight = %d calendar days, want 1", got)
	}
	if got := calendarDaysBetween(at(2, 0, 1), at(2, 23, 59)); got != 0 {
		t.Fatalf("same day = %d calendar days, want 0", got)
	}
	if got := calendarDaysBetween(at(9, 3, 0), at(2, 3, 0)); got != -7 {
		t.Fatalf("a future last-run = %d, want -7 (and therefore never due)", got)
	}
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	spring := time.Date(2026, time.March, 29, 12, 0, 0, 0, berlin) // 23h day
	autumn := time.Date(2026, time.October, 25, 12, 0, 0, 0, berlin)
	if got := calendarDaysBetween(spring.AddDate(0, 0, -1), spring); got != 1 {
		t.Fatalf("the 23-hour day counted as %d days, want 1", got)
	}
	if got := calendarDaysBetween(autumn.AddDate(0, 0, -1), autumn); got != 1 {
		t.Fatalf("the 25-hour day counted as %d days, want 1", got)
	}
}

// TestCalendarDaysSinceFireAnchorsOnTheFire covers why EveryNDue does not call
// calendarDaysBetween directly. now is one of the cadence's daily fires, so its
// clock time is the fire time. A last stamp earlier in the day than that
// belongs to the previous day's fire, which ran past midnight, and counts one
// day more.
func TestCalendarDaysSinceFireAnchorsOnTheFire(t *testing.T) {
	// Stamped at 03:04, after its 03:00 fire, so stamp and fire share a day.
	if got := calendarDaysSinceFire(at(2, 3, 4), at(9, 3, 0)); got != 7 {
		t.Fatalf("a same-day stamp counted %d days, want 7 (unshifted)", got)
	}
	// Fired at 23:30 on the 1st and stamped 00:10 on the 2nd. Counted from the
	// stamp the 9th is 7 days on, counted from the fire it is 8.
	if got := calendarDaysBetween(at(2, 0, 10), at(9, 3, 0)); got != 7 {
		t.Fatalf("precondition: the raw stamp-to-fire span is %d, want 7", got)
	}
	if got := calendarDaysSinceFire(at(2, 0, 10), at(9, 3, 0)); got != 8 {
		t.Fatalf("an over-midnight stamp counted %d days, want 8 (anchored one day earlier)", got)
	}
	// A stamp exactly at the fire time is not earlier and does not shift.
	if got := calendarDaysSinceFire(at(2, 3, 0), at(9, 3, 0)); got != 7 {
		t.Fatalf("a stamp exactly at the fire time counted %d days, want 7 (unshifted)", got)
	}
	// The seconds `now` carries are the delay between trigger and gate, not part
	// of the schedule, so a stamp at the fire's minute still does not shift.
	if got := calendarDaysSinceFire(at(2, 3, 0), time.Date(2026, time.March, 9, 3, 0, 41, 0, time.Local)); got != 7 {
		t.Fatalf("a gate running 41s after its trigger counted %d days, want 7", got)
	}
}
