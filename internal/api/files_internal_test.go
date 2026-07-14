package api

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestValidateFileSet pins the save-time guard for the files domain: the name
// feeds restic tags + progress keys (strict container-name charset), the path
// must be a contained subpath under the host mount AND exist on disk — with
// the single deliberate exception that a PATH-LESS set is valid while it stays
// DISABLED (DiscoverFileSets rebuilds sets from fileset: tags alone, where the
// original path is unknowable; such a set must be storable but never enabled).
func TestValidateFileSet(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data", "docs"), 0o750); err != nil {
		t.Fatal(err)
	}
	s := &Service{cfg: config.Config{HostMountRoot: root}}

	cases := []struct {
		name    string
		fs      store.FileSet
		wantErr bool
	}{
		{"valid set", store.FileSet{Name: "docs", Path: "data/docs", Enabled: true}, false},
		{"traversal name", store.FileSet{Name: "../evil", Path: "data/docs"}, true},
		{"name with space", store.FileSet{Name: "my docs", Path: "data/docs"}, true},
		{"empty name", store.FileSet{Name: "", Path: "data/docs"}, true},
		{"traversal path", store.FileSet{Name: "docs", Path: "../etc"}, true},
		{"absolute path", store.FileSet{Name: "docs", Path: "/etc"}, true},
		{"non-existent path", store.FileSet{Name: "docs", Path: "data/nope"}, true},
		{"path-less disabled (discovered set)", store.FileSet{Name: "docs", Path: "", Enabled: false}, false},
		{"path-less enabled is refused", store.FileSet{Name: "docs", Path: "", Enabled: true}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := s.validateFileSet(c.fs)
			if c.wantErr && err == nil {
				t.Fatalf("validateFileSet(%+v) = nil, want error", c.fs)
			}
			if !c.wantErr && err != nil {
				t.Fatalf("validateFileSet(%+v) = %v, want nil", c.fs, err)
			}
		})
	}
}

// TestBeginRestoreRunForTarget pins the generalized restore bookkeeping the
// files domain records against file_sets.id directly (no container target row
// lookup): begin opens a kind "restore" run against the given target id and
// finishRestoreRun closes it with the terminal status + snapshot id.
func TestBeginRestoreRunForTarget(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := store.New(db)
	s := &Service{store: st}

	set, err := st.CreateFileSet(store.FileSet{Name: "docs", Path: "data/docs"})
	if err != nil {
		t.Fatalf("create file set: %v", err)
	}

	runID := s.beginRestoreRunForTarget(set.ID)
	if runID == "" {
		t.Fatal("beginRestoreRunForTarget must record a run against the set's id")
	}
	s.finishRestoreRun(runID, "deadbeef12345678", nil)

	runs, err := st.ListRuns(10)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected exactly one recorded run, got %d", len(runs))
	}
	run := runs[0]
	if run.TargetID != set.ID || run.Kind != "restore" || run.Status != "success" {
		t.Fatalf("run = %+v, want target %q kind restore status success", run, set.ID)
	}
	if run.SnapshotID != "deadbeef12345678" {
		t.Fatalf("run snapshot = %q, want the restored snapshot id", run.SnapshotID)
	}
}
