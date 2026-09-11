package store_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestCreateFileSetRoundtrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	fs, err := r.CreateFileSet(store.FileSet{
		Name:    "docs",
		Path:    "user/documents",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if fs.ID == "" {
		t.Fatal("ID must be assigned")
	}
	if fs.CreatedAt == 0 {
		t.Fatal("created_at must be set")
	}

	// Re-read by id.
	got, err := r.GetFileSet(fs.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "docs" || got.Path != "user/documents" || !got.Enabled {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if len(got.Excludes) != 0 {
		t.Fatalf("excludes must default to empty, got %v", got.Excludes)
	}

	// Re-read by name.
	byName, err := r.GetFileSetByName("docs")
	if err != nil {
		t.Fatalf("get by name: %v", err)
	}
	if byName.ID != fs.ID {
		t.Fatalf("id mismatch: %q vs %q", byName.ID, fs.ID)
	}
}

func TestCreateFileSetDuplicateNameFails(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	if _, err := r.CreateFileSet(store.FileSet{Name: "photos", Path: "user/photos"}); err != nil {
		t.Fatal(err)
	}
	// name is UNIQUE — a second set with the same name must fail.
	if _, err := r.CreateFileSet(store.FileSet{Name: "photos", Path: "user/other"}); err == nil {
		t.Fatal("duplicate name must fail (name is UNIQUE)")
	}
}

func TestFileSetExcludesRoundtrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	excludes := []string{"*.tmp", "node_modules", ".cache/**"}
	fs, err := r.CreateFileSet(store.FileSet{Name: "code", Path: "user/code", Excludes: excludes, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.GetFileSet(fs.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Excludes, excludes) {
		t.Fatalf("excludes not round-tripped: %v vs %v", got.Excludes, excludes)
	}
}

// TestFileSetSelectedPathsRoundTrip pins the selected_paths column contract
// (Phase 4, D-03): a set created through CreateFileSet scans with nil
// SelectedPaths (the INSERT omits the column, so it stores SQL NULL — NULL is
// the "never touched by the tree" legacy switch); SetFileSetSelectedPaths
// persists a written selection and re-reads it equal through ALL THREE select
// paths; nil stores SQL NULL again (never the JSON literal '[]' — a stored
// '[]' would scan to a non-nil empty slice and silently flip the legacy
// switch); and UpdateFileSet's name/path/excludes/enabled save never clears a
// stored selection.
func TestFileSetSelectedPathsRoundTrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	// Created via CreateFileSet (which omits the column) → NULL → nil.
	fs, err := r.CreateFileSet(store.FileSet{Name: "sel", Path: "user/sel", Enabled: true})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := r.GetFileSet(fs.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.SelectedPaths != nil {
		t.Fatalf("a fresh set must scan with nil SelectedPaths (NULL column), got %v", got.SelectedPaths)
	}

	// A written selection round-trips — by id, by name, and through the list
	// (all three SELECT lists must carry the column together).
	sel := []string{"/host/user/sel", "!/host/user/sel/cache"}
	if err := r.SetFileSetSelectedPaths(fs.ID, sel); err != nil {
		t.Fatalf("set selection: %v", err)
	}
	got, err = r.GetFileSet(fs.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.SelectedPaths, sel) {
		t.Fatalf("selection not round-tripped by id: %v vs %v", got.SelectedPaths, sel)
	}
	byName, err := r.GetFileSetByName("sel")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(byName.SelectedPaths, sel) {
		t.Fatalf("selection not round-tripped by name: %v vs %v", byName.SelectedPaths, sel)
	}
	list, err := r.ListFileSets()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || !reflect.DeepEqual(list[0].SelectedPaths, sel) {
		t.Fatalf("selection not round-tripped through the list: %+v", list)
	}

	// Three consecutive UpdateFileSet saves (name/path/excludes/enabled only)
	// leave the stored selection byte-identical — the owned-setter split means
	// a form that does not know about the selection can never clear one by
	// omitting it (the #199 cadence rationale applied to the tree selection).
	for i := range 3 {
		got.Name = fmt.Sprintf("sel-renamed-%d", i)
		got.Path = fmt.Sprintf("user/sel-%d", i)
		got.Excludes = []string{fmt.Sprintf("*.tmp%d", i)}
		got.Enabled = i%2 == 0
		if err := r.UpdateFileSet(got); err != nil {
			t.Fatalf("update %d: %v", i, err)
		}
		after, err := r.GetFileSet(fs.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(after.SelectedPaths, sel) {
			t.Fatalf("UpdateFileSet %d clobbered the selection: %v vs %v", i, after.SelectedPaths, sel)
		}
	}

	// nil stores SQL NULL and re-reads nil — never the JSON string '[]' (which
	// would scan to a non-nil empty slice and mean something different: the
	// NULL/nil distinction IS the legacy argv switch at the compile site).
	if err := r.SetFileSetSelectedPaths(fs.ID, nil); err != nil {
		t.Fatalf("clear selection: %v", err)
	}
	got, err = r.GetFileSet(fs.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SelectedPaths != nil {
		t.Fatalf("nil selection must store SQL NULL (re-read nil), got %v", got.SelectedPaths)
	}

	// Unknown id must error, mirroring every other owned setter.
	if err := r.SetFileSetSelectedPaths("ghost", sel); err == nil {
		t.Fatal("SetFileSetSelectedPaths on unknown id must fail")
	}
}

func TestUpdateFileSet(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	fs, err := r.CreateFileSet(store.FileSet{Name: "media", Path: "user/media", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	fs.Name = "media-renamed"
	fs.Path = "user/media2"
	fs.Excludes = []string{"*.iso"}
	fs.Enabled = false
	if err := r.UpdateFileSet(fs); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := r.GetFileSet(fs.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "media-renamed" || got.Path != "user/media2" || got.Enabled {
		t.Fatalf("update not persisted: %+v", got)
	}
	if !reflect.DeepEqual(got.Excludes, []string{"*.iso"}) {
		t.Fatalf("excludes not updated: %v", got.Excludes)
	}
	if got.CreatedAt != fs.CreatedAt {
		t.Fatalf("created_at must be immutable: %d vs %d", got.CreatedAt, fs.CreatedAt)
	}

	// Updating an unknown id must error.
	if err := r.UpdateFileSet(store.FileSet{ID: "ghost", Name: "x", Path: "y"}); err == nil {
		t.Fatal("update of unknown id must fail")
	}
}

// TestUpdateFileSetClearingSelection pins the path-change writer (review
// WR-01): the row fields update AND selected_paths is stored as SQL NULL in
// the ONE UPDATE statement, so a path change can never coexist with an
// old-anchor selection — and a failure of the statement leaves the row
// untouched, keeping the handler's fail envelope consistent with storage.
func TestUpdateFileSetClearingSelection(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	fs, err := r.CreateFileSet(store.FileSet{Name: "docs", Path: "user/docs", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	sel := []string{"/host/user/docs/keep", "!/host/user/docs/keep/tmp"}
	if err := r.SetFileSetSelectedPaths(fs.ID, sel); err != nil {
		t.Fatalf("seed selection: %v", err)
	}

	fs.Name = "docs-moved"
	fs.Path = "user/docs/2026"
	fs.Excludes = []string{"*.iso"}
	fs.Enabled = false
	if err := r.UpdateFileSetClearingSelection(fs); err != nil {
		t.Fatalf("update+clear: %v", err)
	}

	got, err := r.GetFileSet(fs.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "docs-moved" || got.Path != "user/docs/2026" || got.Enabled {
		t.Fatalf("row fields not persisted: %+v", got)
	}
	// The clear is SQL NULL (re-reads nil), never the JSON literal '[]' — the
	// NULL/nil state IS the legacy argv switch at the compile site.
	if got.SelectedPaths != nil {
		t.Fatalf("selection must clear to SQL NULL, got %v", got.SelectedPaths)
	}

	// Unknown id must error, mirroring every other owned setter.
	if err := r.UpdateFileSetClearingSelection(store.FileSet{ID: "ghost", Name: "x", Path: "y"}); err == nil {
		t.Fatal("update+clear of unknown id must fail")
	}
}

func TestListFileSets(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	for _, name := range []string{"setB", "setA", "setC"} {
		if _, err := r.CreateFileSet(store.FileSet{Name: name, Path: "user/" + name}); err != nil {
			t.Fatal(err)
		}
	}
	list, err := r.ListFileSets()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3, got %d", len(list))
	}
	// ORDER BY name
	if list[0].Name != "setA" || list[1].Name != "setB" || list[2].Name != "setC" {
		t.Fatalf("order wrong: %v", list)
	}
}

func TestSetFileSetEnabled(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	fs, err := r.CreateFileSet(store.FileSet{Name: "toggleme", Path: "user/toggleme", Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.SetFileSetEnabled(fs.ID, true); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetFileSet(fs.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Enabled {
		t.Fatal("enabled must be true after SetFileSetEnabled(true)")
	}

	// Unknown id must error.
	if err := r.SetFileSetEnabled("ghost", true); err == nil {
		t.Fatal("SetFileSetEnabled on unknown id must fail")
	}
}

func TestDeleteFileSetRemovesRuns(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	fs, err := r.CreateFileSet(store.FileSet{Name: "deleteme", Path: "user/deleteme", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	// Seed a run referencing this file set.
	runID, err := r.StartRun(fs.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.FinishRun(runID, "success", "abc123", 1024, ""); err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteFileSet(fs.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := r.GetFileSet(fs.ID); err == nil {
		t.Fatal("file set must be gone after delete")
	}
	// Runs must also be gone (cascade in tx).
	runs, _ := r.ListRuns(100)
	for _, run := range runs {
		if run.TargetID == fs.ID {
			t.Fatalf("run for deleted file set must be removed: %+v", run)
		}
	}
}

func TestDeleteFileSetNotFoundIsNoop(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	// Deleting a non-existent file set must not error.
	if err := r.DeleteFileSet("ghost"); err != nil {
		t.Fatalf("delete non-existent: %v", err)
	}
}
