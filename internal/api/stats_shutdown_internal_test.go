package api

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// walkingStatsEngine walks in Stats until its context ends and says when a
// walk has begun.
type walkingStatsEngine struct {
	slowStatsEngine
	entered chan struct{}
	walking atomic.Int32
}

func (e *walkingStatsEngine) Stats(ctx context.Context, repo, mode string, m restic.Mode) (restic.StatsResult, error) {
	e.walking.Add(1)
	defer e.walking.Add(-1)
	e.entered <- struct{}{}
	return e.slowStatsEngine.Stats(ctx, repo, mode, m)
}

// stopDuringSample starts a background sample, stops the service while the
// sample walks the repository and waits for the samples, all under a deadline.
func stopDuringSample(t *testing.T, svc *Service, eng *walkingStatsEngine) {
	t.Helper()
	svc.maybeCollectStats(context.Background(), "containers")
	select {
	case <-eng.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("the sample never reached the engine")
	}
	svc.BeginShutdown()
	waited := make(chan struct{})
	go func() {
		svc.WaitForSamples()
		close(waited)
	}()
	select {
	case <-waited:
	case <-time.After(10 * time.Second):
		t.Fatal("waiting for the samples outlasted the stop")
	}
}

func TestStoppingEndsARunningSampleBeforeTheWaitReturns(t *testing.T) {
	eng := &walkingStatsEngine{entered: make(chan struct{}, 1)}
	svc, _ := statsTestService(t, eng)

	stopDuringSample(t, svc, eng)

	if n := eng.walking.Load(); n != 0 {
		t.Fatalf("%d sample(s) still walk the repository after the wait", n)
	}
}

func TestASampleTheStopEndedIsNotLoggedAsAFailure(t *testing.T) {
	eng := &walkingStatsEngine{entered: make(chan struct{}, 1)}
	svc, _ := statsTestService(t, eng)

	out := logOf(func() { stopDuringSample(t, svc, eng) })

	if strings.Contains(out, "failed") {
		t.Fatalf("a sample the stop ended reads as a failure:\n%s", out)
	}
}
