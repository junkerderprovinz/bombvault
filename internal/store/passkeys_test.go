package store

import (
	"errors"
	"testing"
)

// newPasskeyRepo returns a Repo over a migrated in-memory database.
func newPasskeyRepo(t *testing.T) *Repo {
	t.Helper()
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return New(db)
}

func TestPasskeyRoundtrip(t *testing.T) {
	r := newPasskeyRepo(t)

	saved, err := r.AddPasskey(Passkey{
		Name:         "Handy",
		CredentialID: []byte{1, 2, 3},
		PublicKey:    []byte{9, 9},
		AAGUID:       []byte{7},
		Transports:   "internal,hybrid",
		RPID:         "bombvault.example.com",
		BackedUp:     true,
	})
	if err != nil {
		t.Fatalf("AddPasskey: %v", err)
	}
	if saved.ID == "" || saved.CreatedAt == 0 {
		t.Fatalf("the saved row carries no id or timestamp: %+v", saved)
	}

	got, ok, err := r.PasskeyByCredentialID([]byte{1, 2, 3})
	if err != nil || !ok {
		t.Fatalf("PasskeyByCredentialID: %v ok=%v", err, ok)
	}
	if got.Name != "Handy" || got.RPID != "bombvault.example.com" || !got.BackedUp {
		t.Errorf("the row came back changed: %+v", got)
	}

	if err := r.TouchPasskey(saved.ID, 42, 1700000000); err != nil {
		t.Fatal(err)
	}
	got, _, _ = r.PasskeyByCredentialID([]byte{1, 2, 3})
	if got.SignCount != 42 || got.LastUsedAt != 1700000000 {
		t.Errorf("the use was not recorded: count=%d lastUsed=%d", got.SignCount, got.LastUsedAt)
	}

	if err := r.DeletePasskey(saved.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := r.PasskeyByCredentialID([]byte{1, 2, 3}); ok {
		t.Error("the credential is still there after being deleted")
	}
	if err := r.DeletePasskey(saved.ID); err != nil {
		t.Errorf("deleting an absent passkey should be a no-op, got %v", err)
	}
}

// TestAddPasskeyRejectsDuplicateCredential keeps one row per credential. Two
// would make the clone check compare the counter against whichever row the
// lookup found first.
func TestAddPasskeyRejectsDuplicateCredential(t *testing.T) {
	r := newPasskeyRepo(t)
	p := Passkey{Name: "A", CredentialID: []byte{5}, PublicKey: []byte{1}, RPID: "a.example.com"}
	if _, err := r.AddPasskey(p); err != nil {
		t.Fatal(err)
	}
	p.Name = "B"
	if _, err := r.AddPasskey(p); !errors.Is(err, ErrPasskeyExists) {
		t.Errorf("the second registration of one credential gave %v, want ErrPasskeyExists", err)
	}
}

// TestPasskeysForRPFiltersByAddress expects only the keys bound to the login's
// own address, because the browser refuses a credential whose relying-party ID
// does not match the page.
func TestPasskeysForRPFiltersByAddress(t *testing.T) {
	r := newPasskeyRepo(t)
	if _, err := r.AddPasskey(Passkey{Name: "Proxy", CredentialID: []byte{1}, PublicKey: []byte{1}, RPID: "bv.example.com"}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.AddPasskey(Passkey{Name: "Tunnel", CredentialID: []byte{2}, PublicKey: []byte{1}, RPID: "localhost"}); err != nil {
		t.Fatal(err)
	}

	here, err := r.PasskeysForRP("bv.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(here) != 1 || here[0].Name != "Proxy" {
		t.Errorf("PasskeysForRP returned %d rows (%v), want only the one bound to that address", len(here), here)
	}
	// The full list keeps both, so the interface can show keys bound to
	// another address.
	all, err := r.ListPasskeys()
	if err != nil || len(all) != 2 {
		t.Errorf("ListPasskeys returned %d rows, want both", len(all))
	}
}
