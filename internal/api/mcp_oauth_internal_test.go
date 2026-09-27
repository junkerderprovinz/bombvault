package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// oauthClock is a router with OAuth on, a public client registered and the
// endpoint's clock under the test's control.
type oauthClock struct {
	h       *Handler
	router  http.Handler
	session string
	client  string
	now     time.Time
}

const internalRedirect = "https://claude.ai/api/mcp/auth_callback"

func newOAuthClock(t *testing.T) *oauthClock {
	t.Helper()
	h, router, repo, _ := newMCPGateHandler(t)
	hash := enableAuth(t, h, repo)
	s, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	c := &oauthClock{h: h, router: router, now: time.Unix(1_800_000_000, 0)}
	h.mcp.now = func() time.Time { return c.now }
	c.session = secret.NewSessionToken(h.cfg.AppKey, hash, s.SessionEpoch, 24*time.Hour)
	if err := repo.SetMCPOAuthSettings(store.MCPOAuthSettings{Enabled: true, Issuer: "https://vault.example"}, 1); err != nil {
		t.Fatal(err)
	}
	c.client = "bvc_test"
	if err := repo.CreateOAuthClient(store.OAuthClient{ID: c.client, Name: "Claude", RedirectURIs: []string{internalRedirect}, AuthMethod: "none"}, c.now.Unix()); err != nil {
		t.Fatal(err)
	}
	return c
}

func (c *oauthClock) send(t *testing.T, method, path, contentType, body string) map[string]any {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Host = "vault.example"
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: c.session}) //nolint:gosec // G124: request cookie
	w := httptest.NewRecorder()
	c.router.ServeHTTP(w, r)
	var m map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	m["status"] = w.Code
	return m
}

func (c *oauthClock) ticket(t *testing.T) string {
	t.Helper()
	q := url.Values{
		"response_type": {"code"}, "client_id": {c.client}, "redirect_uri": {internalRedirect},
		"code_challenge": {"E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"}, "code_challenge_method": {"S256"},
	}
	m := c.send(t, http.MethodGet, "/api/oauth/authorize?"+q.Encode(), "", "")
	ticket, _ := m["ticket"].(string)
	if ticket == "" {
		t.Fatalf("no ticket: %v", m)
	}
	return ticket
}

func (c *oauthClock) allow(t *testing.T, ticket string) map[string]any {
	t.Helper()
	return c.send(t, http.MethodPost, "/api/oauth/authorize", "application/json",
		fmt.Sprintf(`{"ticket":%q,"allow":true,"canStartBackups":false}`, ticket))
}

func (c *oauthClock) exchange(t *testing.T, code string) map[string]any {
	t.Helper()
	form := url.Values{
		"grant_type": {"authorization_code"}, "client_id": {c.client}, "code": {code},
		"redirect_uri": {internalRedirect}, "code_verifier": {"dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"},
	}
	return c.send(t, http.MethodPost, oauthTokenPath, "application/x-www-form-urlencoded", form.Encode())
}

func TestAnExpiredCodeIsRefused(t *testing.T) {
	c := newOAuthClock(t)
	res := c.allow(t, c.ticket(t))
	back, err := url.Parse(fmt.Sprint(res["redirect"]))
	if err != nil || back.Query().Get("code") == "" {
		t.Fatalf("allow: %v", res)
	}
	c.now = c.now.Add(oauthCodeTTL)
	if m := c.exchange(t, back.Query().Get("code")); m["status"] != http.StatusBadRequest || m["error"] != "invalid_grant" {
		t.Fatalf("an expired code gave %v", m)
	}
}

func TestACodeJustInsideItsLifetimeIsAccepted(t *testing.T) {
	c := newOAuthClock(t)
	res := c.allow(t, c.ticket(t))
	back, _ := url.Parse(fmt.Sprint(res["redirect"]))
	c.now = c.now.Add(oauthCodeTTL - time.Second)
	if m := c.exchange(t, back.Query().Get("code")); m["status"] != http.StatusOK {
		t.Fatalf("a code a second before expiry gave %v", m)
	}
}

func TestAnExpiredConsentPageCannotAllow(t *testing.T) {
	c := newOAuthClock(t)
	ticket := c.ticket(t)
	c.now = c.now.Add(oauthConsentTTL)
	if m := c.allow(t, ticket); m["ok"] != false || m["code"] != "oauth-consent-expired" {
		t.Fatalf("an expired consent ticket gave %v", m)
	}
}

func TestAnExpiredAccessTokenIsRefusedAndRefreshStillWorks(t *testing.T) {
	c := newOAuthClock(t)
	res := c.allow(t, c.ticket(t))
	back, _ := url.Parse(fmt.Sprint(res["redirect"]))
	tok := c.exchange(t, back.Query().Get("code"))
	access, _ := tok["access_token"].(string)

	c.now = c.now.Add(oauthAccessTTL)
	w := mcpInternalPost(c.router, access, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "")
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Header().Get("WWW-Authenticate"), `error="invalid_token"`) {
		t.Fatalf("an expired access token gave %d %q", w.Code, w.Header().Get("WWW-Authenticate"))
	}
	form := url.Values{"grant_type": {"refresh_token"}, "client_id": {c.client}, "refresh_token": {fmt.Sprint(tok["refresh_token"])}}
	if m := c.send(t, http.MethodPost, oauthTokenPath, "application/x-www-form-urlencoded", form.Encode()); m["status"] != http.StatusOK {
		t.Fatalf("refresh after the access token expired: %v", m)
	}
}

func TestPendingCodesAreCapped(t *testing.T) {
	s := newOAuthState()
	now := time.Unix(1000, 0)
	for i := 0; i < oauthPendingMax; i++ {
		if !s.putCode(fmt.Sprint(i), &oauthCode{expires: now.Add(time.Minute)}, now) {
			t.Fatalf("code %d refused below the cap", i)
		}
	}
	if s.putCode("one more", &oauthCode{expires: now.Add(time.Minute)}, now) {
		t.Fatal("a code past the cap was kept")
	}
	if !s.putCode("later", &oauthCode{expires: now.Add(2 * time.Minute)}, now.Add(time.Minute)) {
		t.Fatal("expired codes did not make room")
	}
}

func TestTheRegistrationLimitMakesRoomForANewAddress(t *testing.T) {
	w := newSlidingWindow(time.Hour, 1)
	w.maxKeys = 2
	now := time.Unix(1000, 0)
	for i, addr := range []string{"a", "b"} {
		if ok, _ := w.allow(addr, now.Add(time.Duration(i)*time.Minute)); !ok {
			t.Fatalf("%s refused", addr)
		}
	}
	if ok, _ := w.allow("c", now.Add(2*time.Minute)); !ok {
		t.Fatal("a new address was refused because others filled the table")
	}
	if len(w.hits) != 2 {
		t.Fatalf("the table holds %d addresses, want 2", len(w.hits))
	}
	if ok, _ := w.allow("b", now.Add(3*time.Minute)); ok {
		t.Fatal("the address with the newest hit was the one forgotten")
	}
}

func TestRegistrationsCountPerIPv6Network(t *testing.T) {
	for addr, want := range map[string]string{
		"203.0.113.7":           "203.0.113.7",
		"2001:db8:1:2:3:4:5:6":  "2001:db8:1:2::/64",
		"2001:db8:1:2:ffff::1":  "2001:db8:1:2::/64",
		"::ffff:198.51.100.4":   "198.51.100.4",
		"not an address at all": "not an address at all",
	} {
		if got := registrationKey(addr); got != want {
			t.Errorf("registrationKey(%q) = %q, want %q", addr, got, want)
		}
	}
}
