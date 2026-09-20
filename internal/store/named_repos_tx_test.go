package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The two guarded writes that protect a named repository (#204) had no test at
// all: the guard over them asserted only that the handler CALLS them. These run
// against a real database and assert the refusal and the write.

func namedRepoStore(t *testing.T) *store.Repo {
	t.Helper()
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return store.New(db)
}

func aNamedRepo(t *testing.T, r *store.Repo, name, loc string) store.OffsiteTarget {
	t.Helper()
	row, err := r.UpsertOffsiteTarget(store.OffsiteTarget{
		Role: store.RoleRepo, Name: name, Repo: loc, Enabled: true,
	})
	if err != nil {
		t.Fatalf("UpsertOffsiteTarget: %v", err)
	}
	return row
}

// TestDeleteNamedRepoIfUnusedRefusesWhileAnItemPointsAtIt pins the refusal that
// keeps an item from silently falling back to its domain repository - where its
// next backup would land looking exactly like a working one.
func TestDeleteNamedRepoIfUnusedRefusesWhileAnItemPointsAtIt(t *testing.T) {
	r := namedRepoStore(t)
	repo := aNamedRepo(t, r, "Cold", "backups/cold")
	if _, err := r.UpsertTarget(store.Target{ContainerName: "plex"}); err != nil {
		t.Fatal(err)
	}
	if err := r.SetTargetRepo("plex", repo.ID); err != nil {
		t.Fatal(err)
	}

	n, err := r.DeleteNamedRepoIfUnused(repo.ID)
	if err != nil {
		t.Fatalf("DeleteNamedRepoIfUnused: %v", err)
	}
	if n != 1 {
		t.Fatalf("in-use count = %d, want 1", n)
	}
	if _, err := r.GetNamedRepo(repo.ID); err != nil {
		t.Fatalf("a refused delete must leave the row in place, got %v", err)
	}
}

// TestDeleteNamedRepoIfUnusedDeletesWhenNothingPointsAtIt is the other half: the
// refusal must not become a repository that can never be removed.
func TestDeleteNamedRepoIfUnusedDeletesWhenNothingPointsAtIt(t *testing.T) {
	r := namedRepoStore(t)
	repo := aNamedRepo(t, r, "Cold", "backups/cold")

	n, err := r.DeleteNamedRepoIfUnused(repo.ID)
	if err != nil {
		t.Fatalf("DeleteNamedRepoIfUnused: %v", err)
	}
	if n != 0 {
		t.Fatalf("in-use count = %d, want 0", n)
	}
	if _, err := r.GetNamedRepo(repo.ID); err == nil {
		t.Fatal("the row must be gone once nothing points at it")
	}
}

// TestSetNamedRepoLocationIfUnusedRefusesWhileInUse pins the move refusal.
// Everything written so far stays where it is, so a move under a live item makes
// the next backup succeed into an empty repository.
func TestSetNamedRepoLocationIfUnusedRefusesWhileInUse(t *testing.T) {
	r := namedRepoStore(t)
	repo := aNamedRepo(t, r, "Cold", "backups/cold")
	if _, err := r.UpsertVMTarget(store.VMTarget{Name: "win11"}); err != nil {
		t.Fatal(err)
	}
	if err := r.SetVMRepo("win11", repo.ID); err != nil {
		t.Fatal(err)
	}

	n, err := r.SetNamedRepoLocationIfUnused(repo.ID, "backups/elsewhere")
	if err != nil {
		t.Fatalf("SetNamedRepoLocationIfUnused: %v", err)
	}
	if n != 1 {
		t.Fatalf("in-use count = %d, want 1", n)
	}
	back, err := r.GetNamedRepo(repo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.Repo != "backups/cold" {
		t.Fatalf("a refused move must leave the location alone, got %q", back.Repo)
	}
}

// TestSetNamedRepoLocationIfUnusedWritesWhenUnused is its other half.
func TestSetNamedRepoLocationIfUnusedWritesWhenUnused(t *testing.T) {
	r := namedRepoStore(t)
	repo := aNamedRepo(t, r, "Cold", "backups/cold")

	n, err := r.SetNamedRepoLocationIfUnused(repo.ID, "backups/elsewhere")
	if err != nil {
		t.Fatalf("SetNamedRepoLocationIfUnused: %v", err)
	}
	if n != 0 {
		t.Fatalf("in-use count = %d, want 0", n)
	}
	back, err := r.GetNamedRepo(repo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.Repo != "backups/elsewhere" {
		t.Fatalf("location = %q, want the move written", back.Repo)
	}
}

// TestTheGuardedWritesCountEveryDomain pins that a folder set counts too. The
// in-use query sums three tables, and a domain missing from it would make the
// refusal silently inapplicable for that domain's items.
func TestTheGuardedWritesCountEveryDomain(t *testing.T) {
	for _, tc := range []struct {
		domain string
		point  func(*store.Repo, string) error
	}{
		{"containers", func(r *store.Repo, id string) error {
			if _, err := r.UpsertTarget(store.Target{ContainerName: "plex"}); err != nil {
				return err
			}
			return r.SetTargetRepo("plex", id)
		}},
		{"vms", func(r *store.Repo, id string) error {
			if _, err := r.UpsertVMTarget(store.VMTarget{Name: "win11"}); err != nil {
				return err
			}
			return r.SetVMRepo("win11", id)
		}},
		{"files", func(r *store.Repo, id string) error {
			set, err := r.CreateFileSet(store.FileSet{Name: "docs", Path: "user/docs", Enabled: true})
			if err != nil {
				return err
			}
			return r.SetFileSetRepo(set.ID, id)
		}},
	} {
		t.Run(tc.domain, func(t *testing.T) {
			r := namedRepoStore(t)
			repo := aNamedRepo(t, r, "Cold", "backups/cold")
			if err := tc.point(r, repo.ID); err != nil {
				t.Fatal(err)
			}
			n, err := r.DeleteNamedRepoIfUnused(repo.ID)
			if err != nil {
				t.Fatal(err)
			}
			if n != 1 {
				t.Fatalf("a %s item pointing at the repository must block the delete, count = %d", tc.domain, n)
			}
		})
	}
}

// TestCreateFileSetStoresTheRepository pins that the repository rides along in
// the INSERT. It used to be a second statement, which left a window in which the
// set existed on the domain repository while the caller believed otherwise.
func TestCreateFileSetStoresTheRepository(t *testing.T) {
	r := namedRepoStore(t)
	repo := aNamedRepo(t, r, "Cold", "backups/cold")

	set, err := r.CreateFileSet(store.FileSet{Name: "docs", Path: "user/docs", Enabled: true, Repo: repo.ID, RepoChosen: store.RepoChosen})
	if err != nil {
		t.Fatalf("CreateFileSet: %v", err)
	}
	back, err := r.GetFileSet(set.ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.Repo != repo.ID {
		t.Fatalf("repo = %q, want the chosen repository stored by the insert itself", back.Repo)
	}
}
