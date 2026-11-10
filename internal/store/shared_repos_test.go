package store_test

import (
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func sharedRepoStore(t *testing.T) *store.Repo {
	t.Helper()
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return store.New(db)
}

func TestASharedRepositoryIsFoundAtItsLocationWithItsMembers(t *testing.T) {
	r := sharedRepoStore(t)
	saved, err := r.PutSharedRepo(store.SharedRepo{
		Repo:  "rest:http://nas:8001/shared",
		Owner: "nas-id",
		Members: []store.SharedMember{
			{Host: "bv-0123456789ab", Instance: "tower-id", Name: "Tower"},
			{Host: "bv-ba9876543210", Instance: "cube-id", Name: "Cube"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == "" || saved.CreatedAt == 0 {
		t.Fatalf("saved = %+v, want an id and a creation time", saved)
	}

	got, ok, err := r.SharedRepoAt("rest:http://nas:8001/shared")
	if err != nil || !ok {
		t.Fatalf("SharedRepoAt: ok %v, err %v", ok, err)
	}
	if got.ID != saved.ID || got.Owner != "nas-id" || len(got.Members) != 2 || got.Members[1] != saved.Members[1] {
		t.Fatalf("read back %+v, want %+v", got, saved)
	}
	if _, ok, err := r.SharedRepoAt("rest:http://nas:8000/tower"); err != nil || ok {
		t.Fatalf("a location of no shared repository: ok %v, err %v", ok, err)
	}
}

func TestSavingASharedRepositoryAgainReplacesItsMembers(t *testing.T) {
	r := sharedRepoStore(t)
	sr, err := r.PutSharedRepo(store.SharedRepo{Repo: "/mnt/user/shared", Owner: "nas-id",
		Members: []store.SharedMember{{Host: "bv-0123456789ab", Instance: "tower-id"}}})
	if err != nil {
		t.Fatal(err)
	}
	sr.Members = []store.SharedMember{{Host: "bv-ba9876543210", Instance: "cube-id"}}
	if _, err := r.PutSharedRepo(sr); err != nil {
		t.Fatal(err)
	}
	all, err := r.ListSharedRepos()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || len(all[0].Members) != 1 || all[0].Members[0].Host != "bv-ba9876543210" {
		t.Fatalf("listed %+v, want the one repository with the new member alone", all)
	}
}

// Two servers with the same backup key derive the same host, and their
// snapshots could not be told apart in the repository.
func TestASharedRepositoryRefusesAHostTwice(t *testing.T) {
	r := sharedRepoStore(t)
	_, err := r.PutSharedRepo(store.SharedRepo{Repo: "/mnt/user/shared", Owner: "nas-id", Members: []store.SharedMember{
		{Host: "bv-0123456789ab", Instance: "tower-id"},
		{Host: "bv-0123456789ab", Instance: "copy-id"},
	}})
	if !errors.Is(err, store.ErrSharedHostTaken) {
		t.Fatalf("err = %v, want ErrSharedHostTaken", err)
	}
	if all, _ := r.ListSharedRepos(); len(all) != 0 {
		t.Fatalf("a refused save wrote %+v", all)
	}
}

func TestASharedRepositoryNeedsALocationAndAnOwner(t *testing.T) {
	r := sharedRepoStore(t)
	for _, sr := range []store.SharedRepo{{Repo: " ", Owner: "nas-id"}, {Repo: "/mnt/user/shared"}} {
		if _, err := r.PutSharedRepo(sr); !errors.Is(err, store.ErrIncompleteSharedRepo) {
			t.Errorf("%+v: err = %v, want ErrIncompleteSharedRepo", sr, err)
		}
	}
}

func TestDeletingASharedRepositoryTakesItsMembersAlong(t *testing.T) {
	r := sharedRepoStore(t)
	sr, err := r.PutSharedRepo(store.SharedRepo{Repo: "/mnt/user/shared", Owner: "nas-id",
		Members: []store.SharedMember{{Host: "bv-0123456789ab", Instance: "tower-id"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteSharedRepo(sr.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := r.SharedRepoAt("/mnt/user/shared"); err != nil || ok {
		t.Fatalf("after delete: ok %v, err %v", ok, err)
	}
	// The same location can be set up anew, members and all.
	if _, err := r.PutSharedRepo(store.SharedRepo{Repo: "/mnt/user/shared", Owner: "nas-id",
		Members: []store.SharedMember{{Host: "bv-0123456789ab", Instance: "tower-id"}}}); err != nil {
		t.Fatal(err)
	}
}

func TestTheServerNamedAsOwnerHoldsASharedRepository(t *testing.T) {
	sr := store.SharedRepo{Repo: "/mnt/user/shared", Owner: "nas-id"}
	if got := sr.Sharing("nas-id"); got != restic.SharingOwner {
		t.Errorf("on the owner: %v, want SharingOwner", got)
	}
	if got := sr.Sharing("tower-id"); got != restic.SharingMember {
		t.Errorf("on another server: %v, want SharingMember", got)
	}
}

func TestASharedRepositoryMovesWithTheTargetsAndJoinsThatReachIt(t *testing.T) {
	r := sharedRepoStore(t)
	const from, to = "rest:http://nas:8001/shared", "rest:http://cube:8001/shared"
	sr, err := r.PutSharedRepo(store.SharedRepo{Repo: from, Owner: "nas-id",
		Members: []store.SharedMember{{Host: "bv-0123456789ab", Instance: "tower-id", Name: "Tower"}}})
	if err != nil {
		t.Fatal(err)
	}
	there, err := r.CreateOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Shared with nas", Repo: from, CredsRef: "set-1", Immutable: true, Enabled: true, Compression: "max"})
	if err != nil {
		t.Fatal(err)
	}
	elsewhere, err := r.CreateOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Cloud", Repo: "s3:s3.example.com/bucket"})
	if err != nil {
		t.Fatal(err)
	}
	for _, j := range []store.SharedJoin{
		{Direction: store.SharedJoinOut, MemberID: "nas-id", State: store.SharedJoinJoined, Repo: from},
		{Direction: store.SharedJoinOut, MemberID: "attic-id", State: store.SharedJoinJoined, Repo: "rest:http://attic:8001/shared"},
	} {
		if err := r.SaveSharedJoin(j); err != nil {
			t.Fatal(err)
		}
	}

	if err := r.MoveSharedRepo(sr.ID, to); err != nil {
		t.Fatal(err)
	}

	got, ok, err := r.SharedRepoAt(to)
	if err != nil || !ok || got.ID != sr.ID || got.Owner != "nas-id" || len(got.Members) != 1 {
		t.Fatalf("at the new location: %+v, %v, %v; want the same repository with its members", got, ok, err)
	}
	if _, ok, _ := r.SharedRepoAt(from); ok {
		t.Error("the repository is still at the old location")
	}
	targets, err := r.ListOffsiteTargets()
	if err != nil {
		t.Fatal(err)
	}
	for _, tg := range targets {
		switch tg.ID {
		case there.ID:
			if tg.Repo != to || tg.CredsRef != "set-1" || !tg.Immutable || !tg.Enabled || tg.Compression != "max" || tg.Name != "Shared with nas" {
				t.Errorf("the shared target is %+v, want it at %s with its settings", tg, to)
			}
		case elsewhere.ID:
			if tg.Repo != elsewhere.Repo {
				t.Errorf("a target elsewhere moved to %s", tg.Repo)
			}
		}
	}
	if j, _, _ := r.GetSharedJoin(store.SharedJoinOut, "nas-id"); j.Repo != to {
		t.Errorf("the join is at %s, want %s", j.Repo, to)
	}
	if j, _, _ := r.GetSharedJoin(store.SharedJoinOut, "attic-id"); j.Repo != "rest:http://attic:8001/shared" {
		t.Errorf("another join moved to %s", j.Repo)
	}
}
