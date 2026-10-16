package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
)

func listsMember(t *testing.T, in *instance, id string) bool {
	t.Helper()
	rows, err := in.svc.syncFleetPeers()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.MemberID == id {
			return true
		}
	}
	return false
}

func TestAPhoneThatLeavesTheGroupIsTakenOffTheInstancesPage(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	phrase, relayURL := groupOf(t, tower, attic)
	ph := joinPhone(t, relayURL, phrase, wordsHolderID, attic)
	if !listsMember(t, attic, ph.id) {
		t.Fatal("the phone has no row before it left")
	}

	body, err := json.Marshal(ph.signed(ph.seed, attic, keydeposit.ProofLeave, nil))
	if err != nil {
		t.Fatal(err)
	}
	if status, raw := ph.ask(attic.id(t), ph.id, "/api/group/peer/leave", body); status != http.StatusOK || !strings.Contains(string(raw), `"ok":true`) {
		t.Fatalf("the leave: %d %s", status, raw)
	}
	if listsMember(t, attic, ph.id) {
		t.Fatal("the phone keeps its row right after it left")
	}
	_ = ph.client.Close()
	deadline := time.Now().Add(10 * time.Second)
	for isMember(attic, ph.id) {
		if time.Now().After(deadline) {
			t.Fatal("the phone stayed on the relay")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if listsMember(t, attic, ph.id) {
		t.Fatal("the phone's row came back once it was gone from the relay")
	}
}

func TestAPhoneCannotBeTakenOffInItsNameWithoutItsKey(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	phrase, relayURL := groupOf(t, tower, attic)
	ph := joinPhone(t, relayURL, phrase, wordsHolderID, attic)
	if !listsMember(t, attic, ph.id) {
		t.Fatal("the phone has no row")
	}
	other, err := keydeposit.NewOwnerSeed()
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(ph.signed(other, attic, keydeposit.ProofLeave, nil))
	if err != nil {
		t.Fatal(err)
	}
	if status, raw := ph.ask(attic.id(t), ph.id, "/api/group/peer/leave", body); strings.Contains(string(raw), `"ok":true`) {
		t.Fatalf("a leave signed with another key was taken: %d %s", status, raw)
	}
	if !listsMember(t, attic, ph.id) {
		t.Fatal("a leave signed with another key took the phone off")
	}
}

func TestAnInstanceThatLeavesTellsThePhonesOfTheGroup(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	phrase, relayURL := groupOf(t, tower, attic)
	ph := joinPhone(t, relayURL, phrase, wordsHolderID, attic)
	atticID, atticKey := attic.id(t), keydeposit.OwnerKey(depositSeedOf(t, attic))

	if code, out := attic.do(t, http.MethodDelete, "/api/group", nil); code != http.StatusOK || out["active"] != false {
		t.Fatalf("leave: %d %v", code, out)
	}
	select {
	case call := <-ph.heard:
		if call.Method != http.MethodPost || call.Path != "/api/group/peer/leave" || call.Sender != atticID {
			t.Fatalf("the phone heard %s %s from %q", call.Method, call.Path, call.Sender)
		}
		var p peerProof
		if err := json.Unmarshal(call.Body, &p); err != nil {
			t.Fatal(err)
		}
		claim := keydeposit.Claim{Action: keydeposit.ProofLeave, OwnerID: atticID, KeeperID: ph.id, Counter: p.Counter}
		if !keydeposit.Proven(atticKey, p.Proof, claim) {
			t.Fatal("the word the phone got is not signed with the key of the instance that left")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the phone was not told that the instance left")
	}
}
