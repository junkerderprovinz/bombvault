package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/relay"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// signedAs is a call body signed with seed in the name of member.
func signedAs(t *testing.T, in *instance, seed []byte, member, action string, counter int64, payload []byte, body map[string]any) []byte {
	t.Helper()
	claim := keydeposit.Claim{Action: action, OwnerID: member, KeeperID: in.id(t), Counter: counter, Deposit: payload}
	body["key"], body["counter"], body["proof"] = keydeposit.OwnerKey(seed), counter, keydeposit.Prove(seed, claim)
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestARoleAnswerAndThePasswordNeedTheKeyOfTheMemberNamed(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	const member = "0123456789abcdef0123456789abcdef"
	if _, err := in.st.SaveSentRoleRequest(store.RoleRequest{MemberID: member, Role: store.RoleFetcher, Sections: []string{"containers"}, State: store.RoleAsked}); err != nil {
		t.Fatal(err)
	}
	call := func(method, path string, body []byte) string {
		_, out := in.svc.servePeer(context.Background(), relay.ProxyCall{Method: method, Path: path, Body: body, Sender: member})
		return string(out)
	}
	password := restickey.Derive(in.appKey)
	state := func() string {
		req, _, err := in.st.FindRoleRequest(store.RoleRequestOut, member, store.RoleFetcher)
		if err != nil {
			t.Fatal(err)
		}
		return req.State
	}
	member1, err := keydeposit.NewOwnerSeed()
	if err != nil {
		t.Fatal(err)
	}
	forger, err := keydeposit.NewOwnerSeed()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	answer := func(seed []byte, counter int64) []byte {
		return signedAs(t, in, seed, member, keydeposit.ProofRoleAnswer, counter, callPayload("fetcher", "allowed"), map[string]any{"role": "fetcher", "state": "allowed"})
	}

	// Nobody has the member's key yet, and the member is out of reach to ask.
	call(http.MethodPost, "/api/group/peer/roles/answer", []byte(`{"role":"fetcher","state":"allowed"}`))
	call(http.MethodPost, "/api/group/peer/roles/answer", answer(forger, now))
	if state() != store.RoleAsked {
		t.Fatalf("an answer in the member's name was taken: %s", state())
	}

	if err := in.st.PinMemberKey(member, keydeposit.OwnerKey(member1)); err != nil {
		t.Fatal(err)
	}
	call(http.MethodPost, "/api/group/peer/roles/answer", answer(forger, now+1))
	if state() != store.RoleAsked {
		t.Fatal("an answer signed with another key than the member's was taken")
	}
	call(http.MethodPost, "/api/group/peer/roles/answer", answer(member1, now+2))
	if state() != store.RoleAllowed {
		t.Fatalf("the member's own answer was not taken: %s", state())
	}

	pairing := func(seed []byte, counter int64) []byte {
		return signedAs(t, in, seed, member, keydeposit.ProofPairing, counter, nil, map[string]any{})
	}
	for _, body := range [][]byte{nil, pairing(forger, now+3), pairing(member1, now+2)} {
		if out := call(http.MethodGet, "/api/group/peer/pairing", body); strings.Contains(out, password) {
			t.Fatalf("the restic password went out on a call the member did not sign: %s", body)
		}
	}
	if out := call(http.MethodGet, "/api/group/peer/pairing", pairing(member1, now+4)); !strings.Contains(out, password) {
		t.Fatalf("the member's own signed call got no password: %s", out)
	}

	withdraw := signedAs(t, in, forger, member, keydeposit.ProofRoleWithdraw, now+5, callPayload("fetcher"), map[string]any{"role": "fetcher"})
	call(http.MethodPost, "/api/group/peer/roles/withdraw", withdraw)
	if _, ok, _ := in.st.FindRoleRequest(store.RoleRequestOut, member, store.RoleFetcher); !ok {
		t.Fatal("a withdrawal signed with another key went through")
	}
}

func TestAMemberIsAskedForItsKeyThroughTheGroup(t *testing.T) {
	a := newInstance(t, "cellar", strings.Repeat("a1", 32))
	b := newInstance(t, "attic", strings.Repeat("b2", 32))
	pairThroughRelay(t, a, b)
	key, err := a.svc.memberKey(context.Background(), b.id(t))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := b.svc.nextDepositSigner()
	if err != nil {
		t.Fatal(err)
	}
	if string(key) != string(keydeposit.OwnerKey(signer.seed)) {
		t.Fatal("the key pinned is not the one the member signs with")
	}
}

func TestTheOwnerForgetsAMembersKeyAndRemovingOrLeavingDropsIt(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	const member = "0123456789abcdef0123456789abcdef"
	pinned := func() bool {
		_, found, err := in.st.MemberKey(member)
		if err != nil {
			t.Fatal(err)
		}
		return found
	}
	pin := func() {
		if err := in.st.PinMemberKey(member, []byte("somebody's key")); err != nil {
			t.Fatal(err)
		}
	}
	path := "/api/group/members/" + member + "/key"

	pin()
	own := in.session
	hash, epoch, _ := in.h.authEnabled()
	in.as(secret.NewPairedSessionToken(in.appKey, hash, epoch, time.Hour), func() {
		if code, out := in.do(t, http.MethodDelete, path, nil); code != http.StatusForbidden || out["code"] != codeOwnLoginNeeded {
			t.Fatalf("forgetting a key with a paired session: %d %v", code, out)
		}
	})
	if !pinned() {
		t.Fatal("a paired session forgot a member's key")
	}
	in.session = own
	if _, out := in.do(t, http.MethodDelete, path, nil); out["ok"] != true || pinned() {
		t.Fatalf("forgetting the key: %v", out)
	}

	pin()
	peer, err := in.st.CreateFleetPeer(store.FleetPeer{MemberID: member, Name: "tower", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, out := in.do(t, http.MethodDelete, "/api/fleet/peers/"+peer.ID, nil); out["ok"] != true || pinned() {
		t.Fatalf("removing the member left its key: %v", out)
	}

	pin()
	if code, _ := in.do(t, http.MethodDelete, "/api/group", nil); code != http.StatusOK || pinned() {
		t.Fatalf("leaving the group left a member's key: %d", code)
	}
}
