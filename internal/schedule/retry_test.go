package schedule_test

import (
	"errors"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func at(day, hh, mm int) time.Time {
	return time.Date(2026, time.October, day, hh, mm, 0, 0, time.Local)
}

func policy(retryMin, maxHours int, from, to string) store.DeferPolicy {
	p := store.DefaultDeferPolicy("t1")
	p.RetryMin, p.MaxHours, p.WindowFrom, p.WindowTo = retryMin, maxHours, from, to
	return p
}

func TestARetryComesOneIntervalLater(t *testing.T) {
	p := policy(30, 12, "", "")
	next, deadline := schedule.NextRetry(p, at(24, 1, 0), at(24, 1, 0))
	if !next.Equal(at(24, 1, 30)) || deadline {
		t.Fatalf("next = %v, %v; want 01:30 before the deadline", next, deadline)
	}
}

func TestARetryOutsideTheWindowWaitsForItsStart(t *testing.T) {
	p := policy(30, 48, "00:00", "08:00")
	next, _ := schedule.NextRetry(p, at(24, 12, 0), at(24, 12, 0))
	if !next.Equal(at(25, 0, 0)) {
		t.Fatalf("next = %v, want midnight", next)
	}
	next, _ = schedule.NextRetry(p, at(24, 12, 0), at(25, 7, 40))
	if !next.Equal(at(26, 0, 0)) {
		t.Fatalf("a try that would land at 08:10 = %v, want the next midnight", next)
	}
}

func TestAWindowAcrossMidnightAllowsBothEnds(t *testing.T) {
	p := policy(30, 48, "22:00", "06:00")
	for _, tc := range []struct {
		at   time.Time
		want bool
	}{
		{at(24, 21, 59), false},
		{at(24, 22, 0), true},
		{at(24, 23, 59), true},
		{at(25, 0, 0), true},
		{at(25, 5, 59), true},
		{at(25, 6, 0), false},
		{at(25, 12, 0), false},
	} {
		if got := schedule.InWindow(p, tc.at); got != tc.want {
			t.Errorf("InWindow(%s) = %v, want %v", tc.at.Format("15:04"), got, tc.want)
		}
	}
	if next := schedule.IntoWindow(p, at(25, 12, 0)); !next.Equal(at(25, 22, 0)) {
		t.Fatalf("IntoWindow at noon = %v, want 22:00 the same day", next)
	}
	if next := schedule.IntoWindow(p, at(25, 23, 0)); !next.Equal(at(25, 23, 0)) {
		t.Fatalf("IntoWindow inside the window moved the time to %v", next)
	}
}

func TestNoWindowAllowsAnyTime(t *testing.T) {
	for _, p := range []store.DeferPolicy{policy(30, 12, "", ""), policy(30, 12, "03:00", "03:00")} {
		if !schedule.InWindow(p, at(24, 15, 17)) {
			t.Fatalf("%+v keeps a try out", p)
		}
	}
}

func TestATryPastTheDeadlineComesAtTheDeadline(t *testing.T) {
	p := policy(90, 2, "", "")
	since := at(24, 1, 0)
	next, deadline := schedule.NextRetry(p, since, at(24, 2, 0))
	if !deadline || !next.Equal(at(24, 3, 0)) {
		t.Fatalf("next = %v, %v; want the deadline at 03:00", next, deadline)
	}
	p = policy(30, 6, "00:00", "02:00")
	next, deadline = schedule.NextRetry(p, since, at(24, 1, 50))
	if !deadline || !next.Equal(at(24, 7, 0)) {
		t.Fatalf("a window that only opens after the deadline gave %v, %v", next, deadline)
	}
}

// deferredErr stands for a backup that waits for its app.
type deferredErr struct{}

func (deferredErr) Error() string  { return "backup deferred: busy" }
func (deferredErr) Deferred() bool { return true }

func TestADeferredContainerNeitherFailsNorHoldsUpTheRun(t *testing.T) {
	targets := []store.Target{
		{ContainerName: "plex", IncludeInSchedule: true},
		{ContainerName: "db", IncludeInSchedule: true},
		{ContainerName: "web", IncludeInSchedule: true},
	}
	var ran []string
	attempted, failed, failures := schedule.RunContainersJob(targets, func(name string) error {
		ran = append(ran, name)
		switch name {
		case "plex":
			return deferredErr{}
		case "web":
			return errors.New("disk full")
		}
		return nil
	})
	if len(ran) != 3 {
		t.Fatalf("ran %v, want every container", ran)
	}
	if attempted != 3 || failed != 1 || len(failures) != 1 || failures[0].Name != "web" {
		t.Fatalf("attempted %d, failed %d, failures %+v", attempted, failed, failures)
	}
}
