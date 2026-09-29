package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/secret"
)

// gateStatusNamed is gateStatus (authgate_internal_test.go) with the cookie
// name a caller chooses, so a test can prove the gate reads the name this
// instance's own mode issues and no other.
func gateStatusNamed(t *testing.T, h *Handler, path, cookieName, cookie string) (code int, nextCalled bool) {
	t.Helper()
	gate := h.authGate(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	r := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: cookieName, Value: cookie}) //nolint:gosec // G124: request cookie; Secure and HttpOnly only apply to responses
	}
	w := httptest.NewRecorder()
	gate.ServeHTTP(w, r)
	return w.Code, nextCalled
}

// A cookie is scoped to a host, not a port. An instance reachable both over
// plain HTTP and HTTPS on different ports of the same host would otherwise
// have the Secure cookie its HTTPS side issues block the HTTP side from ever
// reading or overwriting it: the browser refuses to let an insecure origin
// touch a Secure cookie of the same name, so a login that succeeds on the
// server looks like a wrong password once the page reloads.
func TestSessionCookieNameDiffersOverPlainHTTP(t *testing.T) {
	h, _, _ := newAuthGateHandler(t)
	h.cfg.HTTPOnly = true
	pw := strings.Repeat("x", secret.MinPasswordLen)

	r := httptest.NewRequest(http.MethodPost, "/api/auth/password", strings.NewReader(`{"password":"`+pw+`"}`))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "10.0.0.1:1"
	w := httptest.NewRecorder()
	h.handleSetPassword(w, r)

	var got []string
	for _, c := range w.Result().Cookies() {
		got = append(got, c.Name)
	}
	if len(got) != 1 || got[0] != sessionCookieNameHTTP {
		t.Fatalf("HTTP-only login issued cookies %v, want exactly %q", got, sessionCookieNameHTTP)
	}
}

func TestSessionCookieNameIsTheHTTPSOneByDefault(t *testing.T) {
	h, _, _ := newAuthGateHandler(t)
	pw := strings.Repeat("x", secret.MinPasswordLen)

	r := httptest.NewRequest(http.MethodPost, "/api/auth/password", strings.NewReader(`{"password":"`+pw+`"}`))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "10.0.0.1:1"
	w := httptest.NewRecorder()
	h.handleSetPassword(w, r)

	var got []string
	for _, c := range w.Result().Cookies() {
		got = append(got, c.Name)
	}
	if len(got) != 1 || got[0] != sessionCookieName {
		t.Fatalf("default (HTTPS) login issued cookies %v, want exactly %q", got, sessionCookieName)
	}
}

// authGate must read the cookie name this instance's own mode issues, and
// must not accept a session under the other mode's name: that would defeat
// the fix, since the two names would again collide in practice whenever a
// deployment answers both http:// and https://.
func TestAuthGateAcceptsOnlyItsOwnModesCookieName(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	h.cfg.HTTPOnly = true
	hash := enableAuth(t, h, repo)
	settings, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	tok := secret.NewSessionToken(h.cfg.AppKey, hash, settings.SessionEpoch, time.Hour)

	if code, called := gateStatusNamed(t, h, "/api/status", sessionCookieNameHTTP, tok); code != http.StatusOK || !called {
		t.Fatalf("a valid session under this mode's own cookie name was refused: code=%d called=%v", code, called)
	}

	// Break the guard: the very same token under the HTTPS name, which an
	// HTTP-only instance never issues, must not authenticate.
	if code, called := gateStatusNamed(t, h, "/api/status", sessionCookieName, tok); code != http.StatusUnauthorized || called {
		t.Fatalf("a session cookie under the HTTPS name authenticated an HTTP-only instance: code=%d called=%v", code, called)
	}
}
