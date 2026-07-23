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

// authGatePublicPaths mirrors the allowlist inside authGate: these paths must
// stay reachable both when auth is on without a cookie AND when the settings
// store is erroring (so the SPA can render the login screen and health checks
// keep working).
var authGatePublicPaths = []string{"/api/auth", "/api/login", "/api/health", "/metrics"}

// newAuthGateHandler wires a Handler over a fresh in-memory store and also
// returns the raw *sql.DB so a test can force store errors by closing it.
// Only cfg + store are populated — authGate touches nothing else.
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
	s.AuthPasswordHash = secret.HashPassword(h.cfg.AppKey, "hunter2")
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
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: cookie}) //nolint:gosec // G124: a REQUEST cookie in a test — Secure/HttpOnly are response attributes
	}
	w := httptest.NewRecorder()
	gate.ServeHTTP(w, r)
	return w.Code, nextCalled
}

// Auth OFF (no password hash stored): every request — protected paths included —
// passes straight through to next.
func TestAuthGateOffPassesThrough(t *testing.T) {
	h, _, _ := newAuthGateHandler(t)
	code, called := gateStatus(t, h, "/api/status", "")
	if code != http.StatusOK || !called {
		t.Fatalf("auth off: protected path must pass through, got code=%d called=%v", code, called)
	}
}

// Auth ON without a cookie: protected paths answer 401 and never reach next;
// the public allowlist stays reachable.
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

// Auth ON with cookies: a token minted under the CURRENT epoch passes; garbage
// is rejected; and a previously valid token dies the moment the epoch rotates
// (the logout-all revocation path, end to end through the gate).
func TestAuthGateOnCookieValidation(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	hash := enableAuth(t, h, repo)

	// Fresh install: the epoch is the legacy empty string — a normal epoch value.
	tok := secret.NewSessionToken(h.cfg.AppKey, hash, "", sessionTTL)
	code, called := gateStatus(t, h, "/api/status", tok)
	if code != http.StatusOK || !called {
		t.Fatalf("valid cookie (empty epoch): want 200 reaching next, got code=%d called=%v", code, called)
	}

	code, called = gateStatus(t, h, "/api/status", "garbage.cookie")
	if code != http.StatusUnauthorized || called {
		t.Fatalf("garbage cookie: want 401 without reaching next, got code=%d called=%v", code, called)
	}

	// Rotate the epoch (what POST /api/logout-all does): the old cookie must be
	// revoked, and a token minted under the NEW epoch must pass.
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

// Store error: the gate FAILS CLOSED — protected paths answer 503 (never
// silently dropping the gate), while the public allowlist stays reachable so
// the SPA can still render and recover.
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
