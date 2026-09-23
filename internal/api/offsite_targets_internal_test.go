package api

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestPrimaryOffsiteTarget(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := store.New(db)
	s := &Service{store: st}

	if _, ok := s.primaryOffsiteTarget("containers"); ok {
		t.Fatal("primaryOffsiteTarget should be false with no targets")
	}

	// A disabled target sorts before the enabled one.
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "Disabled", Repo: "s3:a", Enabled: false, SortOrder: 0,
	}); err != nil {
		t.Fatal(err)
	}
	wanted, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "Primary", Repo: "s3:b", Enabled: true, SortOrder: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	got, ok := s.primaryOffsiteTarget("containers")
	if !ok {
		t.Fatal("primaryOffsiteTarget should find the enabled target")
	}
	if got.ID != wanted.ID || got.Repo != "s3:b" {
		t.Fatalf("primaryOffsiteTarget = %+v, want id %s repo s3:b", got, wanted.ID)
	}

	if _, ok := s.primaryOffsiteTarget("vms"); ok {
		t.Fatal("primaryOffsiteTarget(vms) should be false")
	}
}

func TestOffsiteTargetsFor(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := store.New(db)
	s := &Service{store: st}

	if got := s.offsiteTargetsFor("containers"); len(got) != 0 {
		t.Fatalf("offsiteTargetsFor on an unconfigured domain = %d targets, want 0", len(got))
	}

	want, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "Primary", Repo: "s3:c", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "Disabled", Repo: "s3:x", Enabled: false, SortOrder: 9,
	}); err != nil {
		t.Fatal(err)
	}

	got := s.offsiteTargetsFor("containers")
	if len(got) != 1 {
		t.Fatalf("offsiteTargetsFor(containers) = %d targets, want 1 (the enabled one)", len(got))
	}
	if got[0].ID != want.ID || got[0].Repo != "s3:c" {
		t.Fatalf("offsiteTargetsFor(containers) = %+v, want id %s repo s3:c", got[0], want.ID)
	}

	if got := s.offsiteTargetsFor("vms"); len(got) != 0 {
		t.Fatalf("offsiteTargetsFor(vms) = %d targets, want 0", len(got))
	}
}
