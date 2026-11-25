package api

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/parleyport/kit/signin"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// authGatePublicPaths is the complete allowlist: the sign-in and the health
// check have to work without a session. The widget and group handlers check
// their own token or gate the answer inside the handler, because an embedding
// iframe and a group member carry no session cookie. /mcp is self-gated on its
// own keys and answers 404 while none exists; the key management routes under
// /api/mcp stay session-protected. The kit's routes decide who may call them.
var authGatePublicPaths = []string{
	"/api/auth", "/api/health", "/metrics", "/widget", "/api/widget/data",
	"/api/group/call", "/relay/connect", "/mcp",
	"/api/signin/v1/options", "/api/signin/v1/login", "/signin/oidc/callback",
}

// newAuthGateHandler returns a Handler on a fresh in-memory store, plus the
// *sql.DB so a test can break the store by closing it. authGate needs only cfg
// and store.
func newAuthGateHandler(t *testing.T) (*Handler, *store.Repo, *sql.DB) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := store.New(db)
	h := &Handler{
		cfg:      config.Config{AppKey: strings.Repeat("a", 64)},
		store:    repo,
		resetOut: io.Discard,
	}
	return h, repo, db
}

// enableAuth gives h a password the way an install from before the kit kept
// it, opens the sign-in on it and returns the kit's contract behind the gate.
func enableAuth(t *testing.T, h *Handler, repo *store.Repo) http.Handler {
	t.Helper()
	hash, err := secret.HashPassword(h.cfg.AppKey, "hunter2")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if _, err := repo.MutateSettings(func(s *store.Settings) error { s.AuthPasswordHash = hash; return nil }); err != nil {
		t.Fatalf("update settings: %v", err)
	}
	return kitRouter(t, h)
}

// kitRouter opens h's sign-in and returns its contract behind the gate.
func kitRouter(t *testing.T, h *Handler) http.Handler {
	t.Helper()
	if err := h.OpenSignIn(); err != nil {
		t.Fatalf("open the sign-in: %v", err)
	}
	t.Cleanup(func() { _ = h.auth.Close() })
	mux := http.NewServeMux()
	mux.Handle(signin.APIPrefix, h.auth.Handler())
	return h.authGate(mux)
}

// gateStatus sends one GET through authGate wrapped around a sentinel next
// handler and reports the response code plus whether next was reached.
func gateStatus(t *testing.T, h *Handler, path, session string) (code int, nextCalled bool) {
	t.Helper()
	return gateRequest(t, h, path, func(r *http.Request) {
		if session != "" {
			r.AddCookie(sessionCookieFor(session))
		}
	})
}

func gateRequest(t *testing.T, h *Handler, path string, with func(*http.Request)) (code int, nextCalled bool) {
	t.Helper()
	gate := h.authGate(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	r := httptest.NewRequest(http.MethodGet, path, nil)
	with(r)
	w := httptest.NewRecorder()
	gate.ServeHTTP(w, r)
	return w.Code, nextCalled
}

func TestAuthGateOffPassesThrough(t *testing.T) {
	h, _, _ := newAuthGateHandler(t)
	kitRouter(t, h)
	code, called := gateStatus(t, h, "/api/status", "")
	if code != http.StatusOK || !called {
		t.Fatalf("auth off: protected path must pass through, got code=%d called=%v", code, called)
	}
}

func TestAuthGateOnBlocksWithoutCookie(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	enableAuth(t, h, repo)

	code, called := gateStatus(t, h, "/api/status", "")
	if code != http.StatusUnauthorized || called {
		t.Fatalf("auth on, no cookie: want 401 without reaching next, got code=%d called=%v", code, called)
	}

	for _, p := range authGatePublicPaths {
		code, called := gateStatus(t, h, p, "")
		if code != http.StatusOK || !called {
			t.Fatalf("auth on, no cookie: public path %s must pass through, got code=%d called=%v", p, code, called)
		}
	}
}

// TestAuthGatePublicPathListIsExact keeps the mirror above honest: a path added
// to the gate without a line here, or a line here the gate does not honour,
// fails. The key management routes and a sub-path of /mcp are named explicitly
// because both would be a way past the session.
func TestAuthGatePublicPathListIsExact(t *testing.T) {
	public := map[string]bool{}
	for _, p := range authGatePublicPaths {
		public[p] = true
		if !authGatePublicPath(p) {
			t.Errorf("authGatePublicPath(%q) = false, want true", p)
		}
	}
	for _, p := range []string{"/api/mcp/keys", "/api/mcp/certificate", "/api/status", "/mcp/x", "/api/widget/token", "/api/fleet/peers", "/api/signin", "/signin"} {
		if public[p] {
			t.Fatalf("%q is on the mirror; it must not be", p)
		}
		if authGatePublicPath(p) {
			t.Errorf("authGatePublicPath(%q) = true, want the session gate to hold", p)
		}
	}

	h, repo, _ := newAuthGateHandler(t)
	enableAuth(t, h, repo)
	code, called := gateStatus(t, h, "/mcp/x", "")
	if code != http.StatusUnauthorized || called {
		t.Fatalf("/mcp/x with a password set: want 401 without reaching next, got code=%d called=%v", code, called)
	}
}

func TestTheGateLetsInASessionUntilItEnds(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	router := enableAuth(t, h, repo)
	session := signInWith(t, router, "hunter2")
	if code, called := gateStatus(t, h, "/api/status", session); code != http.StatusOK || !called {
		t.Fatalf("signed in: want 200 reaching next, got code=%d called=%v", code, called)
	}
	if code, called := gateStatus(t, h, "/api/status", "garbage"); code != http.StatusUnauthorized || called {
		t.Fatalf("garbage cookie: want 401 without reaching next, got code=%d called=%v", code, called)
	}
	if w := kitRequest(t, router, http.MethodDelete, "me/sessions", session, nil); w.Code != http.StatusOK {
		t.Fatalf("sign out everywhere: %d %s", w.Code, w.Body.String())
	}
	if code, called := gateStatus(t, h, "/api/status", session); code != http.StatusUnauthorized || called {
		t.Fatalf("after signing out everywhere: want 401, got code=%d called=%v", code, called)
	}
}

func TestATokenOfTheKitOpensNoneOfBombVaultsRoutes(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	router := enableAuth(t, h, repo)
	session := signInWith(t, router, "hunter2")
	w := kitRequest(t, router, http.MethodPost, "me/tokens", session, map[string]any{"name": "script", "scopes": []string{"all"}})
	var made struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &made); err != nil || made.Value == "" {
		t.Fatalf("make a token: %d %s", w.Code, w.Body.String())
	}
	code, called := gateRequest(t, h, "/api/settings", func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+made.Value) })
	if code != http.StatusUnauthorized || called {
		t.Fatalf("a kit token on BombVault's settings: want 401, got code=%d called=%v", code, called)
	}
}

// TestAuthGateStoreErrorFailsClosed: a failing store closes the gate, while
// the public paths stay open so the SPA can still load.
func TestAuthGateStoreErrorFailsClosed(t *testing.T) {
	h, repo, db := newAuthGateHandler(t)
	router := enableAuth(t, h, repo)
	session := signInWith(t, router, "hunter2")
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	code, called := gateStatus(t, h, "/api/status", session)
	if code != http.StatusUnauthorized || called {
		t.Fatalf("store error: protected path must fail closed, got code=%d called=%v", code, called)
	}

	for _, p := range authGatePublicPaths {
		code, called := gateStatus(t, h, p, "")
		if code != http.StatusOK || !called {
			t.Fatalf("store error: public path %s must still pass through, got code=%d called=%v", p, code, called)
		}
	}
}
