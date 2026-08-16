package store_test

import (
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestFleetPeerEmptyURLRejected pins the empty-URL guard on both Create and
// Update: a fleet peer with a blank (or whitespace-only) URL addresses nowhere
// and is refused with ErrEmptyFleetPeer, writing nothing.
func TestFleetPeerEmptyURLRejected(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	for _, u := range []string{"", "   "} {
		if _, err := r.CreateFleetPeer(store.FleetPeer{Name: "Bad", URL: u}); !errors.Is(err, store.ErrEmptyFleetPeer) {
			t.Fatalf("CreateFleetPeer(url=%q) err = %v, want ErrEmptyFleetPeer", u, err)
		}
	}
	if err := r.UpdateFleetPeer(store.FleetPeer{ID: "x", URL: ""}); !errors.Is(err, store.ErrEmptyFleetPeer) {
		t.Fatalf("UpdateFleetPeer(url=\"\") err = %v, want ErrEmptyFleetPeer", err)
	}
	all, err := r.ListFleetPeers()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Fatalf("a rejected create must write nothing, got %d rows", len(all))
	}
}

// TestFleetPeerCRUD exercises Create/Get/List/Update/Delete and, crucially,
// that the PEER's fleet token round-trips as ENCRYPTED bytes: the store
// persists the ciphertext verbatim (never plaintext), and only a holder of
// this instance's app secret key can decrypt it back to the original token.
// LastPollOK is nullable and starts NULL (never polled).
func TestFleetPeerCRUD(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	appKey := strings.Repeat("ab", 32) // THIS instance's app secret key
	peerToken := "deadbeefcafef00d"    // the PEER's fleet_token (what we store)
	enc, err := secret.Encrypt(appKey, []byte(peerToken))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if strings.Contains(string(enc), peerToken) {
		t.Fatal("stored token_enc must not contain the plaintext token")
	}

	in := store.FleetPeer{
		Name:      "tower",
		URL:       "https://192.168.1.50:3443",
		TokenEnc:  enc,
		Enabled:   true,
		SortOrder: 0,
	}
	got, err := r.CreateFleetPeer(in)
	if err != nil {
		t.Fatalf("CreateFleetPeer: %v", err)
	}
	if got.ID == "" {
		t.Fatal("CreateFleetPeer did not assign an ID")
	}
	if got.CreatedAt == 0 {
		t.Fatal("CreateFleetPeer did not stamp CreatedAt")
	}

	back, ok, err := r.GetFleetPeer(got.ID)
	if err != nil || !ok {
		t.Fatalf("GetFleetPeer: ok=%v err=%v", ok, err)
	}
	if back.Name != "tower" || back.URL != in.URL || !back.Enabled {
		t.Fatalf("GetFleetPeer round-trip mismatch: %+v", back)
	}
	if back.LastPollOK.Valid {
		t.Fatalf("a freshly created peer must have LastPollOK unset (never polled), got %+v", back.LastPollOK)
	}
	dec, err := secret.Decrypt(appKey, back.TokenEnc)
	if err != nil || string(dec) != peerToken {
		t.Fatalf("stored token did not decrypt back to the original: dec=%q err=%v", dec, err)
	}

	all, err := r.ListFleetPeers()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].ID != got.ID {
		t.Fatalf("ListFleetPeers = %+v, want exactly the one created peer", all)
	}

	// Update: name/enabled change, token untouched when TokenEnc carries the
	// existing ciphertext forward (mirrors how buildFleetPeer folds an edit).
	back.Name = "tower (renamed)"
	back.Enabled = false
	if err := r.UpdateFleetPeer(back); err != nil {
		t.Fatalf("UpdateFleetPeer: %v", err)
	}
	updated, ok, err := r.GetFleetPeer(got.ID)
	if err != nil || !ok {
		t.Fatalf("GetFleetPeer after update: ok=%v err=%v", ok, err)
	}
	if updated.Name != "tower (renamed)" || updated.Enabled {
		t.Fatalf("UpdateFleetPeer did not persist: %+v", updated)
	}
	dec2, err := secret.Decrypt(appKey, updated.TokenEnc)
	if err != nil || string(dec2) != peerToken {
		t.Fatalf("token must survive an update that keeps the same ciphertext: dec=%q err=%v", dec2, err)
	}

	// UpdateFleetPeerPollResult writes ONLY the last-poll columns.
	if err := r.UpdateFleetPeerPollResult(got.ID, 1_700_000_000, sql.NullBool{Valid: true, Bool: true}, "", "tower-instance", "1.2.3", `[{"domain":"containers"}]`); err != nil {
		t.Fatalf("UpdateFleetPeerPollResult: %v", err)
	}
	polled, ok, err := r.GetFleetPeer(got.ID)
	if err != nil || !ok {
		t.Fatalf("GetFleetPeer after poll result: ok=%v err=%v", ok, err)
	}
	if polled.LastPollAt != 1_700_000_000 || !polled.LastPollOK.Valid || !polled.LastPollOK.Bool ||
		polled.LastPollInstanceName != "tower-instance" || polled.LastPollVersion != "1.2.3" ||
		polled.LastPollDomainsJSON != `[{"domain":"containers"}]` {
		t.Fatalf("UpdateFleetPeerPollResult round-trip mismatch: %+v", polled)
	}
	// Name/URL/enabled must be UNCHANGED by a poll-result write (leaving config alone).
	if polled.Name != "tower (renamed)" || polled.Enabled {
		t.Fatalf("UpdateFleetPeerPollResult must not touch config columns: %+v", polled)
	}

	if err := r.DeleteFleetPeer(got.ID); err != nil {
		t.Fatalf("DeleteFleetPeer: %v", err)
	}
	if _, ok, err := r.GetFleetPeer(got.ID); err != nil || ok {
		t.Fatalf("GetFleetPeer after delete: ok=%v err=%v, want ok=false", ok, err)
	}
	// Deleting a missing id is a harmless no-op.
	if err := r.DeleteFleetPeer("does-not-exist"); err != nil {
		t.Fatalf("DeleteFleetPeer(missing id): %v", err)
	}
}
