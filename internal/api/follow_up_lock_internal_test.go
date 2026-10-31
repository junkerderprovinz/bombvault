package api

import (
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// probeHolds holds domain the way a first-backup probe does until somebody
// asks for it, and reports whether it was asked to stop.
func probeHolds(t *testing.T, s *Service, domain string) *atomic.Bool {
	t.Helper()
	stopped := &atomic.Bool{}
	var unlock func()
	unlock, ok := s.tryLockFollowUp(domain, "verify", func() {
		if stopped.CompareAndSwap(false, true) {
			go unlock()
		}
	})
	if !ok {
		t.Fatalf("%s is taken already", domain)
	}
	t.Cleanup(func() {
		if !stopped.Load() {
			unlock()
		}
	})
	return stopped
}

func TestEveryActionOnASectionStopsAProbeAndGoesThrough(t *testing.T) {
	for name, act := range map[string]func(f *placementFixture) error{
		"moving the own folder": func(f *placementFixture) error {
			vault := f.destination("Vault", vaultRepo)
			res := f.do("POST", "/api/storage/own-folders", map[string]any{"domains": []string{"containers"}, "location": "destination:" + vault.ID})
			if res["ok"] != true {
				return errors.New(res["error"].(string))
			}
			return nil
		},
		"choosing where an entry backs up": func(f *placementFixture) error {
			nas := f.namedRepo("NAS", "nas")
			f.openContainer("nginx")
			_, err := f.svc.writeItemPlacement(context.Background(), store.ItemRef{Domain: "containers", Key: "nginx"}, placementChange{Home: &homeChoice{Repo: &nas.ID}})
			return err
		},
		"applying the default": func(f *placementFixture) error {
			_, _, err := f.svc.applyDefault(context.Background(), "containers", nil)
			return err
		},
		"removing a storage location": func(f *placementFixture) error {
			box := f.destination("Box", boxRepo)
			f.tickDestination(box, "containers")
			if res := f.remove(box, f.removalPreview(box)); res["ok"] != true {
				return errors.New(res["error"].(string))
			}
			return nil
		},
		"replicating now": func(f *placementFixture) error {
			f.target("containers", "Box", "s3:box/containers")
			if err := f.svc.StartReplicateOffsite("containers"); err != nil {
				return err
			}
			for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
				if unlock, ok := f.svc.tryLockDomainFor("containers", "test"); ok {
					unlock()
					return nil
				}
			}
			return errors.New("the replication never let go")
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newPlacementFixture(t)
			stopped := probeHolds(t, f.svc, "containers")

			if err := act(f); err != nil {
				t.Fatalf("%s while a probe held the section = %v, want it done", name, err)
			}
			if !stopped.Load() {
				t.Fatal("the probe was not asked to give way")
			}
		})
	}
}

func TestASectionHeldByABackupRefusesAtOnceAndSaysWhatRuns(t *testing.T) {
	f := newPlacementFixture(t)
	vault := f.destination("Vault", vaultRepo)
	unlock, ok := f.svc.tryLockDomainFor("containers", "backup")
	if !ok {
		t.Fatal("containers is taken already")
	}
	defer unlock()

	began := time.Now()
	res := f.do("POST", "/api/storage/own-folders", map[string]any{"domains": []string{"containers"}, "location": "destination:" + vault.ID})

	if res["ok"] != false || res["code"] != "domain-busy" || res["domain"] != "containers" || res["busyWith"] != "backup" {
		t.Fatalf("answer = %v, want domain-busy naming the backup of containers", res)
	}
	if waited := time.Since(began); waited > userLockWait/2 {
		t.Fatalf("the refusal took %v, want it at once while a backup runs", waited)
	}
}

func TestAQueuedProbeKeepsBackWhileAnActionWaits(t *testing.T) {
	fastLockPolls(t)
	s := &Service{repoMu: map[string]*sync.Mutex{"vms": {}}}
	unlockRun, _ := s.tryLockDomainFor("vms", "backup")
	done := s.wantDomain("vms")

	if _, ok := s.tryLockFollowUp("vms", "verify", func() {}); ok {
		t.Fatal("a follow-up took a section an action waits for")
	}
	unlockRun()
	if _, ok := s.tryLockFollowUp("vms", "verify", func() {}); ok {
		t.Fatal("a follow-up took the section before the waiting action")
	}
	done()
	if unlock, ok := s.tryLockFollowUp("vms", "verify", func() {}); !ok {
		t.Fatal("the follow-up never got the section once nobody waited")
	} else {
		unlock()
	}
}

// waitingProbeEngine lists a snapshot only until its probe is asked to stop.
type waitingProbeEngine struct {
	probeEngine
	reading chan struct{}
}

func (e *waitingProbeEngine) LsStream(ctx context.Context, _, _ string, _ restic.Mode, _ func(restic.FileEntry)) error {
	close(e.reading)
	<-ctx.Done()
	return ctx.Err()
}

func TestAProbeThatGaveWayRecordsNothingAndComesBackAfterTheAction(t *testing.T) {
	eng := &waitingProbeEngine{reading: make(chan struct{})}
	s, tg := newProbeService(t, &eng.probeEngine)
	s.engine = eng
	s.repoMu = map[string]*sync.Mutex{"containers": {}}
	s.EnableFirstProbes()
	s.probeWorking = true
	recordBackup(t, s.store, tg.ID, "abcd1234")

	ended := make(chan struct{})
	go func() {
		defer close(ended)
		s.firstProbe(context.Background(), tg.ID)
	}()
	<-eng.reading
	unlock, ok := s.tryLockDomainFor("containers", "replicate")
	if !ok {
		t.Fatal("the action did not get the section from the probe")
	}
	unlock()
	<-ended

	if _, found, err := s.store.LatestItemProbe(tg.ID); err != nil || found {
		t.Fatalf("probe recorded = %v, %v, want none for one that gave way", found, err)
	}
	s.probeMu.Lock()
	queued := slices.Contains(s.probeQueue, tg.ID)
	s.probeMu.Unlock()
	if !queued {
		t.Fatal("the probe that gave way is not queued to run again")
	}
}
