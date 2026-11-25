package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/signin"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// wordsHolder is somebody who has the twelve words and nothing else: no
// BombVault, no APP_KEY and no password of any instance. The group pinned its
// key, so its calls reach an instance as its own; a call in another's name is
// handed over as if the group vouched for it, which no member can make it do.
type wordsHolder struct {
	t    *testing.T
	id   string
	kind string
	seed []byte
	on   map[string]*instance
}

// holdWords makes the holder a member of kind under id on every instance of
// seenBy, the way the group lists one that proved it holds the words.
func holdWords(t *testing.T, id, kind string, seenBy ...*instance) *wordsHolder {
	t.Helper()
	seed, err := keydeposit.NewOwnerSeed()
	if err != nil {
		t.Fatal(err)
	}
	w := &wordsHolder{t: t, id: id, kind: kind, seed: seed, on: map[string]*instance{}}
	for _, in := range seenBy {
		w.on[in.id(t)] = in
		if err := in.st.PinMemberKey(id, keydeposit.OwnerKey(seed)); err != nil {
			t.Fatal(err)
		}
		if _, err := in.st.CreateFleetPeer(store.FleetPeer{MemberID: id, Name: "tower", Kind: kind, Enabled: true}); err != nil {
			t.Fatal(err)
		}
	}
	return w
}

// call sends target a peer call that names sender as where it came from.
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
	in := w.on[target]
	if in == nil {
		w.t.Fatalf("the holder is no member on %s", target)
	}
	kind := ""
	if sender == w.id {
		kind = w.kind
	}
	status, res := in.svc.callPeer(context.Background(), peerCall{Method: method, Path: path, Body: raw, Sender: sender, Kind: kind, Key: keydeposit.OwnerKey(w.seed)})
	out := map[string]any{}
	_ = json.Unmarshal(res, &out)
	out["_status"] = float64(status)
	return out
}

// pairedSession is the session an instance gives the holder of the words for
// calling itself a phone.
func (w *wordsHolder) pairedSession(in *instance) string {
	w.t.Helper()
	at := time.Now().UnixMilli()
	claim := keydeposit.Claim{Action: keydeposit.ProofSession, OwnerID: w.id, KeeperID: in.id(w.t), Counter: at, Deposit: callPayload(w.id)}
	out := w.call(in.id(w.t), w.id, http.MethodPost, "/api/group/peer/session", map[string]any{
		"instanceId": w.id, "key": keydeposit.OwnerKey(w.seed), "counter": at, "proof": keydeposit.Prove(w.seed, claim),
	})
	value, _ := out["value"].(string)
	hash, epoch, _ := in.h.phoneSigning()
	if !secret.ValidSessionToken(in.appKey, hash, epoch, value) {
		w.t.Fatalf("no session that signs in: %v", out)
	}
	return value
}

// post sends target a peer call in the name of sender and decodes the answer.
func (p *phone) post(target, sender, path string, body any) map[string]any {
	p.t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		p.t.Fatal(err)
	}
	status, res := p.ask(target, sender, path, raw)
	out := map[string]any{}
	_ = json.Unmarshal(res, &out)
	out["_status"] = float64(status)
	return out
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
	mallory := joinPhone(t, relayURL, phrase, attic)
	mallory.signIn(attic)
	paired := mallory.session

	inOwnersName := mallory.post(atticID, towerID, "/api/group/peer/deposit/release", peerReleaseAsk{OwnerID: towerID, Name: "tower", Renew: true})
	if inOwnersName["ok"] != false || len(releasesOn(t, attic)) != 0 {
		t.Fatalf("a request in the owner's own id was taken: %v", inOwnersName)
	}

	// The words make a member of whoever holds them, with a key of its own.
	seed := mallory.seed
	signed := signedAs(t, attic, seed, mallory.id, keydeposit.ProofRelease, time.Now().UnixMilli(), callPayload(towerID, "tower", "true"),
		map[string]any{"ownerId": towerID, "name": "tower", "renew": true})
	asked := mallory.post(atticID, mallory.id, "/api/group/peer/deposit/release", json.RawMessage(signed))
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
	again := signedAs(t, attic, seed, mallory.id, keydeposit.ProofRelease, time.Now().UnixMilli()+1, callPayload(towerID, "tower", "false"),
		map[string]any{"ownerId": towerID, "name": "tower", "renew": false})
	if got := mallory.post(atticID, mallory.id, "/api/group/peer/deposit/release", json.RawMessage(again)); got["deposit"] != nil || got["state"] != store.ReleaseAsked {
		t.Fatalf("the keeper handed something out: %v", got)
	}

	if out := decideRelease(t, attic, waiting[0].ID, "allow"); out["state"] != store.ReleaseAllowed {
		t.Fatalf("a signed-in person allows: %v", out)
	}
}

func TestAPairedSessionOpensThePagesAndStopsAtKeysAndTheLogin(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	groupOf(t, tower, attic)
	paired := holdWords(t, wordsHolderID, "android", attic).pairedSession(attic)
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
			{http.MethodPut, signin.APIPrefix + "me/password", map[string]any{"next": "a new password of mine"}},
			{http.MethodPost, signin.APIPrefix + "me/totp", map[string]any{}},
			{http.MethodPost, signin.APIPrefix + "me/passkeys/options", map[string]any{}},
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

func TestAPhoneTakenOffTheInstancesPageLosesItsSessionAtTheOwnAddress(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	groupOf(t, tower, attic)
	paired := holdWords(t, wordsHolderID, "android", attic).pairedSession(attic)
	status := func() int {
		var code int
		attic.as(paired, func() { code, _ = attic.do(t, http.MethodGet, "/api/status", nil) })
		return code
	}
	if code := status(); code != http.StatusOK {
		t.Fatalf("the phone's session before anything changed: %d", code)
	}
	rows, err := attic.st.ListFleetPeers()
	if err != nil {
		t.Fatal(err)
	}
	row := ""
	for _, p := range rows {
		if p.MemberID == wordsHolderID {
			row = p.ID
		}
	}
	if code, out := attic.do(t, http.MethodPut, "/api/fleet/peers/"+row, map[string]any{"enabled": false}); out["ok"] != true {
		t.Fatalf("switching the phone's row off: %d %v", code, out)
	}
	if code := status(); code != http.StatusUnauthorized {
		t.Fatalf("the session of a phone whose row is off: %d", code)
	}
	if code, out := attic.do(t, http.MethodPut, "/api/fleet/peers/"+row, map[string]any{"enabled": true}); out["ok"] != true {
		t.Fatalf("switching the phone's row on: %d %v", code, out)
	}
	if code := status(); code != http.StatusOK {
		t.Fatalf("the session of a phone switched on again: %d", code)
	}
	if code, out := attic.do(t, http.MethodDelete, "/api/group/members/"+wordsHolderID, nil); out["ok"] != true {
		t.Fatalf("removing the phone: %d %v", code, out)
	}
	if code := status(); code != http.StatusUnauthorized {
		t.Fatalf("the session of a removed phone: %d", code)
	}
	attic.as(paired, func() {
		if _, out := attic.do(t, http.MethodGet, "/api/auth", nil); out["authed"] != false {
			t.Fatalf("the auth status of a removed phone's session: %v", out)
		}
	})
}

func TestTheKeyIsDepositedWithoutALoginPassword(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	groupOf(t, tower, attic)
	removePasswordOn(t, tower.router, tower.session, testPassword)
	tower.session = ""
	if out := deposit(t, tower, attic); out["ok"] != true {
		t.Fatalf("depositing without a password: %v", out)
	}
	if _, found := kept(t, attic, tower); !found {
		t.Fatal("the keeper holds no deposit")
	}
	if code, out := tower.do(t, http.MethodPost, "/api/group/deposit/code", nil); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("a new recovery code without a password: %d %v", code, out)
	}
}

func TestAPhoneOfTheGroupPairsAgainAfterAReset(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	groupOf(t, tower, attic)
	phone := holdWords(t, wordsHolderID, "android", attic)
	old := phone.pairedSession(attic)

	resetOn(t, attic, "a new password")
	attic.as(old, func() {
		if code, _ := attic.do(t, http.MethodGet, "/api/status", nil); code != http.StatusUnauthorized {
			t.Fatalf("the phone's session from before the reset: %d", code)
		}
	})
	fresh := phone.pairedSession(attic)
	attic.as(fresh, func() {
		if code, _ := attic.do(t, http.MethodGet, "/api/status", nil); code != http.StatusOK {
			t.Fatalf("the phone's session after pairing again: %d", code)
		}
	})
}

func isMember(in *instance, id string) bool {
	for _, m := range in.svc.pairing().Members() {
		if m.ID == id {
			return true
		}
	}
	return false
}
