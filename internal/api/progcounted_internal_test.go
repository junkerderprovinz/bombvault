package api

import (
	"context"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/progress"
)

func TestRemainingSecondsWaitsForEnoughToGoBy(t *testing.T) {
	if got := remainingSeconds(2*time.Second, 10, 100); got != 0 {
		t.Fatalf("after 2s = %d, want unknown", got)
	}
	if got := remainingSeconds(10*time.Second, 0, 100); got != 0 {
		t.Fatalf("with nothing done = %d, want unknown", got)
	}
	if got := remainingSeconds(10*time.Second, 25, 100); got != 30 {
		t.Fatalf("25 of 100 in 10s = %d, want 30", got)
	}
	if got := remainingSeconds(10*time.Second, 100, 100); got != 0 {
		t.Fatalf("a finished step = %d, want 0", got)
	}
}

func TestProgCountedPublishesTheCounts(t *testing.T) {
	store := progress.NewStore()
	s := &Service{progress: store}
	ch, cancel := store.Subscribe()
	defer cancel()
	ctx, stop := s.progCounted(context.Background(), "verify:containers", 1000)
	defer stop()
	progress.CountSinkFrom(ctx)(progress.CountProgress{Done: 12, Total: 47, Unit: "packs"})

	ev := <-ch
	if ev.Key != "verify:containers" || ev.Done != 12 || ev.Total != 47 || ev.Unit != "packs" || !ev.Active || ev.StartedAt != 1000 {
		t.Fatalf("event = %+v", ev)
	}
	if ev.Percent < 25 || ev.Percent > 26 {
		t.Fatalf("percent = %v", ev.Percent)
	}
}

func TestProgCountedKeepsASilentStepAlive(t *testing.T) {
	old := countHeartbeat
	countHeartbeat = 10 * time.Millisecond
	defer func() { countHeartbeat = old }()
	store := progress.NewStore()
	s := &Service{progress: store}
	ch, cancel := store.Subscribe()
	defer cancel()

	ctx, stop := s.progCounted(context.Background(), "prune:vms", 1000)
	progress.CountSinkFrom(ctx)(progress.CountProgress{Done: 3, Total: 9, Unit: "packs"})
	first := <-ch
	again := <-ch
	if again.Key != "prune:vms" || !again.Active || again.Done != first.Done || again.Total != first.Total {
		t.Fatalf("heartbeat = %+v, want the last count again", again)
	}

	stop()
	for len(ch) > 0 {
		<-ch
	}
	time.Sleep(50 * time.Millisecond)
	if len(ch) != 0 {
		t.Fatalf("an event came after stop: %+v", <-ch)
	}
}
