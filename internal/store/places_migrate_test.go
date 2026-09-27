package store_test

import (
	"errors"
	"maps"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

var everyDomainAtTheBase = map[string]string{"containers": "", "vms": "", "flash": "", "config": "", "files": ""}

// placesMigrationScene is a fresh database with the global local rule
// keep-last 5, one B2 target on keep-last 3 and one local named repository
// whose own keep columns are all zero.
func placesMigrationScene(t *testing.T) (*store.Repo, store.OffsiteTarget, store.OffsiteTarget) {
	t.Helper()
	r, _ := migratedStore(t)
	if _, err := r.MutateSettings(func(s *store.Settings) error {
		s.RetentionKeepLast = 5
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	b2, err := r.CreateOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "B2", Repo: "s3:https://s3.example.com/bv/containers", Enabled: true, RetentionKeepLast: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	nas, err := r.UpsertOffsiteTarget(store.OffsiteTarget{Role: store.RoleRepo, Name: "NAS", Repo: "remotes/nas/bv", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	return r, b2, nas
}

// migrationFor is the plan for placesMigrationScene: the default domain paths
// on one local place, B2 on a place of its own and the named repository as a
// place that is one repository for every domain.
func migrationFor(b2, nas store.OffsiteTarget) []store.MigratedPlace {
	return []store.MigratedPlace{
		{
			Place: store.Place{
				Name: "Unraid", Provider: "unraid-folder", Kind: "local", Base: "user/bombvault",
				Folders:           map[string]string{"containers": "container", "vms": "vms", "flash": "flash", "config": "config", "files": "files"},
				RetentionKeepLast: 5, Enabled: true,
			},
			HomeDomains: []string{"containers", "vms", "flash", "config", "files"},
		},
		{
			Place: store.Place{
				Name: "B2", Provider: "s3-other", Kind: "s3", Base: "s3:https://s3.example.com/bv",
				Folders: map[string]string{"containers": "containers"}, OffPremises: true, RetentionKeepLast: 3, Enabled: true,
			},
			Rows: []store.PlaceRowRef{{RowID: b2.ID, Domain: "containers", Repo: b2.Repo}},
		},
		{
			Place: store.Place{
				Name: "NAS", Provider: "share", Kind: "local", Base: nas.Repo,
				Folders: everyDomainAtTheBase, RetentionKeepLast: 5, Enabled: true,
			},
			Rows: []store.PlaceRowRef{{RowID: nas.ID, Repo: nas.Repo}},
		},
	}
}

// assertNothingMigrated checks that a refused write left no trace.
func assertNothingMigrated(t *testing.T, r *store.Repo, rowID string) {
	t.Helper()
	if all, err := r.ListPlaces(); err != nil || len(all) != 0 {
		t.Errorf("places = %+v, %v, want none", all, err)
	}
	if homes, err := r.DomainPlaces(); err != nil || len(homes) != 0 {
		t.Errorf("home places = %v, %v, want none", homes, err)
	}
	if row, ok, err := r.GetOffsiteTarget(rowID); err != nil || !ok || row.PlaceID != "" {
		t.Errorf("row = %+v, %v, %v, want it without a place", row, ok, err)
	}
	if s, err := r.GetSettings(); err != nil || s.PlacesMigrated != 0 {
		t.Errorf("places_migrated = %d, %v, want 0", s.PlacesMigrated, err)
	}
}

func TestApplyPlacesMigrationWritesPlacesRowsAndHomesInOneGo(t *testing.T) {
	r, b2, nas := placesMigrationScene(t)
	if err := r.ApplyPlacesMigration(migrationFor(b2, nas)); err != nil {
		t.Fatalf("ApplyPlacesMigration: %v", err)
	}
	all, err := r.ListPlaces()
	if err != nil || len(all) != 3 {
		t.Fatalf("places = %+v, %v, want three", all, err)
	}
	ids := map[string]string{}
	var names []string
	for _, p := range all {
		if p.ID == "" || p.CreatedAt == 0 {
			t.Errorf("place %q was stored without an id or a time: %+v", p.Name, p)
		}
		ids[p.Name] = p.ID
		names = append(names, p.Name)
	}
	if !slices.Equal(names, []string{"Unraid", "B2", "NAS"}) {
		t.Errorf("places in order %q, want the plan's order", names)
	}
	homes, err := r.DomainPlaces()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"containers", "vms", "flash", "config", "files"} {
		if homes[d] != ids["Unraid"] {
			t.Errorf("home of %s = %q, want Unraid %q", d, homes[d], ids["Unraid"])
		}
	}
	onB2, err := r.PlaceRows(ids["B2"])
	if err != nil || len(onB2) != 1 || onB2[0].ID != b2.ID || onB2[0].PlaceDomain != "containers" || onB2[0].PlaceSuffix != "" || onB2[0].RetentionKeepLast != 3 {
		t.Fatalf("rows on B2 = %+v, %v", onB2, err)
	}
	onNAS, err := r.PlaceRows(ids["NAS"])
	if err != nil || len(onNAS) != 1 || onNAS[0].ID != nas.ID || onNAS[0].PlaceDomain != "" || onNAS[0].Repo != nas.Repo {
		t.Fatalf("rows on NAS = %+v, %v", onNAS, err)
	}
	if onNAS[0].RetentionKeepLast != 5 {
		t.Errorf("the named repository keeps %d, want the place's keep-last 5 copied onto it", onNAS[0].RetentionKeepLast)
	}
	if p, err := r.GetPlace(ids["NAS"]); err != nil || !maps.Equal(p.Folders, everyDomainAtTheBase) {
		t.Errorf("NAS = %+v, %v", p, err)
	}
	if s, err := r.GetSettings(); err != nil || s.PlacesMigrated == 0 {
		t.Errorf("places_migrated = %d, %v, want it set", s.PlacesMigrated, err)
	}
}

func TestApplyPlacesMigrationWritesNothingWhenARowMovedSinceThePlan(t *testing.T) {
	r, b2, nas := placesMigrationScene(t)
	plan := migrationFor(b2, nas)
	nas.Repo = "remotes/nas/elsewhere"
	if _, err := r.UpsertOffsiteTarget(nas); err != nil {
		t.Fatal(err)
	}
	if err := r.ApplyPlacesMigration(plan); err == nil {
		t.Fatal("a row that moved after the plan was put on a place")
	}
	assertNothingMigrated(t, r, b2.ID)
}

func TestApplyPlacesMigrationWritesNothingWhenADomainPathMovedSinceThePlan(t *testing.T) {
	r, b2, nas := placesMigrationScene(t)
	plan := migrationFor(b2, nas)
	if _, err := r.MutateSettings(func(s *store.Settings) error {
		s.FilesPath = "user/elsewhere/files"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := r.ApplyPlacesMigration(plan); err == nil {
		t.Fatal("a domain path that moved after the plan got a home place")
	}
	assertNothingMigrated(t, r, b2.ID)
}

// A migrated place is off only when every row on it was, and nothing says which
// switch turned them off, so they go with the place as migration 127 has it.
func TestARowOffAtAMigratedPlaceThatIsOffComesOnWithIt(t *testing.T) {
	r, b2, nas := placesMigrationScene(t)
	b2.Enabled = false
	if _, err := r.UpsertOffsiteTarget(b2); err != nil {
		t.Fatal(err)
	}
	plan := migrationFor(b2, nas)
	plan[1].Place.Enabled = false
	if err := r.ApplyPlacesMigration(plan); err != nil {
		t.Fatal(err)
	}
	row, _, err := r.GetOffsiteTarget(b2.ID)
	if err != nil || !row.OffWithPlace {
		t.Fatalf("B2 = %+v, %v, want it marked off with its place", row, err)
	}

	p, err := r.GetPlace(row.PlaceID)
	if err != nil {
		t.Fatal(err)
	}
	p.Enabled = true
	if _, err := r.WritePlace(store.PlaceWrite{Place: p}); err != nil {
		t.Fatal(err)
	}
	if row, _, err := r.GetOffsiteTarget(b2.ID); err != nil || !row.Enabled || row.OffWithPlace {
		t.Fatalf("B2 after its place came on = %+v, %v, want it on", row, err)
	}
}

func TestApplyPlacesMigrationRunsOnce(t *testing.T) {
	r, b2, nas := placesMigrationScene(t)
	if err := r.ApplyPlacesMigration(migrationFor(b2, nas)); err != nil {
		t.Fatal(err)
	}
	err := r.ApplyPlacesMigration([]store.MigratedPlace{{Place: store.Place{Name: "Another", Kind: "local", Base: "x", Folders: map[string]string{}}}})
	if !errors.Is(err, store.ErrPlacesMigrated) {
		t.Fatalf("second ApplyPlacesMigration = %v, want ErrPlacesMigrated", err)
	}
	if all, err := r.ListPlaces(); err != nil || len(all) != 3 {
		t.Fatalf("places = %d, %v, want the three of the first run", len(all), err)
	}
}
