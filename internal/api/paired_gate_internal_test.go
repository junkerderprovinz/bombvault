package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/group"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// listedPhoneSession is the session of a phone the instance lists, the only
// kind of paired session the gate lets through.
func listedPhoneSession(t *testing.T, st *store.Repo, appKey, hash, epoch string) string {
	t.Helper()
	rows, err := st.ListFleetPeers()
	if err != nil {
		t.Fatal(err)
	}
	if !listsPhone(rows, wordsHolderID) {
		if _, err := st.CreateFleetPeer(store.FleetPeer{MemberID: wordsHolderID, Name: "phone", Kind: "android", Enabled: true}); err != nil {
			t.Fatal(err)
		}
	}
	return secret.NewPairedSessionToken(appKey, hash, epoch, wordsHolderID, time.Hour)
}

func pairedCookie(t *testing.T, in *instance) *http.Cookie {
	hash, epoch, _ := in.h.authEnabled()
	return &http.Cookie{Name: sessionCookieNameHTTP, Value: listedPhoneSession(t, in.st, in.appKey, hash, epoch)} //nolint:gosec // G124: a cookie on a test request
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
		req.AddCookie(pairedCookie(t, in))
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
	req.AddCookie(pairedCookie(t, in))
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
	req.AddCookie(pairedCookie(t, in))
	w := httptest.NewRecorder()
	in.h.authGate(wrapped).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), codeOwnLoginNeeded) {
		t.Fatalf("a paired session behind a wrapped handler: %d %s", w.Code, w.Body.String())
	}
}

// A lost phone keeps the group's words and is back in the list as soon as
// the app opens, so removing its card keeps it switched off from then on.
func TestARemovedPhoneComesBackSwitchedOff(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	phone := []group.Member{{ID: wordsHolderID, Name: "phone", Kind: "android"}}
	rows, err := in.svc.syncFleetPeersWith(phone, time.Now())
	if err != nil || !listsPhone(rows, wordsHolderID) {
		t.Fatalf("setup: rows %+v, %v; want the phone switched on", rows, err)
	}
	if code, out := in.do(t, http.MethodDelete, "/api/fleet/peers/"+rows[0].ID, nil); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("remove: %d %v", code, out)
	}

	rows, err = in.svc.syncFleetPeersWith(phone, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Enabled || listsPhone(rows, wordsHolderID) {
		t.Fatalf("rows once the phone is back = %+v, want it switched off", rows)
	}
	hash, epoch, _ := in.h.authEnabled()
	if in.h.pairedPhoneListed(secret.NewPairedSessionToken(in.appKey, hash, epoch, wordsHolderID, time.Hour)) {
		t.Fatal("the removed phone's session still counts")
	}

	instance := []group.Member{{ID: "fedcba9876543210fedcba9876543210", Name: "cellar"}}
	if rows, err := in.svc.syncFleetPeersWith(instance, time.Now()); err != nil || !rows[len(rows)-1].Enabled {
		t.Fatalf("an instance came back switched off: %+v, %v", rows, err)
	}
}

func TestAuthStatusTellsAPairedSessionApart(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	for _, c := range []struct {
		cookie *http.Cookie
		want   bool
	}{{pairedCookie(t, in), true}, {nil, false}} {
		req := httptest.NewRequest(http.MethodGet, "/api/auth", nil)
		if c.cookie != nil {
			req.AddCookie(c.cookie)
		}
		w := httptest.NewRecorder()
		in.router.ServeHTTP(w, req)
		if got := strings.Contains(w.Body.String(), `"paired":true`); got != c.want {
			t.Errorf("cookie %v: paired = %v, want %v (%s)", c.cookie != nil, got, c.want, w.Body.String())
		}
	}
}
