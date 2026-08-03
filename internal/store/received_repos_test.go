package store_test

import (
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestReceivedRepoEmptyLocationRejected pins the empty-location guard on both
// Create and Update: a received repo with a blank (or whitespace-only) location
// addresses nowhere and is refused with ErrEmptyReceivedRepo, writing nothing.
func TestReceivedRepoEmptyLocationRejected(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	for _, loc := range []string{"", "   "} {
		if _, err := r.CreateReceivedRepo(store.ReceivedRepo{Name: "Bad", Repo: loc}); !errors.Is(err, store.ErrEmptyReceivedRepo) {
			t.Fatalf("CreateReceivedRepo(repo=%q) err = %v, want ErrEmptyReceivedRepo", loc, err)
		}
	}
	if err := r.UpdateReceivedRepo(store.ReceivedRepo{ID: "x", Repo: ""}); !errors.Is(err, store.ErrEmptyReceivedRepo) {
		t.Fatalf("UpdateReceivedRepo(repo=\"\") err = %v, want ErrEmptyReceivedRepo", err)
	}
	all, err := r.ListReceivedRepos()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Fatalf("a rejected create must write nothing, got %d rows", len(all))
	}
}

// TestReceivedRepoCRUD exercises Create/Get/List/Update/Delete and, crucially,
// that the SENDING APP_KEY round-trips as ENCRYPTED bytes: the store persists the
// ciphertext verbatim (never plaintext), and only a holder of the app secret key
// can decrypt it back to the original 64-hex key. last_check_ok is nullable and
// starts NULL (never checked).
func TestReceivedRepoCRUD(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	appKey := strings.Repeat("ab", 32)     // THIS instance's app secret key
	sendingKey := strings.Repeat("cd", 32) // the SENDING instance's APP_KEY (what we store)
	enc, err := secret.Encrypt(appKey, []byte(sendingKey))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if strings.Contains(string(enc), sendingKey) {
		t.Fatal("stored app_key_enc must not contain the plaintext key")
	}

	in := store.ReceivedRepo{
		Name:            "Off-site A",
		Repo:            "rest:https://box:8000/vault",
		AppKeyEnc:       enc,
		DeadManHours:    26,
		CheckCadence:    "weekly Sun 05:00",
		ReadDataPercent: 5,
		Enabled:         true,
		SortOrder:       0,
	}
	got, err := r.CreateReceivedRepo(in)
	if err != nil {
		t.Fatalf("CreateReceivedRepo: %v", err)
	}
	if got.ID == "" {
		t.Fatal("CreateReceivedRepo did not assign an ID")
	}
	if got.CreatedAt == 0 {
		t.Fatal("CreateReceivedRepo did not stamp CreatedAt")
	}

	back, ok, err := r.GetReceivedRepo(got.ID)
	if err != nil || !ok {
		t.Fatalf("GetReceivedRepo: ok=%v err=%v", ok, err)
	}
	if back.Name != "Off-site A" || back.Repo != in.Repo || back.DeadManHours != 26 ||
		back.CheckCadence != "weekly Sun 05:00" || back.ReadDataPercent != 5 || !back.Enabled {
		t.Fatalf("GetReceivedRepo round-trip mismatch: %+v", back)
	}
	if back.LastCheckOK.Valid {
		t.Fatalf("last_check_ok must start NULL (never checked), got %+v", back.LastCheckOK)
	}
	if back.LastCheckAt != 0 || back.LastCheckReadData {
		t.Fatalf("check state should be zero initially: %+v", back)
	}

	// The encrypted key round-trips: decrypt with the app secret key yields the
	// original sending key exactly.
	dec, err := secret.Decrypt(appKey, back.AppKeyEnc)
	if err != nil {
		t.Fatalf("Decrypt stored app_key_enc: %v", err)
	}
	if string(dec) != sendingKey {
		t.Fatalf("app_key round-trip mismatch: got %q want %q", dec, sendingKey)
	}
	// A wrong app secret key must NOT decrypt it.
	if _, err := secret.Decrypt(strings.Repeat("ff", 32), back.AppKeyEnc); err == nil {
		t.Fatal("a wrong app secret key must fail to decrypt the stored key")
	}

	// Update in place: record a successful check + toggle read-data.
	back.LastCheckAt = 1234
	back.LastCheckOK = sql.NullBool{Bool: true, Valid: true}
	back.LastCheckReadData = true
	back.LastCheckError = ""
	back.Enabled = false
	if err := r.UpdateReceivedRepo(back); err != nil {
		t.Fatalf("UpdateReceivedRepo: %v", err)
	}
	upd, _, err := r.GetReceivedRepo(got.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !upd.LastCheckOK.Valid || !upd.LastCheckOK.Bool || upd.LastCheckAt != 1234 ||
		!upd.LastCheckReadData || upd.Enabled {
		t.Fatalf("in-place update failed: %+v", upd)
	}

	// List ordering by sort_order then created_at.
	if _, err := r.CreateReceivedRepo(store.ReceivedRepo{Name: "B", Repo: "s3:bucket/b", Enabled: true, SortOrder: 5}); err != nil {
		t.Fatal(err)
	}
	all, err := r.ListReceivedRepos()
	if err != nil {
		t.Fatalf("ListReceivedRepos: %v", err)
	}
	if len(all) != 2 || all[0].ID != got.ID || all[1].Name != "B" {
		t.Fatalf("ListReceivedRepos order/len wrong: %+v", all)
	}

	// Delete removes it; a second delete is a no-op; Get on a missing id is (false, nil).
	if err := r.DeleteReceivedRepo(got.ID); err != nil {
		t.Fatalf("DeleteReceivedRepo: %v", err)
	}
	if _, ok, _ := r.GetReceivedRepo(got.ID); ok {
		t.Fatal("received repo still present after delete")
	}
	if err := r.DeleteReceivedRepo(got.ID); err != nil {
		t.Fatalf("DeleteReceivedRepo (missing) should be a no-op: %v", err)
	}
	if _, ok, err := r.GetReceivedRepo("does-not-exist"); ok || err != nil {
		t.Fatalf("GetReceivedRepo(missing) = ok:%v err:%v", ok, err)
	}
}

// TestReceiverEnabledPersists pins the receiverEnabled settings flag: it defaults
// to false (opt-in, like the domain tabs) and survives an update round-trip.
func TestReceiverEnabledPersists(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	s, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.ReceiverEnabled {
		t.Fatal("default receiver_enabled must be false")
	}
	s.ReceiverEnabled = true
	if err := r.UpdateSettings(s); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	back, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !back.ReceiverEnabled {
		t.Fatal("receiver_enabled did not persist")
	}
}
