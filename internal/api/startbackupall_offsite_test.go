package api_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// offsiteBatchTestService builds a service like backupTestService, but with a
// COUPLED off-site repo (remote location, blank off-site schedule → replication
// is coupled to the backup run) configured for both the containers and files
// domains, and returns the store so tests can create file sets. Used by the #95
// batch-replication tests below.
func offsiteBatchTestService(t *testing.T) (*api.Service, *fakeResticEngine, *store.Repo) {
	t.Helper()
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: root}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	s.ContainersOffsite = "rest:http://192.168.1.2:8000/containers"
	s.FilesPath = "backups/files"
	s.FilesOffsite = "rest:http://192.168.1.2:8000/files"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	// Source dirs the batches back up: conventional appdata dirs for the
	// containers batch (mount translation falls back to <root>/appdata/<name>),
	// and the file-set source folders for the files batch.
	for _, p := range []string{"appdata/plex", "appdata/radarr", "data/docs", "data/pics"} {
		if err := os.MkdirAll(filepath.Join(dir, filepath.FromSlash(p)), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	d := &fakeServiceDocker{inspect: model.Inspect{Name: "/app", Image: "app:latest", Running: true}}
	eng := &fakeResticEngine{}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)
	return svc, eng, st
}

// TestStartBackupAllReplicatesOffsiteOnce pins #95 for the manual containers
// batch: with a coupled off-site repo (blank off-site schedule), a "back up all"
// of N containers must perform exactly ONE engine Copy for the whole batch —
// each item's inline replication is suppressed via the bulk flag on bctx and
// ReplicateOffsiteAfterBulk runs once after the loop. Without the suppression,
// every item would open the off-site repo for a full round-trip of its own.
func TestStartBackupAllReplicatesOffsiteOnce(t *testing.T) {
	svc, eng, _ := offsiteBatchTestService(t)

	if started, err := svc.StartBackupAll(context.Background(), []string{"plex", "radarr"}); err != nil || !started {
		t.Fatalf("StartBackupAll should start: started=%v err=%v", started, err)
	}
	// The shared guard is released only AFTER ReplicateOffsiteAfterBulk, so this
	// happens-after the batched copy (and makes temp-dir cleanup race-free).
	waitForBackupDone(t, svc)

	if len(eng.backedUp) != 2 {
		t.Fatalf("want 2 local backups, got %d (%v)", len(eng.backedUp), eng.backedUp)
	}
	if len(eng.copied) != 1 {
		t.Fatalf("a coupled off-site batch must replicate exactly ONCE for the whole batch (#95), got %d copies: %v", len(eng.copied), eng.copied)
	}
}

// TestStartBackupFilesAllReplicatesOffsiteOnce pins #95 for the manual files
// batch (the regression this test was written against: StartBackupFilesAll used
// to build its detached context WITHOUT the bulk flag and never batch-replicated,
// so N file sets meant N full off-site round-trips): with a coupled off-site
// repo, backing up N file sets must perform exactly ONE engine Copy for the
// whole batch.
func TestStartBackupFilesAllReplicatesOffsiteOnce(t *testing.T) {
	svc, eng, st := offsiteBatchTestService(t)
	docs, err := st.CreateFileSet(store.FileSet{Name: "docs", Path: "data/docs", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	pics, err := st.CreateFileSet(store.FileSet{Name: "pics", Path: "data/pics", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	if started, serr := svc.StartBackupFilesAll(context.Background(), []string{docs.ID, pics.ID}); serr != nil || !started {
		t.Fatalf("StartBackupFilesAll should start: started=%v err=%v", started, serr)
	}
	waitForBackupDone(t, svc)

	if len(eng.backedUp) != 2 {
		t.Fatalf("want 2 local backups, got %d (%v)", len(eng.backedUp), eng.backedUp)
	}
	if len(eng.copied) != 1 {
		t.Fatalf("a coupled off-site files batch must replicate exactly ONCE for the whole batch (#95), got %d copies: %v", len(eng.copied), eng.copied)
	}
}
