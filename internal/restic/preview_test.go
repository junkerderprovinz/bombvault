package restic_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

func TestRestorePreviewArgsAlwaysDryRun(t *testing.T) {
	steps := []restic.PreviewStep{
		{SnapshotID: "abcd1234", Target: "/t"},
		{SnapshotID: "abcd1234", Subtree: "/host/user/appdata/x", Target: "/host/user/appdata/x"},
		{SnapshotID: "abcd1234", Target: "/", Include: "/host/a", Excludes: []string{"/child"}},
	}
	for _, st := range steps {
		args := restic.RestorePreviewArgs("/repo", st, restic.Mode{Encrypted: true})
		sep := slices.Index(args, "--")
		dry := slices.Index(args, "--dry-run")
		if dry < 0 || dry > sep {
			t.Fatalf("%v: --dry-run missing before the selector", args)
		}
		if !slices.Contains(args, "--no-lock") {
			t.Errorf("%v: a preview must not lock the repository", args)
		}
	}
}

func TestRestorePreviewArgsSelector(t *testing.T) {
	args := restic.RestorePreviewArgs("/repo", restic.PreviewStep{SnapshotID: "abcd1234", Subtree: "/src", Target: "/dst", Include: "/a b", Excludes: []string{"/c"}}, restic.Mode{})
	if got := args[len(args)-1]; got != "abcd1234:/src" {
		t.Errorf("selector = %q", got)
	}
	for _, want := range [][]string{{"--target", "/dst"}, {"--include", "/a b"}, {"--exclude", "/c"}} {
		i := slices.Index(args, want[0])
		if i < 0 || args[i+1] != want[1] {
			t.Errorf("%v: want %s %s", args, want[0], want[1])
		}
	}
}

// TestRestorePreviewReportsWithoutWriting runs a dry run against the real
// restic and checks both what it reports and that the target is untouched.
func TestRestorePreviewReportsWithoutWriting(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}
	ctx := context.Background()
	dir := resolved(t, t.TempDir())
	repo := filepath.Join(dir, "repo")
	src := filepath.Join(dir, "src")
	live := filepath.Join(dir, "live")
	write := func(p, s string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil { //nolint:gosec // G301: test temp dir
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil { //nolint:gosec // G306: test file
			t.Fatal(err)
		}
	}
	write(filepath.Join(src, "same"), "same")
	write(filepath.Join(src, "sub", "changed"), "old")
	write(filepath.Join(src, "new"), "new")

	r := restic.Restic{Bin: "restic"}
	m := restic.Mode{}
	if err := r.Init(ctx, repo, m); err != nil {
		t.Fatal(err)
	}
	sum, err := r.Backup(ctx, repo, []string{src}, nil, m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(live, os.DirFS(src)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(live, "new")); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(live, "sub", "changed"), "longer now")
	write(filepath.Join(live, "extra"), "extra")

	got := map[string]string{}
	err = r.RestorePreview(ctx, repo, restic.PreviewStep{SnapshotID: sum.SnapshotID, Subtree: inSnapshot(src), Target: live}, m, func(it restic.PreviewItem) {
		got[filepath.ToSlash(it.Item)] = it.Action
	})
	if err != nil {
		t.Fatal(err)
	}
	for item, action := range map[string]string{"/same": "unchanged", "/sub/changed": "updated", "/new": "restored", "/extra": "deleted"} {
		if got[item] != action {
			t.Errorf("%s: action %q, want %q (all: %v)", item, got[item], action, got)
		}
	}
	if _, err := os.Stat(filepath.Join(live, "new")); !os.IsNotExist(err) {
		t.Error("the dry run wrote a file")
	}
	if _, err := os.Stat(filepath.Join(live, "extra")); err != nil {
		t.Error("the dry run deleted a file")
	}
}
