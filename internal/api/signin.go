package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/signin"
	"github.com/junkerderprovinz/parleyport/kit/signin/sqlstore"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// BombVault signs in through the kit: the password, two-factor, passkeys,
// sessions, sign-in services and the reset through the container log. Its
// own ways in stay beside it. A paired phone comes through a door, and MCP
// keys, API tokens and OAuth grants end when the kit ends the owner's ways
// in.

// ownerID is the id of the one account that signs in with the password.
// Services add accounts of their own beside it.
const ownerID = "owner"

// openSignIn opens the kit's sign-in on BombVault's database. On its first
// start it imports the login the version before kept in the settings.
func (h *Handler) openSignIn() error {
	key, err := hex.DecodeString(h.cfg.AppKey)
	if err != nil {
		return fmt.Errorf("signin: APP_KEY: %w", err)
	}
	seal, err := signin.NewSealer(key)
	if err != nil {
		return err
	}
	ctx := context.Background()
	docs, err := sqlstore.New(ctx, h.store.DB(), store.SignInTable)
	if err != nil {
		return err
	}
	if err := h.loadPublicAddress(); err != nil {
		return err
	}
	auth, err := signin.Open(h.signInConfig(docs, seal))
	if err != nil && strings.Contains(err.Error(), "another seal key") {
		// APP_KEY changed, and nothing the sign-in sealed or hashed opens
		// under the new one. It starts over from the settings, where a set
		// password keeps the instance closed until the reset through the log.
		log.Printf("signin: APP_KEY changed, so the sign-in starts over; a password that was set has to be reset with the code from the log")
		if err := h.store.ForgetSignIn(); err != nil {
			return err
		}
		auth, err = signin.Open(h.signInConfig(docs, seal))
	}
	if err != nil {
		return err
	}
	h.auth = auth
	return nil
}

func (h *Handler) signInConfig(docs signin.Store, seal signin.Sealer) signin.Config {
	return signin.Config{
		Domain:      "bombvault",
		Name:        "BombVault",
		Store:       docs,
		Seal:        seal,
		Owner:       true,
		TokenPrefix: "bv",
		Public:      h.publicURL,
		Proxies:     trustedPrefixes(h.cfg.TrustedProxies),
		MinPassword: secret.RecommendedPasswordLen,
		NewID:       h.newAccountID,
		Import:      h.importLogin,
		Legacy: signin.Legacy{
			Password: func(_ context.Context, hash, pw string) (bool, error) {
				return secret.VerifyPassword(h.cfg.AppKey, pw, hash), nil
			},
			Session:  h.oldSession,
			Recovery: func(hash, code string) bool { return secret.MatchRecoveryCode(h.cfg.AppKey, code, []string{hash}) == 0 },
			Drop:     h.dropOldLogin,
		},
		Hooks: signin.Hooks{
			Ended: h.signInEnded,
			Doors: []signin.Door{h.phoneDoor},
		},
		Logger: slog.New(resetCodeHandler{out: func() io.Writer { return h.resetOut }, next: signInLog().Handler()}),
	}
}

// OpenSignIn opens the sign-in, which Router mounts. An instance whose
// sign-in does not open must not start, since it could not tell anybody apart.
func (h *Handler) OpenSignIn() error {
	if h.auth != nil {
		return nil
	}
	return h.openSignIn()
}

// signInBroken answers every request of a Router whose sign-in did not open.
func signInBroken() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": "authentication unavailable"})
	})
}

// mountSignIn serves the kit's contract and the pages a browser is sent to on
// the way to a sign-in service and back.
func (h *Handler) mountSignIn(mux *http.ServeMux) {
	if h.auth == nil {
		return
	}
	mux.Handle(signin.APIPrefix, h.auth.Handler())
	mux.Handle(signin.BrowserPrefix, h.auth.Handler())
}

// handleLogout signs a paired phone out on this instance. A browser signs
// out through the kit.
// POST /api/logout
func (h *Handler) handleLogout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, h.newSessionCookie("", -1))
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// signInOpen reports whether nobody can sign in, so every visitor is the
// owner. A handler built without a sign-in counts as open.
func (h *Handler) signInOpen() bool {
	return h.auth == nil || h.auth.Anyone()
}

// newAccountID gives the first account the owner's id, and every account a
// sign-in service adds later one of its own.
func (h *Handler) newAccountID() (string, error) {
	if h.auth != nil {
		_, err := h.auth.Account(context.Background(), ownerID)
		if errors.Is(err, signin.ErrNotFound) {
			return ownerID, nil
		}
		if err != nil {
			return "", err
		}
	}
	return strings.ToLower(rand.Text()), nil
}

// visitor is who the guard found for r.
func visitor(r *http.Request) (signin.Visitor, bool) {
	return signin.From(r.Context())
}

// creator is the account a key, grant or phone r makes answers to. Empty is
// the owner, who also stands for everybody while nobody can sign in.
func creator(r *http.Request) string {
	v, found := visitor(r)
	if !found || v.Via == signin.ViaAnyone || v.Account == ownerID {
		return ""
	}
	return v.Account
}

// requireSignIn refuses the session of a paired phone and an API token where
// somebody can sign in. Signing in at the web interface and sending the
// request again is how the caller gets past it. While nobody can sign in
// there is nothing to tell apart, and it lets every request through.
func (h *Handler) requireSignIn(w http.ResponseWriter, r *http.Request, action string) bool {
	if h.signInOpen() {
		return true
	}
	if v, ok := visitor(r); ok && v.Via != signin.ViaDoor && v.Via != signin.ViaToken {
		return true
	}
	writeJSON(w, http.StatusForbidden, map[string]any{
		"ok":    false,
		"error": "sign in at this instance's web interface before " + action,
		"code":  codeOwnLoginNeeded,
	})
	return false
}

// mint runs add, which adds a lasting way in such as a key, for a person
// who signed in moments ago. It holds off a reset while add runs. A sign-in
// that is older is answered as the kit answers it, so the page asks for the
// password and sends the request again.
func (h *Handler) mint(w http.ResponseWriter, r *http.Request, action string, add func() error) (done bool, err error) {
	if !h.requireSignIn(w, r, action) {
		return false, nil
	}
	if h.auth == nil {
		return true, add()
	}
	err = h.auth.Mint(r, add)
	if errors.Is(err, signin.ErrNotFresh) {
		h.auth.SignInAgain(w, r)
		return false, nil
	}
	return true, err
}

// fresh reports whether the person signed in moments ago, for what shows a
// secret, and answers the request itself when not.
func (h *Handler) fresh(w http.ResponseWriter, r *http.Request, action string) bool {
	done, err := h.mint(w, r, action, func() error { return nil })
	return done && err == nil
}

// The codes a 403 carries when a request needs more than its session: the
// person has to sign in themselves, or a phone asks for a session while
// pairing phones is switched off.
const (
	codeOwnLoginNeeded  = "own-login-needed"
)

// importLogin hands the kit the login the version before kept: the password
// hash, the second factor and its recovery codes, and the passkeys. Without
// a password there is nobody to import, and the app stays open.
func (h *Handler) importLogin(_ context.Context) ([]signin.Imported, error) {
	s, err := h.store.GetSettings()
	if err != nil {
		return nil, err
	}
	if s.AuthPasswordHash == "" {
		return nil, nil
	}
	owner := signin.Imported{
		ID: ownerID, Name: "admin", Role: "admin", Owner: true,
		Password: s.AuthPasswordHash,
		Handle:   passkeyUserID(h.cfg.AppKey),
	}
	// A second factor sealed under another APP_KEY does not open, and neither
	// does the password then. The reset through the log is the way back in.
	if plain, err := h.decryptTOTPSecret(s.TOTPSecret); s.TOTPEnabled && err == nil {
		if owner.TOTP, err = secret.TOTPKey(plain); err != nil {
			return nil, err
		}
		owner.Recovery = decodeRecoveryCodes(s.TOTPRecovery)
	}
	keys, err := h.store.ListPasskeys()
	if err != nil {
		return nil, err
	}
	for _, p := range keys {
		owner.Passkeys = append(owner.Passkeys, signin.ImportedPasskey{
			Name: p.Name, ID: p.CredentialID, PublicKey: p.PublicKey, SignCount: p.SignCount, AAGUID: p.AAGUID,
			Transports:     strings.FieldsFunc(p.Transports, func(c rune) bool { return c == ',' || c == ' ' }),
			BackupEligible: p.BackedUp, BackupState: p.BackedUp, AttestationType: "none", RPID: p.RPID,
			Created: unixTime(p.CreatedAt), Used: unixTime(p.LastUsedAt),
		})
	}
	if err := h.seedPublicAddress(keys); err != nil {
		return nil, err
	}
	log.Printf("signin: took over the login password, %d passkey(s) and two-factor authentication %s", len(keys), onOff(s.TOTPEnabled))
	return []signin.Imported{owner}, nil
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

func unixTime(sec int64) time.Time {
	if sec == 0 {
		return time.Time{}
	}
	return time.Unix(sec, 0)
}

// passkeyUserID is the user handle the version before registered every
// passkey with. A browser finds a passkey only under its handle.
func passkeyUserID(appKey string) []byte {
	sum := sha256.Sum256([]byte("bombvault-passkey-user:" + appKey))
	return sum[:]
}

// oldSession reads the session cookie of the version before, signed with the
// password hash and the session epoch it kept. A paired phone's session is
// the phone door's.
func (h *Handler) oldSession(r *http.Request) (signin.OldSession, bool) {
	c, err := h.sessionCookie(r)
	if err != nil || secret.PairedSessionToken(c.Value) {
		return signin.OldSession{}, false
	}
	s, err := h.store.GetSettings()
	if err != nil || s.AuthPasswordHash == "" || !secret.ValidSessionToken(h.cfg.AppKey, s.AuthPasswordHash, s.SessionEpoch, c.Value) {
		return signin.OldSession{}, false
	}
	exp, _, _ := strings.Cut(c.Value, ".")
	sec, err := strconv.ParseInt(exp, 10, 64)
	if err != nil {
		return signin.OldSession{}, false
	}
	expires := time.Unix(sec, 0)
	sum := sha256.Sum256([]byte(c.Value))
	// Every session lasted sessionTTL from its sign-in.
	return signin.OldSession{
		Account: ownerID, Key: hex.EncodeToString(sum[:]),
		Issued: expires.Add(-sessionTTL), Expires: expires, Via: signin.ViaPassword,
	}, true
}

// dropOldLogin clears the login of the version before once the kit ended
// one of the owner's ways in, so a settings restore of that version's
// columns cannot bring an ended password or passkey back.
func (h *Handler) dropOldLogin(_ context.Context, account string, what signin.Ending) error {
	if account != ownerID || what == signin.EndResetOther {
		return nil
	}
	return h.store.DropOldLogin(what != signin.EndOpened)
}

// signInEnded ends BombVault's own ways in when the kit ended the owner's.
// It runs again until it returns nil, so every step takes what is gone
// already as done.
func (h *Handler) signInEnded(ctx context.Context, account string, what signin.Ending) error {
	switch what {
	case signin.EndBlocked, signin.EndRemoved, signin.EndRole, signin.EndRefused:
		if account != ownerID {
			return h.endWaysOf(account, what)
		}
	}
	acc, err := h.auth.Account(ctx, account)
	if errors.Is(err, signin.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !acc.Owner {
		return nil
	}
	switch what {
	case signin.EndFirstPassword, signin.EndPassword, signin.EndReset:
		if err := h.store.MarkOldLogin(acc.Password); err != nil {
			return err
		}
	case signin.EndOpened:
		if err := h.store.MarkOldLogin(false); err != nil {
			return err
		}
	}
	switch what {
	case signin.EndReset, signin.EndRevoked:
		return h.endEveryOwnWay(what)
	case signin.EndFirstPassword, signin.EndPassword, signin.EndEverywhere, signin.EndOpened:
		return h.endPhoneSessions()
	}
	return nil
}

// endEveryOwnWay leaves whoever reset the password alone in the instance:
// every MCP key, API token and OAuth grant goes and the phones' sessions end.
// A phone of the group pairs again with the twelve words, and the Instances
// page is where one is kept out.
func (h *Handler) endEveryOwnWay(what signin.Ending) error {
	h.ownWays.Lock()
	defer h.ownWays.Unlock()
	n, err := h.store.RevokeAllMCPKeys("password-reset", time.Now().Unix())
	if err != nil {
		return err
	}
	if h.mcp != nil {
		h.mcp.oauth.dropCodes()
	}
	if err := h.endPhoneSessions(); err != nil {
		return err
	}
	log.Printf("signin: %s: revoked %d MCP key(s), API token(s) and OAuth grant(s), ended the phones' sessions", what, n)
	return nil
}

// endWaysOf ends what account made or let in once it may no longer: its
// keys, tokens and grants, the codes it allowed and the phones it approved.
func (h *Handler) endWaysOf(account string, what signin.Ending) error {
	h.ownWays.Lock()
	defer h.ownWays.Unlock()
	now := time.Now().Unix()
	n, err := h.store.RevokeMCPKeysOf(account, "account-"+string(what), now)
	if err != nil {
		return err
	}
	if h.mcp != nil {
		h.mcp.oauth.dropCodesOf(account)
	}
	if err := h.store.EndPhonesOf(account, now); err != nil {
		return err
	}
	log.Printf("signin: account %s %s: revoked %d MCP key(s), API token(s) and OAuth grant(s) and ended its phones", account, what, n)
	return nil
}

// endPhoneSessions ends the session of every paired phone, which is signed
// with the session epoch.
func (h *Handler) endPhoneSessions() error {
	epoch, err := newSessionEpoch()
	if err != nil {
		return err
	}
	_, err = h.store.MutateSettings(func(s *store.Settings) error {
		s.SessionEpoch = epoch
		return nil
	})
	return err
}

// newSessionEpoch returns a fresh random session epoch (16 bytes, hex).
func newSessionEpoch() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate session epoch: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// phoneDoor lets a paired phone in as the account that approved it, with the
// session this instance gave it, for as long as the phone keeps its row.
// authGate holds it to the routes a phone reaches.
func (h *Handler) phoneDoor(r *http.Request) (signin.Visitor, bool) {
	c, err := h.sessionCookie(r)
	if err != nil || !secret.PairedSessionToken(c.Value) {
		return signin.Visitor{}, false
	}
	phone, ok := secret.PairedSessionPhone(c.Value)
	if !ok || !h.phoneSessionValid(phone, c.Value) {
		return signin.Visitor{}, false
	}
	if !h.pairedPhoneListed(c.Value) {
		return signin.Visitor{}, false
	}
	account, ok := h.phoneAccount(phone)
	if !ok {
		return signin.Visitor{}, false
	}
	return signin.Visitor{Account: account}, true
}

// phoneAccount is the account phone answers to, which the sign-in then holds
// to its block and removal. A phone whose account ended stays out until it is
// let in again.
func (h *Handler) phoneAccount(phone string) (string, bool) {
	account, ended, err := h.store.PhoneAccount(phone)
	if err != nil || ended {
		return "", false
	}
	if account == "" {
		return ownerID, true
	}
	return account, true
}

// phoneStands reports whether phone's account still lets it in, for a page
// over the relay, which the sign-in does not see.
func (h *Handler) phoneStands(phone string) bool {
	account, ok := h.phoneAccount(phone)
	if !ok || h.auth == nil {
		return ok
	}
	acc, err := h.auth.Account(context.Background(), account)
	return err == nil && !acc.Blocked
}

// phoneSessionValid reports whether token is a session this instance gave
// phoneID and nothing has ended since. One from the version before the kit
// is signed with the password hash it kept, until the kit drops it.
func (h *Handler) phoneSessionValid(phoneID, token string) bool {
	if phone, ok := secret.PairedSessionPhone(token); !ok || phone != phoneID {
		return false
	}
	s, err := h.store.GetSettings()
	if err != nil {
		return false
	}
	return secret.ValidSessionToken(h.cfg.AppKey, "", s.SessionEpoch, token) ||
		(s.AuthPasswordHash != "" && secret.ValidSessionToken(h.cfg.AppKey, s.AuthPasswordHash, s.SessionEpoch, token))
}

// signInLog is where the sign-in logs what goes wrong inside it.
var signInLog = slog.Default

// resetCodeHandler writes the reset code the kit logs as a block of its own
// on stdout, never into the log ring, which the diagnostics bundle copies.
// Everything else goes on to next.
type resetCodeHandler struct {
	out  func() io.Writer
	next slog.Handler
}

// Enabled lets every warning through, since the reset code comes as one and
// goes to stdout whatever next takes.
func (h resetCodeHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return l >= slog.LevelWarn || h.next.Enabled(ctx, l)
}

func (h resetCodeHandler) Handle(ctx context.Context, rec slog.Record) error {
	code := ""
	rec.Attrs(func(a slog.Attr) bool {
		if a.Key == "code" {
			code = a.Value.String()
			return false
		}
		return true
	})
	if code == "" {
		if !h.next.Enabled(ctx, rec.Level) {
			return nil
		}
		return h.next.Handle(ctx, rec)
	}
	printResetCode(h.out(), strings.ReplaceAll(code, "-", ""))
	return nil
}

func (h resetCodeHandler) WithAttrs(as []slog.Attr) slog.Handler {
	return resetCodeHandler{out: h.out, next: h.next.WithAttrs(as)}
}

func (h resetCodeHandler) WithGroup(name string) slog.Handler {
	return resetCodeHandler{out: h.out, next: h.next.WithGroup(name)}
}

// printResetCode writes the code as a block of its own in bold yellow, set
// off like the ready line, so it stands out between the timestamped lines of
// a busy log.
func printResetCode(w io.Writer, code string) {
	if len(code) != 12 {
		_, _ = fmt.Fprintf(w, "\n  \033[1;33m▶ PASSWORD RESET CODE: %s\033[0m\n\n", code)
		return
	}
	_, _ = fmt.Fprintf(w, "\n  \033[1;33m▶ PASSWORD RESET CODE: %s-%s-%s\033[0m\n"+
		"    Valid for 10 minutes. Enter it on the sign-in page.\n"+
		"    If nobody here asked for a reset, ignore this. The password is unchanged.\n\n",
		code[:4], code[4:8], code[8:])
}

// publicURL is where people reach this instance, for passkeys and sign-in
// services. Nil while nobody set it.
func (h *Handler) publicURL() *url.URL {
	return h.public.Load()
}

func (h *Handler) loadPublicAddress() error {
	raw, err := h.store.PublicAddress()
	if err != nil {
		return err
	}
	h.setPublic(raw)
	return nil
}

func (h *Handler) setPublic(raw string) {
	if u, err := url.Parse(raw); err == nil && raw != "" {
		h.public.Store(u)
		return
	}
	h.public.Store(nil)
}

// seedPublicAddress takes the address of an install from before there was
// one: the issuer MCP's sign-in runs under, or else where the passkey used
// last belongs, so passkeys keep working where they were made.
func (h *Handler) seedPublicAddress(keys []store.Passkey) error {
	if cur, err := h.store.PublicAddress(); err != nil || cur != "" {
		return err
	}
	addr := ""
	if o, err := h.store.MCPOAuthSettings(); err == nil && o.Issuer != "" {
		addr = o.Issuer
	}
	if addr == "" {
		var last *store.Passkey
		for i, p := range keys {
			if last == nil || p.LastUsedAt > last.LastUsedAt || (p.LastUsedAt == last.LastUsedAt && p.CreatedAt > last.CreatedAt) {
				last = &keys[i]
			}
		}
		if last != nil {
			addr, _ = normalizePublicAddress("https://" + last.RPID)
		}
	}
	if addr == "" {
		return nil
	}
	if err := h.store.SetPublicAddress(addr); err != nil {
		return err
	}
	h.setPublic(addr)
	return nil
}

// normalizePublicAddress spells an address the way browsers send their
// origin: http or https, a lower-case host and no default port, path or
// query.
func normalizePublicAddress(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || strings.ContainsAny(raw, "?#") || u.Opaque != "" || u.User != nil || (u.Path != "" && u.Path != "/") {
		return "", false
	}
	scheme := strings.ToLower(u.Scheme)
	host, port := strings.ToLower(u.Hostname()), u.Port()
	if host == "" || (scheme != "https" && scheme != "http") {
		return "", false
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port != "" && (scheme != "https" || port != "443") && (scheme != "http" || port != "80") {
		host += ":" + port
	}
	return scheme + "://" + host, true
}

// handleGetPublicAddress says where people reach this instance.
// GET /api/address
func (h *Handler) handleGetPublicAddress(w http.ResponseWriter, _ *http.Request) {
	addr, err := h.store.PublicAddress()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"address": addr}))
}

// handleSetPublicAddress stores where people reach this instance. Passkeys
// work at that address only, and sign-in services send people back there.
// PUT /api/address
func (h *Handler) handleSetPublicAddress(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Address string `json:"address"`
	}
	if !decodeBody(w, r, &body) || !h.requireSignIn(w, r, "changing the address") {
		return
	}
	addr := ""
	if strings.TrimSpace(body.Address) != "" {
		var ok bool
		if addr, ok = normalizePublicAddress(body.Address); !ok {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "code": "address-invalid", "error": "that is not an address such as https://backup.example"})
			return
		}
	}
	if err := h.store.SetPublicAddress(addr); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	h.setPublic(addr)
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"address": addr}))
}
