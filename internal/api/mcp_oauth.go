package api

import (
	"errors"
	"log"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// BombVault is the authorization server of its own MCP endpoint, so clients
// that can only sign in through OAuth reach it too. It follows the MCP
// authorization specification: protected resource metadata (RFC 9728),
// authorization server metadata (RFC 8414), dynamic client registration
// (RFC 7591), the authorization code grant with PKCE S256, resource
// indicators (RFC 8707), the iss response parameter (RFC 9207) and token
// revocation (RFC 7009).
const (
	oauthScope = "mcp"

	oauthAuthorizePath  = "/oauth/authorize"
	oauthTokenPath      = "/oauth/token" //nolint:gosec // G101: a URL path, not a credential
	oauthRegisterPath   = "/oauth/register"
	oauthRevokePath     = "/oauth/revoke"
	oauthServerMetaPath = "/.well-known/oauth-authorization-server"
	oauthResourceMeta   = "/.well-known/oauth-protected-resource" + mcpEndpointPath

	oauthAccessTTL  = time.Hour
	oauthRefreshTTL = 30 * 24 * time.Hour
	oauthCodeTTL    = 2 * time.Minute
	oauthConsentTTL = 10 * time.Minute

	// A code waits here between the consent and the client's exchange, which
	// takes seconds. Only a signed-in operator can add one, so the cap is
	// about a stuck client, not about an attacker.
	oauthPendingMax = 64

	// Registration needs no credentials. Per address, or per /64 for IPv6, it
	// is held to a few an hour; the store caps the clients nobody signed in with.
	oauthRegistrationsPerHour = 10
	oauthRegistrationAddrs    = 4096

	oauthFormMax = 16 << 10
)

var (
	errMCPOAuthNeedsPassword = errors.New("set a login password before switching on sign-in through OAuth")
	errMCPOAuthIssuer        = errors.New("the public address must be an https address without a path, such as https://backup.example.com")
	errOAuthOff              = errors.New("sign-in through OAuth is not switched on")
	errOAuthUnknownClient    = errors.New("this client is not registered here, remove BombVault from the client and add it again")
	errOAuthBadRedirect      = errors.New("the return address of this request is not one the client registered")
	errOAuthConsentExpired   = errors.New("this page has expired, start the sign-in again from the client")
	errOAuthBusy             = errors.New("too many sign-ins are waiting, try again in a minute")
)

// oauthState is what the authorization server keeps between requests. Codes
// and spent consent tickets live only here: a restart ends a sign-in half way,
// which the client simply starts again.
type oauthState struct {
	mu      sync.Mutex
	codes   map[string]*oauthCode
	tickets map[string]int64

	registrations *slidingWindow
}

// oauthCode is a code handed to a client after the operator allowed it,
// keyed by its digest. used stays set until it expires, so a code presented
// twice is recognised and the grant it made revoked.
type oauthCode struct {
	clientID    string
	redirectURI string
	challenge   string
	resource    string
	label       string
	known       string
	canStart    bool
	expires     time.Time
	used        bool
	grantID     string
}

func newOAuthState() *oauthState {
	reg := newSlidingWindow(time.Hour, oauthRegistrationsPerHour)
	reg.maxKeys = oauthRegistrationAddrs
	return &oauthState{
		codes:         map[string]*oauthCode{},
		tickets:       map[string]int64{},
		registrations: reg,
	}
}

// registrationKey is the bucket a registration from addr counts in. An IPv6
// host usually gets a whole /64, so its addresses share one bucket.
func registrationKey(addr string) string {
	ip := net.ParseIP(addr)
	if ip == nil {
		return addr
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	return (&net.IPNet{IP: ip.Mask(net.CIDRMask(64, 128)), Mask: net.CIDRMask(64, 128)}).String()
}

// oauthKnownClients are the clients the card and the consent page show with
// their own mark. A client counts as one of them only when every redirect URI
// it registered is the vendor's own callback, because a name is whatever the
// caller of the registration endpoint typed, and a vendor's domain also
// serves pages its users write.
var oauthKnownClients = []struct {
	id        string
	name      string
	callbacks []string
}{
	{"chatgpt", "ChatGPT", []string{"https://chatgpt.com/connector_platform_oauth_redirect"}},
	{"claudeai", "Claude", []string{"https://claude.ai/api/mcp/auth_callback", "https://claude.com/api/mcp/auth_callback"}},
}

func knownOAuthClient(uris []string) string {
	found := ""
	for _, uri := range uris {
		id := ""
		for _, k := range oauthKnownClients {
			if slices.Contains(k.callbacks, uri) {
				id = k.id
			}
		}
		if id == "" || (found != "" && found != id) {
			return ""
		}
		found = id
	}
	return found
}

func knownOAuthClientName(id string) string {
	for _, k := range oauthKnownClients {
		if k.id == id {
			return k.name
		}
	}
	return ""
}

// oauthIssuer returns the issuer while OAuth is offered: the switch is on, a
// public address is stored and a login password is set. Without the password
// nobody could be asked for consent, so the whole authorization server is off.
func (h *Handler) oauthIssuer() (string, bool) {
	if _, _, on := h.authEnabled(); !on {
		return "", false
	}
	s, err := h.store.MCPOAuthSettings()
	if err != nil {
		log.Printf("api: mcp: oauth: could not read the settings: %v", err)
		return "", false
	}
	return s.Issuer, s.Enabled && s.Issuer != ""
}

// oauthResource is the canonical URI of the MCP endpoint under issuer, the
// audience of every token.
func oauthResource(issuer string) string {
	return issuer + mcpEndpointPath
}

// normalizeIssuer turns what the operator typed into the form the issuer is
// compared in: https, a lower-case host, no default port, and nothing after
// the host. Clients compare the issuer as a plain string, so there is exactly
// one spelling of it.
func normalizeIssuer(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || strings.ContainsAny(raw, "?#") || !strings.EqualFold(u.Scheme, "https") ||
		u.Opaque != "" || u.User != nil || (u.Path != "" && u.Path != "/") {
		return "", false
	}
	host, port := strings.ToLower(u.Hostname()), u.Port()
	if host == "" {
		return "", false
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port != "" && port != "443" {
		host += ":" + port
	}
	return "https://" + host, true
}

// sameResource reports whether a resource parameter names want, allowing for
// the spelling differences RFC 8707 clients are free to make: the case of the
// scheme and host, a default port and a trailing slash.
func sameResource(given, want string) bool {
	u, err := url.Parse(given)
	if err != nil || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return false
	}
	base, ok := normalizeIssuer(u.Scheme + "://" + u.Host)
	return ok && base+strings.TrimSuffix(u.Path, "/") == want
}

// validRedirectURI reports whether a client may register uri: https with a
// host, or plain http to a loopback address for a client on the user's own
// machine, never with a fragment or credentials. The host has to be plain
// ASCII, punycode for an international name, so the consent page never shows
// a lookalike of a familiar one.
func validRedirectURI(uri string) bool {
	if len(uri) > 512 || strings.Contains(uri, "#") {
		return false
	}
	u, err := url.Parse(uri)
	if err != nil || u.Opaque != "" || u.User != nil || u.Host == "" || u.Hostname() == "" ||
		strings.IndexFunc(u.Host, func(r rune) bool { return r > unicode.MaxASCII || r == '%' }) >= 0 {
		return false
	}
	switch u.Scheme {
	case "https":
		return true
	case "http":
		return loopbackHost(u.Hostname())
	}
	return false
}

func loopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// redirectRegistered reports whether given is one of the registered redirect
// URIs. The match is exact, except that a loopback redirect may name any port:
// a native client listens on whatever port is free (RFC 8252 section 7.3).
func redirectRegistered(registered []string, given string) bool {
	for _, r := range registered {
		if r == given || loopbackMatch(r, given) {
			return true
		}
	}
	return false
}

func loopbackMatch(registered, given string) bool {
	a, errA := url.Parse(registered)
	b, errB := url.Parse(given)
	if errA != nil || errB != nil || a.Scheme != "http" || b.Scheme != "http" || !loopbackHost(a.Hostname()) {
		return false
	}
	return a.Hostname() == b.Hostname() && a.Path == b.Path && a.RawQuery == b.RawQuery &&
		b.User == nil && !strings.Contains(given, "#")
}

// cleanClientName keeps what a client calls itself to 64 printable characters,
// because the consent page and the tile show it. Format characters go too:
// they include the ones that reorder text or join letters invisibly.
func cleanClientName(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == utf8.RuneError {
			return -1
		}
		return r
	}, strings.TrimSpace(s))
	if utf8.RuneCountInString(s) > 64 {
		s = string([]rune(s)[:64])
	}
	return strings.TrimSpace(s)
}

// mcpChallenge is the WWW-Authenticate value of a 401 from /mcp. While OAuth
// is offered it names the protected resource metadata, which is how a client
// learns that it can sign in.
func mcpChallenge(issuer string, oauth bool) string {
	if !oauth {
		return mcpAuthRealm
	}
	return mcpAuthRealm + `, resource_metadata="` + issuer + oauthResourceMeta + `", scope="` + oauthScope + `"`
}

// oauthGrantFor returns the grant behind an access token. The token must have
// been issued for this server under its current address: a token for another
// audience is refused like an unknown one.
func (h *Handler) oauthGrantFor(token, issuer string, now time.Time) (store.MCPKey, bool) {
	g, err := h.store.OAuthAccessGrant(secret.HashOAuthSecret(h.cfg.AppKey, "access", token), now.Unix())
	if err != nil {
		if !errors.Is(err, store.ErrOAuthTokenNotFound) {
			log.Printf("api: mcp: oauth: could not look up an access token: %v", err)
		}
		return store.MCPKey{}, false
	}
	return g, g.Resource == oauthResource(issuer)
}

type mcpOAuthView struct {
	Enabled       bool   `json:"enabled"`
	Issuer        string `json:"issuer"`
	Active        bool   `json:"active"`
	GrantLimit    int    `json:"grantLimit"`
	ConnectorPath string `json:"connectorPath"`
}

func (h *Handler) mcpOAuthView() (mcpOAuthView, error) {
	s, err := h.store.MCPOAuthSettings()
	if err != nil {
		return mcpOAuthView{}, err
	}
	_, active := h.oauthIssuer()
	return mcpOAuthView{
		Enabled:       s.Enabled,
		Issuer:        s.Issuer,
		Active:        active,
		GrantLimit:    store.MCPGrantLimit,
		ConnectorPath: mcpEndpointPath,
	}, nil
}

// handleSetMCPOAuth stores the switch and the public address. Switching on
// needs a login password, because the consent page is where the operator
// signs in, and it will face the internet.
func (h *Handler) handleSetMCPOAuth(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled bool   `json:"enabled"`
		Issuer  string `json:"issuer"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if _, _, on := h.authEnabled(); body.Enabled && !on {
		writeJSON(w, http.StatusOK, codedFailEnvelope(errMCPOAuthNeedsPassword, "mcp-oauth-needs-password"))
		return
	}
	issuer := ""
	if body.Enabled || strings.TrimSpace(body.Issuer) != "" {
		var ok bool
		if issuer, ok = normalizeIssuer(body.Issuer); !ok {
			writeJSON(w, http.StatusOK, codedFailEnvelope(errMCPOAuthIssuer, "mcp-oauth-issuer-invalid"))
			return
		}
	}
	before, err := h.store.MCPOAuthSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	now := h.mcp.now().Unix()
	if err := h.store.SetMCPOAuthSettings(store.MCPOAuthSettings{Enabled: body.Enabled, Issuer: issuer}, now); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	state := "off"
	if body.Enabled {
		state = "on"
	}
	log.Printf("api: mcp: sign-in through OAuth switched %s", state)

	// Switching off is how the operator cuts every cloud client off, so the
	// grants end here instead of waking up again with the switch. A new
	// address leaves them with tokens nothing accepts any more.
	reason, why := "", ""
	switch {
	case !body.Enabled:
		reason, why = "oauth-off", "revoked when sign-in through OAuth was switched off"
	case issuer != before.Issuer:
		reason, why = "oauth-moved", "revoked when the public address changed"
	}
	if reason != "" {
		rows, err := h.store.RevokeOAuthGrants(reason, now)
		if err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
		for _, k := range rows {
			h.recordMCPKeyChange(r, k, why, "")
		}
	}
	view, err := h.mcpOAuthView()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"oauth": view}))
}

// handleOAuthProtectedResource serves the RFC 9728 metadata of /mcp. It is
// only served under the path of the endpoint: RFC 9728 has the resource match
// the address the document was fetched from, and the 401 names this one.
func (h *Handler) handleOAuthProtectedResource(w http.ResponseWriter, r *http.Request) {
	issuer, on := h.oauthIssuer()
	if !on {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"resource":                 oauthResource(issuer),
		"authorization_servers":    []string{issuer},
		"scopes_supported":         []string{oauthScope},
		"bearer_methods_supported": []string{"header"},
		"resource_name":            "BombVault",
	})
}

// handleOAuthServerMetadata serves the RFC 8414 metadata of the authorization
// server.
func (h *Handler) handleOAuthServerMetadata(w http.ResponseWriter, r *http.Request) {
	issuer, on := h.oauthIssuer()
	if !on {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                         issuer,
		"authorization_endpoint":                         issuer + oauthAuthorizePath,
		"token_endpoint":                                 issuer + oauthTokenPath,
		"registration_endpoint":                          issuer + oauthRegisterPath,
		"revocation_endpoint":                            issuer + oauthRevokePath,
		"scopes_supported":                               []string{oauthScope},
		"response_types_supported":                       []string{"code"},
		"response_modes_supported":                       []string{"query"},
		"grant_types_supported":                          []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":               []string{"S256"},
		"token_endpoint_auth_methods_supported":          oauthAuthMethods,
		"revocation_endpoint_auth_methods_supported":     oauthAuthMethods,
		"authorization_response_iss_parameter_supported": true,
	})
}

// oauthAuthMethods are the ways a client can authenticate at the token and
// revocation endpoints. Cloud connectors register as public clients and rely
// on PKCE; a client that asks for a secret gets one.
var oauthAuthMethods = []string{"none", "client_secret_basic", "client_secret_post"}
