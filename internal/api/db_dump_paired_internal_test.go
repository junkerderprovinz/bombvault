package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
)

// pairedSave asks to save the dump of "pg" into target with the session of a
// paired phone.
func pairedSave(t *testing.T, svc *Service, target string) map[string]any {
	t.Helper()
	h := NewHandler(svc.cfg, svc.store, nil, svc, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/containers/pg/dbdumps/3f9c2a1be0d4aaaa/save", strings.NewReader(`{"targetPath":"`+target+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("name", "pg")
	req.SetPathValue("id", "3f9c2a1be0d4aaaa")
	w := httptest.NewRecorder()
	h.handleSaveDBDumpTo(w, withPaired(req))
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("the save answered %q", w.Body.String())
	}
	return out
}

func TestAPairedPhoneSavesADumpIntoANewFolderInsideTheRestoreFolder(t *testing.T) {
	svc, _ := downloadFixture(t, []byte("-- dump\nSELECT 1;\n"))
	settings, err := svc.store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	folder := settings.RestoreFolderFor("containers") + "/pg-20261126-120000"

	if out := pairedSave(t, svc, folder); out["ok"] != true {
		t.Fatalf("a phone saving into a new folder of the restore folder: %v", out)
	}
	waitForDetachedRun(t, svc)
	saved, err := filepath.Glob(filepath.Join(svc.cfg.HostMountRoot, filepath.FromSlash(folder), "pg-*.sql"))
	if err != nil || len(saved) != 1 {
		t.Fatalf("saved %v (%v), want the one dump in the new folder", saved, err)
	}
}

func TestAPairedPhoneSavesNoDumpOutsideTheRestoreFolderOrIntoOneInUse(t *testing.T) {
	svc, _ := downloadFixture(t, []byte("-- dump\nSELECT 1;\n"))
	settings, err := svc.store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	inUse := settings.RestoreFolderFor("containers") + "/kept"
	if err := os.MkdirAll(filepath.Join(svc.cfg.HostMountRoot, filepath.FromSlash(inUse)), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(svc.cfg.HostMountRoot, filepath.FromSlash(inUse), "notes.txt"), []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, target := range []string{"user/appdata", "user/elsewhere/pg", inUse} {
		if out := pairedSave(t, svc, target); out["ok"] == true {
			t.Errorf("a phone saved a dump into %s: %v", target, out)
		}
	}
	if svc.BackupInProgress() {
		waitForDetachedRun(t, svc)
	}
	saved, _ := filepath.Glob(filepath.Join(svc.cfg.HostMountRoot, "user", "*", "pg-*.sql"))
	kept, _ := filepath.Glob(filepath.Join(svc.cfg.HostMountRoot, filepath.FromSlash(inUse), "pg-*.sql"))
	if len(saved)+len(kept) != 0 {
		t.Errorf("a refused save wrote %v %v", saved, kept)
	}
}

// unlistedDocker is a Docker whose container list does not answer, so no
// backup of a container can be taken.
type unlistedDocker struct{ *importFakeDocker }

func (unlistedDocker) List(context.Context) ([]dockercli.ContainerInfo, error) {
	return nil, errors.New("cannot connect to the Docker daemon")
}

func TestAPairedPhoneImportsADumpOnlyAfterTheDatabaseIsBackedUp(t *testing.T) {
	rig := newImportRig(t)
	rig.svc.docker = unlistedDocker{rig.dock}
	ctx := context.WithValue(context.Background(), pairedKey{}, true)

	started, err := rig.svc.StartImportDBDump(ctx, "pg", "local", importDumpID)
	if err != nil || !started {
		t.Fatalf("started=%v err=%v", started, err)
	}
	waitForDetachedRun(t, rig.svc)

	for _, c := range rig.dock.calls {
		if strings.HasPrefix(c, "stop:") || strings.HasPrefix(c, "import:") {
			t.Errorf("the import touched the database although no backup was taken first: %v", rig.dock.calls)
			break
		}
	}
	if kept := siblingsOf(t, rig.dataDir, ".bombvault-before-import-*"); len(kept) != 0 {
		t.Errorf("the data folder was set aside: %v", kept)
	}
	runs := runsOfKind(t, rig.svc.store, "dbimport")
	if len(runs) != 1 || runs[0].Status != "failed" || !strings.Contains(runs[0].Error, "backup of the current state failed") {
		t.Errorf("runs = %+v, want one failed import that names the backup first", runs)
	}
}

func TestAPageOverTheRelayDownloadsNoDump(t *testing.T) {
	const download = "GET /api/containers/{name}/dbdumps/{id}/download"
	if !pairedRoutes[download] {
		t.Fatal("a paired phone does not reach the download, so this test proves nothing")
	}
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	mux := http.NewServeMux()
	mux.HandleFunc(download, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	req := requestFor(download)
	w := httptest.NewRecorder()
	in.h.authGate(mux).ServeHTTP(w, req.WithContext(context.WithValue(req.Context(), relayedKey{}, true)))
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), codeOwnLoginNeeded) {
		t.Errorf("a page over the relay downloading a dump = %d %s, want own-login-needed", w.Code, w.Body.String())
	}
}
