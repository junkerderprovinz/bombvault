package api

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// authGatePublicPaths is the complete allowlist: the login screen and the
// health check have to work without a session and while the store is failing.
// The widget, fleet and passkey handlers check their own token or gate the
// answer inside the handler, because an embedding iframe, a polling peer and
// somebody who is not signed in yet carry no session cookie. /mcp is self-gated
// on its own keys and answers 404 while none exists; the key management routes
// under /api/mcp stay session-protected.
var authGatePublicPaths = []string{
	"/api/auth", "/api/login", "/api/health", "/metrics", "/widget", "/api/widget/data",
	"/api/fleet/status", "/api/fleet/mesh-offer",
	"/api/auth/passkeys", "/api/auth/passkey/login/begin", "/api/auth/passkey/login/finish",
	"/mcp",
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
		cfg:   config.Config{AppKey: strings.Repeat("a", 64)},
		store: repo,
	}
	return h, repo, db
}

// enableAuth stores a password hash (turning the gate on) and returns the hash
// so tests can mint session tokens against it.
func enableAuth(t *testing.T, h *Handler, repo *store.Repo) string {
	t.Helper()
	s, err := repo.GetSettings()
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	pwHash, pwErr := secret.HashPassword(h.cfg.AppKey, "hunter2")
	if pwErr != nil {
		t.Fatalf("HashPassword: %v", pwErr)
	}
	s.AuthPasswordHash = pwHash
	if err := repo.UpdateSettings(s); err != nil {
		t.Fatalf("update settings: %v", err)
	}
	return s.AuthPasswordHash
}

// enableAuthLegacyHash stores the password in the legacy bare-HMAC format and
// returns it. The upgrade-on-sign-in test needs it, and so does the throttle
// flood test, whose ten thousand attempts would not fit in loginWindow with
// Argon2id.
func enableAuthLegacyHash(t *testing.T, h *Handler, repo *store.Repo) string {
	t.Helper()
	s, err := repo.GetSettings()
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	s.AuthPasswordHash = legacyHashForTest(h.cfg.AppKey, "hunter2")
	if err := repo.UpdateSettings(s); err != nil {
		t.Fatalf("update settings: %v", err)
	}
	return s.AuthPasswordHash
}

// setEpoch rotates the stored session epoch (what POST /api/logout-all does).
func setEpoch(t *testing.T, repo *store.Repo, epoch string) {
	t.Helper()
	s, err := repo.GetSettings()
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	s.SessionEpoch = epoch
	if err := repo.UpdateSettings(s); err != nil {
		t.Fatalf("update settings: %v", err)
	}
}

// gateStatus sends one GET through authGate wrapped around a sentinel next
// handler and reports the response code plus whether next was reached.
func gateStatus(t *testing.T, h *Handler, path, cookie string) (code int, nextCalled bool) {
	t.Helper()
	gate := h.authGate(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	r := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: cookie}) //nolint:gosec // G124: request cookie; Secure and HttpOnly only apply to responses
	}
	w := httptest.NewRecorder()
	gate.ServeHTTP(w, r)
	return w.Code, nextCalled
}

// TestAuthGateOffPassesThrough: with no password stored, everything passes.
func TestAuthGateOffPassesThrough(t *testing.T) {
	h, _, _ := newAuthGateHandler(t)
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
	for _, p := range []string{"/api/mcp/keys", "/api/mcp/certificate", "/api/status", "/mcp/x", "/api/widget/token", "/api/fleet/peers"} {
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

// TestAuthGateOnCookieValidation also covers logout-all: rotating the epoch
// revokes every token minted before it.
func TestAuthGateOnCookieValidation(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	hash := enableAuth(t, h, repo)

	// A fresh install has an empty epoch, which is a valid value.
	tok := secret.NewSessionToken(h.cfg.AppKey, hash, "", sessionTTL)
	code, called := gateStatus(t, h, "/api/status", tok)
	if code != http.StatusOK || !called {
		t.Fatalf("valid cookie (empty epoch): want 200 reaching next, got code=%d called=%v", code, called)
	}

	code, called = gateStatus(t, h, "/api/status", "garbage.cookie")
	if code != http.StatusUnauthorized || called {
		t.Fatalf("garbage cookie: want 401 without reaching next, got code=%d called=%v", code, called)
	}

	setEpoch(t, repo, "0123456789abcdef0123456789abcdef")
	code, called = gateStatus(t, h, "/api/status", tok)
	if code != http.StatusUnauthorized || called {
		t.Fatalf("cookie from before epoch rotation: want 401, got code=%d called=%v", code, called)
	}
	tok2 := secret.NewSessionToken(h.cfg.AppKey, hash, "0123456789abcdef0123456789abcdef", sessionTTL)
	code, called = gateStatus(t, h, "/api/status", tok2)
	if code != http.StatusOK || !called {
		t.Fatalf("valid cookie (rotated epoch): want 200 reaching next, got code=%d called=%v", code, called)
	}
}

// TestAuthGateStoreErrorFailsClosed: a failing store closes the gate with 503,
// while the public paths stay open so the SPA can still load.
func TestAuthGateStoreErrorFailsClosed(t *testing.T) {
	h, repo, db := newAuthGateHandler(t)
	enableAuth(t, h, repo)
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	code, called := gateStatus(t, h, "/api/status", "")
	if code != http.StatusServiceUnavailable || called {
		t.Fatalf("store error: protected path must fail closed with 503, got code=%d called=%v", code, called)
	}

	for _, p := range authGatePublicPaths {
		code, called := gateStatus(t, h, p, "")
		if code != http.StatusOK || !called {
			t.Fatalf("store error: public path %s must still pass through, got code=%d called=%v", p, code, called)
		}
	}
}
