// Package selfrestore applies a staged restore of BombVault's own /config at
// boot, before the settings database is opened. The running process holds the
// SQLite file open, so the API layer restores into a staging directory and
// writes a marker, and cmd/bombvault calls ApplyPending before store.Open to
// swap the files into place.
//
// The package imports nothing from internal/, so both cmd/bombvault and
// internal/api can use it without an import cycle.
package selfrestore

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // "sqlite" driver for validSQLite
)

const stagingDirName = ".restore-staging"
const markerName = ".restore-pending"
const appliedMarkerName = ".restore-applied"

// StagingRoot is the directory a staged config restore is restic-restored into.
func StagingRoot(dataDir string) string { return filepath.Join(dataDir, stagingDirName) }

// MarkerPath is the file whose presence tells the next boot that a staged
// config restore is waiting to be applied.
func MarkerPath(dataDir string) string { return filepath.Join(dataDir, markerName) }

// AppliedMarkerPath is the file whose presence says that a restored database
// is in place and the boot has not finished what a restore asks of it yet.
// ApplyPending writes it; whoever finishes that work removes it.
func AppliedMarkerPath(dataDir string) string { return filepath.Join(dataDir, appliedMarkerName) }

// RestoredSnapshotDir is where restic recreates the snapshot subtree under the
// staging root. The config backup source is <dataDir>/.snapshot and restic
// restores absolute paths beneath --target, so for dataDir "/config" this is
// "/config/.restore-staging/config/.snapshot". ApplyPending and the API's
// RestoreConfig both take the path from here so they agree on it.
func RestoredSnapshotDir(dataDir string) string {
	return filepath.Join(StagingRoot(dataDir), dataDir, ".snapshot")
}

// WriteMarker records that a staged config restore is pending, to be applied by
// ApplyPending on the next boot.
func WriteMarker(dataDir string) error {
	return os.WriteFile(MarkerPath(dataDir), []byte("pending"), 0o600)
}

// ApplyPending swaps a staged config restore into place if the marker is
// present and reports whether it did. Call it before store.Open. A missing or
// invalid staged database leaves the live one untouched, moves the staging
// directory aside to <root>.bad and clears the marker, so boot cannot loop on a
// broken restore.
func ApplyPending(dataDir string) (bool, error) {
	if _, err := os.Stat(MarkerPath(dataDir)); os.IsNotExist(err) {
		return false, nil
	}
	staged := RestoredSnapshotDir(dataDir)
	stagedDB := filepath.Join(staged, "bombvault.sqlite")
	if !validSQLite(stagedDB) {
		// Remove an older .bad first, or the rename fails and leaves the bad
		// staging where the next boot would find it.
		_ = os.RemoveAll(StagingRoot(dataDir) + ".bad")
		_ = os.Rename(StagingRoot(dataDir), StagingRoot(dataDir)+".bad")
		_ = os.Remove(MarkerPath(dataDir))
		return false, fmt.Errorf("selfrestore: staged config DB missing/invalid at %q; kept live DB", stagedDB)
	}
	// The database goes last. A crash in between leaves a new rclone.conf or
	// ssh directory next to the old database, which is harmless.
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
	// SQLite would replay a leftover WAL of the old database into the restored one.
	_ = os.Remove(live + "-wal")
	_ = os.Remove(live + "-shm")
	// Written before the swap, so that a boot dying right after it still leaves
	// the next one knowing where the database came from.
	if err := os.WriteFile(AppliedMarkerPath(dataDir), []byte("applied"), 0o600); err != nil {
		return false, fmt.Errorf("selfrestore: mark the restore as applied: %w", err)
	}
	if err := replace(stagedDB, live); err != nil {
		_ = os.Remove(AppliedMarkerPath(dataDir))
		return false, err
	}
	_ = os.RemoveAll(StagingRoot(dataDir))
	// A <root>.bad from an earlier failed restore holds a plaintext rclone.conf
	// and the ssh private key.
	_ = os.RemoveAll(StagingRoot(dataDir) + ".bad")
	_ = os.Remove(MarkerPath(dataDir))
	return true, nil
}

// validSQLite reports whether path is an intact SQLite database. It runs
// PRAGMA quick_check instead of reading the header, because a truncated file
// with a valid header still opens and must not replace the live database.
func validSQLite(path string) bool {
	if !fileExists(path) {
		return false
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return false
	}
	defer func() { _ = db.Close() }()
	var res string
	if err := db.QueryRow("PRAGMA quick_check(1)").Scan(&res); err != nil {
		return false
	}
	return res == "ok"
}

// replace moves src onto dst. Both live on the /config mount, so on POSIX the
// first rename replaces dst atomically and leaves it intact if it fails.
// Windows refuses to rename onto an existing file, so on error it removes dst
// and renames again.
func replace(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
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
