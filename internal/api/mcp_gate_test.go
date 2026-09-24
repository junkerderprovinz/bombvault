package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The handshake every gate test sends once it is past the gate, in the legacy
// era Claude Code and mcp-remote negotiate.
const mcpInitializeBody = `{"jsonrpc":"2.0","id":1,"method":"initialize",` +
	`"params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"bombvault-tests","version":"1"}}}`

const mcpAccept = "application/json, text/event-stream"

// mcpReq is one request to /mcp. An empty field means the default a working
// client sends: a POST carrying the two headers the streamable transport
// requires and the key in an Authorization header.
type mcpReq struct {
	method  string
	key     string
	body    string
	host    string
	addr    string
	headers [][2]string
}

// do sends the request through the router and returns the recorded response. A
// header whose value is empty is removed rather than set, which is how a test
// drops one of the defaults.
func (rq mcpReq) do(t *testing.T, h http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	method := rq.method
	if method == "" {
		method = http.MethodPost
	}
	var body io.Reader
	if rq.body != "" {
		body = strings.NewReader(rq.body)
	}
	r := httptest.NewRequest(method, "/mcp", body)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", mcpAccept)
	if rq.key != "" {
		r.Header.Set("Authorization", "Bearer "+rq.key)
	}
	if rq.host != "" {
		r.Host = rq.host
	}
	if rq.addr != "" {
		r.RemoteAddr = rq.addr
	}
	for _, kv := range rq.headers {
		if kv[1] == "" {
			r.Header.Del(kv[0])
			continue
		}
		r.Header.Set(kv[0], kv[1])
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// newGateRouter is a router with a working service behind it and no keys yet.
func newGateRouter(t *testing.T) (http.Handler, *store.Repo) {
	t.Helper()
	h, st, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})
	return h, st
}

// mcpResultName reads serverInfo.name out of a JSON-RPC initialize response.
func mcpResultName(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var env struct {
		Result struct {
			ServerInfo struct {
				Name string `json:"name"`
			} `json:"serverInfo"`
		} `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode initialize response %q: %v", w.Body.String(), err)
	}
	return env.Result.ServerInfo.Name
}

func TestMCPOffWithoutKeys(t *testing.T) {
	h, _ := newGateRouter(t)

	for _, rq := range []mcpReq{
		{method: http.MethodGet},
		{body: mcpInitializeBody},
		{body: mcpInitializeBody, key: "bvmcp_not-a-real-key"},
	} {
		if w := rq.do(t, h); w.Code != http.StatusNotFound {
			t.Fatalf("%s without keys: status = %d, want 404", rq.method+" /mcp", w.Code)
		}
	}

	key, id := createMCPKey(t, h, "Laptop", true)
	if w := (mcpReq{key: key, body: mcpInitializeBody}).do(t, h); w.Code != http.StatusOK {
		t.Fatalf("with a key: status = %d body=%q", w.Code, w.Body.String())
	}

	if w, m := doMCPKey(t, h, http.MethodPost, "/api/mcp/keys/"+id+"/revoke", ""); w.Code != http.StatusOK {
		t.Fatalf("revoke: status=%d body=%v", w.Code, m)
	}
	if w := (mcpReq{key: key, body: mcpInitializeBody}).do(t, h); w.Code != http.StatusNotFound {
		t.Fatalf("after revoking the only key: status = %d, want 404", w.Code)
	}
}

func TestMCPRequiresKey(t *testing.T) {
	h, _ := newGateRouter(t)
	key, _ := createMCPKey(t, h, "Laptop", true)

	w := (mcpReq{body: mcpInitializeBody}).do(t, h)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("no key: status = %d, want 401", w.Code)
	}
	if got := w.Header().Get("WWW-Authenticate"); got != `Bearer realm="bombvault-mcp"` {
		t.Fatalf("no key: WWW-Authenticate = %q", got)
	}

	w = (mcpReq{key: "bvmcp_wrong", body: mcpInitializeBody}).do(t, h)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong key: status = %d, want 401", w.Code)
	}
	if got := w.Header().Get("WWW-Authenticate"); !strings.Contains(got, `error="invalid_token"`) {
		t.Fatalf("wrong key: WWW-Authenticate = %q, want an invalid_token hint", got)
	}

	w = (mcpReq{key: key, body: mcpInitializeBody}).do(t, h)
	if w.Code != http.StatusOK {
		t.Fatalf("right key: status = %d body=%q", w.Code, w.Body.String())
	}
	if name := mcpResultName(t, w); name != "bombvault" {
		t.Fatalf("serverInfo.name = %q, want bombvault", name)
	}
}

// mcp-remote passes the key in X-API-Key because its argument handling breaks a
// header value with a space in it on some platforms.
func TestMCPAcceptsBearerAnyCaseAndXAPIKey(t *testing.T) {
	h, _ := newGateRouter(t)
	key, _ := createMCPKey(t, h, "Laptop", true)

	for _, hdr := range [][2]string{
		{"Authorization", "bearer " + key},
		{"Authorization", "BEARER  " + key + " "},
		{"X-API-Key", key},
	} {
		rq := mcpReq{body: mcpInitializeBody, headers: [][2]string{hdr}}
		if hdr[0] == "X-API-Key" {
			rq.headers = append(rq.headers, [2]string{"Authorization", ""})
		}
		if w := rq.do(t, h); w.Code != http.StatusOK {
			t.Fatalf("%s: %q: status = %d body=%q", hdr[0], hdr[1], w.Code, w.Body.String())
		}
	}
}

func TestMCPConflictingKeyHeadersRejected(t *testing.T) {
	h, _ := newGateRouter(t)
	first, _ := createMCPKey(t, h, "Laptop", true)
	second, _ := createMCPKey(t, h, "Desktop", true)

	w := (mcpReq{key: first, body: mcpInitializeBody, headers: [][2]string{{"X-API-Key", second}}}).do(t, h)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("two different keys: status = %d, want 401", w.Code)
	}
}

func TestMCPRevokeAndRotateTakeEffectImmediately(t *testing.T) {
	h, _ := newGateRouter(t)
	laptop, laptopID := createMCPKey(t, h, "Laptop", true)
	desktop, desktopID := createMCPKey(t, h, "Desktop", true)

	if w := (mcpReq{key: laptop, body: mcpInitializeBody}).do(t, h); w.Code != http.StatusOK {
		t.Fatalf("before the revoke: status = %d", w.Code)
	}
	if w, m := doMCPKey(t, h, http.MethodPost, "/api/mcp/keys/"+laptopID+"/revoke", ""); w.Code != http.StatusOK {
		t.Fatalf("revoke: status=%d body=%v", w.Code, m)
	}
	if w := (mcpReq{key: laptop, body: mcpInitializeBody}).do(t, h); w.Code != http.StatusUnauthorized {
		t.Fatalf("after the revoke: status = %d, want 401", w.Code)
	}

	w, m := doMCPKey(t, h, http.MethodPost, "/api/mcp/keys/"+desktopID+"/rotate", "")
	if w.Code != http.StatusOK {
		t.Fatalf("rotate: status=%d body=%v", w.Code, m)
	}
	rotated, _ := m["key"].(string)
	if rotated == "" || rotated == desktop {
		t.Fatalf("rotate returned %q, want a fresh secret", rotated)
	}
	if w := (mcpReq{key: desktop, body: mcpInitializeBody}).do(t, h); w.Code != http.StatusUnauthorized {
		t.Fatalf("old secret after the rotation: status = %d, want 401", w.Code)
	}
	if w := (mcpReq{key: rotated, body: mcpInitializeBody}).do(t, h); w.Code != http.StatusOK {
		t.Fatalf("rotated secret: status = %d body=%q", w.Code, w.Body.String())
	}
}

// A browser session is not an MCP credential: the endpoint is allow-listed in
// authGate and decides on its own keys alone.
func TestMCPSessionCookieIsNotACredential(t *testing.T) {
	h, st := newGateRouter(t)
	createMCPKey(t, h, "Laptop", true)
	cookie := enableLogin(t, st, strings.Repeat("a", 64))

	w := (mcpReq{body: mcpInitializeBody, headers: [][2]string{{"Cookie", "bv_session=" + cookie}}}).do(t, h)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("session cookie, no key: status = %d, want 401", w.Code)
	}
}

func TestMCPWorksWithLoginPasswordAndNoCookie(t *testing.T) {
	h, st := newGateRouter(t)
	key, _ := createMCPKey(t, h, "Laptop", true)
	enableLogin(t, st, strings.Repeat("a", 64))

	if w := (mcpReq{key: key, body: mcpInitializeBody}).do(t, h); w.Code != http.StatusOK {
		t.Fatalf("key without a session: status = %d body=%q", w.Code, w.Body.String())
	}
}

func TestMCPTrustedLANModeStillNeedsKey(t *testing.T) {
	h, _ := newGateRouter(t)
	createMCPKey(t, h, "Laptop", true)

	if w := (mcpReq{body: mcpInitializeBody}).do(t, h); w.Code != http.StatusUnauthorized {
		t.Fatalf("no password, no key header: status = %d, want 401", w.Code)
	}
}

func TestMCPCrossSitePostRefused(t *testing.T) {
	h, _ := newGateRouter(t)
	key, _ := createMCPKey(t, h, "Laptop", true)

	w := (mcpReq{key: key, body: mcpInitializeBody, headers: [][2]string{{"Sec-Fetch-Site", "cross-site"}}}).do(t, h)
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-site POST: status = %d, want csrfGate's 403", w.Code)
	}
	if w := (mcpReq{key: key, body: mcpInitializeBody}).do(t, h); w.Code != http.StatusOK {
		t.Fatalf("same request without the header: status = %d body=%q", w.Code, w.Body.String())
	}
}

func TestMCPForeignOriginRefusedSameOriginAllowed(t *testing.T) {
	h, _ := newGateRouter(t)
	key, _ := createMCPKey(t, h, "Laptop", true)

	for _, origin := range []string{"http://evil.example", "null", "::not a url"} {
		w := (mcpReq{key: key, body: mcpInitializeBody, host: "tower", headers: [][2]string{{"Origin", origin}}}).do(t, h)
		if w.Code != http.StatusForbidden {
			t.Fatalf("Origin %q: status = %d, want 403", origin, w.Code)
		}
		if !strings.Contains(w.Body.String(), `"id":null`) {
			t.Fatalf("Origin %q: body = %q, want a JSON-RPC error carrying a null id", origin, w.Body.String())
		}
		var env struct {
			JSONRPC string `json:"jsonrpc"`
			ID      *int   `json:"id"`
			Error   struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		dec := json.NewDecoder(strings.NewReader(w.Body.String()))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&env); err != nil {
			t.Fatalf("Origin %q: %v in %q", origin, err, w.Body.String())
		}
		if env.JSONRPC != "2.0" || env.ID != nil || env.Error.Code != -32600 {
			t.Fatalf("Origin %q: envelope = %+v", origin, env)
		}
	}

	w := (mcpReq{key: key, body: mcpInitializeBody, host: "tower", headers: [][2]string{{"Origin", "http://tower"}}}).do(t, h)
	if w.Code != http.StatusOK {
		t.Fatalf("same-origin: status = %d body=%q", w.Code, w.Body.String())
	}
}

// A DNS-rebinding page sends its own name in both Origin and Host, so it passes
// the Origin check. The key is what it cannot produce.
func TestMCPRebindingPageStillNeedsKey(t *testing.T) {
	h, _ := newGateRouter(t)
	createMCPKey(t, h, "Laptop", true)

	rebind := [][2]string{{"Origin", "http://rebind.example"}}
	for _, key := range []string{"", "bvmcp_wrong"} {
		w := (mcpReq{key: key, body: mcpInitializeBody, host: "rebind.example", headers: rebind}).do(t, h)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("rebinding page with key %q: status = %d, want 401", key, w.Code)
		}
	}
}

func TestMCPFailedAttemptsThrottledPerAddress(t *testing.T) {
	h, st := newGateRouter(t)
	key, _ := createMCPKey(t, h, "Laptop", true)
	enableLogin(t, st, strings.Repeat("a", 64))

	const guesser = "10.0.0.9:1"
	// A request without any key header is not an attempt, so it must not count.
	for i := 0; i < 3; i++ {
		(mcpReq{body: mcpInitializeBody, addr: guesser}).do(t, h)
	}
	for i := 0; i < 5; i++ {
		w := (mcpReq{key: "bvmcp_wrong", body: mcpInitializeBody, addr: guesser}).do(t, h)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("wrong key %d: status = %d, want 401", i, w.Code)
		}
	}

	w := (mcpReq{key: key, body: mcpInitializeBody, addr: guesser}).do(t, h)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("right key after five failures: status = %d, want 429", w.Code)
	}
	if got := w.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("Retry-After = %q, want 60", got)
	}

	if w := (mcpReq{key: key, body: mcpInitializeBody, addr: "10.0.0.8:1"}).do(t, h); w.Code != http.StatusOK {
		t.Fatalf("another address: status = %d, want 200", w.Code)
	}

	login := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"password":"hunter2"}`))
	login.Header.Set("Content-Type", "application/json")
	login.RemoteAddr = guesser
	lw := httptest.NewRecorder()
	h.ServeHTTP(lw, login)
	if lw.Code == http.StatusTooManyRequests {
		t.Fatalf("the login form shares the MCP failure bucket: status = %d", lw.Code)
	}
}

// Behind a shared proxy address a polling assistant would otherwise reset a
// guesser's count with every call, and the throttle would never engage.
func TestMCPSuccessDoesNotResetFailuresOnSharedAddress(t *testing.T) {
	h, _ := newGateRouter(t)
	key, _ := createMCPKey(t, h, "Laptop", true)

	const shared = "10.0.0.7:1"
	for i := 0; i < 4; i++ {
		(mcpReq{key: "bvmcp_wrong", body: mcpInitializeBody, addr: shared}).do(t, h)
	}
	if w := (mcpReq{key: key, body: mcpInitializeBody, addr: shared}).do(t, h); w.Code != http.StatusOK {
		t.Fatalf("valid request between failures: status = %d", w.Code)
	}
	(mcpReq{key: "bvmcp_wrong", body: mcpInitializeBody, addr: shared}).do(t, h)

	if w := (mcpReq{key: key, body: mcpInitializeBody, addr: shared}).do(t, h); w.Code != http.StatusTooManyRequests {
		t.Fatalf("after five failures around a success: status = %d, want 429", w.Code)
	}
}

func TestMCPBodyLimit(t *testing.T) {
	h, _ := newGateRouter(t)
	key, _ := createMCPKey(t, h, "Laptop", true)

	big := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"pad":"` +
		strings.Repeat("a", 1<<20) + `"}}`
	if w := (mcpReq{key: key, body: big}).do(t, h); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized body: status = %d, want 413", w.Code)
	}
}

func TestMCPTransportMethodAndHeaderRules(t *testing.T) {
	h, _ := newGateRouter(t)
	key, _ := createMCPKey(t, h, "Laptop", true)

	w := (mcpReq{method: http.MethodGet, key: key, headers: [][2]string{{"Accept", "text/event-stream"}}}).do(t, h)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET: status = %d, want 405", w.Code)
	}
	if got := w.Header().Get("Allow"); got != http.MethodPost {
		t.Fatalf("GET: Allow = %q, want POST", got)
	}
	if w := (mcpReq{method: http.MethodDelete, key: key}).do(t, h); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("DELETE: status = %d, want 405", w.Code)
	}

	w = (mcpReq{key: key, body: mcpInitializeBody, headers: [][2]string{{"Accept", "application/json"}}}).do(t, h)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("POST without the event-stream Accept: status = %d, want 400", w.Code)
	}
	w = (mcpReq{key: key, body: mcpInitializeBody, headers: [][2]string{{"Content-Type", "text/plain"}}}).do(t, h)
	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("POST as text/plain: status = %d, want 415", w.Code)
	}

	w = (mcpReq{key: key, body: `{"jsonrpc":"2.0","method":"notifications/initialized"}`}).do(t, h)
	if w.Code != http.StatusAccepted {
		t.Fatalf("notification: status = %d, want 202", w.Code)
	}
}

// One POST carrying hundreds of calls would count as one request against the
// per-key budget, so the shape is refused before the transport unpacks it.
func TestMCPBatchBodyRefused(t *testing.T) {
	h, _ := newGateRouter(t)
	key, _ := createMCPKey(t, h, "Laptop", true)

	w := (mcpReq{key: key, body: "  \n\t" + `[{"jsonrpc":"2.0","id":1,"method":"tools/list"}]`}).do(t, h)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("batch: status = %d, want 400", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, `"id":null`) ||
		!strings.Contains(body, "batch requests are not accepted") {
		t.Fatalf("batch: body = %q", body)
	}

	if w := (mcpReq{key: key, body: mcpInitializeBody}).do(t, h); w.Code != http.StatusOK {
		t.Fatalf("single message after a batch: status = %d body=%q", w.Code, w.Body.String())
	}

	big := `[{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"pad":"` +
		strings.Repeat("a", 1<<20) + `"}}]`
	if w := (mcpReq{key: key, body: big}).do(t, h); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized batch: status = %d, want 413", w.Code)
	}
}
