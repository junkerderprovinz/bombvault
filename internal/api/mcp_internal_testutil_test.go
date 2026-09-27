package api

import (
	"context"
	"sync"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// blockingSnapshotsEngine holds every listing until its release channel closes
// or the caller's context ends. The shared fakeResticEngine lives in package
// api_test and cannot be reached from here.
//
// Everything but Snapshots comes from the embedded nil interface, so a call
// nothing in these tests makes panics instead of answering.
type blockingSnapshotsEngine struct {
	ResticEngine

	release chan struct{}
	// entered reports that a listing is inside the engine, so a test can take
	// its next step instead of sleeping.
	entered chan struct{}

	mu        sync.Mutex
	calls     int
	cancelled bool
}

func newBlockingSnapshotsEngine() *blockingSnapshotsEngine {
	return &blockingSnapshotsEngine{release: make(chan struct{}), entered: make(chan struct{}, 1)}
}

func (e *blockingSnapshotsEngine) Snapshots(ctx context.Context, _ string, _ restic.Mode) ([]restic.Snapshot, error) {
	e.mu.Lock()
	e.calls++
	e.mu.Unlock()
	select {
	case e.entered <- struct{}{}:
	default:
	}

	select {
	case <-e.release:
		return nil, nil
	case <-ctx.Done():
		e.mu.Lock()
		e.cancelled = true
		e.mu.Unlock()
		return nil, ctx.Err()
	}
}

func (e *blockingSnapshotsEngine) callCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.calls
}

func (e *blockingSnapshotsEngine) sawCancellation() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cancelled
}
