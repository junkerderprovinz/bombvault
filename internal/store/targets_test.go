package store_test

import (
	"encoding/json"
	"reflect"
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

	// A backup-time UpsertTarget keeps the user's selection.
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

func TestSetExcludesRoundTripAndUpsertPreserves(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	// SetExcludes creates the target row when none exists yet.
	if err := r.SetExcludes("plex", []string{"/config/x/Cache", ".git"}); err != nil {
		t.Fatalf("SetExcludes: %v", err)
	}
	got, err := r.GetTargetByContainer("plex")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Excludes) != 2 || got.Excludes[0] != "/config/x/Cache" || got.Excludes[1] != ".git" {
		t.Fatalf("excludes not stored: %v", got.Excludes)
	}

	// A backup-time UpsertTarget keeps the user's excludes.
	if _, err := r.UpsertTarget(store.Target{ContainerName: "plex", AppdataPaths: []string{"/host/user/appdata/plex"}, Definition: "{}"}); err != nil {
		t.Fatal(err)
	}
	got, _ = r.GetTargetByContainer("plex")
	if len(got.Excludes) != 2 || got.Excludes[0] != "/config/x/Cache" || got.Excludes[1] != ".git" {
		t.Fatalf("Upsert clobbered excludes: %v", got.Excludes)
	}

	// An empty set clears the excludes.
	if err := r.SetExcludes("plex", nil); err != nil {
		t.Fatal(err)
	}
	got, _ = r.GetTargetByContainer("plex")
	if len(got.Excludes) != 0 {
		t.Fatalf("excludes should be cleared, got %v", got.Excludes)
	}
}

// ExcludeCaches, the per-root CACHEDIR.TAG toggle stored as a JSON map, has the
// same lifecycle as the excludes.
func TestSetExcludeCachesRoundTripAndUpsertPreserves(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	// SetExcludeCaches creates the target row when none exists yet.
	m := map[string]bool{"/host/user/user/appdata/plex": true, "/host/user/user/appdata/plex/custom": false}
	if err := r.SetExcludeCaches("plex", m); err != nil {
		t.Fatalf("SetExcludeCaches: %v", err)
	}
	got, err := r.GetTargetByContainer("plex")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.ExcludeCaches, m) {
		t.Fatalf("exclude caches not stored: %v", got.ExcludeCaches)
	}

	// A backup-time UpsertTarget keeps the per-root toggles.
	if _, err := r.UpsertTarget(store.Target{ContainerName: "plex", AppdataPaths: []string{"/host/user/appdata/plex"}, Definition: "{}"}); err != nil {
		t.Fatal(err)
	}
	got, _ = r.GetTargetByContainer("plex")
	if !reflect.DeepEqual(got.ExcludeCaches, m) {
		t.Fatalf("Upsert clobbered exclude caches: %v", got.ExcludeCaches)
	}

	// A row inserted without the column gets the schema default '{}' and scans
	// as an empty map.
	if _, err := db.Exec(`INSERT INTO targets (id, container_name, appdata_paths, created_at) VALUES ('legacy', 'legacyrow', '[]', 1)`); err != nil {
		t.Fatal(err)
	}
	legacy, err := r.GetTargetByContainer("legacyrow")
	if err != nil {
		t.Fatal(err)
	}
	if len(legacy.ExcludeCaches) != 0 {
		t.Fatalf("never-set column should scan empty, got %v", legacy.ExcludeCaches)
	}

	// encoding/json sorts map keys, so the same map stores the same bytes
	// however it was built.
	again := map[string]bool{"/host/user/user/appdata/plex/custom": false, "/host/user/user/appdata/plex": true}
	if err := r.SetExcludeCaches("plex", again); err != nil {
		t.Fatalf("SetExcludeCaches again: %v", err)
	}
	wantJSON, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var raw1, raw2 string
	if err := db.QueryRow(`SELECT exclude_caches FROM targets WHERE container_name = 'plex'`).Scan(&raw1); err != nil {
		t.Fatal(err)
	}
	if raw1 != string(wantJSON) {
		t.Fatalf("stored JSON %q, want %q", raw1, wantJSON)
	}
	if err := r.SetExcludeCaches("plex", again); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT exclude_caches FROM targets WHERE container_name = 'plex'`).Scan(&raw2); err != nil {
		t.Fatal(err)
	}
	if raw1 != raw2 {
		t.Fatalf("re-setting the same map changed the stored JSON: %q vs %q", raw1, raw2)
	}

	// An empty (or nil) map clears every root toggle.
	if err := r.SetExcludeCaches("plex", nil); err != nil {
		t.Fatalf("SetExcludeCaches nil: %v", err)
	}
	got, _ = r.GetTargetByContainer("plex")
	if len(got.ExcludeCaches) != 0 {
		t.Fatalf("exclude caches should be cleared, got %v", got.ExcludeCaches)
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

// Unlike the user's settings, the definition is replaced by every upsert.
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

	got, err := r.GetTargetByContainer("myapp")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Definition != def1 {
		t.Fatalf("definition mismatch: got %q want %q", got.Definition, def1)
	}

	list, err := r.ListTargets()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Definition != def1 {
		t.Fatalf("list definition mismatch: %+v", list)
	}

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

// A backup-time UpsertTarget keeps the update-check stamp, and stamping an
// unknown container is an error rather than a silent no-op.
func TestSetUpdateCheckRoundTripAndUpsertPreserves(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	if _, err := r.UpsertTarget(store.Target{ContainerName: "plex", AppdataPaths: []string{"/host/user/appdata/plex"}}); err != nil {
		t.Fatal(err)
	}

	if err := r.SetUpdateCheck("plex", 1700000000, "up-to-date"); err != nil {
		t.Fatalf("SetUpdateCheck: %v", err)
	}
	got, err := r.GetTargetByContainer("plex")
	if err != nil {
		t.Fatal(err)
	}
	if got.LastUpdateCheck != 1700000000 || got.LastUpdateResult != "up-to-date" {
		t.Fatalf("update check not stored: at=%d result=%q", got.LastUpdateCheck, got.LastUpdateResult)
	}

	if _, err := r.UpsertTarget(store.Target{ContainerName: "plex", AppdataPaths: []string{"/host/user/appdata/plex"}, Definition: "{}"}); err != nil {
		t.Fatal(err)
	}
	got, _ = r.GetTargetByContainer("plex")
	if got.LastUpdateCheck != 1700000000 || got.LastUpdateResult != "up-to-date" {
		t.Fatalf("Upsert clobbered the update check: at=%d result=%q", got.LastUpdateCheck, got.LastUpdateResult)
	}

	if err := r.SetUpdateCheck("ghost", 1700000001, "failed"); err == nil {
		t.Fatal("SetUpdateCheck on an unknown container must error")
	}
}

func TestDBDumpTargetFieldsSurviveUpsert(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	if _, err := r.UpsertTarget(store.Target{ContainerName: "pg", AppdataPaths: []string{"/host/user/appdata/pg"}}); err != nil {
		t.Fatal(err)
	}

	if err := r.SetDBDumpOff("pg", true); err != nil {
		t.Fatalf("SetDBDumpOff: %v", err)
	}
	if err := r.SetDBDumpEngine("pg", "mariadb"); err != nil {
		t.Fatalf("SetDBDumpEngine: %v", err)
	}

	// A backup runs UpsertTarget on every pass; it must not reset the choice.
	if _, err := r.UpsertTarget(store.Target{ContainerName: "pg", AppdataPaths: []string{"/host/user/appdata/pg"}, Definition: "{}"}); err != nil {
		t.Fatal(err)
	}

	got, err := r.GetTargetByContainer("pg")
	if err != nil {
		t.Fatal(err)
	}
	if !got.DBDumpOff || got.DBDumpEngine != "mariadb" {
		t.Fatalf("GetTargetByContainer: off=%v engine=%q, want true and mariadb", got.DBDumpOff, got.DBDumpEngine)
	}

	list, err := r.ListTargets()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || !list[0].DBDumpOff || list[0].DBDumpEngine != "mariadb" {
		t.Fatalf("ListTargets dropped the dump fields: %+v", list)
	}

	ordered, err := r.ListTargetsScheduleOrder()
	if err != nil {
		t.Fatal(err)
	}
	if len(ordered) != 1 || !ordered[0].DBDumpOff || ordered[0].DBDumpEngine != "mariadb" {
		t.Fatalf("ListTargetsScheduleOrder dropped the dump fields: %+v", ordered)
	}

	if err := r.SetDBDumpEngine("pg", "oracle"); err == nil {
		t.Fatal("SetDBDumpEngine must refuse an engine BombVault cannot dump")
	}
	got, _ = r.GetTargetByContainer("pg")
	if got.DBDumpEngine != "mariadb" {
		t.Fatalf("a refused engine changed the row: %q", got.DBDumpEngine)
	}
}

func TestSetDBDumpOffCreatesTargetRow(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	if err := r.SetDBDumpOff("pg", true); err != nil {
		t.Fatalf("SetDBDumpOff: %v", err)
	}
	got, err := r.GetTargetByContainer("pg")
	if err != nil {
		t.Fatalf("GetTargetByContainer: %v", err)
	}
	if !got.DBDumpOff {
		t.Fatal("the created row does not carry the opt-out")
	}

	if err := r.SetDBDumpEngine("maria", "mysql"); err != nil {
		t.Fatalf("SetDBDumpEngine: %v", err)
	}
	got, err = r.GetTargetByContainer("maria")
	if err != nil {
		t.Fatalf("GetTargetByContainer: %v", err)
	}
	if got.DBDumpEngine != "mysql" {
		t.Fatalf("the created row carries engine %q, want mysql", got.DBDumpEngine)
	}
}

func TestDeleteTargetRemovesAnomalyState(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	tg, err := r.UpsertTarget(store.Target{ContainerName: "deleteme"})
	if err != nil {
		t.Fatal(err)
	}
	keep, err := r.UpsertTarget(store.Target{ContainerName: "keepme"})
	if err != nil {
		t.Fatal(err)
	}
	seedAnomalyState(t, r, tg.ID, "container")
	seedAnomalyState(t, r, keep.ID, "container")
	seedDomainAnomaly(t, r)

	if err := r.DeleteTarget("deleteme"); err != nil {
		t.Fatalf("DeleteTarget: %v", err)
	}
	assertAnomalyStateGone(t, r, tg.ID)

	rows, _, err := r.ListAnomalies(store.AnomalyFilter{TargetID: keep.ID})
	if err != nil {
		t.Fatalf("ListAnomalies: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("the other container kept %d of its findings", len(rows))
	}
	domainRows, _, err := r.ListAnomalies(store.AnomalyFilter{ScopeKind: "domain"})
	if err != nil {
		t.Fatalf("ListAnomalies domain: %v", err)
	}
	if len(domainRows) != 1 {
		t.Fatal("the domain finding went with the container")
	}
}
