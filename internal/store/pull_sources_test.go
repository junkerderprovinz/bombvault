package store_test

import (
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestPullSourceEmptyLocationRejected expects Create and Update to refuse a
// blank location without writing anything.
func TestPullSourceEmptyLocationRejected(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	for _, loc := range []string{"", "   "} {
		if _, err := r.CreatePullSource(store.PullSource{Name: "Bad", Repo: loc}); !errors.Is(err, store.ErrEmptyPullRepo) {
			t.Fatalf("CreatePullSource(repo=%q) err = %v, want ErrEmptyPullRepo", loc, err)
		}
	}
	if err := r.UpdatePullSource(store.PullSource{ID: "x", Repo: ""}); !errors.Is(err, store.ErrEmptyPullRepo) {
		t.Fatalf("UpdatePullSource(repo=\"\") err = %v, want ErrEmptyPullRepo", err)
	}
	all, err := r.ListPullSources()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Fatalf("a rejected create must write nothing, got %d rows", len(all))
	}
}

// TestPullSourceCRUD also checks that the source instance's APP_KEY is stored
// as ciphertext that only this instance's key can decrypt: it belongs to
// another machine.
func TestPullSourceCRUD(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	const appKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	const foreignKey = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	enc, err := secret.Encrypt(appKey, []byte(foreignKey))
	if err != nil {
		t.Fatal(err)
	}

	made, err := r.CreatePullSource(store.PullSource{
		Name:          "Tower next door",
		Repo:          "rest:http://192.168.1.9:8000/containers",
		AppKeyEnc:     enc,
		Domain:        "containers",
		Cadence:       "daily 04:00",
		LimitDownload: 2048,
		Enabled:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if made.ID == "" || made.CreatedAt == 0 {
		t.Fatalf("Create must assign an id and stamp a creation time, got %+v", made)
	}

	got, ok, err := r.GetPullSource(made.ID)
	if err != nil || !ok {
		t.Fatalf("GetPullSource: ok=%v err=%v", ok, err)
	}
	if got.LastPullOK.Valid {
		t.Fatalf("a source that has never been pulled must report NULL, not a verdict: %+v", got.LastPullOK)
	}
	if strings.Contains(string(got.AppKeyEnc), foreignKey) {
		t.Fatal("the foreign APP_KEY is stored in the clear.\n" +
			"It belongs to another machine, so this is the one secret in the app that is not ours to lose.")
	}
	back, err := secret.Decrypt(appKey, got.AppKeyEnc)
	if err != nil || string(back) != foreignKey {
		t.Fatalf("the stored ciphertext must decrypt back to the key that was entered: %q err=%v", back, err)
	}

	// Writing a verdict leaves the configuration columns alone.
	if err := r.UpdatePullSourceResult(made.ID, 1700000000, sql.NullBool{Bool: true, Valid: true}, "", 7); err != nil {
		t.Fatal(err)
	}
	got, _, err = r.GetPullSource(made.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.LastPullOK.Valid || !got.LastPullOK.Bool || got.SnapshotsPulled != 7 || got.LastPullAt != 1700000000 {
		t.Fatalf("the verdict did not land: %+v", got)
	}
	if got.Name != "Tower next door" || got.Cadence != "daily 04:00" || got.LimitDownload != 2048 {
		t.Fatalf("writing a verdict must leave the configuration alone: %+v", got)
	}

	got.Name = "Renamed"
	got.Enabled = false
	if err := r.UpdatePullSource(got); err != nil {
		t.Fatal(err)
	}
	all, err := r.ListPullSources()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].Name != "Renamed" || all[0].Enabled {
		t.Fatalf("update did not round-trip: %+v", all)
	}

	if err := r.DeletePullSource(made.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := r.GetPullSource(made.ID); ok {
		t.Fatal("the row survived its own delete")
	}
	// A retried delete must not fail.
	if err := r.DeletePullSource(made.ID); err != nil {
		t.Fatalf("deleting a missing row must be a no-op, got %v", err)
	}
}
