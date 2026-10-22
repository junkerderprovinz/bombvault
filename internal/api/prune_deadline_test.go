package api_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
)

func pruneDeadlineService(t *testing.T) (*api.Service, *fakeResticEngine) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: filepath.ToSlash(dir)}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	s.RetentionKeepLast = 3
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(dir, "backups", "containers")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	eng := &fakeResticEngine{}
	return api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng), eng
}

// The prune after a round may run as long as a backup may, so a large remote
// repository is not cut off after half an hour every night.
func TestPruneAfterBulkGetsTheBackupCap(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Setenv("BACKUP_MAX_HOURS", "")
		svc, eng := pruneDeadlineService(t)
		svc.PruneAfterBulk(context.Background(), "containers")
		if len(eng.pruneDeadlines) != 1 {
			t.Fatalf("prunes = %d, want 1", len(eng.pruneDeadlines))
		}
		if left := eng.pruneDeadlines[0]; left < 47*time.Hour {
			t.Fatalf("prune had %v, want the 48h backup cap", left)
		}
	})
	t.Run("unlimited", func(t *testing.T) {
		t.Setenv("BACKUP_MAX_HOURS", "0")
		svc, eng := pruneDeadlineService(t)
		svc.PruneAfterBulk(context.Background(), "containers")
		if len(eng.pruneDeadlines) != 1 || eng.pruneDeadlines[0] != 0 {
			t.Fatalf("prune deadlines = %v, want one without a deadline", eng.pruneDeadlines)
		}
	})
}
