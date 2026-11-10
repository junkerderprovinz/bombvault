package api

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// identifiedEngine is an instance's engine in a test with the id of the
// repository its keyring stands for.
type identifiedEngine struct {
	*keyringEngine
	id string
}

func (e *identifiedEngine) RepoID(_ context.Context, _ string, m restic.Mode) (string, error) {
	e.ring.mu.Lock()
	defer e.ring.mu.Unlock()
	if !e.ring.opens(m.Password) {
		return "", errors.New("Fatal: wrong password or no key found")
	}
	return e.id, nil
}

// heldByCube is nas's shared repository with tower in it, and cube, which took
// nas's kit over and holds the repository now, with tower and nas still to
// hear of it. cubeSees is the id of the repository cube reaches there.
func heldByCube(t *testing.T, cubeSees string) (nas, tower, cube *instance, sr store.SharedRepo) {
	t.Helper()
	nas, tower, ring := sharedPair(t)
	setUpShared(t, nas)
	joined(t, nas, tower)
	for _, in := range []*instance{nas, tower} {
		in.svc.engine = &identifiedEngine{keyringEngine: in.svc.engine.(*keyringEngine), id: "5f3c0a1e"}
	}
	cube = newInstance(t, "cube", strings.Repeat("c3", 32))
	joinThroughRelay(t, cube, nas, nas, tower)
	if _, err := cube.st.MutateSettings(func(s *store.Settings) error {
		s.EncryptionEnabled = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	ring.add(cube.svc.backupPassword(), cube.svc.resticHost(), "cube")
	cube.svc.engine = &identifiedEngine{keyringEngine: &keyringEngine{ResticEngine: cube.engine, ring: ring}, id: cubeSees}
	at, _, err := nas.st.SharedRepoAt("shared/shared")
	if err != nil {
		t.Fatal(err)
	}
	sr, err = cube.st.PutSharedRepo(store.SharedRepo{Repo: "shared/shared", Owner: cube.id(t), Members: at.Members})
	if err != nil {
		t.Fatal(err)
	}
	if err := cube.st.QueueSharedHolderNotices(sr.ID, []string{tower.id(t), nas.id(t)}); err != nil {
		t.Fatal(err)
	}
	return nas, tower, cube, sr
}

func ownerAt(t *testing.T, in *instance, repo string) store.SharedRepo {
	t.Helper()
	sr, ok, err := in.st.SharedRepoAt(repo)
	if err != nil || !ok {
		t.Fatalf("no shared repository at %s: %v", repo, err)
	}
	return sr
}

func TestTheServersOfASharedRepositoryHearOfItsNewHolder(t *testing.T) {
	nas, tower, cube, sr := heldByCube(t, "5f3c0a1e")
	towerRepo := joinOf(t, tower, store.SharedJoinOut, nas).Repo

	cube.svc.tellSharedHolders(context.Background())

	if got := ownerAt(t, tower, towerRepo).Owner; got != cube.id(t) {
		t.Errorf("tower names %s as the holder, want cube", got)
	}
	if left, err := cube.st.SharedHolderNotices(); err != nil || len(left) != 0 {
		t.Errorf("still to tell: %+v, %v", left, err)
	}
	// Only cube forgets and prunes there from now on.
	held := ownerAt(t, nas, "shared/shared")
	if err := nas.svc.keepSharedRepo(context.Background(), held, &fakeMembers{}, time.Now(), true); !errors.Is(err, errNotSharedOwner) {
		t.Errorf("nas still keeps the repository: %v", err)
	}
	if !cube.svc.ownsSharedRepo(ownerAt(t, cube, sr.Repo)) {
		t.Error("cube does not hold the repository")
	}
	taken, ok, err := nas.st.SharedTakenOver(held.ID)
	if err != nil || !ok || taken.By != cube.id(t) || taken.Name != "cube" {
		t.Errorf("nas does not say who took its repository over: %+v, %v, %v", taken, ok, err)
	}
}

func TestAnAnnouncementOfAnotherRepositoryChangesNothing(t *testing.T) {
	nas, tower, cube, _ := heldByCube(t, "9d8c7b6a")
	towerRepo := joinOf(t, tower, store.SharedJoinOut, nas).Repo

	cube.svc.tellSharedHolders(context.Background())

	if got := ownerAt(t, tower, towerRepo).Owner; got != nas.id(t) {
		t.Errorf("tower names %s as the holder, want nas still", got)
	}
	held := ownerAt(t, nas, "shared/shared")
	if !nas.svc.ownsSharedRepo(held) {
		t.Error("nas gave its repository up for an announcement of another one")
	}
	if _, ok, _ := nas.st.SharedTakenOver(held.ID); ok {
		t.Error("nas says its repository was taken over")
	}
}

func TestAServerTakesItsSharedRepositoryBack(t *testing.T) {
	nas, tower, cube, sr := heldByCube(t, "5f3c0a1e")
	towerRepo := joinOf(t, tower, store.SharedJoinOut, nas).Repo
	cube.svc.tellSharedHolders(context.Background())
	held := ownerAt(t, nas, "shared/shared")

	if err := nas.svc.TakeBackSharedRepo(context.Background(), held.ID); err != nil {
		t.Fatal(err)
	}

	if !nas.svc.ownsSharedRepo(ownerAt(t, nas, "shared/shared")) {
		t.Error("nas does not hold its repository again")
	}
	if _, ok, _ := nas.st.SharedTakenOver(held.ID); ok {
		t.Error("nas still says its repository was taken over")
	}
	if got := ownerAt(t, tower, towerRepo).Owner; got != nas.id(t) {
		t.Errorf("tower names %s as the holder, want nas", got)
	}
	if cube.svc.ownsSharedRepo(ownerAt(t, cube, sr.Repo)) {
		t.Error("cube still holds the repository")
	}
	if taken, ok, _ := cube.st.SharedTakenOver(sr.ID); !ok || taken.By != nas.id(t) {
		t.Errorf("cube does not say nas took the repository back: %+v", taken)
	}
}
