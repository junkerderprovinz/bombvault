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

// TestFileSetSelectedPathsRoundTrip follows a selection through its life: a new
// set scans with nil SelectedPaths, a written selection reads back through all
// three queries, UpdateFileSet leaves it alone, and nil stores SQL NULL again
// rather than '[]', which would read back as a non-nil empty slice.
func TestFileSetSelectedPathsRoundTrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

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

	if err := r.UpdateFileSet(store.FileSet{ID: "ghost", Name: "x", Path: "y"}); err == nil {
		t.Fatal("update of unknown id must fail")
	}
}

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
	if got.SelectedPaths != nil {
		t.Fatalf("selection must clear to SQL NULL, got %v", got.SelectedPaths)
	}

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
	runs, _ := r.ListRuns(100)
	for _, run := range runs {
		if run.TargetID == fs.ID {
			t.Fatalf("run for deleted file set must be removed: %+v", run)
		}
	}
}

func TestDeleteFileSetRemovesItsCopyRule(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	fs, err := r.CreateFileSet(store.FileSet{Name: "keepme", Path: "user/keepme", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.SetCopyRule("files", "fileset:keepme", []string{"target-1"}); err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteFileSet(fs.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	state, err := r.ReadPlacement("files")
	if err != nil {
		t.Fatal(err)
	}
	if state.HasRules() {
		t.Fatalf("a deleted file set's copy rule must not linger: %+v", state.Rules)
	}

	if _, err := r.CreateFileSet(store.FileSet{Name: "other", Path: "user/other", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := r.SetCopyRule("files", "fileset:other", []string{"target-1"}); err != nil {
		t.Fatal(err)
	}
	if err := r.MoveCopyRule("files", "fileset:other", "fileset:keepme"); err != nil {
		t.Fatalf("a rename onto the freed name must not be refused: %v", err)
	}
}

func TestDeleteFileSetNotFoundIsNoop(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	if err := r.DeleteFileSet("ghost"); err != nil {
		t.Fatalf("delete non-existent: %v", err)
	}
}
