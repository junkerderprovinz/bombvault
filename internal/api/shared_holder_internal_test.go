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
// repository its keyring stands for. appendOnly reaches the repository
// through its append-only server, which removes no key.
type identifiedEngine struct {
	*keyringEngine
	id         string
	appendOnly bool
}

func (e *identifiedEngine) RemoveKey(ctx context.Context, repo string, m restic.Mode, id string) error {
	if e.appendOnly {
		return errors.New("Fatal: unexpected HTTP response (403): 403 Forbidden")
	}
	return e.keyringEngine.RemoveKey(ctx, repo, m, id)
}

// proofKeys counts the keys left from checks of a holder in ring.
func proofKeys(ring *keyring) int {
	n := 0
	for _, h := range ring.hosts() {
		if strings.HasSuffix(h, "/"+sharedProofUser) {
			n++
		}
	}
	return n
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
// hear of it. cubeSees is the id of the repository cube reaches there, and
// cubeAtFolder whether it sits at the repository's folder or reaches it only
// through the append-only server like tower.
func heldByCube(t *testing.T, cubeSees string, cubeAtFolder bool) (nas, tower, cube *instance, sr store.SharedRepo, ring *keyring) {
	t.Helper()
	nas, tower, ring = sharedPair(t)
	setUpShared(t, nas)
	joined(t, nas, tower)
	nas.svc.engine = &identifiedEngine{keyringEngine: nas.svc.engine.(*keyringEngine), id: "5f3c0a1e"}
	tower.svc.engine = &identifiedEngine{keyringEngine: tower.svc.engine.(*keyringEngine), id: "5f3c0a1e", appendOnly: true}
	cube = newInstance(t, "cube", strings.Repeat("c3", 32))
	joinThroughRelay(t, cube, nas, nas, tower)
	if _, err := cube.st.MutateSettings(func(s *store.Settings) error {
		s.EncryptionEnabled = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	ring.add(cube.svc.backupPassword(), cube.svc.resticHost(), "cube")
	cube.svc.engine = &identifiedEngine{keyringEngine: &keyringEngine{ResticEngine: cube.engine, ring: ring}, id: cubeSees, appendOnly: !cubeAtFolder}
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
	return nas, tower, cube, sr, ring
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
	nas, tower, cube, sr, ring := heldByCube(t, "5f3c0a1e", true)
	towerRepo := joinOf(t, tower, store.SharedJoinOut, nas).Repo

	cube.svc.tellSharedHolders(context.Background())

	if n := proofKeys(ring); n != 0 {
		t.Errorf("%d keys of the checks are left", n)
	}

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
	nas, tower, cube, _, _ := heldByCube(t, "9d8c7b6a", true)
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
	nas, tower, cube, sr, _ := heldByCube(t, "5f3c0a1e", true)
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

func TestAServerAwayFromTheFolderCannotClaimTheSharedRepository(t *testing.T) {
	nas, tower, cube, _, ring := heldByCube(t, "5f3c0a1e", false)
	towerRepo := joinOf(t, tower, store.SharedJoinOut, nas).Repo
	ctx := context.Background()

	cube.svc.tellSharedHolders(ctx)

	if got := ownerAt(t, tower, towerRepo).Owner; got != nas.id(t) {
		t.Errorf("tower took cube as the holder, which cannot remove a key at the folder")
	}
	held := ownerAt(t, nas, "shared/shared")
	if !nas.svc.ownsSharedRepo(held) {
		t.Fatal("nas gave its repository up to a server away from the folder")
	}

	// cube answers the check without having removed anything.
	for _, to := range []*instance{nas, tower} {
		first, err := cube.svc.announceHolder(ctx, to.id(t), "5f3c0a1e", "")
		if err != nil || first.Challenge == "" {
			t.Fatalf("no check from %s: %+v, %v", to.id(t), first, err)
		}
		if ans, err := cube.svc.announceHolder(ctx, to.id(t), "5f3c0a1e", first.Challenge); err != nil || ans.Taken {
			t.Errorf("%s took a check whose key is still there: %+v, %v", to.id(t), ans, err)
		}
	}
	if got := ownerAt(t, tower, towerRepo).Owner; got != nas.id(t) {
		t.Errorf("tower names %s as the holder, want nas", got)
	}
	if !nas.svc.ownsSharedRepo(ownerAt(t, nas, "shared/shared")) {
		t.Error("nas gave its repository up")
	}
	if left, _ := cube.st.SharedHolderNotices(); len(left) == 0 {
		t.Error("cube forgot whom it has still to tell")
	}

	// tower cannot remove the keys it made through the append-only server, so
	// they stay until nas, at the folder, clears them in its upkeep.
	if n := proofKeys(ring); n != 1 {
		t.Fatalf("%d keys of checks are left, want tower's one: asking again yields the same check", n)
	}
	holdings, err := nas.svc.sharedHoldings(ctx)
	if err != nil || len(holdings) != 1 || holdings[0].ProofKeys != proofKeys(ring) {
		t.Fatalf("nas shows %+v (%v), want the %d keys left", holdings, err, proofKeys(ring))
	}
	restore := sharedProofKeyAge
	sharedProofKeyAge = 0
	t.Cleanup(func() { sharedProofKeyAge = restore })
	nas.svc.clearProofKeys(ctx)
	if n := proofKeys(ring); n != 0 {
		t.Errorf("%d keys of the checks are left after nas's upkeep", n)
	}
}
