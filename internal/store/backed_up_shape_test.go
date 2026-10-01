package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestBackedUpShapeSurvivesTheNextUpsert(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	tg, err := r.UpsertTarget(store.Target{ContainerName: "plex", Definition: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.SetBackedUpShape(tg.ID, `{"ID":"abc"}`); err != nil {
		t.Fatal(err)
	}
	// The next backup rewrites the definition before restic runs; the shape
	// of the last good backup must stay until that backup succeeds.
	if _, err := r.UpsertTarget(store.Target{ContainerName: "plex", Definition: `{"new":1}`}); err != nil {
		t.Fatal(err)
	}
	shapes, err := r.BackedUpShapes()
	if err != nil {
		t.Fatal(err)
	}
	if got := shapes[tg.ID]; got != `{"ID":"abc"}` {
		t.Fatalf("shape = %q", got)
	}
}

func TestBackedUpShapesLeavesOutTargetsWithoutOne(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	if _, err := r.UpsertTarget(store.Target{ContainerName: "plex"}); err != nil {
		t.Fatal(err)
	}
	shapes, err := r.BackedUpShapes()
	if err != nil {
		t.Fatal(err)
	}
	if len(shapes) != 0 {
		t.Fatalf("shapes = %v", shapes)
	}
}

func TestSetBackedUpShapeOfAMissingTargetFails(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := store.New(db).SetBackedUpShape("nope", "{}"); err == nil {
		t.Fatal("want an error for a target that does not exist")
	}
}
