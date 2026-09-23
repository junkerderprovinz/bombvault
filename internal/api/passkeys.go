package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Passkeys are a second way to sign in and never replace the password: an
// operator who lost the phone or reaches the box by IP still has to get into
// the tool that restores everything.
//
// WebAuthn binds a credential to a relying-party id, which has to be a domain,
// and browsers refuse the ceremony on a certificate they do not trust. The
// Unraid template opens https://[IP]:3443 with a self-signed certificate, so
// passkeys only work behind a reverse proxy with a real host name.

// passkeyCeremony is one in-flight registration or login. It lives in memory
// only: it expires within minutes, and a restart costs a second click at most.
type passkeyCeremony struct {
	session webauthn.SessionData
	rpID    string
	expires time.Time
}

const (
	// passkeyCeremonyTTL bounds how long a started ceremony can be finished.
	// The browser prompt usually times out sooner.
	passkeyCeremonyTTL = 5 * time.Minute
	// passkeyCeremonyMax caps the map, because a login ceremony can be started
	// without a session.
	passkeyCeremonyMax = 64
)

// beginPasskeyCeremony stores the session data and returns the random,
// single-use handle the finish call has to present.
func (h *Handler) beginPasskeyCeremony(s *webauthn.SessionData, rpID string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("ceremony handle: %w", err)
	}
	id := hex.EncodeToString(raw)

	h.passkeyMu.Lock()
	defer h.passkeyMu.Unlock()
	if h.passkeyCeremonies == nil {
		h.passkeyCeremonies = map[string]passkeyCeremony{}
	}
	now := time.Now()
	for k, c := range h.passkeyCeremonies {
		if now.After(c.expires) {
			delete(h.passkeyCeremonies, k)
		}
	}
	// Still full: drop the oldest. That costs somebody one more click, while an
	// unbounded map reachable without a session costs memory.
	for len(h.passkeyCeremonies) >= passkeyCeremonyMax {
		oldestKey, oldest := "", time.Time{}
		for k, c := range h.passkeyCeremonies {
			if oldest.IsZero() || c.expires.Before(oldest) {
				oldestKey, oldest = k, c.expires
			}
		}
		delete(h.passkeyCeremonies, oldestKey)
	}
	h.passkeyCeremonies[id] = passkeyCeremony{session: *s, rpID: rpID, expires: now.Add(passkeyCeremonyTTL)}
	return id, nil
}

// takePasskeyCeremony consumes a handle, so an answered challenge cannot be
// replayed.
func (h *Handler) takePasskeyCeremony(id string) (passkeyCeremony, bool) {
	h.passkeyMu.Lock()
	defer h.passkeyMu.Unlock()
	c, ok := h.passkeyCeremonies[id]
	if !ok {
		return passkeyCeremony{}, false
	}
	delete(h.passkeyCeremonies, id)
	if time.Now().After(c.expires) {
		return passkeyCeremony{}, false
	}
	return c, true
}

// errPasskeyOrigin is the refusal for an IP-address origin, which is what the
// default installation gets, so it carries the whole explanation. It must not
// contain a slash: scrubError redacts slash-led tokens on the way out.
var errPasskeyOrigin = errors.New(
	"passkeys need a host name, and this page was opened on an IP address. " +
		"The standard binds a passkey to a domain and browsers refuse the whole exchange on a bare address, " +
		"and they refuse it again on a certificate the browser does not trust. " +
		"Reach BombVault through a reverse proxy under a real name with a valid certificate, open it there, and register the key on that address")

// rpIDFor derives the relying-party id from the request host, without the
// port, and refuses an address that cannot carry one. It comes from the request
// rather than a setting because the box is often reachable several ways and the
// browser only offers a key whose id matches the address bar.
func rpIDFor(r *http.Request) (string, error) {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if host == "" {
		return "", errPasskeyOrigin
	}
	// Browsers treat localhost as a secure context and accept it as a
	// relying-party id, so a port-forwarded tunnel works.
	if strings.EqualFold(host, "localhost") {
		return "localhost", nil
	}
	if net.ParseIP(host) != nil {
		return "", errPasskeyOrigin
	}
	return strings.ToLower(host), nil
}

// originFor rebuilds the origin the browser will report, so the library can
// check the ceremony against it rather than against a guess.
func (h *Handler) originFor(r *http.Request) string {
	scheme := "https"
	if h.cfg.HTTPOnly {
		scheme = "http"
	}
	// A proxy that terminates TLS forwards plain HTTP, but the origin has to
	// carry the scheme the browser saw.
	if fp := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); fp != "" {
		if i := strings.IndexByte(fp, ','); i > 0 {
			fp = strings.TrimSpace(fp[:i])
		}
		if fp == "http" || fp == "https" {
			scheme = fp
		}
	}
	return scheme + "://" + r.Host
}

// webAuthnFor builds the library handle for the request's address.
func (h *Handler) webAuthnFor(r *http.Request) (*webauthn.WebAuthn, string, error) {
	rpID, err := rpIDFor(r)
	if err != nil {
		return nil, "", err
	}
	w, err := webauthn.New(&webauthn.Config{
		RPID:          rpID,
		RPDisplayName: "BombVault",
		RPOrigins:     []string{h.originFor(r)},
	})
	if err != nil {
		return nil, "", err
	}
	return w, rpID, nil
}

// passkeyUser adapts the instance to the library's user model. BombVault has
// one operator and no user table, so the instance is the account. Its id must
// be stable, or every registered key stops working, and unguessable, so it is
// derived from APP_KEY, which survives a reinstall through the config backup.
type passkeyUser struct {
	id    []byte
	name  string
	creds []webauthn.Credential
}

func (u passkeyUser) WebAuthnID() []byte                         { return u.id }
func (u passkeyUser) WebAuthnName() string                       { return u.name }
func (u passkeyUser) WebAuthnDisplayName() string                { return u.name }
func (u passkeyUser) WebAuthnCredentials() []webauthn.Credential { return u.creds }

// passkeyUserID derives the account id. It is hashed so APP_KEY itself never
// leaves the box inside a credential.
func passkeyUserID(appKey string) []byte {
	sum := sha256.Sum256([]byte("bombvault-passkey-user:" + appKey))
	return sum[:]
}

// passkeyUserFor builds the account with the credentials registered for rpID.
// The browser refuses a key for any other relying-party id, so offering those
// would produce a prompt that cannot succeed.
func (h *Handler) passkeyUserFor(rpID string) (passkeyUser, []store.Passkey, error) {
	rows, err := h.store.PasskeysForRP(rpID)
	if err != nil {
		return passkeyUser{}, nil, err
	}
	u := passkeyUser{
		id:   passkeyUserID(h.cfg.AppKey),
		name: "bombvault",
	}
	for _, p := range rows {
		u.creds = append(u.creds, webauthn.Credential{
			ID:        p.CredentialID,
			PublicKey: p.PublicKey,
			Transport: parseTransports(p.Transports),
			Flags:     webauthn.CredentialFlags{BackupEligible: p.BackedUp, BackupState: p.BackedUp},
			Authenticator: webauthn.Authenticator{
				AAGUID:    p.AAGUID,
				SignCount: p.SignCount,
			},
		})
	}
	return u, rows, nil
}

func parseTransports(s string) []protocol.AuthenticatorTransport {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]protocol.AuthenticatorTransport, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, protocol.AuthenticatorTransport(p))
		}
	}
	return out
}

func joinTransports(ts []protocol.AuthenticatorTransport) string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		if s := strings.TrimSpace(string(t)); s != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, ",")
}

type passkeyView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// RPID is the address this key belongs to, shown in the list because a key
	// registered through the proxy does not exist over the IP.
	RPID string `json:"rpId"`
	// UsableHere is whether this key can answer on the address the browser has
	// open right now.
	UsableHere bool   `json:"usableHere"`
	BackedUp   bool   `json:"backedUp"`
	CreatedAt  int64  `json:"createdAt"`
	LastUsedAt int64  `json:"lastUsedAt"`
	Transports string `json:"transports"`
}

func passkeyViews(rows []store.Passkey, hereRPID string) []passkeyView {
	out := make([]passkeyView, 0, len(rows))
	for _, p := range rows {
		out = append(out, passkeyView{
			ID: p.ID, Name: p.Name, RPID: p.RPID,
			UsableHere: hereRPID != "" && p.RPID == hereRPID,
			BackedUp:   p.BackedUp, CreatedAt: p.CreatedAt, LastUsedAt: p.LastUsedAt,
			Transports: p.Transports,
		})
	}
	return out
}

// handlePasskeyStatus serves GET /api/auth/passkeys. It is public, like
// GET /api/auth, because the login screen has to know whether to offer the
// button. It never returns a credential.
func (h *Handler) handlePasskeyStatus(w http.ResponseWriter, r *http.Request) {
	rpID, rpErr := rpIDFor(r)
	all, err := h.store.ListPasskeys()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	here := 0
	for _, p := range all {
		if rpID != "" && p.RPID == rpID {
			here++
		}
	}
	body := map[string]any{
		"ok": true,
		// supported says whether this address can carry passkeys at all.
		"supported": rpErr == nil,
		"rpId":      rpID,
		"total":     len(all),
		"here":      here,
	}
	if rpErr != nil {
		body["reason"] = rpErr.Error()
	}
	// The list itself is only for somebody signed in. Before that the login
	// screen needs the counts and nothing else.
	hash, epoch, on := h.authEnabled()
	authed := false
	if on {
		if c, cErr := r.Cookie(sessionCookieName); cErr == nil {
			authed = secret.ValidSessionToken(h.cfg.AppKey, hash, epoch, c.Value)
		}
	}
	if !on || authed {
		body["passkeys"] = passkeyViews(all, rpID)
	}
	writeJSON(w, http.StatusOK, body)
}

// handlePasskeyRegisterBegin serves POST /api/auth/passkey/register/begin.
// Behind authGate: enrolling a key is something only a signed-in operator does.
func (h *Handler) handlePasskeyRegisterBegin(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuthForSecrets(w, "registering a passkey") {
		return
	}
	wa, rpID, err := h.webAuthnFor(r)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	user, _, err := h.passkeyUserFor(rpID)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	creation, session, err := wa.BeginRegistration(
		user,
		// Excluding the keys already registered here lets the browser say so
		// itself instead of producing a duplicate this box has to refuse.
		webauthn.WithExclusions(credentialDescriptors(user.creds)),
		// Discoverable keys allow signing in without naming an account first.
		// Preferred, not required, so an older security key without storage
		// still works.
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementPreferred,
			UserVerification: protocol.VerificationPreferred,
		}),
	)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": scrubError(err)})
		return
	}
	handle, err := h.beginPasskeyCeremony(session, rpID)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"ceremonyId": handle,
		"options":    creation.Response,
	}))
}

func credentialDescriptors(creds []webauthn.Credential) []protocol.CredentialDescriptor {
	out := make([]protocol.CredentialDescriptor, 0, len(creds))
	for _, c := range creds {
		out = append(out, c.Descriptor())
	}
	return out
}

// handlePasskeyRegisterFinish serves POST /api/auth/passkey/register/finish.
// The credential is the browser's own answer, handed to the library verbatim:
// re-encoding it here would mean re-implementing the parsing that validates it.
func (h *Handler) handlePasskeyRegisterFinish(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuthForSecrets(w, "registering a passkey") {
		return
	}
	var body struct {
		CeremonyID string          `json:"ceremonyId"`
		Name       string          `json:"name"`
		Credential json.RawMessage `json:"credential"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	cer, ok := h.takePasskeyCeremony(body.CeremonyID)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": "that registration has expired, start it again",
		})
		return
	}
	wa, rpID, err := h.webAuthnFor(r)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if rpID != cer.rpID {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": "this registration was started on a different address; open the one you want the key to work on and start again",
		})
		return
	}
	user, _, err := h.passkeyUserFor(rpID)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(body.Credential)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": scrubError(err)})
		return
	}
	cred, err := wa.CreateCredential(user, cer.session, parsed)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": scrubError(err)})
		return
	}

	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = "Passkey"
	}
	saved, err := h.store.AddPasskey(store.Passkey{
		Name:         name,
		CredentialID: cred.ID,
		PublicKey:    cred.PublicKey,
		AAGUID:       cred.Authenticator.AAGUID,
		SignCount:    cred.Authenticator.SignCount,
		Transports:   joinTransports(cred.Transport),
		RPID:         rpID,
		BackedUp:     cred.Flags.BackupEligible,
	})
	if errors.Is(err, store.ErrPasskeyExists) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	log.Printf("api: passkey %q registered for %s", saved.Name, rpID) //nolint:gosec // G706: the name is %q-quoted and the rp id is a host name
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"passkey": passkeyViews([]store.Passkey{saved}, rpID)[0],
	}))
}

// handlePasskeyLoginBegin serves POST /api/auth/passkey/login/begin.
// Public: it is one half of signing in.
func (h *Handler) handlePasskeyLoginBegin(w http.ResponseWriter, r *http.Request) {
	if _, _, on := h.authEnabled(); !on {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "no login is set up"})
		return
	}
	wa, rpID, err := h.webAuthnFor(r)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	user, rows, err := h.passkeyUserFor(rpID)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if len(rows) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": "no passkey is registered for this address",
		})
		return
	}
	assertion, session, err := wa.BeginLogin(user)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": scrubError(err)})
		return
	}
	handle, err := h.beginPasskeyCeremony(session, rpID)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"ceremonyId": handle,
		"options":    assertion.Response,
	}))
}

// handlePasskeyLoginFinish serves POST /api/auth/passkey/login/finish and, on a
// valid assertion, issues the session cookie.
//
// It shares the password login's throttle. A signature cannot be guessed, but
// without it this endpoint would be a way around the limit that protects the
// password.
func (h *Handler) handlePasskeyLoginFinish(w http.ResponseWriter, r *http.Request) {
	hash, epoch, on := h.authEnabled()
	if !on {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "no login is set up"})
		return
	}
	key := loginClientKey(r)
	if h.loginThrottled(key) {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"ok":    false,
			"error": "too many attempts, wait a moment",
		})
		return
	}
	var body struct {
		CeremonyID string          `json:"ceremonyId"`
		Credential json.RawMessage `json:"credential"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	cer, ok := h.takePasskeyCeremony(body.CeremonyID)
	if !ok {
		h.recordLoginFail(key)
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "that sign-in has expired, try again"})
		return
	}
	wa, rpID, err := h.webAuthnFor(r)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if rpID != cer.rpID {
		h.recordLoginFail(key)
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "that sign-in was started on a different address"})
		return
	}
	user, rows, err := h.passkeyUserFor(rpID)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(body.Credential)
	if err != nil {
		h.recordLoginFail(key)
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": scrubError(err)})
		return
	}
	cred, err := wa.ValidateLogin(user, cer.session, parsed)
	if err != nil {
		h.recordLoginFail(key)
		log.Printf("api: passkey sign-in refused: %v", scrubError(err))
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "that passkey was not accepted"})
		return
	}
	// A counter that failed to advance is the documented sign of a cloned key.
	// The library does not warn when both counters are zero, which is how an
	// authenticator without a counter reports.
	if cred.Authenticator.CloneWarning {
		h.recordLoginFail(key)
		log.Printf("api: passkey sign-in refused: the authenticator's counter went backwards, which is how a cloned key shows")
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": "that passkey was refused: its counter went backwards, which is how a copied key looks. Remove it and register a new one",
		})
		return
	}

	for _, p := range rows {
		if string(p.CredentialID) == string(cred.ID) {
			if tErr := h.store.TouchPasskey(p.ID, cred.Authenticator.SignCount, time.Now().Unix()); tErr != nil {
				log.Printf("api: passkey: recording use: %v", tErr)
			}
			log.Printf("api: passkey %q signed in", p.Name) //nolint:gosec // G706: the name is %q-quoted
			break
		}
	}

	tok := secret.NewSessionToken(h.cfg.AppKey, hash, epoch, sessionTTL)
	http.SetCookie(w, h.newSessionCookie(tok, int(sessionTTL.Seconds())))
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleDeletePasskey serves DELETE /api/auth/passkeys/{id}.
func (h *Handler) handleDeletePasskey(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "no passkey id"})
		return
	}
	if err := h.store.DeletePasskey(id); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleRenamePasskey serves PATCH /api/auth/passkeys/{id}. Body {name}.
func (h *Handler) handleRenamePasskey(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	var body struct {
		Name string `json:"name"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a passkey needs a name"})
		return
	}
	if err := h.store.RenamePasskey(id, name); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}
