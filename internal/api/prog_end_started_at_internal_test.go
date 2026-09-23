package api

import (
	"context"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/progress"
)

// Without StartedAt on the terminal event the live duration in the UI
// disappears while that event lingers.
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
