package api

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/phrase"

	"github.com/junkerderprovinz/bombvault/internal/secret"
)

// newPhrase is a fresh phrase and the secret it encodes.
func newPhrase(t *testing.T) (string, []byte) {
	t.Helper()
	sec, words, err := phrase.New()
	if err != nil {
		t.Fatal(err)
	}
	return words, sec
}

func joinedSvc(t *testing.T) (*Service, string) {
	t.Helper()
	svc, _ := newFortressSvc(t)
	words, _ := newPhrase(t)
	if err := svc.JoinFortress(words); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.CloseFortress)
	return svc, words
}

func TestJoiningWithThePhraseOpensTheFortressAndKeepsItSealed(t *testing.T) {
	svc, st := newFortressSvc(t)
	words, sec := newPhrase(t)
	if err := svc.JoinFortress(words); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.CloseFortress)
	if svc.fortressNow() == nil {
		t.Fatal("joining opened no fortress")
	}
	m, err := st.FortressMembership()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(m.SecretEnc, sec) {
		t.Fatal("the secret is kept in the clear")
	}
	if plain, err := secret.Decrypt(svc.cfg.AppKey, m.SecretEnc); err != nil || !bytes.Equal(plain, sec) {
		t.Fatalf("the sealed secret opens to %x, %v", plain, err)
	}
	if got, err := svc.fortressPhrase(); err != nil || got != words {
		t.Fatalf("the phrase reads %q, %v", got, err)
	}
}

func TestAMistypedPhraseJoinsNothing(t *testing.T) {
	svc, st := newFortressSvc(t)
	err := svc.JoinFortress("abandon abandon abandon abandon abandon abandon recieve abandon abandon abandon abandon about")
	var de *phrase.DecodeError
	if !errors.As(err, &de) || de.Word != "recieve" || de.Position != 7 {
		t.Fatalf("joining with a typo: %v", err)
	}
	if m, _ := st.FortressMembership(); m.Joined() || svc.fortressNow() != nil {
		t.Fatal("a phrase that did not decode joined")
	}
}

func TestBombVaultJoinsOneFortressAtATime(t *testing.T) {
	svc, _ := joinedSvc(t)
	words, _ := newPhrase(t)
	if err := svc.JoinFortress(words); !errors.Is(err, errInFortress) {
		t.Fatalf("joining a second fortress: %v", err)
	}
}

func TestTheMembershipOpensAgainOnTheNextStart(t *testing.T) {
	svc, words := joinedSvc(t)
	svc.CloseFortress()
	if err := svc.OpenFortress(nil); err != nil {
		t.Fatal(err)
	}
	if svc.fortressNow() == nil {
		t.Fatal("the membership did not open again")
	}
	if got, _ := svc.fortressPhrase(); got != words {
		t.Fatalf("the phrase after a start reads %q", got)
	}
}

func TestLeavingForgetsTheMembership(t *testing.T) {
	svc, st := newFortressSvc(t)
	words, _ := newPhrase(t)
	if err := svc.JoinFortress(words); err != nil {
		t.Fatal(err)
	}
	if err := svc.LeaveFortress(); err != nil {
		t.Fatal(err)
	}
	if m, _ := st.FortressMembership(); m.Joined() || svc.fortressNow() != nil {
		t.Fatal("the membership stayed after leaving")
	}
	if _, err := svc.fortressPhrase(); !errors.Is(err, errNoFortress) {
		t.Fatalf("the phrase after leaving: %v", err)
	}
	if err := svc.LeaveFortress(); !errors.Is(err, errNoFortress) {
		t.Fatalf("leaving twice: %v", err)
	}
}

func TestLeavingAlsoForgetsAMembershipFromTheConsole(t *testing.T) {
	svc, _ := newFortressSvc(t)
	joinFromConsole(t, svc)
	if err := svc.OpenFortress(nil); err != nil {
		t.Fatal(err)
	}
	if err := svc.LeaveFortress(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(svc.fortressDir()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the membership folder is still there: %v", err)
	}
}

func TestChangingThePhraseKeepsTheNewOneAndTheOldForTheMembers(t *testing.T) {
	svc, old := joinedSvc(t)
	oldSecret, _ := phrase.Decode(old)
	words, left, err := svc.ChangeFortressPhrase(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if words == old || len(left) != 0 {
		t.Fatalf("changed to %q, left %v", words, left)
	}
	if got, _ := svc.fortressPhrase(); got != words {
		t.Fatalf("the phrase reads %q after the change to %q", got, words)
	}
	m, _ := svc.store.FortressMembership()
	if prev, err := secret.Decrypt(svc.cfg.AppKey, m.PreviousEnc); err != nil || !bytes.Equal(prev, oldSecret) {
		t.Fatal("the old secret is not kept to carry the members over")
	}
	if svc.fortressNow() == nil {
		t.Fatal("the fortress did not open on the new phrase")
	}
}

func TestANewPhraseFromAnotherMemberIsKeptAndOpened(t *testing.T) {
	svc, _ := joinedSvc(t)
	before := svc.fortressNow()
	words, sec := newPhrase(t)
	if err := (fortressKeeper{svc}).Keep(sec); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.fortressPhrase(); got != words {
		t.Fatalf("the phrase reads %q, want the new one", got)
	}
	deadline := time.Now().Add(5 * time.Second)
	for svc.fortressNow() == before || svc.fortressNow() == nil {
		if time.Now().After(deadline) {
			t.Fatal("the fortress did not open again on the new phrase")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestTheServerIsNamedAfterUnraidUntilSomebodyNamesIt(t *testing.T) {
	svc, _ := newFortressSvc(t)
	svc.cfg.FlashDir = t.TempDir()
	if got := svc.fortressDevice(); got != "BombVault" {
		t.Fatalf("without a flash the server is %q", got)
	}
	if err := os.MkdirAll(filepath.Join(svc.cfg.FlashDir, "config"), 0o750); err != nil {
		t.Fatal(err)
	}
	ident := "NAME=\"Tower\"\nCOMMENT=\"Media server\"\n"
	if err := os.WriteFile(filepath.Join(svc.cfg.FlashDir, "config", "ident.cfg"), []byte(ident), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := svc.fortressDevice(); got != "Tower" {
		t.Fatalf("on Unraid the server is %q", got)
	}

	words, _ := newPhrase(t)
	if err := svc.JoinFortress(words); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.CloseFortress)
	if err := svc.SetFortressDevice("  cellar "); err != nil {
		t.Fatal(err)
	}
	if got := svc.fortressDevice(); got != "cellar" {
		t.Fatalf("named, the server is %q", got)
	}
	svc.house.mu.Lock()
	device := svc.house.device
	svc.house.mu.Unlock()
	if device != "cellar" {
		t.Fatalf("the open fortress knows the server as %q", device)
	}
	if err := svc.SetFortressDevice(""); err != nil {
		t.Fatal(err)
	}
	if got := svc.fortressDevice(); got != "Tower" {
		t.Fatalf("cleared, the server is %q", got)
	}
}

func TestAMemberThatTookBombVaultOutEndsTheMembership(t *testing.T) {
	svc, _ := joinedSvc(t)
	st := svc.store
	svc.house.mu.Lock()
	gen := svc.house.gen
	svc.house.mu.Unlock()
	svc.removedFrom(gen)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if m, _ := st.FortressMembership(); !m.Joined() && svc.fortressNow() == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the membership stayed after another member took BombVault out")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
