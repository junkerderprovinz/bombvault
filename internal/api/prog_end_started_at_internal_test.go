package api

import (
	"context"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/progress"
)

// TestProgEndPublishesStartedAt calls the REAL progBegin/progEnd pair — not a
// hand-built progress.Event — and asserts the terminal SSE event they
// produce carries the SAME StartedAt as the begin event. Before the review
// fix this pinned, progEnd published StartedAt:0 on the terminal event,
// which made a client-rendered live duration visibly vanish during the
// terminal-event linger (see web/src/lib/progress.ts's COMPLETE_LINGER_MS
// and OffsiteIndicator's MIN_VISIBLE_MS).
//
// progress_test.go's TestEventStartedAtSurvivesTerminalEvent has a doc
// comment claiming to pin exactly this, but it only calls
// progress.Store.Publish directly with a hand-built Event{StartedAt: ...} —
// it never calls api.progEnd at all, so a regression that hardcoded
// StartedAt:0 (or dropped the parameter entirely) inside progEnd itself
// would leave that test green. This test closes that gap by driving the
// actual production function.
func TestProgEndPublishesStartedAt(t *testing.T) {
	prog := progress.NewStore()
	svc := &Service{progress: prog}

	ch, cancel := prog.Subscribe()
	defer cancel()

	_, startedAt := svc.progBegin(context.Background(), "container:plex", "backup")
	if startedAt <= 0 {
		t.Fatalf("progBegin returned a non-positive StartedAt: %d", startedAt)
	}

	begin := <-ch
	if begin.StartedAt != startedAt {
		t.Fatalf("begin event StartedAt = %d, want %d", begin.StartedAt, startedAt)
	}

	svc.progEnd("container:plex", "backup", true, startedAt)

	term := <-ch
	if term.Active {
		t.Fatal("expected progEnd's event to be Active:false (terminal)")
	}
	if term.Percent != 100 {
		t.Fatalf("expected 100%% on a successful progEnd, got %v", term.Percent)
	}
	if term.StartedAt != startedAt {
		t.Fatalf("progEnd's terminal event StartedAt = %d, want %d (the SAME value progBegin returned)", term.StartedAt, startedAt)
	}
}

// TestProgEndFailurePublishesStartedAtAndZeroPercent mirrors the failure
// branch (ok=false): progEnd still owes StartedAt to the terminal event even
// though the run failed, and reports 0% rather than 100%.
func TestProgEndFailurePublishesStartedAtAndZeroPercent(t *testing.T) {
	prog := progress.NewStore()
	svc := &Service{progress: prog}

	ch, cancel := prog.Subscribe()
	defer cancel()

	_, startedAt := svc.progBegin(context.Background(), "offsite:files", "replicate")
	<-ch // begin event

	svc.progEnd("offsite:files", "replicate", false, startedAt)

	term := <-ch
	if term.Active {
		t.Fatal("expected progEnd's failure event to be Active:false (terminal)")
	}
	if term.Percent != 0 {
		t.Fatalf("expected 0%% on a failed progEnd, got %v", term.Percent)
	}
	if term.StartedAt != startedAt {
		t.Fatalf("progEnd's terminal failure event StartedAt = %d, want %d", term.StartedAt, startedAt)
	}
}
