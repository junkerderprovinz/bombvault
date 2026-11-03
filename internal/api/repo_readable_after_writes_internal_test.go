package api

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// writeAsRestic adds a file to a local repository the way restic run as root
// does once the relax pass has opened the repository up: restic derives
// group-readable modes from it, never other-readable ones. It reports with
// Error rather than Fatal because some writers run on their own goroutine.
func writeAsRestic(t *testing.T, repo string, elems ...string) {
	t.Helper()
	p := filepath.Join(append([]string{filepath.FromSlash(repo)}, elems...)...)
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Error(err)
		return
	}
	if err := os.WriteFile(p, []byte("restic"), 0o440); err != nil { //nolint:gosec // G306: the group-readable mode restic writes is what the relax pass has to fix
		t.Error(err)
	}
}

// rootWritesEngine is the placement engine with a backup, a prune and a tag
// that leave files behind as restic does.
type rootWritesEngine struct {
	*placementEngine
	t *testing.T
}

func (e rootWritesEngine) Backup(_ context.Context, repo string, _, _ []string, _ restic.Mode, _ ...string) (restic.Summary, error) {
	writeAsRestic(e.t, repo, "data", "aa", "backedup")
	return restic.Summary{SnapshotID: "s1"}, nil
}

func (e rootWritesEngine) Prune(ctx context.Context, repo string, mode restic.Mode) error {
	writeAsRestic(e.t, repo, "data", "bb", "repacked")
	writeAsRestic(e.t, repo, "index", "repacked")
	return e.placementEngine.Prune(ctx, repo, mode)
}

func (e rootWritesEngine) Tag(ctx context.Context, repo string, ids, add, remove []string, mode restic.Mode) error {
	writeAsRestic(e.t, repo, "snapshots", "retagged")
	return e.placementEngine.Tag(ctx, repo, ids, add, remove, mode)
}

// unreadableByOthers lists what under repo a user outside the owner's group
// could not read or enter.
func unreadableByOthers(t *testing.T, repo string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(filepath.FromSlash(repo), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		want := fs.FileMode(0o004)
		if d.IsDir() {
			want = 0o005
		}
		if info.Mode().Perm()&want != want {
			out = append(out, p+" "+info.Mode().Perm().String())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// filesBulkRun backs up one file set as an item of a bulk run, which forgets
// without pruning and relaxes the repository before the prune comes.
func filesBulkRun(t *testing.T) (*placementFixture, context.Context, string) {
	t.Helper()
	f := newPlacementFixture(t)
	withOwnRetention(t, f.st, 3, nil)
	set := f.fileSet("docs", "")
	if err := os.WriteFile(filepath.Join(filepath.FromSlash(f.root), "files", "docs", "a.txt"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	repo := f.domainPath("files")
	f.hold(repo, snap("s1", 100, "fileset:docs"))
	f.svc.engine = rootWritesEngine{placementEngine: f.eng, t: t}
	ctx := WithBulkReplicateSuppressed(context.Background())
	if _, err := f.svc.BackupFileSet(ctx, set.ID); err != nil {
		t.Fatal(err)
	}
	if got := unreadableByOthers(t, repo); len(got) > 0 {
		t.Fatalf("the backup left %v unreadable", got)
	}
	if _, err := os.Stat(permStampPath(f.svc.cfg.DataDir, filepath.FromSlash(repo))); err != nil {
		t.Fatalf("the backup's relax pass left no stamp, so the next pass would not be a scoped one: %v", err)
	}
	return f, ctx, repo
}

func TestAPruneLeavesWhatItWroteReadableByOthers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits are not modelled on windows")
	}
	for _, tc := range []struct {
		name  string
		prune func(context.Context, *placementFixture) error
	}{
		{"after a bulk run", func(ctx context.Context, f *placementFixture) error {
			f.svc.PruneAfterBulk(ctx, "files")
			return nil
		}},
		{"by hand", func(ctx context.Context, f *placementFixture) error {
			_, err := f.svc.PruneDomain(ctx, "files", "local")
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, ctx, repo := filesBulkRun(t)
			if err := tc.prune(ctx, f); err != nil {
				t.Fatal(err)
			}
			if !prunedAt(f, repo) {
				t.Fatal("the repository was not pruned")
			}
			if got := unreadableByOthers(t, repo); len(got) > 0 {
				t.Fatalf("the prune left %v unreadable", got)
			}
		})
	}
}

func TestAPruneAfterADeleteLeavesWhatItWroteReadableByOthers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits are not modelled on windows")
	}
	f := newPlacementFixture(t)
	f.svc.engine = rootWritesEngine{placementEngine: f.eng, t: t}
	repo := f.domainPath("containers")
	f.svc.pruneAfterDelete("containers", filepath.FromSlash(repo), restic.Mode{})
	f.svc.deletePrunes.Wait()
	if !prunedAt(f, repo) {
		t.Fatal("the repository was not pruned")
	}
	if got := unreadableByOthers(t, repo); len(got) > 0 {
		t.Fatalf("the prune left %v unreadable", got)
	}
}

func TestATagLeavesTheRewrittenSnapshotReadableByOthers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits are not modelled on windows")
	}
	f := newPlacementFixture(t)
	f.container("web", "")
	repo := f.domainPath("containers")
	f.hold(repo, snap("aaaaaaaa", 100, "container:web"))
	f.svc.engine = rootWritesEngine{placementEngine: f.eng, t: t}
	if err := f.svc.TagBackup(context.Background(), "containers", "web", "local", "aaaaaaaa", []string{"keep"}, nil); err != nil {
		t.Fatal(err)
	}
	if len(f.eng.tags) != 1 {
		t.Fatalf("tag calls = %+v, want one", f.eng.tags)
	}
	if got := unreadableByOthers(t, repo); len(got) > 0 {
		t.Fatalf("the tag left %v unreadable", got)
	}
}

// rootWritesPull is the pull recorder with a copy that lands files as restic
// does.
type rootWritesPull struct {
	pullRecorder
	t *testing.T
}

func (e *rootWritesPull) Copy(ctx context.Context, dest, src string, ids []string, lim restic.Limits, m restic.Mode) error {
	writeAsRestic(e.t, dest, "data", "cc", "pulled")
	return e.pullRecorder.Copy(ctx, dest, src, ids, lim, m)
}

func TestAPullLeavesWhatItCopiedReadableByOthers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits are not modelled on windows")
	}
	eng := &rootWritesPull{pullRecorder: pullRecorder{snaps: map[string][]restic.Snapshot{pullSrcLoc: {{ID: "aaa"}}}}, t: t}
	svc, st, ps := pullFixture(t, eng)
	if _, err := st.MutateSettings(func(s *store.Settings) error {
		s.ContainersPath = "backups/containers"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PullFromSource(context.Background(), ps); err != nil {
		t.Fatal(err)
	}
	dest := pulledRepo(filepath.Join(svc.cfg.HostMountRoot, "backups", "containers"), ps.ID)
	if _, err := os.Stat(filepath.Join(dest, "data", "cc", "pulled")); err != nil {
		t.Fatalf("the copy did not land in %s: %v", dest, err)
	}
	if got := unreadableByOthers(t, dest); len(got) > 0 {
		t.Fatalf("the pull left %v unreadable", got)
	}
}

func TestAFlashBackupStraightToATargetOnAShareLeavesItReadableByOthers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits are not modelled on windows")
	}
	f := newPlacementFixture(t)
	f.svc.cfg.FlashDir = t.TempDir()
	if err := os.WriteFile(filepath.Join(f.svc.cfg.FlashDir, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.singletonRepo("flash")
	share := f.localRepo("backups/share-flash")
	f.target("flash", "Share", "backups/share-flash")
	switchLocal(t, f, "flash", false)
	f.svc.engine = rootWritesEngine{placementEngine: f.eng, t: t}
	if _, err := f.svc.BackupFlash(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(filepath.FromSlash(share), "data", "aa", "backedup")); err != nil {
		t.Fatalf("the backup did not land on the share: %v", err)
	}
	if got := unreadableByOthers(t, share); len(got) > 0 {
		t.Fatalf("the backup left %v unreadable", got)
	}
}

func TestAStackBackupLeavesWhatItWroteReadableByOthers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits are not modelled on windows")
	}
	f := newPlacementFixture(t)
	f.svc.engine = rootWritesEngine{placementEngine: f.eng, t: t}
	dir := t.TempDir()
	if err := f.svc.backupStackDir(WithBulkReplicateSuppressed(context.Background()), "media", dir); err != nil {
		t.Fatal(err)
	}
	repo := f.domainPath("containers")
	if _, err := os.Stat(filepath.Join(filepath.FromSlash(repo), "data", "aa", "backedup")); err != nil {
		t.Fatalf("the stack was not backed up into %s: %v", repo, err)
	}
	if got := unreadableByOthers(t, repo); len(got) > 0 {
		t.Fatalf("the stack backup left %v unreadable", got)
	}
}
