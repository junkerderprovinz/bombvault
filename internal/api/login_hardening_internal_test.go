package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The three things v8.6.0 changed about signing in, and the one it added.
//
// Each of these was a real gap read out of this file, not a hypothetical:
// the password field had no minimum at all, the throttle collapsed into a
// single shared bucket the moment a reverse proxy was in front, and the stored
// hash was a fast MAC an offline attacker walks in hours.

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustCIDR(t *testing.T, s string) net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		t.Fatalf("ParseCIDR(%q): %v", s, err)
	}
	return *n
}

// postJSON drives a handler the way the SPA does and returns the decoded body.
func postJSON(t *testing.T, h http.HandlerFunc, path, body, remoteAddr string, hdr http.Header) (int, map[string]any) {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = remoteAddr
	for k, vs := range hdr {
		for _, v := range vs {
			r.Header.Add(k, v)
		}
	}
	w := httptest.NewRecorder()
	h(w, r)
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("response is not JSON: %v (%s)", err, w.Body.String())
	}
	return w.Code, out
}

// storeTOTP arms the second factor directly, the way a completed enrolment
// leaves the settings.
func storeTOTP(t *testing.T, h *Handler, repo *store.Repo, plainSecret string, recovery []string) {
	t.Helper()
	sealed, err := h.encryptTOTPSecret(plainSecret)
	if err != nil {
		t.Fatalf("encrypt totp secret: %v", err)
	}
	s, err := repo.GetSettings()
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	s.TOTPSecret = sealed
	s.TOTPEnabled = true
	s.TOTPRecovery = encodeRecoveryCodes(recovery)
	if err := repo.UpdateSettings(s); err != nil {
		t.Fatalf("update settings: %v", err)
	}
}

// ---------------------------------------------------------------------------
// The throttle behind a reverse proxy
// ---------------------------------------------------------------------------

// Without TRUSTED_PROXY nothing changes, and that is the point: believing a
// forwarded-for header by default would let any caller pick their own bucket.
func TestLoginKeyIgnoresForwardedHeaderByDefault(t *testing.T) {
	h, _, _ := newAuthGateHandler(t)
	r := httptest.NewRequest(http.MethodPost, "/api/login", nil)
	r.RemoteAddr = "203.0.113.9:5555"
	r.Header.Set("X-Forwarded-For", "10.9.9.9")
	if got := h.loginClientKey(r); got != "203.0.113.9" {
		t.Fatalf("key = %q, want the real peer 203.0.113.9", got)
	}
}

func TestLoginKeyUsesForwardedClientBehindATrustedProxy(t *testing.T) {
	h, _, _ := newAuthGateHandler(t)
	h.cfg.TrustedProxies = []net.IPNet{mustCIDR(t, "192.168.20.0/24")}
	r := httptest.NewRequest(http.MethodPost, "/api/login", nil)
	r.RemoteAddr = "192.168.20.11:44321"
	r.Header.Set("X-Forwarded-For", "203.0.113.7")
	if got := h.loginClientKey(r); got != "203.0.113.7" {
		t.Fatalf("key = %q, want the client the proxy named", got)
	}
}

// The header is only believed from the proxy. Somebody reaching the port
// directly must not inherit the trust by sending the same header.
func TestForwardedHeaderIsIgnoredFromAnUntrustedPeer(t *testing.T) {
	h, _, _ := newAuthGateHandler(t)
	h.cfg.TrustedProxies = []net.IPNet{mustCIDR(t, "192.168.20.0/24")}
	r := httptest.NewRequest(http.MethodPost, "/api/login", nil)
	r.RemoteAddr = "203.0.113.9:5555"
	r.Header.Set("X-Forwarded-For", "10.0.0.1")
	if got := h.loginClientKey(r); got != "203.0.113.9" {
		t.Fatalf("key = %q, want the untrusted peer's own address", got)
	}
}

// The chain is read right to left. An attacker controls the LEFT end, so a
// left-to-right reader would hand them a fresh bucket per request, which is the
// whole hole this setting was meant to avoid opening.
func TestForwardedChainIsReadFromTheRight(t *testing.T) {
	proxies := []net.IPNet{mustCIDR(t, "192.168.20.0/24"), mustCIDR(t, "10.0.0.0/8")}
	cases := []struct{ header, want string }{
		{"203.0.113.7", "203.0.113.7"},
		{"1.2.3.4, 203.0.113.7", "203.0.113.7"},
		{"203.0.113.7, 10.0.0.5", "203.0.113.7"},
		{"1.2.3.4, 203.0.113.7, 10.0.0.5, 192.168.20.11", "203.0.113.7"},
		// Every entry trusted: nothing left to name a client.
		{"10.0.0.5, 192.168.20.11", ""},
		{"", ""},
		// Garbage anywhere means the whole header is refused, so an attacker
		// cannot steer which entry we land on by inserting one.
		{"1.2.3.4, nonsense, 10.0.0.5", ""},
	}
	for _, c := range cases {
		if got := forwardedClient(c.header, proxies); got != c.want {
			t.Fatalf("forwardedClient(%q) = %q, want %q", c.header, got, c.want)
		}
	}
}

// The failure this whole setting exists to stop: two clients arriving through
// one proxy must not share a bucket, or an attacker's failures lock the
// operator out of their own instance.
func TestTwoClientsBehindOneProxyGetSeparateBuckets(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	h.cfg.TrustedProxies = []net.IPNet{mustCIDR(t, "192.168.20.0/24")}
	enableAuth(t, h, repo)

	attacker := http.Header{"X-Forwarded-For": []string{"203.0.113.7"}}
	operator := http.Header{"X-Forwarded-For": []string{"203.0.113.8"}}

	for i := range loginMaxFails {
		code, _ := postJSON(t, h.handleLogin, "/api/login", `{"password":"wrong"}`, "192.168.20.11:1", attacker)
		if code != http.StatusOK {
			t.Fatalf("failed attempt %d answered %d, want 200 with ok=false", i, code)
		}
	}
	code, _ := postJSON(t, h.handleLogin, "/api/login", `{"password":"wrong"}`, "192.168.20.11:1", attacker)
	if code != http.StatusTooManyRequests {
		t.Fatalf("the attacker must be throttled after %d failures, got %d", loginMaxFails, code)
	}

	code, body := postJSON(t, h.handleLogin, "/api/login", `{"password":"hunter2"}`, "192.168.20.11:1", operator)
	if code != http.StatusOK || body["ok"] != true {
		t.Fatalf("the operator must still get in: %d %v", code, body)
	}
}

// And the regression it replaces: with no TRUSTED_PROXY set, they DO share a
// bucket. Asserted so nobody reads the test above as a promise the default
// deployment makes.
func TestWithoutTrustedProxyTheBucketIsStillShared(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	enableAuth(t, h, repo)
	attacker := http.Header{"X-Forwarded-For": []string{"203.0.113.7"}}
	operator := http.Header{"X-Forwarded-For": []string{"203.0.113.8"}}
	for range loginMaxFails {
		postJSON(t, h.handleLogin, "/api/login", `{"password":"wrong"}`, "192.168.20.11:1", attacker)
	}
	code, _ := postJSON(t, h.handleLogin, "/api/login", `{"password":"hunter2"}`, "192.168.20.11:1", operator)
	if code != http.StatusTooManyRequests {
		t.Fatalf("without TRUSTED_PROXY the operator shares the bucket; got %d", code)
	}
}

// ---------------------------------------------------------------------------
// The password minimum
// ---------------------------------------------------------------------------

func TestSetPasswordRefusesAShortOne(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	code, body := postJSON(t, h.handleSetPassword, "/api/auth/password", `{"password":"1234"}`, "10.0.0.1:1", nil)
	if code != http.StatusOK || body["ok"] != false {
		t.Fatalf("a four-character password must be refused: %d %v", code, body)
	}
	s, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.AuthPasswordHash != "" {
		t.Fatal("a refused password must not be stored")
	}
}

func TestSetPasswordAcceptsTheMinimum(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	pw := strings.Repeat("x", secret.MinPasswordLen)
	code, body := postJSON(t, h.handleSetPassword, "/api/auth/password", `{"password":"`+pw+`"}`, "10.0.0.1:1", nil)
	if code != http.StatusOK || body["ok"] != true {
		t.Fatalf("a password at the minimum must be accepted: %d %v", code, body)
	}
	s, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(s.AuthPasswordHash, "argon2id$") {
		t.Fatalf("a new password must be stored with Argon2id, got %q", s.AuthPasswordHash)
	}
}

// Counted in runes. A passphrase in a script that spends three bytes per
// character is not twice as long for it.
func TestPasswordLengthIsCountedInCharactersNotBytes(t *testing.T) {
	h, _, _ := newAuthGateHandler(t)
	// Eleven characters, well over twelve bytes.
	short := strings.Repeat("ä", secret.MinPasswordLen-1)
	_, body := postJSON(t, h.handleSetPassword, "/api/auth/password", `{"password":"`+short+`"}`, "10.0.0.1:1", nil)
	if body["ok"] != false {
		t.Fatalf("%d multi-byte characters must still be too short: %v", secret.MinPasswordLen-1, body)
	}
}

// An existing short password must keep working. Locking somebody out of their
// own backups because they upgraded is a worse outcome than the weak password.
func TestAnExistingShortPasswordStillSignsIn(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	s, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	hash, err := secret.HashPassword(h.cfg.AppKey, "1234")
	if err != nil {
		t.Fatal(err)
	}
	s.AuthPasswordHash = hash
	if err := repo.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	_, body := postJSON(t, h.handleLogin, "/api/login", `{"password":"1234"}`, "10.0.0.1:1", nil)
	if body["ok"] != true {
		t.Fatalf("an existing short password must still work: %v", body)
	}
}

// Switching the login off must take the second factor with it, or re-enabling
// the password later would demand a code from an app that is long gone.
func TestClearingThePasswordAlsoClearsTheSecondFactor(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	enableAuth(t, h, repo)
	storeTOTP(t, h, repo, "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", []string{"deadbeef"})

	_, body := postJSON(t, h.handleSetPassword, "/api/auth/password", `{"password":""}`, "10.0.0.1:1", nil)
	if body["ok"] != true {
		t.Fatalf("clearing the password must succeed: %v", body)
	}
	s, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.TOTPEnabled || s.TOTPSecret != "" || s.TOTPRecovery != "" {
		t.Fatalf("the second factor must be cleared with the password: %+v", s.TOTPEnabled)
	}
}

// ---------------------------------------------------------------------------
// Upgrading the stored hash
// ---------------------------------------------------------------------------

// The migration that matters: a database written before v8.6.0 holds a bare
// HMAC, the owner signs in as always, and the hash is Argon2id afterwards.
func TestLegacyHashIsUpgradedOnASuccessfulLogin(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	s, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	// The pre-v8.6.0 format, written the way the old code wrote it.
	s.AuthPasswordHash = legacyHashForTest(h.cfg.AppKey, "correct horse battery")
	if err := repo.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	code, body := postJSON(t, h.handleLogin, "/api/login", `{"password":"correct horse battery"}`, "10.0.0.1:1", nil)
	if code != http.StatusOK || body["ok"] != true {
		t.Fatalf("a legacy password must still sign in: %d %v", code, body)
	}
	after, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(after.AuthPasswordHash, "argon2id$") {
		t.Fatalf("the hash must be upgraded in place, got %q", after.AuthPasswordHash)
	}
	if !secret.VerifyPassword(h.cfg.AppKey, "correct horse battery", after.AuthPasswordHash) {
		t.Fatal("the upgraded hash must verify the same password")
	}
}

// The subtle one. The session token signs the stored hash, so if the cookie were
// minted before the rehash it would be invalid the instant the rehash landed and
// the operator would bounce straight back to the login screen.
func TestTheCookieFromAnUpgradingLoginIsStillValidAfterwards(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	s, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.AuthPasswordHash = legacyHashForTest(h.cfg.AppKey, "correct horse battery")
	if err := repo.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"password":"correct horse battery"}`))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "10.0.0.1:1"
	w := httptest.NewRecorder()
	h.handleLogin(w, r)

	var cookie string
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookieName {
			cookie = c.Value
		}
	}
	if cookie == "" {
		t.Fatal("the login must set a session cookie")
	}
	after, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !secret.ValidSessionToken(h.cfg.AppKey, after.AuthPasswordHash, after.SessionEpoch, cookie) {
		t.Fatal("the cookie must still be valid against the upgraded hash")
	}
}

// ---------------------------------------------------------------------------
// The second factor
// ---------------------------------------------------------------------------

const testTOTPSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ" //nolint:gosec // G101: the RFC 6238 test vector, not a credential

func currentCode(t *testing.T) string {
	t.Helper()
	c, err := secret.TOTPCode(testTOTPSecret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// The right password alone must not be a session once a second factor is armed.
func TestPasswordAloneDoesNotSignInWithTOTPOn(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	enableAuth(t, h, repo)
	storeTOTP(t, h, repo, testTOTPSecret, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"password":"hunter2"}`))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "10.0.0.1:1"
	h.handleLogin(w, r)

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["ok"] != false || body["needCode"] != true {
		t.Fatalf("the password alone must ask for the code: %v", body)
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookieName {
			t.Fatal("no session cookie may be issued before the second factor")
		}
	}
}

func TestPasswordPlusCodeSignsIn(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	enableAuth(t, h, repo)
	storeTOTP(t, h, repo, testTOTPSecret, nil)

	_, body := postJSON(t, h.handleLogin, "/api/login",
		`{"password":"hunter2","code":"`+currentCode(t)+`"}`, "10.0.0.1:1", nil)
	if body["ok"] != true {
		t.Fatalf("password plus a valid code must sign in: %v", body)
	}
}

func TestAWrongCodeDoesNotSignIn(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	enableAuth(t, h, repo)
	storeTOTP(t, h, repo, testTOTPSecret, nil)

	_, body := postJSON(t, h.handleLogin, "/api/login",
		`{"password":"hunter2","code":"000000"}`, "10.0.0.1:1", nil)
	if body["ok"] != false {
		t.Fatalf("a wrong code must not sign in: %v", body)
	}
}

// Asking without a code is the client discovering it needs one. Counting that
// as a failed attempt would throttle people for doing the right thing.
func TestAskingWithoutACodeIsNotAFailedAttempt(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	enableAuth(t, h, repo)
	storeTOTP(t, h, repo, testTOTPSecret, nil)

	for range loginMaxFails + 3 {
		postJSON(t, h.handleLogin, "/api/login", `{"password":"hunter2"}`, "10.0.0.1:1", nil)
	}
	code, body := postJSON(t, h.handleLogin, "/api/login",
		`{"password":"hunter2","code":"`+currentCode(t)+`"}`, "10.0.0.1:1", nil)
	if code != http.StatusOK || body["ok"] != true {
		t.Fatalf("the real code must still work: %d %v", code, body)
	}
}

// But a wrong code IS a failed attempt, or the code becomes guessable at speed:
// six digits is a million, which falls in under a day at full rate.
func TestAWrongCodeCountsTowardsTheThrottle(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	enableAuth(t, h, repo)
	storeTOTP(t, h, repo, testTOTPSecret, nil)

	for range loginMaxFails {
		postJSON(t, h.handleLogin, "/api/login", `{"password":"hunter2","code":"000000"}`, "10.0.0.1:1", nil)
	}
	code, _ := postJSON(t, h.handleLogin, "/api/login",
		`{"password":"hunter2","code":"`+currentCode(t)+`"}`, "10.0.0.1:1", nil)
	if code != http.StatusTooManyRequests {
		t.Fatalf("wrong codes must throttle, got %d", code)
	}
}

// A recovery code works once and then never again. A code that survives its own
// use is a permanent second password.
func TestARecoveryCodeIsSpentWhenUsed(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	enableAuth(t, h, repo)
	plain, hashed, err := secret.NewRecoveryCodes(h.cfg.AppKey)
	if err != nil {
		t.Fatal(err)
	}
	storeTOTP(t, h, repo, testTOTPSecret, hashed)

	_, body := postJSON(t, h.handleLogin, "/api/login",
		`{"password":"hunter2","code":"`+plain[0]+`"}`, "10.0.0.1:1", nil)
	if body["ok"] != true {
		t.Fatalf("a recovery code must sign in: %v", body)
	}
	s, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if left := len(decodeRecoveryCodes(s.TOTPRecovery)); left != len(plain)-1 {
		t.Fatalf("%d codes left, want %d", left, len(plain)-1)
	}
	_, body = postJSON(t, h.handleLogin, "/api/login",
		`{"password":"hunter2","code":"`+plain[0]+`"}`, "10.0.0.2:1", nil)
	if body["ok"] != false {
		t.Fatalf("a spent recovery code must not work twice: %v", body)
	}
}

// An enrolment that was started and never confirmed must leave the login alone.
func TestAnUnconfirmedEnrolmentDoesNotDemandACode(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	enableAuth(t, h, repo)
	sealed, err := h.encryptTOTPSecret(testTOTPSecret)
	if err != nil {
		t.Fatal(err)
	}
	s, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.TOTPSecret = sealed
	s.TOTPEnabled = false // setup ran, confirm never did
	if err := repo.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	_, body := postJSON(t, h.handleLogin, "/api/login", `{"password":"hunter2"}`, "10.0.0.1:1", nil)
	if body["ok"] != true {
		t.Fatalf("an abandoned enrolment must not lock anybody out: %v", body)
	}
}

// The full enrolment, through the handlers, the way the settings page does it.
func TestEnrolmentRoundTrip(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	enableAuth(t, h, repo)

	_, body := postJSON(t, h.handleTOTPSetup, "/api/auth/totp/setup", `{}`, "10.0.0.1:1", nil)
	if body["ok"] != true {
		t.Fatalf("setup must succeed: %v", body)
	}
	sec, _ := body["secret"].(string)
	if sec == "" {
		t.Fatalf("setup must return a secret: %v", body)
	}
	// Not armed yet.
	s, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.TOTPEnabled {
		t.Fatal("setup alone must not arm the second factor")
	}

	code, err := secret.TOTPCode(sec, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	_, body = postJSON(t, h.handleTOTPConfirm, "/api/auth/totp/confirm", `{"code":"`+code+`"}`, "10.0.0.1:1", nil)
	if body["ok"] != true {
		t.Fatalf("confirm must succeed with a valid code: %v", body)
	}
	codes, _ := body["recoveryCodes"].([]any)
	if len(codes) == 0 {
		t.Fatalf("confirm must hand back recovery codes: %v", body)
	}
	s, err = repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !s.TOTPEnabled {
		t.Fatal("confirm must arm the second factor")
	}

	// Turning it off needs a live code, so an unattended session cannot quietly
	// remove the factor.
	_, body = postJSON(t, h.handleTOTPDisable, "/api/auth/totp/disable", `{"code":"000000"}`, "10.0.0.1:1", nil)
	if body["ok"] != false {
		t.Fatalf("disable must refuse a wrong code: %v", body)
	}
	live, err := secret.TOTPCode(sec, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	_, body = postJSON(t, h.handleTOTPDisable, "/api/auth/totp/disable", `{"code":"`+live+`"}`, "10.0.0.1:1", nil)
	if body["ok"] != true {
		t.Fatalf("disable must accept a live code: %v", body)
	}
}

// A second factor on an instance with no password would protect nothing and
// would be the only thing standing between the LAN and the API.
func TestSetupRefusesWithoutAPassword(t *testing.T) {
	h, _, _ := newAuthGateHandler(t)
	code, body := postJSON(t, h.handleTOTPSetup, "/api/auth/totp/setup", `{}`, "10.0.0.1:1", nil)
	if code != http.StatusForbidden && body["ok"] != false {
		t.Fatalf("setup must refuse while no password is set: %d %v", code, body)
	}
}

// The public status endpoint may say a code is needed (the login screen has to
// know), but must not count somebody's remaining recovery codes for them.
func TestAuthStatusKeepsRecoveryCountForSignedInCallers(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	enableAuth(t, h, repo)
	storeTOTP(t, h, repo, testTOTPSecret, []string{"a", "b"})

	r := httptest.NewRequest(http.MethodGet, "/api/auth", nil)
	w := httptest.NewRecorder()
	h.handleAuthStatus(w, r)
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["totp"] != true {
		t.Fatalf("the login screen must be told a code is needed: %v", body)
	}
	if _, present := body["recoveryCodesLeft"]; present {
		t.Fatalf("a stranger must not be told how many codes are left: %v", body)
	}
}

// legacyHashForTest reproduces the pre-v8.6.0 stored format from outside the
// secret package, written out here on purpose: the upgrade is proved against
// the real old value, not against whatever the current code happens to produce.
func legacyHashForTest(appKey, password string) string {
	keyBytes, err := hex.DecodeString(appKey)
	if err != nil {
		panic(err)
	}
	mac := hmac.New(sha256.New, keyBytes)
	mac.Write([]byte("bombvault:auth:" + password))
	return hex.EncodeToString(mac.Sum(nil))
}
