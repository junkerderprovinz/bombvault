package api

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

type countingListEngine struct {
	ResticEngine
	snaps []restic.Snapshot
	calls int
}

func (e *countingListEngine) Snapshots(context.Context, string, restic.Mode) ([]restic.Snapshot, error) {
	e.calls++
	return e.snaps, nil
}

// backupTimesFixture is a service whose containers repository is a local
// directory with a snapshots folder, as restic lays one out.
func backupTimesFixture(t *testing.T) (*Service, *countingListEngine, *store.Repo, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(dir, "backups", "containers")
	if err := os.MkdirAll(filepath.Join(repo, "snapshots"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	eng := &countingListEngine{snaps: []restic.Snapshot{
		{ID: "aaaa1111", Time: "2024-06-01T00:00:00Z", Tags: []string{"container:radarr"}},
	}}
	svc := NewService(config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}, st, nil, nil, eng)
	addSnapshotFile(t, repo, "aaaa1111")
	return svc, eng, st, repo
}

func addSnapshotFile(t *testing.T, repo, id string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, "snapshots", id), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestBackupTimesListAnUnchangedRepositoryOnce(t *testing.T) {
	svc, eng, _, _ := backupTimesFixture(t)
	for range 3 {
		times, _, err := svc.LatestContainerBackupTimes(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if times["radarr"].Files == 0 {
			t.Fatalf("times = %+v, want radarr dated", times)
		}
	}
	if eng.calls != 1 {
		t.Fatalf("restic listed the repository %d times, want once", eng.calls)
	}
}

func TestBackupTimesSeeANewSnapshot(t *testing.T) {
	svc, eng, _, repo := backupTimesFixture(t)
	if _, _, err := svc.LatestContainerBackupTimes(context.Background()); err != nil {
		t.Fatal(err)
	}
	eng.snaps = append(eng.snaps, restic.Snapshot{ID: "bbbb2222", Time: "2024-07-01T00:00:00Z", Tags: []string{"container:sonarr"}})
	addSnapshotFile(t, repo, "bbbb2222")

	times, _, err := svc.LatestContainerBackupTimes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if eng.calls != 2 || times["sonarr"].Files == 0 {
		t.Fatalf("calls = %d, times = %+v, want a second listing that dates sonarr", eng.calls, times)
	}
}

func TestBackupTimesListAgainWhenEncryptionChanges(t *testing.T) {
	svc, eng, st, _ := backupTimesFixture(t)
	if _, _, err := svc.LatestContainerBackupTimes(context.Background()); err != nil {
		t.Fatal(err)
	}
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.EncryptionEnabled = !settings.EncryptionEnabled
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.LatestContainerBackupTimes(context.Background()); err != nil {
		t.Fatal(err)
	}
	if eng.calls != 2 {
		t.Fatalf("restic listed the repository %d times, want a second listing with the new credentials", eng.calls)
	}
}

func TestBackupTimesListEveryTimeWithoutASnapshotsFolder(t *testing.T) {
	svc, eng, _, repo := backupTimesFixture(t)
	if err := os.RemoveAll(filepath.Join(repo, "snapshots")); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, _, err := svc.LatestContainerBackupTimes(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if eng.calls != 2 {
		t.Fatalf("restic listed the repository %d times, want every time", eng.calls)
	}
}
