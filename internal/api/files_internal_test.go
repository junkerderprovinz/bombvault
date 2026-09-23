package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/platform"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// A set without a path is valid only while disabled: DiscoverFileSets rebuilds
// sets from their restic tags, which do not carry the path.
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
			// Create always checks the path.
			// TestValidateFileSetChecksPathOnlyWhenAsked covers the flag.
			err := s.validateFileSet(c.fs, true)
			if c.wantErr && err == nil {
				t.Fatalf("validateFileSet(%+v) = nil, want error", c.fs)
			}
			if !c.wantErr && err != nil {
				t.Fatalf("validateFileSet(%+v) = %v, want nil", c.fs, err)
			}
		})
	}
}

// A well-formed, contained path that does not exist is refused only when
// checkPathExists is set.
func TestValidateFileSetChecksPathOnlyWhenAsked(t *testing.T) {
	root := t.TempDir()
	s := &Service{cfg: config.Config{HostMountRoot: root}}
	dead := store.FileSet{Name: "docs", Path: "data/gone", Enabled: true}

	if err := s.validateFileSet(dead, true); err == nil {
		t.Fatal("checkPathExists=true must refuse a dead path")
	}
	if err := s.validateFileSet(dead, false); err != nil {
		t.Fatalf("checkPathExists=false must pass a dead path through, got %v", err)
	}

	// The flag never waives containment or the empty-path/enabled rule, only
	// the on-disk existence check.
	traversal := store.FileSet{Name: "docs", Path: "../etc", Enabled: false}
	if err := s.validateFileSet(traversal, false); err == nil {
		t.Fatal("checkPathExists=false must still refuse a traversal path")
	}
	emptyEnabled := store.FileSet{Name: "docs", Path: "", Enabled: true}
	if err := s.validateFileSet(emptyEnabled, false); err == nil {
		t.Fatal("checkPathExists=false must still refuse enabling a path-less set")
	}
}

// A directory holding only nested empty directories is empty, while a file at
// any depth makes it non-empty.
func TestFileSetSourceEmptyCountsOnlyFilesAsContent(t *testing.T) {
	root := t.TempDir()

	empty := filepath.Join(root, "empty")
	if err := os.MkdirAll(filepath.Join(empty, "nested", "deeper"), 0o750); err != nil {
		t.Fatal(err)
	}
	got, err := fileSetSourceEmpty(empty)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Fatal("a tree of only empty directories must read as empty")
	}

	withFile := filepath.Join(root, "with-file")
	if err := os.MkdirAll(filepath.Join(withFile, "nested"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(withFile, "nested", "leaf.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = fileSetSourceEmpty(withFile)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Fatal("a tree with a deeply nested file must not read as empty")
	}
}

func TestDefaultHostConfigFileSet(t *testing.T) {
	if name, path, excludes, ok := defaultHostConfigFileSet(platform.KindGeneric); !ok || name == "" || path == "" || excludes != nil {
		t.Fatalf("KindGeneric: got name=%q path=%q excludes=%v ok=%v, want a non-empty name/path, nil excludes, ok=true", name, path, excludes, ok)
	}
	if name, path, excludes, ok := defaultHostConfigFileSet(platform.KindTrueNAS); !ok || name == "" || path == "" || excludes != nil {
		t.Fatalf("KindTrueNAS: got name=%q path=%q excludes=%v ok=%v, want a non-empty name/path, nil excludes, ok=true", name, path, excludes, ok)
	}
	if name, path, excludes, ok := defaultHostConfigFileSet(platform.KindUnraid); ok || name != "" || path != "" || excludes != nil {
		t.Fatalf("KindUnraid: got name=%q path=%q excludes=%v ok=%v, want all-zero, ok=false (flash domain already covers this)", name, path, excludes, ok)
	}
	// paths.Resolve rejects an absolute sub, so the preset path must be relative.
	if _, path, _, _ := defaultHostConfigFileSet(platform.KindGeneric); strings.HasPrefix(path, "/") {
		t.Fatalf("preset path %q must be relative to HostMountRoot, not absolute", path)
	}
}

// fileSetPositionals turns a stored selection into restic's source paths. Every
// entry is checked against the set root on each compile: entries outside it are
// dropped, and when nothing is left the root itself is used.
func TestFileSetPositionals(t *testing.T) {
	const src = "/host/user/data/docs"
	cases := []struct {
		name     string
		selected []string
		want     []string
	}{
		{"nil selection is the set root", nil, []string{src}},
		{"entry equal to the root anchors", []string{src}, []string{src}},
		{"disjoint entries under the root are kept, canonically ordered", []string{src + "/b", src + "/a"}, []string{src + "/a", src + "/b"}},
		{"redundant descendant collapses to the maximal root", []string{src, src + "/child"}, []string{src}},
		{"entries outside the root are filtered", []string{"/elsewhere/other", src + "/a"}, []string{src + "/a"}},
		{"all entries outside the root fall back to the root", []string{"/elsewhere/other"}, []string{src}},
		{"zero includes after filtering fall back to the root", []string{"!" + src + "/gone"}, []string{src}},
		{"exact child entry is kept as-is", []string{src + "/exact"}, []string{src + "/exact"}},
		{"deeply nested descendant collapses to the maximal root", []string{src, src + "/child/deep"}, []string{src}},
		{"sibling sharing the root's prefix does not match it", []string{src[:len(src)-1] /* ".../doc" */}, []string{src}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := fileSetPositionals(c.selected, src)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("fileSetPositionals(%v, %q) = %v, want %v", c.selected, src, got, c.want)
			}
		})
	}

	t.Run("compile is deterministic and canonically ordered", func(t *testing.T) {
		// Stable output keeps snapshot paths the same from run to run. The
		// entries are stored unsorted because the compile owns the order.
		stored := []string{src + "/zeta", src + "/alpha/inner", "/elsewhere/filtered-out"}
		want := []string{src + "/alpha/inner", src + "/zeta"}
		first := fileSetPositionals(stored, src)
		second := fileSetPositionals(stored, src)
		if !reflect.DeepEqual(first, want) {
			t.Fatalf("first compile = %v, want the canonical order %v", first, want)
		}
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("compile is not deterministic: %v vs %v", first, second)
		}
	})
}

// A file-set restore records its run against the set's id, with no container
// target lookup.
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

// When the store cannot say whether the repository was ever established, it
// may sit on a share that is not mounted with every snapshot still in it, so
// the refusal treats that as having backups. It is an internal test because
// only a broken schema makes that read fail.
func TestFileSetHasBackupsUnknownEstablishmentCountsAsHasBackups(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.FilesPath = "backups/files" // never created on disk
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	set, err := st.CreateFileSet(store.FileSet{Name: "docs", Path: "data/docs", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}, st, nil, nil, nil)

	if _, err := db.Exec("DROP TABLE established_repos"); err != nil {
		t.Fatal(err)
	}

	hasBackups, bErr := svc.fileSetHasBackups(context.Background(), set.ID)
	if !hasBackups {
		t.Fatalf("an unreadable establishment marker must count as having backups, got hasBackups=%v err=%v", hasBackups, bErr)
	}
	if !errors.Is(bErr, errFileSetRepoUnreachable) {
		t.Fatalf("want errFileSetRepoUnreachable, got %v", bErr)
	}
}

// The busy refusal names whichever operation holds the files domain lock, not
// always "backup".
func TestFileSetPatchBusyRefusalNamesTheHoldingOperation(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "data", "docs"), 0o750); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	set, err := st.CreateFileSet(store.FileSet{Name: "docs", Path: "data/docs", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(cfg, st, nil, nil, nil)
	h := NewHandler(cfg, st, nil, svc, nil, nil)

	unlock := svc.lockDomainFor("files", "delete")
	defer unlock()

	req := httptest.NewRequest(http.MethodPatch, "/api/files/sets/"+set.ID, strings.NewReader(`{"name":"renamed"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", set.ID)
	w := httptest.NewRecorder()
	h.handlePatchFileSet(w, req)

	var resp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	if resp.OK {
		t.Fatal("rename while delete holds the files lock must be refused")
	}
	if !strings.Contains(resp.Error, "delete") {
		t.Fatalf("want the refusal to name the holding operation, got %q", resp.Error)
	}
}

// A repository chosen for the set after discovery first read it is the
// operator's, so the repair leaves it.
func TestDiscoverHomeKeepsARepositoryChosenSinceTheFirstRead(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	chosen, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Role: store.RoleRepo, Name: "chosen", Repo: "backups/chosen", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	set, err := st.CreateFileSet(store.FileSet{Name: "docs", Repo: chosen.ID, RepoChosen: store.RepoChosen})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}, st, nil, nil, nil)

	item := store.ItemRef{Domain: "files", Key: set.ID}
	if left, err := svc.discoverHome(context.Background(), item, "found-elsewhere", nil, true); err != nil || left {
		t.Fatalf("discoverHome = %v, %v; want it to leave the chosen repository alone", left, err)
	}
	if got, err := st.GetFileSet(set.ID); err != nil || got.Repo != chosen.ID {
		t.Fatalf("repo = %q, %v; want the chosen %q kept", got.Repo, err, chosen.ID)
	}
}
