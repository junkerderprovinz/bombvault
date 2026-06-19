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
	// Terminal event clears the key from the snapshot.
	s.Publish(progress.Event{Key: "vm:win", Phase: "backup", Percent: 100, Active: false})
	if snap := s.Snapshot(); len(snap) != 0 {
		t.Fatalf("terminal event should clear the key, got %+v", snap)
	}
}

func TestStoreCancelUnsubscribes(t *testing.T) {
	s := progress.NewStore()
	ch, cancel := s.Subscribe()
	cancel()
	// Publishing after cancel must not panic (channel closed, removed from subs).
	s.Publish(progress.Event{Key: "flash", Phase: "backup", Percent: 1, Active: true})
	// Double cancel must be safe.
	cancel()
	// Draining a closed channel returns the zero value with ok=false.
	if _, ok := <-ch; ok {
		t.Fatal("channel should be closed after cancel")
	}
}
