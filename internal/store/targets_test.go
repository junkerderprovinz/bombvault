package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestTargetRoundtrip(t *testing.T) {
	db := store.OpenMem(t)
	store.Migrate(db) //nolint:errcheck,gosec // test helper; errors caught by subsequent test assertions
	r := store.New(db)
	tg, _ := r.UpsertTarget(store.Target{ContainerName: "plex", AppdataPaths: []string{"/host/user/appdata/plex"}})
	got, _ := r.GetTargetByContainer("plex")
	if got.ID != tg.ID || got.AppdataPaths[0] != "/host/user/appdata/plex" {
		t.Fatal("roundtrip")
	}
}

func TestTargetIncludeToggle(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	if _, err := r.UpsertTarget(store.Target{ContainerName: "jellyfin", AppdataPaths: []string{"/data"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.SetInclude("jellyfin", true); err != nil {
		t.Fatalf("SetInclude: %v", err)
	}
	got, err := r.GetTargetByContainer("jellyfin")
	if err != nil {
		t.Fatal(err)
	}
	if !got.IncludeInSchedule {
		t.Fatal("IncludeInSchedule should be true")
	}
}
