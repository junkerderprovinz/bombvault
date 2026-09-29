package api

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// TestCleanupDrillSandboxMarkerGuard: cleanupDrillSandbox removes only a
// directory that carries the drill marker, so it cannot RemoveAll a path it did
// not create.
func TestCleanupDrillSandboxMarkerGuard(t *testing.T) {
	t.Run("refuses an unmarked dir", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "not-a-drill")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		// A real file inside, to prove nothing is deleted.
		payload := filepath.Join(dir, "important.txt")
		if err := os.WriteFile(payload, []byte("keep me"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := cleanupDrillSandbox(dir); err == nil {
			t.Fatal("cleanup must refuse a dir without the .bombvault-drill marker")
		}
		if _, err := os.Stat(payload); err != nil {
			t.Fatalf("an unmarked dir must NOT be removed, stat=%v", err)
		}
	})
	t.Run("removes a marked sandbox", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "bombvault-drill-containers-123")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, drillMarkerName), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := cleanupDrillSandbox(dir); err != nil {
			t.Fatalf("cleanup of a marked sandbox must succeed: %v", err)
		}
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("a marked sandbox must be removed, stat=%v", err)
		}
	})
}

func TestTheContainersDrillRestoresTheDumpOfAContainerWithOnlyDumps(t *testing.T) {
	s, _, settings := zfsDomainFixture(t)
	settings.DRDrillTarget = "nextcloud-db"
	eng := s.engine.(*zfsFakeEngine)
	eng.snaps = []restic.Snapshot{
		{ID: "other", Time: "2026-09-28T03:00:00Z", Tags: []string{"container:plex"}},
		{ID: "dump1", Time: "2026-09-27T03:00:00Z", Tags: []string{"dbdump:nextcloud-db", "p1"}},
		{ID: "dump2", Time: "2026-09-28T03:00:00Z", Tags: []string{"dbdump:nextcloud-db", "p1"}},
	}
	id, err := s.pickDRSnapshot(context.Background(), "containers", settings, "repo", restic.Mode{})
	if err != nil || id != "dump2" {
		t.Fatalf("pickDRSnapshot = %q, %v, want the newest dump of the container", id, err)
	}

	eng.snaps = append(eng.snaps, restic.Snapshot{ID: "volumes", Time: "2026-09-26T03:00:00Z", Tags: []string{"container:nextcloud-db"}})
	if id, err = s.pickDRSnapshot(context.Background(), "containers", settings, "repo", restic.Mode{}); err != nil || id != "volumes" {
		t.Fatalf("pickDRSnapshot = %q, %v, want the container's own snapshot while it has one", id, err)
	}
}
