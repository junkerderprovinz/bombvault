package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/relay"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// peerCallWith sends in one peer call with a method of the caller's choice,
// as peerCallAs does a POST.
func peerCallWith(t *testing.T, in *instance, method, sender, path string, body any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	_, answer := in.svc.servePeer(context.Background(), relay.ProxyCall{Method: method, Path: path, Body: raw, Sender: sender})
	out := map[string]any{}
	_ = json.Unmarshal(answer, &out)
	return out
}

// plantedDeposit is a deposit somebody else makes in an install's name,
// signed with a key of their own.
func plantedDeposit(t *testing.T, ownerID, keeperID string, counter int64) (peerDeposit, []byte) {
	t.Helper()
	seed, err := keydeposit.NewOwnerSeed()
	if err != nil {
		t.Fatal(err)
	}
	d := peerDeposit{Name: "tower", Deposit: []byte("not the owner's deposit"), Salt: []byte("0123456789abcdef"), Version: keydeposit.Version, Renew: true}
	claim := keydeposit.Claim{Action: keydeposit.ProofKeep, OwnerID: ownerID, KeeperID: keeperID, Counter: counter, Deposit: d.Deposit, Salt: d.Salt}
	d.peerProof = peerProof{Key: keydeposit.OwnerKey(seed), Counter: counter, Proof: keydeposit.Prove(seed, claim)}
	return d, seed
}

func TestAMemberCanNeitherReplaceNorTakeBackAnotherInstallsDeposit(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	phrase, relayURL := groupOf(t, tower, attic)
	code, _ := deposit(t, tower, attic)["code"].(string)
	towerID, atticID := tower.id(t), attic.id(t)
	mallory := holdWords(t, relayURL, phrase, wordsHolderID, "", attic)
	before, _ := kept(t, attic, tower)
	far := before.Counter + 1_000_000

	unsigned := peerDeposit{Name: "tower", Deposit: []byte("junk"), Salt: []byte("0123456789abcdef"), Version: keydeposit.Version}
	if out := mallory.call(atticID, towerID, http.MethodPut, "/api/group/peer/deposit", unsigned); out["ok"] != false {
		t.Fatalf("a deposit without a proof: %v", out)
	}
	planted, seed := plantedDeposit(t, towerID, atticID, far)
	if out := mallory.call(atticID, towerID, http.MethodPut, "/api/group/peer/deposit", planted); out["state"] != store.DepositChanged {
		t.Fatalf("a deposit signed with another key: %v", out)
	}
	drop := keydeposit.Claim{Action: keydeposit.ProofDrop, OwnerID: towerID, KeeperID: atticID, Counter: far + 1}
	for name, proof := range map[string]peerProof{
		"no proof":            {},
		"another key's proof": {Key: keydeposit.OwnerKey(seed), Counter: far + 1, Proof: keydeposit.Prove(seed, drop)},
	} {
		if out := mallory.call(atticID, towerID, http.MethodDelete, "/api/group/peer/deposit", proof); out["ok"] != false {
			t.Fatalf("taking the deposit back with %s: %v", name, out)
		}
	}

	after, found := kept(t, attic, tower)
	if !found || !bytes.Equal(after.Deposit, before.Deposit) {
		t.Fatal("the keeper does not hold the owner's deposit")
	}
	if plain, err := keydeposit.Open(after.Deposit, code, after.Salt); err != nil || !strings.Contains(string(plain), tower.appKey) {
		t.Fatalf("the deposit does not open with the owner's code: %v", err)
	}

	if told, err := tower.svc.WithdrawDeposit(context.Background(), atticID); err != nil || !told {
		t.Fatalf("the owner takes its deposit back: %v, %v", told, err)
	}
	if _, found := kept(t, attic, tower); found {
		t.Fatal("the deposit outlived the owner's own call")
	}
}

func TestAnIdThatIsNoMemberReachesNoDepositRoute(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	phrase, relayURL := groupOf(t, tower, attic)
	deposit(t, tower, attic)
	towerID, atticID := tower.id(t), attic.id(t)
	mallory := holdWords(t, relayURL, phrase, wordsHolderID, "", attic)

	for i := range 12 {
		id := fmt.Sprintf("%032x", i+1)
		out := mallory.call(atticID, id, http.MethodPost, "/api/group/peer/deposit/release", peerReleaseAsk{OwnerID: towerID, Name: "tower", Renew: true})
		if out["ok"] != false || out["_status"] != float64(http.StatusForbidden) {
			t.Fatalf("a request under an id nobody joined with: %v", out)
		}
	}
	if asked := releasesOn(t, attic); len(asked) != 0 {
		t.Fatalf("requests recorded: %+v", asked)
	}
	stranger := fmt.Sprintf("%032x", 99)
	planted, _ := plantedDeposit(t, stranger, atticID, 1)
	if out := mallory.call(atticID, stranger, http.MethodPut, "/api/group/peer/deposit", planted); out["ok"] != false {
		t.Fatalf("a deposit under an id nobody joined with: %v", out)
	}
	if all, err := attic.st.KeptDeposits(); err != nil || len(all) != 1 {
		t.Fatalf("the keeper holds %d deposits, %v", len(all), err)
	}
}

func TestAKeeperTakesNoDepositLargerThanAKitCanBe(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	groupOf(t, tower, attic)
	planted, _ := plantedDeposit(t, tower.id(t), attic.id(t), 1)
	planted.Deposit = bytes.Repeat([]byte("x"), maxDepositBytes+1)
	out := peerCallWith(t, attic, http.MethodPut, tower.id(t), "/api/group/peer/deposit", planted)
	if out["ok"] != false {
		t.Fatalf("an oversized deposit: %v", out)
	}
	if _, found := kept(t, attic, tower); found {
		t.Fatal("it was kept")
	}
}

func TestAnInstallNoticesAKeeperThatHoldsSomethingElse(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	groupOf(t, tower, attic)
	code, _ := deposit(t, tower, attic)["code"].(string)
	opens := func() bool {
		held, found := kept(t, attic, tower)
		if !found {
			return false
		}
		_, err := keydeposit.Open(held.Deposit, code, held.Salt)
		return err == nil
	}

	tower.svc.RenewDeposits(context.Background())
	if sent, _ := tower.st.SentDeposits(); sent[0].ChangedAt != 0 || slices.Contains(keyRunsOf(t, tower), "keychanged failed") {
		t.Fatalf("a keeper that holds what was sent counts as changed: %+v", sent[0])
	}

	for name, change := range map[string]string{
		"other bytes": `UPDATE key_deposits SET deposit = x'6a756e6b'`,
		"nothing":     `DELETE FROM key_deposits`,
	} {
		if _, err := attic.db.Exec(change); err != nil {
			t.Fatal(err)
		}
		if _, err := tower.db.Exec(`UPDATE key_deposits_sent SET changed_at = 0`); err != nil {
			t.Fatal(err)
		}
		if opens() {
			t.Fatalf("%s: the deposit was not changed", name)
		}
		tower.svc.RenewDeposits(context.Background())
		if !opens() {
			t.Fatalf("%s: the renewal did not put the owner's deposit back", name)
		}
		sent, err := tower.st.SentDeposits()
		if err != nil || sent[0].ChangedAt == 0 || sent[0].State != store.DepositHeld {
			t.Fatalf("%s: the owner records %+v, %v", name, sent, err)
		}
	}
	runs := keyRunsOf(t, tower)
	if n := len(runs); n < 2 || runs[n-2] != "keychanged failed" || runs[n-1] != "keyrenew success" {
		t.Fatalf("the owner's activity log ends with %v", runs)
	}
	_, roles := tower.do(t, http.MethodGet, "/api/group/roles", nil)
	keeper := roles["members"].([]any)[0].(map[string]any)["byMember"].(map[string]any)["keeper"].(map[string]any)
	if keeper["changedAt"] == nil || keeper["changedAt"] == "" {
		t.Fatalf("the tile is not told: %v", keeper)
	}
}

func TestADepositSomebodyElseMadeUnderAnInstallsIdShowsOnItsSide(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	groupOf(t, tower, attic)
	planted, _ := plantedDeposit(t, tower.id(t), attic.id(t), 1)
	if out := peerCallWith(t, attic, http.MethodPut, tower.id(t), "/api/group/peer/deposit", planted); out["state"] != store.DepositHeld {
		t.Fatalf("the first deposit under the id: %v", out)
	}

	out := deposit(t, tower, attic)
	if keeper, _ := out["keeper"].(map[string]any); out["ok"] != true || keeper["state"] != store.DepositChanged {
		t.Fatalf("the owner's deposit against a planted one: %v", out)
	}
	if held, _ := kept(t, attic, tower); string(held.Deposit) != "not the owner's deposit" {
		t.Fatal("the keeper replaced a deposit of another key")
	}

	if code, res := attic.do(t, http.MethodDelete, "/api/group/deposits/"+tower.id(t), map[string]any{}); code != http.StatusOK || res["ok"] != true {
		t.Fatalf("the keeper's person deletes it: %d %v", code, res)
	}
	out = deposit(t, tower, attic)
	if keeper, _ := out["keeper"].(map[string]any); keeper["state"] != store.DepositHeld {
		t.Fatalf("the owner's deposit after the delete: %v", out)
	}
	held, _ := kept(t, attic, tower)
	if string(held.Deposit) == "not the owner's deposit" || len(held.OwnerKey) == 0 {
		t.Fatal("the keeper still holds the planted deposit")
	}
}
