package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"mime"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// sameSiteOnly refuses a request a browser fired from another website, judged by
// the Sec-Fetch-Site header, which the browser sets and a page cannot forge.
// Only "cross-site" is refused: what a browser counts as one site for a bare LAN
// IP is too uncertain to bet an install on, and a missing header means a
// non-browser client such as curl or a peer's mesh POST.
//
// It runs over every unsafe method, not only in decodeBody, because several
// state-changing routes take no body, such as POST /api/backup-everything.
func sameSiteOnly(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Site") != "cross-site" {
		return true
	}
	writeJSON(w, http.StatusForbidden, map[string]any{
		"ok":    false,
		"error": "refused a cross-site request",
	})
	return false
}

// csrfGate applies sameSiteOnly to every method that can change state. Safe
// methods (GET/HEAD/OPTIONS) pass through untouched: they are reachable
// cross-site by design, the browser's same-origin policy keeps the response
// unreadable, and gating them would break the widget iframe and a peer's status
// poll, both of which are GETs on purpose.
func csrfGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if !sameSiteOnly(w, r) {
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// crossOriginGuard refuses a body-carrying request that a browser fired from
// another website. With no login password set, authGate lets every request
// through on the assumption that an attacker has to be on the LAN, but a
// cross-site HTML form with enctype="text/plain" can post a body the JSON
// decoder accepts from the operator's own browser, with no preflight and no
// cookie. That would be enough to repoint notifications or set the Backup
// Everything hooks, which run as `sh -c` next to the Docker socket.
//
// Requiring a JSON Content-Type does the work: a cross-origin form cannot send
// it, and fetch() with it needs a CORS preflight this server never answers.
// sameSiteOnly adds the Sec-Fetch-Site check. Neither needs a token, which
// matters because trusted-LAN mode has no session to hang one on.
func crossOriginGuard(w http.ResponseWriter, r *http.Request) bool {
	if !sameSiteOnly(w, r) {
		return false
	}
	ct := r.Header.Get("Content-Type")
	if mediaType, _, err := mime.ParseMediaType(ct); err != nil || mediaType != "application/json" {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]any{
			"ok":    false,
			"error": "this endpoint takes application/json; set the Content-Type header",
		})
		return false
	}
	return true
}

const (
	sessionCookieName = "bv_session"
	sessionTTL        = 7 * 24 * time.Hour // 7 days
)

// authEnabled reads the stored password hash and session epoch and reports
// whether authentication is enabled. A store error counts as off, so a passing
// database error does not lock everyone out of a trusted-LAN tool.
func (h *Handler) authEnabled() (hash, epoch string, on bool) {
	s, err := h.store.GetSettings()
	if err != nil {
		log.Printf("api: authEnabled: GetSettings: %v", err)
		return "", "", false
	}
	return s.AuthPasswordHash, s.SessionEpoch, s.AuthPasswordHash != ""
}

// requireAuthForSecrets reports whether a handler that hands out stored secrets
// in the clear may proceed, answering 403 itself when it may not. Without a
// login password authGate lets everything through, which is acceptable for
// current data but not for keys that decrypt every repository, including the
// append-only off-site archives, so these require a password. A store error
// refuses too. action names what is refused, such as "downloading the
// recovery kit".
func (h *Handler) requireAuthForSecrets(w http.ResponseWriter, action string) bool {
	if _, _, on := h.authEnabled(); on {
		return true
	}
	writeJSON(w, http.StatusForbidden, map[string]any{
		"ok":    false,
		"error": "set a login password before " + action,
	})
	return false
}

// newSessionCookie builds the bv_session cookie. Secure is off only in HTTP-only
// mode, for LAN installs without TLS.
func (h *Handler) newSessionCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{ //nolint:gosec // G124: Secure is conditionally false only in HTTP-only (cfg.HTTPOnly) mode; intentional for LAN deployments
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   !h.cfg.HTTPOnly,
	}
}

// handleAuthStatus handles GET /api/auth, telling the SPA whether to show the
// login screen and what the settings page should say about the account. The
// route is public, so second-factor details such as the recovery codes left
// go only to a caller who is signed in.
func (h *Handler) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	hash, epoch, on := h.authEnabled()
	authed := false
	if on {
		if c, err := r.Cookie(sessionCookieName); err == nil {
			authed = secret.ValidSessionToken(h.cfg.AppKey, hash, epoch, c.Value)
		}
	}
	out := map[string]any{
		"ok":      true,
		"enabled": on,
		"authed":  authed,
		// Told to everyone, because the login screen has to know whether to ask
		// for a code, and it asks before anybody is signed in. It reveals only
		// that this instance is harder to get into.
		"totp": false,
		// The rule the password field enforces, so the frontend can say the
		// number rather than hard-code a second copy of it.
		"minPasswordLen": secret.MinPasswordLen,
	}
	if on {
		if s, err := h.store.GetSettings(); err == nil {
			out["totp"] = s.TOTPEnabled
			if authed || !on {
				out["recoveryCodesLeft"] = len(decodeRecoveryCodes(s.TOTPRecovery))
				out["passwordNeedsUpgrade"] = secret.NeedsRehash(hash)
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// A client is locked out after loginMaxFails failed logins within loginWindow.
// The throttle is checked before the password is hashed, so a locked-out
// client cannot learn from a correct guess either; it gets a 429 even with the
// right password.
const (
	loginMaxFails = 5
	loginWindow   = time.Minute
)

// passwordHashSlots bounds how many Argon2id verifications run at once. Each
// costs about 19 MiB, and the login throttle is per client, so an attacker
// rotating source addresses could otherwise push a memory-capped container into
// the OOM killer. Two slots cap hashing at about 38 MiB; further logins wait.
var passwordHashSlots = make(chan struct{}, 2)

// verifyPassword is secret.VerifyPassword behind that cap.
func verifyPassword(appKey, password, storedHash string) bool {
	passwordHashSlots <- struct{}{}
	defer func() { <-passwordHashSlots }()
	return secret.VerifyPassword(appKey, password, storedHash)
}

// loginClientKey returns the throttle key for r: the TCP peer's IP without its
// port. It never reads X-Forwarded-For, which a caller could set to pick a fresh
// bucket per request; behind a reverse proxy all clients share the proxy's
// bucket unless TRUSTED_PROXY is set (see (*Handler).loginClientKey).
func loginClientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// The raw value, so callers without a port (tests set RemoteAddr
		// directly) do not all share one "" bucket.
		return r.RemoteAddr
	}
	return host
}

// loginClientKey is loginClientKey plus the TRUSTED_PROXY step: when the peer is
// a configured trusted proxy, the throttle keys on the client that proxy names
// rather than on the proxy itself.
func (h *Handler) loginClientKey(r *http.Request) string {
	peer := loginClientKey(r)
	if len(h.cfg.TrustedProxies) == 0 {
		return peer
	}
	ip := net.ParseIP(peer)
	if ip == nil || !trusted(h.cfg.TrustedProxies, ip) {
		// Someone other than the proxy connects directly, so their header is
		// ignored, or naming a trusted proxy would let anyone spoof the key.
		return peer
	}
	if fwd := forwardedClient(r.Header.Get("X-Forwarded-For"), h.cfg.TrustedProxies); fwd != "" {
		return fwd
	}
	return peer
}

// forwardedClient returns the client address from an X-Forwarded-For chain,
// reading right to left and stopping at the first entry that is not a trusted
// proxy. Each hop appends to the header, so the left end is whatever the caller
// sent and only the right end was written by hops we trust.
func forwardedClient(header string, proxies []net.IPNet) string {
	parts := strings.Split(header, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		entry := strings.TrimSpace(parts[i])
		if entry == "" {
			continue
		}
		// A chain entry may carry a port (rare, but legal for IPv6 forms).
		if host, _, err := net.SplitHostPort(entry); err == nil {
			entry = host
		}
		ip := net.ParseIP(strings.Trim(entry, "[]"))
		if ip == nil {
			// Refuse the whole header, so an unparseable entry cannot steer
			// which entry is used.
			return ""
		}
		if trusted(proxies, ip) {
			continue
		}
		return ip.String()
	}
	return ""
}

func trusted(proxies []net.IPNet, ip net.IP) bool {
	for _, n := range proxies {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// loginSweepEvery is how many throttle checks pass between sweeps of the whole
// failure map. loginThrottled prunes only the key it is asked about, so keys
// that fail once and never return, such as an attacker rotating through an
// IPv6 /64, would otherwise stay forever.
const loginSweepEvery = 256

// loginMaxTracked caps the keys loginFails holds, for a burst of one-off keys
// that arrives faster than the periodic sweep.
const loginMaxTracked = 10_000

// loginThrottled prunes key's failure window and reports whether logins from it
// are locked out. It also sweeps the whole map now and then and caps its size,
// so keys queried only once cannot pile up.
func (h *Handler) loginThrottled(key string) bool {
	h.loginMu.Lock()
	defer h.loginMu.Unlock()
	h.sweepLoginFailsLocked()
	cutoff := time.Now().Add(-loginWindow)
	kept := pruneLoginFails(h.loginFails[key], cutoff)
	if len(kept) == 0 {
		delete(h.loginFails, key)
	} else {
		h.loginFails[key] = kept
	}
	return len(kept) >= loginMaxFails
}

// pruneLoginFails returns fails with every timestamp at or before cutoff
// dropped, reusing fails' backing array (no allocation on the common case).
func pruneLoginFails(fails []time.Time, cutoff time.Time) []time.Time {
	kept := fails[:0]
	for _, ts := range fails {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	return kept
}

// sweepLoginFailsLocked prunes every key's window once every loginSweepEvery
// calls, or at once when the map has grown past loginMaxTracked, and evicts
// when a full prune still leaves it over the cap. The caller holds loginMu.
func (h *Handler) sweepLoginFailsLocked() {
	h.loginSweepCalls++
	if h.loginSweepCalls < loginSweepEvery && len(h.loginFails) <= loginMaxTracked {
		return
	}
	h.loginSweepCalls = 0
	cutoff := time.Now().Add(-loginWindow)
	for k, fails := range h.loginFails {
		if kept := pruneLoginFails(fails, cutoff); len(kept) == 0 {
			delete(h.loginFails, k)
		} else {
			h.loginFails[k] = kept
		}
	}
	if len(h.loginFails) > loginMaxTracked {
		h.evictLeastRecentlyTouchedLocked()
	}
}

// evictLeastRecentlyTouchedLocked deletes entries from h.loginFails, the one
// with the oldest latest failure first, until it is back under loginMaxTracked.
// The caller holds loginMu.
//
// A throttled key is never evicted. Its timestamps stop advancing once it is
// locked out, because handleLogin checks the throttle before recording a
// failure, so by age alone a flood of fresh one-off keys would evict it and
// lift its lockout. Throttled keys may sit over the cap, but only callers that
// reach loginMaxFails count, which is a small population.
func (h *Handler) evictLeastRecentlyTouchedLocked() {
	type keyAge struct {
		key  string
		last time.Time
	}
	ages := make([]keyAge, 0, len(h.loginFails))
	for k, fails := range h.loginFails {
		if len(fails) >= loginMaxFails {
			continue // throttled, see above
		}
		ages = append(ages, keyAge{k, fails[len(fails)-1]})
	}
	sort.Slice(ages, func(i, j int) bool { return ages[i].last.Before(ages[j].last) })
	for _, a := range ages {
		if len(h.loginFails) <= loginMaxTracked {
			break
		}
		delete(h.loginFails, a.key)
	}
}

func (h *Handler) recordLoginFail(key string) {
	h.loginMu.Lock()
	if h.loginFails == nil {
		h.loginFails = make(map[string][]time.Time)
	}
	h.loginFails[key] = append(h.loginFails[key], time.Now())
	h.loginMu.Unlock()
}

func (h *Handler) recordLoginSuccess(key string) {
	h.loginMu.Lock()
	delete(h.loginFails, key)
	h.loginMu.Unlock()
}

// handleLogin handles POST /api/login.
func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	hash, epoch, on := h.authEnabled()
	if !on {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "authentication is not enabled"})
		return
	}
	key := h.loginClientKey(r)
	if h.loginThrottled(key) {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"ok": false, "error": "too many failed attempts; wait a minute and try again"})
		return
	}

	var body struct {
		Password string `json:"password"`
		// Code is the authenticator app's six digits, or a recovery code, and
		// is only read when the second factor is switched on.
		Code string `json:"code"`
	}
	if !decodeBody(w, r, &body) {
		return
	}

	if !verifyPassword(h.cfg.AppKey, body.Password, hash) {
		h.recordLoginFail(key)
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "invalid password"})
		return
	}

	// With a second factor on, nothing is granted until it passes too, not even
	// the throttle reset, so someone with the password still gets five tries a
	// minute at the code.
	s, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if s.TOTPEnabled {
		if strings.TrimSpace(body.Code) == "" {
			// Not a failure: the client now knows to show the code field.
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":       false,
				"needCode": true,
				"error":    "enter the code from your authenticator app",
			})
			return
		}
		if !h.secondFactorOK(&s, body.Code) {
			h.recordLoginFail(key)
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":       false,
				"needCode": true,
				"error":    "that code is not valid",
			})
			return
		}
	}
	h.recordLoginSuccess(key)

	// A legacy password hash can only be upgraded while the verified plaintext
	// is at hand. It happens before the token is minted, because the session
	// HMAC signs the stored hash and the rehash would invalidate a token issued
	// against the old one.
	if secret.NeedsRehash(hash) {
		if fresh, hErr := secret.HashPassword(h.cfg.AppKey, body.Password); hErr != nil {
			log.Printf("api: login: rehash: %v", hErr)
		} else if _, mErr := h.store.MutateSettings(func(st *store.Settings) error {
			// Re-check inside the mutation: a parallel password change between
			// the verify and here must not be overwritten with the old one.
			if st.AuthPasswordHash == hash {
				st.AuthPasswordHash = fresh
			}
			return nil
		}); mErr != nil {
			// Not fatal: the hash stays in the old format until the next login.
			log.Printf("api: login: storing upgraded password hash: %v", mErr)
		} else {
			hash = fresh
		}
	}

	tok := secret.NewSessionToken(h.cfg.AppKey, hash, epoch, sessionTTL)
	http.SetCookie(w, h.newSessionCookie(tok, int(sessionTTL.Seconds())))
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// secondFactorOK checks code against the time-based secret and, failing that,
// against the single-use recovery codes. A matching recovery code is removed
// from the stored list before this returns true.
func (h *Handler) secondFactorOK(s *store.Settings, code string) bool {
	if sec, err := h.decryptTOTPSecret(s.TOTPSecret); err == nil && sec != "" {
		if secret.ValidTOTP(sec, code, time.Now()) {
			return true
		}
	}
	stored := decodeRecoveryCodes(s.TOTPRecovery)
	idx := secret.MatchRecoveryCode(h.cfg.AppKey, code, stored)
	if idx < 0 {
		return false
	}
	remaining := append(append([]string{}, stored[:idx]...), stored[idx+1:]...)
	if _, err := h.store.MutateSettings(func(st *store.Settings) error {
		st.TOTPRecovery = encodeRecoveryCodes(remaining)
		return nil
	}); err != nil {
		// The code was correct, but it could not be burned. Refuse the login:
		// a recovery code that survives its own use is a permanent password.
		log.Printf("api: login: spending recovery code: %v", err)
		return false
	}
	log.Printf("api: login: a recovery code was used, %d left", len(remaining))
	return true
}

func decodeRecoveryCodes(raw string) []string {
	if raw == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		log.Printf("api: recovery codes: unreadable, treating as none: %v", err)
		return nil
	}
	return out
}

func encodeRecoveryCodes(codes []string) string {
	if len(codes) == 0 {
		// Empty string, not "[]": the column's zero value and "none left" are
		// the same state, and only one of them should exist in the database.
		return ""
	}
	b, err := json.Marshal(codes)
	if err != nil {
		return ""
	}
	return string(b)
}

// decryptTOTPSecret opens the stored (hex-encoded, APP_KEY-encrypted) secret.
func (h *Handler) decryptTOTPSecret(stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	raw, err := hex.DecodeString(stored)
	if err != nil {
		return "", fmt.Errorf("totp secret: %w", err)
	}
	plain, err := secret.Decrypt(h.cfg.AppKey, raw)
	if err != nil {
		return "", fmt.Errorf("totp secret: %w", err)
	}
	return string(plain), nil
}

func (h *Handler) encryptTOTPSecret(plain string) (string, error) {
	sealed, err := secret.Encrypt(h.cfg.AppKey, []byte(plain))
	if err != nil {
		return "", fmt.Errorf("totp secret: %w", err)
	}
	return hex.EncodeToString(sealed), nil
}

// handleLogout handles POST /api/logout by clearing the session cookie. The
// stateless token stays valid until it expires, so a copied cookie would still
// work; handleLogoutAll is the revocation.
func (h *Handler) handleLogout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, h.newSessionCookie("", -1))
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// newSessionEpoch returns a fresh random session epoch (16 bytes, hex).
func newSessionEpoch() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate session epoch: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// handleLogoutAll handles POST /api/logout-all ("log out everywhere"). It
// rotates the session epoch that every token's HMAC is bound to, so every
// outstanding cookie becomes invalid at once, and clears the caller's own.
func (h *Handler) handleLogoutAll(w http.ResponseWriter, _ *http.Request) {
	epoch, err := newSessionEpoch()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if _, err := h.store.MutateSettings(func(s *store.Settings) error {
		s.SessionEpoch = epoch
		return nil
	}); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	http.SetCookie(w, h.newSessionCookie("", -1))
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleSetPassword handles POST /api/auth/password with body {password}; an
// empty password turns the login off.
func (h *Handler) handleSetPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if !decodeBody(w, r, &body) {
		return
	}

	hash := ""
	if body.Password != "" {
		// The minimum applies only when a password is set, so an older, shorter
		// password still verifies and its owner is asked to change it. Runes,
		// not bytes, so a passphrase in a multi-byte script is not favoured.
		if utf8.RuneCountInString(body.Password) < secret.MinPasswordLen {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":     false,
				"error":  fmt.Sprintf("the password must be at least %d characters", secret.MinPasswordLen),
				"minLen": secret.MinPasswordLen,
			})
			return
		}
		var err error
		if hash, err = secret.HashPassword(h.cfg.AppKey, body.Password); err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
	}
	epoch := ""
	if _, err := h.store.MutateSettings(func(s *store.Settings) error {
		s.AuthPasswordHash = hash
		if hash == "" {
			// Switching the login off takes the second factor with it, so a
			// login switched on again later does not ask for a code from an app
			// the operator may have deleted long ago.
			s.TOTPEnabled = false
			s.TOTPSecret = ""
			s.TOTPRecovery = ""
		}
		// Setting a password ends every other session, because a password that
		// may have leaked is worth nothing while sessions minted under it stay
		// alive. The caller keeps working: the cookie below is signed with the
		// new epoch.
		next, err := newSessionEpoch()
		if err != nil {
			return err
		}
		s.SessionEpoch = next
		epoch = next
		return nil
	}); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}

	// Setting a password signs the operator in, or every later request would
	// get a 401 until the page is reloaded and the password typed again. It
	// grants nothing new: without a login this route was already open to the
	// caller, and with one authGate guards it. The token signs the new hash,
	// so it is minted after the write; clearing the password clears the cookie.
	if hash == "" {
		http.SetCookie(w, h.newSessionCookie("", -1))
	} else {
		tok := secret.NewSessionToken(h.cfg.AppKey, hash, epoch, sessionTTL)
		http.SetCookie(w, h.newSessionCookie(tok, int(sessionTTL.Seconds())))
	}

	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"enabled": hash != "",
		// The caller is signed in as of this answer, so the Security card can
		// show the sign-out controls without a round trip or a reload.
		"authed": hash != "",
	}))
}

// handleTOTPSetup handles POST /api/auth/totp/setup: it mints a fresh secret,
// stores it encrypted but not yet armed, and returns the otpauth URI for the QR
// code. Storing it lets the confirm step check against the server's copy, and
// TOTPEnabled stays false until a working code arrives, so an enrolment
// abandoned halfway leaves the login as it was.
func (h *Handler) handleTOTPSetup(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuthForSecrets(w, "setting up two-factor authentication") {
		return
	}
	s, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if s.AuthPasswordHash == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": "set a login password first: a second factor with no first one protects nothing",
		})
		return
	}
	if s.TOTPEnabled {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": "two-factor authentication is already on; turn it off before setting it up again",
		})
		return
	}

	plain, err := secret.NewTOTPSecret()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	sealed, err := h.encryptTOTPSecret(plain)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if _, err := h.store.MutateSettings(func(st *store.Settings) error {
		st.TOTPSecret = sealed
		st.TOTPEnabled = false
		return nil
	}); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}

	name := s.InstanceName
	if name == "" {
		name = "BombVault"
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"secret": plain,
		"uri":    secret.TOTPURI("BombVault", name, plain),
	}))
}

// handleTOTPConfirm handles POST /api/auth/totp/confirm: it arms the second
// factor once the operator types a code the stored secret produces, and returns
// the recovery codes. They are stored hashed, so this answer is the only time
// they are shown.
func (h *Handler) handleTOTPConfirm(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuthForSecrets(w, "setting up two-factor authentication") {
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	s, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	plain, err := h.decryptTOTPSecret(s.TOTPSecret)
	if err != nil || plain == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": "no pending setup; start again",
		})
		return
	}
	if !secret.ValidTOTP(plain, body.Code, time.Now()) {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": "that code is not valid; check the clock on your phone and try the next one",
		})
		return
	}

	codes, hashed, err := secret.NewRecoveryCodes(h.cfg.AppKey)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if _, err := h.store.MutateSettings(func(st *store.Settings) error {
		st.TOTPEnabled = true
		st.TOTPRecovery = encodeRecoveryCodes(hashed)
		return nil
	}); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"recoveryCodes": codes,
	}))
}

// handleTOTPDisable handles POST /api/auth/totp/disable. It requires a current
// code or a recovery code, so an unattended session cannot remove the factor.
func (h *Handler) handleTOTPDisable(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuthForSecrets(w, "changing two-factor authentication") {
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	s, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if !s.TOTPEnabled {
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"enabled": false}))
		return
	}
	if !h.secondFactorOK(&s, body.Code) {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    false,
			"error": "that code is not valid",
		})
		return
	}
	if _, err := h.store.MutateSettings(func(st *store.Settings) error {
		st.TOTPEnabled = false
		st.TOTPSecret = ""
		st.TOTPRecovery = ""
		return nil
	}); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"enabled": false}))
}

// authGate requires a session cookie once a login password is set, and passes
// everything through while none is. These paths are always open:
//   - GET /api/auth, POST /api/login and GET /api/health, so the SPA can load
//     and sign in.
//   - GET /metrics, GET /widget, GET /api/widget/data and GET
//     /api/fleet/status. Prometheus, an embedding iframe and a polling peer
//     cannot carry the cookie, so each gates itself on its own token and
//     refuses with 403 when none is set. Managing those tokens needs a session.
//   - POST /api/fleet/mesh-offer, the one write on this list, behind the same
//     fleet token. It only stores a pending offer for a person to review.
//   - The passkey status and the two passkey login halves.
func (h *Handler) authGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A store error must not drop the gate, so it fails closed, keeping
		// only the public endpoints reachable so the SPA can still recover.
		s, err := h.store.GetSettings()
		if err != nil {
			log.Printf("api: authGate: GetSettings: %v", err)
			switch r.URL.Path {
			case "/api/auth", "/api/login", "/api/health", "/metrics", "/widget", "/api/widget/data", "/api/fleet/status", "/api/fleet/mesh-offer",
				"/api/auth/passkeys", "/api/auth/passkey/login/begin", "/api/auth/passkey/login/finish":
				next.ServeHTTP(w, r)
			default:
				writeJSON(w, http.StatusServiceUnavailable, map[string]any{
					"ok":    false,
					"error": "authentication unavailable",
				})
			}
			return
		}
		hash := s.AuthPasswordHash
		on := hash != ""
		if !on {
			next.ServeHTTP(w, r)
			return
		}

		// The public paths from the doc comment above.
		switch r.URL.Path {
		case "/api/auth", "/api/login", "/api/health", "/metrics", "/widget", "/api/widget/data", "/api/fleet/status", "/api/fleet/mesh-offer",
			// The passkey login halves are how someone signs in. The status
			// tells an unauthenticated caller only counts and whether this
			// address can carry a passkey; the key list needs a session.
			"/api/auth/passkeys", "/api/auth/passkey/login/begin", "/api/auth/passkey/login/finish":
			next.ServeHTTP(w, r)
			return
		}

		// All other /api/* routes require a valid session cookie.
		c, err := r.Cookie(sessionCookieName)
		if err != nil || !secret.ValidSessionToken(h.cfg.AppKey, hash, s.SessionEpoch, c.Value) {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"ok":    false,
				"error": "authentication required",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}
