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
