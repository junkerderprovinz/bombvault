package schedule

import (
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// RetryDeadline is when a backup first deferred at since stops waiting.
func RetryDeadline(p store.DeferPolicy, since time.Time) time.Time {
	return since.Add(time.Duration(p.MaxHours) * time.Hour)
}

// NextRetry is when a deferred backup is tried again after a try at now: one
// interval later, moved to the start of the window when that falls outside it.
// A try that would come at or after the deadline comes at the deadline, and
// atDeadline reports it.
func NextRetry(p store.DeferPolicy, since, now time.Time) (at time.Time, atDeadline bool) {
	deadline := RetryDeadline(p, since)
	next := IntoWindow(p, now.Add(time.Duration(p.RetryMin)*time.Minute))
	if !next.Before(deadline) {
		return deadline, true
	}
	return next, false
}

// InWindow reports whether t lies in the policy's window. A policy without a
// window allows any time.
func InWindow(p store.DeferPolicy, t time.Time) bool {
	from, to, ok := window(p)
	if !ok {
		return true
	}
	m := t.Hour()*60 + t.Minute()
	if from < to {
		return m >= from && m < to
	}
	return m >= from || m < to
}

// IntoWindow is t, or the next start of the window after it when t lies
// outside the window.
func IntoWindow(p store.DeferPolicy, t time.Time) time.Time {
	if InWindow(p, t) {
		return t
	}
	from, _, _ := window(p)
	start := time.Date(t.Year(), t.Month(), t.Day(), from/60, from%60, 0, 0, t.Location())
	if !start.After(t) {
		start = time.Date(t.Year(), t.Month(), t.Day()+1, from/60, from%60, 0, 0, t.Location())
	}
	return start
}

// window reads the policy's window in minutes of the day. ok is false for a
// policy without one.
func window(p store.DeferPolicy) (from, to int, ok bool) {
	if p.WindowFrom == "" || p.WindowTo == "" {
		return 0, 0, false
	}
	fh, fm, err := parseHHMM(p.WindowFrom)
	if err != nil {
		return 0, 0, false
	}
	th, tm, err := parseHHMM(p.WindowTo)
	if err != nil {
		return 0, 0, false
	}
	from, to = fh*60+fm, th*60+tm
	return from, to, from != to
}

// ValidWindowTime reports whether s is an HH:MM time of day.
func ValidWindowTime(s string) bool {
	_, _, err := parseHHMM(s)
	return err == nil
}
