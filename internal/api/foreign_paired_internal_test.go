package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// pairedForeign sends body to a foreign route's handler with the session of a
// paired phone.
func pairedForeign(t *testing.T, s *Service, handle func(*Handler) http.HandlerFunc, body any) (int, map[string]any) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/foreign/x", strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handle(NewHandler(s.cfg, s.store, nil, s, nil, nil))(w, withPaired(req))
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("the route answered %q", w.Body.String())
	}
	return w.Code, out
}

func openForeign(h *Handler) http.HandlerFunc    { return h.handleForeignOpen }
func restoreForeign(h *Handler) http.HandlerFunc { return h.handleForeignRestore }

func refusedToPhone(code int, out map[string]any) bool {
	return code == http.StatusForbidden && out["code"] == codeOwnLoginNeeded
}

// restoreFolderOf is the restore folder of a section, relative to the host mount.
func restoreFolderOf(t *testing.T, s *Service, section string) string {
	t.Helper()
	settings, err := s.store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	return settings.RestoreFolderFor(section)
}

func foreignDocsService(t *testing.T) (*Service, *foreignRecordingEngine) {
	t.Helper()
	eng := &foreignRecordingEngine{
		opens: opensEncrypted,
		snaps: []restic.Snapshot{
			{ID: "eeeeeeee55555555", Time: "2026-07-05T10:00:00Z", Tags: []string{"fileset:docs"}},
			{ID: "abcdef0123456789", Time: "2026-07-05T10:00:00Z", Tags: []string{"container:web"}, Paths: []string{"/host/user/appdata/web"}},
		},
	}
	return newForeignTestService(t, eng), eng
}

func TestAPairedPhoneOpensAnOldFolderOnlyInsideTheRestoreFolder(t *testing.T) {
	s, _ := foreignDocsService(t)
	inside := restoreFolderOf(t, s, "files") + "/old"
	seedForeignRepoMarker(t, s, inside)
	seedForeignRepoMarker(t, s, "backups/other")

	if code, out := pairedForeign(t, s, openForeign, map[string]any{"location": "backups/other", "key": foreignTestKey}); !refusedToPhone(code, out) {
		t.Errorf("a phone opened a folder outside the restore folder: %d %v", code, out)
	}
	code, out := pairedForeign(t, s, openForeign, map[string]any{"location": inside, "key": foreignTestKey})
	if code != http.StatusOK || out["ok"] != true || out["session"] == "" {
		t.Fatalf("a phone opening an old folder inside the restore folder: %d %v", code, out)
	}
	if raw, _ := json.Marshal(out); strings.Contains(string(raw), foreignTestKey) {
		t.Errorf("the answer carries the key: %s", raw)
	}
}

func TestAPairedPhoneOpensARemoteRepositoryWithoutHearingItsCredentialsBack(t *testing.T) {
	s, _ := foreignDocsService(t)
	code, out := pairedForeign(t, s, openForeign, map[string]any{
		"location": "s3:https://s3.example.com/old-bucket",
		"key":      foreignTestKey,
		"creds":    map[string]any{"s3KeyId": "AKIDOLDREPO", "s3Secret": "OLDREPOSECRET"},
	})
	if code != http.StatusOK || out["ok"] != true {
		t.Fatalf("a phone opening a remote repository: %d %v", code, out)
	}
	raw, _ := json.Marshal(out)
	for _, leak := range []string{"AKIDOLDREPO", "OLDREPOSECRET", foreignTestKey} {
		if strings.Contains(string(raw), leak) {
			t.Errorf("the answer carries %q: %s", leak, raw)
		}
	}
}

func TestAPairedPhoneRestoresFromAnOldFolderOnlyIntoANewFolderOfTheRestoreFolder(t *testing.T) {
	s, _ := foreignDocsService(t)
	inside := restoreFolderOf(t, s, "files") + "/old"
	seedForeignRepoMarker(t, s, inside)
	id, _, err := s.OpenForeign(context.Background(), inside, foreignTestKey, nil)
	if err != nil {
		t.Fatal(err)
	}
	inUse := restoreFolderOf(t, s, "files") + "/kept"
	kept, err := paths.Resolve(s.cfg.HostMountRoot, inUse)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(kept, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kept, "mine.txt"), []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}

	fresh := restoreFolderOf(t, s, "files") + "/docs-20261126-120000"
	for _, body := range []map[string]any{
		{"domain": "containers", "item": "web", "target": restoreFolderOf(t, s, "containers") + "/web"},
		{"domain": "vms", "item": "win11", "target": restoreFolderOf(t, s, "vms") + "/win11"},
		{"domain": "files", "item": "docs", "target": "restore-here/docs"},
		{"domain": "files", "item": "docs", "target": ""},
		{"domain": "files", "item": "docs", "target": inUse},
		{"domain": "files", "item": "docs", "target": fresh, "overwrite": true},
		{"domain": "containers", "item": "web", "toFolder": true, "target": "restore-here/web"},
		{"domain": "vms", "item": "win11", "toFolder": true, "target": restoreFolderOf(t, s, "vms") + "/win11", "zvolPool": "tank"},
	} {
		body["session"], body["snapshot"], body["confirm"] = id, "latest", true
		if code, out := pairedForeign(t, s, restoreForeign, body); !refusedToPhone(code, out) {
			t.Errorf("a phone restored %v: %d %v", body, code, out)
		}
	}
	if _, err := os.Stat(filepath.Join(kept, "docs")); err == nil {
		t.Error("a refused restore wrote into the folder in use")
	}

	code, out := pairedForeign(t, s, restoreForeign, map[string]any{
		"session": id, "domain": "files", "item": "docs", "snapshot": "latest", "confirm": true, "target": fresh,
	})
	if code != http.StatusOK || out["ok"] != true {
		t.Fatalf("a phone restoring a folder set into a new folder of the restore folder: %d %v", code, out)
	}
	waitForeignIdle(t, s)
}
