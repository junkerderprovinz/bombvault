package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const (
	resetStartPath  = "/api/auth/reset"
	resetFinishPath = "/api/auth/reset/finish"
	resetNewPass    = "correct horse battery"
)

var loggedResetCode = regexp.MustCompile(`RESET CODE: ([2-9A-Z]{4}(?:-[2-9A-Z]{4})+)`)

const wrongResetCode = "2222-2222-2222"

// newResetHandler is a handler with a login password and a buffer in place of
// the container log.
func newResetHandler(t *testing.T) (*Handler, *store.Repo, *bytes.Buffer) {
	t.Helper()
	h, repo, _ := newAuthGateHandler(t)
	enableAuth(t, h, repo)
	out := &bytes.Buffer{}
	h.resetOut = out
	return h, repo, out
}

// askForResetCode presses "Forgot password?" and reads the code off the log.
func askForResetCode(t *testing.T, h *Handler, out *bytes.Buffer) string {
	t.Helper()
	out.Reset()
	_, body := postJSON(t, h.handleStartPasswordReset, resetStartPath, `{}`, "10.0.0.1:1", nil)
	if body["ok"] != true {
		t.Fatalf("asking for a code must succeed: %v", body)
	}
	m := loggedResetCode.FindStringSubmatch(out.String())
	if m == nil {
		t.Fatalf("no code in the log: %q", out.String())
	}
	return m[1]
}

func finishReset(t *testing.T, h *Handler, code, password string) (map[string]any, *http.Response) {
	t.Helper()
	return finishResetFrom(t, h, "10.0.0.1:1", code, password)
}

func finishResetFrom(t *testing.T, h *Handler, addr, code, password string) (map[string]any, *http.Response) {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, resetFinishPath,
		strings.NewReader(`{"code":"`+code+`","password":"`+password+`"}`))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = addr
	w := httptest.NewRecorder()
	h.handleFinishPasswordReset(w, r)
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v (%s)", err, w.Body.String())
	}
	return body, w.Result()
}

func signsIn(t *testing.T, h *Handler, password string) bool {
	t.Helper()
	_, body := postJSON(t, h.handleLogin, "/api/login", `{"password":"`+password+`"}`, "10.0.0.9:1", nil)
	return body["ok"] == true
}

func TestTheResetCodeGoesToTheLogAndNotToTheBrowser(t *testing.T) {
	h, _, out := newResetHandler(t)

	out.Reset()
	r := httptest.NewRequest(http.MethodPost, resetStartPath, strings.NewReader(`{}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleStartPasswordReset(w, r)

	m := loggedResetCode.FindStringSubmatch(out.String())
	if m == nil {
		t.Fatalf("no code in the log: %q", out.String())
	}
	bare := strings.ReplaceAll(m[1], "-", "")
	if strings.Contains(w.Body.String(), m[1]) || strings.Contains(w.Body.String(), bare) {
		t.Fatalf("the answer to the browser carries the code: %s", w.Body.String())
	}
}

func TestTheResetCodeStaysOutOfTheOrdinaryLog(t *testing.T) {
	h, _, out := newResetHandler(t)
	ordinary := &bytes.Buffer{}
	prev, flags := log.Writer(), log.Flags()
	log.SetOutput(ordinary)
	t.Cleanup(func() { log.SetOutput(prev); log.SetFlags(flags) })

	code := askForResetCode(t, h, out)
	finishReset(t, h, code, resetNewPass)

	if strings.Contains(ordinary.String(), code) {
		t.Fatalf("the code reached the log the diagnostics bundle copies: %q", ordinary.String())
	}
}

func TestResetWithTheCodeSetsThePasswordAndSignsIn(t *testing.T) {
	h, repo, out := newResetHandler(t)
	code := askForResetCode(t, h, out)

	body, res := finishReset(t, h, code, resetNewPass)
	if body["ok"] != true {
		t.Fatalf("the right code must reset the password: %v", body)
	}
	s, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	var session string
	for _, c := range res.Cookies() {
		if c.Name == h.sessionCookieNameFor() {
			session = c.Value
		}
	}
	if !secret.ValidSessionToken(h.cfg.AppKey, s.AuthPasswordHash, s.SessionEpoch, session) {
		t.Fatal("the reset must leave the browser signed in")
	}
	if !signsIn(t, h, resetNewPass) {
		t.Fatal("the new password must sign in")
	}
	if signsIn(t, h, "hunter2") {
		t.Fatal("the old password must not sign in any more")
	}
}

func TestResetSwitchesTheSecondFactorOff(t *testing.T) {
	h, repo, out := newResetHandler(t)
	storeTOTP(t, h, repo, testTOTPSecret, []string{"deadbeef"})
	code := askForResetCode(t, h, out)

	finishReset(t, h, code, resetNewPass)

	s, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.TOTPEnabled || s.TOTPSecret != "" || s.TOTPRecovery != "" {
		t.Fatal("a lost authenticator must not keep the owner out after a reset")
	}
}

func TestResetEndsEveryOtherSession(t *testing.T) {
	h, repo, out := newResetHandler(t)
	before, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	old := secret.NewSessionToken(h.cfg.AppKey, before.AuthPasswordHash, before.SessionEpoch, sessionTTL)
	code := askForResetCode(t, h, out)

	finishReset(t, h, code, resetNewPass)

	after, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if secret.ValidSessionToken(h.cfg.AppKey, after.AuthPasswordHash, after.SessionEpoch, old) {
		t.Fatal("a session from before the reset is still valid")
	}
}

func TestAWrongResetCodeChangesNothing(t *testing.T) {
	h, repo, out := newResetHandler(t)
	askForResetCode(t, h, out)
	before, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	body, res := finishReset(t, h, wrongResetCode, resetNewPass)

	if body["ok"] != false || body["code"] != "resetCodeInvalid" {
		t.Fatalf("a wrong code must be refused as invalid: %v", body)
	}
	if len(res.Cookies()) != 0 {
		t.Fatal("a wrong code must not issue a session")
	}
	after, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if after.AuthPasswordHash != before.AuthPasswordHash || after.SessionEpoch != before.SessionEpoch {
		t.Fatal("a wrong code changed the stored login")
	}
}

func TestAResetCodeWorksOnce(t *testing.T) {
	h, _, out := newResetHandler(t)
	code := askForResetCode(t, h, out)
	finishReset(t, h, code, resetNewPass)

	body, _ := finishReset(t, h, code, "another long password")

	if body["ok"] != false || body["code"] != "resetCodeExpired" {
		t.Fatalf("a spent code must be refused: %v", body)
	}
	if !signsIn(t, h, resetNewPass) {
		t.Fatal("the second use of the code changed the password")
	}
}

func TestAResetCodeExpires(t *testing.T) {
	h, _, out := newResetHandler(t)
	code := askForResetCode(t, h, out)
	h.reset.expires = time.Now().Add(-time.Second)

	body, _ := finishReset(t, h, code, resetNewPass)

	if body["ok"] != false || body["code"] != "resetCodeExpired" {
		t.Fatalf("an expired code must be refused: %v", body)
	}
	if !signsIn(t, h, "hunter2") {
		t.Fatal("an expired code changed the password")
	}
}

func TestAResetCodeHasSixtyBits(t *testing.T) {
	h, _, out := newResetHandler(t)

	code := strings.ReplaceAll(askForResetCode(t, h, out), "-", "")

	if len(code) != 12 {
		t.Fatalf("code %q has %d symbols, want 12 of five bits each", code, len(code))
	}
}

// Anybody who reaches the login page can send wrong codes, so they must not
// be able to use up the code the owner is reading off the log.
func TestWrongGuessesLeaveTheResetCodeAlone(t *testing.T) {
	h, _, out := newResetHandler(t)
	code := askForResetCode(t, h, out)
	for i := range 3 * loginMaxFails {
		finishResetFrom(t, h, fmt.Sprintf("192.0.2.%d:1", i), wrongResetCode, resetNewPass)
	}

	body, _ := finishReset(t, h, code, resetNewPass)

	if body["ok"] != true {
		t.Fatalf("the owner's code must still work after wrong guesses: %v", body)
	}
}

// Behind a reverse proxy without TRUSTED_PROXY every caller shares the proxy's
// address, and with it one throttle bucket.
func TestAStrangerBehindTheSameProxyCannotBlockTheRightResetCode(t *testing.T) {
	h, _, out := newResetHandler(t)
	code := askForResetCode(t, h, out)
	for range loginMaxFails {
		postJSON(t, h.handleLogin, "/api/login", `{"password":"not the password"}`, "10.0.0.1:2", nil)
		finishResetFrom(t, h, "10.0.0.1:3", wrongResetCode, resetNewPass)
	}

	body, res := finishResetFrom(t, h, "10.0.0.1:4", code, resetNewPass)

	if res.StatusCode != http.StatusOK || body["ok"] != true {
		t.Fatalf("status %d, %v: the right code must pass a throttle strangers filled", res.StatusCode, body)
	}
}

func TestAWrongResetCodeCountsTowardsTheThrottle(t *testing.T) {
	h, _, out := newResetHandler(t)
	askForResetCode(t, h, out)
	for range loginMaxFails {
		finishReset(t, h, wrongResetCode, resetNewPass)
	}

	_, res := finishReset(t, h, wrongResetCode, resetNewPass)

	if res.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want the login throttle to answer", res.StatusCode)
	}
}

func TestAskingAgainKeepsTheLiveCode(t *testing.T) {
	h, _, out := newResetHandler(t)
	code := askForResetCode(t, h, out)

	out.Reset()
	_, body := postJSON(t, h.handleStartPasswordReset, resetStartPath, `{}`, "10.0.0.2:1", nil)

	if body["ok"] != true || out.Len() != 0 {
		t.Fatalf("a second request must neither fail nor write to the log: %v %q", body, out.String())
	}
	if got, _ := finishReset(t, h, code, resetNewPass); got["ok"] != true {
		t.Fatalf("the first code must still work: %v", got)
	}
}

func TestResetNeedsALoginPassword(t *testing.T) {
	h, _, _ := newAuthGateHandler(t)
	out := &bytes.Buffer{}
	h.resetOut = out

	_, body := postJSON(t, h.handleStartPasswordReset, resetStartPath, `{}`, "10.0.0.1:1", nil)

	if body["ok"] != false || out.Len() != 0 {
		t.Fatalf("with the login off there is nothing to reset: %v %q", body, out.String())
	}
}

func TestAShortPasswordIsRefusedAndKeepsTheCode(t *testing.T) {
	h, _, out := newResetHandler(t)
	code := askForResetCode(t, h, out)

	body, _ := finishReset(t, h, code, "short")
	if body["ok"] != false {
		t.Fatalf("a short password must be refused: %v", body)
	}

	if got, _ := finishReset(t, h, code, resetNewPass); got["ok"] != true {
		t.Fatalf("the code must survive a password that was too short: %v", got)
	}
}

func TestTheResetCodeIsAcceptedAsTyped(t *testing.T) {
	h, _, out := newResetHandler(t)
	code := askForResetCode(t, h, out)
	typed := " " + strings.ToLower(strings.ReplaceAll(code, "-", "")) + " "

	if body, _ := finishReset(t, h, typed, resetNewPass); body["ok"] != true {
		t.Fatalf("lower case without the dash must be accepted: %v", body)
	}
}

func TestResetIsReachableWithoutASession(t *testing.T) {
	for _, p := range []string{resetStartPath, resetFinishPath} {
		if !authGatePublicPath(p) {
			t.Errorf("%s is behind the login it is meant to recover", p)
		}
	}
}

func TestALockedOutOwnerResetsThePasswordThroughTheRouter(t *testing.T) {
	in := newInstance(t, "alpha", strings.Repeat("a", 64))
	in.session = ""
	out := &bytes.Buffer{}
	in.h.resetOut = out

	if status, _ := in.do(t, http.MethodGet, "/api/settings", nil); status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want the instance locked before the reset", status)
	}
	if _, body := in.do(t, http.MethodPost, resetStartPath, map[string]any{}); body["ok"] != true {
		t.Fatalf("asking for a code without a session: %v", body)
	}
	m := loggedResetCode.FindStringSubmatch(out.String())
	if m == nil {
		t.Fatalf("no code in the log: %q", out.String())
	}
	reset := map[string]any{"code": m[1], "password": resetNewPass}
	if _, body := in.do(t, http.MethodPost, resetFinishPath, reset); body["ok"] != true {
		t.Fatalf("resetting without a session: %v", body)
	}
	if _, body := in.do(t, http.MethodPost, "/api/login", map[string]any{"password": resetNewPass}); body["ok"] != true {
		t.Fatalf("the new password must sign in: %v", body)
	}
}
