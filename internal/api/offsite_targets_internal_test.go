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

	// No targets yet: not found.
	if _, ok := s.primaryOffsiteTarget("containers"); ok {
		t.Fatal("primaryOffsiteTarget should be false with no targets")
	}

	// A disabled target followed (in sort order) by an enabled one: the helper
	// returns the first ENABLED target, not merely the first target.
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

	// A domain with no configured target stays not-found.
	if _, ok := s.primaryOffsiteTarget("vms"); ok {
		t.Fatal("primaryOffsiteTarget(vms) should be false")
	}
}

// TestOffsiteTargetsFor pins the stage-2 target resolver: it returns the domain's
// ENABLED targets in stable order for a configured domain (the backfilled single
// target for an N=1 install) and an empty slice for an unconfigured one.
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

	// Unconfigured domain: empty slice.
	if got := s.offsiteTargetsFor("containers"); len(got) != 0 {
		t.Fatalf("offsiteTargetsFor on an unconfigured domain = %d targets, want 0", len(got))
	}

	// One enabled target (the shape the stage-1 backfill produces for N=1).
	want, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "Primary", Repo: "s3:c", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	// A disabled target for the same domain must be filtered out.
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

	// A different, unconfigured domain still returns empty.
	if got := s.offsiteTargetsFor("vms"); len(got) != 0 {
		t.Fatalf("offsiteTargetsFor(vms) = %d targets, want 0", len(got))
	}
}
