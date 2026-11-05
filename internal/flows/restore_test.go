//go:build flows

package flows

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// newestOf is the newest snapshot of a tag in a repository.
func (f *flow) newestOf(repo, tag string) resticSnapshot {
	f.t.Helper()
	snaps := f.b.snapshotsIn(repo, tag)
	if len(snaps) == 0 {
		f.t.Fatalf("no snapshot of %s in %s", tag, repo)
	}
	return slices.MaxFunc(snaps, func(a, b resticSnapshot) int { return a.Time.Compare(b.Time) })
}

// wantFile checks a restored file's content and, where the system keeps them,
// its mode.
func (f *flow) wantFile(p, content string, mode os.FileMode) {
	f.t.Helper()
	got, err := os.ReadFile(p)
	if err != nil {
		f.errorf("restored file: %v", err)
		return
	}
	if string(got) != content {
		f.errorf("%s reads %q, want %q", p, got, content)
	}
	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(p)
	if err != nil {
		f.errorf("restored file: %v", err)
	} else if info.Mode().Perm() != mode {
		f.errorf("%s has mode %o, want %o", p, info.Mode().Perm(), mode)
	}
}

// findBelow locates a file by the end of its path below a folder, since a restore
// to a folder recreates the path the snapshot was taken of.
func findBelow(root, suffix string) string {
	var found string
	suffix = string(filepath.Separator) + filepath.FromSlash(suffix)
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, suffix) {
			found = p
		}
		return nil
	})
	return found
}

func TestRestoreOfOneFileBringsBackItsContentAndMode(t *testing.T) {
	mountedSetsCell(t, restorePlaces, func(f *flow, sets folderSets) {
		f.b.write(photosPath, "sub/b.txt", "the version to get back\n", 0o600)
		f.backupSet(sets.photos)
		sn := f.newestOf(f.own.repo, photosTag)

		var listed struct {
			Files []struct {
				Path string `json:"path"`
			} `json:"files"`
		}
		f.b.call("GET", "/api/files/sets/"+sets.photos+"/files?snapshot="+sn.ID, nil, &listed)
		i := slices.IndexFunc(listed.Files, func(e struct {
			Path string `json:"path"`
		}) bool {
			return strings.HasSuffix(e.Path, "/sub/b.txt")
		})
		if i < 0 {
			f.t.Fatalf("the snapshot lists no sub/b.txt: %+v", listed.Files)
		}
		chosen := listed.Files[i].Path

		// Unconfirmed, whichever way it would write.
		for _, body := range []map[string]any{
			{"snapshotId": sn.ID, "paths": []string{chosen}},
			{"snapshotId": sn.ID, "paths": []string{chosen}, "targetPath": "user/restored"},
		} {
			if _, raw := f.b.send("POST", "/api/files/sets/"+sets.photos+"/restore-files", body); !strings.Contains(string(raw), `"ok":false`) {
				f.errorf("a restore without confirm started: %s", raw)
			}
		}
		f.b.settle()

		// Into a folder first, which leaves the source alone.
		f.b.write(photosPath, "sub/b.txt", "changed since\n", 0o600)
		const target = "user/restored"
		f.b.call("POST", "/api/files/sets/"+sets.photos+"/restore-files",
			map[string]any{"snapshotId": sn.ID, "paths": []string{chosen}, "targetPath": target, "confirm": true}, nil)
		f.b.settle()
		got := findBelow(f.b.hostPath(target), "b.txt")
		if got == "" {
			f.t.Fatalf("no b.txt below %s", target)
		}
		f.wantFile(got, "the version to get back\n", 0o600)
		if other := findBelow(f.b.hostPath(target), "a.txt"); other != "" {
			f.errorf("the restore of one file also wrote %s", other)
		}
		f.wantFile(filepath.Join(f.b.hostPath(photosPath), "sub", "b.txt"), "changed since\n", 0o600)

		// Then over the original.
		f.b.call("POST", "/api/files/sets/"+sets.photos+"/restore-files",
			map[string]any{"snapshotId": sn.ID, "paths": []string{chosen}, "confirm": true}, nil)
		f.b.settle()
		f.wantFile(filepath.Join(f.b.hostPath(photosPath), "sub", "b.txt"), "the version to get back\n", 0o600)

		restores := f.b.runsOf("restore")
		if len(restores) < 2 || restores[0].Status != "success" {
			f.errorf("the restores are recorded as %+v", restores)
		}
		f.wantNoFailedRuns()
	})
}

func TestRestoreBesideLeavesTheOriginalAlone(t *testing.T) {
	mountedSetsCell(t, restorePlaces, func(f *flow, sets folderSets) {
		f.b.write(photosPath, "a.txt", "as backed up\n", 0o640)
		f.backupSet(sets.photos)
		sn := f.newestOf(f.own.repo, photosTag)
		f.b.write(photosPath, "a.txt", "changed since\n", 0o640)
		f.b.write(photosPath, "new.txt", "came later\n", 0o644)

		var answer struct {
			Target string `json:"target"`
		}
		f.b.call("POST", "/api/files/sets/"+sets.photos+"/restore", map[string]any{"snapshotId": sn.ID, "target": "beside"}, &answer)
		f.b.settle()

		beside := f.b.hostPath(photosPath + "-restored")
		f.wantFile(filepath.Join(beside, "a.txt"), "as backed up\n", 0o640)
		f.wantFile(filepath.Join(beside, "sub", "b.txt"), "stays the same\n", 0o600)
		if _, err := os.Stat(filepath.Join(beside, "new.txt")); err == nil {
			f.errorf("the copy holds a file the backup did not")
		}
		f.wantFile(filepath.Join(f.b.hostPath(photosPath), "a.txt"), "changed since\n", 0o640)
		f.wantFile(filepath.Join(f.b.hostPath(photosPath), "new.txt"), "came later\n", 0o644)
		f.wantNoFailedRuns()
	})
}

func TestRestoreOverTheOriginalBringsBackTheBackup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("BombVault ships for Linux, and a restore over the original finds nothing in a snapshot path recorded on Windows")
	}
	setsCell(t, restorePlaces, func(f *flow, sets folderSets) {
		f.b.write(photosPath, "a.txt", "as backed up\n", 0o640)
		f.backupSet(sets.photos)
		sn := f.newestOf(f.own.repo, photosTag)
		f.b.write(photosPath, "a.txt", "changed since\n", 0o600)
		if err := os.Remove(filepath.Join(f.b.hostPath(photosPath), "sub", "b.txt")); err != nil {
			f.t.Fatal(err)
		}

		f.b.call("POST", "/api/files/sets/"+sets.photos+"/restore", map[string]any{"snapshotId": sn.ID, "confirm": true}, nil)
		f.b.settle()

		f.wantFile(filepath.Join(f.b.hostPath(photosPath), "a.txt"), "as backed up\n", 0o640)
		f.wantFile(filepath.Join(f.b.hostPath(photosPath), "sub", "b.txt"), "stays the same\n", 0o600)
		f.wantNoFailedRuns()
	})
}
