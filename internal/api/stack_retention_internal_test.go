package api

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// stackEngine records the retention a stack backup asks restic for.
type stackEngine struct {
	ResticEngine
	backupErr error
	snaps     []restic.Snapshot
	forgets   []stackForget
	prunes    int
}

type stackForget struct {
	Tag   string
	Prune bool
}

func (e *stackEngine) RepoOpens(context.Context, string, restic.Mode) bool { return true }

func (e *stackEngine) Unlock(context.Context, string, bool, restic.Mode) error { return nil }

func (e *stackEngine) Backup(context.Context, string, []string, []string, restic.Mode, ...string) (restic.Summary, error) {
	return restic.Summary{}, e.backupErr
}

func (e *stackEngine) Snapshots(context.Context, string, restic.Mode) ([]restic.Snapshot, error) {
	return e.snaps, nil
}

func (e *stackEngine) ForgetPolicy(_ context.Context, _ string, _ restic.RetentionPolicy, _ restic.Mode, tags []string, prune bool) error {
	e.forgets = append(e.forgets, stackForget{Tag: strings.Join(tags, ","), Prune: prune})
	return nil
}

func (e *stackEngine) Prune(context.Context, string, restic.Mode) error {
	e.prunes++
	return nil
}

// stackSvc is a containers domain with a local retention of three snapshots,
// with the host's /mnt mounted into a temporary directory.
func stackSvc(t *testing.T, eng *stackEngine) *Service {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := store.New(db)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersPath = "backups/containers"
	settings.RetentionKeepLast = 3
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	svc := &Service{store: st, engine: eng}
	svc.cfg.HostSourceRoot = "/mnt"
	svc.cfg.HostMountRoot = filepath.ToSlash(dir)
	svc.cfg.AppKey = strings.Repeat("a", 64)
	svc.cfg.DataDir = dir
	return svc
}

func TestAStackBackupAppliesTheLocalRetentionToItsProject(t *testing.T) {
	eng := &stackEngine{}
	if err := stackSvc(t, eng).backupStackDir(context.Background(), "immich", "/host/appdata/immich"); err != nil {
		t.Fatalf("backupStackDir: %v", err)
	}
	if want := []stackForget{{Tag: "stack:immich", Prune: true}}; !slices.Equal(eng.forgets, want) {
		t.Fatalf("forgets = %v, want %v", eng.forgets, want)
	}
}

func TestAStackBackupInARoundLeavesThePruneToTheRound(t *testing.T) {
	eng := &stackEngine{}
	ctx := WithBulkReplicateSuppressed(context.Background())
	if err := stackSvc(t, eng).backupStackDir(ctx, "immich", "/host/appdata/immich"); err != nil {
		t.Fatalf("backupStackDir: %v", err)
	}
	if want := []stackForget{{Tag: "stack:immich", Prune: false}}; !slices.Equal(eng.forgets, want) {
		t.Fatalf("forgets = %v, want %v", eng.forgets, want)
	}
}

func TestAFailedStackBackupForgetsNothing(t *testing.T) {
	eng := &stackEngine{backupErr: errors.New("permission denied")}
	if err := stackSvc(t, eng).backupStackDir(context.Background(), "immich", "/host/appdata/immich"); err == nil {
		t.Fatal("backupStackDir succeeded over a failed backup")
	}
	if len(eng.forgets) != 0 {
		t.Fatalf("forgets = %v, want none", eng.forgets)
	}
}

func TestIdentityTagsCountStackProjects(t *testing.T) {
	snaps := []restic.Snapshot{
		{Tags: []string{"container:immich-server", "p1"}},
		{Tags: []string{"stack:immich", "p1"}},
		{Tags: []string{"stack:immich", "p1"}},
	}
	if got, want := identityTags(snaps), []string{"container:immich-server", "stack:immich"}; !slices.Equal(got, want) {
		t.Fatalf("identityTags = %v, want %v", got, want)
	}
}

func TestPerIdentityRetentionAgesARepositoryOfProjectFolders(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	eng := &stackEngine{snaps: []restic.Snapshot{{ID: "a1", Tags: []string{"stack:immich", "p1"}}}}
	s := &Service{engine: eng, store: store.New(db)}
	if err := s.applyRetentionPerIdentity(context.Background(), "/repo", restic.RetentionPolicy{KeepLast: 3}, restic.Mode{}); err != nil {
		t.Fatalf("applyRetentionPerIdentity: %v", err)
	}
	if want := []stackForget{{Tag: "stack:immich", Prune: false}}; !slices.Equal(eng.forgets, want) || eng.prunes != 1 {
		t.Fatalf("forgets = %v, prunes = %d; want %v and one prune", eng.forgets, eng.prunes, want)
	}
}

// stackDocker reports every container as a member of one compose project.
type stackDocker struct {
	dockercli.Docker
	labels map[string]string
}

func (d *stackDocker) Inspect(context.Context, string) (model.Inspect, error) {
	return model.Inspect{Config: model.Config{Labels: d.labels}}, nil
}

func TestTheScheduledStackJobLeavesThePruneToTheRound(t *testing.T) {
	eng := &stackEngine{}
	s := stackSvc(t, eng)
	s.docker = &stackDocker{labels: map[string]string{
		"com.docker.compose.project":             "immich",
		"com.docker.compose.project.working_dir": "/mnt/appdata/immich",
	}}
	s.BackupStacksAfterBulk(context.Background(), []string{"immich-server", "immich-db"})
	if want := []stackForget{{Tag: "stack:immich", Prune: false}}; !slices.Equal(eng.forgets, want) || eng.prunes != 0 {
		t.Fatalf("forgets = %v, prunes = %d; want %v and no prune", eng.forgets, eng.prunes, want)
	}
}
