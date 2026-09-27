package store_test

import (
	"maps"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

const importBase = "s3:https://s3.example.com/bv"

func importRows(t *testing.T, r *store.Repo) {
	t.Helper()
	for _, id := range []string{"row-1", "row-2"} {
		if _, err := r.UpsertOffsiteTarget(store.OffsiteTarget{
			ID: id, Domain: "containers", Name: id, Repo: importBase + "/" + id, Enabled: true,
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func onePlace(id string, folders map[string]string) store.Place {
	return store.Place{ID: id, Name: "B2", Provider: "b2", Kind: "s3", Base: importBase, Folders: folders, Enabled: true}
}

func TestReplacePlacesLeavesOnlyTheImportedPlaces(t *testing.T) {
	r := newRepo(t)
	importRows(t, r)
	if err := r.ReplacePlaces(store.PlacesImport{
		Places:      []store.Place{onePlace("p-old", map[string]string{"containers": "row-2"})},
		HomeDomains: map[string]string{"containers": "p-old"},
		Links:       []store.PlaceLink{{RowID: "row-2", PlaceID: "p-old", Domain: "containers"}},
	}); err != nil {
		t.Fatal(err)
	}

	folders := map[string]string{"containers": "row-1", "vms": "vms"}
	next := onePlace("p-new", folders)
	next.CreatedAt, next.UpdatedAt = 100, 200
	if err := r.ReplacePlaces(store.PlacesImport{
		Places:      []store.Place{next},
		HomeDomains: map[string]string{"vms": "p-new"},
		Links:       []store.PlaceLink{{RowID: "row-1", PlaceID: "p-new", Domain: "containers", Suffix: "-copies"}},
	}); err != nil {
		t.Fatalf("a second replace under the first one's name: %v", err)
	}

	got, err := r.ListPlaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "p-new" || got[0].CreatedAt != 100 || got[0].UpdatedAt != 200 || !maps.Equal(got[0].Folders, folders) {
		t.Fatalf("places = %+v, want only p-new as the file has it", got)
	}
	homes, err := r.DomainPlaces()
	if err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(homes, map[string]string{"vms": "p-new"}) {
		t.Fatalf("home places = %v, want only vms on p-new", homes)
	}
	one, _, err := r.GetOffsiteTarget("row-1")
	if err != nil {
		t.Fatal(err)
	}
	if one.PlaceID != "p-new" || one.PlaceDomain != "containers" || one.PlaceSuffix != "-copies" {
		t.Fatalf("row-1 = %+v, want it on p-new with the -copies suffix", one)
	}
	two, _, err := r.GetOffsiteTarget("row-2")
	if err != nil {
		t.Fatal(err)
	}
	if two.PlaceID != "" || two.PlaceDomain != "" || two.PlaceSuffix != "" {
		t.Fatalf("row-2 = %+v, want its link to the dropped place gone", two)
	}
	s, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.PlacesMigrated == 0 {
		t.Fatal("places_migrated is clear after a replace; the startup migration would build a second set")
	}
}

func TestARowOffWithItsImportedPlaceComesBackWhenThePlaceIsSwitchedOn(t *testing.T) {
	r, db := placesRepo(t)
	bucket := mustWritePlace(t, r, store.PlaceWrite{Place: bucketPlace()})
	row := upsertRow(t, r, store.OffsiteTarget{Domain: "containers", Name: "B2", Repo: addressAt(t, bucket, "containers", ""), Enabled: true})
	attachRow(t, db, row.ID, bucket.ID, "containers", "")
	bucket.Enabled = false
	bucket = mustWritePlace(t, r, store.PlaceWrite{Place: bucket})

	if err := r.ReplacePlaces(store.PlacesImport{
		Places: []store.Place{bucket},
		Links:  []store.PlaceLink{{RowID: row.ID, PlaceID: bucket.ID, Domain: "containers"}},
	}); err != nil {
		t.Fatal(err)
	}
	bucket = mustWritePlace(t, r, store.PlaceWrite{Place: bucket})
	bucket.Enabled = true
	bucket = mustWritePlace(t, r, store.PlaceWrite{Place: bucket})

	if got := switchesAt(t, r, bucket.ID); !got[row.ID] {
		t.Fatalf("after the import and the place switched on the rows are %v, want the row back on", got)
	}
}

func TestDropPlacesClearsEveryPlaceAndTheMigrationMark(t *testing.T) {
	r := newRepo(t)
	importRows(t, r)
	if err := r.ReplacePlaces(store.PlacesImport{
		Places:      []store.Place{onePlace("p-1", map[string]string{"containers": "row-1"})},
		HomeDomains: map[string]string{"containers": "p-1"},
		Links:       []store.PlaceLink{{RowID: "row-1", PlaceID: "p-1", Domain: "containers"}},
	}); err != nil {
		t.Fatal(err)
	}

	if err := r.DropPlaces(); err != nil {
		t.Fatal(err)
	}

	if got, err := r.ListPlaces(); err != nil || len(got) != 0 {
		t.Fatalf("places = %+v (err %v), want none", got, err)
	}
	if homes, err := r.DomainPlaces(); err != nil || len(homes) != 0 {
		t.Fatalf("home places = %v (err %v), want none", homes, err)
	}
	if row, _, err := r.GetOffsiteTarget("row-1"); err != nil || row.PlaceID != "" {
		t.Fatalf("row-1 = %+v (err %v), want it on no place", row, err)
	}
	if s, err := r.GetSettings(); err != nil || s.PlacesMigrated != 0 {
		t.Fatalf("places_migrated = %d (err %v), want it clear so the places migration can build them again", s.PlacesMigrated, err)
	}
}
