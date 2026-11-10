package api

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const sharedTarget = "rest:http://nas:8001/shared"

func TestNoCopyStartsWhileTheOwnerPrunesAfterItWasTold(t *testing.T) {
	var c sharedCopies
	now := time.Unix(1000, 0)
	if busy := c.hush("nas-id", []string{sharedTarget}, now.Add(time.Hour)); busy {
		t.Fatal("busy without a copy")
	}
	if _, ok := c.begin(sharedTarget, "nas-id", now.Add(30*time.Minute)); ok {
		t.Fatal("a copy started inside the prune window")
	}
	release, ok := c.begin(sharedTarget, "nas-id", now.Add(2*time.Hour))
	if !ok {
		t.Fatal("no copy started once the window was over")
	}
	release()
}

func TestTheOwnerHearsOfACopyStillRunningAndItGoesOn(t *testing.T) {
	var c sharedCopies
	now := time.Unix(1000, 0)
	release, ok := c.begin(sharedTarget, "nas-id", now)
	if !ok {
		t.Fatal("the copy did not start")
	}
	if busy := c.hush("nas-id", []string{sharedTarget}, now.Add(time.Hour)); !busy {
		t.Fatal("the owner did not hear of the running copy")
	}
	second, ok := c.begin(sharedTarget, "nas-id", now)
	if !ok {
		t.Fatal("a member that said it is busy promised to start nothing")
	}
	second()
	release()
	release()
	if busy := c.hush("nas-id", []string{sharedTarget}, now.Add(time.Hour)); busy {
		t.Fatal("busy after every copy ended, a release counted twice")
	}
}

func TestCopiesStartAgainOnceTheOwnerIsDone(t *testing.T) {
	var c sharedCopies
	now := time.Unix(1000, 0)
	c.hush("nas-id", []string{sharedTarget}, now.Add(time.Hour))
	c.lift("nas-id")
	release, ok := c.begin(sharedTarget, "nas-id", now)
	if !ok {
		t.Fatal("no copy started after the owner was done")
	}
	release()
}

func TestAnotherOwnersPruneHoldsUpNoCopy(t *testing.T) {
	var c sharedCopies
	now := time.Unix(1000, 0)
	c.hush("cube-id", []string{"rest:http://cube:8001/shared"}, now.Add(time.Hour))
	release, ok := c.begin(sharedTarget, "nas-id", now)
	if !ok {
		t.Fatal("the prune of another repository held the copy up")
	}
	release()
}

// The owner's word and a copy that starts at the same time meet under one
// lock: either the copy runs and the owner hears it, or the copy waits.
func TestACopyAndAnAnnouncedPruneNeverOverlap(t *testing.T) {
	var c sharedCopies
	now := time.Unix(1000, 0)
	var wg sync.WaitGroup
	var mu sync.Mutex
	running := 0
	for range 8 {
		wg.Go(func() {
			for range 200 {
				release, ok := c.begin(sharedTarget, "nas-id", now)
				if !ok {
					continue
				}
				mu.Lock()
				running++
				mu.Unlock()
				mu.Lock()
				running--
				mu.Unlock()
				release()
			}
		})
	}
	for range 200 {
		if !c.hush("nas-id", []string{sharedTarget}, now.Add(time.Hour)) {
			mu.Lock()
			if running != 0 {
				mu.Unlock()
				t.Fatalf("%d copies ran while the owner was told none did", running)
			}
			mu.Unlock()
			c.lift("nas-id")
		}
	}
	wg.Wait()
}

// copyCountEngine counts what an off-site pass does to its destination.
type copyCountEngine struct {
	ResticEngine
	copies, forgets, prunes int
}

func (e *copyCountEngine) RepoOpens(context.Context, string, restic.Mode) bool { return true }

func (e *copyCountEngine) Unlock(context.Context, string, bool, restic.Mode) error { return nil }

// Snapshots answers with two snapshots of this server in its own folder and
// none in the shared repository, so each pass has something to send there.
func (e *copyCountEngine) Snapshots(_ context.Context, repo string, m restic.Mode) ([]restic.Snapshot, error) {
	if repo != "/local" {
		return nil, nil
	}
	return []restic.Snapshot{snapAt("t1", m.Host, "container:plex", 1), snapAt("t2", m.Host, "container:plex", 2)}, nil
}

func (e *copyCountEngine) Copy(context.Context, string, string, []string, restic.Limits, restic.Mode) error {
	e.copies++
	return nil
}

func (e *copyCountEngine) ForgetPolicy(context.Context, string, restic.RetentionPolicy, restic.Mode, []string, bool) error {
	e.forgets++
	return nil
}

func (e *copyCountEngine) Prune(context.Context, string, restic.Mode) error {
	e.prunes++
	return nil
}

func sharedCopyService(t *testing.T, owner string) (*Service, *copyCountEngine, store.OffsiteTarget) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	eng := &copyCountEngine{}
	s := &Service{store: st, engine: eng, progress: progress.NewStore()}
	s.cfg.HostMountRoot = t.TempDir()
	s.cfg.AppKey = strings.Repeat("ab", 32)
	if owner == "" {
		g, err := st.GetGroupState()
		if err != nil {
			t.Fatal(err)
		}
		owner = g.InstanceID
	}
	target := store.OffsiteTarget{ID: "t1", Domain: "containers", Repo: "backups/shared", Enabled: true, RetentionKeepLast: 1}
	if _, err := st.PutSharedRepo(store.SharedRepo{Repo: target.Repo, Owner: owner}); err != nil {
		t.Fatal(err)
	}
	return s, eng, target
}

func copyOnce(t *testing.T, s *Service, target store.OffsiteTarget) {
	t.Helper()
	settings, err := s.store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.copyToOffsiteTarget(context.Background(), "containers", settings, target, []domainRepoRef{ownRef("/local")}, nil, false, time.Now().Unix(), nil, targetVisit{}); err != nil && !errors.Is(err, errCopyDeferred) {
		t.Fatalf("a copy the owner holds off is no failure: %v", err)
	}
}

func TestAMemberCopiesNothingWhileTheOwnerPrunesAndCatchesUpAfter(t *testing.T) {
	s, eng, target := sharedCopyService(t, "nas-id")
	s.sharedCopies.hush("nas-id", []string{target.Repo}, time.Now().Add(time.Hour))
	copyOnce(t, s, target)
	if eng.copies != 0 {
		t.Fatalf("%d copies while the owner prunes, want none", eng.copies)
	}
	s.sharedCopies.lift("nas-id")
	copyOnce(t, s, target)
	if eng.copies != 1 {
		t.Fatalf("%d copies once the owner was done, want one", eng.copies)
	}
}

// The owner ages a shared repository in its upkeep, by rules that cover every
// member, so a copy there never runs a keep-policy of its own.
func TestACopyToASharedRepositoryAgesNothingThere(t *testing.T) {
	for name, owner := range map[string]string{"member": "nas-id", "owner": ""} {
		t.Run(name, func(t *testing.T) {
			s, eng, target := sharedCopyService(t, owner)
			copyOnce(t, s, target)
			if eng.copies != 1 || eng.forgets != 0 || eng.prunes != 0 {
				t.Fatalf("copies %d, forgets %d, prunes %d; want one copy and nothing aged", eng.copies, eng.forgets, eng.prunes)
			}
		})
	}
}
