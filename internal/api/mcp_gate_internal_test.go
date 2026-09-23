package api

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const mcpInternalInitialize = `{"jsonrpc":"2.0","id":1,"method":"initialize",` +
	`"params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"bombvault-tests","version":"1"}}}`

// newMCPGateHandler returns a Handler with a service behind it, its router, and
// the *sql.DB so a test can break the store by closing it.
func newMCPGateHandler(t *testing.T) (*Handler, http.Handler, *store.Repo, *sql.DB) {
	t.Helper()
	h, repo, db := newAuthGateHandler(t)
	dir := t.TempDir()
	h.cfg.DataDir = dir
	h.cfg.HostMountRoot = dir
	h.svc = NewService(h.cfg, repo, nil, nil, nil)
	return h, h.Router(), repo, db
}

// seedMCPKey writes one active key straight into the store and returns its
// secret and id, which is all an internal test needs to get past the gate.
func seedMCPKey(t *testing.T, h *Handler, repo *store.Repo, label string) (key, id string) {
	t.Helper()
	key, err := secret.NewMCPKey()
	if err != nil {
		t.Fatalf("new mcp key: %v", err)
	}
	id = newMCPKeyID()
	_, err = repo.CreateMCPKey(id, label,
		secret.HashMCPKey(h.cfg.AppKey, key), secret.MCPKeyHint(key),
		secret.MCPKeyCheck(h.cfg.AppKey, id), true, time.Now().Unix())
	if err != nil {
		t.Fatalf("create mcp key: %v", err)
	}
	return key, id
}

// mcpInternalPost sends one POST to /mcp with the headers the transport wants.
func mcpInternalPost(h http.Handler, key, body, addr string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, mcpEndpointPath, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	if key != "" {
		r.Header.Set("Authorization", "Bearer "+key)
	}
	if addr != "" {
		r.RemoteAddr = addr
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// A store that cannot be read must never look like a switched-off feature: 404
// would tell a client to stop trying, and passing the request on would drop the
// key check altogether.
func TestMCPStoreErrorFailsClosed(t *testing.T) {
	h, router, repo, db := newMCPGateHandler(t)
	seedMCPKey(t, h, repo, "Laptop")
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	var w *httptest.ResponseRecorder
	out := captureLog(t, func() {
		w = mcpInternalPost(router, "", mcpInternalInitialize, "")
	})
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("closed store: status = %d, want 503", w.Code)
	}
	if !strings.Contains(w.Body.String(), "mcp unavailable") {
		t.Fatalf("closed store: body = %q", w.Body.String())
	}
	if !strings.Contains(out, "api: mcp") {
		t.Fatalf("closed store: log = %q, want an api: mcp line", out)
	}
}

// tailscale serve and a reverse proxy in the same network namespace both reach
// a loopback listener under a name that is not localhost. The SDK's own guard
// would answer 403 there, before any key is looked at.
func TestMCPLoopbackWithForeignHostAccepted(t *testing.T) {
	h, router, repo, _ := newMCPGateHandler(t)
	key, _ := seedMCPKey(t, h, repo, "Laptop")

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	post := func(withKey bool) *http.Response {
		req, err := http.NewRequest(http.MethodPost, srv.URL+mcpEndpointPath, strings.NewReader(mcpInternalInitialize))
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		req.Host = "bombvault.example.ts.net"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		if withKey {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatalf("do request: %v", err)
		}
		t.Cleanup(func() { _ = resp.Body.Close() })
		_, _ = io.Copy(io.Discard, resp.Body)
		return resp
	}

	if resp := post(true); resp.StatusCode != http.StatusOK {
		t.Fatalf("foreign Host with a key: status = %d, want 200", resp.StatusCode)
	}
	if resp := post(false); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("foreign Host without a key: status = %d, want 401", resp.StatusCode)
	}
}

func TestMCPPerKeyCallLimit(t *testing.T) {
	h, router, repo, _ := newMCPGateHandler(t)
	first, _ := seedMCPKey(t, h, repo, "Laptop")
	second, _ := seedMCPKey(t, h, repo, "Desktop")
	h.mcp.calls = newSlidingWindow(time.Minute, 3)

	for i := 0; i < 3; i++ {
		if w := mcpInternalPost(router, first, mcpInternalInitialize, ""); w.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d", i, w.Code)
		}
	}
	w := mcpInternalPost(router, first, mcpInternalInitialize, "")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("over the budget: status = %d, want 429", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("over the budget: no Retry-After header")
	}
	if w := mcpInternalPost(router, second, mcpInternalInitialize, ""); w.Code != http.StatusOK {
		t.Fatalf("another key: status = %d, want its own budget", w.Code)
	}
}

// A tool that outlives its request must not outlive the process either: docker
// stop gives BombVault ten seconds.
func TestMCPToolContextEndsOnShutdown(t *testing.T) {
	h, _, _, _ := newMCPGateHandler(t)

	ctx, cancel := h.mcpToolContext(context.Background(), time.Hour)
	defer cancel()
	h.svc.BeginShutdown()

	select {
	case <-ctx.Done():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("the tool context is still open after BeginShutdown")
	}
}

func TestMCPTouchesLastUsedAtMostOncePerMinute(t *testing.T) {
	h, router, repo, _ := newMCPGateHandler(t)
	key, id := seedMCPKey(t, h, repo, "Laptop")

	base := time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)
	h.mcp.now = func() time.Time { return base }

	if w := mcpInternalPost(router, key, mcpInternalInitialize, "10.0.0.5:1"); w.Code != http.StatusOK {
		t.Fatalf("first request: status = %d", w.Code)
	}
	first, err := repo.GetMCPKey(id)
	if err != nil {
		t.Fatal(err)
	}
	if first.LastUsedAt != base.Unix() || first.LastUsedFrom != "10.0.0.5" {
		t.Fatalf("after the first request: lastUsedAt=%d from=%q", first.LastUsedAt, first.LastUsedFrom)
	}

	h.mcp.now = func() time.Time { return base.Add(30 * time.Second) }
	mcpInternalPost(router, key, mcpInternalInitialize, "10.0.0.5:1")
	again, err := repo.GetMCPKey(id)
	if err != nil {
		t.Fatal(err)
	}
	if again.LastUsedAt != first.LastUsedAt {
		t.Fatalf("second request in the same minute wrote %d, want %d", again.LastUsedAt, first.LastUsedAt)
	}

	h.mcp.now = func() time.Time { return base.Add(61 * time.Second) }
	mcpInternalPost(router, key, mcpInternalInitialize, "10.0.0.6:1")
	later, err := repo.GetMCPKey(id)
	if err != nil {
		t.Fatal(err)
	}
	if later.LastUsedAt != base.Add(61*time.Second).Unix() || later.LastUsedFrom != "10.0.0.6" {
		t.Fatalf("after a minute: lastUsedAt=%d from=%q", later.LastUsedAt, later.LastUsedFrom)
	}
}

// The standard logger is teed into the ring the diagnostics bundle ships, so a
// key or an operator's name for it would travel with every bug report.
func TestMCPLogsNeverContainKeyOrLabel(t *testing.T) {
	h, router, repo, _ := newMCPGateHandler(t)
	key, id := seedMCPKey(t, h, repo, "Karin's laptop")

	out := captureLog(t, func() {
		mcpInternalPost(router, "bvmcp_wrong", mcpInternalInitialize, "10.0.0.4:1")
		mcpInternalPost(router, key, mcpInternalInitialize, "10.0.0.4:1")
		mcpInternalPost(router, key, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_health","arguments":{}}}`, "10.0.0.4:1")
	})

	for _, secretText := range []string{key, "Karin", secret.HashMCPKey(h.cfg.AppKey, key)} {
		if strings.Contains(out, secretText) {
			t.Fatalf("the log carries %q:\n%s", secretText, out)
		}
	}
	if !strings.Contains(out, "10.0.0.4") {
		t.Fatalf("the failed attempt is not logged at all:\n%s", out)
	}
	if strings.Contains(out, id) && !strings.Contains(out, "..."+secret.MCPKeyHint(key)) {
		t.Fatalf("a line names the key id without its hint:\n%s", out)
	}
	want := "key " + id + " ..." + secret.MCPKeyHint(key) + " tool get_health -> ok"
	if !strings.Contains(out, want) {
		t.Fatalf("the call line %q is missing:\n%s", want, out)
	}
}

func TestMCPMetricsCountOutcomes(t *testing.T) {
	h, router, repo, _ := newMCPGateHandler(t)
	settings, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.MetricsEnabled = true
	if err := repo.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	mcpInternalPost(router, "", mcpInternalInitialize, "")
	key, _ := seedMCPKey(t, h, repo, "Laptop")
	mcpInternalPost(router, "", mcpInternalInitialize, "")
	mcpInternalPost(router, key, mcpInternalInitialize, "")
	mcpInternalPost(router, key, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_health","arguments":{}}}`, "")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("metrics: status = %d", w.Code)
	}
	body := w.Body.String()
	for _, line := range []string{
		`bombvault_mcp_requests_total{outcome="not_found"} 1`,
		`bombvault_mcp_requests_total{outcome="no_key"} 1`,
		`bombvault_mcp_requests_total{outcome="ok"} 2`,
		`bombvault_mcp_tool_calls_total{tool="get_health",outcome="ok"} 1`,
		`bombvault_mcp_active_keys 1`,
	} {
		if !strings.Contains(body, line) {
			t.Fatalf("metrics are missing %q:\n%s", line, body)
		}
	}
}

// A bump changes the protocol surface and the SDK's own security defaults, so
// it goes in on its own, after somebody has re-run the interop checks. The
// version is read from go.mod because a test binary carries no dependency list
// of its own, and go.mod is what a dependency-update PR changes.
func TestMCPSDKVersionPinned(t *testing.T) {
	gomod, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	want := "github.com/modelcontextprotocol/go-sdk " + mcpSDKVersion
	for _, line := range strings.Split(string(gomod), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "github.com/modelcontextprotocol/go-sdk ") {
			if line != want {
				t.Fatalf("go.mod requires %q, mcpSDKVersion says %s", line, mcpSDKVersion)
			}
			return
		}
	}
	t.Fatal("go.mod does not require github.com/modelcontextprotocol/go-sdk")
}

// Two route-registration tests build a Handler with nothing in it. The server is
// constructed here, so a registration panic fails in CI rather than at boot.
func TestZeroHandlerRouterBuildsMCP(t *testing.T) {
	h := &Handler{}
	h.Router()
	if h.mcp == nil || h.mcp.http == nil {
		t.Fatal("Router() left the MCP handler unbuilt")
	}
}

func TestMCPGetServerNilWithoutCaller(t *testing.T) {
	h, _, _, _ := newMCPGateHandler(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, mcpEndpointPath, strings.NewReader(mcpInternalInitialize))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	h.mcp.http.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("transport reached without the gate: status = %d, want 400", w.Code)
	}
}
