package api_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/logring"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const (
	oauthIssuer   = "https://vault.example"
	oauthResource = oauthIssuer + "/mcp"
	oauthHost     = "vault.example"
	chatgptReturn = "https://chatgpt.com/connector_platform_oauth_redirect"
	pkceVerifier  = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	pkceChallenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
)

// oauthEnv is a router with a login password and OAuth switched on under
// oauthIssuer, and a session cookie of the operator.
type oauthEnv struct {
	h      http.Handler
	st     *store.Repo
	cookie string
}

func newOAuthEnv(t *testing.T) oauthEnv {
	t.Helper()
	st := newMemStore(t)
	h, _ := newMCPKeyRouter(t, st, mcpAppKey)
	cookie := enableLogin(t, st, mcpAppKey)
	w, m := doMCPKeyJSON(t, h, http.MethodPut, "/api/mcp/oauth",
		`{"enabled":true,"issuer":"`+oauthIssuer+`"}`, oauthHost, cookie)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("switch OAuth on: status=%d body=%v", w.Code, m)
	}
	return oauthEnv{h: h, st: st, cookie: cookie}
}

// get sends a GET without a session.
func (e oauthEnv) get(t *testing.T, path string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.Host = oauthHost
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	var m map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	return w, m
}

// register posts a client registration from addr and returns the answer.
func (e oauthEnv) register(t *testing.T, body, addr string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
	r.Host = oauthHost
	r.Header.Set("Content-Type", "application/json")
	if addr != "" {
		r.RemoteAddr = addr
	}
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	var m map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	return w, m
}

// publicClient registers a public client that returns to redirect.
func (e oauthEnv) publicClient(t *testing.T, name, redirect string) string {
	t.Helper()
	w, m := e.register(t, fmt.Sprintf(`{"client_name":%q,"redirect_uris":[%q],"grant_types":["authorization_code","refresh_token"],"token_endpoint_auth_method":"none"}`, name, redirect), "")
	if w.Code != http.StatusCreated {
		t.Fatalf("register: status=%d body=%v", w.Code, m)
	}
	id, _ := m["client_id"].(string)
	return id
}

func authorizeQuery(client, redirect string, extra ...string) url.Values {
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {client},
		"redirect_uri":          {redirect},
		"code_challenge":        {pkceChallenge},
		"code_challenge_method": {"S256"},
		"state":                 {"st-123"},
		"resource":              {oauthResource},
		"scope":                 {"mcp"},
	}
	for i := 0; i+1 < len(extra); i += 2 {
		if extra[i+1] == "" {
			q.Del(extra[i])
			continue
		}
		q.Set(extra[i], extra[i+1])
	}
	return q
}

// consent asks for the consent page's data with the operator's session.
func (e oauthEnv) consent(t *testing.T, q url.Values) map[string]any {
	t.Helper()
	w, m := doMCPKeyJSON(t, e.h, http.MethodGet, "/api/oauth/authorize?"+q.Encode(), "", oauthHost, e.cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("consent data: status=%d body=%v", w.Code, m)
	}
	return m
}

// decide posts the operator's answer to a consent ticket.
func (e oauthEnv) decide(t *testing.T, ticket string, allow, canStart bool) map[string]any {
	t.Helper()
	_, m := doMCPKeyJSON(t, e.h, http.MethodPost, "/api/oauth/authorize",
		fmt.Sprintf(`{"ticket":%q,"allow":%t,"canStartBackups":%t}`, ticket, allow, canStart), oauthHost, e.cookie)
	return m
}

// approve runs the consent step and returns the redirect it ends in.
func (e oauthEnv) approve(t *testing.T, q url.Values, canStart bool) *url.URL {
	t.Helper()
	m := e.consent(t, q)
	ticket, _ := m["ticket"].(string)
	if m["ok"] != true || ticket == "" {
		t.Fatalf("consent data: %v", m)
	}
	res := e.decide(t, ticket, true, canStart)
	if res["ok"] != true {
		t.Fatalf("allow: %v", res)
	}
	return mustURL(t, res["redirect"])
}

func mustURL(t *testing.T, v any) *url.URL {
	t.Helper()
	s, _ := v.(string)
	u, err := url.Parse(s)
	if err != nil || s == "" {
		t.Fatalf("no redirect URL in %v", v)
	}
	return u
}

// token posts to the token endpoint.
func (e oauthEnv) token(t *testing.T, form url.Values, user, pass string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	r.Host = oauthHost
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if user != "" {
		r.SetBasicAuth(url.QueryEscape(user), url.QueryEscape(pass))
	}
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	var m map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	return w, m
}

func codeExchange(client, code, redirect string) url.Values {
	return url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {client},
		"code":          {code},
		"redirect_uri":  {redirect},
		"code_verifier": {pkceVerifier},
		"resource":      {oauthResource},
	}
}

// signIn runs the whole flow for a new public client and returns the client
// and the token response.
func (e oauthEnv) signIn(t *testing.T, canStart bool) (string, map[string]any) {
	t.Helper()
	client := e.publicClient(t, "ChatGPT", chatgptReturn)
	back := e.approve(t, authorizeQuery(client, chatgptReturn), canStart)
	w, m := e.token(t, codeExchange(client, back.Query().Get("code"), chatgptReturn), "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("code exchange: status=%d body=%v", w.Code, m)
	}
	return client, m
}

// mcpStatus is the status /mcp answers a tools/list with token.
func mcpStatus(t *testing.T, h http.Handler, token string) int {
	t.Helper()
	return (mcpReq{key: token, host: oauthHost, body: `{"jsonrpc":"2.0","id":3,"method":"tools/list"}`}).do(t, h).Code
}

func str(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return s
}

func TestOAuthMetadataDescribesTheResourceAndTheServer(t *testing.T) {
	e := newOAuthEnv(t)

	w, prm := e.get(t, "/.well-known/oauth-protected-resource/mcp")
	if w.Code != http.StatusOK {
		t.Fatalf("protected resource metadata: %d", w.Code)
	}
	if str(prm, "resource") != oauthResource {
		t.Fatalf("resource = %v", prm["resource"])
	}
	if servers, _ := prm["authorization_servers"].([]any); len(servers) != 1 || servers[0] != oauthIssuer {
		t.Fatalf("authorization_servers = %v", prm["authorization_servers"])
	}

	w, as := e.get(t, "/.well-known/oauth-authorization-server")
	if w.Code != http.StatusOK {
		t.Fatalf("authorization server metadata: %d", w.Code)
	}
	for k, want := range map[string]string{
		"issuer":                 oauthIssuer,
		"authorization_endpoint": oauthIssuer + "/oauth/authorize",
		"token_endpoint":         oauthIssuer + "/oauth/token",
		"registration_endpoint":  oauthIssuer + "/oauth/register",
		"revocation_endpoint":    oauthIssuer + "/oauth/revoke",
	} {
		if str(as, k) != want {
			t.Errorf("%s = %v, want %s", k, as[k], want)
		}
	}
	if fmt.Sprint(as["code_challenge_methods_supported"]) != "[S256]" {
		t.Errorf("code_challenge_methods_supported = %v", as["code_challenge_methods_supported"])
	}
	if fmt.Sprint(as["response_types_supported"]) != "[code]" {
		t.Errorf("response_types_supported = %v", as["response_types_supported"])
	}
	if as["authorization_response_iss_parameter_supported"] != true {
		t.Error("the server must announce the iss parameter it sends")
	}
	if !strings.Contains(fmt.Sprint(as["token_endpoint_auth_methods_supported"]), "none") {
		t.Errorf("token_endpoint_auth_methods_supported = %v", as["token_endpoint_auth_methods_supported"])
	}
}

func TestAnUnauthenticatedMCPRequestPointsAtTheMetadata(t *testing.T) {
	e := newOAuthEnv(t)
	w := (mcpReq{host: oauthHost, body: `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`}).do(t, e.h)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status %d without any key or grant, want 401", w.Code)
	}
	challenge := w.Header().Get("WWW-Authenticate")
	for _, part := range []string{
		`Bearer `,
		`resource_metadata="` + oauthIssuer + `/.well-known/oauth-protected-resource/mcp"`,
		`scope="mcp"`,
	} {
		if !strings.Contains(challenge, part) {
			t.Errorf("WWW-Authenticate %q lacks %s", challenge, part)
		}
	}
}

func TestNoOAuthWithoutALoginPassword(t *testing.T) {
	st := newMemStore(t)
	h, _ := newMCPKeyRouter(t, st, mcpAppKey)

	_, m := doMCPKeyJSON(t, h, http.MethodPut, "/api/mcp/oauth", `{"enabled":true,"issuer":"`+oauthIssuer+`"}`, mcpTestHost, "")
	if m["ok"] != false || m["code"] != "mcp-oauth-needs-password" {
		t.Fatalf("switching OAuth on without a password gave %v", m)
	}
	// Even with the switch stored, nothing is offered while there is no password.
	if err := st.SetMCPOAuthSettings(store.MCPOAuthSettings{Enabled: true, Issuer: oauthIssuer}, 1); err != nil {
		t.Fatal(err)
	}
	e := oauthEnv{h: h, st: st}
	for _, path := range []string{"/.well-known/oauth-protected-resource/mcp", "/.well-known/oauth-authorization-server"} {
		if w, _ := e.get(t, path); w.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, w.Code)
		}
	}
	if w, _ := e.register(t, `{"redirect_uris":["https://chatgpt.com/cb"]}`, ""); w.Code != http.StatusNotFound {
		t.Errorf("registration = %d, want 404", w.Code)
	}
	if w := (mcpReq{host: oauthHost, body: `{}`}).do(t, h); w.Code != http.StatusNotFound {
		t.Errorf("/mcp without keys = %d, want 404", w.Code)
	}
}

func TestATokenStopsWorkingWhenTheLoginPasswordGoes(t *testing.T) {
	e := newOAuthEnv(t)
	_, tok := e.signIn(t, false)
	if got := mcpStatus(t, e.h, str(tok, "access_token")); got != http.StatusOK {
		t.Fatalf("fresh token: %d", got)
	}
	s := mustSettings(t, e.st)
	s.AuthPasswordHash = ""
	if err := e.st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	if got := mcpStatus(t, e.h, str(tok, "access_token")); got == http.StatusOK {
		t.Fatal("an OAuth token still works after the login password was removed")
	}
}

func TestTheIssuerMustBeAnHTTPSOrigin(t *testing.T) {
	e := newOAuthEnv(t)
	for _, issuer := range []string{
		"http://vault.example",
		"https://vault.example/bombvault",
		"https://vault.example?x=1",
		"https://user@vault.example",
		"vault.example",
		"",
	} {
		_, m := doMCPKeyJSON(t, e.h, http.MethodPut, "/api/mcp/oauth",
			fmt.Sprintf(`{"enabled":true,"issuer":%q}`, issuer), oauthHost, e.cookie)
		if m["ok"] != false || m["code"] != "mcp-oauth-issuer-invalid" {
			t.Errorf("issuer %q gave %v", issuer, m)
		}
	}
	_, m := doMCPKeyJSON(t, e.h, http.MethodPut, "/api/mcp/oauth",
		`{"enabled":true,"issuer":"HTTPS://Vault.Example:443/"}`, oauthHost, e.cookie)
	o, _ := m["oauth"].(map[string]any)
	if m["ok"] != true || o["issuer"] != oauthIssuer {
		t.Fatalf("a spelling of the same origin gave %v", m)
	}
}

func TestRegistrationAcceptsOnlyHTTPSAndLoopbackRedirects(t *testing.T) {
	e := newOAuthEnv(t)
	for uri, ok := range map[string]bool{
		"https://chatgpt.com/connector_platform_oauth_redirect": true,
		"https://claude.ai/api/mcp/auth_callback":               true,
		"http://localhost:3118/callback":                        true,
		"http://127.0.0.1/callback":                             true,
		"http://[::1]:9000/cb":                                  true,
		"http://evil.example/cb":                                false,
		"https://chatgpt.com/cb#frag":                           false,
		"cursor://anysphere.cursor-retrieval/oauth":             false,
		"https:///nohost":                                       false,
		"javascript:alert(1)":                                   false,
	} {
		w, m := e.register(t, fmt.Sprintf(`{"redirect_uris":[%q],"token_endpoint_auth_method":"none"}`, uri), "")
		if ok && w.Code != http.StatusCreated {
			t.Errorf("%s refused: %d %v", uri, w.Code, m)
		}
		if !ok && (w.Code != http.StatusBadRequest || m["error"] != "invalid_redirect_uri") {
			t.Errorf("%s accepted or wrongly refused: %d %v", uri, w.Code, m)
		}
	}
}

func TestRegistrationIsLimitedPerAddress(t *testing.T) {
	e := newOAuthEnv(t)
	body := `{"redirect_uris":["https://chatgpt.com/cb"],"token_endpoint_auth_method":"none"}`
	for i := 0; i < 10; i++ {
		if w, m := e.register(t, body, "198.51.100.7:4000"); w.Code != http.StatusCreated {
			t.Fatalf("registration %d: %d %v", i+1, w.Code, m)
		}
	}
	w, _ := e.register(t, body, "198.51.100.7:4001")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("the eleventh registration in an hour gave %d, want 429", w.Code)
	}
	if w, _ := e.register(t, body, "198.51.100.8:4000"); w.Code != http.StatusCreated {
		t.Fatalf("another address was refused too: %d", w.Code)
	}
}

func TestOnlyTheVendorsOwnCallbackMarksAClientAsKnown(t *testing.T) {
	e := newOAuthEnv(t)
	for uri, want := range map[string]string{
		chatgptReturn: "chatgpt",
		"https://claude.ai/api/mcp/auth_callback": "claudeai",
		"https://claude.ai/share/anything":        "",
		"https://chatgpt.com/g/some-gpt":          "",
	} {
		client := e.publicClient(t, "Claude", uri)
		c, _ := e.consent(t, authorizeQuery(client, uri))["client"].(map[string]any)
		if c["known"] != want {
			t.Errorf("a client returning to %s is shown as known=%v, want %q", uri, c["known"], want)
		}
	}
}

func TestARedirectHostMustBeWrittenInASCII(t *testing.T) {
	e := newOAuthEnv(t)
	for uri, ok := range map[string]bool{
		"https://cl\u0430ude.ai/api/mcp/auth_callback": false,
		"https://cl%61ude.ai/api/mcp/auth_callback":    false,
		"https://xn--clude-9ve.ai/cb":                  true,
	} {
		w, m := e.register(t, fmt.Sprintf(`{"redirect_uris":[%q],"token_endpoint_auth_method":"none"}`, uri), "")
		if ok != (w.Code == http.StatusCreated) {
			t.Errorf("%s: %d %v", uri, w.Code, m)
		}
	}
}

func TestAClientNameLosesInvisibleAndDirectionCharacters(t *testing.T) {
	e := newOAuthEnv(t)
	client := e.publicClient(t, "Chat\u200dGPT\u202e\u2066", chatgptReturn)
	c, _ := e.consent(t, authorizeQuery(client, chatgptReturn))["client"].(map[string]any)
	if c["name"] != "ChatGPT" {
		t.Fatalf("the consent page shows the name as %+q", c["name"])
	}
}

func TestRegistrationIsLimitedPerIPv6Network(t *testing.T) {
	e := newOAuthEnv(t)
	body := `{"redirect_uris":["https://chatgpt.com/cb"],"token_endpoint_auth_method":"none"}`
	for i := 0; i < 10; i++ {
		if w, m := e.register(t, body, fmt.Sprintf("[2001:db8::%x]:4000", i+1)); w.Code != http.StatusCreated {
			t.Fatalf("registration %d: %d %v", i+1, w.Code, m)
		}
	}
	if w, _ := e.register(t, body, "[2001:db8::ff]:4000"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("another address of the same /64 gave %d, want 429", w.Code)
	}
	if w, _ := e.register(t, body, "[2001:db8:0:1::1]:4000"); w.Code != http.StatusCreated {
		t.Fatalf("another network was refused too: %d", w.Code)
	}
}

func TestManyAddressesCannotShutRegistration(t *testing.T) {
	e := newOAuthEnv(t)
	body := `{"redirect_uris":["https://x.example/cb"],"token_endpoint_auth_method":"none"}`
	for i := 0; i < 4096; i++ {
		addr := fmt.Sprintf("10.%d.%d.%d:1234", i>>16&0xff, i>>8&0xff, i&0xff)
		if w, m := e.register(t, body, addr); w.Code != http.StatusCreated {
			t.Fatalf("registration from address %d: %d %v", i, w.Code, m)
		}
	}
	w, m := e.register(t, `{"redirect_uris":["`+chatgptReturn+`"],"token_endpoint_auth_method":"none"}`, "20.0.0.1:443")
	if w.Code != http.StatusCreated {
		t.Fatalf("a registration after 4096 other addresses: %d %v", w.Code, m)
	}
}

func TestTheFullCodeFlowWithPKCE(t *testing.T) {
	e := newOAuthEnv(t)
	client := e.publicClient(t, "Some name", chatgptReturn)

	info := e.consent(t, authorizeQuery(client, chatgptReturn))
	c, _ := info["client"].(map[string]any)
	if c["known"] != "chatgpt" || c["redirectHost"] != "chatgpt.com" {
		t.Fatalf("consent data names the client as %v", c)
	}

	back := e.approve(t, authorizeQuery(client, chatgptReturn), true)
	if got := back.Scheme + "://" + back.Host + back.Path; got != chatgptReturn {
		t.Fatalf("redirected to %s", got)
	}
	q := back.Query()
	if q.Get("state") != "st-123" || q.Get("iss") != oauthIssuer || !strings.HasPrefix(q.Get("code"), secret.OAuthCodePrefix) {
		t.Fatalf("redirect query %v", q)
	}

	w, tok := e.token(t, codeExchange(client, q.Get("code"), chatgptReturn), "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("exchange: %d %v", w.Code, tok)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Error("the token response may be cached")
	}
	if tok["token_type"] != "Bearer" || tok["scope"] != "mcp" || tok["expires_in"] != float64(3600) ||
		!strings.HasPrefix(str(tok, "access_token"), secret.OAuthAccessPrefix) ||
		!strings.HasPrefix(str(tok, "refresh_token"), secret.OAuthRefreshPrefix) {
		t.Fatalf("token response %v", tok)
	}
	if got := mcpStatus(t, e.h, str(tok, "access_token")); got != http.StatusOK {
		t.Fatalf("the access token opens /mcp with %d", got)
	}

	rows := mcpKeyRows(t, mcpKeyListAs(t, e), "keys")
	if len(rows) != 1 || rows[0]["viaOAuth"] != true || rows[0]["label"] != "ChatGPT" ||
		rows[0]["client"] != "chatgpt" || rows[0]["canStartBackups"] != true {
		t.Fatalf("grant tile %v", rows)
	}
}

// mcpKeyListAs reads the key listing with the operator's session.
func mcpKeyListAs(t *testing.T, e oauthEnv) map[string]any {
	t.Helper()
	w, m := doMCPKeyJSON(t, e.h, http.MethodGet, "/api/mcp/keys", "", oauthHost, e.cookie)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("list: %d %v", w.Code, m)
	}
	return m
}

func TestTheStartSwitchIsOffUnlessTheOperatorTurnsItOn(t *testing.T) {
	e := newOAuthEnv(t)
	e.signIn(t, false)
	rows := mcpKeyRows(t, mcpKeyListAs(t, e), "keys")
	if len(rows) != 1 || rows[0]["canStartBackups"] != false {
		t.Fatalf("grant %v, want read only", rows)
	}
}

func TestAWrongVerifierIsRefusedAndLeavesTheCodeUsable(t *testing.T) {
	e := newOAuthEnv(t)
	client := e.publicClient(t, "ChatGPT", chatgptReturn)
	code := e.approve(t, authorizeQuery(client, chatgptReturn), false).Query().Get("code")

	form := codeExchange(client, code, chatgptReturn)
	form.Set("code_verifier", strings.Repeat("x", 43))
	if w, m := e.token(t, form, "", ""); w.Code != http.StatusBadRequest || m["error"] != "invalid_grant" {
		t.Fatalf("wrong verifier: %d %v", w.Code, m)
	}
	form.Del("code_verifier")
	if w, m := e.token(t, form, "", ""); w.Code != http.StatusBadRequest || m["error"] != "invalid_grant" {
		t.Fatalf("missing verifier: %d %v", w.Code, m)
	}
	if w, m := e.token(t, codeExchange(client, code, chatgptReturn), "", ""); w.Code != http.StatusOK {
		t.Fatalf("the right verifier afterwards: %d %v", w.Code, m)
	}
}

func TestAWrongRedirectIsRefused(t *testing.T) {
	e := newOAuthEnv(t)
	client := e.publicClient(t, "ChatGPT", chatgptReturn)

	m := e.consent(t, authorizeQuery(client, "https://chatgpt.com/other"))
	if m["ok"] != false || m["code"] != "oauth-bad-redirect" || m["redirect"] != nil {
		t.Fatalf("an unregistered redirect gave %v; it must be shown, never followed", m)
	}
	m = e.consent(t, authorizeQuery(client, chatgptReturn+"/"))
	if m["ok"] != false || m["code"] != "oauth-bad-redirect" {
		t.Fatalf("a redirect that only nearly matches gave %v", m)
	}

	code := e.approve(t, authorizeQuery(client, chatgptReturn), false).Query().Get("code")
	if w, m := e.token(t, codeExchange(client, code, "https://chatgpt.com/other"), "", ""); w.Code != http.StatusBadRequest || m["error"] != "invalid_grant" {
		t.Fatalf("exchange with another redirect: %d %v", w.Code, m)
	}
}

func TestALoopbackRedirectMatchesOnAnyPort(t *testing.T) {
	e := newOAuthEnv(t)
	client := e.publicClient(t, "Claude Code", "http://localhost/callback")
	back := e.approve(t, authorizeQuery(client, "http://localhost:53682/callback"), false)
	if back.Host != "localhost:53682" {
		t.Fatalf("redirected to %s", back)
	}
	m := e.consent(t, authorizeQuery(client, "http://localhost:53682/other"))
	if m["code"] != "oauth-bad-redirect" {
		t.Fatalf("a loopback redirect with another path gave %v", m)
	}
	info := e.consent(t, authorizeQuery(client, "http://localhost:1/callback"))
	c, _ := info["client"].(map[string]any)
	if c["loopback"] != true || c["known"] != "" {
		t.Fatalf("a loopback client is shown as %v", c)
	}
}

func TestAReplayedCodeRevokesTheGrantItMade(t *testing.T) {
	e := newOAuthEnv(t)
	client := e.publicClient(t, "ChatGPT", chatgptReturn)
	code := e.approve(t, authorizeQuery(client, chatgptReturn), false).Query().Get("code")
	w, tok := e.token(t, codeExchange(client, code, chatgptReturn), "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("first exchange: %d", w.Code)
	}
	if w, m := e.token(t, codeExchange(client, code, chatgptReturn), "", ""); w.Code != http.StatusBadRequest || m["error"] != "invalid_grant" {
		t.Fatalf("replay: %d %v", w.Code, m)
	}
	if got := mcpStatus(t, e.h, str(tok, "access_token")); got != http.StatusUnauthorized {
		t.Fatalf("the grant of a replayed code still works: %d", got)
	}
	revoked := mcpKeyRows(t, mcpKeyListAs(t, e), "revoked")
	if len(revoked) != 1 || revoked[0]["revokedReason"] != "code-replay" {
		t.Fatalf("revoked rows %v", revoked)
	}
}

func TestRefreshRotatesAndAReusedTokenRevokesTheGrant(t *testing.T) {
	e := newOAuthEnv(t)
	client, first := e.signIn(t, false)
	refresh := url.Values{"grant_type": {"refresh_token"}, "client_id": {client}, "refresh_token": {str(first, "refresh_token")}, "resource": {oauthResource}}

	w, second := e.token(t, refresh, "", "")
	if w.Code != http.StatusOK || str(second, "refresh_token") == str(first, "refresh_token") || str(second, "access_token") == "" {
		t.Fatalf("refresh: %d %v", w.Code, second)
	}
	if got := mcpStatus(t, e.h, str(second, "access_token")); got != http.StatusOK {
		t.Fatalf("the refreshed access token: %d", got)
	}

	next := url.Values{"grant_type": {"refresh_token"}, "client_id": {client}, "refresh_token": {str(second, "refresh_token")}}
	w, third := e.token(t, next, "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("the second refresh: %d %v", w.Code, third)
	}

	if w, m := e.token(t, refresh, "", ""); w.Code != http.StatusBadRequest || m["error"] != "invalid_grant" {
		t.Fatalf("reusing the first refresh token: %d %v", w.Code, m)
	}
	if got := mcpStatus(t, e.h, str(third, "access_token")); got != http.StatusUnauthorized {
		t.Fatalf("the grant survived a reused refresh token: %d", got)
	}
	last := url.Values{"grant_type": {"refresh_token"}, "client_id": {client}, "refresh_token": {str(third, "refresh_token")}}
	if w, m := e.token(t, last, "", ""); w.Code != http.StatusBadRequest || m["error"] != "invalid_grant" {
		t.Fatalf("the newest refresh token after reuse: %d %v", w.Code, m)
	}
	revoked := mcpKeyRows(t, mcpKeyListAs(t, e), "revoked")
	if len(revoked) != 1 || revoked[0]["revokedReason"] != "refresh-reuse" {
		t.Fatalf("revoked rows %v", revoked)
	}
}

// Every request in these tests comes from one address, which is what clients
// behind a reverse proxy without TRUSTED_PROXY look like.
func TestJunkAtTheTokenEndpointDoesNotStopARefresh(t *testing.T) {
	e := newOAuthEnv(t)
	client, tok := e.signIn(t, false)
	for i := 0; i < 5; i++ {
		e.token(t, url.Values{"grant_type": {"refresh_token"}, "client_id": {"bvc_nope"}, "refresh_token": {"x"}}, "", "")
		e.token(t, url.Values{"grant_type": {"refresh_token"}, "client_id": {client}, "refresh_token": {"bvrt_expired"}}, "", "")
		e.token(t, codeExchange(client, "bvac_unknown", chatgptReturn), "", "")
	}
	w, m := e.token(t, url.Values{"grant_type": {"refresh_token"}, "client_id": {client}, "refresh_token": {str(tok, "refresh_token")}}, "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("the client's own refresh after junk from its address: %d %v", w.Code, m)
	}
}

func TestAWrongClientSecretIsThrottledPerClient(t *testing.T) {
	e := newOAuthEnv(t)
	_, reg := e.register(t, `{"redirect_uris":["`+chatgptReturn+`"]}`, "")
	client, clientSecret := str(reg, "client_id"), str(reg, "client_secret")
	code := e.approve(t, authorizeQuery(client, chatgptReturn), false).Query().Get("code")
	for i := 0; i < 5; i++ {
		if w, _ := e.token(t, codeExchange(client, code, chatgptReturn), client, "bvcs_wrong"); w.Code != http.StatusUnauthorized {
			t.Fatalf("wrong secret %d: %d", i, w.Code)
		}
	}
	if w, _ := e.token(t, codeExchange(client, code, chatgptReturn), client, clientSecret); w.Code != http.StatusTooManyRequests {
		t.Fatalf("the right secret after five wrong ones: %d, want 429", w.Code)
	}
	other, tok := e.signIn(t, false)
	refresh := url.Values{"grant_type": {"refresh_token"}, "client_id": {other}, "refresh_token": {str(tok, "refresh_token")}}
	if w, m := e.token(t, refresh, "", ""); w.Code != http.StatusOK {
		t.Fatalf("another client from the same address: %d %v", w.Code, m)
	}
}

func TestWrongKeysFromAProxyDoNotStopAnOAuthClient(t *testing.T) {
	e := newOAuthEnv(t)
	_, tok := e.signIn(t, false)
	for i := 0; i < 5; i++ {
		mcpStatus(t, e.h, "bvmcp_wrong")
	}
	if got := mcpStatus(t, e.h, str(tok, "access_token")); got != http.StatusOK {
		t.Fatalf("an access token after five wrong keys from its address: %d", got)
	}
}

func TestStaleAccessTokensDoNotCountAsFailedAttempts(t *testing.T) {
	e := newOAuthEnv(t)
	w, m := doMCPKeyJSON(t, e.h, http.MethodPost, "/api/mcp/keys", `{"label":"Laptop","canStartBackups":false}`, oauthHost, e.cookie)
	key := str(m, "key")
	if w.Code != http.StatusOK || key == "" {
		t.Fatalf("create key: %d %v", w.Code, m)
	}
	for i := 0; i < 5; i++ {
		if got := mcpStatus(t, e.h, secret.OAuthAccessPrefix+"expired"); got != http.StatusUnauthorized {
			t.Fatalf("a stale access token: %d", got)
		}
	}
	if got := mcpStatus(t, e.h, key); got != http.StatusOK {
		t.Fatalf("a key after five stale access tokens from its address: %d", got)
	}
}

// A client whose refresh answer got lost on the way sends the same request
// again. That is no sign of a stolen token.
func TestARetriedRefreshKeepsTheGrant(t *testing.T) {
	e := newOAuthEnv(t)
	client, tok := e.signIn(t, false)
	form := url.Values{"grant_type": {"refresh_token"}, "client_id": {client}, "refresh_token": {str(tok, "refresh_token")}}
	if w, m := e.token(t, form, "", ""); w.Code != http.StatusOK {
		t.Fatalf("refresh: %d %v", w.Code, m)
	}
	w, retry := e.token(t, form, "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("the retried refresh: %d %v", w.Code, retry)
	}
	if got := mcpStatus(t, e.h, str(retry, "access_token")); got != http.StatusOK {
		t.Fatalf("the retry's access token: %d", got)
	}
	next := url.Values{"grant_type": {"refresh_token"}, "client_id": {client}, "refresh_token": {str(retry, "refresh_token")}}
	if w, m := e.token(t, next, "", ""); w.Code != http.StatusOK {
		t.Fatalf("the retry's refresh token: %d %v", w.Code, m)
	}
	if revoked := mcpKeyRows(t, mcpKeyListAs(t, e), "revoked"); len(revoked) != 0 {
		t.Fatalf("a retry revoked the grant: %v", revoked)
	}
}

func TestATokenForAnotherResourceIsRefused(t *testing.T) {
	e := newOAuthEnv(t)
	client := e.publicClient(t, "ChatGPT", chatgptReturn)

	m := e.consent(t, authorizeQuery(client, chatgptReturn, "resource", "https://other.example/mcp"))
	if m["ok"] != false || m["code"] != "oauth-invalid-request" || !strings.Contains(str(m, "error"), oauthResource) {
		t.Fatalf("authorizing another resource gave %v", m)
	}

	code := e.approve(t, authorizeQuery(client, chatgptReturn), false).Query().Get("code")
	form := codeExchange(client, code, chatgptReturn)
	form.Set("resource", "https://other.example/mcp")
	if w, m := e.token(t, form, "", ""); w.Code != http.StatusBadRequest || m["error"] != "invalid_target" {
		t.Fatalf("exchanging for another resource: %d %v", w.Code, m)
	}

	_, tok := e.token(t, codeExchange(client, code, chatgptReturn), "", "")
	_, res := doMCPKeyJSON(t, e.h, http.MethodPut, "/api/mcp/oauth",
		`{"enabled":true,"issuer":"https://elsewhere.example"}`, oauthHost, e.cookie)
	if res["ok"] != true {
		t.Fatalf("moving the issuer: %v", res)
	}
	if got := mcpStatus(t, e.h, str(tok, "access_token")); got != http.StatusUnauthorized {
		t.Fatalf("a token issued for the old address still opens /mcp: %d", got)
	}
}

func TestNoTokensAreIssuedForAnAddressThatMoved(t *testing.T) {
	e := newOAuthEnv(t)
	client, tok := e.signIn(t, false)
	pending := e.publicClient(t, "ChatGPT", chatgptReturn)
	code := e.approve(t, authorizeQuery(pending, chatgptReturn), false).Query().Get("code")
	if _, m := doMCPKeyJSON(t, e.h, http.MethodPut, "/api/mcp/oauth",
		`{"enabled":true,"issuer":"https://elsewhere.example"}`, oauthHost, e.cookie); m["ok"] != true {
		t.Fatalf("moving the issuer: %v", m)
	}
	refresh := url.Values{"grant_type": {"refresh_token"}, "client_id": {client}, "refresh_token": {str(tok, "refresh_token")}}
	if w, m := e.token(t, refresh, "", ""); w.Code != http.StatusBadRequest || m["error"] != "invalid_grant" {
		t.Fatalf("a refresh of a grant for the old address: %d %v", w.Code, m)
	}
	exchange := codeExchange(pending, code, chatgptReturn)
	exchange.Del("resource")
	if w, m := e.token(t, exchange, "", ""); w.Code != http.StatusBadRequest || m["error"] != "invalid_grant" {
		t.Fatalf("a code allowed for the old address: %d %v", w.Code, m)
	}
}

func TestSwitchingOAuthOffEndsEveryGrant(t *testing.T) {
	e := newOAuthEnv(t)
	client, tok := e.signIn(t, false)
	for _, enabled := range []bool{false, true} {
		if _, m := doMCPKeyJSON(t, e.h, http.MethodPut, "/api/mcp/oauth",
			fmt.Sprintf(`{"enabled":%t,"issuer":%q}`, enabled, oauthIssuer), oauthHost, e.cookie); m["ok"] != true {
			t.Fatalf("switch enabled=%t: %v", enabled, m)
		}
	}
	if got := mcpStatus(t, e.h, str(tok, "access_token")); got != http.StatusUnauthorized {
		t.Fatalf("an access token from before the switch-off: %d", got)
	}
	refresh := url.Values{"grant_type": {"refresh_token"}, "client_id": {client}, "refresh_token": {str(tok, "refresh_token")}}
	if w, m := e.token(t, refresh, "", ""); w.Code != http.StatusBadRequest || m["error"] != "invalid_grant" {
		t.Fatalf("a refresh token from before the switch-off: %d %v", w.Code, m)
	}
	revoked := mcpKeyRows(t, mcpKeyListAs(t, e), "revoked")
	if len(revoked) != 1 || revoked[0]["revokedReason"] != "oauth-off" {
		t.Fatalf("revoked rows %v", revoked)
	}
}

func TestMovingThePublicAddressEndsEveryGrant(t *testing.T) {
	e := newOAuthEnv(t)
	e.signIn(t, false)
	w, m := doMCPKeyJSON(t, e.h, http.MethodPost, "/api/mcp/keys", `{"label":"Laptop","canStartBackups":false}`, oauthHost, e.cookie)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("create key: %d %v", w.Code, m)
	}
	if _, m := doMCPKeyJSON(t, e.h, http.MethodPut, "/api/mcp/oauth",
		`{"enabled":true,"issuer":"https://elsewhere.example"}`, oauthHost, e.cookie); m["ok"] != true {
		t.Fatalf("moving the issuer: %v", m)
	}
	list := mcpKeyListAs(t, e)
	if revoked := mcpKeyRows(t, list, "revoked"); len(revoked) != 1 || revoked[0]["revokedReason"] != "oauth-moved" {
		t.Fatalf("revoked rows %v", revoked)
	}
	if keys := mcpKeyRows(t, list, "keys"); len(keys) != 1 || keys[0]["viaOAuth"] == true {
		t.Fatalf("the key went with the grants: %v", keys)
	}
}

func TestARevokedGrantStopsAtOnce(t *testing.T) {
	e := newOAuthEnv(t)
	client, tok := e.signIn(t, false)
	id := str(mcpKeyRows(t, mcpKeyListAs(t, e), "keys")[0], "id")
	if w, m := doMCPKeyJSON(t, e.h, http.MethodPost, "/api/mcp/keys/"+id+"/revoke", "", oauthHost, e.cookie); m["ok"] != true {
		t.Fatalf("revoke: %d %v", w.Code, m)
	}
	if got := mcpStatus(t, e.h, str(tok, "access_token")); got != http.StatusUnauthorized {
		t.Fatalf("access after revoke: %d", got)
	}
	refresh := url.Values{"grant_type": {"refresh_token"}, "client_id": {client}, "refresh_token": {str(tok, "refresh_token")}}
	if w, m := e.token(t, refresh, "", ""); w.Code != http.StatusBadRequest || m["error"] != "invalid_grant" {
		t.Fatalf("refresh after revoke: %d %v", w.Code, m)
	}
}

func TestTheClientCanRevokeItsOwnGrant(t *testing.T) {
	e := newOAuthEnv(t)
	client, tok := e.signIn(t, false)
	r := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(url.Values{
		"token": {str(tok, "refresh_token")}, "client_id": {client},
	}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("revocation: %d %s", w.Code, w.Body.String())
	}
	if got := mcpStatus(t, e.h, str(tok, "access_token")); got != http.StatusUnauthorized {
		t.Fatalf("access after the client revoked: %d", got)
	}
}

func TestDenyReturnsAccessDenied(t *testing.T) {
	e := newOAuthEnv(t)
	client := e.publicClient(t, "ChatGPT", chatgptReturn)
	m := e.consent(t, authorizeQuery(client, chatgptReturn))
	res := e.decide(t, str(m, "ticket"), false, false)
	back := mustURL(t, res["redirect"])
	if back.Query().Get("error") != "access_denied" || back.Query().Get("state") != "st-123" || back.Query().Get("iss") != oauthIssuer {
		t.Fatalf("deny redirected to %s", back)
	}
	if rows := mcpKeyRows(t, mcpKeyListAs(t, e), "keys"); len(rows) != 0 {
		t.Fatalf("a denied request left a grant: %v", rows)
	}
}

func TestAuthorizeRequiresPKCEWithS256(t *testing.T) {
	e := newOAuthEnv(t)
	client := e.publicClient(t, "ChatGPT", chatgptReturn)
	for name, q := range map[string]url.Values{
		"no challenge": authorizeQuery(client, chatgptReturn, "code_challenge", ""),
		"plain":        authorizeQuery(client, chatgptReturn, "code_challenge_method", "plain"),
		"no method":    authorizeQuery(client, chatgptReturn, "code_challenge_method", ""),
		"token flow":   authorizeQuery(client, chatgptReturn, "response_type", "token"),
	} {
		if m := e.consent(t, q); m["ok"] != false || m["code"] != "oauth-invalid-request" || m["ticket"] != nil {
			t.Errorf("%s: %v", name, m)
		}
	}
	if m := e.consent(t, authorizeQuery("bvc_unknown", chatgptReturn)); m["code"] != "oauth-unknown-client" || m["redirect"] != nil {
		t.Errorf("an unknown client gave %v", m)
	}
}

// Anybody can register a client with any https return address, so a request
// the server refuses must not send the operator there: the sign-in page would
// be a redirector that works right after the operator logged in.
func TestARefusedAuthorizationRequestStaysOnTheSignInPage(t *testing.T) {
	e := newOAuthEnv(t)
	trap := "https://attacker.example/fake-login"
	client := e.publicClient(t, "Anything", trap)
	for name, q := range map[string]url.Values{
		"token flow":     authorizeQuery(client, trap, "response_type", "token"),
		"no challenge":   authorizeQuery(client, trap, "code_challenge", ""),
		"other resource": authorizeQuery(client, trap, "resource", "https://other.example/mcp"),
	} {
		m := e.consent(t, q)
		if m["ok"] != false || m["code"] != "oauth-invalid-request" {
			t.Errorf("%s: %v", name, m)
		}
		if _, sent := m["redirect"]; sent {
			t.Errorf("%s: the refusal carries a redirect to the client: %v", name, m["redirect"])
		}
	}
}

func TestConsentIsProtectedAgainstForgedRequests(t *testing.T) {
	e := newOAuthEnv(t)
	client := e.publicClient(t, "ChatGPT", chatgptReturn)
	q := authorizeQuery(client, chatgptReturn)

	if w, _ := doMCPKeyJSON(t, e.h, http.MethodGet, "/api/oauth/authorize?"+q.Encode(), "", oauthHost, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("consent data without a session: %d", w.Code)
	}

	ticket := str(e.consent(t, q), "ticket")
	body := fmt.Sprintf(`{"ticket":%q,"allow":true,"canStartBackups":true}`, ticket)

	r := httptest.NewRequest(http.MethodPost, "/api/oauth/authorize", strings.NewReader(body))
	r.Host = oauthHost
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	r.AddCookie(&http.Cookie{Name: "bv_session", Value: e.cookie}) //nolint:gosec // G124: request cookie
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("a cross-site POST gave %d", w.Code)
	}

	other := secret.NewSessionToken(mcpAppKey, mustSettings(t, e.st).AuthPasswordHash, mustSettings(t, e.st).SessionEpoch, 3600*1e9)
	_, m := doMCPKeyJSON(t, e.h, http.MethodPost, "/api/oauth/authorize", body, oauthHost, other)
	if m["ok"] != false || m["code"] != "oauth-consent-expired" {
		t.Fatalf("a ticket from another session gave %v", m)
	}

	parts := strings.SplitN(ticket, ".", 2)
	payload, _ := base64.RawURLEncoding.DecodeString(parts[0])
	forged := strings.Replace(string(payload), chatgptReturn, "https://chatgpt.com/stolen", 1)
	forgedTicket := base64.RawURLEncoding.EncodeToString([]byte(forged)) + "." + parts[1]
	if m := e.decide(t, forgedTicket, true, true); m["ok"] != false || m["code"] != "oauth-consent-expired" {
		t.Fatalf("an altered ticket gave %v", m)
	}

	if m := e.decide(t, ticket, true, false); m["ok"] != true {
		t.Fatalf("the genuine ticket: %v", m)
	}
	if m := e.decide(t, ticket, true, false); m["ok"] != false || m["code"] != "oauth-consent-expired" {
		t.Fatalf("a ticket used twice gave %v", m)
	}
}

func TestAConfidentialClientMustAuthenticate(t *testing.T) {
	e := newOAuthEnv(t)
	w, reg := e.register(t, `{"client_name":"Server app","redirect_uris":["`+chatgptReturn+`"],"grant_types":["authorization_code","refresh_token"]}`, "")
	if w.Code != http.StatusCreated || reg["token_endpoint_auth_method"] != "client_secret_basic" ||
		!strings.HasPrefix(str(reg, "client_secret"), secret.OAuthSecretPrefix) {
		t.Fatalf("a registration without a method must default to client_secret_basic: %d %v", w.Code, reg)
	}
	client, clientSecret := str(reg, "client_id"), str(reg, "client_secret")
	code := e.approve(t, authorizeQuery(client, chatgptReturn), false).Query().Get("code")

	if w, m := e.token(t, codeExchange(client, code, chatgptReturn), "", ""); w.Code != http.StatusUnauthorized || m["error"] != "invalid_client" {
		t.Fatalf("no secret: %d %v", w.Code, m)
	}
	if w, m := e.token(t, codeExchange(client, code, chatgptReturn), client, "bvcs_wrong"); w.Code != http.StatusUnauthorized || m["error"] != "invalid_client" {
		t.Fatalf("wrong secret: %d %v", w.Code, m)
	}
	if w, m := e.token(t, codeExchange(client, code, chatgptReturn), client, clientSecret); w.Code != http.StatusOK {
		t.Fatalf("right secret: %d %v", w.Code, m)
	}
}

func TestAnAppKeyChangeMarksTheGrantUnusable(t *testing.T) {
	e := newOAuthEnv(t)
	_, tok := e.signIn(t, false)
	h2, _ := newMCPKeyRouter(t, e.st, mcpAppKeyAfter)
	cookie := enableLogin(t, e.st, mcpAppKeyAfter)
	if got := mcpStatus(t, h2, str(tok, "access_token")); got != http.StatusUnauthorized {
		t.Fatalf("a token under the old APP_KEY: %d", got)
	}
	_, m := doMCPKeyJSON(t, h2, http.MethodGet, "/api/mcp/keys", "", oauthHost, cookie)
	rows := mcpKeyRows(t, m, "keys")
	if len(rows) != 1 || rows[0]["unusable"] != "app-key-changed" {
		t.Fatalf("grant after the APP_KEY change %v", rows)
	}
}

func TestDiagnosticsCountGrantsWithoutTheirTokensOrTheAddress(t *testing.T) {
	prev := log.Writer()
	log.SetOutput(logring.Default.Tee(io.Discard))
	t.Cleanup(func() { log.SetOutput(prev) })
	e := newOAuthEnv(t)
	_, tok := e.signIn(t, false)
	w := getRaw(t, e.h, "/api/diagnostics", &http.Cookie{Name: "bv_session", Value: e.cookie}) //nolint:gosec // G124: request cookie
	if w.Code != http.StatusOK {
		t.Fatalf("diagnostics: %d %s", w.Code, w.Body.String())
	}
	members := zipMembers(t, w.Body.Bytes())
	var manifest struct {
		MCP struct {
			ActiveKeys        int  `json:"activeKeys"`
			OAuthOn           bool `json:"oauthOn"`
			ActiveGrants      int  `json:"activeGrants"`
			RegisteredClients int  `json:"registeredClients"`
		} `json:"mcp"`
	}
	if err := json.Unmarshal([]byte(members["manifest.json"]), &manifest); err != nil {
		t.Fatal(err)
	}
	if !manifest.MCP.OAuthOn || manifest.MCP.ActiveGrants != 1 || manifest.MCP.RegisteredClients != 1 || manifest.MCP.ActiveKeys != 0 {
		t.Fatalf("mcp counts %+v", manifest.MCP)
	}
	if !strings.Contains(members["log.txt"], "granted") {
		t.Fatalf("the log did not capture the grant, so the check below proves nothing: %q", members["log.txt"])
	}
	for name, body := range members {
		for _, s := range []string{oauthHost, str(tok, "access_token"), str(tok, "refresh_token")} {
			if strings.Contains(body, s) {
				t.Errorf("%s carries %q", name, s)
			}
		}
	}
}
