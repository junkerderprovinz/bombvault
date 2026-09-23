package store_test

// UpdateSettings rewrites the whole settings row, so a read-modify-write around
// it reverts whatever another writer stored in between. MutateSettings prevents
// that as long as the mutation sees the current row and concurrent mutations
// keep each other's writes.

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

// TestMutateSettingsChangesOnlyWhatItSets expects a mutation of one field to
// leave every other column as it is, including one another writer changed after
// the caller last read the row.
func TestMutateSettingsChangesOnlyWhatItSets(t *testing.T) {
	r := newSettingsRepo(t)

	// A caller reads the row, as DetectEncryption does before its probe.
	stale, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	// Someone else saves unrelated settings in the meantime.
	if _, err := r.MutateSettings(func(s *store.Settings) error {
		s.ContainersPath = "user/backups/containers-NEW"
		s.InstanceName = "saved-in-between"
		s.AuthPasswordHash = "hash-set-in-between"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// Then the first caller changes one field; its stale snapshot must not be
	// written back.
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
		t.Fatalf("ContainersPath = %q; the in-between save was reverted", after.ContainersPath)
	}
	if after.InstanceName != "saved-in-between" {
		t.Fatalf("InstanceName = %q; the in-between save was reverted", after.InstanceName)
	}
	if after.AuthPasswordHash != "hash-set-in-between" {
		t.Fatalf("AuthPasswordHash = %q; the in-between save was reverted", after.AuthPasswordHash)
	}

	stored, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if stored != after {
		t.Fatalf("MutateSettings returned %+v but the row holds %+v", after, stored)
	}
}

// TestMutateSettingsLosesNoConcurrentUpdate has four goroutines bump the same
// counter 50 times each, and every bump must survive. An unserialized
// read-modify-write drops most of them.
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
		t.Fatalf("RetentionKeepLast = %d, want %d (%d update(s) lost)", got.RetentionKeepLast, want, want-got.RetentionKeepLast)
	}
}

// TestMutateSettingsWritesNothingOnError expects a failing mutation to leave
// the row untouched, so a validation error halfway through a multi-field edit
// stores nothing.
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
