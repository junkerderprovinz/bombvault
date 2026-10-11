package api

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/secret"
)

func TestTheRecoveryCodesPublicKeyIsStoredSealed(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	groupOf(t, tower, attic)
	code, _ := deposit(t, tower, attic)["recoveryCode"].(string)

	identity, found, err := tower.st.DepositIdentity()
	if err != nil || !found {
		t.Fatalf("the identity: %v, %v", found, err)
	}
	recipient, err := keydeposit.Recipient(code, identity.Salt)
	if err != nil {
		t.Fatal(err)
	}
	if len(identity.Recipient) != 0 || bytes.Contains(identity.RecipientEnc, recipient) {
		t.Fatal("the public key of the recovery code is stored in the clear")
	}
	if opened, err := secret.Decrypt(tower.appKey, identity.RecipientEnc); err != nil || !bytes.Equal(opened, recipient) {
		t.Fatalf("the sealed public key is not the code's: %v", err)
	}

	// A database from before the key was sealed is brought up on first use.
	if _, err := tower.db.Exec(`UPDATE key_deposit_identity SET recipient = ?, recipient_enc = x''`, recipient); err != nil {
		t.Fatal(err)
	}
	if _, err := tower.db.Exec(`UPDATE key_deposits_sent SET digest = ''`); err != nil {
		t.Fatal(err)
	}
	tower.svc.RenewDeposits(context.Background())
	identity, _, _ = tower.st.DepositIdentity()
	if len(identity.Recipient) != 0 || len(identity.RecipientEnc) == 0 {
		t.Fatal("a public key kept in the clear stayed that way")
	}
	held, _ := kept(t, attic, tower)
	if _, err := keydeposit.Open(held.Deposit, code, held.Salt); err != nil {
		t.Fatalf("the renewed deposit does not open with the code: %v", err)
	}
}

func TestADepositSaysInsideWhoseItIsAndWhenItWasSealed(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	groupOf(t, tower, attic)
	code, _ := deposit(t, tower, attic)["recoveryCode"].(string)
	held, _ := kept(t, attic, tower)
	plain, err := keydeposit.Open(held.Deposit, code, held.Salt)
	if err != nil {
		t.Fatal(err)
	}
	content, err := keydeposit.Unpack(plain)
	if err != nil {
		t.Fatal(err)
	}
	if content.OwnerID != tower.id(t) || content.OwnerName != "tower" || content.Counter != held.Counter {
		t.Fatalf("the deposit names %q %q at %d, the keeper has it at %d", content.OwnerID, content.OwnerName, content.Counter, held.Counter)
	}
	if age := time.Now().Unix() - content.SealedAt; age < 0 || age > 60 {
		t.Fatalf("the deposit was sealed %d seconds ago", age)
	}
}

func TestADepositThatAnotherInstallSealedIsNotUsed(t *testing.T) {
	attic, fresh, towerID, towerKey, code := goneTower(t)
	held, found, err := attic.st.KeptDeposit(towerID)
	if err != nil || !found {
		t.Fatal(err)
	}
	plain, err := keydeposit.Open(held.Deposit, code, held.Salt)
	if err != nil {
		t.Fatal(err)
	}
	content, err := keydeposit.Unpack(plain)
	if err != nil {
		t.Fatal(err)
	}
	// The same kit under the same code, sealed as another install's.
	content.OwnerID = strings.Repeat("7", 32)
	planted, err := keydeposit.Pack(content)
	if err != nil {
		t.Fatal(err)
	}
	recipient, err := keydeposit.Recipient(code, held.Salt)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := keydeposit.Seal(planted, recipient)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := attic.db.Exec(`UPDATE key_deposits SET deposit = ? WHERE owner_id = ?`, sealed, towerID); err != nil {
		t.Fatal(err)
	}

	emergencyGroup(t, fresh, attic, towerID, code, true)
	decideRelease(t, attic, releasesOn(t, attic)[0].ID, "allow")
	out := emergencyGroup(t, fresh, attic, towerID, code, false)
	if out["ok"] != false || out["code"] != "deposit-owner" || out["emergency"] != nil {
		t.Fatalf("a deposit sealed as another install's: %v", out)
	}
	if _, err := fresh.svc.currentEmergency(); !errors.Is(err, errNoEmergency) {
		t.Fatalf("an emergency started from it (key known: %v)", towerKey != "")
	}
}

func TestAnEmergencyNamesWhoseKitItReadsAndHowOldItIs(t *testing.T) {
	attic, fresh, towerID, _, code := goneTower(t)
	emergencyGroup(t, fresh, attic, towerID, code, true)
	decideRelease(t, attic, releasesOn(t, attic)[0].ID, "allow")
	out := emergencyGroup(t, fresh, attic, towerID, code, false)
	kit, _ := out["emergency"].(map[string]any)["session"].(map[string]any)["kit"].(map[string]any)
	written, _ := kit["writtenAt"].(float64)
	if kit["ownerId"] != towerID || kit["server"] != "tower" || written == 0 || kit["newerWith"] != nil {
		t.Fatalf("the emergency says of its kit: %v", kit)
	}

	f := newPlacementFixture(t)
	_, view := f.emergency("POST", "/api/emergency/read", map[string]any{"kit": kitOfTheOldInstall(t)})
	if got := view.Session.Kit; got.Server != "BombVault" || got.OwnerID != "" || time.Now().Unix()-got.WrittenAt > 120 || got.WrittenAt == 0 {
		t.Fatalf("an uploaded kit is described as %+v", got)
	}
}

func TestANewerDepositWithAnotherMemberIsNamed(t *testing.T) {
	tower := newInstance(t, "tower", strings.Repeat("a1", 32))
	attic := newInstance(t, "attic", strings.Repeat("b2", 32))
	barn := newInstance(t, "barn", strings.Repeat("d4", 32))
	phrase, relayURL := groupOf(t, tower, attic, barn)
	code, _ := deposit(t, tower, attic)["recoveryCode"].(string)
	deposit(t, tower, barn)
	towerID := tower.id(t)
	tower.svc.StopGroup()
	fresh := newInstance(t, "fresh", strings.Repeat("c3", 32))
	joinGroup(t, fresh, relayURL, phrase)
	deadline := time.Now().Add(10 * time.Second)
	for len(fresh.svc.pairing().Members()) < 2 || !isMember(attic, fresh.id(t)) || !isMember(barn, fresh.id(t)) {
		if time.Now().After(deadline) {
			t.Fatal("the fresh install never reached both keepers")
		}
		time.Sleep(20 * time.Millisecond)
	}

	_, offered := fresh.do(t, http.MethodGet, "/api/emergency/deposits", nil)
	newest := map[string]bool{}
	for _, raw := range offered["deposits"].([]any) {
		d := raw.(map[string]any)
		newest[d["memberName"].(string)] = d["newest"].(bool)
	}
	if newest["attic"] || !newest["barn"] {
		t.Fatalf("the later deposit is with barn, the list says newest: %v", newest)
	}

	emergencyGroup(t, fresh, attic, towerID, code, true)
	decideRelease(t, attic, releasesOn(t, attic)[0].ID, "allow")
	out := emergencyGroup(t, fresh, attic, towerID, code, false)
	kit, _ := out["emergency"].(map[string]any)["session"].(map[string]any)["kit"].(map[string]any)
	if kit["newerWith"] != "barn" {
		t.Fatalf("the older deposit was opened and the emergency says of it: %v", kit)
	}
}
