package store_test

import (
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func sortOrderOf(t *testing.T, r *store.Repo, id string) int {
	t.Helper()
	tg, ok, err := r.GetOffsiteTarget(id)
	if err != nil || !ok {
		t.Fatalf("GetOffsiteTarget(%s): ok=%v err=%v", id, ok, err)
	}
	return tg.SortOrder
}

func TestCreateOffsiteTargetGoesBehindTheDomainsLastTarget(t *testing.T) {
	r := newRepo(t)

	first := store.SeedOffsiteTarget(t, r, "containers", "s3:first")
	if first.SortOrder != 1 {
		t.Fatalf("first target of an empty domain got sort_order %d, want 1", first.SortOrder)
	}
	store.SeedFieldTarget(t, r, "containers", "s3:field")
	got, err := r.CreateOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Repo: "s3:second", Role: store.RoleRepo, SortOrder: 0, Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateOffsiteTarget: %v", err)
	}
	if got.SortOrder != 2 || sortOrderOf(t, r, got.ID) != 2 {
		t.Fatalf("second target got sort_order %d, want 2", got.SortOrder)
	}
	if got.Role != store.RoleOffsite || got.CreatedAt == 0 {
		t.Fatalf("created row = %+v, want role offsite and a creation time", got)
	}
	if other := store.SeedOffsiteTarget(t, r, "vms", "s3:vms"); other.SortOrder != 1 {
		t.Fatalf("another domain's first target got sort_order %d, want 1", other.SortOrder)
	}
}

func TestCreateOffsiteTargetRefusesAnEmptyLocation(t *testing.T) {
	r := newRepo(t)
	if _, err := r.CreateOffsiteTarget(store.OffsiteTarget{Domain: "containers", Repo: " "}); !errors.Is(err, store.ErrEmptyOffsiteRepo) {
		t.Fatalf("err = %v, want ErrEmptyOffsiteRepo", err)
	}
}

func TestUpsertOffsiteTargetKeepsTheStoredSortOrder(t *testing.T) {
	r := newRepo(t)
	store.SeedFieldTarget(t, r, "containers", "s3:field")
	tg := store.SeedOffsiteTarget(t, r, "containers", "s3:second")

	tg.Name = "Hetzner"
	tg.SortOrder = 0
	tg.CreatedAt = 0
	got, err := r.UpsertOffsiteTarget(tg)
	if err != nil {
		t.Fatalf("UpsertOffsiteTarget: %v", err)
	}
	if got.SortOrder != 1 || sortOrderOf(t, r, tg.ID) != 1 {
		t.Fatalf("update moved the row to sort_order %d, want 1", got.SortOrder)
	}
	if got.Name != "Hetzner" || got.CreatedAt == 0 {
		t.Fatalf("returned row = %+v, want the stored one", got)
	}
}

func TestUpsertOffsiteTargetInsertsWithTheGivenSortOrder(t *testing.T) {
	r := newRepo(t)
	got, err := r.UpsertOffsiteTarget(store.OffsiteTarget{ID: "from-file", Domain: "containers", Repo: "s3:x", SortOrder: 7})
	if err != nil {
		t.Fatalf("UpsertOffsiteTarget: %v", err)
	}
	if got.SortOrder != 7 {
		t.Fatalf("inserted row got sort_order %d, want 7", got.SortOrder)
	}
}

func TestFieldOffsiteTargetIsTheOldestRowOnZero(t *testing.T) {
	r := newRepo(t)
	if _, ok, err := r.FieldOffsiteTarget("containers"); ok || err != nil {
		t.Fatalf("empty domain: ok=%v err=%v, want neither", ok, err)
	}
	store.SeedNamedRepo(t, r, "NAS", "/mnt/nas")
	for _, tg := range []store.OffsiteTarget{
		{ID: "newer", Domain: "containers", Repo: "rest:http://peer/containers", Enabled: true, CreatedAt: 2000},
		{ID: "older", Domain: "containers", Repo: "s3:field", Enabled: false, CreatedAt: 1000},
		{ID: "behind", Domain: "containers", Repo: "s3:second", Enabled: true, CreatedAt: 500, SortOrder: 1},
	} {
		if _, err := r.UpsertOffsiteTarget(tg); err != nil {
			t.Fatal(err)
		}
	}
	got, ok, err := r.FieldOffsiteTarget("containers")
	if err != nil || !ok || got.ID != "older" {
		t.Fatalf("FieldOffsiteTarget = %q ok=%v err=%v, want the switched-off row older", got.ID, ok, err)
	}
	if _, ok, _ := r.FieldOffsiteTarget("vms"); ok {
		t.Fatal("a domain without targets has no field row")
	}
}

func TestNormalizeOffsiteSortOrderGivesZeroToTheFieldsRow(t *testing.T) {
	r := newRepo(t)
	for _, tg := range []store.OffsiteTarget{
		{ID: "field", Domain: "containers", Repo: "s3:b2", CreatedAt: 1000, SortOrder: 3},
		{ID: "second", Domain: "containers", Repo: "sftp:hetzner", CreatedAt: 1100, SortOrder: 1},
		{ID: "mesh-a", Domain: "containers", Repo: "rest:http://a/c", CreatedAt: 1200},
		{ID: "mesh-b", Domain: "containers", Repo: "rest:http://b/c", CreatedAt: 1300},
		{ID: "vms-field", Domain: "vms", Repo: "s3:vms", CreatedAt: 1000},
	} {
		if _, err := r.UpsertOffsiteTarget(tg); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.NormalizeOffsiteSortOrder("containers", "s3:b2"); err != nil {
		t.Fatalf("NormalizeOffsiteSortOrder: %v", err)
	}
	want := map[string]int{"field": 0, "second": 1, "mesh-a": 4, "mesh-b": 5, "vms-field": 0}
	for id, order := range want {
		if got := sortOrderOf(t, r, id); got != order {
			t.Errorf("%s: sort_order %d, want %d", id, got, order)
		}
	}
}

func TestNormalizeOffsiteSortOrderWithAnEmptyFieldLeavesNothingOnZero(t *testing.T) {
	r := newRepo(t)
	for _, tg := range []store.OffsiteTarget{
		{ID: "old-field", Domain: "files", Repo: "s3:b2", CreatedAt: 1000},
		{ID: "second", Domain: "files", Repo: "sftp:hetzner", CreatedAt: 1100, SortOrder: 1},
	} {
		if _, err := r.UpsertOffsiteTarget(tg); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.NormalizeOffsiteSortOrder("files", ""); err != nil {
		t.Fatalf("NormalizeOffsiteSortOrder: %v", err)
	}
	if got := sortOrderOf(t, r, "old-field"); got != 2 {
		t.Fatalf("old-field: sort_order %d, want 2", got)
	}
	if _, ok, _ := r.FieldOffsiteTarget("files"); ok {
		t.Fatal("an empty field must leave no row on sort_order 0")
	}
}
