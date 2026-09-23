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

// listOnlyEngine answers Snapshots, the one call LatestContainerBackupTimes
// makes. Anything else hits the nil embed and panics.
type listOnlyEngine struct {
	ResticEngine
	snaps []restic.Snapshot
}

func (e *listOnlyEngine) Snapshots(_ context.Context, _ string, _ restic.Mode) ([]restic.Snapshot, error) {
	return e.snaps, nil
}

// Without the target list the fold's reused-name guard has nothing to check
// against, so every tag stays its own identity. A missed fold is cosmetic, a
// wrong one merges two entries' retention for good. It is an internal test
// because only a broken schema makes ListTargets fail.
func TestFoldAliasedIdentityTagsDoesNotFoldWhenTargetsUnreadable(t *testing.T) {
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

	tg, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAlias("container", "radarr-movies", tg.ID); err != nil {
		t.Fatal(err)
	}

	svc := NewService(config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}, st, nil, nil, nil)
	// radarr-movies's only snapshot predates the link, so the alias may fold.
	snaps := []restic.Snapshot{
		{ID: "aaaa1111", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}},
		{ID: "bbbb2222", Time: "2024-06-01T00:00:00Z", Tags: []string{"container:radarr", "p1"}},
	}

	// With the target list readable, the alias folds the two tags into one
	// group.
	before, _ := svc.foldAliasedIdentityTags([]string{"container:radarr-movies", "container:radarr"}, snaps)
	if len(before) != 1 || len(before[0]) != 2 {
		t.Fatalf("baseline fold = %v, want one group of two", before)
	}

	if _, err := db.Exec("DROP TABLE targets"); err != nil {
		t.Fatal(err)
	}

	after, _ := svc.foldAliasedIdentityTags([]string{"container:radarr-movies", "container:radarr"}, snaps)
	if len(after) != 2 {
		t.Fatalf("fold with ListTargets unreadable = %v, want 2 separate groups (no fold at all)", after)
	}
}

// With the alias table unreadable the fold cannot tell whether a container or
// VM tag is an old name whose snapshots two machines share, so it leaves
// those tags out of the pass instead of forgetting each as its own group.
// Tags that can never be an old name are unaffected.
func TestFoldAliasedIdentityTagsSkipsNamesWhenAliasesAreUnreadable(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	svc := NewService(config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}, store.New(db), nil, nil, nil)
	if _, err := db.Exec("DROP TABLE target_aliases"); err != nil {
		t.Fatal(err)
	}
	snaps := []restic.Snapshot{
		{ID: "aaaa1111", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr", "p1"}},
		{ID: "bbbb2222", Time: "2024-01-01T00:00:00Z", Tags: []string{"vm:win11", "p2"}},
		{ID: "cccc3333", Time: "2024-01-01T00:00:00Z", Tags: []string{"flash"}},
	}
	groups, skipped := svc.foldAliasedIdentityTags([]string{"container:radarr", "vm:win11", "flash"}, snaps)
	if len(groups) != 1 || len(groups[0]) != 1 || groups[0][0] != "flash" {
		t.Fatalf("groups = %v, want [[flash]]", groups)
	}
	if strings.Join(skipped, ",") != "container:radarr,vm:win11" {
		t.Fatalf("skipped = %v, want the container and the VM tag", skipped)
	}
}

// LatestContainerBackupTimes has its own copy of the guard: with ListTargets
// unreadable, the old name keeps its own row instead of folding into the
// current one's date.
func TestLatestContainerBackupTimesDoesNotFoldWhenTargetsUnreadable(t *testing.T) {
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
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	tg, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAlias("container", "radarr-movies", tg.ID); err != nil {
		t.Fatal(err)
	}
	eng := &listOnlyEngine{snaps: []restic.Snapshot{
		{ID: "aaaa1111", Time: "2024-06-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}},
		{ID: "bbbb2222", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr", "p1"}},
	}}
	svc := NewService(config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}, st, nil, nil, eng)

	if _, err := db.Exec("DROP TABLE targets"); err != nil {
		t.Fatal(err)
	}

	times, err := svc.LatestContainerBackupTimes(context.Background())
	if err != nil {
		t.Fatalf("LatestContainerBackupTimes: %v", err)
	}
	if _, ok := times["radarr-movies"]; !ok {
		t.Fatalf("with ListTargets unreadable, radarr-movies must keep its own row rather than fold into radarr's, got %+v", times)
	}
}
