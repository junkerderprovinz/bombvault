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

func TestSetBackupPathsRoundTripAndUpsertPreserves(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	// SetBackupPaths creates the target row when none exists yet.
	if err := r.SetBackupPaths("plex", []string{"/host/user/appdata/plex", "/host/user/media"}); err != nil {
		t.Fatalf("SetBackupPaths: %v", err)
	}
	got, err := r.GetTargetByContainer("plex")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.SelectedPaths) != 2 || got.SelectedPaths[1] != "/host/user/media" {
		t.Fatalf("selected paths not stored: %v", got.SelectedPaths)
	}

	// A subsequent backup-time UpsertTarget (which sets AppdataPaths/Definition)
	// must NOT clobber the user's selection.
	if _, err := r.UpsertTarget(store.Target{ContainerName: "plex", AppdataPaths: []string{"/host/user/appdata/plex"}, Definition: "{}"}); err != nil {
		t.Fatal(err)
	}
	got, _ = r.GetTargetByContainer("plex")
	if len(got.SelectedPaths) != 2 {
		t.Fatalf("Upsert clobbered selection: %v", got.SelectedPaths)
	}

	// An empty selection clears it (falls back to auto appdata at backup time).
	if err := r.SetBackupPaths("plex", nil); err != nil {
		t.Fatal(err)
	}
	got, _ = r.GetTargetByContainer("plex")
	if len(got.SelectedPaths) != 0 {
		t.Fatalf("selection should be cleared, got %v", got.SelectedPaths)
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

// TestTargetDefinitionRoundtrip verifies that the definition field is persisted
// and retrieved correctly via both GetTargetByContainer and ListTargets, and
// that a second upsert replaces the definition.
func TestTargetDefinitionRoundtrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	const def1 = `{"inspect":{"Image":"myapp:1.0"},"template_xml":"<xml/>"}}`
	if _, err := r.UpsertTarget(store.Target{
		ContainerName: "myapp",
		AppdataPaths:  []string{"/data"},
		Definition:    def1,
	}); err != nil {
		t.Fatalf("upsert with definition: %v", err)
	}

	// GetTargetByContainer must return the definition.
	got, err := r.GetTargetByContainer("myapp")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Definition != def1 {
		t.Fatalf("definition mismatch: got %q want %q", got.Definition, def1)
	}

	// ListTargets must also return the definition.
	list, err := r.ListTargets()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Definition != def1 {
		t.Fatalf("list definition mismatch: %+v", list)
	}

	// Second upsert must update the definition.
	const def2 = `{"inspect":{"Image":"myapp:2.0"},"template_xml":"<xml2/>"}}`
	if _, err := r.UpsertTarget(store.Target{
		ContainerName: "myapp",
		AppdataPaths:  []string{"/data"},
		Definition:    def2,
	}); err != nil {
		t.Fatalf("upsert update definition: %v", err)
	}
	got2, err := r.GetTargetByContainer("myapp")
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got2.Definition != def2 {
		t.Fatalf("updated definition mismatch: got %q want %q", got2.Definition, def2)
	}
}

// TestTargetDefinitionEmptyDefault verifies that a target upserted without a
// definition has an empty Definition field (migration v2 DEFAULT ” applies).
func TestTargetDefinitionEmptyDefault(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	if _, err := r.UpsertTarget(store.Target{
		ContainerName: "sonarr",
		AppdataPaths:  []string{"/sonarr"},
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := r.GetTargetByContainer("sonarr")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Definition != "" {
		t.Fatalf("expected empty definition for legacy target, got %q", got.Definition)
	}
}
