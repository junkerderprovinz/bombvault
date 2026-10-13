package restic_test

import (
	"context"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// fileNodeRepo backs up a folder, a database file next to it and a settings
// file one level up, each as its own path, the way a container's binds are
// backed up. It returns the engine, the repository, the snapshot and the
// folder that holds everything.
func fileNodeRepo(t *testing.T) (restic.Restic, string, string, string) {
	t.Helper()
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}
	if runtime.GOOS == "windows" {
		t.Skip("the <id>:<path> selector cannot carry a Windows drive letter")
	}
	dir := resolved(t, t.TempDir())
	root := filepath.Join(dir, "appdata")
	writeFile(t, filepath.Join(root, "app", "data", "notes.txt"), "notes", 0o644)
	writeFile(t, filepath.Join(root, "app", "app.db"), "database", 0o640)
	writeFile(t, filepath.Join(root, "app.json"), "settings", 0o644)
	r := restic.Restic{Bin: "restic"}
	repo := filepath.Join(dir, "repo")
	ctx := context.Background()
	if err := r.Init(ctx, repo, restic.Mode{}); err != nil {
		t.Fatal(err)
	}
	paths := []string{filepath.Join(root, "app", "data"), filepath.Join(root, "app", "app.db"), filepath.Join(root, "app.json")}
	sum, err := r.Backup(ctx, repo, paths, nil, restic.Mode{})
	if err != nil {
		t.Fatal(err)
	}
	return r, repo, sum.SnapshotID, root
}

func writeFile(t *testing.T, p, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil { //nolint:gosec // G301: test temp dir
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, mode); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p) //nolint:gosec // G304: a file below the test's own folder
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestHoldsFileTellsFilesFromFolders(t *testing.T) {
	r, repo, snap, root := fileNodeRepo(t)
	for rel, want := range map[string]bool{"app/app.db": true, "app.json": true, "app/data": false, "app": false, "app/missing": false} {
		got, err := r.HoldsFile(context.Background(), repo, snap, filepath.Join(root, rel), restic.Mode{})
		if err != nil || got != want {
			t.Errorf("HoldsFile(%s) = %v, %v, want %v", rel, got, err, want)
		}
	}
}

func TestRestorePathBringsBackAFileAfterItsFolderWasDeleted(t *testing.T) {
	r, repo, snap, root := fileNodeRepo(t)
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, rel := range []string{"app/data", "app/app.db", "app.json"} {
		if err := r.RestorePath(ctx, repo, snap, filepath.Join(root, rel), restic.Mode{}); err != nil {
			t.Fatalf("RestorePath(%s): %v", rel, err)
		}
	}
	for rel, want := range map[string]string{"app/data/notes.txt": "notes", "app/app.db": "database", "app.json": "settings"} {
		if got := readFile(t, filepath.Join(root, rel)); got != want {
			t.Errorf("%s = %q, want %q", rel, got, want)
		}
	}
	if info, err := os.Stat(filepath.Join(root, "app", "app.db")); err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("the database lost its mode: %v %v", info, err)
	}
}

func TestRestorePathReplacesAFileAndLeavesItsFolderAlone(t *testing.T) {
	r, repo, snap, root := fileNodeRepo(t)
	db := filepath.Join(root, "app", "app.db")
	writeFile(t, db, "a newer database", 0o600)
	writeFile(t, filepath.Join(root, "app", "beside.txt"), "not backed up", 0o644)

	if err := r.RestorePath(context.Background(), repo, snap, db, restic.Mode{}); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, db); got != "database" {
		t.Fatalf("app.db = %q", got)
	}
	if got := readFile(t, filepath.Join(root, "app", "beside.txt")); got != "not backed up" {
		t.Fatalf("a file next to the database changed: %q", got)
	}
}

func TestRestoreSubtreeToPutsAFileInThePlaceOfAnEmptyFolder(t *testing.T) {
	for name, target := range map[string]string{"under its own name": "app.json", "under another name": "app.json-restored"} {
		t.Run(name, func(t *testing.T) {
			r, repo, snap, root := fileNodeRepo(t)
			copyDir := filepath.Join(filepath.Dir(root), "copy")
			to := filepath.Join(copyDir, target)
			if err := os.MkdirAll(to, 0o755); err != nil { //nolint:gosec // G301: test temp dir
				t.Fatal(err)
			}
			if err := r.RestoreSubtreeTo(context.Background(), repo, snap, filepath.Join(root, "app.json"), to, restic.Mode{}); err != nil {
				t.Fatal(err)
			}
			if got := readFile(t, to); got != "settings" {
				t.Fatalf("%s = %q", target, got)
			}
			list, err := os.ReadDir(copyDir)
			if err != nil || len(list) != 1 || list[0].Name() != target {
				t.Fatalf("the folder holds %v (%v), want only %s", list, err, target)
			}
		})
	}
}

func TestRestoreSubtreeToRefusesToReplaceAFolderWithFilesInIt(t *testing.T) {
	for name, target := range map[string]string{"under its own name": "app.json", "under another name": "app.json-restored"} {
		t.Run(name, func(t *testing.T) {
			r, repo, snap, root := fileNodeRepo(t)
			to := filepath.Join(filepath.Dir(root), "copy", target)
			writeFile(t, filepath.Join(to, "kept.txt"), "kept", 0o644)
			if err := r.RestoreSubtreeTo(context.Background(), repo, snap, filepath.Join(root, "app.json"), to, restic.Mode{}); err == nil {
				t.Fatal("the restore replaced a folder with files in it")
			}
			if got := readFile(t, filepath.Join(to, "kept.txt")); got != "kept" {
				t.Fatalf("kept.txt = %q", got)
			}
			if list, _ := os.ReadDir(filepath.Dir(to)); len(list) != 1 {
				t.Fatalf("the restore left %v behind", list)
			}
		})
	}
}

func TestRestorePreviewOfAFileReportsItAtTheTarget(t *testing.T) {
	r, repo, snap, root := fileNodeRepo(t)
	db := filepath.Join(root, "app", "app.db")
	writeFile(t, filepath.Join(root, "app", "beside.txt"), "not backed up", 0o644)
	preview := func(target string) []restic.PreviewItem {
		t.Helper()
		var got []restic.PreviewItem
		st := restic.PreviewStep{SnapshotID: snap, Subtree: filepath.ToSlash(db), Target: filepath.ToSlash(target)}
		if err := r.RestorePreview(context.Background(), repo, st, restic.Mode{}, func(it restic.PreviewItem) { got = append(got, it) }); err != nil {
			t.Fatal(err)
		}
		return got
	}
	want := func(action string) []restic.PreviewItem {
		return []restic.PreviewItem{{Action: action, Item: "", Size: int64(len("database"))}}
	}

	if got := preview(db); !slices.Equal(got, want("unchanged")) {
		t.Fatalf("an untouched database previews as %+v", got)
	}
	writeFile(t, db, "a newer database", 0o640)
	if got := preview(db); !slices.Equal(got, want("updated")) {
		t.Fatalf("a changed database previews as %+v", got)
	}
	if got := preview(path.Join(filepath.ToSlash(root), "app-restored", "app.db")); !slices.Equal(got, want("restored")) {
		t.Fatalf("a copy previews as %+v", got)
	}
	if got := preview(db + "-restored"); !slices.Equal(got, want("restored")) {
		t.Fatalf("a copy under another name previews as %+v", got)
	}
	if got := readFile(t, db); got != "a newer database" {
		t.Fatalf("the preview wrote app.db: %q", got)
	}
}
