package schedule

import (
	"testing"
	"time"
)

// A Thursday in November 2026, and the Sunday before it.
var (
	thu0930 = time.Date(2026, time.November, 5, 9, 30, 0, 0, time.Local)
	sun0200 = time.Date(2026, time.November, 1, 2, 0, 0, 0, time.Local)
)

func TestFireDueHoldsTheTimeAndWeekday(t *testing.T) {
	cases := []struct {
		name, cadence string
		since, now    time.Time
		want          bool
	}{
		{"weekly, on the wrong day", "weekly Sun 02:00", sun0200.Add(time.Minute), thu0930, false},
		{"weekly, one minute before its fire", "weekly Sun 02:00", sun0200.Add(time.Minute), sun0200.AddDate(0, 0, 7).Add(-time.Minute), false},
		{"weekly, at its fire", "weekly Sun 02:00", sun0200.Add(time.Minute), sun0200.AddDate(0, 0, 7), true},
		{"weekly, a run that took longer than a minute", "weekly Sun 02:00", sun0200.Add(40 * time.Minute), sun0200.Add(41 * time.Minute), false},
		{"daily, before today's time", "daily 04:00", thu0930.AddDate(0, 0, -1), thu0930.Add(-6 * time.Hour), false},
		{"daily, past today's time", "daily 04:00", thu0930.AddDate(0, 0, -1), thu0930, true},
		{"hourly cron, the next hour", "0 * * * *", thu0930, thu0930.Add(31 * time.Minute), true},
		{"hourly cron, the same hour", "0 * * * *", thu0930, thu0930.Add(5 * time.Minute), false},
		{"a fire missed while the box was off", "weekly Sun 02:00", sun0200.AddDate(0, 0, -7).Add(time.Minute), thu0930, true},
		{"every third day, the day after", "everyN 3 04:00", time.Date(2026, 11, 3, 4, 5, 0, 0, time.Local), time.Date(2026, 11, 4, 4, 0, 0, 0, time.Local), false},
		{"every third day, the third day", "everyN 3 04:00", time.Date(2026, 11, 3, 4, 5, 0, 0, time.Local), time.Date(2026, 11, 6, 4, 0, 0, 0, time.Local), true},
		{"off", "off", time.Time{}, thu0930, false},
		{"a stamp from a wrong clock", "weekly Sun 02:00", thu0930.AddDate(1, 0, 0), thu0930, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mustCadence(t, tc.cadence).FireDue(tc.since, tc.now); got != tc.want {
				t.Fatalf("FireDue(%v, %v) = %v, want %v", tc.since, tc.now, got, tc.want)
			}
		})
	}
}

func TestRetryDueComesBackAtTheSameClockTimeNextDay(t *testing.T) {
	failed := sun0200.Add(20 * time.Second)
	weekly := mustCadence(t, "weekly Sun 02:00")
	cases := []struct {
		name string
		now  time.Time
		want bool
	}{
		{"the same night", sun0200.Add(2 * time.Hour), false},
		{"the next day, before the time", sun0200.AddDate(0, 0, 1).Add(-time.Minute), false},
		{"the next day, at the time", sun0200.AddDate(0, 0, 1), true},
		{"later that day", sun0200.AddDate(0, 0, 1).Add(5 * time.Hour), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := weekly.RetryDue(failed, tc.now); got != tc.want {
				t.Fatalf("RetryDue at %v = %v, want %v", tc.now, got, tc.want)
			}
		})
	}
	if mustCadence(t, "off").RetryDue(failed, thu0930) {
		t.Fatal("a source that is off retried")
	}
}

func TestNextDueIsTheCadencesOwnFire(t *testing.T) {
	next, ok := mustCadence(t, "weekly Sun 02:00").NextDue(sun0200, thu0930)
	if !ok || !next.Equal(sun0200.AddDate(0, 0, 7)) {
		t.Fatalf("next = %v, %v", next, ok)
	}
	last := time.Date(2026, 11, 3, 4, 5, 0, 0, time.Local)
	next, ok = mustCadence(t, "everyN 3 04:00").NextDue(last, last.Add(time.Hour))
	if want := time.Date(2026, 11, 6, 4, 0, 0, 0, time.Local); !ok || !next.Equal(want) {
		t.Fatalf("every third day: next = %v, %v, want %v", next, ok, want)
	}
	if _, ok := mustCadence(t, "off").NextDue(last, thu0930); ok {
		t.Fatal("off has a next fire")
	}
}
