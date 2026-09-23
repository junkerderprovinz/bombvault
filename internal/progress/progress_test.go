package progress_test

import (
	"context"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/progress"
)

func TestSinkRoundTrip(t *testing.T) {
	var got float64
	ctx := progress.WithSink(context.Background(), func(p float64) { got = p })
	sink := progress.SinkFrom(ctx)
	if sink == nil {
		t.Fatal("SinkFrom returned nil for a context with a sink")
	}
	sink(42.5)
	if got != 42.5 {
		t.Fatalf("sink got %v, want 42.5", got)
	}
}

func TestSinkFromEmptyContext(t *testing.T) {
	if progress.SinkFrom(context.Background()) != nil {
		t.Fatal("SinkFrom should be nil when no sink is set")
	}
}

func TestWithSinkNilIsNoop(t *testing.T) {
	ctx := progress.WithSink(context.Background(), nil)
	if progress.SinkFrom(ctx) != nil {
		t.Fatal("WithSink(nil) must not install a sink")
	}
}

func TestStorePublishReachesSubscriber(t *testing.T) {
	s := progress.NewStore()
	ch, cancel := s.Subscribe()
	defer cancel()

	want := progress.Event{Key: "container:plex", Phase: "backup", Percent: 10, Active: true}
	s.Publish(want)

	select {
	case got := <-ch:
		if got != want {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	default:
		t.Fatal("subscriber did not receive the published event")
	}
}

func TestStoreSnapshotKeepsActiveDropsTerminal(t *testing.T) {
	s := progress.NewStore()
	s.Publish(progress.Event{Key: "vm:win", Phase: "backup", Percent: 50, Active: true})
	if snap := s.Snapshot(); len(snap) != 1 || snap[0].Key != "vm:win" {
		t.Fatalf("active event should be in snapshot, got %+v", snap)
	}
	s.Publish(progress.Event{Key: "vm:win", Phase: "backup", Percent: 100, Active: false})
	if snap := s.Snapshot(); len(snap) != 0 {
		t.Fatalf("terminal event should clear the key, got %+v", snap)
	}
}

func TestCopySinkRoundTrip(t *testing.T) {
	var got progress.CopyProgress
	ctx := progress.WithCopySink(context.Background(), func(p progress.CopyProgress) { got = p })
	sink := progress.CopySinkFrom(ctx)
	if sink == nil {
		t.Fatal("CopySinkFrom returned nil for a context with a copy sink")
	}
	sink(progress.CopyProgress{SnapshotIndex: 2, Percent: 63.5})
	if got.SnapshotIndex != 2 || got.Percent != 63.5 {
		t.Fatalf("sink got %+v, want SnapshotIndex=2 Percent=63.5", got)
	}
}

func TestCopySinkFromEmptyContext(t *testing.T) {
	if progress.CopySinkFrom(context.Background()) != nil {
		t.Fatal("CopySinkFrom should be nil when no copy sink is set")
	}
}

func TestWithCopySinkNilIsNoop(t *testing.T) {
	ctx := progress.WithCopySink(context.Background(), nil)
	if progress.CopySinkFrom(ctx) != nil {
		t.Fatal("WithCopySink(nil) must not install a sink")
	}
}

func TestCopySinkDoesNotLeakIntoPlainSink(t *testing.T) {
	ctx := progress.WithCopySink(context.Background(), func(progress.CopyProgress) {})
	if progress.SinkFrom(ctx) != nil {
		t.Fatal("a context carrying only a CopySink must not yield a plain Sink")
	}
	ctx2 := progress.WithSink(context.Background(), func(float64) {})
	if progress.CopySinkFrom(ctx2) != nil {
		t.Fatal("a context carrying only a plain Sink must not yield a CopySink")
	}
}

func TestEventStartedAtRoundTrip(t *testing.T) {
	s := progress.NewStore()
	ch, cancel := s.Subscribe()
	defer cancel()

	want := progress.Event{Key: "offsite:files", Phase: "replicate", Percent: 12, Active: true, StartedAt: 1_700_000_000}
	s.Publish(want)

	select {
	case got := <-ch:
		if got.StartedAt != want.StartedAt {
			t.Fatalf("got StartedAt %d, want %d", got.StartedAt, want.StartedAt)
		}
	default:
		t.Fatal("subscriber did not receive the published event")
	}
}

// TestEventStartedAtSurvivesTerminalEvent checks that the final event keeps
// the run's StartedAt. Snapshot holds only active events, so the final event
// is read from a subscriber.
func TestEventStartedAtSurvivesTerminalEvent(t *testing.T) {
	s := progress.NewStore()
	ch, cancel := s.Subscribe()
	defer cancel()

	s.Publish(progress.Event{Key: "offsite:files", Phase: "replicate", Percent: 12, Active: true, StartedAt: 1_700_000_000})
	<-ch // begin event

	s.Publish(progress.Event{Key: "offsite:files", Phase: "replicate", Percent: 100, Active: false, StartedAt: 1_700_000_000})
	term := <-ch
	if term.Active {
		t.Fatal("expected the terminal event to be Active:false")
	}
	if term.StartedAt != 1_700_000_000 {
		t.Fatalf("terminal event StartedAt = %d, want 1700000000 (same as the begin event)", term.StartedAt)
	}
}

func TestStoreCancelUnsubscribes(t *testing.T) {
	s := progress.NewStore()
	ch, cancel := s.Subscribe()
	cancel()
	// Neither a publish after cancel nor a second cancel may panic.
	s.Publish(progress.Event{Key: "flash", Phase: "backup", Percent: 1, Active: true})
	cancel()
	if _, ok := <-ch; ok {
		t.Fatal("channel should be closed after cancel")
	}
}
