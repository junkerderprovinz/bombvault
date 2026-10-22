package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// A proxy that gives up after a minute, or a closed tab, cancels the request
// while restic is still at work. The check, trial, unlock and prune carry on
// and record their own outcome instead of being killed part way.
func TestAManualMaintenanceRunOutlivesItsRequest(t *testing.T) {
	eng := &fakeResticEngine{snaps: []restic.Snapshot{{ID: "aaaa1111bbbb2222", Time: "2026-06-01T00:00:00Z"}}}
	h, st, _, dir := newTestRouterSvcDir(t, &fakeServiceDocker{}, eng)
	s := mustSettings(t, st)
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(dir, "backups", "containers")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	cookie := enableLogin(t, st, strings.Repeat("a", 64))

	for _, path := range []string{"/api/check/containers", "/api/verify/containers", "/api/unlock/containers", "/api/prune/containers"} {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		r := httptest.NewRequest(http.MethodPost, path, nil).WithContext(ctx)
		r.AddCookie(&http.Cookie{Name: "bv_session", Value: cookie}) //nolint:gosec // G124: request cookie; Secure and HttpOnly only apply to responses
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var m map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &m)
		if m["ok"] != true {
			t.Errorf("%s after the request went away = %d %v, want it done", path, w.Code, m)
		}
	}
	if len(eng.maintCtxErrs) == 0 {
		t.Fatal("restic was never called")
	}
	for i, err := range eng.maintCtxErrs {
		if err != nil {
			t.Errorf("restic call %d ran on a cancelled context: %v", i, err)
		}
	}
}
