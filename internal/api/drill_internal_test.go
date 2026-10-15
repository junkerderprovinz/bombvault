package api

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
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

// drillTargetFixture records one successful backup run per container, the
// first name finishing last, and lists snaps as the repo's snapshots.
func drillTargetFixture(t *testing.T, snaps []restic.Snapshot, newestFirst ...string) (*Service, store.Settings) {
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
	for i, name := range newestFirst {
		tg, err := st.UpsertTarget(store.Target{ContainerName: name})
		if err != nil {
			t.Fatal(err)
		}
		recordBackup(t, st, tg.ID, "")
		if _, err := db.Exec(`UPDATE runs SET finished_at = ? WHERE target_id = ?`, 2000-i, tg.ID); err != nil {
			t.Fatal(err)
		}
	}
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	return &Service{store: st, engine: &zfsFakeEngine{snaps: snaps}}, settings
}

func TestTheContainersDrillSkipsANewerContainerWithoutASnapshot(t *testing.T) {
	snaps := []restic.Snapshot{
		{ID: "webpage", Time: "2026-10-14T03:00:00Z", Tags: []string{"container:ix-webpage-1"}},
		{ID: "other", Time: "2026-10-15T03:00:00Z", Tags: []string{"container:plex"}},
	}
	s, settings := drillTargetFixture(t, snaps, "ix-kuma-uptime-kuma-1", "ix-webpage-1", "plex")

	id, err := s.pickDRSnapshot(context.Background(), "containers", settings, "repo", restic.Mode{})
	if err != nil || id != "webpage" {
		t.Fatalf("pickDRSnapshot = %q, %v, want the newest container that has a snapshot", id, err)
	}
}

func TestThePinnedDrillTargetFailsWithoutASnapshot(t *testing.T) {
	snaps := []restic.Snapshot{{ID: "webpage", Time: "2026-10-14T03:00:00Z", Tags: []string{"container:ix-webpage-1"}}}
	s, settings := drillTargetFixture(t, snaps, "ix-kuma-uptime-kuma-1", "ix-webpage-1")
	settings.DRDrillTarget = "ix-kuma-uptime-kuma-1"

	_, err := s.pickDRSnapshot(context.Background(), "containers", settings, "repo", restic.Mode{})
	if err == nil || !strings.Contains(err.Error(), `"ix-kuma-uptime-kuma-1"`) {
		t.Fatalf("pickDRSnapshot err = %v, want the pinned target named", err)
	}
}

func TestTheContainersDrillFailsWhenNoBackedUpContainerHasASnapshot(t *testing.T) {
	snaps := []restic.Snapshot{{ID: "gone", Time: "2026-10-14T03:00:00Z", Tags: []string{"container:removed-app"}}}
	s, settings := drillTargetFixture(t, snaps, "ix-kuma-uptime-kuma-1")

	if id, err := s.pickDRSnapshot(context.Background(), "containers", settings, "repo", restic.Mode{}); err == nil {
		t.Fatalf("pickDRSnapshot = %q, want an error", id)
	}
}
