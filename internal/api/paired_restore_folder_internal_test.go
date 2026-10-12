package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// folderRestoreEngine lists one snapshot of the container plex and one of the
// file set Fotos, and records where each restore would write.
type folderRestoreEngine struct {
	checkEngine
	mu      sync.Mutex
	targets []string
}

func (e *folderRestoreEngine) restored(target string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.targets = append(e.targets, target)
	return nil
}

func (e *folderRestoreEngine) written() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.targets...)
}

func (e *folderRestoreEngine) RestoreInclude(_ context.Context, _, _, _, target string, _ restic.Mode) error {
	return e.restored(target)
}

func (e *folderRestoreEngine) RestoreSubtreeTo(_ context.Context, _, _, _, target string, _ restic.Mode) error {
	return e.restored(target)
}

func (e *folderRestoreEngine) RestoreSubtreeInclude(_ context.Context, _, _, _, _, target string, _ restic.Mode) error {
	return e.restored(target)
}

type folderRestoreRig struct {
	h    *Handler
	svc  *Service
	eng  *folderRestoreEngine
	root string
	set  string
}

func newFolderRestoreRig(t *testing.T) *folderRestoreRig {
	t.Helper()
	root := filepath.ToSlash(t.TempDir())
	st := newTestStore(t)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersPath = "backups/containers"
	settings.FilesPath = "backups/files"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	for _, repo := range []string{"backups/containers", "backups/files"} {
		if err := os.MkdirAll(root+"/"+repo, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(root+"/"+repo+"/config", []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.UpsertTarget(store.Target{ContainerName: "plex", AppdataPaths: []string{root + "/appdata/plex"}}); err != nil {
		t.Fatal(err)
	}
	set, err := st.CreateFileSet(store.FileSet{Name: "Fotos", Path: "fotos", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	eng := &folderRestoreEngine{checkEngine: checkEngine{snaps: []restic.Snapshot{
		{ID: "aaaa1111", Tags: []string{"container:plex"}, Paths: []string{root + "/appdata/plex"}},
		{ID: "bbbb2222", Tags: []string{"fileset:Fotos"}, Paths: []string{root + "/fotos"}},
	}}}
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: root + "/bombvault", HostMountRoot: root, HostSourceRoot: "/mnt/user"}
	svc := NewService(cfg, st, nil, nil, eng)
	return &folderRestoreRig{h: NewHandler(cfg, st, nil, svc, nil, nil), svc: svc, eng: eng, root: root, set: set.ID}
}

// pairedPost is a JSON request made with the session of a paired phone.
func pairedPost(t *testing.T, body any) *http.Request {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	return withPaired(req.WithContext(context.Background()))
}

// restore sends one paired restore into folder, below the host mount, by
// each route that restores into a folder.
func (r *folderRestoreRig) restore(t *testing.T, route, folder string) (int, map[string]any) {
	t.Helper()
	var body map[string]any
	var handle http.HandlerFunc
	param, value := "name", "plex"
	switch route {
	case "file set":
		body = map[string]any{"snapshotId": "bbbb2222", "targetPath": folder}
		handle = r.h.handleRestoreFileSet
		param, value = "id", r.set
	case "file set files":
		body = map[string]any{"snapshotId": "bbbb2222", "targetPath": folder, "confirm": true, "paths": []string{r.root + "/fotos/garten.jpg"}}
		handle = r.h.handleRestoreFileSetFiles
		param, value = "id", r.set
	case "container files":
		body = map[string]any{"snapshotId": "aaaa1111", "targetPath": folder, "confirm": true, "paths": []string{r.root + "/appdata/plex/library.db"}}
		handle = r.h.handleRestoreFiles
	case "container":
		body = map[string]any{"snapshotId": "aaaa1111", "targetPath": folder}
		handle = r.h.handleRestoreContainerTo
	}
	req := pairedPost(t, body)
	req.SetPathValue(param, value)
	w := httptest.NewRecorder()
	handle(w, req)
	deadline := time.Now().Add(5 * time.Second)
	for r.svc.BackupInProgress() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("%s: %v in %s", route, err, w.Body)
	}
	return w.Code, out
}

var folderRestoreRoutes = []string{"file set", "file set files", "container files", "container"}

func TestAPairedSessionCannotRestoreIntoAFolderThatHoldsFiles(t *testing.T) {
	for _, route := range folderRestoreRoutes {
		r := newFolderRestoreRig(t)
		live := r.root + "/appdata/vaultwarden"
		if err := os.MkdirAll(live, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(live+"/db.sqlite3", []byte("live"), 0o600); err != nil {
			t.Fatal(err)
		}
		code, out := r.restore(t, route, "appdata/vaultwarden")
		if code != http.StatusForbidden || out["code"] != codeOwnLoginNeeded {
			t.Errorf("%s into a live folder: %d %v", route, code, out)
		}
		if got := r.eng.written(); len(got) != 0 {
			t.Errorf("%s wrote into %v", route, got)
		}
	}
}

func TestAPairedSessionRestoresIntoANewFolder(t *testing.T) {
	for _, route := range folderRestoreRoutes {
		r := newFolderRestoreRig(t)
		code, out := r.restore(t, route, "restore/new")
		if code != http.StatusOK || out["ok"] != true {
			t.Errorf("%s into a new folder: %d %v", route, code, out)
			continue
		}
		if info, err := os.Stat(r.root + "/restore/new"); err != nil || !info.IsDir() {
			t.Errorf("%s did not create the folder: %v", route, err)
		}
	}
}

func TestAPairedSessionRestoresIntoAnEmptyFolderAndLeavesItsMode(t *testing.T) {
	for _, route := range folderRestoreRoutes {
		r := newFolderRestoreRig(t)
		empty := r.root + "/restore/empty"
		if err := os.MkdirAll(empty, 0o700); err != nil {
			t.Fatal(err)
		}
		code, out := r.restore(t, route, "restore/empty")
		if code != http.StatusOK || out["ok"] != true {
			t.Errorf("%s into an empty folder: %d %v", route, code, out)
			continue
		}
		info, err := os.Stat(empty)
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0o700 {
			t.Errorf("%s changed the mode of the folder to %o", route, info.Mode().Perm())
		}
	}
}

func TestAPairedSessionCannotRestoreAZFSMemberIntoAFolderThatHoldsFiles(t *testing.T) {
	s, st, _, eng := zfsRestoreFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)
	live := s.cfg.HostMountRoot + "/" + zfsMountedSub + "/share"
	if err := os.MkdirAll(live, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(live+"/doc.txt", []byte("live"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(s.cfg, st, nil, s, nil, nil)
	send := func(folder string) (int, string) {
		req := pairedPost(t, ZFSRestoreRequest{Stamp: zfsStamp, Dataset: zfsChild, TargetPath: zfsMountedSub + "/" + folder})
		req.SetPathValue("id", d.ID)
		w := httptest.NewRecorder()
		h.handleRestoreZFS(w, req)
		return w.Code, w.Body.String()
	}
	if code, body := send("share"); code != http.StatusForbidden || !strings.Contains(body, codeOwnLoginNeeded) {
		t.Fatalf("a member into a live folder: %d %s", code, body)
	}
	if got := eng.readRestores(); len(got) != 0 {
		t.Fatalf("restic calls = %v, want none", got)
	}
	if code, body := send("copy"); code != http.StatusOK || !strings.Contains(body, `"ok":true`) {
		t.Fatalf("a member into a new folder: %d %s", code, body)
	}
	zfsAwaitRestore(t, st, d.ID)
}
