package store_test

import (
	"database/sql"
	"errors"
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/places"
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

func mustWritePlace(t *testing.T, r *store.Repo, w store.PlaceWrite) store.Place {
	t.Helper()
	p, err := r.WritePlace(w)
	if err != nil {
		t.Fatalf("WritePlace %s: %v", w.Place.Name, err)
	}
	return p
}

func storedSettings(t *testing.T, r *store.Repo) store.Settings {
	t.Helper()
	s, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// diskPlace is a folder on this server offering every domain in its default folder.
func diskPlace() store.Place {
	return store.Place{Name: "Unraid", Provider: "unraid-folder", Kind: string(places.KindLocal), Base: "user/bombvault",
		Folders: places.DefaultFolders(), Enabled: true}
}

// bucketPlace is an S3 bucket off the premises offering every domain in its default folder.
func bucketPlace() store.Place {
	return store.Place{Name: "B2", Provider: "b2", Kind: string(places.KindS3), Base: "s3:https://s3.example.com/bucket",
		Folders: places.DefaultFolders(), CredsRef: "set-b2", StorageClass: "STANDARD", RetentionKeepDaily: 30,
		LimitUpload: 1000, GrowthBudgetGB: 50, OffPremises: true, Enabled: true}
}

func TestANewPlaceReadsBackAsWritten(t *testing.T) {
	r, _ := placesRepo(t)
	p := mustWritePlace(t, r, store.PlaceWrite{Place: bucketPlace()})
	if p.ID == "" || p.CreatedAt == 0 || p.UpdatedAt != p.CreatedAt {
		t.Fatalf("new place = %+v, want an id and both timestamps", p)
	}
	got, err := r.GetPlace(p.ID)
	if err != nil || !reflect.DeepEqual(got, p) {
		t.Fatalf("GetPlace = %+v, %v, want %+v", got, err, p)
	}
	list, err := r.ListPlaces()
	if err != nil || len(list) != 1 || !reflect.DeepEqual(list[0], p) {
		t.Fatalf("ListPlaces = %+v, %v", list, err)
	}
	if _, err := r.GetPlace("nope"); !errors.Is(err, store.ErrPlaceNotFound) {
		t.Fatalf("GetPlace(nope) = %v, want ErrPlaceNotFound", err)
	}
}

func TestNewPlacesQueueBehindTheOthers(t *testing.T) {
	r, _ := placesRepo(t)
	first := mustWritePlace(t, r, store.PlaceWrite{Place: diskPlace()})
	second := mustWritePlace(t, r, store.PlaceWrite{Place: bucketPlace()})
	list, err := r.ListPlaces()
	if err != nil || len(list) != 2 || list[0].ID != first.ID || list[1].ID != second.ID || second.SortOrder <= first.SortOrder {
		t.Fatalf("ListPlaces = %+v, %v, want %s before %s", list, err, first.Name, second.Name)
	}
}

func TestAPlaceNeedsAUniqueNameAndABase(t *testing.T) {
	r, _ := placesRepo(t)
	first := mustWritePlace(t, r, store.PlaceWrite{Place: bucketPlace()})
	second := diskPlace()
	second.Name = first.Name
	if _, err := r.WritePlace(store.PlaceWrite{Place: second}); !errors.Is(err, store.ErrPlaceNameTaken) {
		t.Fatalf("a second place named %q = %v, want ErrPlaceNameTaken", first.Name, err)
	}
	if _, err := r.WritePlace(store.PlaceWrite{Place: first}); err != nil {
		t.Fatalf("saving a place under its own name: %v", err)
	}
	for _, bad := range []store.Place{{Name: " ", Base: "backups"}, {Name: "No base", Base: ""}} {
		if _, err := r.WritePlace(store.PlaceWrite{Place: bad}); err == nil {
			t.Errorf("WritePlace(%+v) wrote a place without a name or base", bad)
		}
	}
}

func TestSavingAnUnchangedPlaceKeepsItsUpdateTime(t *testing.T) {
	r, db := placesRepo(t)
	p := mustWritePlace(t, r, store.PlaceWrite{Place: diskPlace()})
	if _, err := db.Exec(`UPDATE storage_places SET updated_at = 1 WHERE id = ?`, p.ID); err != nil {
		t.Fatal(err)
	}
	p.UpdatedAt = 1
	if same := mustWritePlace(t, r, store.PlaceWrite{Place: p}); same.UpdatedAt != 1 {
		t.Fatalf("an unchanged save moved updated_at to %d", same.UpdatedAt)
	}
	p.Name = "Unraid 2"
	if renamed := mustWritePlace(t, r, store.PlaceWrite{Place: p}); renamed.UpdatedAt <= 1 || renamed.Name != "Unraid 2" {
		t.Fatalf("renamed = %+v, want the new name and a new updated_at", renamed)
	}
}

func TestAPlaceWriteSetsHomePlacesAndTheCredentialBlob(t *testing.T) {
	r, _ := placesRepo(t)
	disk := mustWritePlace(t, r, store.PlaceWrite{Place: diskPlace(),
		HomeDomains: map[string]string{"containers": "", "vms": ""}, CredSetsBlob: []byte("sealed")})
	homes, err := r.DomainPlaces()
	if err != nil || !reflect.DeepEqual(homes, map[string]string{"containers": disk.ID, "vms": disk.ID}) {
		t.Fatalf("DomainPlaces = %v, %v, want containers and vms at %s", homes, err, disk.ID)
	}
	mustWritePlace(t, r, store.PlaceWrite{Place: disk})
	if s := storedSettings(t, r); s.CloudCredSets != "sealed" {
		t.Fatalf("cloud_cred_sets = %q, want the blob kept by a write without one", s.CloudCredSets)
	}
	other := mustWritePlace(t, r, store.PlaceWrite{Place: bucketPlace()})
	if _, err := r.WritePlace(store.PlaceWrite{Place: disk, HomeDomains: map[string]string{"flash": other.ID}}); err == nil {
		t.Fatal("a write of one place made another place a home place")
	}
	if homes, err := r.DomainPlaces(); err != nil || homes["flash"] != "" {
		t.Fatalf("DomainPlaces = %v, %v, want no home for flash", homes, err)
	}
}

func TestSetDomainPlaceTxTakesKnownDomainsAndPlacesOnly(t *testing.T) {
	r, db := placesRepo(t)
	disk := mustWritePlace(t, r, store.PlaceWrite{Place: diskPlace()})
	set := func(domain, placeID string) error {
		return runTx(t, db, func(tx *sql.Tx) error { return r.SetDomainPlaceTx(tx, domain, placeID) })
	}
	if err := set("containers", disk.ID); err != nil {
		t.Fatal(err)
	}
	if err := set("music", disk.ID); err == nil {
		t.Error("an unknown domain got a home place")
	}
	if err := set("vms", "nope"); !errors.Is(err, store.ErrPlaceNotFound) {
		t.Errorf("an unknown place as home = %v, want ErrPlaceNotFound", err)
	}
	if err := set("containers", ""); err != nil {
		t.Fatal(err)
	}
	if homes, err := r.DomainPlaces(); err != nil || len(homes) != 0 {
		t.Fatalf("DomainPlaces = %v, %v, want none left", homes, err)
	}
}
