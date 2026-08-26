package store_test

// MutateSettings — the only safe way to change part of the settings row.
//
// The row is written as ONE full-row UPDATE. So the natural-looking pairing
// (GetSettings → change a field → UpdateSettings) is not a patch: it re-writes
// every column from a snapshot, and whatever another writer stored in between
// is reverted across the whole row while the losing writer is told its save
// succeeded. That is the shape that let a minutes-long encryption probe undo a
// user's just-saved backup paths (internal/api/encryption_detect.go).
//
// These tests pin the two properties that make MutateSettings the fix rather
// than a nicer spelling of the same bug: the mutation sees the CURRENT row, and
// concurrent mutations cannot lose each other's writes.

import (
	"fmt"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func newSettingsRepo(t *testing.T) *store.Repo {
	t.Helper()
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return store.New(db)
}

// TestMutateSettingsChangesOnlyWhatItSets is the core contract: a mutation that
// touches one field leaves every other column exactly as it was — including a
// column written by SOMEONE ELSE after the mutating caller last read the row.
func TestMutateSettingsChangesOnlyWhatItSets(t *testing.T) {
	r := newSettingsRepo(t)

	// A caller reads the row here (as DetectEncryption does before its probe).
	stale, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	// …and someone else saves unrelated settings in the meantime.
	if _, err := r.MutateSettings(func(s *store.Settings) error {
		s.ContainersPath = "user/backups/containers-NEW"
		s.InstanceName = "saved-in-between"
		s.AuthPasswordHash = "hash-set-in-between"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// The first caller now applies its own single-field decision. It must NOT
	// carry its stale snapshot into the write.
	if stale.EncryptionEnabled == false {
		t.Fatal("precondition: the migration default is encryption on")
	}
	after, err := r.MutateSettings(func(s *store.Settings) error {
		s.EncryptionEnabled = false
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if after.EncryptionEnabled {
		t.Fatal("the mutation's own field was not applied")
	}
	if after.ContainersPath != "user/backups/containers-NEW" {
		t.Fatalf("ContainersPath = %q — the in-between save was reverted", after.ContainersPath)
	}
	if after.InstanceName != "saved-in-between" {
		t.Fatalf("InstanceName = %q — the in-between save was reverted", after.InstanceName)
	}
	if after.AuthPasswordHash != "hash-set-in-between" {
		t.Fatalf("AuthPasswordHash = %q — the in-between save was reverted", after.AuthPasswordHash)
	}

	// …and the returned value really is what is stored.
	stored, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if stored != after {
		t.Fatalf("MutateSettings returned %+v but the row holds %+v", after, stored)
	}
}

// TestMutateSettingsLosesNoConcurrentUpdate is the lost-update proof. Four
// goroutines each bump the same counter 50 times; every bump must survive.
// Read-modify-write without serialization drops most of them.
func TestMutateSettingsLosesNoConcurrentUpdate(t *testing.T) {
	r := newSettingsRepo(t)

	const goroutines, bumps = 4, 50
	var wg sync.WaitGroup
	errs := make(chan error, goroutines*bumps)
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < bumps; i++ {
				if _, err := r.MutateSettings(func(s *store.Settings) error {
					s.RetentionKeepLast++
					return nil
				}); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("MutateSettings: %v", err)
	}

	got, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if want := goroutines * bumps; got.RetentionKeepLast != want {
		t.Fatalf("RetentionKeepLast = %d, want %d — %d update(s) were lost", got.RetentionKeepLast, want, want-got.RetentionKeepLast)
	}
}

// TestMutateSettingsWritesNothingOnError: a mutation that fails must leave the
// row untouched, so a validation failure halfway through a multi-field edit
// cannot persist a half-applied state.
func TestMutateSettingsWritesNothingOnError(t *testing.T) {
	r := newSettingsRepo(t)
	before, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	boom := fmt.Errorf("rejected")
	if _, err := r.MutateSettings(func(s *store.Settings) error {
		s.ContainersPath = "user/should-never-be-stored"
		s.InstanceName = "nor-this"
		return boom
	}); err == nil {
		t.Fatal("expected the mutation's error to be returned")
	}

	after, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("a failed mutation wrote to the row: %+v", after)
	}
}
