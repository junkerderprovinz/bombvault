package selfrestore_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/junkerderprovinz/bombvault/internal/selfrestore"
)

// newDataDir returns a relative data dir inside a fresh temp working directory.
// RestoredSnapshotDir joins dataDir under the staging root, and on Windows an
// absolute dataDir would put a "C:" mid-path that MkdirAll rejects.
func newDataDir(t *testing.T) string {
	t.Helper()
	t.Chdir(t.TempDir())
	dataDir := "config"
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dataDir
}

// writeSQLiteMarker writes a minimal single-file SQLite DB at path holding one
// marker string, so a swap can be proven by reading the marker back.
func writeSQLiteMarker(t *testing.T, path, marker string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open %q: %v", path, err)
	}
	if _, err := db.Exec("CREATE TABLE t(v TEXT)"); err != nil {
		t.Fatalf("create table in %q: %v", path, err)
	}
	if _, err := db.Exec("INSERT INTO t(v) VALUES(?)", marker); err != nil {
		t.Fatalf("insert into %q: %v", path, err)
	}
	if err := db.Close(); err != nil { // close before any rename (Windows file lock)
		t.Fatalf("close %q: %v", path, err)
	}
}

func readSQLiteMarker(t *testing.T, path string) string {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open %q: %v", path, err)
	}
	defer func() { _ = db.Close() }()
	var v string
	if err := db.QueryRow("SELECT v FROM t LIMIT 1").Scan(&v); err != nil {
		t.Fatalf("read marker from %q: %v", path, err)
	}
	return v
}

func TestApplyPendingSwapsValidStaging(t *testing.T) {
	dataDir := newDataDir(t)

	live := filepath.Join(dataDir, "bombvault.sqlite")
	writeSQLiteMarker(t, live, "OLD")
	if err := os.WriteFile(live+"-wal", []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(live+"-shm", []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "rclone.conf"), []byte("OLDCONF"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "ssh", "id_ed25519"), []byte("OLDKEY"), 0o600); err != nil {
		t.Fatal(err)
	}

	staged := selfrestore.RestoredSnapshotDir(dataDir)
	if err := os.MkdirAll(filepath.Join(staged, "ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeSQLiteMarker(t, filepath.Join(staged, "bombvault.sqlite"), "NEW")
	if err := os.WriteFile(filepath.Join(staged, "rclone.conf"), []byte("NEWCONF"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staged, "ssh", "id_ed25519"), []byte("NEWKEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A <root>.bad left by an earlier failed restore holds a plaintext
	// rclone.conf and ssh key, so a successful apply has to remove it.
	badRoot := selfrestore.StagingRoot(dataDir) + ".bad"
	if err := os.MkdirAll(badRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(badRoot, "id_ed25519"), []byte("STALEKEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := selfrestore.WriteMarker(dataDir); err != nil {
		t.Fatal(err)
	}

	applied, err := selfrestore.ApplyPending(dataDir)
	if err != nil || !applied {
		t.Fatalf("ApplyPending: applied=%v err=%v", applied, err)
	}

	if got := readSQLiteMarker(t, live); got != "NEW" {
		t.Fatalf("live DB not replaced: marker=%q, want NEW", got)
	}
	if _, err := os.Stat(live + "-wal"); !os.IsNotExist(err) {
		t.Fatalf("stale -wal not removed: %v", err)
	}
	if _, err := os.Stat(live + "-shm"); !os.IsNotExist(err) {
		t.Fatalf("stale -shm not removed: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dataDir, "rclone.conf")); string(b) != "NEWCONF" { //nolint:gosec // G304: test-controlled path under the test's own temp dir
		t.Fatalf("rclone.conf not swapped: got %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(dataDir, "ssh", "id_ed25519")); string(b) != "NEWKEY" { //nolint:gosec // G304: test-controlled path under the test's own temp dir
		t.Fatalf("ssh key not swapped: got %q", b)
	}
	if _, err := os.Stat(selfrestore.MarkerPath(dataDir)); !os.IsNotExist(err) {
		t.Fatalf("marker not cleared: %v", err)
	}
	// The boot that applied the restore may die before it has dealt with the
	// restored database, so the next boot has to be able to tell.
	if _, err := os.Stat(selfrestore.AppliedMarkerPath(dataDir)); err != nil {
		t.Fatalf("the applied restore is not marked for the boot that follows: %v", err)
	}
	if _, err := os.Stat(selfrestore.StagingRoot(dataDir)); !os.IsNotExist(err) {
		t.Fatalf("staging root not removed: %v", err)
	}
	if _, err := os.Stat(selfrestore.StagingRoot(dataDir) + ".bad"); !os.IsNotExist(err) {
		t.Fatalf("stale <root>.bad secret copy not GC'd after successful apply: %v", err)
	}
}

// TestApplyPendingRejectsTruncatedDB stages a DB truncated behind an intact
// header. Such a file opens fine, so rejecting it proves validSQLite runs PRAGMA
// quick_check rather than only probing the header.
func TestApplyPendingRejectsTruncatedDB(t *testing.T) {
	dataDir := newDataDir(t)

	live := filepath.Join(dataDir, "bombvault.sqlite")
	writeSQLiteMarker(t, live, "OLD")

	staged := selfrestore.RestoredSnapshotDir(dataDir)
	if err := os.MkdirAll(staged, 0o700); err != nil {
		t.Fatal(err)
	}
	stagedDB := filepath.Join(staged, "bombvault.sqlite")
	writeSQLiteMarker(t, stagedDB, "NEW")
	if fi, err := os.Stat(stagedDB); err != nil {
		t.Fatal(err)
	} else if fi.Size() <= 200 {
		t.Fatalf("expected a multi-page DB to truncate; got only %d bytes", fi.Size())
	}
	if err := os.Truncate(stagedDB, 200); err != nil {
		t.Fatal(err)
	}
	if err := selfrestore.WriteMarker(dataDir); err != nil {
		t.Fatal(err)
	}

	applied, err := selfrestore.ApplyPending(dataDir)
	if applied {
		t.Fatal("must NOT apply a truncated (header-valid but incomplete) staged DB")
	}
	if err == nil {
		t.Fatal("expected an error describing the invalid staged DB")
	}
	if got := readSQLiteMarker(t, live); got != "OLD" {
		t.Fatalf("live DB was modified: marker=%q, want OLD", got)
	}
	if _, err := os.Stat(selfrestore.MarkerPath(dataDir)); !os.IsNotExist(err) {
		t.Fatalf("marker not cleared after truncated staging: %v", err)
	}
	if _, err := os.Stat(selfrestore.AppliedMarkerPath(dataDir)); !os.IsNotExist(err) {
		t.Fatalf("a restore that was not applied is marked as applied: %v", err)
	}
	if _, err := os.Stat(selfrestore.StagingRoot(dataDir) + ".bad"); err != nil {
		t.Fatalf("bad staging not preserved as .bad: %v", err)
	}
}

func TestApplyPendingNoMarkerIsNoop(t *testing.T) {
	dataDir := newDataDir(t)
	applied, err := selfrestore.ApplyPending(dataDir)
	if err != nil || applied {
		t.Fatalf("expected no-op, got applied=%v err=%v", applied, err)
	}
}

// TestApplyPendingInvalidStagingKeepsLive checks that a staged file that is not
// SQLite leaves the live DB alone and is moved aside to <root>.bad, so the next
// boot cannot loop on it.
func TestApplyPendingInvalidStagingKeepsLive(t *testing.T) {
	dataDir := newDataDir(t)

	live := filepath.Join(dataDir, "bombvault.sqlite")
	writeSQLiteMarker(t, live, "OLD")

	staged := selfrestore.RestoredSnapshotDir(dataDir)
	if err := os.MkdirAll(staged, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staged, "bombvault.sqlite"), []byte("this is not sqlite"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := selfrestore.WriteMarker(dataDir); err != nil {
		t.Fatal(err)
	}

	applied, err := selfrestore.ApplyPending(dataDir)
	if applied {
		t.Fatal("must NOT apply an invalid staged DB")
	}
	if err == nil {
		t.Fatal("expected an error describing the invalid staged DB")
	}
	if got := readSQLiteMarker(t, live); got != "OLD" {
		t.Fatalf("live DB was modified: marker=%q, want OLD", got)
	}
	if _, err := os.Stat(selfrestore.MarkerPath(dataDir)); !os.IsNotExist(err) {
		t.Fatalf("marker not cleared after invalid staging: %v", err)
	}
	if _, err := os.Stat(selfrestore.StagingRoot(dataDir)); !os.IsNotExist(err) {
		t.Fatalf("bad staging root not moved aside: %v", err)
	}
	if _, err := os.Stat(selfrestore.StagingRoot(dataDir) + ".bad"); err != nil {
		t.Fatalf("bad staging not preserved as .bad: %v", err)
	}
}
