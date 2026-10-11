package api

import (
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// A forgotten login password is reset with a code that BombVault writes to its
// own container log. Reading that log takes access to the Docker host, which is
// root on it already, so the code proves more than the password did.
const (
	resetCodeTTL = 10 * time.Minute
	// A code holds 40 bits, so five tries leave a guess no chance worth
	// counting.
	resetCodeTries = 5
	resetCodeLen   = 8
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
	tries   int
}

func newPasswordReset() (passwordReset, error) {
	buf := make([]byte, resetCodeLen)
	if _, err := rand.Read(buf); err != nil {
		return passwordReset{}, fmt.Errorf("generate reset code: %w", err)
	}
	for i, b := range buf {
		buf[i] = resetCodeAlphabet[b&31]
	}
	return passwordReset{code: string(buf), expires: time.Now().Add(resetCodeTTL), tries: resetCodeTries}, nil
}

func (p *passwordReset) live() bool {
	return p.code != "" && time.Now().Before(p.expires)
}

// guess reports whether typed is the code and spends the code if it is. A
// wrong guess costs one try. live is false when there was no code to guess at:
// none was asked for, it expired, or its tries are used up.
func (p *passwordReset) guess(typed string) (ok, live bool) {
	if !p.live() {
		*p = passwordReset{}
		return false, false
	}
	// The log prints the code in capitals with a dash in the middle.
	typed = strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(typed))
	if subtle.ConstantTimeCompare([]byte(typed), []byte(p.code)) == 1 {
		*p = passwordReset{}
		return true, true
	}
	if p.tries--; p.tries == 0 {
		*p = passwordReset{}
	}
	return false, true
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
		h.resetLog.Printf("password reset: the code is %s-%s and stays valid for %d minutes. Enter it on the login page. If nobody here asked for it, ignore this line; the password is unchanged.",
			fresh.code[:4], fresh.code[4:], int(resetCodeTTL.Minutes()))
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
	key := h.loginClientKey(r)
	if h.loginThrottled(key) {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"ok": false, "error": "too many failed attempts, wait a minute and try again"})
		return
	}

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
	ok, live := h.reset.guess(body.Code)
	h.resetMu.Unlock()
	if !live {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "code": "resetCodeExpired", "error": "that code is no longer valid"})
		return
	}
	if !ok {
		h.recordLoginFail(key)
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "code": "resetCodeInvalid", "error": "that code is not right"})
		return
	}

	hash, err := secret.HashPassword(h.cfg.AppKey, body.Password)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	epoch := ""
	if _, err := h.store.MutateSettings(func(s *store.Settings) error {
		next, err := newSessionEpoch()
		if err != nil {
			return err
		}
		s.AuthPasswordHash = hash
		// Whoever lost the password may have lost the authenticator with it.
		s.TOTPEnabled = false
		s.TOTPSecret = ""
		s.TOTPRecovery = ""
		s.SessionEpoch = next
		epoch = next
		return nil
	}); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	h.recordLoginSuccess(key)
	log.Printf("api: the login password was reset with a code from the container log")

	tok := secret.NewSessionToken(h.cfg.AppKey, hash, epoch, sessionTTL)
	http.SetCookie(w, h.newSessionCookie(tok, int(sessionTTL.Seconds())))
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}
