package api

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
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
	if err := os.WriteFile(p, []byte("restic"), 0o440); err != nil {
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
