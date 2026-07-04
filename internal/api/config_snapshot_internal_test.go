package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestStageConfigSnapshot verifies stageConfigSnapshot builds a restic-ready
// staging dir: a VACUUM-INTO snapshot of the live DB (readable as an independent
// database) plus verbatim copies of rclone.conf and the ssh/ keypair. The store
// is opened on-disk under DataDir so VacuumInto has a real source file.
func TestStageConfigSnapshot(t *testing.T) {
	dataDir := t.TempDir()
	db, err := store.Open(filepath.Join(dataDir, "bombvault.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() }) // close before TempDir cleanup (Windows file lock)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	svc := &Service{
		cfg:   config.Config{AppKey: strings.Repeat("a", 64), DataDir: dataDir},
		store: store.New(db),
	}

	if err := os.WriteFile(filepath.Join(dataDir, "rclone.conf"), []byte("[r]\ntype = local\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "ssh", "id_ed25519"), []byte("key"), 0o600); err != nil {
		t.Fatal(err)
	}

	dir, err := svc.stageConfigSnapshot()
	if err != nil {
		t.Fatalf("stageConfigSnapshot: %v", err)
	}
	for _, p := range []string{"bombvault.sqlite", "rclone.conf", filepath.Join("ssh", "id_ed25519")} {
		if _, err := os.Stat(filepath.Join(dir, p)); err != nil {
			t.Fatalf("missing %s: %v", p, err)
		}
	}

	// The staged DB must open as a real, independent, readable SQLite database.
	snap, err := store.Open(filepath.Join(dir, "bombvault.sqlite"))
	if err != nil {
		t.Fatalf("open staged snapshot: %v", err)
	}
	t.Cleanup(func() { _ = snap.Close() }) // close before TempDir cleanup (Windows file lock)
	var n int
	if err := snap.QueryRow("SELECT count(*) FROM settings").Scan(&n); err != nil {
		t.Fatalf("staged snapshot is not a readable DB: %v", err)
	}
}
