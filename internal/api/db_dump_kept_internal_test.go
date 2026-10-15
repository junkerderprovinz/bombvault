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
	"sync"
	"testing"
)

// keptFolder puts a folder an earlier import kept next to the rig's data
// folder and returns it.
func keptFolder(t *testing.T, rig *importRig, stamp string) string {
	t.Helper()
	kept := rig.dataDir + dbImportKept + stamp
	if err := os.MkdirAll(filepath.Join(kept, "base"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kept, "PG_VERSION"), []byte("16\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return kept
}

func TestTheKeptDataFolderOfAnImportCanBeRemoved(t *testing.T) {
	rig := newImportRig(t)
	kept := keptFolder(t, rig, "20261015-194119")

	removed, err := rig.svc.RemoveKeptDataDir(context.Background(), "pg", "20261015-194119")
	if err != nil {
		t.Fatalf("RemoveKeptDataDir: %v", err)
	}
	if removed != "/data/pg.bombvault-before-import-20261015-194119" {
		t.Errorf("removed = %q, want the folder as the host names it", removed)
	}
	if _, err := os.Stat(kept); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the kept folder is still there: %v", err)
	}
	if _, err := os.Stat(filepath.Join(rig.dataDir, "PG_VERSION")); err != nil {
		t.Fatalf("the data folder itself was touched: %v", err)
	}
}

func TestRemovingAKeptFolderTakesOnlyAStampOfOne(t *testing.T) {
	rig := newImportRig(t)
	other := filepath.Join(filepath.Dir(rig.dataDir), "pg.bombvault-before-import-x")
	if err := os.MkdirAll(other, 0o750); err != nil {
		t.Fatal(err)
	}

	for _, stamp := range []string{"x", "../pg", "20261015-1941190", "2026-10-15", ""} {
		if _, err := rig.svc.RemoveKeptDataDir(context.Background(), "pg", stamp); !errors.Is(err, errBadKeptStamp) {
			t.Errorf("stamp %q: err = %v, want errBadKeptStamp", stamp, err)
		}
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("a folder no stamp names was removed: %v", err)
	}
	if _, err := rig.svc.RemoveKeptDataDir(context.Background(), "pg", "20261015-194119"); !errors.Is(err, errKeptGone) {
		t.Fatalf("missing folder: err = %v, want errKeptGone", err)
	}
}

func TestAKeptFolderStaysWhileAnImportRuns(t *testing.T) {
	rig := newImportRig(t)
	kept := keptFolder(t, rig, "20261015-194119")
	rig.svc.repoMu = map[string]*sync.Mutex{"containers": {}}
	unlock := rig.svc.lockDomainFor("containers", "restore")
	defer unlock()

	if _, err := rig.svc.RemoveKeptDataDir(context.Background(), "pg", "20261015-194119"); !errors.Is(err, errDomainBusy) {
		t.Fatalf("err = %v, want errDomainBusy", err)
	}
	if _, err := os.Stat(kept); err != nil {
		t.Fatalf("the kept folder went while the domain was busy: %v", err)
	}
}

func TestTheRemoveRouteAnswersWithTheRemovedFolder(t *testing.T) {
	rig := newImportRig(t)
	keptFolder(t, rig, "20261015-194119")
	h := &Handler{svc: rig.svc}

	r := httptest.NewRequest(http.MethodDelete, "/api/containers/pg/dbimport-kept/20261015-194119", nil)
	r.SetPathValue("name", "pg")
	r.SetPathValue("stamp", "20261015-194119")
	w := httptest.NewRecorder()
	h.handleRemoveKeptDataDir(w, r)

	var body struct {
		OK      bool   `json:"ok"`
		Removed string `json:"removed"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %s: %v", w.Body.String(), err)
	}
	if !body.OK || !strings.HasSuffix(body.Removed, "pg.bombvault-before-import-20261015-194119") {
		t.Fatalf("answer = %s", w.Body.String())
	}
}
