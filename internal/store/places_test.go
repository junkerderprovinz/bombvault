package store_test

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// placesRepo is a migrated store and the database under it, for the writers
// that take a transaction.
func placesRepo(t *testing.T) (*store.Repo, *sql.DB) {
	t.Helper()
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return store.New(db), db
}

// runTx runs fn in a transaction and commits when it returns nil.
func runTx(t *testing.T, db *sql.DB, fn func(*sql.Tx) error) error {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// attachRow puts a row at a place the way the migration does.
func attachRow(t *testing.T, db *sql.DB, rowID, placeID, domain, suffix string) {
	t.Helper()
	if err := runTx(t, db, func(tx *sql.Tx) error { return store.AttachRowTx(tx, rowID, placeID, domain, suffix) }); err != nil {
		t.Fatalf("attach %s: %v", rowID, err)
	}
}

func upsertRow(t *testing.T, r *store.Repo, row store.OffsiteTarget) store.OffsiteTarget {
	t.Helper()
	stored, err := r.UpsertOffsiteTarget(row)
	if err != nil {
		t.Fatalf("upsert %s: %v", row.Name, err)
	}
	return stored
}

func TestAPlaceLinkReadsBackThroughEveryReader(t *testing.T) {
	r, db := placesRepo(t)
	target := upsertRow(t, r, store.OffsiteTarget{Domain: "containers", Name: "B2", Repo: "s3:https://s3.example.com/bucket/container", Enabled: true})
	named := upsertRow(t, r, store.OffsiteTarget{Role: store.RoleRepo, Name: "NAS", Repo: "remotes/nas/bv", Enabled: true})
	direct, err := r.CreateCompanionRepo(target.ID, "B2 direct", "s3:https://s3.example.com/bucket/container-direct")
	if err != nil {
		t.Fatal(err)
	}
	primary, err := r.UpsertPrimaryRemoteTarget("vms", store.OffsiteTarget{Repo: "rest:http://tower:8000/bv/vms", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	attachRow(t, db, target.ID, "p-b2", "containers", "")
	attachRow(t, db, direct.ID, "p-b2", "containers", "-direct")
	attachRow(t, db, named.ID, "p-nas", "", "")
	attachRow(t, db, primary.ID, "p-tower", "vms", "")

	placed := func(reader string, row store.OffsiteTarget, placeID, domain, suffix string) {
		t.Helper()
		if row.PlaceID != placeID || row.PlaceDomain != domain || row.PlaceSuffix != suffix {
			t.Errorf("%s: place %q, domain %q, suffix %q, want %q, %q, %q",
				reader, row.PlaceID, row.PlaceDomain, row.PlaceSuffix, placeID, domain, suffix)
		}
	}
	got, _, err := r.GetOffsiteTarget(target.ID)
	if err != nil {
		t.Fatal(err)
	}
	placed("GetOffsiteTarget", got, "p-b2", "containers", "")
	all, err := r.ListOffsiteTargets()
	if err != nil || len(all) != 1 {
		t.Fatalf("ListOffsiteTargets = %+v, %v", all, err)
	}
	placed("ListOffsiteTargets", all[0], "p-b2", "containers", "")
	ofDomain, err := r.OffsiteTargetsForDomain("containers")
	if err != nil || len(ofDomain) != 1 {
		t.Fatalf("OffsiteTargetsForDomain = %+v, %v", ofDomain, err)
	}
	placed("OffsiteTargetsForDomain", ofDomain[0], "p-b2", "containers", "")
	field, _, err := r.FieldOffsiteTarget("containers")
	if err != nil {
		t.Fatal(err)
	}
	placed("FieldOffsiteTarget", field, "p-b2", "containers", "")
	companion, _, err := r.CompanionFor(target.ID)
	if err != nil {
		t.Fatal(err)
	}
	placed("CompanionFor", companion, "p-b2", "containers", "-direct")
	repo, err := r.GetNamedRepo(named.ID)
	if err != nil {
		t.Fatal(err)
	}
	placed("GetNamedRepo", repo, "p-nas", "", "")
	repos, err := r.ListNamedRepos()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range repos {
		if row.ID == named.ID {
			placed("ListNamedRepos", row, "p-nas", "", "")
		}
	}
	prim, _, err := r.PrimaryRemoteTarget("vms")
	if err != nil {
		t.Fatal(err)
	}
	placed("PrimaryRemoteTarget", prim, "p-tower", "vms", "")
	rows, err := r.PlaceRows("p-b2")
	if err != nil || len(rows) != 2 || rows[0].ID != target.ID || rows[1].ID != direct.ID {
		t.Fatalf("PlaceRows(p-b2) = %+v, %v, want the target and its direct repository", rows, err)
	}
}

func TestAPlainSaveKeepsARowAtItsPlace(t *testing.T) {
	r, db := placesRepo(t)
	target := upsertRow(t, r, store.OffsiteTarget{Domain: "containers", Name: "B2", Repo: "s3:https://s3.example.com/bucket/container", Enabled: true})
	direct, err := r.CreateCompanionRepo(target.ID, "B2 direct", "s3:https://s3.example.com/bucket/container-direct")
	if err != nil {
		t.Fatal(err)
	}
	attachRow(t, db, target.ID, "p-b2", "containers", "")
	attachRow(t, db, direct.ID, "p-b2", "containers", "-direct")

	upsertRow(t, r, store.OffsiteTarget{ID: target.ID, Domain: "containers", Name: "B2 renamed", Repo: target.Repo,
		RetentionKeepDaily: 14, Enabled: true})

	got, _, err := r.GetOffsiteTarget(target.ID)
	if err != nil || got.Name != "B2 renamed" || got.RetentionKeepDaily != 14 || got.PlaceID != "p-b2" || got.PlaceDomain != "containers" {
		t.Fatalf("target = %+v, %v, want the new fields at its place", got, err)
	}
	companion, _, err := r.CompanionFor(target.ID)
	if err != nil || companion.RetentionKeepDaily != 14 || companion.PlaceID != "p-b2" || companion.PlaceSuffix != "-direct" {
		t.Fatalf("direct repository = %+v, %v, want the mirrored retention at its place", companion, err)
	}
}

func TestANewRowCanArriveAtItsPlace(t *testing.T) {
	r, _ := placesRepo(t)
	field := upsertRow(t, r, store.OffsiteTarget{Domain: "flash", Name: "B2", Repo: "s3:https://s3.example.com/bucket/flash",
		Enabled: true, PlaceID: "p-b2", PlaceDomain: "flash"})
	copies, err := r.CreateOffsiteTarget(store.OffsiteTarget{Domain: "flash", Name: "B2 copies", Repo: "s3:https://s3.example.com/bucket/flash-copies",
		Enabled: true, PlaceID: "p-b2", PlaceDomain: "flash", PlaceSuffix: "-copies"})
	if err != nil {
		t.Fatal(err)
	}
	if field.PlaceID != "p-b2" || field.PlaceDomain != "flash" || copies.PlaceSuffix != "-copies" || copies.PlaceID != "p-b2" {
		t.Fatalf("field = %+v, copies = %+v, want both at p-b2", field, copies)
	}
}

func TestARowLeavesItsPlaceWithItsAddressAndSettings(t *testing.T) {
	r, db := placesRepo(t)
	target := upsertRow(t, r, store.OffsiteTarget{Domain: "containers", Name: "B2", Repo: "s3:https://s3.example.com/bucket/container",
		RetentionKeepDaily: 7, Enabled: true})
	attachRow(t, db, target.ID, "p-b2", "containers", "")

	if err := runTx(t, db, func(tx *sql.Tx) error { return store.DetachRowTx(tx, target.ID) }); err != nil {
		t.Fatal(err)
	}
	got, _, err := r.GetOffsiteTarget(target.ID)
	if err != nil || got.PlaceID != "" || got.PlaceDomain != "" || got.PlaceSuffix != "" || got.Repo != target.Repo || got.RetentionKeepDaily != 7 {
		t.Fatalf("detached = %+v, %v, want no place and everything else kept", got, err)
	}
	err = runTx(t, db, func(tx *sql.Tx) error { return store.AttachRowTx(tx, "nope", "p-b2", "containers", "") })
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("attaching an unknown row = %v, want sql.ErrNoRows", err)
	}
}
