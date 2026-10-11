package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/secret"
)

func TestAPairedSessionBacksUpTheContainerItReplaces(t *testing.T) {
	r := newRestoreRig(t)
	tg, snap := r.seedPlex(t)
	r.write(t, map[string]string{"appdata/plex/library.db": "a newer library"})
	h := NewHandler(r.svc.cfg, r.st, nil, r.svc, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/containers/plex/restore", strings.NewReader(`{"snapshotId":"`+snap+`","confirm":true,"backupFirst":false}`))
	req.SetPathValue("name", "plex")
	w := httptest.NewRecorder()
	h.handleRestore(w, withPaired(req.WithContext(context.Background())))
	if !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Fatalf("restore: %s", w.Body.String())
	}
	r.wait(t)
	if r.backupRuns(t, tg.ID) != 2 {
		t.Fatalf("backup runs = %d, want the seed and the one before the restore", r.backupRuns(t, tg.ID))
	}
}

func TestAPairedSessionIsRefusedARestoreItCouldNotUndo(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	hash, epoch, _ := in.h.authEnabled()
	in.session = secret.NewPairedSessionToken(in.appKey, hash, epoch, time.Hour)
	for _, route := range []struct {
		path string
		body map[string]any
	}{
		{"/api/zfs/datasets/abc/restore", map[string]any{"stamp": "s", "dataset": "tank/data", "confirm": true, "safetyOffConfirm": true}},
		{"/api/containers/plex/restore-files", map[string]any{"snapshotId": "abc", "paths": []string{"/a"}, "confirm": true}},
		{"/api/files/sets/abc/restore-files", map[string]any{"snapshotId": "abc", "paths": []string{"/a"}, "confirm": true}},
		{"/api/stacks/media/restore", map[string]any{"confirm": true}},
		{"/api/anomalies/acknowledge", map[string]any{"ids": []string{"abc"}}},
	} {
		code, out := in.do(t, http.MethodPost, route.path, route.body)
		if code != http.StatusForbidden || out["code"] != codeOwnLoginNeeded {
			t.Errorf("%s with a paired session: %d %v", route.path, code, out)
		}
	}
}

func TestAPairedSessionIsRefusedWhatReachesPastTheApp(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	hash, epoch, _ := in.h.authEnabled()
	if code, out := in.do(t, http.MethodGet, "/api/auth/passkeys", nil); code != http.StatusOK || out["passkeys"] == nil {
		t.Fatalf("the person's own session gets no passkey list, so this test proves nothing: %d %v", code, out)
	}
	in.session = secret.NewPairedSessionToken(in.appKey, hash, epoch, time.Hour)
	for _, route := range []struct {
		path string
		body map[string]any
	}{
		{"/api/containers/postgres/dbdumps/abc/import", map[string]any{}},
		{"/api/flash/plugins/restore", map[string]any{"name": "a.plg"}},
		{"/api/runs/ack", map[string]any{"all": true}},
		{"/api/runs/ack", map[string]any{"ids": []string{"1", "2"}}},
		{"/api/zfs/datasets/abc/restore", map[string]any{"stamp": "s", "dataset": "tank/data", "newDataset": "tank/new", "safetySnapshot": true}},
	} {
		code, out := in.do(t, http.MethodPost, route.path, route.body)
		if code != http.StatusForbidden || out["code"] != codeOwnLoginNeeded {
			t.Errorf("%s %v with a paired session: %d %v", route.path, route.body, code, out)
		}
	}
	if code, out := in.do(t, http.MethodPost, "/api/runs/ack", map[string]any{"ids": []string{strings.Repeat("a", 32)}}); code != http.StatusOK || out["ok"] != true {
		t.Errorf("acknowledging one failure with a paired session: %d %v", code, out)
	}
	if code, out := in.do(t, http.MethodGet, "/api/auth/passkeys", nil); code != http.StatusOK || out["passkeys"] != nil {
		t.Errorf("a paired session read the passkeys: %d %v", code, out)
	}
	// What the phone app reads for its own screens.
	for _, path := range []string{"/api/runs", "/api/progress", "/api/schedule/next", "/api/display-prefs"} {
		if code, _ := in.do(t, http.MethodGet, path, nil); code == http.StatusForbidden || code == http.StatusUnauthorized {
			t.Errorf("%s with a paired session: %d", path, code)
		}
	}
}
