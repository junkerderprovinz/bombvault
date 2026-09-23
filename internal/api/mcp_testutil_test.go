package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
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

// newMCPToolRouter is a router with one key that may start backups, which is
// what every tool test needs before it can call anything.
func newMCPToolRouter(t *testing.T, d *fakeServiceDocker, eng *fakeResticEngine) (http.Handler, *store.Repo, *api.Service, string) {
	t.Helper()
	h, st, svc := newTestRouterSvc(t, d, eng)
	key, _ := createMCPKey(t, h, "Laptop", true)
	return h, st, svc, key
}

// mcpToolResult is one tools/call answer with the JSON-RPC envelope taken off.
type mcpToolResult struct {
	IsError bool `json:"isError"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Structured map[string]any `json:"structuredContent"`
}

// text is the one text block a tool result carries next to its structured form.
func (r mcpToolResult) text(t *testing.T) string {
	t.Helper()
	if len(r.Content) != 1 || r.Content[0].Type != "text" {
		t.Fatalf("want exactly one text content block, got %+v", r.Content)
	}
	return r.Content[0].Text
}

// code is the error code of a refused call and "" for a successful one.
func (r mcpToolResult) code(t *testing.T) string {
	t.Helper()
	if !r.IsError {
		return ""
	}
	e, ok := r.Structured["error"].(map[string]any)
	if !ok {
		t.Fatalf("a failed call carries no error object: %v", r.Structured)
	}
	s, _ := e["code"].(string)
	return s
}

// message is the sentence of a refused call.
func (r mcpToolResult) message(t *testing.T) string {
	t.Helper()
	e, ok := r.Structured["error"].(map[string]any)
	if !ok {
		t.Fatalf("a failed call carries no error object: %v", r.Structured)
	}
	s, _ := e["message"].(string)
	return s
}

// mcpCallTool calls one tool through the gate. args is the JSON arguments
// object; an empty string means none.
func mcpCallTool(t *testing.T, h http.Handler, key, tool, args string) mcpToolResult {
	t.Helper()
	if args == "" {
		args = "{}"
	}
	body := fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":%q,"arguments":%s}}`, tool, args)
	w := (mcpReq{key: key, body: body}).do(t, h)
	if w.Code != http.StatusOK {
		t.Fatalf("tools/call %s: status = %d body = %q", tool, w.Code, w.Body.String())
	}
	var env struct {
		Result mcpToolResult `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode tools/call %s response %q: %v", tool, w.Body.String(), err)
	}
	if env.Error != nil {
		t.Fatalf("tools/call %s answered a protocol error: %s", tool, env.Error.Message)
	}
	return env.Result
}

// mcpListTools returns the tools/list entries as decoded JSON objects, which is
// what a client sees rather than what the Go definitions hold.
func mcpListTools(t *testing.T, h http.Handler, key string) []map[string]any {
	t.Helper()
	w := (mcpReq{key: key, body: `{"jsonrpc":"2.0","id":3,"method":"tools/list"}`}).do(t, h)
	if w.Code != http.StatusOK {
		t.Fatalf("tools/list: status = %d body = %q", w.Code, w.Body.String())
	}
	var env struct {
		Result struct {
			Tools []map[string]any `json:"tools"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode tools/list response %q: %v", w.Body.String(), err)
	}
	if env.Error != nil {
		t.Fatalf("tools/list answered a protocol error: %s", env.Error.Message)
	}
	return env.Result.Tools
}

// mcpToolNames is the sorted name list of a tools/list answer.
func mcpToolNames(t *testing.T, tools []map[string]any) []string {
	t.Helper()
	out := make([]string, 0, len(tools))
	for _, tool := range tools {
		name, _ := tool["name"].(string)
		if name == "" {
			t.Fatalf("a tool entry has no name: %v", tool)
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
