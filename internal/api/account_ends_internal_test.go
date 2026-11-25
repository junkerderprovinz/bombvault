package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/signin"
	"github.com/junkerderprovinz/parleyport/kit/signin/signintest"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// friend is a second account of in, which a sign-in service let in.
type friend struct {
	in      *instance
	session string
	account string
}

// friendOf adds a sign-in service to in, binds the owner to it and lets
// another person in through it.
func friendOf(t *testing.T, in *instance) *friend {
	t.Helper()
	if code, out := in.do(t, http.MethodPut, "/api/address", map[string]any{"address": "http://localhost"}); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("set the address: %d %v", code, out)
	}
	p := signintest.NewProvider(t)
	w := kitRequest(t, in.router, http.MethodPost, "methods/oidc", in.session, map[string]any{
		"provider": "pocket-id", "issuer": p.URL, "clientId": p.ClientID, "secret": p.Secret, "ca": string(p.CA),
	})
	service, _ := decodeMap(t, w)["service"].(map[string]any)
	id, _ := service["id"].(string)
	if id == "" {
		t.Fatalf("add a sign-in service: %d %s", w.Code, w.Body.String())
	}
	p.Next(signintest.Person{Subject: "sub-owner", Name: "owner-at-service"})
	w = kitRequest(t, in.router, http.MethodPost, "methods/oidc/"+id+"/test", in.session, map[string]any{"return": "/settings"})
	start, _ := decodeMap(t, w)["url"].(string)
	if jar := throughService(t, in, p, start, map[string]string{kitCookie: in.session}); jar == nil {
		t.Fatal("the test sign-in did not come back")
	}
	p.Next(signintest.Person{Subject: "sub-friend", Name: "friend"})
	jar := throughService(t, in, p, signin.BrowserPrefix+"oidc/"+id+"?return=/", map[string]string{})
	f := &friend{in: in, session: jar[kitCookie]}
	if f.session == "" {
		t.Fatal("the friend got no session")
	}
	f.account, _ = decodeMap(t, kitRequest(t, in.router, http.MethodGet, "me", f.session, nil))["account"].(string)
	if f.account == "" || f.account == ownerID {
		t.Fatalf("the friend signed in as %q", f.account)
	}
	return f
}

// throughService goes from path on in to the sign-in service and back, as a
// browser with the cookies of jar does, and returns the cookies it holds then.
func throughService(t *testing.T, in *instance, p *signintest.Provider, path string, jar map[string]string) map[string]string {
	t.Helper()
	get := func(target string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, target, nil)
		r.Host = "localhost"
		for name, value := range jar {
			r.AddCookie(&http.Cookie{Name: name, Value: value}) //nolint:gosec // G124: a cookie on a test request
		}
		w := httptest.NewRecorder()
		in.router.ServeHTTP(w, r)
		for _, c := range w.Result().Cookies() {
			jar[c.Name] = c.Value
		}
		return w
	}
	w := get(path)
	to := w.Header().Get("Location")
	if !strings.HasPrefix(to, p.URL) {
		t.Fatalf("start %s went to %q: %d %s", path, to, w.Code, w.Body.String())
	}
	res, err := p.Client().Get(to)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	back, err := url.Parse(res.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if w := get(back.RequestURI()); w.Code != http.StatusFound || (strings.Contains(w.Header().Get("Location"), "signin=") && !strings.Contains(w.Header().Get("Location"), "signin=testOk")) {
		t.Fatalf("back from the service: %d %s", w.Code, w.Header().Get("Location"))
	}
	return jar
}

func (f *friend) block(t *testing.T) {
	t.Helper()
	if w := kitRequest(t, f.in.router, http.MethodPatch, "accounts/"+f.account, f.in.session, map[string]any{"blocked": true}); w.Code != http.StatusOK {
		t.Fatalf("block the friend: %d %s", w.Code, w.Body.String())
	}
}

func (f *friend) remove(t *testing.T) {
	t.Helper()
	if w := kitRequest(t, f.in.router, http.MethodDelete, "accounts/"+f.account, f.in.session, nil); w.Code != http.StatusOK {
		t.Fatalf("remove the friend: %d %s", w.Code, w.Body.String())
	}
}

func decodeMap(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("%d %s: %v", w.Code, w.Body.String(), err)
	}
	return out
}

// bearer sends path to in with token and returns the status.
func bearer(in *instance, method, path, token, body string) int {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Host = "localhost"
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	w := httptest.NewRecorder()
	in.router.ServeHTTP(w, r)
	return w.Code
}

const mcpHello = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`

func TestAnMCPKeyEndsWithTheAccountThatMadeIt(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	f := friendOf(t, in)
	var key string
	in.as(f.session, func() {
		_, out := in.do(t, http.MethodPost, "/api/mcp/keys", map[string]any{"label": "friend's laptop"})
		key, _ = out["key"].(string)
	})
	if key == "" {
		t.Fatal("the friend made no key")
	}
	if code := bearer(in, http.MethodPost, mcpEndpointPath, key, mcpHello); code != http.StatusOK {
		t.Fatalf("the key before the block: %d", code)
	}
	f.block(t)
	if code := bearer(in, http.MethodPost, mcpEndpointPath, key, mcpHello); code == http.StatusOK {
		t.Fatalf("the key of a blocked account: %d", code)
	}
}

func TestAnAPITokenEndsWithTheAccountThatMadeIt(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	f := friendOf(t, in)
	var token string
	in.as(f.session, func() {
		_, out := in.do(t, http.MethodPost, "/api/tokens", map[string]any{"label": "friend's script"})
		token, _ = out["key"].(string)
	})
	if token == "" {
		t.Fatal("the friend made no token")
	}
	if code := bearer(in, http.MethodGet, "/api/v1/status", token, ""); code != http.StatusOK {
		t.Fatalf("the token before the removal: %d", code)
	}
	f.remove(t)
	if code := bearer(in, http.MethodGet, "/api/v1/status", token, ""); code != http.StatusUnauthorized {
		t.Fatalf("the token of a removed account: %d", code)
	}
}

func TestAnOAuthGrantAndItsCodesEndWithTheAccountThatAllowedThem(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	f := friendOf(t, in)
	now := time.Now()
	grantID := newMCPKeyID()
	access := secret.OAuthAccessPrefix + strings.Repeat("a", 43)
	if _, err := in.st.CreateOAuthGrant(store.OAuthGrant{
		ID: grantID, Label: "Assistant", Account: f.account,
		Check:          secret.MCPKeyCheck(in.appKey, grantID),
		AccessDigest:   secret.HashOAuthSecret(in.appKey, "access", access),
		RefreshDigest:  secret.HashOAuthSecret(in.appKey, "refresh", "refresh"),
		AccessExpires:  now.Add(time.Hour).Unix(),
		RefreshExpires: now.Add(24 * time.Hour).Unix(),
	}, now.Unix()); err != nil {
		t.Fatal(err)
	}
	if !in.h.mcp.oauth.putCode("waiting", &oauthCode{clientID: "c", expires: now.Add(time.Minute), account: f.account}, now) ||
		!in.h.mcp.oauth.putCode("the owner's", &oauthCode{clientID: "c", expires: now.Add(time.Minute)}, now) {
		t.Fatal("no room for the codes")
	}
	f.block(t)
	if grant, err := in.st.GetMCPKey(grantID); err != nil || grant.RevokedAt == 0 {
		t.Fatalf("the grant of a blocked account: %+v %v", grant, err)
	}
	in.h.mcp.oauth.mu.Lock()
	_, waiting := in.h.mcp.oauth.codes["waiting"]
	_, owners := in.h.mcp.oauth.codes["the owner's"]
	in.h.mcp.oauth.mu.Unlock()
	if waiting || !owners {
		t.Fatalf("codes after the block: the friend's kept %v, the owner's kept %v", waiting, owners)
	}
}

func TestAPhoneEndsWithTheAccountThatLetItIn(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	f := friendOf(t, in)
	hash, epoch, _ := in.h.phoneSigning()
	phone := listedPhoneSession(t, in.st, in.appKey, hash, epoch)
	if err := in.st.SetPhoneAccount(wordsHolderID, f.account); err != nil {
		t.Fatal(err)
	}
	in.as(phone, func() {
		if code, out := in.do(t, http.MethodGet, "/api/status", nil); code != http.StatusOK {
			t.Fatalf("the phone before the block: %d %v", code, out)
		}
	})
	if _, ok := in.h.phoneSessionCookie(wordsHolderID, phone); !ok {
		t.Fatal("the phone's page over the relay before the block")
	}
	f.block(t)
	in.as(phone, func() {
		if code, _ := in.do(t, http.MethodGet, "/api/status", nil); code != http.StatusUnauthorized {
			t.Fatalf("the phone of a blocked account: %d", code)
		}
	})
	if _, ok := in.h.phoneSessionCookie(wordsHolderID, phone); ok {
		t.Fatal("the phone of a blocked account opened a page over the relay")
	}
	if got := in.h.phoneSession(wordsHolderID); got.Value != "" {
		t.Fatal("the phone of a blocked account got a new session")
	}
}

func TestAFortressTabEndsWithItsAccount(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	f := friendOf(t, in)
	var valid func() bool
	tab := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, valid, _ = in.h.tabSession(r)
	})
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Host = "localhost"
	r.AddCookie(sessionCookieFor(f.session))
	in.h.auth.Guard(tab).ServeHTTP(httptest.NewRecorder(), r)
	if valid == nil || !valid() {
		t.Fatal("the friend's tab before the block")
	}
	f.block(t)
	if valid() {
		t.Fatal("the tab of a blocked account stays open")
	}
}
