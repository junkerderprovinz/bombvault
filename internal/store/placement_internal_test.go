package store

import (
	"errors"
	"slices"
	"testing"
)

func TestReadPlacementFailsWhenOneRuleOrTheDefaultCannotBeRead(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := New(db)
	SeedCopyRule(t, r, "containers", "container:plex")
	if _, err := db.Exec(`INSERT INTO offsite_copy_rules (domain, identity, skip)
		VALUES ('containers', 'container:nginx', 'not json')`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReadPlacement("containers"); !errors.Is(err, ErrBadSkip) {
		t.Fatalf("ReadPlacement with one broken rule = %v, want ErrBadSkip", err)
	}
	if _, err := db.Exec(`DELETE FROM offsite_copy_rules`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO placement_defaults (domain, skip, confirmed_at) VALUES ('containers', 'null', 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReadPlacement("containers"); !errors.Is(err, ErrBadSkip) {
		t.Fatalf("ReadPlacement with a broken default = %v, want ErrBadSkip", err)
	}
}

func TestPlacementDomainsUsingRepoTxNamesTheDefaultsOnARepository(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := New(db)
	SeedDefault(t, r, "vms", "nas")
	SeedDefault(t, r, "containers", "nas")
	SeedDefault(t, r, "files", "")
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback() //nolint:errcheck // the test only reads
	if got, err := placementDomainsUsingRepoTx(tx, "nas"); err != nil || !slices.Equal(got, []string{"containers", "vms"}) {
		t.Fatalf("placementDomainsUsingRepoTx(nas) = %v, %v", got, err)
	}
	if got, err := placementDomainsUsingRepoTx(tx, "other"); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("placementDomainsUsingRepoTx(other) = %#v, %v, want an empty list", got, err)
	}
}
