package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/signin"

	"github.com/junkerderprovinz/bombvault/internal/secret"
)

// kitCookie is the name of the kit's session cookie over plain http, which
// every test request is.
const kitCookie = "bombvault_session"

// phoneSigning is what a paired phone's session is signed with, and whether
// anybody can sign in.
func (h *Handler) phoneSigning() (hash, epoch string, on bool) {
	s, err := h.store.GetSettings()
	if err != nil {
		panic(err)
	}
	return "", s.SessionEpoch, !h.signInOpen()
}

// kitRequest sends router a request of the kit's contract the way the page
// does, with the session cookie when there is one.
func kitRequest(t *testing.T, router http.Handler, method, path, session string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rd *strings.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rd = strings.NewReader(string(b))
	} else {
		rd = strings.NewReader("")
	}
	r := httptest.NewRequest(method, signin.APIPrefix+path, rd)
	r.Host = "localhost"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	if session != "" {
		r.AddCookie(&http.Cookie{Name: kitCookie, Value: session}) //nolint:gosec // G124: a cookie on a test request
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w
}

// sessionFrom returns the kit's session cookie w set.
func sessionFrom(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == kitCookie && c.Value != "" {
			return c.Value
		}
	}
	t.Fatalf("no session: %d %s", w.Code, w.Body.String())
	return ""
}

// signInWith signs in to router with password and returns the session.
func signInWith(t *testing.T, router http.Handler, password string) string {
	t.Helper()
	return sessionFrom(t, kitRequest(t, router, http.MethodPost, "login", "", map[string]any{"password": password}))
}

// setPasswordOn gives an open instance its first password and returns the
// session that comes with it.
func setPasswordOn(t *testing.T, router http.Handler, password string) string {
	t.Helper()
	return sessionFrom(t, kitRequest(t, router, http.MethodPut, "me/password", "", map[string]any{"next": password}))
}

// removePasswordOn opens an instance again, signed in with session.
func removePasswordOn(t *testing.T, router http.Handler, session, password string) {
	t.Helper()
	if w := kitRequest(t, router, http.MethodDelete, "me/password", session, map[string]any{"current": password}); w.Code != http.StatusOK {
		t.Fatalf("remove the password: %d %s", w.Code, w.Body.String())
	}
}

// sessionCookieFor is the cookie a request carries for session: the kit's,
// or BombVault's own for a paired phone.
func sessionCookieFor(session string) *http.Cookie {
	if secret.PairedSessionToken(session) {
		return &http.Cookie{Name: sessionCookieNameHTTP, Value: session} //nolint:gosec // G124: a cookie on a test request
	}
	return &http.Cookie{Name: kitCookie, Value: session} //nolint:gosec // G124: a cookie on a test request
}

// resetOn resets the password of in with the code the reset writes to the
// log, the way somebody who lost it does, and returns the session it signs in.
func resetOn(t *testing.T, in *instance, password string) string {
	t.Helper()
	out := &strings.Builder{}
	in.h.resetOut = out
	if w := kitRequest(t, in.router, http.MethodPost, "reset-code", "", nil); w.Code != http.StatusOK {
		t.Fatalf("ask for a reset code: %d %s", w.Code, w.Body.String())
	}
	_, rest, ok := strings.Cut(out.String(), "PASSWORD RESET CODE: ")
	if !ok || len(rest) < 14 {
		t.Fatalf("no reset code in the log: %q", out.String())
	}
	w := kitRequest(t, in.router, http.MethodPost, "reset", "", map[string]any{"code": rest[:14], "password": password})
	return sessionFrom(t, w)
}

// sessionOf is the header of a browser signed in to h, for a handler called
// without the gate in front. Such a handler sees no visitor, and one built
// without a sign-in counts as open, so the header carries nothing.
func sessionOf(*Handler) http.Header { return http.Header{} }

// staleSession is a session of in from a sign-in days ago: the one the
// browser cookie of the version before the kit is swapped for.
func staleSession(t *testing.T, in *instance) string {
	t.Helper()
	s, err := in.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	old := secret.NewSessionToken(in.appKey, s.AuthPasswordHash, s.SessionEpoch, time.Hour)
	r := httptest.NewRequest(http.MethodGet, "/api/auth", nil)
	r.Host = "localhost"
	r.AddCookie(&http.Cookie{Name: sessionCookieNameHTTP, Value: old}) //nolint:gosec // G124: a cookie on a test request
	w := httptest.NewRecorder()
	in.router.ServeHTTP(w, r)
	return sessionFrom(t, w)
}
