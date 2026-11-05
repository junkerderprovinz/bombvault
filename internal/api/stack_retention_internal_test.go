package api

import (
	"context"
	"errors"
	"os"
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
	backups    int
	unreadable string
	ResticEngine
	backupErr error
	snaps     []restic.Snapshot
	forgets   []stackForget
	prunes    int
	restored  int
}

func (e *stackEngine) RestorePath(context.Context, string, string, string, restic.Mode) error {
	e.restored++
	return nil
}

func (e *stackEngine) RestoreSubtreeTo(context.Context, string, string, string, string, restic.Mode) error {
	e.restored++
	return nil
}

// emptiedStack is a stack whose member has a finished backup run, with an empty
// project folder and a repository that holds the snaps of eng.
func emptiedStack(t *testing.T, eng *stackEngine) (*Service, string, store.Target) {
	t.Helper()
	svc := stackSvc(t, eng)
	repo := filepath.Join(svc.cfg.HostMountRoot, "backups", "containers")
	if err := os.MkdirAll(repo, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	tg, err := svc.store.UpsertTarget(store.Target{ContainerName: "immich-server"})
	if err != nil {
		t.Fatal(err)
	}
	id, err := svc.store.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.store.FinishRun(id, "success", "aa11", 1, ""); err != nil {
		t.Fatal(err)
	}
	return svc, t.TempDir(), tg
}

func TestAnEmptiedProjectFolderIsNotBackedUpAndAgesNothing(t *testing.T) {
	eng := &stackEngine{snaps: []restic.Snapshot{{Tags: []string{"stack:immich"}, Summary: &restic.SnapshotSummary{TotalBytesProcessed: 10}}}}
	svc, dir, tg := emptiedStack(t, eng)
	if err := svc.backupStackDir(context.Background(), "immich", stackFolder{dir: dir, member: "immich-server"}); err != nil {
		t.Fatalf("backupStackDir: %v", err)
	}
	if eng.backups != 0 || len(eng.forgets) != 0 {
		t.Fatalf("backups = %d, forgets = %v, want neither", eng.backups, eng.forgets)
	}
	last, err := svc.store.LastSuccessfulBackup(tg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(last.Error, store.NoteStackFolderEmpty+": ") {
		t.Fatalf("the member's run reads %q, want the warning", last.Error)
	}
	if svc.lastBackupSawLess(anomalyScope{Kind: anomalyScopeItem, ID: tg.ID}) {
		t.Fatal("the member's own data is complete, and its backups are held")
	}
}

func TestAProjectFolderThatWasAlwaysEmptyIsStillBackedUp(t *testing.T) {
	eng := &stackEngine{snaps: []restic.Snapshot{{Tags: []string{"stack:immich"}, Summary: &restic.SnapshotSummary{}}}}
	svc, dir, _ := emptiedStack(t, eng)
	if err := svc.backupStackDir(context.Background(), "immich", stackFolder{dir: dir, member: "immich-server"}); err != nil {
		t.Fatalf("backupStackDir: %v", err)
	}
	if eng.backups != 1 || len(eng.forgets) != 1 {
		t.Fatalf("backups = %d, forgets = %v, want one of each", eng.backups, eng.forgets)
	}
}

func TestAProjectFolderWithUnreadableFilesAgesNothing(t *testing.T) {
	eng := &stackEngine{unreadable: "open /appdata/immich/secret: permission denied"}
	svc, _, tg := emptiedStack(t, eng)
	if err := svc.backupStackDir(context.Background(), "immich", stackFolder{dir: "/host/appdata/immich", member: "immich-server"}); err != nil {
		t.Fatalf("backupStackDir: %v", err)
	}
	if len(eng.forgets) != 0 {
		t.Fatalf("forgets = %v, want none", eng.forgets)
	}
	last, err := svc.store.LastSuccessfulBackup(tg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(last.Error, store.NoteFilesUnreadable+": ") {
		t.Fatalf("the member's run reads %q, want the warning", last.Error)
	}
}

type stackForget struct {
	Tag   string
	Prune bool
}

func (e *stackEngine) RepoOpens(context.Context, string, restic.Mode) bool { return true }

func (e *stackEngine) Unlock(context.Context, string, bool, restic.Mode) error { return nil }

func (e *stackEngine) Backup(context.Context, string, []string, []string, restic.Mode, ...string) (restic.Summary, error) {
	e.backups++
	return restic.Summary{Unreadable: e.unreadable}, e.backupErr
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
	if err := stackSvc(t, eng).backupStackDir(context.Background(), "immich", stackFolder{dir: "/host/appdata/immich"}); err != nil {
		t.Fatalf("backupStackDir: %v", err)
	}
	if want := []stackForget{{Tag: "stack:immich", Prune: true}}; !slices.Equal(eng.forgets, want) {
		t.Fatalf("forgets = %v, want %v", eng.forgets, want)
	}
}

func TestAStackBackupInARoundLeavesThePruneToTheRound(t *testing.T) {
	eng := &stackEngine{}
	ctx := WithBulkReplicateSuppressed(context.Background())
	if err := stackSvc(t, eng).backupStackDir(ctx, "immich", stackFolder{dir: "/host/appdata/immich"}); err != nil {
		t.Fatalf("backupStackDir: %v", err)
	}
	if want := []stackForget{{Tag: "stack:immich", Prune: false}}; !slices.Equal(eng.forgets, want) {
		t.Fatalf("forgets = %v, want %v", eng.forgets, want)
	}
}

func TestAFailedStackBackupForgetsNothing(t *testing.T) {
	eng := &stackEngine{backupErr: errors.New("permission denied")}
	if err := stackSvc(t, eng).backupStackDir(context.Background(), "immich", stackFolder{dir: "/host/appdata/immich"}); err == nil {
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
	if _, err := s.applyRetentionPerIdentity(context.Background(), "/repo", "", restic.RetentionPolicy{KeepLast: 3}, restic.Mode{}); err != nil {
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
