package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// docsRouteRe finds a route written as `GET /api/v1/...` in a docs page.
var docsRouteRe = regexp.MustCompile("`(GET|POST|PUT|PATCH|DELETE) (/api/v1/[^`\\s]+)`")

func apiV1RouteSet() map[string]string {
	out := map[string]string{}
	for _, r := range apiV1Routes() {
		scope := "read"
		if r.start {
			scope = "start"
		}
		out[r.method+" "+r.path] = scope
	}
	return out
}

func TestAPIV1RoutesMatchOpenAPIAndDocs(t *testing.T) {
	routes := apiV1RouteSet()

	var doc struct {
		Paths map[string]map[string]struct {
			Scope    string `json:"x-bombvault-scope"`
			Security []any  `json:"security"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(apiV1OpenAPI, &doc); err != nil {
		t.Fatalf("openapi.json does not parse: %v", err)
	}
	described := map[string]string{}
	for path, ops := range doc.Paths {
		for method, op := range ops {
			described[strings.ToUpper(method)+" "+path] = op.Scope
		}
	}
	if described["GET "+apiV1OpenAPIPath] != "open" {
		t.Fatalf("openapi.json does not describe itself as open: %v", described)
	}
	delete(described, "GET "+apiV1OpenAPIPath)
	for route, scope := range routes {
		got, ok := described[route]
		if !ok {
			t.Errorf("%s is routed but not in openapi.json", route)
			continue
		}
		if got != scope {
			t.Errorf("%s needs a %s token but openapi.json says %q", route, scope, got)
		}
	}
	for route := range described {
		if _, ok := routes[route]; !ok {
			t.Errorf("openapi.json describes %s, which is not routed", route)
		}
	}

	pages, err := filepath.Glob(filepath.Join("..", "..", "docs", "api*.md"))
	if err != nil || len(pages) == 0 {
		t.Fatalf("no docs/api pages found: %v", err)
	}
	for _, page := range pages {
		body, err := os.ReadFile(page) //nolint:gosec // G304: a docs page the glob above found in the repository
		if err != nil {
			t.Fatal(err)
		}
		documented := map[string]bool{}
		for _, m := range docsRouteRe.FindAllStringSubmatch(string(body), -1) {
			documented[m[1]+" "+m[2]] = true
		}
		for route := range routes {
			if !documented[route] {
				t.Errorf("%s does not document %s", filepath.Base(page), route)
			}
		}
		for route := range documented {
			if _, ok := routes[route]; !ok {
				t.Errorf("%s documents %s, which is not routed", filepath.Base(page), route)
			}
		}
	}
}

// seedAPIToken writes one active token straight into the store.
func seedAPIToken(t *testing.T, h *Handler, repo *store.Repo, label string, canStart bool) (token, id string) {
	t.Helper()
	token, err := secret.NewAPIToken()
	if err != nil {
		t.Fatal(err)
	}
	id = newMCPKeyID()
	if _, err := repo.CreateAPIToken(id, label,
		secret.HashMCPKey(h.cfg.AppKey, token), secret.MCPKeyHint(token),
		secret.MCPKeyCheck(h.cfg.AppKey, id), canStart, time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	return token, id
}

func apiV1Do(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestEveryAPIV1RouteDemandsAToken(t *testing.T) {
	h, router, repo, _ := newMCPGateHandler(t)
	mcpKey, _ := seedMCPKey(t, h, repo, "Assistant")
	for _, route := range apiV1Routes() {
		path := strings.NewReplacer("{id}", "0123456789abcdef0123456789abcdef", "{domain}", "containers").Replace(route.path)
		for _, token := range []string{"", mcpKey, secret.APITokenPrefix + "not-a-real-token"} {
			w := apiV1Do(router, route.method, path, token, "")
			if w.Code != http.StatusUnauthorized {
				t.Errorf("%s %s with %q answered %d, want 401", route.method, path, token, w.Code)
			}
		}
		// Three bad tokens per route would soon trip the address throttle,
		// which answers 429 before any token is read.
		h.loginMu.Lock()
		h.loginFails = map[string][]time.Time{}
		h.loginMu.Unlock()
	}
}

func TestAPIV1OpenAPIIsServedWithoutAToken(t *testing.T) {
	h, router, repo, _ := newMCPGateHandler(t)
	enableAuth(t, h, repo)
	w := apiV1Do(router, http.MethodGet, apiV1OpenAPIPath, "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s answered %d, want 200", apiV1OpenAPIPath, w.Code)
	}
	var doc map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil || doc["openapi"] != "3.1.0" {
		t.Fatalf("the description is not OpenAPI 3.1: %v %v", err, doc["openapi"])
	}
}

func TestAPIV1TokenReadsWithAndWithoutALoginPassword(t *testing.T) {
	h, router, repo, _ := newMCPGateHandler(t)
	token, id := seedAPIToken(t, h, repo, "Dashboard", false)
	for _, login := range []bool{false, true} {
		if login {
			enableAuth(t, h, repo)
		}
		w := apiV1Do(router, http.MethodGet, "/api/v1/health", token, "")
		if w.Code != http.StatusOK {
			t.Fatalf("login %v: health answered %d: %s", login, w.Code, w.Body)
		}
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		key, _ := body["key"].(map[string]any)
		if key["label"] != "Dashboard" || key["canStartBackups"] != false {
			t.Fatalf("login %v: health names the token as %v", login, key)
		}
	}
	events, err := repo.MCPKeyEvents(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Tool != "get_health" {
		t.Fatalf("the token's log holds %+v, want the two health calls", events)
	}
}

func TestAPIV1ReadOnlyTokenCannotStartABackup(t *testing.T) {
	h, router, repo, _ := newMCPGateHandler(t)
	token, _ := seedAPIToken(t, h, repo, "Dashboard", false)
	for _, c := range []struct{ path, body string }{
		{"/api/v1/backups", `{"domain":"containers"}`},
		{"/api/v1/backups", `{"domain":"containers","item":"plex"}`},
		{"/api/v1/backups/everything", ""},
		{"/api/v1/runs/0123456789abcdef0123456789abcdef/cancel", ""},
	} {
		w := apiV1Do(router, http.MethodPost, c.path, token, c.body)
		if w.Code != http.StatusForbidden {
			t.Fatalf("POST %s answered %d, want 403: %s", c.path, w.Code, w.Body)
		}
		var body struct {
			Error struct{ Message string } `json:"error"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || !strings.Contains(body.Error.Message, "Settings > Integrations > API tokens") {
			t.Fatalf("POST %s does not say where to allow it: %s", c.path, w.Body)
		}
	}
}

func TestAPIV1RefusesABadArgumentWithoutReachingTheTool(t *testing.T) {
	h, router, repo, _ := newMCPGateHandler(t)
	token, _ := seedAPIToken(t, h, repo, "Dashboard", true)
	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/runs?limit=many", ""},
		{http.MethodGet, "/api/v1/runs?colour=red", ""},
		{http.MethodGet, "/api/v1/items?domain=printers", ""},
		{http.MethodPost, "/api/v1/backups", `not json`},
		{http.MethodPost, "/api/v1/backups", `{"domain":"containers","when":"now"}`},
	} {
		w := apiV1Do(router, c.method, c.path, token, c.body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s %s answered %d, want 400: %s", c.method, c.path, w.Code, w.Body)
		}
		var body struct {
			Error struct{ Code string } `json:"error"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Error.Code != "invalid_argument" {
			t.Fatalf("%s %s answered %s, want an invalid_argument error", c.method, c.path, w.Body)
		}
	}
}

func TestAPIV1StatusFollowsTheToolErrorCode(t *testing.T) {
	codes := map[string]int{
		"invalid_argument": http.StatusBadRequest,
		"not_permitted":    http.StatusForbidden,
		"not_found":        http.StatusNotFound,
		"busy":             http.StatusConflict,
		"cooldown":         http.StatusTooManyRequests,
		"retention_guard":  http.StatusTooManyRequests,
		"unavailable":      http.StatusServiceUnavailable,
		"timeout":          http.StatusGatewayTimeout,
		"failed":           http.StatusInternalServerError,
	}
	for code, want := range codes {
		if got := apiV1Status(code); got != want {
			t.Errorf("%s maps to %d, want %d", code, got, want)
		}
	}
	w := httptest.NewRecorder()
	writeAPIV1Result(w, mcpToolError("cooldown", "wait", map[string]any{"retryAfterSeconds": 90}))
	if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") != "90" {
		t.Fatalf("a cooldown answered %d with Retry-After %q, want 429 and 90", w.Code, w.Header().Get("Retry-After"))
	}
}

func TestAPITokenListLeavesTheMCPKeysOut(t *testing.T) {
	h, router, repo, _ := newMCPGateHandler(t)
	seedMCPKey(t, h, repo, "Assistant")
	seedAPIToken(t, h, repo, "Dashboard", false)

	list := func(path, field string) []string {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Host = "tower"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		rows, _ := body[field].([]any)
		var labels []string
		for _, row := range rows {
			m, _ := row.(map[string]any)
			labels = append(labels, m["label"].(string))
		}
		return labels
	}
	if got := list("/api/tokens", "tokens"); !slices.Equal(got, []string{"Dashboard"}) {
		t.Fatalf("the token card lists %v, want only the token", got)
	}
	if got := list("/api/mcp/keys", "keys"); !slices.Equal(got, []string{"Assistant"}) {
		t.Fatalf("the MCP card lists %v, want only the key", got)
	}
}

// A page on another origin can make a browser send a token it holds, so the
// API refuses an Origin that is not its own, as the MCP endpoint does.
func TestAPIV1RefusesARequestFromAnotherOrigin(t *testing.T) {
	h, router, repo, _ := newMCPGateHandler(t)
	token, _ := seedAPIToken(t, h, repo, "Dashboard", false)
	for origin, want := range map[string]int{
		"":                    http.StatusOK,
		"http://example.com":  http.StatusOK,
		"http://evil.example": http.StatusForbidden,
		"null":                http.StatusForbidden,
	} {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		r.Host = "example.com"
		r.Header.Set("Authorization", "Bearer "+token)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != want {
			t.Errorf("Origin %q answered %d, want %d: %s", origin, w.Code, want, w.Body)
		}
		if want == http.StatusForbidden && !strings.Contains(w.Body.String(), `"forbidden_origin"`) {
			t.Errorf("Origin %q: body %s, want the code forbidden_origin", origin, w.Body)
		}
	}
}
