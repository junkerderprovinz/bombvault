package api

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The empty-backup guard tells apart a container whose folders have gone
// missing (refuse, or an empty backup would look successful and overwrite the
// stored path list) and one the user made stateless by deselecting every
// folder (proceed and capture the definition only). The store cannot separate
// them, because a cleared selection is stored as an empty SelectedPaths just
// like one that was never set, so the guard checks the disk for the paths the
// last backup captured.

func guardService(t *testing.T) (*Service, *store.Repo) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	return &Service{store: st}, st
}

// seedCaptured records a target whose last backup captured the given paths.
func seedCaptured(t *testing.T, st *store.Repo, name string, captured ...string) {
	t.Helper()
	if _, err := st.UpsertTarget(store.Target{ContainerName: name, AppdataPaths: captured}); err != nil {
		t.Fatal(err)
	}
}

// existingDir creates a directory, so onlyExistingPaths reports it as present.
func existingDir(t *testing.T, name string) string {
	t.Helper()
	p := filepath.ToSlash(filepath.Join(t.TempDir(), name))
	if err := os.MkdirAll(p, 0o750); err != nil {
		t.Fatal(err)
	}
	return p
}

// The captured folder is still on disk and the user deselected it, so the
// backup proceeds with the definition only.
func TestEmptyBackupAllowedAfterDeselectingEveryFolder(t *testing.T) {
	s, st := guardService(t)
	stillThere := existingDir(t, "s3store")
	seedCaptured(t, st, "myapp", stillThere)

	if err := st.SetBackupPaths("myapp", []string{}); err != nil {
		t.Fatal(err)
	}

	if s.emptyBackupIsUnreachable("myapp", nil) {
		t.Fatal("refused a backup after the user deselected every folder, with the folder still on disk")
	}
}

// A captured folder missing from disk is what an unmounted share looks like.
func TestEmptyBackupRefusedWhenCapturedDataVanishes(t *testing.T) {
	s, st := guardService(t)
	gone := existingDir(t, "appdata")
	seedCaptured(t, st, "myapp", gone)
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}

	if !s.emptyBackupIsUnreachable("myapp", nil) {
		t.Fatal("accepted an empty backup after the captured data disappeared from disk")
	}
}

func TestEmptyBackupRefusedWhenSelectedPathsVanish(t *testing.T) {
	s, st := guardService(t)
	gone := existingDir(t, "appdata")
	seedCaptured(t, st, "myapp", gone)
	if err := st.SetBackupPaths("myapp", []string{gone}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}

	if !s.emptyBackupIsUnreachable("myapp", nil) {
		t.Fatal("accepted an empty backup while the user's selected folders were missing")
	}
}

// A container without a stored target is on its first backup.
func TestEmptyBackupAllowedForAFirstRun(t *testing.T) {
	s, _ := guardService(t)

	if s.emptyBackupIsUnreachable("fresh", nil) {
		t.Fatal("refused the first backup of a container that has never been backed up")
	}
}

// A stateless container has a target row from earlier definition-only runs but
// no captured path whose absence could mean anything.
func TestEmptyBackupAllowedWhenNothingWasEverCaptured(t *testing.T) {
	s, st := guardService(t)
	seedCaptured(t, st, "stateless")

	if s.emptyBackupIsUnreachable("stateless", nil) {
		t.Fatal("refused a stateless container that never captured any data")
	}
}

func TestNonEmptyBackupNeverRefused(t *testing.T) {
	s, st := guardService(t)
	gone := existingDir(t, "appdata")
	seedCaptured(t, st, "myapp", gone)
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}

	if s.emptyBackupIsUnreachable("myapp", []string{"/host/user/appdata/myapp"}) {
		t.Fatal("refused a backup that had paths to back up")
	}
}
