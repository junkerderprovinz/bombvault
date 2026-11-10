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
	ph := joinPhone(t, relayURL, phrase, attic)
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
	ph := joinPhone(t, relayURL, phrase, attic)
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
