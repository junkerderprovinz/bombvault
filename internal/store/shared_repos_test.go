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
