package api

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The empty-backup guard has to tell two very different situations apart, and
// before #181 it could not: a container whose folders have gone missing (refuse,
// or an empty backup would look successful and overwrite the stored path list)
// versus one the user has deliberately made stateless by deselecting every
// folder (proceed, and capture the definition only).
//
// manilx hit the second: after backing up a folder once and then removing the
// selection, every further backup was refused, and the only apparent way out was
// to re-select the folder that had just been removed on purpose. His container,
// as he put it, "never has an appdata folder", so nothing else was detected
// either and the result stayed empty for good.
//
// What the store records cannot separate the two: clearing the selection stores
// an EMPTY SelectedPaths, which is also what a container that never had one
// carries. So the guard asks the disk about the paths the last backup captured.

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

// seedCaptured records a target whose last backup captured captured[0..n], which
// is what makes the guard eligible to fire at all.
func seedCaptured(t *testing.T, st *store.Repo, name string, captured ...string) {
	t.Helper()
	if _, err := st.UpsertTarget(store.Target{ContainerName: name, AppdataPaths: captured}); err != nil {
		t.Fatal(err)
	}
}

// existingDir returns a directory that is really there, so onlyExistingPaths
// reports it as present rather than gone.
func existingDir(t *testing.T, name string) string {
	t.Helper()
	p := filepath.ToSlash(filepath.Join(t.TempDir(), name))
	if err := os.MkdirAll(p, 0o750); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestEmptyBackupAllowedAfterDeselectingEveryFolder is #181 itself. The folder
// the earlier backup captured is still sitting on disk, untouched: nothing is
// unreachable, the user simply does not want it backed up any more, so the
// backup must proceed and record the definition only.
func TestEmptyBackupAllowedAfterDeselectingEveryFolder(t *testing.T) {
	s, st := guardService(t)
	stillThere := existingDir(t, "s3store")
	seedCaptured(t, st, "myapp", stillThere)

	// SelectedPaths is empty: the user cleared the selection. Indistinguishable
	// in the store from a container that never had one, which is why the guard
	// cannot answer from the store alone.
	if err := st.SetBackupPaths("myapp", []string{}); err != nil {
		t.Fatal(err)
	}

	if s.emptyBackupIsUnreachable("myapp", nil) {
		t.Fatal("refused a backup after the user deselected every folder, with the folder still on disk — this is #181")
	}
}

// TestEmptyBackupRefusedWhenCapturedDataVanishes pins the case the guard exists
// for, so the #181 fix cannot be mistaken for switching it off: the folder the
// last backup captured is no longer on disk, which is what an unmounted share
// looks like.
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

// TestEmptyBackupRefusedWhenSelectedPathsVanish covers the same fault with a
// selection standing: the user still wants those folders, and they are gone.
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

// TestEmptyBackupAllowedForAFirstRun keeps the original exemption: a container
// with no stored target at all is a first backup and is never refused.
func TestEmptyBackupAllowedForAFirstRun(t *testing.T) {
	s, _ := guardService(t)

	if s.emptyBackupIsUnreachable("fresh", nil) {
		t.Fatal("refused the first backup of a container that has never been backed up")
	}
}

// TestEmptyBackupAllowedWhenNothingWasEverCaptured covers a stateless container
// that HAS a target row from earlier definition-only runs: there is no captured
// path whose absence could mean anything, so it must never be refused.
func TestEmptyBackupAllowedWhenNothingWasEverCaptured(t *testing.T) {
	s, st := guardService(t)
	seedCaptured(t, st, "stateless")

	if s.emptyBackupIsUnreachable("stateless", nil) {
		t.Fatal("refused a stateless container that never captured any data")
	}
}

// TestNonEmptyBackupNeverRefused states the obvious boundary explicitly: the
// guard is about EMPTY results only and must never interfere with a backup that
// actually found paths.
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
