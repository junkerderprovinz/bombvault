package schedule

// The everyN due-gate, measured against the grid it is actually compared with.
//
// An everyN cadence is a DAILY cron trigger plus this gate (ParseCadence), so
// consecutive evaluations are exactly one wall-clock day apart. The gate's other
// operand is when the pass last FINISHED, which is later than its own fire by
// the pass's runtime. Against an exact N*24h threshold the Nth day is therefore
// always short by that runtime, the fire is skipped, and — because the fire that
// finally does run re-stamps the record later still — every everyN schedule
// settles into an everyN+1 rhythm. These tests pin the fixed rule from both
// sides: the Nth fire runs, the fires before it do not.

import (
	"testing"
	"time"
)

// at builds a local timestamp, the way both operands of the gate arrive: a cron
// fire and a stored last-run are both local wall-clock instants.
func at(day int, hour, min int) time.Time {
	return time.Date(2026, time.March, day, hour, min, 0, 0, time.Local)
}

// TestEveryNFiresOnTheNthDayDespiteRunDuration is the defect itself. A weekly
// pass fires at 03:00 and takes four minutes; the next Monday's 03:00 trigger
// measures 6d23h56m, which is less than 168h.
func TestEveryNFiresOnTheNthDayDespiteRunDuration(t *testing.T) {
	finished := at(2, 3, 4) // Monday 03:00 fire, pass ended 03:04
	nextFire := at(9, 3, 0) // the following Monday's 03:00 trigger

	if elapsed := nextFire.Sub(finished); elapsed >= 7*24*time.Hour {
		t.Fatalf("precondition: the gap must be SHORT of 7x24h (%v) — that is the trap being guarded", elapsed)
	}
	if !EveryNDue(finished, nextFire, 7) {
		t.Fatal("a 7-day schedule must fire on the 7th day, not the 8th")
	}
}

// TestEveryNOneRunsEveryDay: "everyN 1" is what CadenceBuilder renders as
// "daily at HH:MM". Under the old duration comparison a pass of ANY non-zero
// length made it run every second day.
func TestEveryNOneRunsEveryDay(t *testing.T) {
	finished := at(2, 3, 12) // last night's pass, twelve minutes long
	nextFire := at(3, 3, 0)

	if !EveryNDue(finished, nextFire, 1) {
		t.Fatal("everyN 1 must run on every daily trigger — it IS the daily cadence")
	}
}

// TestEveryNHoldsBeforeTheNthDay is the other side: the gate must still be a
// gate. The fires on days 1..N-1 after a run are skipped.
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

// TestEveryNSurvivesSpringForward: a long pass is not the only way to lose an
// hour. Europe/Berlin's spring-forward night is 23 hours long, so seven of them
// are 167h — again short of the exact threshold, again a skipped fire, for a
// pass that took no time at all.
func TestEveryNSurvivesSpringForward(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err) // Windows without ZONEINFO; the DST case is Linux-verified
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

// TestEveryNNeverRanIsDue pins the deliberate exception the registration loop
// documents: no record at all means "never ran", which runs.
func TestEveryNNeverRanIsDue(t *testing.T) {
	if !EveryNDue(time.Time{}, at(9, 3, 0), 30) {
		t.Fatal("a never-run schedule must fire on its first trigger, not 30 days later")
	}
}

// TestMissedRunUsesTheSameRule closes the second half of the mechanism: the
// anacron catch-up mirrors the gate, and the two must not drift. Before the
// fix, catch-up carried its own copy of the same short comparison.
func TestMissedRunUsesTheSameRule(t *testing.T) {
	cad, err := ParseCadence("everyN 7 03:00")
	if err != nil {
		t.Fatalf("ParseCadence: %v", err)
	}
	// Last success a week ago plus the pass's own runtime; the box boots back up
	// two minutes after the 03:00 fire it slept through, which is the window the
	// short comparison used to swallow (7 days minus the pass's four minutes).
	lastSuccess := at(2, 3, 4)
	now := at(9, 3, 2)

	lastFire, missed := missedRun(cad, lastSuccess, now)
	if !missed {
		t.Fatalf("the 7th day's missed fire (%s) must be caught up, not deferred to day 8", lastFire.Format(time.RFC3339))
	}
	// …and a domain that is genuinely not due yet still must not be caught up.
	if _, tooSoon := missedRun(cad, at(8, 3, 4), now); tooSoon {
		t.Fatal("a domain backed up yesterday must not be flagged as a missed everyN run")
	}
}

// TestCalendarDaysBetweenCountsDays guards the arithmetic the rule rests on,
// including the two DST days that are not 24 hours long.
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
