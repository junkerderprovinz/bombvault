package store_test

import (
	"reflect"
	"testing"
	"time"
)

func TestANewDatabaseIsInNoFortress(t *testing.T) {
	r := kitRepo(t)
	m, err := r.FortressMembership()
	if err != nil {
		t.Fatal(err)
	}
	if m.Joined() || len(m.StateEnc) != 0 {
		t.Fatalf("a new database keeps %+v", m)
	}
}

func TestJoiningKeepsTheSealedSecretAndKeyAndLeavingForgetsThem(t *testing.T) {
	r := kitRepo(t)
	if err := r.JoinFortress([]byte("sealed secret"), []byte("sealed seed")); err != nil {
		t.Fatal(err)
	}
	if err := r.SetFortressState([]byte("sealed state")); err != nil {
		t.Fatal(err)
	}
	m, err := r.FortressMembership()
	if err != nil {
		t.Fatal(err)
	}
	if !m.Joined() || string(m.SecretEnc) != "sealed secret" || string(m.SeedEnc) != "sealed seed" || string(m.StateEnc) != "sealed state" {
		t.Fatalf("joined: %+v", m)
	}

	changed := time.Unix(1_800_000_000, 0)
	if err := r.SetFortressSecret([]byte("new secret"), []byte("sealed secret"), changed); err != nil {
		t.Fatal(err)
	}
	m, _ = r.FortressMembership()
	if string(m.SecretEnc) != "new secret" || string(m.PreviousEnc) != "sealed secret" || string(m.SeedEnc) != "sealed seed" || !m.Changed.Equal(changed) {
		t.Fatalf("after a new phrase: %+v", m)
	}

	if err := r.LeaveFortress(); err != nil {
		t.Fatal(err)
	}
	m, _ = r.FortressMembership()
	if m.Joined() || len(m.SecretEnc)+len(m.PreviousEnc)+len(m.SeedEnc)+len(m.StateEnc) != 0 || !m.Changed.IsZero() {
		t.Fatalf("after leaving: %+v", m)
	}
}

func TestARestoredMembershipIsTheOneReadBefore(t *testing.T) {
	r := kitRepo(t)
	if err := r.JoinFortress([]byte("a"), []byte("seed a")); err != nil {
		t.Fatal(err)
	}
	if err := r.SetFortressSecret([]byte("b"), []byte("a"), time.Unix(1_800_000_000, 0)); err != nil {
		t.Fatal(err)
	}
	for _, set := range []func([]byte) error{r.SetFortressState, r.SetFortressPairing, r.SetFortressRemoved} {
		if err := set([]byte("kept")); err != nil {
			t.Fatal(err)
		}
	}
	before, err := r.FortressMembership()
	if err != nil {
		t.Fatal(err)
	}
	if err := r.JoinFortress([]byte("c"), []byte("seed c")); err != nil {
		t.Fatal(err)
	}
	if err := r.RestoreFortress(before); err != nil {
		t.Fatal(err)
	}
	if after, _ := r.FortressMembership(); !reflect.DeepEqual(after, before) {
		t.Fatalf("restored %+v, want %+v", after, before)
	}

	if err := r.LeaveFortress(); err != nil {
		t.Fatal(err)
	}
	if err := r.SetFortressRemovedHere(true); err != nil {
		t.Fatal(err)
	}
	out, _ := r.FortressMembership()
	if err := r.JoinFortress([]byte("d"), []byte("seed d")); err != nil {
		t.Fatal(err)
	}
	if err := r.RestoreFortress(out); err != nil {
		t.Fatal(err)
	}
	if after, _ := r.FortressMembership(); after.Joined() || !after.RemovedHere {
		t.Fatalf("restoring no membership left %+v", after)
	}
}

func TestJoiningAgainStartsWithoutTheFormerState(t *testing.T) {
	r := kitRepo(t)
	if err := r.JoinFortress([]byte("a"), []byte("seed a")); err != nil {
		t.Fatal(err)
	}
	if err := r.SetFortressState([]byte("state a")); err != nil {
		t.Fatal(err)
	}
	if err := r.SetFortressSecret([]byte("b"), []byte("a"), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := r.JoinFortress([]byte("c"), []byte("seed c")); err != nil {
		t.Fatal(err)
	}
	m, _ := r.FortressMembership()
	if string(m.SecretEnc) != "c" || string(m.SeedEnc) != "seed c" || len(m.PreviousEnc) != 0 || !m.Changed.IsZero() || len(m.StateEnc) != 0 {
		t.Fatalf("joined again: %+v", m)
	}
}

func TestThePairingInTheFortressIsKeptApartFromTheGroup(t *testing.T) {
	r := kitRepo(t)
	if err := r.JoinFortress([]byte("secret"), []byte("seed")); err != nil {
		t.Fatal(err)
	}
	if err := r.SetGroupKitState([]byte("old group")); err != nil {
		t.Fatal(err)
	}
	if err := r.SetFortressPairing([]byte("pairing")); err != nil {
		t.Fatal(err)
	}
	m, _ := r.FortressMembership()
	if string(m.PairingEnc) != "pairing" {
		t.Fatalf("pairing %q", m.PairingEnc)
	}
	if got, _ := r.GroupKitState(); string(got) != "old group" {
		t.Fatalf("the group keeps %q", got)
	}
	if err := r.JoinFortress([]byte("other"), []byte("seed")); err != nil {
		t.Fatal(err)
	}
	if m, _ := r.FortressMembership(); len(m.PairingEnc) != 0 {
		t.Fatalf("joining again keeps the pairing %q", m.PairingEnc)
	}
	if err := r.SetFortressPairing([]byte("pairing")); err != nil {
		t.Fatal(err)
	}
	if err := r.LeaveFortress(); err != nil {
		t.Fatal(err)
	}
	if m, _ := r.FortressMembership(); len(m.PairingEnc) != 0 {
		t.Fatalf("leaving keeps the pairing %q", m.PairingEnc)
	}
}

func TestForgettingTheOldPhraseKeepsTheNewOne(t *testing.T) {
	r := kitRepo(t)
	if err := r.JoinFortress([]byte("a"), []byte("seed")); err != nil {
		t.Fatal(err)
	}
	if err := r.SetFortressSecret([]byte("b"), []byte("a"), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := r.ForgetFortressPrevious(); err != nil {
		t.Fatal(err)
	}
	m, _ := r.FortressMembership()
	if string(m.SecretEnc) != "b" || string(m.SeedEnc) != "seed" || len(m.PreviousEnc) != 0 || !m.Changed.IsZero() {
		t.Fatalf("after forgetting the old phrase: %+v", m)
	}
}

func TestTheRemovedMembersStayUntilLeavingOrJoiningAgain(t *testing.T) {
	r := kitRepo(t)
	if err := r.JoinFortress([]byte("a"), []byte("seed a")); err != nil {
		t.Fatal(err)
	}
	if err := r.SetFortressRemoved([]byte("sealed list")); err != nil {
		t.Fatal(err)
	}
	if m, _ := r.FortressMembership(); string(m.RemovedEnc) != "sealed list" {
		t.Fatalf("removed = %q", m.RemovedEnc)
	}
	if err := r.LeaveFortress(); err != nil {
		t.Fatal(err)
	}
	if err := r.SetFortressRemovedHere(true); err != nil {
		t.Fatal(err)
	}
	if m, _ := r.FortressMembership(); len(m.RemovedEnc) != 0 || !m.RemovedHere {
		t.Fatalf("after another member took the install out: %+v", m)
	}
	if err := r.JoinFortress([]byte("b"), []byte("seed b")); err != nil {
		t.Fatal(err)
	}
	if m, _ := r.FortressMembership(); m.RemovedHere {
		t.Fatal("joining again still says another member took the install out")
	}
}

func TestAMemberSeenInTheSecondOfAPhraseChangeMayStillHoldTheOldPhrase(t *testing.T) {
	r := kitRepo(t)
	if err := r.JoinFortress([]byte("a"), []byte("seed")); err != nil {
		t.Fatal(err)
	}
	change := time.Unix(1_800_000_000, 700_000_000)
	if err := r.SetFortressSecret([]byte("b"), []byte("a"), change); err != nil {
		t.Fatal(err)
	}
	m, err := r.FortressMembership()
	if err != nil {
		t.Fatal(err)
	}
	for _, seen := range []time.Time{change.Add(-time.Hour), change.Add(-400 * time.Millisecond), change} {
		if !m.SeenBeforeChange(seen) {
			t.Errorf("a member last seen at %v is not counted as seen before the change at %v", seen, change)
		}
	}
	if later := change.Add(time.Second); m.SeenBeforeChange(later) {
		t.Errorf("a member seen at %v, after the change at %v, is counted as seen before it", later, change)
	}
}

func TestNoMemberIsSeenBeforeAChangeThatNeverHappened(t *testing.T) {
	r := kitRepo(t)
	if err := r.JoinFortress([]byte("a"), []byte("seed")); err != nil {
		t.Fatal(err)
	}
	m, err := r.FortressMembership()
	if err != nil {
		t.Fatal(err)
	}
	if m.SeenBeforeChange(time.Unix(1_800_000_000, 0)) {
		t.Error("a member is counted as seen before a phrase change in a fortress whose phrase never changed")
	}
}
