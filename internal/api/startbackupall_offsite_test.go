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

// offsiteBatchTestService builds a service like backupTestService with a remote
// off-site repo for containers and files. The off-site schedule is blank, so
// replication is coupled to the backup run. The store is returned so tests can
// create file sets.
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
	// Both domains' own repos, as an earlier backup would have left them: a
	// domain whose local repo was never created has nothing to replicate off
	// site.
	for _, p := range []string{"backups/containers", "backups/files"} {
		repo := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(repo, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	d := &fakeServiceDocker{inspect: model.Inspect{Name: "/app", Image: "app:latest", Running: true}}
	eng := &fakeResticEngine{}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)
	return svc, eng, st
}

// A batch replicates once: the bulk flag on bctx stops each item from copying
// on its own, and ReplicateOffsiteAfterBulk runs after the loop.
func TestStartBackupAllReplicatesOffsiteOnce(t *testing.T) {
	svc, eng, _ := offsiteBatchTestService(t)

	if started, err := svc.StartBackupAll(context.Background(), []string{"plex", "radarr"}); err != nil || !started {
		t.Fatalf("StartBackupAll should start: started=%v err=%v", started, err)
	}
	// The shared guard is released after ReplicateOffsiteAfterBulk, so waiting
	// for it also waits for the copy.
	waitForBackupDone(t, svc)

	if len(eng.backedUp) != 2 {
		t.Fatalf("want 2 local backups, got %d (%v)", len(eng.backedUp), eng.backedUp)
	}
	if len(eng.copied) != 1 {
		t.Fatalf("a coupled off-site batch must replicate exactly ONCE for the whole batch (#95), got %d copies: %v", len(eng.copied), eng.copied)
	}
}

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
