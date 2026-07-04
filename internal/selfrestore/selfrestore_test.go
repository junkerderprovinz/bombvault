package selfrestore_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/junkerderprovinz/bombvault/internal/selfrestore"
)

// newDataDir returns a RELATIVE data dir inside a fresh temp working directory.
// A relative dir is used deliberately: RestoredSnapshotDir joins the (absolute)
// dataDir as trailing components under the staging root, which on Windows would
// embed a "C:" mid-path that MkdirAll rejects. A relative dataDir exercises the
// exact same RestoredSnapshotDir logic (and keeps the swap self-consistent) while
// staying creatable on every OS. Production runs on Linux where the absolute
// "/config" form joins cleanly.
func newDataDir(t *testing.T) string {
	t.Helper()
	t.Chdir(t.TempDir())
	dataDir := "config"
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dataDir
}

// writeSQLiteMarker writes a minimal, valid single-file SQLite DB at path holding
// one marker string, so a byte-level swap can be proven by reading the marker back.
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

// TestApplyPendingSwapsValidStaging: a valid staged DB (plus rclone.conf + ssh/)
// with the marker present is swapped into place — the live DB becomes the staged
// one, stale -wal/-shm are removed, rclone.conf/ssh/ are replaced, and both the
// marker and the staging root are cleared.
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
	// Pre-existing rclone.conf + ssh/ that must be overwritten by the staged ones.
	if err := os.WriteFile(filepath.Join(dataDir, "rclone.conf"), []byte("OLDCONF"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "ssh", "id_ed25519"), []byte("OLDKEY"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Staged restore at the deterministic restic path.
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
	if _, err := os.Stat(selfrestore.StagingRoot(dataDir)); !os.IsNotExist(err) {
		t.Fatalf("staging root not removed: %v", err)
	}
}

// TestApplyPendingRejectsTruncatedDB: a staged DB whose SQLite HEADER is intact but
// whose pages have been truncated away must be rejected — proving validSQLite runs a
// real integrity scan (PRAGMA quick_check), not a header-only probe. Such a file
// opens fine yet is not a usable database; swapping it over the live settings DB
// would destroy it. The live DB must be left untouched and the bad staging moved
// aside to <root>.bad.
func TestApplyPendingRejectsTruncatedDB(t *testing.T) {
	dataDir := newDataDir(t)

	live := filepath.Join(dataDir, "bombvault.sqlite")
	writeSQLiteMarker(t, live, "OLD")

	staged := selfrestore.RestoredSnapshotDir(dataDir)
	if err := os.MkdirAll(staged, 0o700); err != nil {
		t.Fatal(err)
	}
	stagedDB := filepath.Join(staged, "bombvault.sqlite")
	// Build a real, valid multi-page SQLite DB, then truncate it so the header
	// survives but the data pages are gone — a header-only check would wrongly pass.
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
	if _, err := os.Stat(selfrestore.StagingRoot(dataDir) + ".bad"); err != nil {
		t.Fatalf("bad staging not preserved as .bad: %v", err)
	}
}

// TestApplyPendingNoMarkerIsNoop: with no pending marker, ApplyPending does
// nothing and reports applied=false, err=nil (the ordinary boot path).
func TestApplyPendingNoMarkerIsNoop(t *testing.T) {
	dataDir := newDataDir(t)
	applied, err := selfrestore.ApplyPending(dataDir)
	if err != nil || applied {
		t.Fatalf("expected no-op, got applied=%v err=%v", applied, err)
	}
}

// TestApplyPendingInvalidStagingKeepsLive: a garbage (non-SQLite) staged DB must
// NOT be swapped in — the live DB is left untouched, the marker is cleared, and
// the bad staging is moved aside to <root>.bad so the next boot can't loop on it.
func TestApplyPendingInvalidStagingKeepsLive(t *testing.T) {
	dataDir := newDataDir(t)

	live := filepath.Join(dataDir, "bombvault.sqlite")
	writeSQLiteMarker(t, live, "OLD")

	staged := selfrestore.RestoredSnapshotDir(dataDir)
	if err := os.MkdirAll(staged, 0o700); err != nil {
		t.Fatal(err)
	}
	// Not a SQLite database — validSQLite must reject it.
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
