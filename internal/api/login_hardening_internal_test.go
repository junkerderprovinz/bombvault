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

// Believing a forwarded-for header by default would let any caller pick their
// own throttle bucket.
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

// Somebody reaching the port directly must not inherit the proxy's trust by
// sending the same header.
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

// An attacker controls the left end of the chain, so reading it from the left
// would hand them a fresh bucket per request.
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
		// One bad entry refuses the whole header, so an attacker cannot steer
		// which entry is picked by inserting one.
		{"1.2.3.4, nonsense, 10.0.0.5", ""},
	}
	for _, c := range cases {
		if got := forwardedClient(c.header, proxies); got != c.want {
			t.Fatalf("forwardedClient(%q) = %q, want %q", c.header, got, c.want)
		}
	}
}

// Two clients behind one proxy must not share a bucket, or an attacker's
// failures lock the operator out of their own instance.
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

// Without TRUSTED_PROXY, clients behind one proxy share a bucket; the test
// above is not a promise the default deployment makes.
func TestWithoutTrustedProxyClientsShareABucket(t *testing.T) {
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

// A passphrase in a script that spends several bytes per character is not
// longer for it.
func TestPasswordLengthIsCountedInCharactersNotBytes(t *testing.T) {
	h, _, _ := newAuthGateHandler(t)
	short := strings.Repeat("ä", secret.MinPasswordLen-1)
	_, body := postJSON(t, h.handleSetPassword, "/api/auth/password", `{"password":"`+short+`"}`, "10.0.0.1:1", nil)
	if body["ok"] != false {
		t.Fatalf("%d multi-byte characters must still be too short: %v", secret.MinPasswordLen-1, body)
	}
}

// Locking somebody out of their own backups because they upgraded is worse
// than the weak password.
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

// A second factor left behind would demand a code from a long-gone app once a
// password is set again.
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

// An old database holds a bare HMAC; after the owner signs in, it holds
// Argon2id.
func TestLegacyHashIsUpgradedOnASuccessfulLogin(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	s, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
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

// The session token signs the stored hash. A cookie minted before the rehash
// would be invalid right after it, sending the operator back to the login
// screen.
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

const testTOTPSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ" //nolint:gosec // G101: the RFC 6238 test vector, not a credential

func currentCode(t *testing.T) string {
	t.Helper()
	c, err := secret.TOTPCode(testTOTPSecret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

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

// Asking without a code is how the client learns it needs one, so it must not
// count as a failed attempt.
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

// A million six-digit codes fall in under a day at full rate unless wrong
// codes are throttled.
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

// A recovery code that survives its own use is a permanent second password.
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
	s.TOTPEnabled = false
	if err := repo.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	_, body := postJSON(t, h.handleLogin, "/api/login", `{"password":"hunter2"}`, "10.0.0.1:1", nil)
	if body["ok"] != true {
		t.Fatalf("an abandoned enrolment must not lock anybody out: %v", body)
	}
}

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

// A second factor without a password protects nothing.
func TestSetupRefusesWithoutAPassword(t *testing.T) {
	h, _, _ := newAuthGateHandler(t)
	code, body := postJSON(t, h.handleTOTPSetup, "/api/auth/totp/setup", `{}`, "10.0.0.1:1", nil)
	if code != http.StatusForbidden && body["ok"] != false {
		t.Fatalf("setup must refuse while no password is set: %d %v", code, body)
	}
}

// The public status endpoint tells the login screen a code is needed, but not
// how many recovery codes are left.
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

// legacyHashForTest writes the bare HMAC format by hand, so the upgrade is
// tested against a real stored value rather than whatever the secret package
// happens to produce.
func legacyHashForTest(appKey, password string) string {
	keyBytes, err := hex.DecodeString(appKey)
	if err != nil {
		panic(err)
	}
	mac := hmac.New(sha256.New, keyBytes)
	mac.Write([]byte("bombvault:auth:" + password))
	return hex.EncodeToString(mac.Sum(nil))
}

// The route is reachable without a session only while the login is off. Once
// it stores a hash, authGate wants a cookie, and without one every following
// request answers 401. Issuing it grants nothing: whoever reaches the route
// unauthenticated already had the whole API.
func TestSetPasswordIssuesASession(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	pw := strings.Repeat("x", secret.MinPasswordLen)

	r := httptest.NewRequest(http.MethodPost, "/api/auth/password", strings.NewReader(`{"password":"`+pw+`"}`))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "10.0.0.1:1"
	w := httptest.NewRecorder()
	h.handleSetPassword(w, r)

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v (%s)", err, w.Body.String())
	}
	if body["ok"] != true || body["authed"] != true {
		t.Fatalf("setting a password must report the caller signed in, got %v", body)
	}

	var tok string
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookieName {
			tok = c.Value
		}
	}
	if tok == "" {
		t.Fatal("no session cookie was issued.\n" +
			"The request that switches the login ON is the last one this browser may make without\n" +
			"one, so every control answers 401 until the page is reloaded and the password typed\n" +
			"again - the second factor above all, which is exactly what somebody sets up next.")
	}

	// A token minted before the write would be signed against the old, empty
	// hash.
	s, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !secret.ValidSessionToken(h.cfg.AppKey, s.AuthPasswordHash, s.SessionEpoch, tok) {
		t.Error("the issued session does not validate against the password that was just stored")
	}
}

// A token signed against a hash that no longer exists means nothing; left in
// the browser it looks like a session and is not one.
func TestClearingThePasswordSendsTheSessionAway(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	enableAuth(t, h, repo)

	r := httptest.NewRequest(http.MethodPost, "/api/auth/password", strings.NewReader(`{"password":""}`))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "10.0.0.1:1"
	w := httptest.NewRecorder()
	h.handleSetPassword(w, r)

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v (%s)", err, w.Body.String())
	}
	if body["enabled"] != false || body["authed"] != false {
		t.Fatalf("clearing the password must report the login off and nobody signed in, got %v", body)
	}
	found := false
	for _, c := range w.Result().Cookies() {
		if c.Name != sessionCookieName {
			continue
		}
		found = true
		if c.MaxAge >= 0 || c.Value != "" {
			t.Errorf("the session cookie must be expired, got value %q maxAge %d", c.Value, c.MaxAge)
		}
	}
	if !found {
		t.Error("clearing the password left the browser's session cookie in place")
	}
}

// Session tokens are stateless and signed against the stored epoch, so
// rotating it is the one way to revoke a cookie held by another browser, which
// is what somebody changing a password out of suspicion expects. The caller's
// own new cookie has to survive the rotation.
func TestSetPasswordEndsEveryOtherSession(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	pw := strings.Repeat("x", secret.MinPasswordLen)

	first := postPassword(t, h, pw)
	before, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !secret.ValidSessionToken(h.cfg.AppKey, before.AuthPasswordHash, before.SessionEpoch, first) {
		t.Fatal("the first session is invalid before anything was changed - the test cannot prove what it is here for")
	}

	second := postPassword(t, h, pw+"2")
	after, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	if after.SessionEpoch == before.SessionEpoch {
		t.Fatal("the session epoch did not move.\n" +
			"Every outstanding seven-day token stays valid, and with the card's own button gone\n" +
			"there is no way left to revoke them at all.")
	}
	if secret.ValidSessionToken(h.cfg.AppKey, after.AuthPasswordHash, after.SessionEpoch, first) {
		t.Error("a session minted before the password change still validates")
	}
	if second == "" {
		t.Fatal("the password change issued no cookie of its own")
	}
	if !secret.ValidSessionToken(h.cfg.AppKey, after.AuthPasswordHash, after.SessionEpoch, second) {
		t.Error("the caller's own new session does not validate - the cookie was minted from the epoch that was just replaced")
	}
}

// postPassword sets a password and returns the session cookie it issued.
func postPassword(t *testing.T, h *Handler, pw string) string {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/api/auth/password", strings.NewReader(`{"password":"`+pw+`"}`))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "10.0.0.1:1"
	w := httptest.NewRecorder()
	h.handleSetPassword(w, r)
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookieName {
			return c.Value
		}
	}
	return ""
}
