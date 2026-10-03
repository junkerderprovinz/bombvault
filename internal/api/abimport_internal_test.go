package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// importFakeEngine records what an import hands restic, including the tree it
// finds in the staging folder at that moment.
type importFakeEngine struct {
	ResticEngine
	mu       sync.Mutex
	snaps    []restic.Snapshot
	snapsErr error
	// failFor names a container whose archives restic refuses to import.
	failFor string
	imports []importCall
}

type importCall struct {
	dir   string
	tags  []string
	at    time.Time
	files []string
}

func (e *importFakeEngine) RepoOpens(context.Context, string, restic.Mode) bool { return true }

func (e *importFakeEngine) Snapshots(context.Context, string, restic.Mode) ([]restic.Snapshot, error) {
	return e.snaps, e.snapsErr
}

func (e *importFakeEngine) ImportDir(_ context.Context, _, dir string, tags []string, at time.Time, _ restic.Mode) (restic.Summary, error) {
	if e.failFor != "" && slices.Contains(tags, "container:"+e.failFor) {
		return restic.Summary{}, errors.New("repository is full")
	}
	var files []string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(dir, p)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(files)
	e.mu.Lock()
	defer e.mu.Unlock()
	e.imports = append(e.imports, importCall{dir: dir, tags: tags, at: at, files: files})
	return restic.Summary{SnapshotID: fmt.Sprintf("%064x", len(e.imports)), BytesAdded: 10}, nil
}

func (e *importFakeEngine) calls() []importCall {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]importCall(nil), e.imports...)
}

func gzTar(t *testing.T, file string, files map[string]string) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if err := tw.WriteHeader(&tar.Header{Name: n, Mode: 0o644, Size: int64(len(files[n])), Typeflag: tar.TypeReg, ModTime: time.Unix(1_700_000_000, 0)}); err != nil {
			t.Fatal(err)
		}
		_, _ = tw.Write([]byte(files[n]))
	}
	_ = tw.Close()
	_ = gz.Close()
	if err := os.WriteFile(file, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func importFixture(t *testing.T) (*Service, *store.Repo, *importFakeEngine, string) {
	t.Helper()
	root := filepath.ToSlash(t.TempDir())
	st := newTestStore(t)
	repo := filepath.Join(root, "user", "bombvault", "containers")
	if err := os.MkdirAll(repo, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersPath = "user/bombvault/containers"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"plex", "sonarr"} {
		if _, err := st.UpsertTarget(store.Target{ContainerName: name, AppdataPaths: []string{root + "/user/appdata/" + name}, Definition: `{"x":1}`}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.UpsertTarget(store.Target{ContainerName: "fresh"}); err != nil {
		t.Fatal(err)
	}
	eng := &importFakeEngine{}
	s := &Service{
		store:  st,
		engine: eng,
		cfg:    config.Config{HostMountRoot: root, HostSourceRoot: "/mnt", DataDir: filepath.Join(root, "config"), AppKey: strings.Repeat("a", 64)},
	}
	src := filepath.Join(root, "user", "backups")
	for folder, archives := range map[string][]string{
		"ab_20250101_030000": {"plex", "sonarr", "fresh", "gone"},
		"ab_20250201_030000": {"plex"},
	} {
		dir := filepath.Join(src, folder)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
		for _, name := range archives {
			gzTar(t, filepath.Join(dir, name+".tar.gz"), map[string]string{
				"/mnt/user/appdata/" + name + "/config.xml": folder + " " + name,
			})
		}
		if err := os.WriteFile(filepath.Join(dir, "my-plex.xml"), []byte("<Container>"+folder+"</Container>"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return s, st, eng, "user/backups"
}

func TestScanSaysWhatAnImportWouldDoWithEachArchive(t *testing.T) {
	s, _, eng, src := importFixture(t)
	eng.snaps = []restic.Snapshot{{ID: "old", Tags: []string{"container:plex", "import:ab_20250101_030000"}}}

	got, err := s.ScanAppdataBackup(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	status := map[string]string{}
	for _, a := range got {
		status[a.Folder+"/"+a.Container] = a.Status
	}
	want := map[string]string{
		"ab_20250101_030000/plex":   abStatusImported,
		"ab_20250101_030000/sonarr": abStatusNew,
		"ab_20250101_030000/fresh":  abStatusNotBackedUp,
		"ab_20250101_030000/gone":   abStatusNoContainer,
		"ab_20250201_030000/plex":   abStatusNew,
	}
	for k, v := range want {
		if status[k] != v {
			t.Errorf("%s = %q, want %q", k, status[k], v)
		}
	}
}

// importDocker knows one running container that BombVault never backed up.
type importDocker struct{ dockercli.Docker }

func (importDocker) Inspect(_ context.Context, name string) (model.Inspect, error) {
	if name == "gone" {
		return model.Inspect{Name: "/gone"}, nil
	}
	return model.Inspect{}, errors.New("no such container")
}

func TestScanTellsAContainerWithoutABackupFromOneThatDoesNotExist(t *testing.T) {
	s, _, _, src := importFixture(t)
	s.docker = importDocker{}
	got, err := s.ScanAppdataBackup(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range got {
		if a.Container == "gone" && a.Status != abStatusNotBackedUp {
			t.Fatalf("gone = %q, want %q: the container exists, it only has no backup", a.Status, abStatusNotBackedUp)
		}
	}
}

func TestImportMakesARestorePointPerNewArchive(t *testing.T) {
	s, st, eng, src := importFixture(t)
	archive := filepath.Join(s.cfg.HostMountRoot, src, "ab_20250201_030000", "plex.tar.gz")
	before := fileHash(t, archive)

	n, err := s.StartImportAppdataBackup(context.Background(), src)
	if err != nil || n != 3 {
		t.Fatalf("started %d, %v; want the three new archives", n, err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for s.batchActive.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	calls := eng.calls()
	if len(calls) != 3 {
		t.Fatalf("imports = %d, want 3", len(calls))
	}
	var plex *importCall
	for i, c := range calls {
		if strings.Join(c.tags, ",") == "container:plex,import:ab_20250201_030000" {
			plex = &calls[i]
		}
	}
	if plex == nil {
		t.Fatalf("no import tagged for plex of February: %+v", calls)
	}
	wantAt := time.Date(2025, 2, 1, 3, 0, 0, 0, time.Local)
	if !plex.at.Equal(wantAt) {
		t.Fatalf("snapshot time = %v, want the folder's %v", plex.at, wantAt)
	}
	if len(plex.files) != 1 || !strings.HasSuffix(plex.files[0], "/user/appdata/plex/config.xml") {
		t.Fatalf("staged files = %v, want the archive's file below the container path", plex.files)
	}
	if _, err := os.Stat(plex.dir); err == nil {
		t.Fatal("the staging folder was left behind")
	}
	if fileHash(t, archive) != before {
		t.Fatal("the source archive changed")
	}
	tmpl, err := os.ReadFile(filepath.Join(s.cfg.DataDir, "templates", "my-"+importedID(calls, plex)+"-plex.xml"))
	if err != nil || string(tmpl) != "<Container>ab_20250201_030000</Container>" {
		t.Fatalf("template = %q %v, want the one saved in that folder", tmpl, err)
	}
	tg, _ := st.GetTargetByContainer("plex")
	runs, err := st.RecentRunsOfKind(tg.ID, "import", 5)
	if err != nil || len(runs) != 2 || runs[0].Status != "success" {
		t.Fatalf("import runs = %+v %v", runs, err)
	}
}

func importedID(calls []importCall, c *importCall) string {
	for i := range calls {
		if &calls[i] == c {
			return fmt.Sprintf("%064x", i+1)
		}
	}
	return ""
}

func TestAContainerWhoseRepositoryIsSwitchedOffLeavesTheOthersToImport(t *testing.T) {
	s, st, eng, src := importFixture(t)
	off, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Role: store.RoleRepo, Name: "Cold", Repo: "backups/cold", Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.WritePlacement(store.ItemRef{Domain: "containers", Key: "sonarr"}, &store.HomeWrite{Repo: off.ID, Choice: store.RepoChosen}, nil, nil); err != nil {
		t.Fatal(err)
	}
	got, err := s.ScanAppdataBackup(context.Background(), src)
	if err != nil {
		t.Fatalf("scan failed for the whole folder: %v", err)
	}
	status := map[string]string{}
	for _, a := range got {
		status[a.Folder+"/"+a.Container] = a.Status
	}
	if status["ab_20250101_030000/sonarr"] != abStatusRepoUnavailable || status["ab_20250201_030000/plex"] != abStatusNew {
		t.Fatalf("statuses = %v", status)
	}
	n, err := s.StartImportAppdataBackup(context.Background(), src)
	if err != nil || n != 2 {
		t.Fatalf("started %d, %v; want the two archives of plex", n, err)
	}
	waitFor(t, "the import", func() bool { return !s.batchActive.Load() })
	if calls := eng.calls(); len(calls) != 2 {
		t.Fatalf("imports = %d, want 2", len(calls))
	}
}

func TestAnUnreadableRepositoryMarksItsArchivesAndTheScanGoesOn(t *testing.T) {
	s, _, eng, src := importFixture(t)
	eng.snapsErr = errors.New("repository is locked")
	got, err := s.ScanAppdataBackup(context.Background(), src)
	if err != nil {
		t.Fatalf("scan failed for the whole folder: %v", err)
	}
	for _, a := range got {
		if a.Container == "plex" && a.Status != abStatusRepoUnavailable {
			t.Fatalf("%s/%s = %q, want %q", a.Folder, a.Container, a.Status, abStatusRepoUnavailable)
		}
	}
}

func TestTheLastImportEventCountsTheArchivesThatFailed(t *testing.T) {
	s, _, eng, src := importFixture(t)
	eng.failFor = "plex"
	s.progress = progress.NewStore()
	events, stop := s.progress.Subscribe()
	defer stop()
	if _, err := s.StartImportAppdataBackup(context.Background(), src); err != nil {
		t.Fatal(err)
	}
	timeout := time.After(5 * time.Second)
	for {
		select {
		case e := <-events:
			if e.Key != appdataImportKey || e.Active {
				continue
			}
			if e.Failed != 2 {
				t.Fatalf("last event = %+v, want the two archives of plex counted as failed", e)
			}
			return
		case <-timeout:
			t.Fatal("no last event")
		}
	}
}

func TestImportRefusesWhenNothingIsNew(t *testing.T) {
	s, _, eng, src := importFixture(t)
	eng.snaps = []restic.Snapshot{
		{ID: "a", Tags: []string{"container:plex", "import:ab_20250101_030000"}},
		{ID: "b", Tags: []string{"container:plex", "import:ab_20250201_030000"}},
		{ID: "c", Tags: []string{"container:sonarr", "import:ab_20250101_030000"}},
	}
	if n, err := s.StartImportAppdataBackup(context.Background(), src); err == nil || n != 0 {
		t.Fatalf("started %d, %v; want nothing to import", n, err)
	}
}

func TestImportedSnapshotIsRestoredFromItsTree(t *testing.T) {
	s := &Service{cfg: config.Config{HostMountRoot: "/host/user"}}
	stored := []string{"/host/user/user/appdata/plex"}
	imported := restic.Snapshot{Paths: []string{"/host/user/user/bombvault/.bombvault-import/import-1"}, Tags: []string{"container:plex", "import:ab_20250101_030000"}}
	mapped, skipped, narrowed := mapRestorePaths(stored, s.restoreRootsOf(imported))
	if len(mapped) != 1 || mapped[0] != stored[0] || len(skipped) != 0 || len(narrowed) != 1 {
		t.Fatalf("mapped %v skipped %v narrowed %v; want the stored path checked against the tree", mapped, skipped, narrowed)
	}
	native := restic.Snapshot{Paths: stored, Tags: []string{"container:plex"}}
	if got := s.restoreRootsOf(native); len(got) != 1 || got[0] != stored[0] {
		t.Fatalf("a native snapshot keeps its recorded paths, got %v", got)
	}
}

func fileHash(t *testing.T, p string) [32]byte {
	t.Helper()
	b, err := os.ReadFile(p) //nolint:gosec // G304: a file inside the test's temp folder
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(b)
}
