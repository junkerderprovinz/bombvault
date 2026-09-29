package api

import (
	"context"
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

// A manually set address must survive further requests; that is the whole
// point of the manual flag.
func TestLearningAnAddressNeverOverwritesAManualOne(t *testing.T) {
	in := newInstance(t, "cellar", strings.Repeat("a1", 32))
	if err := in.st.SetGroupDirectURL("https://vault.example.org", true); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/api/group", nil)
	r.Host = "192.168.1.20:3443"
	r.AddCookie(&http.Cookie{Name: sessionCookieNameHTTP, Value: in.session}) //nolint:gosec // G124: request cookie, never set by a server
	in.router.ServeHTTP(httptest.NewRecorder(), r)

	g, err := in.st.GetGroupState()
	if err != nil {
		t.Fatal(err)
	}
	if g.DirectURL != "https://vault.example.org" {
		t.Fatalf("DirectURL = %q, want the manual address kept", g.DirectURL)
	}
}

func TestHandleGroupAddressSetsAndClearsTheManualOverride(t *testing.T) {
	in := newInstance(t, "cellar", strings.Repeat("a1", 32))

	code, out := in.do(t, http.MethodPut, "/api/group/address", map[string]any{"url": "https://vault.example.org"})
	if code != http.StatusOK || out["ok"] != true || out["selfAddress"] != "https://vault.example.org" || out["selfAddressManual"] != true {
		t.Fatalf("set address: %d %v", code, out)
	}

	code, out = in.do(t, http.MethodPut, "/api/group/address", map[string]any{"url": "https://user:pass@vault.example.org"})
	if code != http.StatusOK || out["ok"] == true {
		t.Fatalf("an address with credentials was accepted: %d %v", code, out)
	}

	code, out = in.do(t, http.MethodPut, "/api/group/address", map[string]any{"url": ""})
	if code != http.StatusOK || out["ok"] != true || out["selfAddressManual"] != false {
		t.Fatalf("clear address: %d %v", code, out)
	}
}

// The probe is what "Can't find it?" runs: a signed call to one address a
// person typed, recorded as a member exactly as the LAN sweep would.
func TestHandleGroupProbeFindsAndRecordsAMember(t *testing.T) {
	a := newInstance(t, "cellar", strings.Repeat("a1", 32))
	b := newInstance(t, "attic", strings.Repeat("b2", 32))

	bSrv := httptest.NewTLSServer(b.router)
	t.Cleanup(bSrv.Close)

	code, out := a.do(t, http.MethodPost, "/api/group/phrase", nil)
	phrase, _ := out["phrase"].(string)
	if code != http.StatusOK || len(strings.Fields(phrase)) != 12 {
		t.Fatalf("create phrase: %d %v", code, out)
	}
	if code, out := b.do(t, http.MethodPost, "/api/group/join", map[string]any{"phrase": phrase}); code != http.StatusOK || out["active"] != true {
		t.Fatalf("join: %d %v", code, out)
	}

	code, out = a.do(t, http.MethodPost, "/api/group/probe", map[string]any{"url": bSrv.URL})
	if code != http.StatusOK || out["ok"] != true {
		t.Fatalf("probe: %d %v", code, out)
	}
	members, _ := out["members"].([]any)
	if len(members) != 1 {
		t.Fatalf("members after the probe = %v, want one", out["members"])
	}
	m, _ := members[0].(map[string]any)
	if m["id"] != b.id(t) || m["direct"] != true {
		t.Fatalf("member = %v, want b found directly", m)
	}
}

// A probe of an instance that answers but belongs to a different group must
// not add anything: the signed call is refused before it ever identifies
// itself.
func TestHandleGroupProbeOfANonMemberFails(t *testing.T) {
	a := newInstance(t, "cellar", strings.Repeat("a1", 32))
	stranger := newInstance(t, "stranger", strings.Repeat("c3", 32))
	strangerSrv := httptest.NewTLSServer(stranger.router)
	t.Cleanup(strangerSrv.Close)

	if code, out := a.do(t, http.MethodPost, "/api/group/phrase", nil); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("create phrase on a: %d %v", code, out)
	}
	if code, out := stranger.do(t, http.MethodPost, "/api/group/phrase", nil); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("create phrase on stranger: %d %v", code, out)
	}

	code, out := a.do(t, http.MethodPost, "/api/group/probe", map[string]any{"url": strangerSrv.URL})
	if code != http.StatusOK || out["ok"] == true {
		t.Fatalf("probing a member of a different group succeeded: %d %v", code, out)
	}
	if members := a.svc.pairing().Members(); len(members) != 0 {
		t.Fatalf("members after a failed probe = %+v, want none", members)
	}
}

// A member's answer to a poll already carries its direct address (fleet.go's
// fleetStatusResponse.DirectURL); callMember harvests it, and the next call
// to that member uses it instead of the relay.
func TestFleetPollExchangesDirectAddressesOverTheRelay(t *testing.T) {
	a := newInstance(t, "cellar", strings.Repeat("a1", 32))
	b := newInstance(t, "attic", strings.Repeat("b2", 32))

	bSrv := httptest.NewTLSServer(b.router)
	t.Cleanup(bSrv.Close)
	if err := b.st.SetGroupDirectURL(bSrv.URL, true); err != nil {
		t.Fatal(err)
	}

	pairThroughRelay(t, a, b)

	if err := a.svc.RunFleetPolls(context.Background()); err != nil {
		t.Fatal(err)
	}

	// The poll's answer noted b's address; a second call now finds it
	// directly and confirms it.
	var status struct {
		OK bool `json:"ok"`
	}
	if err := a.svc.callMember(context.Background(), b.id(t), http.MethodGet, "/api/group/peer/status", nil, &status); err != nil {
		t.Fatalf("second call to b: %v", err)
	}

	found := false
	for _, m := range a.svc.pairing().Members() {
		if m.ID == b.id(t) {
			found = m.Direct
		}
	}
	if !found {
		t.Fatalf("members after the exchange = %+v, want b reachable directly", a.svc.pairing().Members())
	}
}
