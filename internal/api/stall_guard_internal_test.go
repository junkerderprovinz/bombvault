package api

import (
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// The stall guard decides when a backup that is going nowhere gets cancelled.
//
// It replaces a wall-clock cap that killed runs for taking too long, which is
// the wrong question: a 4 TB first backup over a slow link SHOULD take two
// days, and a run wedged on an unresponsive NFS mount should not get 48 hours
// of them. The right question is whether anything is still happening.
//
// Named a guard, not a watchdog, on purpose: "watchdog" already means the
// overdue-backup watcher in this product, with its own setting, its own
// scheduler job and its own UI strings. A second thing by the same name would
// be confusing in the code, in the log and in a support thread.

// at is a fixed instant to build test timelines from.
func at(seconds int) time.Time {
	return time.Unix(1_700_000_000, 0).Add(time.Duration(seconds) * time.Second)
}

func TestStallGuardDoesNotFireWhileWorkHappens(t *testing.T) {
	g := newStallGuard(30*time.Minute, 2*time.Hour, at(0))

	// Half an hour of steady progress, sampled every ten minutes.
	for i := 1; i <= 3; i++ {
		d := g.observe(restic.Progress{BytesDone: uint64(i) * 1000}, at(i*600))
		if d.cancel || d.warn {
			t.Fatalf("sample %d: guard reacted to a working backup: %+v", i, d)
		}
	}
}

// The case the whole design turns on: restic scanning a large tree writes no
// bytes for minutes while its totals climb. That is work, and the guard has to
// see it as work.
func TestStallGuardSeesTheScanPhaseAsProgress(t *testing.T) {
	g := newStallGuard(30*time.Minute, 2*time.Hour, at(0))

	for i := 1; i <= 12; i++ {
		p := restic.Progress{BytesDone: 0, TotalBytes: uint64(i) * 5_000_000, TotalFiles: uint64(i) * 900}
		if d := g.observe(p, at(i*600)); d.cancel {
			t.Fatalf("the guard cancelled during the scan phase after %d minutes", i*10)
		}
	}
}

func TestStallGuardWarnsBeforeItCancels(t *testing.T) {
	g := newStallGuard(30*time.Minute, 2*time.Hour, at(0))
	stuck := restic.Progress{BytesDone: 1000, TotalBytes: 5000}
	g.observe(stuck, at(0))

	// 29 minutes in: nothing yet.
	if d := g.observe(stuck, at(29*60)); d.warn || d.cancel {
		t.Fatalf("warned too early: %+v", d)
	}
	// 31 minutes: a warning, but the run keeps going.
	d := g.observe(stuck, at(31*60))
	if !d.warn || d.cancel {
		t.Fatalf("want a warning and no cancel at 31 minutes, got %+v", d)
	}
	// The warning is said ONCE. A guard that repeats every poll turns the
	// activity log into a wall of the same sentence.
	if again := g.observe(stuck, at(35*60)); again.warn {
		t.Fatalf("the warning repeated at 35 minutes")
	}
}

func TestStallGuardCancelsAfterTheFullWindow(t *testing.T) {
	g := newStallGuard(30*time.Minute, 2*time.Hour, at(0))
	stuck := restic.Progress{BytesDone: 1000}
	g.observe(stuck, at(0))

	if d := g.observe(stuck, at(119*60)); d.cancel {
		t.Fatalf("cancelled a minute early: %+v", d)
	}
	if d := g.observe(stuck, at(121*60)); !d.cancel {
		t.Fatalf("want a cancel past two hours of silence, got %+v", d)
	}
}

// Movement after a warning clears it, and a later stall warns again. Otherwise
// a run that recovers and wedges a second time would die without a word.
func TestStallGuardResetsOnMovement(t *testing.T) {
	g := newStallGuard(30*time.Minute, 2*time.Hour, at(0))
	g.observe(restic.Progress{BytesDone: 1000}, at(0))

	if d := g.observe(restic.Progress{BytesDone: 1000}, at(31*60)); !d.warn {
		t.Fatalf("precondition: expected a warning at 31 minutes")
	}
	// It moves again.
	if d := g.observe(restic.Progress{BytesDone: 2000}, at(32*60)); d.warn || d.cancel {
		t.Fatalf("movement must clear the stall, got %+v", d)
	}
	// And stalls a second time: it has to warn again.
	if d := g.observe(restic.Progress{BytesDone: 2000}, at(70*60)); !d.warn {
		t.Fatalf("a second stall must warn again")
	}
	// The cancel deadline counts from the LAST movement, not from the start.
	if d := g.observe(restic.Progress{BytesDone: 2000}, at(100*60)); d.cancel {
		t.Fatalf("cancelled at 100 minutes, but the last movement was at 32: %+v", d)
	}
	if d := g.observe(restic.Progress{BytesDone: 2000}, at(153*60)); !d.cancel {
		t.Fatalf("want a cancel two hours after the last movement")
	}
}

// A zero window disables the guard, which is what lets an operator opt out of
// it the way BACKUP_MAX_HOURS=0 opted out of the old cap.
func TestStallGuardCanBeSwitchedOff(t *testing.T) {
	g := newStallGuard(30*time.Minute, 0, at(0))
	stuck := restic.Progress{BytesDone: 1000}
	g.observe(stuck, at(0))

	if d := g.observe(stuck, at(48*3600)); d.cancel {
		t.Fatalf("a zero window must never cancel, got %+v", d)
	}
}

// TestStallGuardFiresWithoutAnyFurtherStatusLines is the failure mode a
// line-driven guard cannot see, and the reason the real one owns a clock.
//
// A backup wedged on an unresponsive mount does not emit slow progress: it
// emits NOTHING. restic stops printing status lines entirely. A guard that only
// runs when a line arrives would therefore never run again, and the very case
// it exists for would be the one case it sleeps through.
func TestStallGuardFiresWithoutAnyFurtherStatusLines(t *testing.T) {
	g := newStallGuard(30*time.Minute, 2*time.Hour, at(0))
	g.observe(restic.Progress{BytesDone: 1000}, at(0))

	// No further observations arrive. The clock is asked directly, which is
	// what the ticker in armStallGuard does.
	if d := g.tick(at(31 * 60)); !d.warn {
		t.Fatalf("want a warning from the clock alone, got %+v", d)
	}
	if d := g.tick(at(121 * 60)); !d.cancel {
		t.Fatalf("want a cancel from the clock alone, got %+v", d)
	}
}
