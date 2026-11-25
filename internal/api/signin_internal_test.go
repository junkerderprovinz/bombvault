package api

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// oldInstall is a database the version before the kit left: a password, a
// second factor with two recovery codes, a passkey and the session epoch.
type oldInstall struct {
	st       *store.Repo
	appKey   string
	hash     string
	epoch    string
	totp     string
	recovery []string
}

func newOldInstall(t *testing.T, appKey string) *oldInstall {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	o := &oldInstall{st: store.New(db), appKey: appKey, epoch: "0123456789abcdef0123456789abcdef", recovery: []string{"abcde-fghjk", "mnpqr-stuvw"}}
	if o.hash, err = secret.HashPassword(appKey, testPassword); err != nil {
		t.Fatal(err)
	}
	if o.totp, err = secret.NewTOTPSecret(); err != nil {
		t.Fatal(err)
	}
	sealed, err := secret.Encrypt(appKey, []byte(o.totp))
	if err != nil {
		t.Fatal(err)
	}
	codes := `["` + secret.HashRecoveryCode(appKey, o.recovery[0]) + `","` + secret.HashRecoveryCode(appKey, o.recovery[1]) + `"]`
	if _, err := o.st.MutateSettings(func(s *store.Settings) error {
		s.AuthPasswordHash, s.SessionEpoch = o.hash, o.epoch
		s.TOTPEnabled, s.TOTPSecret, s.TOTPRecovery = true, hex.EncodeToString(sealed), codes
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := o.st.AddPasskey(store.Passkey{
		Name: "laptop", CredentialID: []byte("credential-1"), PublicKey: []byte{1, 2, 3}, AAGUID: make([]byte, 16),
		RPID: "backup.example", BackedUp: true, CreatedAt: 1_700_000_000, LastUsedAt: 1_700_000_100,
	}); err != nil {
		t.Fatal(err)
	}
	return o
}

// open starts the version with the kit on the old database.
func (o *oldInstall) open(t *testing.T) (*Handler, http.Handler) {
	t.Helper()
	cfg := config.Config{AppKey: o.appKey, DataDir: t.TempDir(), HTTPOnly: true}
	svc := NewService(cfg, o.st, nil, nil, nil)
	h := NewHandler(cfg, o.st, nil, svc, nil, nil)
	h.resetOut = &strings.Builder{}
	router := h.Router()
	t.Cleanup(func() { _ = h.auth.Close() })
	return h, router
}

// oldCookie is a session cookie the version before gave a browser.
func (o *oldInstall) oldCookie() *http.Cookie {
	return &http.Cookie{Name: sessionCookieNameHTTP, Value: secret.NewSessionToken(o.appKey, o.hash, o.epoch, 6*24*time.Hour)} //nolint:gosec // G124: a cookie on a test request
}

func get(router http.Handler, path string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.Host = "localhost"
	for _, c := range cookies {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w
}

func TestAnUpgradeKeepsTheBrowserSignedIn(t *testing.T) {
	o := newOldInstall(t, strings.Repeat("c3", 32))
	_, router := o.open(t)

	if w := get(router, "/api/status"); w.Code != http.StatusUnauthorized {
		t.Fatalf("without a cookie after the upgrade: %d, want the instance closed", w.Code)
	}
	w := get(router, "/api/status", o.oldCookie())
	if w.Code != http.StatusOK {
		t.Fatalf("the old cookie after the upgrade: %d %s", w.Code, w.Body.String())
	}
	if session := sessionFrom(t, w); get(router, "/api/status", sessionCookieFor(session)).Code != http.StatusOK {
		t.Fatal("the session the old cookie was swapped for does not sign in")
	}
}

func TestAnUpgradeKeepsThePasswordAndTheSecondFactor(t *testing.T) {
	o := newOldInstall(t, strings.Repeat("c3", 32))
	_, router := o.open(t)

	w := kitRequest(t, router, http.MethodPost, "login", "", map[string]any{"password": testPassword})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"code"`) {
		t.Fatalf("the old password: %d %s, want the step that asks for the code", w.Code, w.Body.String())
	}
	step := w.Result().Cookies()
	code, err := secret.TOTPCode(o.totp, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/api/signin/v1/login/totp", strings.NewReader(`{"code":"`+code+`"}`))
	r.Host = "localhost"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	for _, c := range step {
		r.AddCookie(c)
	}
	done := httptest.NewRecorder()
	router.ServeHTTP(done, r)
	if session := sessionFrom(t, done); get(router, "/api/status", sessionCookieFor(session)).Code != http.StatusOK {
		t.Fatal("the old second factor did not sign in")
	}
}

func TestAnUpgradeKeepsTheOldRecoveryCodes(t *testing.T) {
	o := newOldInstall(t, strings.Repeat("c3", 32))
	_, router := o.open(t)

	step := kitRequest(t, router, http.MethodPost, "login", "", map[string]any{"password": testPassword}).Result().Cookies()
	r := httptest.NewRequest(http.MethodPost, "/api/signin/v1/login/totp", strings.NewReader(`{"code":"`+o.recovery[1]+`"}`))
	r.Host = "localhost"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	for _, c := range step {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	sessionFrom(t, w)
}

func TestAnUpgradeTakesThePasskeysWithTheirAddress(t *testing.T) {
	o := newOldInstall(t, strings.Repeat("c3", 32))
	h, router := o.open(t)
	if got := h.publicURL(); got == nil || got.String() != "https://backup.example" {
		t.Fatalf("the address = %v, want where the passkey was used last", got)
	}
	session := signInWithCode(t, router, o)
	w := kitRequest(t, router, http.MethodGet, "me/passkeys", session, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"laptop"`) {
		t.Fatalf("the passkeys after the upgrade: %d %s", w.Code, w.Body.String())
	}
}

// signInWithCode signs in to an upgraded install with the password and the
// first recovery code.
func signInWithCode(t *testing.T, router http.Handler, o *oldInstall) string {
	t.Helper()
	step := kitRequest(t, router, http.MethodPost, "login", "", map[string]any{"password": testPassword}).Result().Cookies()
	r := httptest.NewRequest(http.MethodPost, "/api/signin/v1/login/totp", strings.NewReader(`{"code":"`+o.recovery[0]+`"}`))
	r.Host = "localhost"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	for _, c := range step {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return sessionFrom(t, w)
}

func TestAPhoneSessionFromBeforeTheUpgradeStillOpensThePages(t *testing.T) {
	o := newOldInstall(t, strings.Repeat("c3", 32))
	_, router := o.open(t)
	phone := listedPhoneSession(t, o.st, o.appKey, o.hash, o.epoch)
	if w := get(router, "/api/status", sessionCookieFor(phone)); w.Code != http.StatusOK {
		t.Fatalf("the phone's old session: %d %s", w.Code, w.Body.String())
	}
	if w := get(router, "/api/settings/export", sessionCookieFor(phone)); w.Code != http.StatusForbidden {
		t.Fatalf("the phone's old session on a route phones never reach: %d", w.Code)
	}
}

func TestSigningOutEverywhereEndsThePhonesSessions(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	phone := pairedCookie(t, in)
	if w := get(in.router, "/api/status", phone); w.Code != http.StatusOK {
		t.Fatalf("the phone before: %d", w.Code)
	}
	if w := kitRequest(t, in.router, http.MethodDelete, "me/sessions", in.session, nil); w.Code != http.StatusOK {
		t.Fatalf("sign out everywhere: %d %s", w.Code, w.Body.String())
	}
	if w := get(in.router, "/api/status", phone); w.Code != http.StatusUnauthorized {
		t.Fatalf("the phone after signing out everywhere: %d, want its session ended", w.Code)
	}
}

func TestTheResetEndsEveryWayInBombVaultKeepsItself(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	if _, err := in.st.CreateMCPKey(newMCPKeyID(), "", "laptop", "", "digest", "hint", "check", true, time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err := in.st.CreateAPIToken(newMCPKeyID(), "", "script", "digest2", "hint", "check", false, time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	phone := pairedCookie(t, in)

	resetOn(t, in, "a new password")

	keys, err := in.st.ListMCPKeys()
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range keys {
		if k.RevokedAt == 0 {
			t.Errorf("%s %q outlived the reset", k.Kind, k.Label)
		}
	}
	if w := get(in.router, "/api/status", phone); w.Code == http.StatusOK {
		t.Error("a phone's session outlived the reset")
	}
}

func TestANewKeyWantsAFreshSignIn(t *testing.T) {
	o := newOldInstall(t, strings.Repeat("c3", 32))
	_, router := o.open(t)
	// The session swapped for the old cookie began when that cookie was
	// issued, days ago.
	stale := sessionFrom(t, get(router, "/api/status", o.oldCookie()))
	w := kitRequest(t, router, http.MethodGet, "me", stale, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("me: %d", w.Code)
	}
	r := httptest.NewRequest(http.MethodPost, "/api/mcp/keys", strings.NewReader(`{"label":"laptop"}`))
	r.Host = "localhost"
	r.Header.Set("Content-Type", "application/json")
	r.AddCookie(sessionCookieFor(stale))
	got := httptest.NewRecorder()
	router.ServeHTTP(got, r)
	if !mcpShipped {
		return
	}
	if got.Code != http.StatusForbidden || !strings.Contains(got.Body.String(), `"signInAgain"`) {
		t.Fatalf("a new MCP key on a sign-in from days ago: %d %s", got.Code, got.Body.String())
	}
	keys, err := o.st.ListMCPKeys()
	if err != nil || len(keys) != 0 {
		t.Fatalf("keys = %v, %v, want none", keys, err)
	}
}

func TestAChangedAppKeyKeepsTheInstanceClosed(t *testing.T) {
	o := newOldInstall(t, strings.Repeat("c3", 32))
	h, router := o.open(t)
	session := signInWithCode(t, router, o)
	if w := kitRequest(t, router, http.MethodPut, "me/password", session, map[string]any{"current": testPassword, "next": "a new password"}); w.Code != http.StatusOK {
		t.Fatalf("change the password: %d %s", w.Code, w.Body.String())
	}
	_ = h.auth.Close()

	o.appKey = strings.Repeat("d4", 32)
	_, after := o.open(t)
	if w := get(after, "/api/status"); w.Code != http.StatusUnauthorized {
		t.Fatalf("after APP_KEY changed: %d, want the instance closed", w.Code)
	}
	if w := kitRequest(t, after, http.MethodPost, "login", "", map[string]any{"password": "a new password"}); w.Code == http.StatusOK {
		t.Fatal("a password hashed under the old APP_KEY still signs in")
	}
}

func TestAChangedPasswordDropsTheOldLogin(t *testing.T) {
	o := newOldInstall(t, strings.Repeat("c3", 32))
	_, router := o.open(t)
	session := signInWithCode(t, router, o)
	if w := kitRequest(t, router, http.MethodPut, "me/password", session, map[string]any{"current": testPassword, "next": "a new password"}); w.Code != http.StatusOK {
		t.Fatalf("change the password: %d %s", w.Code, w.Body.String())
	}
	s, err := o.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.AuthPasswordHash != store.OldLoginMarker || s.TOTPEnabled || s.TOTPSecret != "" || s.TOTPRecovery != "" {
		t.Fatalf("the old login after a new password: hash %q, totp %v %q %q", s.AuthPasswordHash, s.TOTPEnabled, s.TOTPSecret, s.TOTPRecovery)
	}
	if keys, err := o.st.ListPasskeys(); err != nil || len(keys) != 0 {
		t.Fatalf("old passkeys = %v, %v, want none", keys, err)
	}
	if w := get(router, "/api/status", o.oldCookie()); w.Code != http.StatusUnauthorized {
		t.Fatalf("an old cookie after the new password: %d", w.Code)
	}
}

func TestTheResetCodeStaysOutOfTheLog(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	out := &strings.Builder{}
	in.h.resetOut = out
	if w := kitRequest(t, in.router, http.MethodPost, "reset-code", "", nil); w.Code != http.StatusOK {
		t.Fatalf("ask for a code: %d", w.Code)
	}
	if !strings.Contains(out.String(), "PASSWORD RESET CODE: ") {
		t.Fatalf("stdout = %q, want the code", out.String())
	}
}

func TestThePublicAddressIsSpelledAsBrowsersSendIt(t *testing.T) {
	for in, want := range map[string]string{
		"https://Backup.Example:443/": "https://backup.example",
		"http://10.0.0.5:3000":        "http://10.0.0.5:3000",
		" https://backup.example ":    "https://backup.example",
		"https://[::1]:8443":          "https://[::1]:8443",
	} {
		if got, ok := normalizePublicAddress(in); !ok || got != want {
			t.Errorf("normalizePublicAddress(%q) = %q, %v, want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"backup.example", "ftp://backup.example", "https://backup.example/sub", "https://backup.example?x=1", "https://user@backup.example"} {
		if got, ok := normalizePublicAddress(in); ok {
			t.Errorf("normalizePublicAddress(%q) = %q, want refused", in, got)
		}
	}
}
