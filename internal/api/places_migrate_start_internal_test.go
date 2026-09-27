package api

import (
	"net/http"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func placesOf(t *testing.T, f *placementFixture) []store.Place {
	t.Helper()
	all, err := f.st.ListPlaces()
	if err != nil {
		t.Fatal(err)
	}
	return all
}

func TestThePlacesMigrationRunsOnceAndMarksTheDatabase(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "s3:https://s3.us-west-004.backblazeb2.com/bv/containers")
	if err := f.svc.MigrateToPlaces(); err != nil {
		t.Fatalf("MigrateToPlaces: %v", err)
	}
	if settingsOf(t, f.svc).PlacesMigrated == 0 {
		t.Fatal("the database is not marked as moved onto places")
	}
	first := placesOf(t, f)
	if row, _, err := f.st.GetOffsiteTarget(b2.ID); err != nil || row.PlaceID == "" {
		t.Fatalf("B2 = %+v, %v, want it on a place", row, err)
	}
	later := f.target("vms", "B2 VMs", "s3:https://s3.us-west-004.backblazeb2.com/bv/vms")
	if err := f.svc.MigrateToPlaces(); err != nil {
		t.Fatalf("second start: %v", err)
	}
	if got := placesOf(t, f); len(got) != len(first) {
		t.Fatalf("the second start left %d places, want the %d of the first", len(got), len(first))
	}
	if row, _, err := f.st.GetOffsiteTarget(later.ID); err != nil || row.PlaceID != "" {
		t.Fatalf("a target added after the move = %+v, %v, want it left alone", row, err)
	}
}

func TestASwitchedOffTargetComesOnWithTheSwitchedOffPlaceTheMoveGaveIt(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "s3:https://s3.us-west-004.backblazeb2.com/bv/containers")
	b2.Enabled = false
	if _, err := f.st.UpsertOffsiteTarget(b2); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.MigrateToPlaces(); err != nil {
		t.Fatal(err)
	}
	placed := f.storedTarget(b2.ID)

	if res := f.do(http.MethodPatch, "/api/places/"+placed.PlaceID, map[string]any{"enabled": true}); res["ok"] != true {
		t.Fatalf("PATCH = %v", res)
	}
	if row := f.storedTarget(b2.ID); !row.Enabled {
		t.Fatalf("B2 = %+v, want it on with its place", row)
	}
}

func TestAFailedPlacesMigrationLeavesTheOldStateAndRunsAgainAtTheNextStart(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "s3:https://s3.us-west-004.backblazeb2.com/bv/containers")
	nas := f.namedRepo("NAS", "remotes/nas/bombvault")
	// Stands in for a write that fails halfway: by the time the named repository
	// is attached, the home places and B2's place are inserted and B2 is on it.
	if _, err := f.db.Exec(`CREATE TRIGGER fail_nas_attach BEFORE UPDATE ON offsite_targets
		WHEN NEW.name = 'NAS' AND NEW.place_id <> ''
		BEGIN SELECT RAISE(ABORT, 'disk I/O error'); END`); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.MigrateToPlaces(); err == nil {
		t.Fatal("the migration reported success over a failed write")
	}
	if got := placesOf(t, f); len(got) != 0 {
		t.Fatalf("places after the failed run = %+v, want none", got)
	}
	if homes, err := f.st.DomainPlaces(); err != nil || len(homes) != 0 {
		t.Fatalf("home places after the failed run = %v, %v, want none", homes, err)
	}
	if row, _, err := f.st.GetOffsiteTarget(b2.ID); err != nil || row.PlaceID != "" {
		t.Fatalf("B2 after the failed run = %+v, %v, want it as it was", row, err)
	}
	if settingsOf(t, f.svc).PlacesMigrated != 0 {
		t.Fatal("a failed run marked the database as moved")
	}

	if _, err := f.db.Exec(`DROP TRIGGER fail_nas_attach`); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.MigrateToPlaces(); err != nil {
		t.Fatalf("the next start: %v", err)
	}
	if row, _, err := f.st.GetOffsiteTarget(b2.ID); err != nil || row.PlaceID == "" {
		t.Fatalf("B2 after the next start = %+v, %v, want it on a place", row, err)
	}
	if row, err := f.st.GetNamedRepo(nas.ID); err != nil || row.PlaceID == "" {
		t.Fatalf("NAS after the next start = %+v, %v, want it on a place", row, err)
	}
	if settingsOf(t, f.svc).PlacesMigrated == 0 {
		t.Fatal("the next start did not mark the database")
	}
}

func TestUnreadableCredentialsStopTheMoveBeforeAnythingIsWritten(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "s3:https://s3.us-west-004.backblazeb2.com/bv/containers")
	settings := settingsOf(t, f.svc)
	settings.CloudCredSets = "bm90IGEgc2VhbGVkIGJsb2I="
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.MigrateToPlaces(); err == nil {
		t.Fatal("the migration ran without being able to read the credential sets")
	}
	if got := placesOf(t, f); len(got) != 0 {
		t.Fatalf("places = %+v, want none", got)
	}
	if settingsOf(t, f.svc).PlacesMigrated != 0 {
		t.Fatal("the database was marked as moved")
	}
}

// A start moves a mesh target off sort order 0 before anything else reads the
// rows, so the move onto places does the same and a mesh place never owns the
// domain's off-site field.
func TestTheMoveOntoPlacesTakesAMeshTargetOffSortOrderZeroFirst(t *testing.T) {
	f := newPlacementFixture(t)
	mesh, err := f.st.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "mesh: tower", Repo: "rest:http://tower:8000/bv/containers"})
	if err != nil {
		t.Fatal(err)
	}
	f.acceptedOffer(mesh.Repo)

	if err := f.svc.MigrateToPlaces(); err != nil {
		t.Fatalf("MigrateToPlaces: %v", err)
	}
	if got := f.storedTarget(mesh.ID); got.PlaceID == "" || got.SortOrder == 0 {
		t.Fatalf("mesh target = %+v, want it on a place behind sort order 0", got)
	}
	if field, ok, err := f.st.FieldOffsiteTarget("containers"); err != nil || ok {
		t.Fatalf("containers field row = %+v, %v, want none", field, err)
	}
}
