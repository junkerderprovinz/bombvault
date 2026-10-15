package api

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// asMountPoint makes the import treat dir as a mount of its own for the test.
func asMountPoint(t *testing.T, dir string) {
	t.Helper()
	prev := dataDirIsMount
	dataDirIsMount = func(d string) bool { return filepath.ToSlash(d) == filepath.ToSlash(dir) }
	t.Cleanup(func() { dataDirIsMount = prev })
}

func TestImportEmptiesAMountPointInsteadOfRenamingIt(t *testing.T) {
	t.Run("the folder stays and its contents are kept next to it", func(t *testing.T) {
		rig := newImportRig(t)
		asMountPoint(t, rig.dataDir)
		if err := os.Mkdir(filepath.Join(rig.dataDir, "base"), 0o700); err != nil {
			t.Fatal(err)
		}
		before := statOf(t, rig.dataDir)
		_ = os.SameFile(before, before)
		var atStart []string
		rig.dock.onStart = func() { atStart = entryNames(t, rig.dataDir) }

		if _, err := rig.svc.StartImportDBDump(context.Background(), "pg", "local", importDumpID); err != nil {
			t.Fatal(err)
		}
		waitForDetachedRun(t, rig.svc)

		if len(atStart) != 0 {
			t.Errorf("at start the data folder held %v, want it empty", atStart)
		}
		if !os.SameFile(before, statOf(t, rig.dataDir)) {
			t.Error("the data folder was replaced, a mount point has to stay")
		}
		kept := siblingsOf(t, rig.dataDir, ".bombvault-before-import-*")
		if len(kept) != 1 {
			t.Fatalf("kept folders = %v, want exactly one", kept)
		}
		if got, err := os.ReadFile(filepath.Join(kept[0], "PG_VERSION")); err != nil || string(got) != "16\n" {
			t.Errorf("the kept folder holds %q (%v), want the old data", got, err)
		}
		if _, err := os.Stat(filepath.Join(kept[0], "base")); err != nil {
			t.Errorf("the kept folder lost a subfolder: %v", err)
		}
	})

	t.Run("a failed start puts the old data back into the mount point", func(t *testing.T) {
		rig := newImportRig(t)
		asMountPoint(t, rig.dataDir)
		rig.dock.startErr = os.ErrDeadlineExceeded
		rig.dock.onStart = func() {
			if _, err := os.Stat(filepath.Join(rig.dataDir, "PG_VERSION")); err == nil {
				return
			}
			if err := os.WriteFile(filepath.Join(rig.dataDir, "fresh"), []byte("x"), 0o600); err != nil {
				t.Error(err)
			}
		}

		if _, err := rig.svc.StartImportDBDump(context.Background(), "pg", "local", importDumpID); err != nil {
			t.Fatal(err)
		}
		waitForDetachedRun(t, rig.svc)

		if got := entryNames(t, rig.dataDir); len(got) != 1 || got[0] != "PG_VERSION" {
			t.Errorf("the data folder holds %v, want only the old data", got)
		}
		failed := siblingsOf(t, rig.dataDir, ".bombvault-import-failed-*")
		if len(failed) != 1 {
			t.Fatalf("failed folders = %v, want the fresh one kept aside", failed)
		}
		if _, err := os.Stat(filepath.Join(failed[0], "fresh")); err != nil {
			t.Errorf("the fresh data is not in the failed folder: %v", err)
		}
		if got := siblingsOf(t, rig.dataDir, ".bombvault-before-import-*"); len(got) != 0 {
			t.Errorf("the kept folder is still aside: %v", got)
		}
	})
}

func TestImportRenamesAnOrdinaryFolder(t *testing.T) {
	rig := newImportRig(t)
	asMountPoint(t, filepath.Join(rig.dataDir, "elsewhere"))
	before := statOf(t, rig.dataDir)
	_ = os.SameFile(before, before)

	if _, err := rig.svc.StartImportDBDump(context.Background(), "pg", "local", importDumpID); err != nil {
		t.Fatal(err)
	}
	waitForDetachedRun(t, rig.svc)

	kept := siblingsOf(t, rig.dataDir, ".bombvault-before-import-*")
	if len(kept) != 1 || !os.SameFile(before, statOf(t, kept[0])) {
		t.Fatalf("kept = %v, want the original folder renamed", kept)
	}
}

func TestImportRefusesAMountPointItCannotEmptyBeforeStopping(t *testing.T) {
	cases := map[string]func(t *testing.T, rig *importRig){
		"the folder next to it is not writable": func(t *testing.T, rig *importRig) {
			prev := dataDirIsMount
			dataDirIsMount = func(string) bool { return true }
			t.Cleanup(func() { dataDirIsMount = prev })
			rig.dock.inspect.Mounts[0].Source = "/data/missing/pg"
		},
		"there is not enough room next to it": func(t *testing.T, rig *importRig) {
			asMountPoint(t, rig.dataDir)
			prev := freeBytesOf
			freeBytesOf = func(string) (uint64, error) { return 0, nil }
			t.Cleanup(func() { freeBytesOf = prev })
			if err := os.WriteFile(filepath.Join(rig.dataDir, "big"), make([]byte, 4096), 0o600); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, prepare := range cases {
		t.Run(name, func(t *testing.T) {
			rig := newImportRig(t)
			prepare(t, rig)

			w := httptest.NewRecorder()
			dumpHandler(rig.svc).handleImportDBDump(w, importRequest("pg", importDumpID))

			if !strings.Contains(w.Body.String(), `"ok":false`) || !strings.Contains(w.Body.String(), "mount of its own") {
				t.Fatalf("answer = %s, want a refusal that names the mount", w.Body.String())
			}
			if got := strings.Join(rig.dock.calls, " "); strings.Contains(got, "stop:") {
				t.Errorf("calls = %q, want the container untouched", got)
			}
			if got := siblingsOf(t, rig.dataDir, ".bombvault-*"); len(got) != 0 {
				t.Errorf("a refusal moved the data folder: %v", got)
			}
		})
	}
}

func TestMoveContentsCopiesATreeAcrossFilesystems(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "a", "b"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a", "b", "f"), []byte("data"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := cloneTree(filepath.Join(src, "a"), filepath.Join(dst, "a")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "a", "b", "f"))
	if err != nil || string(got) != "data" {
		t.Fatalf("copy holds %q (%v), want the file", got, err)
	}
	if info, err := os.Stat(filepath.Join(dst, "a", "b", "f")); err != nil || info.Mode().Perm() != 0o640 && runtime.GOOS != "windows" {
		t.Errorf("mode = %v (%v), want 0640", info.Mode().Perm(), err)
	}
}

func statOf(t *testing.T, p string) os.FileInfo {
	t.Helper()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return info
}
