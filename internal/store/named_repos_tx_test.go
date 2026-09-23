package store_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

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

func chooseRepo(r *store.Repo, domain, key, repo string) error {
	_, err := r.WritePlacement(store.ItemRef{Domain: domain, Key: key}, &store.HomeWrite{Repo: repo, Choice: store.RepoChosen}, nil, nil)
	return err
}

// TestDeleteNamedRepoIfUnusedRefusesWhileInUse expects a refusal, because
// without the repository the item's next backup would silently go to its domain
// repository and look like a working one.
func TestDeleteNamedRepoIfUnusedRefusesWhileInUse(t *testing.T) {
	r := namedRepoStore(t)
	repo := aNamedRepo(t, r, "Cold", "backups/cold")
	if _, err := r.UpsertTarget(store.Target{ContainerName: "plex"}); err != nil {
		t.Fatal(err)
	}
	if err := chooseRepo(r, "containers", "plex", repo.ID); err != nil {
		t.Fatal(err)
	}

	use, err := r.DeleteNamedRepoIfUnused(repo.ID)
	if err != nil {
		t.Fatalf("DeleteNamedRepoIfUnused: %v", err)
	}
	if use.Items != 1 {
		t.Fatalf("in-use count = %d, want 1", use.Items)
	}
	if _, err := r.GetNamedRepo(repo.ID); err != nil {
		t.Fatalf("a refused delete must leave the row in place, got %v", err)
	}
}

// TestDeleteNamedRepoIfUnusedRefusesWhileADefaultPointsAtIt is the same refusal
// for a placement default's home, which points at a repository without a row in
// any item table.
func TestDeleteNamedRepoIfUnusedRefusesWhileADefaultPointsAtIt(t *testing.T) {
	r := namedRepoStore(t)
	repo := aNamedRepo(t, r, "Cold", "backups/cold")
	store.SeedDefault(t, r, "vms", repo.ID)
	store.SeedDefault(t, r, "containers", repo.ID)
	use, err := r.DeleteNamedRepoIfUnused(repo.ID)
	if err != nil {
		t.Fatalf("DeleteNamedRepoIfUnused: %v", err)
	}
	if !use.InUse() || use.Items != 0 || !slices.Equal(use.DefaultDomains, []string{"containers", "vms"}) {
		t.Fatalf("use = %+v, want the two defaults and no item", use)
	}
	if _, err := r.GetNamedRepo(repo.ID); err != nil {
		t.Fatalf("a refused delete must leave the row: %v", err)
	}
}

func TestDeleteNamedRepoIfUnusedDeletesWhenUnused(t *testing.T) {
	r := namedRepoStore(t)
	repo := aNamedRepo(t, r, "Cold", "backups/cold")

	use, err := r.DeleteNamedRepoIfUnused(repo.ID)
	if err != nil {
		t.Fatalf("DeleteNamedRepoIfUnused: %v", err)
	}
	if use.InUse() {
		t.Fatalf("use = %+v, want nothing", use)
	}
	if _, err := r.GetNamedRepo(repo.ID); err == nil {
		t.Fatal("the row must be gone once nothing points at it")
	}
}

// A direct repository goes with its target. Deleting it here would leave the
// snapshots in the bucket with nothing on the box that names them.
func TestDeleteNamedRepoIfUnusedRefusesADirectRepository(t *testing.T) {
	r := namedRepoStore(t)
	target, err := r.CreateOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "B2", Repo: "b2:bkt:containers", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	direct, err := r.CreateCompanionRepo(target.ID, "B2 direct", "b2:bkt:containers-direct")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := r.DeleteNamedRepoIfUnused(direct.ID); !errors.Is(err, store.ErrDirectRepo) {
		t.Fatalf("DeleteNamedRepoIfUnused = %v, want ErrDirectRepo", err)
	}
	if _, err := r.GetNamedRepo(direct.ID); err != nil {
		t.Fatalf("a refused delete must leave the row: %v", err)
	}
}

// TestSetNamedRepoLocationIfUnusedRefusesWhileInUse expects the move to be
// refused: existing snapshots stay where they are, so a live item's next backup
// would succeed into an empty repository.
func TestSetNamedRepoLocationIfUnusedRefusesWhileInUse(t *testing.T) {
	r := namedRepoStore(t)
	repo := aNamedRepo(t, r, "Cold", "backups/cold")
	if _, err := r.UpsertVMTarget(store.VMTarget{Name: "win11"}); err != nil {
		t.Fatal(err)
	}
	if err := chooseRepo(r, "vms", "win11", repo.ID); err != nil {
		t.Fatal(err)
	}

	use, err := r.SetNamedRepoLocationIfUnused(repo.ID, "backups/elsewhere", false)
	if err != nil {
		t.Fatalf("SetNamedRepoLocationIfUnused: %v", err)
	}
	if use.Items != 1 {
		t.Fatalf("in-use count = %d, want 1", use.Items)
	}
	back, err := r.GetNamedRepo(repo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.Repo != "backups/cold" {
		t.Fatalf("a refused move must leave the location alone, got %q", back.Repo)
	}
}

// TestSetNamedRepoLocationIfUnusedRefusesWhileADefaultPointsAtIt is the move's
// side of the same gap DeleteNamedRepoIfUnused already closed: a default that
// names this repository as home has no row in an item table, so the item count
// alone would wave the move through and leave every open item of that domain
// starting a fresh repository at the new place while its snapshots stay behind.
func TestSetNamedRepoLocationIfUnusedRefusesWhileADefaultPointsAtIt(t *testing.T) {
	r := namedRepoStore(t)
	repo := aNamedRepo(t, r, "Cold", "backups/cold")
	store.SeedDefault(t, r, "vms", repo.ID)
	store.SeedDefault(t, r, "containers", repo.ID)

	use, err := r.SetNamedRepoLocationIfUnused(repo.ID, "backups/elsewhere", false)
	if err != nil {
		t.Fatalf("SetNamedRepoLocationIfUnused: %v", err)
	}
	if !use.InUse() || use.Items != 0 || !slices.Equal(use.DefaultDomains, []string{"containers", "vms"}) {
		t.Fatalf("use = %+v, want the two defaults and no item", use)
	}
	back, err := r.GetNamedRepo(repo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.Repo != "backups/cold" {
		t.Fatalf("a refused move must leave the location alone, got %q", back.Repo)
	}
}

func TestSetNamedRepoLocationIfUnusedWritesWhenUnused(t *testing.T) {
	r := namedRepoStore(t)
	repo := aNamedRepo(t, r, "Cold", "backups/cold")

	use, err := r.SetNamedRepoLocationIfUnused(repo.ID, "backups/elsewhere", false)
	if err != nil {
		t.Fatalf("SetNamedRepoLocationIfUnused: %v", err)
	}
	if use.InUse() {
		t.Fatalf("use = %+v, want nothing", use)
	}
	back, err := r.GetNamedRepo(repo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.Repo != "backups/elsewhere" {
		t.Fatalf("location = %q, want the move written", back.Repo)
	}
}

// TestTheMoveWritesTheOffPremisesMarkWithTheLocation pins the pair: the mark
// describes the location, so it is written by the statement that moves it and
// stays untouched when the move is refused.
func TestTheMoveWritesTheOffPremisesMarkWithTheLocation(t *testing.T) {
	r := namedRepoStore(t)
	box := aNamedRepo(t, r, "Storagebox", "sftp:u@box:/bv")
	box.OffPremises = true
	if _, err := r.UpsertOffsiteTarget(box); err != nil {
		t.Fatal(err)
	}

	if _, err := r.SetNamedRepoLocationIfUnused(box.ID, "backups/cold", false); err != nil {
		t.Fatalf("SetNamedRepoLocationIfUnused: %v", err)
	}
	moved, err := r.GetNamedRepo(box.ID)
	if err != nil {
		t.Fatal(err)
	}
	if moved.Repo != "backups/cold" || moved.OffPremises {
		t.Fatalf("after the move: %q, off premises %v; want the array and no mark", moved.Repo, moved.OffPremises)
	}

	if err := chooseRepo(r, "vms", "win11", box.ID); err != nil {
		t.Fatal(err)
	}
	use, err := r.SetNamedRepoLocationIfUnused(box.ID, "sftp:u@box:/bv", true)
	if err != nil {
		t.Fatalf("SetNamedRepoLocationIfUnused: %v", err)
	}
	if !use.InUse() {
		t.Fatalf("use = %+v, want the item counted", use)
	}
	back, err := r.GetNamedRepo(box.ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.Repo != "backups/cold" || back.OffPremises {
		t.Fatalf("a refused move wrote %q, off premises %v", back.Repo, back.OffPremises)
	}
}

// TestGuardedWritesCountEveryDomain covers all three tables the in-use query
// sums; a domain missing from it would let that domain's items lose their
// repository.
func TestGuardedWritesCountEveryDomain(t *testing.T) {
	for _, tc := range []struct {
		domain string
		point  func(*store.Repo, string) error
	}{
		{"containers", func(r *store.Repo, id string) error {
			return chooseRepo(r, "containers", "plex", id)
		}},
		{"vms", func(r *store.Repo, id string) error {
			return chooseRepo(r, "vms", "win11", id)
		}},
		{"files", func(r *store.Repo, id string) error {
			set, err := r.CreateFileSet(store.FileSet{Name: "docs", Path: "user/docs", Enabled: true})
			if err != nil {
				return err
			}
			return chooseRepo(r, "files", set.ID, id)
		}},
	} {
		t.Run(tc.domain, func(t *testing.T) {
			r := namedRepoStore(t)
			repo := aNamedRepo(t, r, "Cold", "backups/cold")
			if err := tc.point(r, repo.ID); err != nil {
				t.Fatal(err)
			}
			use, err := r.DeleteNamedRepoIfUnused(repo.ID)
			if err != nil {
				t.Fatal(err)
			}
			if use.Items != 1 {
				t.Fatalf("a %s item pointing at the repository must block the delete, count = %d", tc.domain, use.Items)
			}
		})
	}
}

// TestCreateFileSetStoresRepository checks that the repository is part of the
// INSERT, so there is no window in which the new set sits on the domain
// repository.
func TestCreateFileSetStoresRepository(t *testing.T) {
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
