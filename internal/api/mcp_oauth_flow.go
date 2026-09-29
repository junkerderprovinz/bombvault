package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// oauthError answers a token, registration or revocation request with the
// error object of RFC 6749 section 5.2.
func oauthError(w http.ResponseWriter, status int, code, description string) {
	w.Header().Set("Cache-Control", "no-store")
	if status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", `Basic realm="bombvault"`)
	}
	writeJSON(w, status, map[string]any{"error": code, "error_description": description})
}

// handleOAuthRegister is dynamic client registration (RFC 7591). Anybody who
// reaches the address may register, so a registration grants nothing: the
// operator still has to sign in and allow the client.
func (h *Handler) handleOAuthRegister(w http.ResponseWriter, r *http.Request) {
	if _, on := h.oauthIssuer(); !on {
		http.NotFound(w, r)
		return
	}
	now := h.mcp.now()
	addr := h.loginClientKey(r)
	if ok, retry := h.mcp.oauth.registrations.allow(registrationKey(addr), now); !ok {
		w.Header().Set("Retry-After", retryAfterSeconds(retry))
		oauthError(w, http.StatusTooManyRequests, "invalid_request", "too many registrations from this address, try again later")
		return
	}
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "the registration must be sent as application/json")
		return
	}
	var body struct {
		RedirectURIs []string `json:"redirect_uris"`
		ClientName   string   `json:"client_name"`
		GrantTypes   []string `json:"grant_types"`
		Response     []string `json:"response_types"`
		AuthMethod   *string  `json:"token_endpoint_auth_method"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, oauthFormMax)).Decode(&body); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "the registration is not valid JSON")
		return
	}
	if len(body.RedirectURIs) == 0 || len(body.RedirectURIs) > 5 {
		oauthError(w, http.StatusBadRequest, "invalid_redirect_uri", "register between one and five redirect URIs")
		return
	}
	for _, uri := range body.RedirectURIs {
		if !validRedirectURI(uri) {
			oauthError(w, http.StatusBadRequest, "invalid_redirect_uri", "a redirect URI must use https, or http to a loopback address, name its host in ASCII (punycode for an international name) and carry no fragment")
			return
		}
	}
	for _, g := range body.GrantTypes {
		if g != "authorization_code" && g != "refresh_token" {
			oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "only the authorization_code and refresh_token grants are supported")
			return
		}
	}
	for _, rt := range body.Response {
		if rt != "code" {
			oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "only the code response type is supported")
			return
		}
	}
	// RFC 7591 section 2 makes client_secret_basic the method of a client
	// that names none.
	method := "client_secret_basic"
	if body.AuthMethod != nil {
		method = *body.AuthMethod
	}
	if !slices.Contains(oauthAuthMethods, method) {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "token_endpoint_auth_method must be none, client_secret_basic or client_secret_post")
		return
	}

	id, err := secret.NewOAuthSecret(secret.OAuthClientPrefix)
	if err != nil {
		oauthError(w, http.StatusInternalServerError, "server_error", "could not create the client")
		return
	}
	c := store.OAuthClient{
		ID:           id,
		Name:         cleanClientName(body.ClientName),
		RedirectURIs: body.RedirectURIs,
		AuthMethod:   method,
		Known:        knownOAuthClient(body.RedirectURIs),
		CreatedFrom:  addr,
	}
	var clientSecret string
	if method != "none" {
		if clientSecret, err = secret.NewOAuthSecret(secret.OAuthSecretPrefix); err != nil {
			oauthError(w, http.StatusInternalServerError, "server_error", "could not create the client")
			return
		}
		c.SecretDigest = secret.HashOAuthSecret(h.cfg.AppKey, "client", clientSecret)
	}
	if err := h.store.CreateOAuthClient(c, now.Unix()); err != nil {
		log.Printf("api: mcp: oauth: could not store a registration: %v", err)
		oauthError(w, http.StatusInternalServerError, "server_error", "could not store the client")
		return
	}
	log.Printf("api: mcp: oauth: client %s registered from %s", id, addr)

	out := map[string]any{
		"client_id":                  id,
		"client_id_issued_at":        now.Unix(),
		"client_name":                c.Name,
		"redirect_uris":              c.RedirectURIs,
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": method,
	}
	if clientSecret != "" {
		out["client_secret"] = clientSecret
		out["client_secret_expires_at"] = 0
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, out)
}

// consentTicket is what the consent page gets to answer with: the request it
// showed, signed together with the session cookie of the operator who saw it.
// A page on another site has neither the cookie nor the ticket, and the
// ticket cannot be moved to another session or another request.
type consentTicket struct {
	Client    string `json:"c"`
	Redirect  string `json:"r"`
	Challenge string `json:"x"`
	Resource  string `json:"s"`
	State     string `json:"t"`
	Expires   int64  `json:"e"`
}

func (h *Handler) consentMAC(session, payload string) string {
	mac := hmac.New(sha256.New, []byte(h.cfg.AppKey))
	mac.Write([]byte("bombvault-oauth-consent:" + session + "." + payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func (h *Handler) signConsent(session string, t consentTicket) string {
	raw, _ := json.Marshal(t) //nolint:errchkjson // a struct of strings and an int always marshals
	payload := base64.RawURLEncoding.EncodeToString(raw)
	return payload + "." + h.consentMAC(session, payload)
}

// openConsent verifies a ticket against the session that sent it and spends
// it, so an answer cannot be sent twice.
func (h *Handler) openConsent(session, ticket string, now time.Time) (consentTicket, bool) {
	payload, mac, found := strings.Cut(ticket, ".")
	if !found || subtle.ConstantTimeCompare([]byte(mac), []byte(h.consentMAC(session, payload))) != 1 {
		return consentTicket{}, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	var t consentTicket
	if err != nil || json.Unmarshal(raw, &t) != nil || t.Expires <= now.Unix() {
		return consentTicket{}, false
	}
	s := h.mcp.oauth
	s.mu.Lock()
	defer s.mu.Unlock()
	for m, exp := range s.tickets {
		if exp <= now.Unix() {
			delete(s.tickets, m)
		}
	}
	if _, spent := s.tickets[mac]; spent {
		return consentTicket{}, false
	}
	s.tickets[mac] = t.Expires
	return t, true
}

// operatorSession returns the session cookie of a signed-in operator. authGate
// has checked it already; the value is what a consent ticket is bound to.
func (h *Handler) operatorSession(r *http.Request) (string, bool) {
	hash, epoch, on := h.authEnabled()
	c, err := h.sessionCookie(r)
	if !on || err != nil || !secret.ValidSessionToken(h.cfg.AppKey, hash, epoch, c.Value) {
		return "", false
	}
	return c.Value, true
}

// withQuery adds v to the query of a redirect URI.
func withQuery(uri string, v url.Values) string {
	u, err := url.Parse(uri)
	if err != nil {
		return uri
	}
	q := u.Query()
	for k, vals := range v {
		q[k] = vals
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// authorizationResponse is what goes back to the client: the result plus the
// state it sent and the issuer, so it can tell which server answered.
func authorizationResponse(redirect, issuer, state string, v url.Values) string {
	v.Set("iss", issuer)
	if state != "" {
		v.Set("state", state)
	}
	return withQuery(redirect, v)
}

// validChallenge reports whether c has the shape of an S256 code challenge:
// 43 characters of unpadded base64url.
func validChallenge(c string) bool {
	if len(c) != 43 {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(c)
	return err == nil
}

func (h *Handler) activeGrantsExcept(client string) (int, error) {
	rows, err := h.store.ListMCPKeys()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, k := range rows {
		if k.Kind == store.MCPKindOAuth && k.RevokedAt == 0 && k.OAuthClient != client {
			n++
		}
	}
	return n, nil
}

// handleOAuthConsentInfo checks an authorization request for the consent page
// and hands it a ticket to answer with. Every fault is shown to the operator
// and none goes back to the client: anybody can register a client with any
// https return address, and sending a refusal there would make the sign-in
// page an open redirector (RFC 9700 section 4.11.2).
func (h *Handler) handleOAuthConsentInfo(w http.ResponseWriter, r *http.Request) {
	issuer, on := h.oauthIssuer()
	if !on {
		writeJSON(w, http.StatusOK, codedFailEnvelope(errOAuthOff, "oauth-off"))
		return
	}
	session, ok := h.operatorSession(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "authentication required"})
		return
	}
	q := r.URL.Query()
	client, err := h.store.GetOAuthClient(q.Get("client_id"))
	if errors.Is(err, store.ErrOAuthClientNotFound) {
		writeJSON(w, http.StatusOK, codedFailEnvelope(errOAuthUnknownClient, "oauth-unknown-client"))
		return
	}
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	redirect := q.Get("redirect_uri")
	if !redirectRegistered(client.RedirectURIs, redirect) {
		writeJSON(w, http.StatusOK, codedFailEnvelope(errOAuthBadRedirect, "oauth-bad-redirect"))
		return
	}

	refuse := func(description string) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "code": "oauth-invalid-request", "error": description})
	}
	switch {
	case q.Get("response_type") != "code":
		refuse("only the authorization code flow is supported")
		return
	case q.Get("code_challenge_method") != "S256" || !validChallenge(q.Get("code_challenge")):
		refuse("a PKCE code challenge with the S256 method is required")
		return
	case q.Get("resource") != "" && !sameResource(q.Get("resource"), oauthResource(issuer)):
		refuse("this server issues tokens for " + oauthResource(issuer) + " only")
		return
	}

	others, err := h.activeGrantsExcept(client.ID)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	u, _ := url.Parse(redirect)
	ticket := h.signConsent(session, consentTicket{
		Client:    client.ID,
		Redirect:  redirect,
		Challenge: q.Get("code_challenge"),
		Resource:  oauthResource(issuer),
		State:     q.Get("state"),
		Expires:   h.mcp.now().Add(oauthConsentTTL).Unix(),
	})
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"client": map[string]any{
			"id":           client.ID,
			"name":         client.Name,
			"known":        client.Known,
			"redirectHost": u.Hostname(),
			"loopback":     loopbackHost(u.Hostname()),
		},
		"ticket":       ticket,
		"limitReached": others >= store.MCPGrantLimit,
		"grantLimit":   store.MCPGrantLimit,
	}))
}

// handleOAuthConsent takes the operator's answer. Allow hands the client a
// code for the token endpoint; deny tells it access_denied.
func (h *Handler) handleOAuthConsent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Ticket          string `json:"ticket"`
		Allow           bool   `json:"allow"`
		CanStartBackups bool   `json:"canStartBackups"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	issuer, on := h.oauthIssuer()
	if !on {
		writeJSON(w, http.StatusOK, codedFailEnvelope(errOAuthOff, "oauth-off"))
		return
	}
	session, ok := h.operatorSession(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "authentication required"})
		return
	}
	now := h.mcp.now()
	t, ok := h.openConsent(session, body.Ticket, now)
	if !ok {
		writeJSON(w, http.StatusOK, codedFailEnvelope(errOAuthConsentExpired, "oauth-consent-expired"))
		return
	}
	client, err := h.store.GetOAuthClient(t.Client)
	if errors.Is(err, store.ErrOAuthClientNotFound) {
		writeJSON(w, http.StatusOK, codedFailEnvelope(errOAuthUnknownClient, "oauth-unknown-client"))
		return
	}
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	addr := h.loginClientKey(r)
	if !body.Allow {
		log.Printf("api: mcp: oauth: client %s was denied from %s", client.ID, addr)
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
			"redirect": authorizationResponse(t.Redirect, issuer, t.State, url.Values{"error": {"access_denied"}}),
		}))
		return
	}
	others, err := h.activeGrantsExcept(client.ID)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if others >= store.MCPGrantLimit {
		writeJSON(w, http.StatusOK, codedFailEnvelope(store.ErrMCPGrantLimit, "mcp-grant-limit"))
		return
	}

	code, err := secret.NewOAuthSecret(secret.OAuthCodePrefix)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	label := knownOAuthClientName(client.Known)
	if label == "" {
		label = client.Name
	}
	if label == "" {
		label = "OAuth client"
	}
	pending := &oauthCode{
		clientID:    client.ID,
		redirectURI: t.Redirect,
		challenge:   t.Challenge,
		resource:    t.Resource,
		label:       label,
		known:       client.Known,
		canStart:    body.CanStartBackups,
		expires:     now.Add(oauthCodeTTL),
	}
	if !h.mcp.oauth.putCode(secret.HashOAuthSecret(h.cfg.AppKey, "code", code), pending, now) {
		writeJSON(w, http.StatusOK, codedFailEnvelope(errOAuthBusy, "oauth-busy"))
		return
	}
	log.Printf("api: mcp: oauth: client %s was allowed from %s", client.ID, addr)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"redirect": authorizationResponse(t.Redirect, issuer, t.State, url.Values{"code": {code}}),
	}))
}

// putCode keeps a code until it expires, refusing a new one while too many
// are waiting.
func (s *oauthState) putCode(digest string, c *oauthCode, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for d, pending := range s.codes {
		if !now.Before(pending.expires) {
			delete(s.codes, d)
		}
	}
	if len(s.codes) >= oauthPendingMax {
		return false
	}
	s.codes[digest] = c
	return true
}

// redeem hands out a code once. A code that was already redeemed answers with
// the grant it made, which the caller revokes. A wrong client, redirect URI or
// verifier leaves the code as it was, so a forged exchange cannot spend it.
func (s *oauthState) redeem(digest, client, redirect, verifier string, now time.Time) (c *oauthCode, replayedGrant string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c = s.codes[digest]
	if c == nil {
		return nil, ""
	}
	if !now.Before(c.expires) {
		delete(s.codes, digest)
		return nil, ""
	}
	if c.used {
		return nil, c.grantID
	}
	if c.clientID != client || c.redirectURI != redirect ||
		!secret.ValidPKCEVerifier(verifier) || !secret.PKCEMatches(verifier, c.challenge) {
		return nil, ""
	}
	c.used = true
	c.grantID = newMCPKeyID()
	return c, ""
}

// oauthClientFrom authenticates the client of a token or revocation request:
// by client_id alone for a public client, and with its secret from the Basic
// header or the form otherwise.
func (h *Handler) oauthClientFrom(r *http.Request) (store.OAuthClient, bool) {
	id, sec := r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	if user, pass, basic := r.BasicAuth(); basic {
		u, errU := url.QueryUnescape(user)
		p, errP := url.QueryUnescape(pass)
		if errU != nil || errP != nil || (id != "" && id != u) {
			return store.OAuthClient{}, false
		}
		id, sec = u, p
	}
	if id == "" {
		return store.OAuthClient{}, false
	}
	c, err := h.store.GetOAuthClient(id)
	if err != nil {
		if !errors.Is(err, store.ErrOAuthClientNotFound) {
			log.Printf("api: mcp: oauth: could not read a client: %v", err)
		}
		return store.OAuthClient{}, false
	}
	if c.AuthMethod == "none" {
		return c, true
	}
	digest := secret.HashOAuthSecret(h.cfg.AppKey, "client", sec)
	return c, sec != "" && subtle.ConstantTimeCompare([]byte(digest), []byte(c.SecretDigest)) == 1
}

// oauthForm reads a form-encoded request body of the token or revocation
// endpoint, answering the refusal itself.
func oauthForm(w http.ResponseWriter, r *http.Request) bool {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/x-www-form-urlencoded" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "the request must be sent as application/x-www-form-urlencoded")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, oauthFormMax)
	if err := r.ParseForm(); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_request", "the request body could not be read")
		return false
	}
	return true
}

// handleOAuthToken is the token endpoint: a code for tokens, and a refresh
// token for new ones. Codes and tokens carry 256 bits, so there is nothing to
// guess and a wrong one costs nothing; throttling them per address would let
// anyone behind the same proxy lock every connector out. Only a wrong client
// secret counts, towards a throttle of that client.
func (h *Handler) handleOAuthToken(w http.ResponseWriter, r *http.Request) {
	issuer, on := h.oauthIssuer()
	if !on {
		http.NotFound(w, r)
		return
	}
	if !oauthForm(w, r) {
		return
	}
	client, ok := h.oauthClientFrom(r)
	bucket := "oauth|" + client.ID
	if client.ID != "" && h.loginThrottled(bucket) {
		w.Header().Set("Retry-After", strconv.Itoa(int(loginWindow.Seconds())))
		oauthError(w, http.StatusTooManyRequests, "invalid_request", "too many failed attempts, wait a minute")
		return
	}
	if !ok {
		if client.ID != "" {
			h.recordLoginFail(bucket)
		}
		oauthError(w, http.StatusUnauthorized, "invalid_client", "the client is unknown or did not authenticate")
		return
	}
	if res := r.PostForm.Get("resource"); res != "" && !sameResource(res, oauthResource(issuer)) {
		oauthError(w, http.StatusBadRequest, "invalid_target", "this server issues tokens for "+oauthResource(issuer)+" only")
		return
	}
	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		h.exchangeOAuthCode(w, r, client, oauthResource(issuer))
	case "refresh_token":
		h.refreshOAuthGrant(w, r, client, oauthResource(issuer))
	default:
		oauthError(w, http.StatusBadRequest, "unsupported_grant_type", "only authorization_code and refresh_token are supported")
	}
}

// newOAuthTokens mints an access and a refresh token and their digests.
func (h *Handler) newOAuthTokens() (access, refresh, accessDigest, refreshDigest string, err error) {
	if access, err = secret.NewOAuthSecret(secret.OAuthAccessPrefix); err != nil {
		return
	}
	if refresh, err = secret.NewOAuthSecret(secret.OAuthRefreshPrefix); err != nil {
		return
	}
	return access, refresh,
		secret.HashOAuthSecret(h.cfg.AppKey, "access", access),
		secret.HashOAuthSecret(h.cfg.AppKey, "refresh", refresh), nil
}

func writeOAuthTokens(w http.ResponseWriter, access, refresh string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  access,
		"token_type":    "Bearer",
		"expires_in":    int(oauthAccessTTL.Seconds()),
		"refresh_token": refresh,
		"scope":         oauthScope,
	})
}

func (h *Handler) exchangeOAuthCode(w http.ResponseWriter, r *http.Request, client store.OAuthClient, resource string) {
	now := h.mcp.now()
	f := r.PostForm
	code, replayed := h.mcp.oauth.redeem(secret.HashOAuthSecret(h.cfg.AppKey, "code", f.Get("code")),
		client.ID, f.Get("redirect_uri"), f.Get("code_verifier"), now)
	if replayed != "" {
		// Somebody holds a copy of the code. The grant it made can no longer
		// be told apart from theirs, so it goes.
		if row, err := h.store.GetMCPKey(replayed); err == nil {
			if err := h.store.RevokeMCPKey(replayed, "code-replay", now.Unix()); err == nil {
				h.recordMCPKeyChange(r, row, "revoked after its code was used twice", "revoked after its authorization code was used twice")
			}
		}
	}
	// A code allowed before the public address changed would make a grant
	// whose tokens /mcp no longer accepts.
	if code == nil || code.resource != resource {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "the code is unknown, expired, already used, or does not match this client, redirect URI and verifier")
		return
	}
	access, refresh, accessDigest, refreshDigest, err := h.newOAuthTokens()
	if err != nil {
		oauthError(w, http.StatusInternalServerError, "server_error", "could not create the tokens")
		return
	}
	if err := h.store.PruneOAuth(now.Unix()); err != nil {
		log.Printf("api: mcp: oauth: prune: %v", err)
	}
	grant, err := h.store.CreateOAuthGrant(store.OAuthGrant{
		ID:              code.grantID,
		Label:           code.label,
		Client:          code.known,
		OAuthClient:     client.ID,
		Resource:        code.resource,
		Check:           secret.MCPKeyCheck(h.cfg.AppKey, code.grantID),
		CanStartBackups: code.canStart,
		AccessDigest:    accessDigest,
		RefreshDigest:   refreshDigest,
		AccessExpires:   now.Add(oauthAccessTTL).Unix(),
		RefreshExpires:  now.Add(oauthRefreshTTL).Unix(),
	}, now.Unix())
	if errors.Is(err, store.ErrMCPGrantLimit) {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "BombVault already has as many signed-in clients as it allows")
		return
	}
	if err != nil {
		log.Printf("api: mcp: oauth: could not store a grant: %v", err)
		oauthError(w, http.StatusInternalServerError, "server_error", "could not store the grant")
		return
	}
	h.recordMCPKeyChange(r, grant, "granted", "granted access through OAuth")
	writeOAuthTokens(w, access, refresh)
}

func (h *Handler) refreshOAuthGrant(w http.ResponseWriter, r *http.Request, client store.OAuthClient, resource string) {
	now := h.mcp.now()
	old := r.PostForm.Get("refresh_token")
	access, refresh, accessDigest, refreshDigest, err := h.newOAuthTokens()
	if err != nil {
		oauthError(w, http.StatusInternalServerError, "server_error", "could not create the tokens")
		return
	}
	grant, err := h.store.RotateOAuthRefresh(secret.HashOAuthSecret(h.cfg.AppKey, "refresh", old), client.ID, resource,
		accessDigest, refreshDigest, now.Add(oauthAccessTTL).Unix(), now.Add(oauthRefreshTTL).Unix(), now.Unix())
	switch {
	case errors.Is(err, store.ErrOAuthRefreshReused):
		h.recordMCPKeyChange(r, grant, "revoked after a refresh token was used twice", "revoked after one of its refresh tokens was used twice")
		oauthError(w, http.StatusBadRequest, "invalid_grant", "the refresh token was already used")
		return
	case errors.Is(err, store.ErrOAuthTokenNotFound):
		oauthError(w, http.StatusBadRequest, "invalid_grant", "the refresh token is unknown, expired or revoked")
		return
	case err != nil:
		log.Printf("api: mcp: oauth: could not rotate a refresh token: %v", err)
		oauthError(w, http.StatusInternalServerError, "server_error", "could not rotate the refresh token")
		return
	}
	writeOAuthTokens(w, access, refresh)
}

// handleOAuthRevoke is token revocation (RFC 7009). An unknown token is
// answered like a revoked one.
func (h *Handler) handleOAuthRevoke(w http.ResponseWriter, r *http.Request) {
	if _, on := h.oauthIssuer(); !on {
		http.NotFound(w, r)
		return
	}
	if !oauthForm(w, r) {
		return
	}
	client, ok := h.oauthClientFrom(r)
	if !ok {
		oauthError(w, http.StatusUnauthorized, "invalid_client", "the client is unknown or did not authenticate")
		return
	}
	token := r.PostForm.Get("token")
	kind := ""
	switch {
	case strings.HasPrefix(token, secret.OAuthAccessPrefix):
		kind = "access"
	case strings.HasPrefix(token, secret.OAuthRefreshPrefix):
		kind = "refresh"
	}
	if kind != "" {
		grant, err := h.store.RevokeOAuthToken(secret.HashOAuthSecret(h.cfg.AppKey, kind, token), client.ID, h.mcp.now().Unix())
		if err != nil {
			log.Printf("api: mcp: oauth: could not revoke a token: %v", err)
			oauthError(w, http.StatusServiceUnavailable, "server_error", "could not revoke the token")
			return
		}
		if grant.ID != "" {
			h.recordMCPKeyChange(r, grant, "revoked by its client", "revoked by its client")
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
}
