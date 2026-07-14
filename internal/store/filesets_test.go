package store_test

import (
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
