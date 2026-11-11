package store_test

import (
	"testing"
)

func TestANewDatabaseIsInNoFortress(t *testing.T) {
	r := kitRepo(t)
	m, err := r.FortressMembership()
	if err != nil {
		t.Fatal(err)
	}
	if m.Joined() || m.Device != "" || len(m.StateEnc) != 0 {
		t.Fatalf("a new database keeps %+v", m)
	}
}

func TestJoiningKeepsTheSealedSecretAndKeyAndLeavingKeepsOnlyTheName(t *testing.T) {
	r := kitRepo(t)
	if err := r.SetFortressDevice("tower"); err != nil {
		t.Fatal(err)
	}
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
	if !m.Joined() || string(m.SecretEnc) != "sealed secret" || string(m.SeedEnc) != "sealed seed" || string(m.StateEnc) != "sealed state" || m.Device != "tower" {
		t.Fatalf("joined: %+v", m)
	}

	if err := r.SetFortressSecret([]byte("new secret"), []byte("sealed secret")); err != nil {
		t.Fatal(err)
	}
	m, _ = r.FortressMembership()
	if string(m.SecretEnc) != "new secret" || string(m.PreviousEnc) != "sealed secret" || string(m.SeedEnc) != "sealed seed" {
		t.Fatalf("after a new phrase: %+v", m)
	}

	if err := r.LeaveFortress(); err != nil {
		t.Fatal(err)
	}
	m, _ = r.FortressMembership()
	if m.Joined() || len(m.SecretEnc)+len(m.PreviousEnc)+len(m.SeedEnc)+len(m.StateEnc) != 0 || m.Device != "tower" {
		t.Fatalf("after leaving: %+v", m)
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
	if err := r.SetFortressSecret([]byte("b"), []byte("a")); err != nil {
		t.Fatal(err)
	}
	if err := r.JoinFortress([]byte("c"), []byte("seed c")); err != nil {
		t.Fatal(err)
	}
	m, _ := r.FortressMembership()
	if string(m.SecretEnc) != "c" || string(m.SeedEnc) != "seed c" || len(m.PreviousEnc) != 0 || len(m.StateEnc) != 0 {
		t.Fatalf("joined again: %+v", m)
	}
}
