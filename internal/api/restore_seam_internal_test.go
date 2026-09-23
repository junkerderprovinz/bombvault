package api

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// prepareRestore resolves the repo from settings, prepareRestoreIn takes it
// explicitly; given the same repo and mode both return the same plan and error.
// A recreate-only target needs no engine, because the uninitialised local repo
// reports no snapshots.
func TestPrepareRestoreSeamEquivalence(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := store.New(db)

	def, err := json.Marshal(containerDefinition{Inspect: model.Inspect{Name: "web"}})
	if err != nil {
		t.Fatalf("marshal definition: %v", err)
	}
	if _, err := st.UpsertTarget(store.Target{ContainerName: "web", Definition: string(def)}); err != nil {
		t.Fatalf("upsert target: %v", err)
	}

	s := &Service{store: st, cfg: config.Config{HostMountRoot: t.TempDir()}}

	settings, err := st.GetSettings()
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	repo, err := s.repoFor(settings, "containers", "")
	if err != nil {
		t.Fatalf("repoFor: %v", err)
	}
	ref := repoRef{repo: repo, mode: s.ModeFor(settings)}

	ctx := context.Background()
	viaSettings, errSettings := s.prepareRestore(ctx, "web", "latest", true, "")
	viaRef, errRef := s.prepareRestoreIn(ctx, ref, "web", "latest", true)

	if (errSettings == nil) != (errRef == nil) {
		t.Fatalf("error divergence: settings=%v ref=%v", errSettings, errRef)
	}
	if errSettings != nil && errSettings.Error() != errRef.Error() {
		t.Fatalf("error text divergence: settings=%q ref=%q", errSettings, errRef)
	}
	if !reflect.DeepEqual(viaSettings, viaRef) {
		t.Fatalf("plan divergence:\nsettings: %+v\nref:      %+v", viaSettings, viaRef)
	}
	if !viaSettings.recreateOnly {
		t.Fatalf("expected a recreate-only plan (no snapshots), got %+v", viaSettings)
	}
}
