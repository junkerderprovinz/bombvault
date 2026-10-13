package api

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// seedNotes creates a file set on the notes share and backs it up. path is
// the set's own path below the Host Data mount, and selected narrows a folder
// set to those entries inside it.
func (r *restoreRig) seedNotes(t *testing.T, setPath string, selected ...string) (store.FileSet, string) {
	t.Helper()
	r.write(t, map[string]string{"notes/todo.txt": "buy milk", "notes/done.txt": "done"})
	set, err := r.st.CreateFileSet(store.FileSet{Name: "Notes", Path: setPath, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) > 0 {
		sel := make([]string, len(selected))
		for i, s := range selected {
			sel[i] = filepath.Join(r.root, s)
		}
		if err := r.st.SetFileSetSelectedPaths(set.ID, sel); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.svc.BackupFileSet(context.Background(), set.ID); err != nil {
		t.Fatalf("seed backup: %v", err)
	}
	return set, r.lastRun(t, set.ID, "backup").SnapshotID
}

func (r *restoreRig) restoreNotes(t *testing.T, set store.FileSet, snap, targetSubPath string, opts RestoreOptions) {
	t.Helper()
	if _, started, err := r.svc.StartRestoreFileSet(context.Background(), set.ID, snap, "local", targetSubPath, true, opts); err != nil || !started {
		t.Fatalf("StartRestoreFileSet: started=%v err=%v", started, err)
	}
	r.wait(t)
	if run := r.lastRun(t, set.ID, "restore"); run.Status != "success" {
		t.Fatalf("restore run = %s: %s", run.Status, run.Error)
	}
}

// filesBelow lists the files below dir as slash paths relative to it.
func filesBelow(t *testing.T, dir string) []string {
	t.Helper()
	var got []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(dir, p)
			got = append(got, filepath.ToSlash(rel))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestRestoreFileSetOfOneSelectedFile(t *testing.T) {
	t.Run("into a folder", func(t *testing.T) {
		r := newRestoreRig(t)
		set, snap := r.seedNotes(t, "notes", "notes/todo.txt")
		res := r.check(t, RestoreCheckRequest{Kind: checkFileSet, Name: set.ID, SnapshotID: snap, TargetPath: "restore/notes"})
		want := PlanFile{Path: "/mnt/user/restore/notes/todo.txt", Change: changeAdded}
		if !res.Ready || res.Plan == nil || res.Plan.Error != "" || !slices.Equal(res.Plan.Files, []PlanFile{want}) {
			t.Fatalf("ready=%v plan=%+v", res.Ready, res.Plan)
		}
		r.restoreNotes(t, set, snap, "restore/notes", RestoreOptions{})
		if got := filesBelow(t, filepath.Join(r.root, "restore/notes")); !slices.Equal(got, []string{"todo.txt"}) {
			t.Fatalf("the folder holds %v", got)
		}
		if got := read(t, filepath.Join(r.root, "restore/notes/todo.txt")); got != "buy milk" {
			t.Fatalf("todo.txt = %q", got)
		}
	})
	t.Run("beside the original", func(t *testing.T) {
		r := newRestoreRig(t)
		set, snap := r.seedNotes(t, "notes", "notes/todo.txt")
		res := r.check(t, RestoreCheckRequest{Kind: checkFileSet, Name: set.ID, SnapshotID: snap, Target: targetBeside})
		want := PlanFile{Path: "/mnt/user/notes-restored/todo.txt", Change: changeAdded}
		if !res.Ready || res.Plan == nil || res.Plan.Error != "" || !slices.Equal(res.Plan.Files, []PlanFile{want}) {
			t.Fatalf("ready=%v plan=%+v", res.Ready, res.Plan)
		}
		r.restoreNotes(t, set, snap, "", RestoreOptions{Beside: true})
		if got := filesBelow(t, filepath.Join(r.root, "notes-restored")); !slices.Equal(got, []string{"todo.txt"}) {
			t.Fatalf("the copy holds %v", got)
		}
	})
	t.Run("in place", func(t *testing.T) {
		r := newRestoreRig(t)
		set, snap := r.seedNotes(t, "notes", "notes/todo.txt")
		if err := os.RemoveAll(filepath.Join(r.root, "notes")); err != nil {
			t.Fatal(err)
		}
		r.restoreNotes(t, set, snap, "", RestoreOptions{})
		if got := filesBelow(t, filepath.Join(r.root, "notes")); !slices.Equal(got, []string{"todo.txt"}) {
			t.Fatalf("the set's folder holds %v", got)
		}
	})
}

func TestRestoreFileSetWhosePathIsAFile(t *testing.T) {
	t.Run("in place", func(t *testing.T) {
		r := newRestoreRig(t)
		set, snap := r.seedNotes(t, "notes/todo.txt")
		r.write(t, map[string]string{"notes/todo.txt": "buy bread"})
		res := r.check(t, RestoreCheckRequest{Kind: checkFileSet, Name: set.ID, SnapshotID: snap})
		want := PlanFile{Path: "/mnt/user/notes/todo.txt", Change: changeChanged}
		if !res.Ready || res.Plan == nil || res.Plan.Error != "" || !slices.Equal(res.Plan.Files, []PlanFile{want}) {
			t.Fatalf("ready=%v plan=%+v", res.Ready, res.Plan)
		}
		r.restoreNotes(t, set, snap, "", RestoreOptions{BackupFirst: true})
		if got := read(t, filepath.Join(r.root, "notes/todo.txt")); got != "buy milk" {
			t.Fatalf("todo.txt = %q", got)
		}
		if got := read(t, filepath.Join(r.root, "notes/done.txt")); got != "done" {
			t.Fatalf("a file next to the set changed: %q", got)
		}
		r.stateBefore(t, "files", "fileset:Notes", snap)
	})
	t.Run("into a folder", func(t *testing.T) {
		r := newRestoreRig(t)
		set, snap := r.seedNotes(t, "notes/todo.txt")
		r.restoreNotes(t, set, snap, "restore", RestoreOptions{})
		if got := filesBelow(t, filepath.Join(r.root, "restore")); !slices.Equal(got, []string{"todo.txt"}) {
			t.Fatalf("the folder holds %v", got)
		}
	})
	t.Run("beside the original", func(t *testing.T) {
		r := newRestoreRig(t)
		set, snap := r.seedNotes(t, "notes/todo.txt")
		r.restoreNotes(t, set, snap, "", RestoreOptions{Beside: true})
		if got := filesBelow(t, filepath.Join(r.root, "notes/todo.txt-restored")); !slices.Equal(got, []string{"todo.txt"}) {
			t.Fatalf("the copy holds %v", got)
		}
	})
}

func TestBackUpAFileSetWhosePathIsAFileAgain(t *testing.T) {
	r := newRestoreRig(t)
	set, _ := r.seedNotes(t, "notes/todo.txt")
	r.write(t, map[string]string{"notes/todo.txt": "buy bread"})
	if _, err := r.svc.BackupFileSet(context.Background(), set.ID); err != nil {
		t.Fatalf("second backup: %v", err)
	}
}
