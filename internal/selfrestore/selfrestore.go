// Package selfrestore applies a staged restore of BombVault's own /config on the
// next boot, BEFORE the settings DB is opened — the only safe moment to swap the
// database, since the running process otherwise holds it open (WAL). A config
// restore cannot overwrite the live SQLite file in place while the process has it
// open, so the API layer instead STAGES a restore into a staging dir + writes a
// marker, and cmd/bombvault calls ApplyPending at boot (before store.Open) to
// perform the file-level swap.
//
// The package is deliberately dependency-light: it imports only the standard
// library plus the modernc.org/sqlite driver (for a validity probe). It pulls in
// nothing from internal/, so both cmd/bombvault/main.go and internal/api can use
// it without an import cycle.
package selfrestore

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // register the "sqlite" driver for the validity probe
)

const stagingDirName = ".restore-staging"
const markerName = ".restore-pending"

// StagingRoot is the directory a staged config restore is restic-restored into.
func StagingRoot(dataDir string) string { return filepath.Join(dataDir, stagingDirName) }

// MarkerPath is the sentinel file whose presence tells the next boot that a
// staged config restore is waiting to be applied.
func MarkerPath(dataDir string) string { return filepath.Join(dataDir, markerName) }

// RestoredSnapshotDir is where restic recreates the staged snapshot subtree under
// the staging root. The config backup source is <dataDir>/.snapshot, so a restic
// restore with --target StagingRoot recreates that absolute subtree beneath the
// target. filepath.Join treats the absolute dataDir as trailing components, e.g.
// Join("/config/.restore-staging", "/config", ".snapshot") ==
// "/config/.restore-staging/config/.snapshot". The staging swap and the API-layer
// RestoreConfig MUST both derive this path from this single function so they agree.
func RestoredSnapshotDir(dataDir string) string {
	return filepath.Join(StagingRoot(dataDir), dataDir, ".snapshot")
}

// WriteMarker records that a staged config restore is pending, to be applied by
// ApplyPending on the next boot.
func WriteMarker(dataDir string) error {
	return os.WriteFile(MarkerPath(dataDir), []byte("pending"), 0o600)
}

// ApplyPending swaps a staged config restore into place if the marker is present.
// It NEVER runs while the DB is open (call it from main BEFORE store.Open). It is
// fail-safe: an invalid/absent staged DB leaves the live DB untouched and clears
// the pending state (moving bad staging aside to <root>.bad) so boot never loops
// re-applying a broken restore. Returns whether a restore was actually applied.
func ApplyPending(dataDir string) (bool, error) {
	if _, err := os.Stat(MarkerPath(dataDir)); os.IsNotExist(err) {
		return false, nil
	}
	staged := RestoredSnapshotDir(dataDir)
	stagedDB := filepath.Join(staged, "bombvault.sqlite")
	if !validSQLite(stagedDB) {
		// Bad or missing staged DB: don't touch the live DB; clear the pending
		// state and move the bad staging aside so the next boot can't loop on it.
		_ = os.Rename(StagingRoot(dataDir), StagingRoot(dataDir)+".bad")
		_ = os.Remove(MarkerPath(dataDir))
		return false, fmt.Errorf("selfrestore: staged config DB missing/invalid at %q; kept live DB", stagedDB)
	}
	// Swap the DB LAST; rclone.conf + ssh/ first (their staleness is harmless if we
	// crash between steps, whereas a swapped DB with a stale rclone/ssh is fine too).
	if src := filepath.Join(staged, "rclone.conf"); fileExists(src) {
		if err := replace(src, filepath.Join(dataDir, "rclone.conf")); err != nil {
			return false, err
		}
	}
	if src := filepath.Join(staged, "ssh"); dirExists(src) {
		_ = os.RemoveAll(filepath.Join(dataDir, "ssh"))
		if err := os.Rename(src, filepath.Join(dataDir, "ssh")); err != nil {
			return false, fmt.Errorf("selfrestore: move ssh into place: %w", err)
		}
	}
	live := filepath.Join(dataDir, "bombvault.sqlite")
	// Drop any stale WAL/SHM sidecars of the OLD DB before swapping in the new file;
	// leaving them would let SQLite fold a stale WAL into the restored database.
	_ = os.Remove(live + "-wal")
	_ = os.Remove(live + "-shm")
	if err := replace(stagedDB, live); err != nil {
		return false, err
	}
	_ = os.RemoveAll(StagingRoot(dataDir))
	_ = os.Remove(MarkerPath(dataDir))
	return true, nil
}

// validSQLite reports whether path is an openable SQLite database (a cheap
// PRAGMA read). A missing file, a non-SQLite file, or an unreadable one all
// return false, so ApplyPending refuses to swap in a corrupt restore.
func validSQLite(path string) bool {
	if !fileExists(path) {
		return false
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return false
	}
	defer func() { _ = db.Close() }()
	var n int
	return db.QueryRow("PRAGMA schema_version").Scan(&n) == nil
}

// replace atomically-ish moves src onto dst: it removes dst (if present) then
// renames src into its place. Both live on the same /config mount, so the rename
// is atomic on POSIX.
func replace(src, dst string) error {
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("selfrestore: remove %q: %w", dst, err)
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("selfrestore: move %q -> %q: %w", src, dst, err)
	}
	return nil
}

func fileExists(p string) bool { fi, err := os.Stat(p); return err == nil && !fi.IsDir() }
func dirExists(p string) bool  { fi, err := os.Stat(p); return err == nil && fi.IsDir() }
