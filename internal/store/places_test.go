package store_test

import (
	"database/sql"
	"errors"
	"maps"
	"reflect"
	"slices"
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

// addressAt is p's address for domain plus suffix.
func addressAt(t *testing.T, p store.Place, domain, suffix string) string {
	t.Helper()
	addr, ok := places.Address(p.Base, p.Folders, domain, suffix)
	if !ok {
		t.Fatalf("%s has no folder for %s", p.Name, domain)
	}
	return addr
}

// totalChanges counts the rows the store's one connection has written so far.
func totalChanges(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	var n int64
	if err := db.QueryRow(`SELECT total_changes()`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// checkPlacedAddresses fails t unless every row at a place holds the address
// its place spells for it, and every domain with a home place has that
// place's address as its path.
func checkPlacedAddresses(t *testing.T, r *store.Repo) {
	t.Helper()
	list, err := r.ListPlaces()
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]store.Place{}
	for _, p := range list {
		byID[p.ID] = p
	}
	rows, err := r.ListOffsiteTargets()
	if err != nil {
		t.Fatal(err)
	}
	named, err := r.ListNamedRepos()
	if err != nil {
		t.Fatal(err)
	}
	rows = append(rows, named...)
	for _, d := range places.Domains {
		primary, found, err := r.PrimaryRemoteTarget(d)
		if err != nil {
			t.Fatal(err)
		}
		if found {
			rows = append(rows, primary)
		}
	}
	for _, row := range rows {
		if row.PlaceID == "" {
			continue
		}
		p, ok := byID[row.PlaceID]
		if !ok {
			t.Errorf("row %s (%s) sits at %s, which is no place", row.ID, row.Name, row.PlaceID)
			continue
		}
		if want, offered := places.Address(p.Base, p.Folders, row.PlaceDomain, row.PlaceSuffix); !offered || row.Repo != want {
			t.Errorf("row %s (%s) holds %q, place %s spells %q (offered %v)", row.ID, row.Name, row.Repo, p.Name, want, offered)
		}
	}
	homes, err := r.DomainPlaces()
	if err != nil {
		t.Fatal(err)
	}
	s := storedSettings(t, r)
	paths := map[string]string{"containers": s.ContainersPath, "vms": s.VMsPath, "flash": s.FlashPath, "config": s.ConfigPath, "files": s.FilesPath}
	for domain, id := range homes {
		p := byID[id]
		if want, _ := places.Address(p.Base, p.Folders, domain, ""); paths[domain] != want {
			t.Errorf("the %s path is %q, its home place %s spells %q", domain, paths[domain], p.Name, want)
		}
	}
}

func TestAPlaceWritesItselfOntoEveryRowAtIt(t *testing.T) {
	r, db := placesRepo(t)
	bucket := mustWritePlace(t, r, store.PlaceWrite{Place: bucketPlace()})
	target := upsertRow(t, r, store.OffsiteTarget{Domain: "containers", Name: "B2", Repo: addressAt(t, bucket, "containers", ""), Enabled: true})
	named := upsertRow(t, r, store.OffsiteTarget{Role: store.RoleRepo, Name: "B2 files", Repo: addressAt(t, bucket, "files", ""), Enabled: true})
	attachRow(t, db, target.ID, bucket.ID, "containers", "")
	attachRow(t, db, named.ID, bucket.ID, "files", "")

	bucket.Base = "s3:https://s3.example.com/bucket-2"
	bucket.CredsRef, bucket.StorageClass, bucket.Immutable = "set-b2-new", "STANDARD_IA", true
	bucket.RetentionKeepLast, bucket.RetentionKeepDaily, bucket.RetentionKeepWeekly, bucket.RetentionKeepMonthly = 3, 14, 8, 12
	bucket.LimitUpload, bucket.LimitDownload, bucket.GrowthBudgetGB = 2000, 4000, 90
	bucket = mustWritePlace(t, r, store.PlaceWrite{Place: bucket})

	want := target
	want.Repo = "s3:https://s3.example.com/bucket-2/container"
	want.CredsRef, want.StorageClass, want.Immutable = "set-b2-new", "STANDARD_IA", true
	want.RetentionKeepLast, want.RetentionKeepDaily, want.RetentionKeepWeekly, want.RetentionKeepMonthly = 3, 14, 8, 12
	want.LimitUpload, want.LimitDownload, want.GrowthBudgetGB = 2000, 4000, 90
	want.PlaceID, want.PlaceDomain = bucket.ID, "containers"
	if got, _, err := r.GetOffsiteTarget(target.ID); err != nil || got != want {
		t.Fatalf("target = %+v, %v, want %+v", got, err, want)
	}
	repo, err := r.GetNamedRepo(named.ID)
	if err != nil || repo.Repo != "s3:https://s3.example.com/bucket-2/files" || repo.CredsRef != "set-b2-new" || !repo.OffPremises || repo.Name != "B2 files" {
		t.Fatalf("named repository = %+v, %v, want the place's address, credentials and site under its own name", repo, err)
	}
	checkPlacedAddresses(t, r)
}

func TestADirectRepositoryFollowsItsPlaceAndKeepsItsCredentials(t *testing.T) {
	r, db := placesRepo(t)
	bucket := mustWritePlace(t, r, store.PlaceWrite{Place: bucketPlace()})
	target := upsertRow(t, r, store.OffsiteTarget{Domain: "containers", Name: "B2", Repo: addressAt(t, bucket, "containers", ""),
		CredsRef: "set-b2", Enabled: true})
	direct, err := r.CreateCompanionRepo(target.ID, "B2 direct", addressAt(t, bucket, "containers", "-direct"))
	if err != nil {
		t.Fatal(err)
	}
	attachRow(t, db, target.ID, bucket.ID, "containers", "")
	attachRow(t, db, direct.ID, bucket.ID, "containers", "-direct")

	bucket.Base, bucket.CredsRef, bucket.RetentionKeepDaily = "s3:https://s3.example.com/bucket-2", "set-b2-new", 60
	bucket = mustWritePlace(t, r, store.PlaceWrite{Place: bucket})

	got, found, err := r.CompanionFor(target.ID)
	if err != nil || !found {
		t.Fatalf("CompanionFor = %v, %v", found, err)
	}
	if got.Repo != "s3:https://s3.example.com/bucket-2/container-direct" || got.RetentionKeepDaily != 60 ||
		got.StorageClass != bucket.StorageClass || got.CredsRef != "set-b2" {
		t.Fatalf("direct repository = %+v, want the new address and retention on its old credentials", got)
	}
	checkPlacedAddresses(t, r)
}

// switchesAt maps each row at the place to whether it is on.
func switchesAt(t *testing.T, r *store.Repo, placeID string) map[string]bool {
	t.Helper()
	rows, err := r.PlaceRows(placeID)
	if err != nil {
		t.Fatal(err)
	}
	on := map[string]bool{}
	for _, row := range rows {
		on[row.ID] = row.Enabled
	}
	return on
}

func TestAPlaceSwitchedBackOnBringsBackOnlyTheRowsItSwitchedOff(t *testing.T) {
	r, db := placesRepo(t)
	bucket := mustWritePlace(t, r, store.PlaceWrite{Place: bucketPlace()})
	containers := upsertRow(t, r, store.OffsiteTarget{Domain: "containers", Name: "B2", Repo: addressAt(t, bucket, "containers", ""), Enabled: true})
	direct, err := r.CreateCompanionRepo(containers.ID, "B2 direct", addressAt(t, bucket, "containers", "-direct"))
	if err != nil {
		t.Fatal(err)
	}
	flash := upsertRow(t, r, store.OffsiteTarget{Domain: "flash", Name: "B2", Repo: addressAt(t, bucket, "flash", ""), Enabled: false})
	attachRow(t, db, containers.ID, bucket.ID, "containers", "")
	attachRow(t, db, direct.ID, bucket.ID, "containers", "-direct")
	attachRow(t, db, flash.ID, bucket.ID, "flash", "")
	own := map[string]bool{containers.ID: true, direct.ID: true, flash.ID: false}
	off := map[string]bool{containers.ID: false, direct.ID: false, flash.ID: false}

	bucket.RetentionKeepDaily = 90
	bucket = mustWritePlace(t, r, store.PlaceWrite{Place: bucket})
	if got := switchesAt(t, r, bucket.ID); !maps.Equal(got, own) {
		t.Fatalf("after an edit of the place the rows are %v, want %v", got, own)
	}
	for range 2 {
		bucket.Enabled = false
		bucket = mustWritePlace(t, r, store.PlaceWrite{Place: bucket})
		if got := switchesAt(t, r, bucket.ID); !maps.Equal(got, off) {
			t.Fatalf("with the place off the rows are %v, want every one off", got)
		}
		bucket.Enabled = true
		bucket = mustWritePlace(t, r, store.PlaceWrite{Place: bucket})
		if got := switchesAt(t, r, bucket.ID); !maps.Equal(got, own) {
			t.Fatalf("with the place back on the rows are %v, want %v", got, own)
		}
	}
}

func TestARowSavedWhileItsPlaceIsOffKeepsItsOwnSwitch(t *testing.T) {
	r, db := placesRepo(t)
	bucket := mustWritePlace(t, r, store.PlaceWrite{Place: bucketPlace()})
	renamed := upsertRow(t, r, store.OffsiteTarget{Domain: "containers", Name: "B2", Repo: addressAt(t, bucket, "containers", ""), Enabled: true})
	switched := upsertRow(t, r, store.OffsiteTarget{Domain: "flash", Name: "B2", Repo: addressAt(t, bucket, "flash", ""), Enabled: true})
	direct, err := r.CreateCompanionRepo(renamed.ID, "B2 direct", addressAt(t, bucket, "containers", "-direct"))
	if err != nil {
		t.Fatal(err)
	}
	attachRow(t, db, renamed.ID, bucket.ID, "containers", "")
	attachRow(t, db, switched.ID, bucket.ID, "flash", "")
	attachRow(t, db, direct.ID, bucket.ID, "containers", "-direct")
	bucket.Enabled = false
	bucket = mustWritePlace(t, r, store.PlaceWrite{Place: bucket})
	stored := func(id string) store.OffsiteTarget {
		t.Helper()
		rows, err := r.PlaceRows(bucket.ID)
		if err != nil {
			t.Fatal(err)
		}
		i := slices.IndexFunc(rows, func(row store.OffsiteTarget) bool { return row.ID == id })
		if i < 0 {
			t.Fatalf("row %s is not at %s", id, bucket.Name)
		}
		return rows[i]
	}

	row := stored(renamed.ID)
	row.Name = "B2 renamed"
	upsertRow(t, r, row)
	for _, id := range []string{switched.ID, direct.ID} {
		row = stored(id)
		row.Enabled = true
		upsertRow(t, r, row)
		row.Enabled = false
		upsertRow(t, r, row)
	}
	bucket.Enabled = true
	mustWritePlace(t, r, store.PlaceWrite{Place: bucket})

	want := map[string]bool{renamed.ID: true, switched.ID: false, direct.ID: false}
	if got := switchesAt(t, r, bucket.ID); !maps.Equal(got, want) {
		t.Fatalf("with the place back on the rows are %v, want the renamed one on and the ones switched off off", got)
	}
}

func TestSavingAnUnchangedSwitchedOffPlaceWritesNothing(t *testing.T) {
	r, db := placesRepo(t)
	bucket := mustWritePlace(t, r, store.PlaceWrite{Place: bucketPlace()})
	on := upsertRow(t, r, store.OffsiteTarget{Domain: "containers", Name: "B2", Repo: addressAt(t, bucket, "containers", ""), Enabled: true})
	off := upsertRow(t, r, store.OffsiteTarget{Domain: "flash", Name: "B2", Repo: addressAt(t, bucket, "flash", ""), Enabled: false})
	attachRow(t, db, on.ID, bucket.ID, "containers", "")
	attachRow(t, db, off.ID, bucket.ID, "flash", "")

	for _, enabled := range []bool{false, true} {
		bucket.Enabled = enabled
		bucket = mustWritePlace(t, r, store.PlaceWrite{Place: bucket})
		changes := totalChanges(t, db)
		mustWritePlace(t, r, store.PlaceWrite{Place: bucket})
		if n := totalChanges(t, db) - changes; n != 0 {
			t.Fatalf("saving an unchanged place that is on=%v wrote %d rows", enabled, n)
		}
	}
}

func TestAPrimaryRowComesOnWithItsHomePlace(t *testing.T) {
	r, _ := placesRepo(t)
	if _, err := r.UpsertPrimaryRemoteTarget("vms", store.OffsiteTarget{Repo: "rest:http://old:8000/vms", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	tower := restPlace()
	tower.Enabled = false
	tower = mustWritePlace(t, r, store.PlaceWrite{Place: tower, HomeDomains: map[string]string{"containers": "", "vms": ""}})
	primaries := func() map[string]bool {
		t.Helper()
		on := map[string]bool{}
		for _, d := range []string{"containers", "vms"} {
			row, found, err := r.PrimaryRemoteTarget(d)
			if err != nil || !found || row.PlaceID != tower.ID {
				t.Fatalf("primary row of %s = %+v, %v, %v, want one at %s", d, row, found, err, tower.Name)
			}
			on[d] = row.Enabled
		}
		return on
	}
	if got := primaries(); got["containers"] || got["vms"] {
		t.Fatalf("primary rows at a place that is off = %v, want both off", got)
	}
	tower.Enabled = true
	mustWritePlace(t, r, store.PlaceWrite{Place: tower})
	if got := primaries(); !got["containers"] || !got["vms"] {
		t.Fatalf("primary rows with their place back on = %v, want both on", got)
	}
}

func TestARowThatLeavesItsSwitchedOffPlaceLeavesItsMarkThere(t *testing.T) {
	for name, leave := range map[string]func(t *testing.T, r *store.Repo, db *sql.DB, rowID, placeID string) error{
		"detached": func(t *testing.T, _ *store.Repo, db *sql.DB, rowID, _ string) error {
			return runTx(t, db, func(tx *sql.Tx) error { return store.DetachRowTx(tx, rowID) })
		},
		"moved through an old route": func(_ *testing.T, r *store.Repo, _ *sql.DB, rowID, _ string) error {
			_, err := r.SetNamedRepoLocationIfUnused(rowID, "s3:https://s3.example.com/other/container-direct", true)
			return err
		},
		"its place removed": func(_ *testing.T, r *store.Repo, _ *sql.DB, _, placeID string) error {
			_, err := r.DeletePlaceIfUnused(placeID)
			return err
		},
		"every place dropped": func(_ *testing.T, r *store.Repo, _ *sql.DB, _, _ string) error {
			return r.DropPlaces()
		},
	} {
		t.Run(name, func(t *testing.T) {
			r, db := placesRepo(t)
			bucket := mustWritePlace(t, r, store.PlaceWrite{Place: bucketPlace()})
			target := upsertRow(t, r, store.OffsiteTarget{Domain: "containers", Name: "Other", Repo: "s3:https://s3.example.com/other/container", Enabled: true})
			direct, err := r.CreateCompanionRepo(target.ID, "B2 direct", addressAt(t, bucket, "containers", "-direct"))
			if err != nil {
				t.Fatal(err)
			}
			attachRow(t, db, direct.ID, bucket.ID, "containers", "-direct")
			bucket.Enabled = false
			mustWritePlace(t, r, store.PlaceWrite{Place: bucket})
			if got, err := r.GetNamedRepo(direct.ID); err != nil || got.Enabled || !got.OffWithPlace {
				t.Fatalf("direct repository at a place that is off = %+v, %v, want it off and marked", got, err)
			}

			if err := leave(t, r, db, direct.ID, bucket.ID); err != nil {
				t.Fatal(err)
			}
			if got, err := r.GetNamedRepo(direct.ID); err != nil || got.PlaceID != "" || got.OffWithPlace {
				t.Fatalf("direct repository = %+v, %v, want it at no place without the mark", got, err)
			}
		})
	}
}

func TestARowThatLeavesASwitchedOffPlaceStaysOffAtTheNextOne(t *testing.T) {
	r, db := placesRepo(t)
	bucket := mustWritePlace(t, r, store.PlaceWrite{Place: bucketPlace()})
	row := upsertRow(t, r, store.OffsiteTarget{Domain: "containers", Name: "B2", Repo: addressAt(t, bucket, "containers", ""), Enabled: true})
	attachRow(t, db, row.ID, bucket.ID, "containers", "")
	bucket.Enabled = false
	mustWritePlace(t, r, store.PlaceWrite{Place: bucket})
	tower := mustWritePlace(t, r, store.PlaceWrite{Place: restPlace()})

	if _, err := r.AdoptRow(row.ID, tower.ID, "containers", ""); err != nil {
		t.Fatal(err)
	}
	mustWritePlace(t, r, store.PlaceWrite{Place: tower})
	if got := switchesAt(t, r, tower.ID); got[row.ID] {
		t.Fatal("a row that came off at its old place was switched on by the next one")
	}
}

func TestAPlaceKeepsTheFolderOfEveryRowAtIt(t *testing.T) {
	r, db := placesRepo(t)
	bucket := mustWritePlace(t, r, store.PlaceWrite{Place: bucketPlace()})
	row := upsertRow(t, r, store.OffsiteTarget{Domain: "vms", Name: "B2", Repo: addressAt(t, bucket, "vms", ""), Enabled: true})
	attachRow(t, db, row.ID, bucket.ID, "vms", "")

	bucket.Folders = map[string]string{"containers": "container"}
	if _, err := r.WritePlace(store.PlaceWrite{Place: bucket}); !errors.Is(err, store.ErrPlaceFolderMissing) {
		t.Fatalf("dropping the folder of a row = %v, want ErrPlaceFolderMissing", err)
	}
	stored, err := r.GetPlace(bucket.ID)
	if err != nil || stored.Folders["vms"] != "vms" {
		t.Fatalf("place = %+v, %v, want its folders unchanged", stored, err)
	}
}

func TestSavingAnUnchangedPlaceWritesNothing(t *testing.T) {
	r, db := placesRepo(t)
	bucket := mustWritePlace(t, r, store.PlaceWrite{Place: bucketPlace()})
	target := upsertRow(t, r, store.OffsiteTarget{Domain: "containers", Name: "B2", Repo: addressAt(t, bucket, "containers", ""), Enabled: true})
	direct, err := r.CreateCompanionRepo(target.ID, "B2 direct", addressAt(t, bucket, "containers", "-direct"))
	if err != nil {
		t.Fatal(err)
	}
	named := upsertRow(t, r, store.OffsiteTarget{Role: store.RoleRepo, Name: "B2 files", Repo: addressAt(t, bucket, "files", ""), Enabled: true})
	attachRow(t, db, target.ID, bucket.ID, "containers", "")
	attachRow(t, db, direct.ID, bucket.ID, "containers", "-direct")
	attachRow(t, db, named.ID, bucket.ID, "files", "")
	bucket = mustWritePlace(t, r, store.PlaceWrite{Place: bucket})

	changes := totalChanges(t, db)
	mustWritePlace(t, r, store.PlaceWrite{Place: bucket})
	if n := totalChanges(t, db) - changes; n != 0 {
		t.Fatalf("saving an unchanged place wrote %d rows", n)
	}
}

// fieldRowAt is the domain's off-site field row, at p and written from it.
func fieldRowAt(t *testing.T, r *store.Repo, db *sql.DB, p store.Place, domain string) store.OffsiteTarget {
	t.Helper()
	row := upsertRow(t, r, store.OffsiteTarget{Domain: domain, Name: p.Name, Repo: addressAt(t, p, domain, ""), Enabled: true})
	attachRow(t, db, row.ID, p.ID, domain, "")
	mustWritePlace(t, r, store.PlaceWrite{Place: p})
	got, _, err := r.GetOffsiteTarget(row.ID)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestAHomePlaceWritesTheDomainPaths(t *testing.T) {
	r, _ := placesRepo(t)
	disk := diskPlace()
	disk.Base = "backups"
	disk = mustWritePlace(t, r, store.PlaceWrite{Place: disk, HomeDomains: map[string]string{"containers": "", "flash": ""}})
	s := storedSettings(t, r)
	if s.ContainersPath != "backups/container" || s.FlashPath != "backups/flash" || s.VMsPath != "user/bombvault/vms" {
		t.Fatalf("paths = %q, %q, %q, want the home place's for containers and flash and vms untouched", s.ContainersPath, s.FlashPath, s.VMsPath)
	}
	disk.Folders["flash"] = "usb"
	mustWritePlace(t, r, store.PlaceWrite{Place: disk})
	if s := storedSettings(t, r); s.FlashPath != "backups/usb" {
		t.Fatalf("flash path = %q, want the new folder", s.FlashPath)
	}
	checkPlacedAddresses(t, r)
}

func TestAHomePlaceNeedsTheDomainsFolder(t *testing.T) {
	r, _ := placesRepo(t)
	disk := diskPlace()
	disk.Folders = map[string]string{"containers": "container"}
	if _, err := r.WritePlace(store.PlaceWrite{Place: disk, HomeDomains: map[string]string{"vms": ""}}); !errors.Is(err, store.ErrPlaceFolderMissing) {
		t.Fatalf("a home place without the domain's folder = %v, want ErrPlaceFolderMissing", err)
	}
	if list, err := r.ListPlaces(); err != nil || len(list) != 0 {
		t.Fatalf("ListPlaces = %+v, %v, want the refused write gone", list, err)
	}
}

func TestTheFieldRowAtAPlaceWritesTheOffsiteField(t *testing.T) {
	r, db := placesRepo(t)
	bucket := mustWritePlace(t, r, store.PlaceWrite{Place: bucketPlace()})
	field := fieldRowAt(t, r, db, bucket, "containers")
	if s := storedSettings(t, r); s.ContainersOffsite != field.Repo || s.ContainersOffsiteImmutable {
		t.Fatalf("field = %q, %v, want the row's address", s.ContainersOffsite, s.ContainersOffsiteImmutable)
	}
	copies, err := r.CreateOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "B2 copies",
		Repo: addressAt(t, bucket, "containers", "-copies"), Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	attachRow(t, db, copies.ID, bucket.ID, "containers", "-copies")

	bucket.Base, bucket.Immutable = "s3:https://s3.example.com/bucket-2", true
	bucket = mustWritePlace(t, r, store.PlaceWrite{Place: bucket})
	if s := storedSettings(t, r); s.ContainersOffsite != "s3:https://s3.example.com/bucket-2/container" || !s.ContainersOffsiteImmutable {
		t.Fatalf("field = %q, %v, want the field row's new address, append-only", s.ContainersOffsite, s.ContainersOffsiteImmutable)
	}

	off, _, err := r.GetOffsiteTarget(field.ID)
	if err != nil {
		t.Fatal(err)
	}
	off.Enabled = false
	upsertRow(t, r, off)
	mustWritePlace(t, r, store.PlaceWrite{Place: bucket})
	if s := storedSettings(t, r); s.ContainersOffsite != "" || s.ContainersOffsiteImmutable {
		t.Fatalf("field = %q, %v, want it empty under a row that is off", s.ContainersOffsite, s.ContainersOffsiteImmutable)
	}
}

func TestAnUnchangedHomePlaceWritesNothing(t *testing.T) {
	r, db := placesRepo(t)
	disk := mustWritePlace(t, r, store.PlaceWrite{Place: diskPlace(), HomeDomains: map[string]string{"containers": "", "vms": ""}})
	bucket := mustWritePlace(t, r, store.PlaceWrite{Place: bucketPlace()})
	fieldRowAt(t, r, db, bucket, "flash")

	changes := totalChanges(t, db)
	mustWritePlace(t, r, store.PlaceWrite{Place: disk, HomeDomains: map[string]string{"containers": ""}})
	mustWritePlace(t, r, store.PlaceWrite{Place: bucket})
	if n := totalChanges(t, db) - changes; n != 0 {
		t.Fatalf("saving unchanged places wrote %d rows", n)
	}
}

// restPlace is a rest-server in another house with its own credentials, caps
// and append-only flag.
func restPlace() store.Place {
	return store.Place{Name: "Tower", Provider: "rest-server", Kind: string(places.KindREST), Base: "rest:http://tower:8000/bv",
		Folders: places.DefaultFolders(), CredsRef: "set-tower", Immutable: true, LimitUpload: 500, GrowthBudgetGB: 20,
		OffPremises: true, Enabled: true}
}

func TestARemoteHomePlaceKeepsTheDomainsPrimaryRow(t *testing.T) {
	r, db := placesRepo(t)
	tower := mustWritePlace(t, r, store.PlaceWrite{Place: restPlace(), HomeDomains: map[string]string{"containers": ""}})

	row, found, err := r.PrimaryRemoteTarget("containers")
	if err != nil || !found {
		t.Fatalf("PrimaryRemoteTarget = %v, %v, want a row", found, err)
	}
	if row.Repo != "rest:http://tower:8000/bv/container" || row.PlaceID != tower.ID || row.PlaceDomain != "containers" ||
		!row.Enabled || !row.Immutable || row.LimitUpload != 500 || row.GrowthBudgetGB != 20 || row.CredsRef != "set-tower" {
		t.Fatalf("primary row = %+v, want the place's address and safety settings", row)
	}
	if s := storedSettings(t, r); s.ContainersPath != row.Repo {
		t.Fatalf("containers path = %q, want %q", s.ContainersPath, row.Repo)
	}
	changes := totalChanges(t, db)
	mustWritePlace(t, r, store.PlaceWrite{Place: tower, HomeDomains: map[string]string{"containers": ""}})
	if n := totalChanges(t, db) - changes; n != 0 {
		t.Fatalf("saving an unchanged home place wrote %d rows", n)
	}
	checkPlacedAddresses(t, r)
}

func TestAnExistingPrimaryRowMovesOntoTheNewHomePlace(t *testing.T) {
	r, _ := placesRepo(t)
	old, err := r.UpsertPrimaryRemoteTarget("vms", store.OffsiteTarget{Repo: "rest:http://old:8000/vms", CredsRef: "set-old", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	tower := mustWritePlace(t, r, store.PlaceWrite{Place: restPlace(), HomeDomains: map[string]string{"vms": ""}})

	row, _, err := r.PrimaryRemoteTarget("vms")
	if err != nil || row.ID != old.ID || row.PlaceID != tower.ID || row.Repo != "rest:http://tower:8000/bv/vms" || row.CredsRef != "set-tower" {
		t.Fatalf("primary row = %+v, %v, want row %s moved onto %s", row, err, old.ID, tower.Name)
	}
	checkPlacedAddresses(t, r)
}

func TestALocalHomePlaceSwitchesThePrimaryRowOff(t *testing.T) {
	r, _ := placesRepo(t)
	tower := mustWritePlace(t, r, store.PlaceWrite{Place: restPlace(), HomeDomains: map[string]string{"containers": ""}})
	first, found, err := r.PrimaryRemoteTarget("containers")
	if err != nil || !found || !first.Enabled {
		t.Fatalf("primary row under a remote home = %+v, %v, %v, want one that is on", first, found, err)
	}

	disk := mustWritePlace(t, r, store.PlaceWrite{Place: diskPlace(), HomeDomains: map[string]string{"containers": ""}})
	off, _, err := r.PrimaryRemoteTarget("containers")
	if err != nil || off.ID != first.ID || off.Enabled || off.PlaceID != "" {
		t.Fatalf("primary row under a local home = %+v, %v, want row %s off and at no place", off, err, first.ID)
	}
	if s := storedSettings(t, r); s.ContainersPath != addressAt(t, disk, "containers", "") {
		t.Fatalf("containers path = %q, want the local place's", s.ContainersPath)
	}

	mustWritePlace(t, r, store.PlaceWrite{Place: tower, HomeDomains: map[string]string{"containers": ""}})
	back, _, err := r.PrimaryRemoteTarget("containers")
	if err != nil || back.ID != first.ID || !back.Enabled || back.PlaceID != tower.ID {
		t.Fatalf("primary row back at a remote home = %+v, %v, want row %s on at %s", back, err, first.ID, tower.Name)
	}
	checkPlacedAddresses(t, r)
}

func TestAPlaceIsHeldByWhatUsesIt(t *testing.T) {
	r, db := placesRepo(t)
	disk := mustWritePlace(t, r, store.PlaceWrite{Place: diskPlace(), HomeDomains: map[string]string{"containers": ""}})
	bucket := mustWritePlace(t, r, store.PlaceWrite{Place: bucketPlace()})
	named := upsertRow(t, r, store.OffsiteTarget{Role: store.RoleRepo, Name: "B2 files", Repo: addressAt(t, bucket, "files", ""), Enabled: true})
	attachRow(t, db, named.ID, bucket.ID, "files", "")
	if _, err := r.WritePlacement(store.ItemRef{Domain: "containers", Key: "nginx"}, &store.HomeWrite{Repo: named.ID, Choice: store.RepoChosen}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := r.PutPlacementDefault("vms", named.ID, nil); err != nil {
		t.Fatal(err)
	}
	target := upsertRow(t, r, store.OffsiteTarget{Domain: "vms", Name: "B2", Repo: addressAt(t, bucket, "vms", ""), Enabled: true})
	attachRow(t, db, target.ID, bucket.ID, "vms", "")
	direct, err := r.CreateCompanionRepo(target.ID, "B2 direct", "s3:https://s3.example.com/other/vms")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.WritePlacement(store.ItemRef{Domain: "vms", Key: "win11"}, &store.HomeWrite{Repo: direct.ID, Choice: store.RepoChosen}, nil, nil); err != nil {
		t.Fatal(err)
	}

	home, err := r.PlaceHolders(disk.ID)
	if err != nil || !home.InUse() || !slices.Equal(home.HomeDomains, []string{"containers"}) {
		t.Fatalf("holders of the home place = %+v, %v, want containers", home, err)
	}
	h, err := r.PlaceHolders(bucket.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.HomeDomains) != 0 || !slices.Equal(h.Defaults, []string{"vms"}) ||
		!reflect.DeepEqual(h.Items, []store.ItemRef{{Domain: "containers", Key: "nginx"}}) || !slices.Equal(h.DirectInUse, []string{target.ID}) {
		t.Fatalf("holders of the bucket = %+v", h)
	}
	if _, err := r.DeletePlaceIfUnused(bucket.ID); !errors.Is(err, store.ErrPlaceInUse) {
		t.Fatalf("DeletePlaceIfUnused = %v, want ErrPlaceInUse", err)
	}
	if _, err := r.GetPlace(bucket.ID); err != nil {
		t.Fatalf("a held place is gone: %v", err)
	}
	if _, err := r.GetNamedRepo(named.ID); err != nil {
		t.Fatalf("a repository of a held place is gone: %v", err)
	}
}

func TestAnUnusedPlaceGoesWithWhatWasDerivedFromIt(t *testing.T) {
	r, db := placesRepo(t)
	bucket := mustWritePlace(t, r, store.PlaceWrite{Place: bucketPlace()})
	target := upsertRow(t, r, store.OffsiteTarget{Domain: "containers", Name: "B2", Repo: addressAt(t, bucket, "containers", ""), Enabled: true})
	attachRow(t, db, target.ID, bucket.ID, "containers", "")
	direct, err := r.CreateCompanionRepo(target.ID, "B2 direct", addressAt(t, bucket, "containers", "-direct"))
	if err != nil {
		t.Fatal(err)
	}
	attachRow(t, db, direct.ID, bucket.ID, "containers", "-direct")
	named := upsertRow(t, r, store.OffsiteTarget{Role: store.RoleRepo, Name: "B2 files", Repo: addressAt(t, bucket, "files", ""), Enabled: true})
	attachRow(t, db, named.ID, bucket.ID, "files", "")
	moved := upsertRow(t, r, store.OffsiteTarget{Domain: "vms", Name: "B2 vms", Repo: addressAt(t, bucket, "vms", ""), Enabled: true})
	movedDirect, err := r.CreateCompanionRepo(moved.ID, "B2 vms direct", addressAt(t, bucket, "vms", "-direct"))
	if err != nil {
		t.Fatal(err)
	}
	attachRow(t, db, movedDirect.ID, bucket.ID, "vms", "-direct")
	primary, err := r.UpsertPrimaryRemoteTarget("flash", store.OffsiteTarget{Repo: addressAt(t, bucket, "flash", ""), Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	attachRow(t, db, primary.ID, bucket.ID, "flash", "")

	removed, err := r.DeletePlaceIfUnused(bucket.ID)
	if err != nil || removed != 1 {
		t.Fatalf("DeletePlaceIfUnused = %d, %v, want one target removed", removed, err)
	}
	if _, err := r.GetPlace(bucket.ID); !errors.Is(err, store.ErrPlaceNotFound) {
		t.Fatalf("GetPlace = %v, want ErrPlaceNotFound", err)
	}
	if _, found, _ := r.GetOffsiteTarget(target.ID); found {
		t.Error("the place's target is still there")
	}
	if _, found, _ := r.CompanionFor(target.ID); found {
		t.Error("the target's direct repository is still there")
	}
	if _, err := r.GetNamedRepo(named.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("the place's named repository = %v, want it gone", err)
	}
	if got, found, err := r.GetOffsiteTarget(moved.ID); err != nil || !found || got.PlaceID != "" {
		t.Errorf("a target at no place = %+v, %v, %v, want it untouched", got, found, err)
	}
	if got, err := r.GetNamedRepo(movedDirect.ID); err != nil || got.PlaceID != "" {
		t.Errorf("a direct repository whose target stands elsewhere = %+v, %v, want it kept at no place", got, err)
	}
	if got, found, err := r.PrimaryRemoteTarget("flash"); err != nil || !found || got.PlaceID != "" {
		t.Errorf("the flash primary row = %+v, %v, %v, want it kept at no place", got, found, err)
	}
	checkPlacedAddresses(t, r)
}

func TestRemovingAPlaceEmptiesTheOffsiteFieldsItFilled(t *testing.T) {
	r, db := placesRepo(t)
	bucket := bucketPlace()
	bucket.Immutable = true
	bucket = mustWritePlace(t, r, store.PlaceWrite{Place: bucket})
	fieldRowAt(t, r, db, bucket, "containers")
	tower := mustWritePlace(t, r, store.PlaceWrite{Place: restPlace()})
	kept := fieldRowAt(t, r, db, tower, "vms")

	if _, err := r.DeletePlaceIfUnused(bucket.ID); err != nil {
		t.Fatal(err)
	}
	s := storedSettings(t, r)
	if s.ContainersOffsite != "" || s.ContainersOffsiteImmutable {
		t.Fatalf("containers field = %q, %v, want it empty with its row gone", s.ContainersOffsite, s.ContainersOffsiteImmutable)
	}
	if s.VMsOffsite != kept.Repo || !s.VMsOffsiteImmutable {
		t.Fatalf("vms field = %q, %v, want the row at the other place", s.VMsOffsite, s.VMsOffsiteImmutable)
	}
}

func TestRemovingAnUnknownPlaceFindsNothing(t *testing.T) {
	r, _ := placesRepo(t)
	if _, err := r.DeletePlaceIfUnused("nope"); !errors.Is(err, store.ErrPlaceNotFound) {
		t.Fatalf("DeletePlaceIfUnused(nope) = %v, want ErrPlaceNotFound", err)
	}
}

func TestEveryPlacedRowHoldsItsPlaceAddress(t *testing.T) {
	r, db := placesRepo(t)
	bucket := mustWritePlace(t, r, store.PlaceWrite{Place: bucketPlace()})
	mustWritePlace(t, r, store.PlaceWrite{Place: restPlace(), HomeDomains: map[string]string{"vms": ""}})
	nas := mustWritePlace(t, r, store.PlaceWrite{Place: store.Place{Name: "NAS", Provider: "share", Kind: string(places.KindLocal),
		Base: "remotes/nas/bv", Folders: map[string]string{}, Enabled: true}})

	target := upsertRow(t, r, store.OffsiteTarget{Domain: "containers", Name: "B2", Repo: addressAt(t, bucket, "containers", ""), Enabled: true})
	attachRow(t, db, target.ID, bucket.ID, "containers", "")
	direct, err := r.CreateCompanionRepo(target.ID, "B2 direct", addressAt(t, bucket, "containers", "-direct"))
	if err != nil {
		t.Fatal(err)
	}
	attachRow(t, db, direct.ID, bucket.ID, "containers", "-direct")
	copies, err := r.CreateOffsiteTarget(store.OffsiteTarget{Domain: "files", Name: "B2 copies", Repo: addressAt(t, bucket, "files", "-copies"), Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	attachRow(t, db, copies.ID, bucket.ID, "files", "-copies")
	shared := upsertRow(t, r, store.OffsiteTarget{Role: store.RoleRepo, Name: "NAS", Repo: "remotes/nas/bv", Enabled: true})
	attachRow(t, db, shared.ID, nas.ID, "", "")
	bucket = mustWritePlace(t, r, store.PlaceWrite{Place: bucket})
	checkPlacedAddresses(t, r)

	writeBucket := func() error {
		var err error
		bucket, err = r.WritePlace(store.PlaceWrite{Place: bucket})
		return err
	}
	for _, step := range []struct {
		name string
		do   func() error
	}{
		{"a new base", func() error {
			bucket.Base = "s3:https://s3.example.com/bucket-2"
			return writeBucket()
		}},
		{"a new folder", func() error {
			bucket.Folders["containers"] = "ct"
			return writeBucket()
		}},
		{"the root of an rclone remote", func() error {
			bucket.Kind, bucket.Base = string(places.KindRclone), "rclone:r:"
			return writeBucket()
		}},
		{"switched off and on", func() error {
			bucket.Enabled = false
			if err := writeBucket(); err != nil {
				return err
			}
			bucket.Enabled = true
			return writeBucket()
		}},
		{"the home place moved to this server", func() error {
			_, err := r.WritePlace(store.PlaceWrite{Place: diskPlace(), HomeDomains: map[string]string{"vms": ""}})
			return err
		}},
		{"a target moved through an old route", func() error {
			row, _, err := r.GetOffsiteTarget(target.ID)
			if err != nil {
				return err
			}
			row.Repo = "s3:https://s3.example.com/elsewhere/container"
			_, err = r.UpsertOffsiteTarget(row)
			return err
		}},
		{"a repository moved through an old route", func() error {
			_, err := r.SetNamedRepoLocationIfUnused(shared.ID, "remotes/nas2/bv", false)
			return err
		}},
		{"the place written after a row left it", func() error {
			bucket.Base = "rclone:r:bucket-3"
			return writeBucket()
		}},
	} {
		t.Run(step.name, func(t *testing.T) {
			if err := step.do(); err != nil {
				t.Fatal(err)
			}
			checkPlacedAddresses(t, r)
		})
	}
}

func TestAnAddressWrittenThroughAnOldRouteTakesTheRowOffItsPlace(t *testing.T) {
	r, db := placesRepo(t)
	bucket := mustWritePlace(t, r, store.PlaceWrite{Place: bucketPlace()})
	target := upsertRow(t, r, store.OffsiteTarget{Domain: "containers", Name: "B2", Repo: addressAt(t, bucket, "containers", ""), Enabled: true})
	attachRow(t, db, target.ID, bucket.ID, "containers", "")
	named := upsertRow(t, r, store.OffsiteTarget{Role: store.RoleRepo, Name: "B2 files", Repo: addressAt(t, bucket, "files", ""), Enabled: true})
	attachRow(t, db, named.ID, bucket.ID, "files", "")
	used := upsertRow(t, r, store.OffsiteTarget{Role: store.RoleRepo, Name: "B2 vms", Repo: addressAt(t, bucket, "vms", ""), Enabled: true})
	attachRow(t, db, used.ID, bucket.ID, "vms", "")
	if _, err := r.WritePlacement(store.ItemRef{Domain: "vms", Key: "win11"}, &store.HomeWrite{Repo: used.ID, Choice: store.RepoChosen}, nil, nil); err != nil {
		t.Fatal(err)
	}

	upsertRow(t, r, store.OffsiteTarget{ID: target.ID, Domain: "containers", Name: "B2", Repo: target.Repo, Enabled: false})
	if got, _, _ := r.GetOffsiteTarget(target.ID); got.PlaceID != bucket.ID {
		t.Fatalf("a save that kept the address took the target off its place: %+v", got)
	}
	upsertRow(t, r, store.OffsiteTarget{ID: target.ID, Domain: "containers", Name: "B2", Repo: "s3:https://s3.example.com/elsewhere/container", Enabled: true})
	if got, _, _ := r.GetOffsiteTarget(target.ID); got.PlaceID != "" || got.PlaceDomain != "" || got.Repo != "s3:https://s3.example.com/elsewhere/container" {
		t.Fatalf("a moved target = %+v, want it at no place on its new address", got)
	}

	if _, err := r.SetNamedRepoLocationIfUnused(named.ID, "s3:https://s3.example.com/elsewhere/files", true); err != nil {
		t.Fatal(err)
	}
	if got, err := r.GetNamedRepo(named.ID); err != nil || got.PlaceID != "" || got.Repo != "s3:https://s3.example.com/elsewhere/files" {
		t.Fatalf("a moved repository = %+v, %v, want it at no place on its new address", got, err)
	}
	use, err := r.SetNamedRepoLocationIfUnused(used.ID, "s3:https://s3.example.com/elsewhere/vms", true)
	if err != nil || !use.InUse() {
		t.Fatalf("moving a repository in use = %+v, %v, want it refused", use, err)
	}
	if got, err := r.GetNamedRepo(used.ID); err != nil || got.PlaceID != bucket.ID || got.Repo != addressAt(t, bucket, "vms", "") {
		t.Fatalf("a repository in use = %+v, %v, want it still at its place", got, err)
	}
	checkPlacedAddresses(t, r)
}

func TestASettingsSaveKeepsWhatPlacesWrote(t *testing.T) {
	r, db := placesRepo(t)
	mustWritePlace(t, r, store.PlaceWrite{Place: diskPlace(), HomeDomains: map[string]string{"containers": ""}})
	bucket := mustWritePlace(t, r, store.PlaceWrite{Place: bucketPlace()})
	flash := fieldRowAt(t, r, db, bucket, "flash")
	before := storedSettings(t, r)

	after, err := r.MutateSettingsKeepingPlaces(func(s *store.Settings) error {
		s.ContainersPath, s.ContainersOffsite, s.ContainersOffsiteImmutable = "stale/containers", "s3:stale/containers", true
		s.FlashPath, s.FlashOffsite, s.FlashOffsiteImmutable = "elsewhere/flash", "s3:stale/flash", true
		s.VMsPath, s.VMsOffsite = "elsewhere/vms", "s3:elsewhere/vms"
		s.DefaultLanguage = "de"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if stored := storedSettings(t, r); stored != after {
		t.Fatalf("returned settings differ from the stored ones: %+v", after)
	}
	if after.ContainersPath != before.ContainersPath || after.ContainersOffsite != before.ContainersOffsite ||
		after.ContainersOffsiteImmutable != before.ContainersOffsiteImmutable {
		t.Errorf("containers = %q, %q, %v, want what its home place wrote", after.ContainersPath, after.ContainersOffsite, after.ContainersOffsiteImmutable)
	}
	if after.FlashOffsite != flash.Repo || after.FlashOffsiteImmutable != before.FlashOffsiteImmutable || after.FlashPath != "elsewhere/flash" {
		t.Errorf("flash = %q, %q, %v, want the placed field kept and the path saved", after.FlashPath, after.FlashOffsite, after.FlashOffsiteImmutable)
	}
	if after.VMsPath != "elsewhere/vms" || after.VMsOffsite != "s3:elsewhere/vms" || after.DefaultLanguage != "de" {
		t.Errorf("vms = %q, %q, language %q, want everything outside places saved", after.VMsPath, after.VMsOffsite, after.DefaultLanguage)
	}
}
