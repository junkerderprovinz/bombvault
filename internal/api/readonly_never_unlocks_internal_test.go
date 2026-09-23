package api

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// lockOnceEngine fails the first listing with a lock error. listSnapshots then
// clears the lock and retries, which is right for our own repositories, where
// an interrupted run leaves a stale lock. On a foreign repository a lock
// usually means its owner is backing up, so callers that only read (foreign
// restore, the receiver, the pull) set Mode.NoLock and must get the error.
type lockOnceEngine struct {
	ResticEngine
	calls   int
	unlocks []string
}

func (e *lockOnceEngine) Snapshots(_ context.Context, _ string, _ restic.Mode) ([]restic.Snapshot, error) {
	e.calls++
	if e.calls == 1 {
		return nil, errors.New("Fatal: unable to create lock in backend: repository is already locked by PID 4711")
	}
	return []restic.Snapshot{{ID: "abc"}}, nil
}

func (e *lockOnceEngine) Unlock(_ context.Context, repo string, _ bool, _ restic.Mode) error {
	e.unlocks = append(e.unlocks, repo)
	return nil
}

func TestListSnapshotsNeverUnlocksAReadOnlyRepository(t *testing.T) {
	t.Run("a read-only caller gets the lock error, and nothing is written", func(t *testing.T) {
		eng := &lockOnceEngine{}
		svc := &Service{engine: eng}

		_, err := svc.listSnapshots(context.Background(), "rest:http://far:8000/repo", restic.Mode{NoLock: true})

		if len(eng.unlocks) != 0 {
			t.Fatalf("a NoLock caller must not unlock anything, got %v.\n"+
				"That is a write into somebody else's repository, and the two callers that set\n"+
				"NoLock both tell the operator on screen that this box only reads.", eng.unlocks)
		}
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "already locked") {
			t.Fatalf("the lock error must reach the caller instead of being repaired away, got %v", err)
		}
		if eng.calls != 1 {
			t.Fatalf("a read-only listing must not retry either, got %d listings", eng.calls)
		}
	})

	t.Run("our own repository still self-heals", func(t *testing.T) {
		// Without this half, deleting the self-heal altogether would pass too.
		eng := &lockOnceEngine{}
		svc := &Service{engine: eng}

		snaps, err := svc.listSnapshots(context.Background(), "/mnt/user/backups/containers", restic.Mode{})

		if err != nil {
			t.Fatalf("a stale lock on our own repository must still be cleared and the listing retried: %v", err)
		}
		if len(eng.unlocks) != 1 {
			t.Fatalf("expected exactly one unlock, got %v", eng.unlocks)
		}
		if len(snaps) != 1 {
			t.Fatalf("the retry must return the listing, got %v", snaps)
		}
	})
}
