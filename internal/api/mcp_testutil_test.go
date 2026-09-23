package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/spike"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Two APP_KEY values, so a test can reopen one database under a different
// pepper and see what a reinstall does to the stored digests.
const (
	mcpAppKey      = "1111111111111111111111111111111111111111111111111111111111111111"
	mcpAppKeyAfter = "2222222222222222222222222222222222222222222222222222222222222222"
)

// mcpTestHost is a single-label name, the shape the host rule lets through
// while no login password is set. httptest's default "example.com" is exactly
// the public-looking name the rule refuses.
const mcpTestHost = "tower"

// newMCPKeyRouter wires a router over st with appKey as the pepper, on a data
// dir that carries the certificate the server writes at boot.
func newMCPKeyRouter(t *testing.T, st *store.Repo, appKey string) (http.Handler, *api.Service) {
	t.Helper()
	dir := t.TempDir()
	if _, _, err := api.EnsureSelfSigned(dir); err != nil {
		t.Fatal(err)
	}
	return mcpRouterWith(t, st, config.Config{AppKey: appKey, DataDir: dir, HostMountRoot: dir})
}

func mcpRouterWith(t *testing.T, st *store.Repo, cfg config.Config) (http.Handler, *api.Service) {
	t.Helper()
	d := &fakeServiceDocker{}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, &fakeResticEngine{})
	sched := schedule.New(func(string) error { return nil }, st.ListTargets)
	return api.NewHandler(cfg, st, d, svc, sched, spike.DefaultProbes()).Router(), svc
}

// doMCPKeyJSON is doJSON with the two things the key routes decide on besides
// the body: the host the request claims to have reached, and a session cookie.
// A host also brings the same-origin Origin header a rebinding page would send.
func doMCPKeyJSON(t *testing.T, h http.Handler, method, path, body, host, cookie string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	if host != "" {
		r.Host = host
		r.Header.Set("Origin", "https://"+host)
	}
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: "bv_session", Value: cookie}) //nolint:gosec // G124: request cookie; Secure and HttpOnly only apply to responses
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var m map[string]any
	if w.Body.Len() > 0 {
		_ = json.Unmarshal(w.Body.Bytes(), &m)
	}
	return w, m
}

// doMCPKey sends one key-route request from a host the rule lets through.
func doMCPKey(t *testing.T, h http.Handler, method, path, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	return doMCPKeyJSON(t, h, method, path, body, mcpTestHost, "")
}

// createMCPKey creates a key through the API and returns the secret, which is
// shown this one time, and the id everything else refers to it by.
func createMCPKey(t *testing.T, h http.Handler, label string, canStart bool) (key, id string) {
	t.Helper()
	w, m := doMCPKey(t, h, http.MethodPost, "/api/mcp/keys",
		fmt.Sprintf(`{"label":%q,"canStartBackups":%t}`, label, canStart))
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("create key %q: status=%d body=%v", label, w.Code, m)
	}
	key, _ = m["key"].(string)
	item, _ := m["item"].(map[string]any)
	id, _ = item["id"].(string)
	if key == "" || id == "" {
		t.Fatalf("create key %q: no key or id in %v", label, m)
	}
	return key, id
}

// mcpKeyList reads GET /api/mcp/keys.
func mcpKeyList(t *testing.T, h http.Handler) map[string]any {
	t.Helper()
	w, m := doMCPKey(t, h, http.MethodGet, "/api/mcp/keys", "")
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("list keys: status=%d body=%v", w.Code, m)
	}
	return m
}

// mcpKeyRows returns the "keys" or "revoked" array of a key listing.
func mcpKeyRows(t *testing.T, list map[string]any, field string) []map[string]any {
	t.Helper()
	raw, ok := list[field].([]any)
	if !ok {
		t.Fatalf("listing has no %s array: %v", field, list)
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		row, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("%s carries a non-object row: %v", field, item)
		}
		out = append(out, row)
	}
	return out
}

// enableLogin stores a login password and returns a session cookie for it.
func enableLogin(t *testing.T, st *store.Repo, appKey string) string {
	t.Helper()
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	hash, err := secret.HashPassword(appKey, "hunter2")
	if err != nil {
		t.Fatal(err)
	}
	s.AuthPasswordHash = hash
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	return secret.NewSessionToken(appKey, hash, s.SessionEpoch, 7*24*time.Hour)
}
