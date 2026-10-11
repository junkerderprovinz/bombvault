package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/secret"
)

func pairedCookie(in *instance) *http.Cookie {
	hash, epoch, _ := in.h.authEnabled()
	return &http.Cookie{Name: sessionCookieNameHTTP, Value: secret.NewPairedSessionToken(in.appKey, hash, epoch, time.Hour)} //nolint:gosec // G124: a cookie on a test request
}

func TestOddSpellingsOfARefusedRouteStayRefusedToAPairedSession(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	for _, c := range []struct{ method, target string }{
		{"GET", "/api/recovery-kit"},
		{"GET", "/api//recovery-kit"},
		{"GET", "/api/./recovery-kit"},
		{"GET", "/api/x/../recovery-kit"},
		{"GET", "/api/recovery-kit/"},
		{"GET", "/api/recovery%2Dkit"},
		{"GET", "/API/recovery-kit"},
		{"HEAD", "/api/recovery-kit"},
		{"HEAD", "/api/diagnostics"},
		{"GET", "/api/settings/export?credentials=true"},
		{"GET", "/api/settings/"},
		{"OPTIONS", "/api/settings"},
		{"GET", "/api/flash/download"},
		{"GET", "/api/containers/a%2Fb/dbdumps/1/download"},
		{"POST", "/api/emergency/read"},
		{"GET", "/mcp"},
	} {
		req := httptest.NewRequest(c.method, c.target, strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(pairedCookie(in))
		w := httptest.NewRecorder()
		in.router.ServeHTTP(w, req)
		if w.Code < 300 {
			t.Errorf("%s %s answered a paired session %d: %.160s", c.method, c.target, w.Code, w.Body.String())
		}
	}
}

func TestAPairedSessionTeachesNoDirectAddress(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	before, err := in.st.GetGroupState()
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.Host = "attacker.example:8443"
	req.AddCookie(pairedCookie(in))
	w := httptest.NewRecorder()
	in.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	after, err := in.st.GetGroupState()
	if err != nil {
		t.Fatal(err)
	}
	if after.DirectURL != before.DirectURL || strings.Contains(in.svc.selfDirectURL(), "attacker") {
		t.Fatalf("a paired session taught the direct address %q", after.DirectURL)
	}
}

func TestTheGateRefusesAPairedSessionItCannotRouteFirst(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodGet, "/api/recovery-kit", nil)
	req.AddCookie(pairedCookie(in))
	w := httptest.NewRecorder()
	in.h.authGate(wrapped).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), codeOwnLoginNeeded) {
		t.Fatalf("a paired session behind a wrapped handler: %d %s", w.Code, w.Body.String())
	}
}
