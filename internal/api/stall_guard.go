package api

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// The stall guard cancels a backup that has stopped making progress.
//
// It answers a better question than the wall-clock cap it replaces. "Has this
// run taken more than 48 hours" punishes the runs that legitimately need the
// most time: a first backup of 4 TB over a slow uplink SHOULD take two days.
// "Has anything happened in the last two hours" catches the case the cap was
// really aimed at, a run wedged on an unresponsive mount, and leaves the slow
// but healthy one alone.
//
// Called a guard rather than a watchdog deliberately. "Watchdog" already means
// the overdue-backup watcher in this product, with its own setting, its own
// scheduler job and its own strings; a second one by the same name would be
// ambiguous in the code, in the log line and in a support thread.
type stallGuard struct {
	// warnAfter is how long without movement before the activity log says so.
	// The warning exists because the cancel is drastic: an operator who sees
	// "no progress for 30 minutes" can look at the mount before it is killed.
	warnAfter time.Duration
	// cancelAfter is how long without movement before the run is cancelled. A
	// zero value disables cancelling entirely, which is the opt-out.
	cancelAfter time.Duration

	// mu guards everything below: observe runs on the goroutine reading
	// restic output, while tick runs on the caller's own ticker.
	mu       sync.Mutex
	last     restic.Progress
	lastMove time.Time
	// warned makes the warning fire once per stall rather than on every poll,
	// so a two-hour stall leaves one line in the activity log instead of
	// hundreds of copies of the same sentence.
	warned bool
	// started is false until the first observation, so the guard never measures
	// silence from a zero-valued baseline it never saw.
	started bool
}

// stallDecision is what the guard concluded from one observation.
type stallDecision struct {
	// warn is true exactly once per stall, when warnAfter has passed.
	warn bool
	// cancel is true once cancelAfter has passed with no movement.
	cancel bool
	// silent is how long nothing has moved, for the message.
	silent time.Duration
}

func newStallGuard(warnAfter, cancelAfter time.Duration, now time.Time) *stallGuard {
	return &stallGuard{warnAfter: warnAfter, cancelAfter: cancelAfter, lastMove: now}
}

// observe records one status line and reports what to do about it.
func (g *stallGuard) observe(p restic.Progress, now time.Time) stallDecision {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.started {
		g.started, g.last, g.lastMove = true, p, now
		return stallDecision{}
	}
	if p.MovedSince(g.last) {
		g.last, g.lastMove, g.warned = p, now, false
		return stallDecision{}
	}
	return g.decideLocked(now)
}

// tick asks the guard for a verdict WITHOUT a new status line, and it is the
// entry that matters most.
//
// A backup wedged on an unresponsive mount does not report slow progress:
// restic stops printing status lines altogether. A guard driven only by
// arriving lines would therefore fall silent at exactly the moment it exists
// for, so the caller runs a ticker and asks the clock directly.
func (g *stallGuard) tick(now time.Time) stallDecision {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.started {
		return stallDecision{}
	}
	return g.decideLocked(now)
}

// decideLocked is the shared verdict; callers hold g.mu.
func (g *stallGuard) decideLocked(now time.Time) stallDecision {
	silent := now.Sub(g.lastMove)
	d := stallDecision{silent: silent}
	if g.cancelAfter > 0 && silent >= g.cancelAfter {
		d.cancel = true
		return d
	}
	if !g.warned && g.warnAfter > 0 && silent >= g.warnAfter {
		g.warned = true
		d.warn = true
	}
	return d
}

// stallWarnAfter is how long a backup may be silent before the log says so.
// Fixed rather than configurable: it is a hint, not a policy, and one more
// environment variable to explain costs more than it buys.
const stallWarnAfter = 30 * time.Minute

// defaultStallCancelAfter is how long a backup may be silent before it is
// cancelled. Two hours is long enough to sit through a slow remote listing or a
// long pack upload on a bad link, and short enough that a wedged run does not
// hold its domain overnight.
const defaultStallCancelAfter = 2 * time.Hour

// stallCancelAfter reads BACKUP_STALL_HOURS.
//
//	unset/empty          -> 2h
//	N (positive integer) -> N hours
//	0                    -> the guard never cancels (it still warns)
//	invalid              -> a warning is logged and the default is used
//
// It sits ALONGSIDE BACKUP_MAX_HOURS rather than replacing it. The two answer
// different questions: the cap bounds how long a run may hold its domain at
// all, including the phases after the backup itself (retention, stats,
// off-site copy) where no byte counter exists to watch. Removing it would have
// left those with no bound whatsoever.
func stallCancelAfter() time.Duration {
	raw := strings.TrimSpace(os.Getenv("BACKUP_STALL_HOURS"))
	if raw == "" {
		return defaultStallCancelAfter
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		log.Printf("api: invalid BACKUP_STALL_HOURS=%q (want a non-negative integer number of hours), using default %v", raw, defaultStallCancelAfter) //nolint:gosec // G706: %q-quoted
		return defaultStallCancelAfter
	}
	if n == 0 {
		return 0
	}
	return time.Duration(n) * time.Hour
}

// stallTickInterval is how often the guard asks the clock. A minute is far
// finer than the half-hour warning and the two-hour cancel need, and costs one
// wakeup per minute per running backup.
const stallTickInterval = time.Minute

// armStallGuard attaches a stall guard to a BACKUP context.
//
// Returns a context that carries the progress watcher, and starts a ticker that
// keeps asking even when restic has gone completely silent, which is the state
// a wedged run is actually in.
//
// Only ever called on backup paths. A restore is deliberately not cancellable
// (an interrupted restore has already removed the container and half-written
// its appdata) and maintenance commands emit no counters at all, so silence
// there would mean nothing.
func armStallGuard(ctx context.Context, cancel context.CancelFunc, label string) context.Context {
	g := newStallGuard(stallWarnAfter, stallCancelAfter(), time.Now())

	act := func(d stallDecision) {
		if d.warn {
			log.Printf("api: %s: no progress for %v. The backup is still running; if the target is a network share, check that it is still responding.", label, d.silent.Round(time.Minute)) //nolint:gosec // G706: label is a fixed caller literal
		}
		if d.cancel {
			log.Printf("api: %s: cancelled after %v without progress. Nothing was written by the stalled attempt: restic writes its snapshot last, so an aborted run leaves unreferenced data and no snapshot.", label, d.silent.Round(time.Minute)) //nolint:gosec // G706: label is a fixed caller literal
			cancel()
		}
	}

	watched := restic.WithWatcher(ctx, func(p restic.Progress) { act(g.observe(p, time.Now())) })

	go func() {
		t := time.NewTicker(stallTickInterval)
		defer t.Stop()
		for {
			select {
			case <-watched.Done():
				return
			case now := <-t.C:
				act(g.tick(now))
			}
		}
	}()
	return watched
}
