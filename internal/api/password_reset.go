package api

import (
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// A forgotten login password is reset with a code that BombVault writes to its
// own container log. Whoever can read that log can reset the password: root on
// the host, but also a log viewer such as Dozzle, anything given the Docker
// socket read-only and a log shipper that forwards container output. Somebody
// who only reaches the web interface never sees the code.
const (
	resetCodeTTL = 10 * time.Minute
	// A code holds 60 bits, which nobody finds in ten minutes at any rate
	// HTTP allows. That is why a wrong guess does not spend it: a code that a
	// few guesses killed would let anybody who reaches the login page keep the
	// owner from ever using one.
	resetCodeLen = 12
	// resetCodeAlphabet has no 0, O, 1 or I, which look alike in a log window.
	// Its 32 symbols take five bits of a random byte each without bias.
	resetCodeAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
)

// passwordReset is the one code that can be live at a time. It is kept in
// memory like a passkey ceremony, so a restart cancels a reset that was under
// way.
type passwordReset struct {
	code    string
	expires time.Time
}

func newPasswordReset() (passwordReset, error) {
	buf := make([]byte, resetCodeLen)
	if _, err := rand.Read(buf); err != nil {
		return passwordReset{}, fmt.Errorf("generate reset code: %w", err)
	}
	for i, b := range buf {
		buf[i] = resetCodeAlphabet[b&31]
	}
	return passwordReset{code: string(buf), expires: time.Now().Add(resetCodeTTL)}, nil
}

// printResetCode writes the code as a block of its own in bold yellow, set
// off like the ready line, so it stands out between the timestamped lines of a
// busy log.
func printResetCode(w io.Writer, code string) {
	_, _ = fmt.Fprintf(w, "\n  \033[1;33m▶ PASSWORD RESET CODE: %s-%s-%s\033[0m\n"+
		"    Valid for %d minutes. Enter it on the login page.\n"+
		"    If nobody here asked for a reset, ignore this. The password is unchanged.\n\n",
		code[:4], code[4:8], code[8:], int(resetCodeTTL.Minutes()))
}

func (p *passwordReset) live() bool {
	return p.code != "" && time.Now().Before(p.expires)
}

// noResetCode stands in for the code while none is live, so a guess costs the
// same comparison either way. It has the length of a code and symbols no code
// has.
const noResetCode = "000000000000"

// guess reports whether typed is the live code and spends the code if it is.
// A wrong guess leaves the code as it was. stale reports that no code was live
// to guess at, which only the log is told.
func (p *passwordReset) guess(typed string) (ok, stale bool) {
	stale = !p.live()
	want := p.code
	if stale {
		*p = passwordReset{}
		want = noResetCode
	}
	// The log prints the code in capitals in groups of four.
	typed = strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(typed))
	if subtle.ConstantTimeCompare([]byte(typed), []byte(want)) == 1 && !stale {
		*p = passwordReset{}
		return true, false
	}
	return false, stale
}

// resetRefusals counts refused reset codes for a log line written at most
// once a minute, so somebody guessing shows in the log without filling it.
type resetRefusals struct {
	wrong, stale int
	reported     time.Time
}

func (c *resetRefusals) add(stale bool) {
	if stale {
		c.stale++
	} else {
		c.wrong++
	}
	if time.Since(c.reported) < time.Minute {
		return
	}
	log.Printf("api: refused %d wrong password reset code(s) and %d entered while no code was live since the last report", c.wrong, c.stale)
	*c = resetRefusals{reported: time.Now()}
}

// holdSignIn is for a request that adds a way into the instance. It holds off
// a password reset until release is called and checks the session again under
// that hold, so what the request adds either exists before the reset removes
// it or is refused because the reset ended its session.
func (h *Handler) holdSignIn(w http.ResponseWriter, r *http.Request) (release func(), ok bool) {
	h.mintMu.RLock()
	if _, ok := h.sessionGeneration(r); !ok {
		h.mintMu.RUnlock()
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "authentication required"})
		return nil, false
	}
	return h.mintMu.RUnlock, true
}

// handleStartPasswordReset handles POST /api/auth/reset. It writes a code to
// the container log unless one is still live, so asking again neither floods
// the log nor takes the code away from whoever is typing it.
func (h *Handler) handleStartPasswordReset(w http.ResponseWriter, _ *http.Request) {
	if _, _, on := h.authEnabled(); !on {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "authentication is not enabled"})
		return
	}
	h.resetMu.Lock()
	defer h.resetMu.Unlock()
	if !h.reset.live() {
		fresh, err := newPasswordReset()
		if err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
		h.reset = fresh
		printResetCode(h.resetOut, fresh.code)
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleFinishPasswordReset handles POST /api/auth/reset/finish.
// Body: {code string, password string}. The right code stores the password,
// switches the second factor off, ends every session and signs the caller in.
func (h *Handler) handleFinishPasswordReset(w http.ResponseWriter, r *http.Request) {
	if _, _, on := h.authEnabled(); !on {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "authentication is not enabled"})
		return
	}
	// A bucket of its own, which only wrong codes reach. Behind a reverse proxy
	// every caller shares one address, so a throttle in front of the right
	// code would let anybody keep the owner out; the code's sixty bits are what
	// stops a guesser.
	key := "reset|" + h.loginClientKey(r)

	var body struct {
		Code     string `json:"code"`
		Password string `json:"password"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	// Checked before the code, so a password that is too short does not cost
	// the code.
	if utf8.RuneCountInString(body.Password) < secret.MinPasswordLen {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     false,
			"error":  fmt.Sprintf("the password must be at least %d characters", secret.MinPasswordLen),
			"minLen": secret.MinPasswordLen,
		})
		return
	}

	h.resetMu.Lock()
	ok, stale := h.reset.guess(body.Code)
	if !ok {
		h.resetRefused.add(stale)
	}
	h.resetMu.Unlock()
	if !ok {
		if h.loginThrottled(key) {
			writeJSON(w, http.StatusTooManyRequests, map[string]any{"ok": false, "error": "too many failed attempts, wait a minute and try again"})
			return
		}
		h.recordLoginFail(key)
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "code": "resetCodeRejected", "error": "that code is wrong or has expired"})
		return
	}

	hash, err := secret.HashPassword(h.cfg.AppKey, body.Password)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	h.mintMu.Lock()
	removed, epoch, err := h.resetLogin(hash)
	h.mintMu.Unlock()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	h.recordLoginSuccess(key)
	log.Printf("api: the login password was reset with a code from the container log: %d passkey(s) removed, "+
		"%d API token(s) and %d MCP key(s) or OAuth grant(s) revoked, every other session ended",
		removed.Passkeys, removed.APITokens, removed.MCPKeys)

	tok := secret.NewSessionToken(h.cfg.AppKey, hash, epoch, sessionTTL)
	http.SetCookie(w, h.newSessionCookie(tok, int(sessionTTL.Seconds())))
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"removed": removed}))
}

// resetRemoved is what a reset took away, for the page the owner sees next.
type resetRemoved struct {
	Passkeys  int64 `json:"passkeys"`
	APITokens int   `json:"apiTokens"`
	MCPKeys   int   `json:"mcpKeys"`
	TwoFactor bool  `json:"twoFactor"`
}

// resetLogin leaves whoever holds the code alone in the instance: every key,
// token, OAuth grant and passkey goes, then the password, the second factor
// and every session. The caller holds mintMu, so nothing can be added in
// between. The keys go first, so a failure never leaves them beside a new
// password.
func (h *Handler) resetLogin(hash string) (resetRemoved, string, error) {
	var removed resetRemoved
	rows, err := h.store.ListMCPKeys()
	if err != nil {
		return removed, "", err
	}
	for _, k := range rows {
		switch {
		case k.RevokedAt != 0:
		case k.Kind == store.MCPKindAPI:
			removed.APITokens++
		default:
			removed.MCPKeys++
		}
	}
	if _, err := h.store.RevokeAllMCPKeys("password-reset", time.Now().Unix()); err != nil {
		return removed, "", err
	}
	h.mcp.oauth.dropCodes()
	if removed.Passkeys, err = h.store.DeleteAllPasskeys(); err != nil {
		return removed, "", err
	}

	epoch := ""
	_, err = h.store.MutateSettings(func(s *store.Settings) error {
		next, err := newSessionEpoch()
		if err != nil {
			return err
		}
		s.AuthPasswordHash = hash
		removed.TwoFactor = s.TOTPEnabled
		// Whoever lost the password may have lost the authenticator with it.
		s.TOTPEnabled = false
		s.TOTPSecret = ""
		s.TOTPRecovery = ""
		s.SessionEpoch = next
		epoch = next
		return nil
	})
	return removed, epoch, err
}
