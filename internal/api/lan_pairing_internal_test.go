package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestDirectAddressFromRequestIgnoresLocalAddresses(t *testing.T) {
	for _, host := range []string{"localhost", "localhost:3443", "127.0.0.1:3443", "[::1]:3443", "169.254.1.5:3443", ""} {
		r := httptest.NewRequest(http.MethodGet, "/api/group", nil)
		r.Host = host
		if got := directAddressFromRequest(false, r); got != "" {
			t.Errorf("directAddressFromRequest(%q) = %q, want empty", host, got)
		}
	}
}

func TestDirectAddressFromRequestUsesTheHostHeaderAndScheme(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/group", nil)
	r.Host = "192.168.1.20:3443"
	if got := directAddressFromRequest(false, r); got != "https://192.168.1.20:3443" {
		t.Fatalf("got %q, want https for a normal (HTTPS) instance", got)
	}

	r2 := httptest.NewRequest(http.MethodGet, "/api/group", nil)
	r2.Host = "192.168.1.20:3003"
	if got := directAddressFromRequest(true, r2); got != "http://192.168.1.20:3003" {
		t.Fatalf("got %q, want http for an HTTPOnly instance", got)
	}
}

// Behind a reverse proxy the request arrives as plain HTTP, but the address
// the browser used was HTTPS; X-Forwarded-Proto is what says so.
func TestDirectAddressFromRequestHonoursForwardedProto(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/group", nil)
	r.Host = "bombvault.example.org"
	r.Header.Set("X-Forwarded-Proto", "https")
	if got := directAddressFromRequest(true, r); got != "https://bombvault.example.org" {
		t.Fatalf("got %q, want the forwarded proto to override HTTPOnly", got)
	}
}

// authGate is where every signed-in request passes through learnDirectURL;
// this proves the wiring, not just the parsing function above.
func TestSignedInRequestLearnsThisInstancesDirectAddress(t *testing.T) {
	in := newInstance(t, "cellar", strings.Repeat("a1", 32))
	r := httptest.NewRequest(http.MethodGet, "/api/group", nil)
	r.Host = "192.168.1.20:3443"
	r.AddCookie(&http.Cookie{Name: sessionCookieNameHTTP, Value: in.session}) //nolint:gosec // G124: request cookie, never set by a server
	w := httptest.NewRecorder()
	in.router.ServeHTTP(w, r)

	g, err := in.st.GetGroupState()
	if err != nil {
		t.Fatal(err)
	}
	if g.DirectURL != "http://192.168.1.20:3443" {
		t.Fatalf("DirectURL = %q, want the address learned from the request (this instance runs HTTPOnly)", g.DirectURL)
	}
}

// Only a signed-in browser may name this instance's address: without that, any
// host on the network could steer the members' direct calls elsewhere.
func TestOnlyASignedInRequestTeachesTheDirectAddress(t *testing.T) {
	in := newInstance(t, "cellar", strings.Repeat("a1", 32))
	learned := func() string {
		t.Helper()
		g, err := in.st.GetGroupState()
		if err != nil {
			t.Fatal(err)
		}
		return g.DirectURL
	}
	before := learned()

	r := httptest.NewRequest(http.MethodGet, "/api/group", nil)
	r.Host = "10.0.0.66:3443"
	in.router.ServeHTTP(httptest.NewRecorder(), r)
	if got := learned(); got != before {
		t.Fatalf("a request without a session set DirectURL to %q", got)
	}

	// Whoever holds the group's words has a paired session and names no host.
	hash, epoch, _ := in.h.authEnabled()
	r = httptest.NewRequest(http.MethodGet, "/api/group", nil)
	r.Host = "10.0.0.66:3443"
	r.AddCookie(&http.Cookie{Name: sessionCookieNameHTTP, Value: listedPhoneSession(t, in.st, in.appKey, hash, epoch)}) //nolint:gosec // G124: request cookie, never set by a server
	in.router.ServeHTTP(httptest.NewRecorder(), r)
	if got := learned(); got != before {
		t.Fatalf("a paired session set DirectURL to %q", got)
	}

	if _, err := in.st.MutateSettings(func(s *store.Settings) error {
		s.AuthPasswordHash = ""
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	r = httptest.NewRequest(http.MethodGet, "/api/group", nil)
	r.Host = "10.0.0.66:3443"
	in.router.ServeHTTP(httptest.NewRecorder(), r)
	if got := learned(); got != before {
		t.Fatalf("a request to an instance without a password set DirectURL to %q", got)
	}
}

// Pairing goes by the phrase alone: there is nowhere to type the address of
// an instance, this one's own included.
func TestPairingTakesNoTypedAddresses(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	in.do(t, http.MethodPost, "/api/group/phrase", nil)
	for _, route := range []struct{ method, path string }{
		{http.MethodPut, "/api/group/address"},
		{http.MethodPost, "/api/group/probe"},
	} {
		code, out := in.do(t, route.method, route.path, map[string]any{"url": "https://192.168.1.20:3443"})
		if code == http.StatusOK && out["ok"] == true {
			t.Errorf("%s %s took an address: %v", route.method, route.path, out)
		}
	}
	_, view := in.do(t, http.MethodGet, "/api/group", nil)
	if _, ok := view["selfAddressManual"]; ok {
		t.Fatalf("the group view still offers an address set by hand: %v", view)
	}
}
