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

// TestEveryNSurvivesAPassThatCrossesMidnight is the half of the defect whole
// calendar days did NOT remove. Counting days from the stamp only works for a
// pass that finishes on the day it fired: a 23:30 fire that runs forty minutes
// stamps 00:10 the next morning, the count is one day short from then on, the
// 7th day's fire is skipped, and the fire that finally runs stamps later still —
// the everyN+1 rhythm again, for the schedules whose fire sits late enough in
// the evening to be the likeliest to have it.
func TestEveryNSurvivesAPassThatCrossesMidnight(t *testing.T) {
	finished := at(3, 0, 10)  // the 2nd's 23:30 fire, forty minutes long
	nextFire := at(9, 23, 30) // the 7th day's fire

	if days := calendarDaysBetween(finished, nextFire); days >= 7 {
		t.Fatalf("precondition: the stamp must be one calendar day short of the interval (%d) — that is the trap being guarded", days)
	}
	if !EveryNDue(finished, nextFire, 7) {
		t.Fatal("a pass that ran past midnight must not cost its schedule a day: the 7th day's fire must run, not the 8th's")
	}
	// "everyN 1" — the cadence the UI calls "daily at 23:30" — is the same case
	// one interval down: it ran every SECOND night.
	if !EveryNDue(finished, at(3, 23, 30), 1) {
		t.Fatal("everyN 1 at 23:30 must run on the next night's trigger, not the one after it")
	}
	// …and the gate must still be a gate for that schedule: every fire before the
	// 7th day is skipped.
	for day := 3; day <= 8; day++ {
		if EveryNDue(finished, at(day, 23, 30), 7) {
			t.Fatalf("day %d fire ran: a 7-day 23:30 schedule must skip every trigger before the 7th day", day)
		}
	}
}

// TestMissedRunAsksTheGateAboutTheFire is the catch-up half of the same anchor.
// The pass that produced the stamp fired on the 2nd at 23:30; the box boots on
// the 9th at 08:00, so the last fire is the 8th's — six days on, not yet due,
// nothing missed. Asked at the boot instant instead of at that fire, the gate
// reads the seven days it will only reach at 23:30 tonight and the catch-up runs
// a whole pass early, on every boot.
func TestMissedRunAsksTheGateAboutTheFire(t *testing.T) {
	cad := mustCadence(t, "everyN 7 23:30")
	finished := at(3, 0, 10)

	if lastFire, missed := missedRun(cad, finished, at(9, 8, 0)); missed {
		t.Fatalf("the last fire (%s) is six days after the pass's own fire — nothing was missed yet",
			lastFire.Format(time.RFC3339))
	}
	// The fire that IS seven days on must still be caught up when the box slept
	// through it.
	if _, missed := missedRun(cad, finished, at(10, 8, 0)); !missed {
		t.Fatal("the 7th day's 23:30 fire was slept through and must be caught up")
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

// TestCalendarDaysSinceFireAnchorsOnTheFire guards the anchor correction itself,
// which had no direct test of its own until a date-dependent failure in
// nextruns_everyn_internal_test.go exposed it (2026-08-28): the stamp-vs-fire
// distinction is the whole reason EveryNDue does not simply call
// calendarDaysBetween, so it deserves to be pinned rather than only exercised
// through a caller.
//
// The rule: `now` is one of the cadence's own daily fires, so its clock time IS
// the fire time. A `last` stamp EARLIER in the day than that belongs to the
// PREVIOUS day's fire, which ran past midnight, and must count one day more.
func TestCalendarDaysSinceFireAnchorsOnTheFire(t *testing.T) {
	// The plain case: a pass that started at 03:00 and finished at 03:04 is
	// stamped after the fire, so stamp and fire share a day and nothing shifts.
	if got := calendarDaysSinceFire(at(2, 3, 4), at(9, 3, 0)); got != 7 {
		t.Fatalf("a same-day stamp counted %d days, want 7 (unshifted)", got)
	}
	// The correction: a pass that fired at 23:30 on day 1 and ran past midnight
	// is stamped 00:10 on day 2. Counting from the STAMP gives 6, which would
	// skip the seventh day's fire; counting from the FIRE gives 7.
	if got := calendarDaysBetween(at(2, 0, 10), at(9, 3, 0)); got != 7 {
		t.Fatalf("precondition: the raw stamp-to-fire span is %d, want 7", got)
	}
	if got := calendarDaysSinceFire(at(2, 0, 10), at(9, 3, 0)); got != 8 {
		t.Fatalf("an over-midnight stamp counted %d days, want 8 (anchored one day earlier)", got)
	}
	// Exactly at the fire time is NOT earlier, so it must not shift.
	if got := calendarDaysSinceFire(at(2, 3, 0), at(9, 3, 0)); got != 7 {
		t.Fatalf("a stamp exactly at the fire time counted %d days, want 7 (unshifted)", got)
	}
	// The seconds `now` carries are the delay between trigger and gate, not part
	// of the schedule, so a stamp at the fire's minute still does not shift.
	if got := calendarDaysSinceFire(at(2, 3, 0), time.Date(2026, time.March, 9, 3, 0, 41, 0, time.Local)); got != 7 {
		t.Fatalf("a gate running 41s after its trigger counted %d days, want 7", got)
	}
}
