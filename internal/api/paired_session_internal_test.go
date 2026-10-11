package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/relay"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/seedphrase"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// wordsHolder is somebody who has the twelve words and nothing else: no
// BombVault, no APP_KEY and no password of any instance.
type wordsHolder struct {
	t      *testing.T
	id     string
	client *relay.Client
}

// holdWords connects to the group's relay with keys made from the words and
// announces itself under kind, the way a phone does.
func holdWords(t *testing.T, relayURL, phrase, id, kind string, seenBy ...*instance) *wordsHolder {
	t.Helper()
	sec, err := seedphrase.Decode(phrase)
	if err != nil {
		t.Fatal(err)
	}
	c, err := relay.NewClient(relay.ClientOptions{
		URL: relayURL, Key: relay.DeriveKey(sec), FrameKey: relay.DeriveFrameKey(sec), InstanceID: id,
		Identity: relay.Identity{Name: "tower", Version: Version, Kind: kind},
		Serve:    func(context.Context, relay.ProxyCall) (int, []byte) { return http.StatusNotFound, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	c.Start()
	t.Cleanup(func() { _ = c.Close() })
	deadline := time.Now().Add(10 * time.Second)
	for _, in := range seenBy {
		for !isMember(in, id) {
			if time.Now().After(deadline) {
				t.Fatal("an instance never saw the holder of the words")
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	return &wordsHolder{t: t, id: id, client: c}
}

func isMember(in *instance, id string) bool {
	for _, m := range in.svc.pairing().Members() {
		if m.ID == id {
			return true
		}
	}
	return false
}

// call sends target a peer call that names sender as where it came from,
// which the holder of the words is free to choose.
func (w *wordsHolder) call(target, sender, method, path string, body any) map[string]any {
	w.t.Helper()
	var raw []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			w.t.Fatal(err)
		}
		raw = b
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		res, err := w.client.Call(context.Background(), target, relay.ProxyCall{Method: method, Path: path, Body: raw, Sender: sender})
		if err == nil {
			out := map[string]any{}
			_ = json.Unmarshal(res.Body, &out)
			out["_status"] = float64(res.Status)
			return out
		}
		if time.Now().After(deadline) {
			w.t.Fatalf("%s %s: %v", method, path, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// pairedSession is the session an instance gives the holder of the words for
// calling itself a phone.
func (w *wordsHolder) pairedSession(in *instance) string {
	w.t.Helper()
	out := w.call(in.id(w.t), w.id, http.MethodPost, "/api/group/peer/session", map[string]any{"instanceId": w.id})
	value, _ := out["value"].(string)
	hash, epoch, _ := in.h.authEnabled()
	if !secret.ValidSessionToken(in.appKey, hash, epoch, value) {
		w.t.Fatalf("no session that signs in: %v", out)
	}
	return value
}

// as runs fn with the instance's session replaced by another one.
func (in *instance) as(session string, fn func()) {
	own := in.session
	in.session = session
	defer func() { in.session = own }()
	fn()
}

const wordsHolderID = "0123456789abcdef0123456789abcdef"

func TestTheTwelveWordsAloneNeitherAllowAReleaseNorReachTheKit(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	phrase, relayURL := groupOf(t, tower, attic)
	if code, _ := deposit(t, tower, attic)["recoveryCode"].(string); code == "" {
		t.Fatal("no deposit")
	}
	towerID, atticID := tower.id(t), attic.id(t)
	mallory := holdWords(t, relayURL, phrase, wordsHolderID, "android", attic)
	paired := mallory.pairedSession(attic)

	inOwnersName := mallory.call(atticID, towerID, http.MethodPost, "/api/group/peer/deposit/release", peerReleaseAsk{OwnerID: towerID, Name: "tower", Renew: true})
	if inOwnersName["ok"] != false || len(releasesOn(t, attic)) != 0 {
		t.Fatalf("a request in the owner's own id was taken: %v", inOwnersName)
	}

	// The words make a member of whoever holds them, with a key of its own.
	seed, err := keydeposit.NewOwnerSeed()
	if err != nil {
		t.Fatal(err)
	}
	if err := attic.st.PinMemberKey(wordsHolderID, keydeposit.OwnerKey(seed)); err != nil {
		t.Fatal(err)
	}
	signed := signedAs(t, attic, seed, wordsHolderID, keydeposit.ProofRelease, time.Now().UnixMilli(), callPayload(towerID, "tower", "true"),
		map[string]any{"ownerId": towerID, "name": "tower", "renew": true})
	asked := mallory.call(atticID, wordsHolderID, http.MethodPost, "/api/group/peer/deposit/release", json.RawMessage(signed))
	if asked["state"] != store.ReleaseAsked {
		t.Fatalf("the request: %v", asked)
	}
	waiting := releasesOn(t, attic)
	if len(waiting) != 1 {
		t.Fatalf("requests on the keeper: %+v", waiting)
	}
	attic.as(paired, func() {
		if code, out := attic.do(t, http.MethodGet, "/api/group/deposits", nil); code != http.StatusForbidden || out["code"] != codeOwnLoginNeeded {
			t.Fatalf("the deposits with the paired session: %d %v", code, out)
		}
		code, out := attic.do(t, http.MethodPost, "/api/group/deposits/releases/"+waiting[0].ID, map[string]any{"decision": "allow"})
		if code != http.StatusForbidden || out["code"] != codeOwnLoginNeeded {
			t.Fatalf("allow with the paired session: %d %v", code, out)
		}
		if code, out := attic.do(t, http.MethodGet, "/api/recovery-kit", nil); code != http.StatusForbidden || out["code"] != codeOwnLoginNeeded {
			t.Fatalf("the kit with the paired session: %d %v", code, out)
		}
	})
	again := signedAs(t, attic, seed, wordsHolderID, keydeposit.ProofRelease, time.Now().UnixMilli()+1, callPayload(towerID, "tower", "false"),
		map[string]any{"ownerId": towerID, "name": "tower", "renew": false})
	if got := mallory.call(atticID, wordsHolderID, http.MethodPost, "/api/group/peer/deposit/release", json.RawMessage(again)); got["deposit"] != nil || got["state"] != store.ReleaseAsked {
		t.Fatalf("the keeper handed something out: %v", got)
	}

	if out := decideRelease(t, attic, waiting[0].ID, "allow"); out["state"] != store.ReleaseAllowed {
		t.Fatalf("a signed-in person allows: %v", out)
	}
}

func TestAPairedSessionOpensThePagesAndStopsAtKeysAndTheLogin(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	phrase, relayURL := groupOf(t, tower, attic)
	paired := holdWords(t, relayURL, phrase, wordsHolderID, "android", attic).pairedSession(attic)
	if !secret.PairedSessionToken(paired) {
		t.Fatal("the session of a phone is not marked as one")
	}

	attic.as(paired, func() {
		if code, out := attic.do(t, http.MethodGet, "/api/status", nil); code != http.StatusOK || out["ok"] != true {
			t.Fatalf("the paired session reads the status with %d %v", code, out)
		}
		for _, route := range []struct {
			method, path string
			body         any
		}{
			{http.MethodGet, "/api/recovery-kit", nil},
			{http.MethodGet, "/api/diagnostics", nil},
			{http.MethodGet, "/api/settings/export?includeCredentials=1", nil},
			{http.MethodPost, "/api/auth/password", map[string]any{"password": "a new password of mine"}},
			{http.MethodPost, "/api/auth/totp/setup", map[string]any{}},
			{http.MethodPost, "/api/auth/passkey/register/begin", map[string]any{}},
			{http.MethodPut, "/api/group/members/" + tower.id(t) + "/roles/keeper", map[string]any{}},
			{http.MethodDelete, "/api/group/members/" + tower.id(t) + "/roles/keeper", map[string]any{}},
			{http.MethodPost, "/api/group/deposits/releases/any", map[string]any{"decision": "refuse"}},
			{http.MethodPost, "/api/tokens", map[string]any{"label": "mine"}},
			{http.MethodPost, "/api/mcp/keys", map[string]any{"label": "mine"}},
			{http.MethodPost, "/api/tokens/0123456789abcdef0123456789abcdef/rotate", map[string]any{}},
			{http.MethodPost, "/api/mcp/keys/0123456789abcdef0123456789abcdef/rotate", map[string]any{}},
			{http.MethodPost, "/api/group/deposit/code", map[string]any{}},
			{http.MethodPost, "/api/emergency/adopt", map[string]any{}},
			{http.MethodPost, "/api/emergency/sign-ins", map[string]any{}},
			{http.MethodPost, "/api/emergency/undo", map[string]any{}},
		} {
			code, out := attic.do(t, route.method, route.path, route.body)
			if code != http.StatusForbidden || out["code"] != codeOwnLoginNeeded {
				t.Errorf("%s %s with a paired session: %d %v", route.method, route.path, code, out)
			}
		}
	})
	settings, err := attic.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !secret.VerifyPassword(attic.appKey, testPassword, settings.AuthPasswordHash) {
		t.Fatal("the password changed")
	}
	if code, out := attic.do(t, http.MethodPost, "/api/group/deposit/code", map[string]any{}); code != http.StatusOK || out["code"] == codeOwnLoginNeeded {
		t.Fatalf("a signed-in session is asked to sign in: %d %v", code, out)
	}
}

func TestKeyMaterialIsNotMadeOrHandedOutWithoutALoginPassword(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	groupOf(t, tower, attic)
	if _, err := tower.st.MutateSettings(func(s *store.Settings) error { s.AuthPasswordHash = ""; return nil }); err != nil {
		t.Fatal(err)
	}
	for _, route := range []struct{ method, path string }{
		{http.MethodPut, "/api/group/members/" + attic.id(t) + "/roles/keeper"},
		{http.MethodPost, "/api/group/deposit/code"},
		{http.MethodPost, "/api/group/deposits/releases/any"},
	} {
		code, out := tower.do(t, route.method, route.path, map[string]any{"decision": "allow"})
		if code != http.StatusForbidden || out["code"] != codeNoPassword {
			t.Errorf("%s %s without a password: %d %v", route.method, route.path, code, out)
		}
	}
	if _, found := kept(t, attic, tower); found {
		t.Fatal("a deposit was made")
	}
}

// heldBody tells entered when the request first reads it and hands out its
// content once release is closed. That holds a request after the gates have
// let it in and before it reads its body.
type heldBody struct {
	entered func()
	release <-chan struct{}
	body    *strings.Reader
}

func (b heldBody) Read(p []byte) (int, error) {
	b.entered()
	<-b.release
	return b.body.Read(p)
}

func TestASessionTheResetEndedCannotSwitchPairingPhonesOn(t *testing.T) {
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	entered, release := make(chan struct{}), make(chan struct{})
	body := heldBody{entered: sync.OnceFunc(func() { close(entered) }), release: release, body: strings.NewReader(`{"on":true}`)}
	req := httptest.NewRequest(http.MethodPut, "/api/auth/phone-pairing", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: sessionCookieNameHTTP, Value: attic.session}) //nolint:gosec // G124: a cookie on a test request, never set by a server
	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		attic.router.ServeHTTP(w, req)
		close(done)
	}()

	hash, err := secret.HashPassword(attic.appKey, "a new password")
	if err != nil {
		t.Fatal(err)
	}
	<-entered
	attic.h.mintMu.Lock()
	_, _, err = attic.h.resetLogin(hash)
	attic.h.mintMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	close(release)
	<-done
	if on, err := attic.st.PhonePairing(); err != nil || on {
		t.Fatalf("pairing phones after the reset = %v, %v; answer %d %s", on, err, w.Code, w.Body)
	}
}

func TestAResetKeepsPhonesOutUntilAPersonLetsThemPairAgain(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	phrase, relayURL := groupOf(t, tower, attic)
	phone := holdWords(t, relayURL, phrase, wordsHolderID, "android", attic)
	phone.pairedSession(attic)

	hash, err := secret.HashPassword(attic.appKey, "a new password")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := attic.h.resetLogin(hash); err != nil {
		t.Fatal(err)
	}
	if out := phone.call(attic.id(t), wordsHolderID, http.MethodPost, "/api/group/peer/session", map[string]any{"instanceId": wordsHolderID}); out["ok"] != false || out["code"] != codePhonePairingOff {
		t.Fatalf("a phone asking for a session after the reset: %v", out)
	}
	hash, epoch, _ := attic.h.authEnabled()
	attic.as(secret.NewPairedSessionToken(attic.appKey, hash, epoch, time.Hour), func() {
		if code, _ := attic.do(t, http.MethodGet, "/api/status", nil); code != http.StatusUnauthorized {
			t.Fatalf("a paired session while pairing phones is off: %d", code)
		}
	})

	attic.session = secret.NewSessionToken(attic.appKey, hash, epoch, time.Hour)
	if _, out := attic.do(t, http.MethodGet, "/api/auth/phone-pairing", nil); out["on"] != false {
		t.Fatalf("pairing phones after the reset: %v", out)
	}
	if _, out := attic.do(t, http.MethodPut, "/api/auth/phone-pairing", map[string]any{"on": true}); out["ok"] != true {
		t.Fatalf("switching pairing phones on: %v", out)
	}
	phone.pairedSession(attic)
}
