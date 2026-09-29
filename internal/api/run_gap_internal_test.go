package api

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

func fastLockPolls(t *testing.T) {
	t.Helper()
	probe, drill := probeLockPoll, drillLockPoll
	probeLockPoll, drillLockPoll = 5*time.Millisecond, 5*time.Millisecond
	t.Cleanup(func() { probeLockPoll, drillLockPoll = probe, drill })
}

// A scheduled run frees its domain between two items. A probe queued by the
// first of them waits for the whole run, prune and copy included.
func TestAProbeWaitsUntilTheScheduledRunOfItsDomainCloses(t *testing.T) {
	fastLockPolls(t)
	s := &Service{repoMu: map[string]*sync.Mutex{"containers": {}, "vms": {}}}
	closeRun := s.OpenScheduledRun("containers")
	s.OpenScheduledRun("vms")

	got := make(chan func(), 1)
	go func() {
		unlock, ok := s.waitProbeLock("containers")
		if ok {
			got <- unlock
		}
	}()
	select {
	case <-got:
		t.Fatal("the probe took the domain while its scheduled run was open")
	case <-time.After(50 * time.Millisecond):
	}
	closeRun()
	select {
	case unlock := <-got:
		unlock()
	case <-time.After(2 * time.Second):
		t.Fatal("the probe never took the domain after the run closed")
	}
}

type countingPruneEngine struct {
	ResticEngine
	pruned atomic.Int32
}

func (e *countingPruneEngine) Unlock(context.Context, string, bool, restic.Mode) error { return nil }

func (e *countingPruneEngine) Prune(context.Context, string, restic.Mode) error {
	e.pruned.Add(1)
	return nil
}

// Whatever holds the domain when a scheduled run ends, the batched prune waits
// for it rather than giving up with the domain busy.
func TestTheBatchedPruneWaitsForTheDomain(t *testing.T) {
	fastLockPolls(t)
	eng := &countingPruneEngine{}
	s, _ := newProbeService(t, &probeEngine{})
	s.engine = eng
	s.repoMu = map[string]*sync.Mutex{"containers": {}}
	settings, err := s.store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.RetentionKeepDaily = 7
	if err := s.store.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(s.cfg.HostMountRoot, "backups", "containers")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	unlock, ok := s.tryLockDomainFor("containers", "verify")
	if !ok {
		t.Fatal("could not take the domain")
	}
	done := make(chan struct{})
	go func() {
		s.PruneAfterBulk(context.Background(), "containers")
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	if eng.pruned.Load() != 0 {
		t.Fatal("the prune ran while the domain was held")
	}
	unlock()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the prune never ran after the domain was free")
	}
	if eng.pruned.Load() != 1 {
		t.Fatalf("pruned %d times, want once", eng.pruned.Load())
	}
}
