package api

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// makeRepoReadable skips the files of every directory whose mtime is older than
// the last clean pass. That catches every file restic writes, because a new
// file changes its directory's mtime. It misses a file that something else
// chmods without touching the directory; the stamp expiry bounds that to a day.
// The subtests pin both sides so the miss is not mistaken for a bug.
//
// All times are set with Chtimes rather than waited for: tmpfs stamps a
// directory at the timer tick, so two quick writes can get the same mtime.
func TestMakeRepoReadableScopesToChangedDirectories(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits are not modelled on windows")
	}

	newRepo := func(t *testing.T) (repo, stampDir, dirA, dirB string) {
		t.Helper()
		repo = t.TempDir()
		stampDir = t.TempDir()
		dirA = filepath.Join(repo, "data", "aa")
		dirB = filepath.Join(repo, "data", "bb")
		for _, d := range []string{dirA, dirB} {
			if err := os.MkdirAll(d, 0o700); err != nil {
				t.Fatal(err)
			}
		}
		return repo, stampDir, dirA, dirB
	}

	write := func(t *testing.T, dir, name string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("pack"), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}

	perm := func(t *testing.T, p string) os.FileMode {
		t.Helper()
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		return fi.Mode().Perm()
	}

	setMtime := func(t *testing.T, p string, at time.Time) {
		t.Helper()
		if err := os.Chtimes(p, at, at); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("a file written after the last pass is still relaxed", func(t *testing.T) {
		repo, stampDir, dirA, dirB := newRepo(t)
		write(t, dirA, "first")
		makeRepoReadable(repo, stampDir)

		// Move the last pass an hour back, then write a pack as restic would.
		setMtime(t, permStampPath(stampDir, repo), time.Now().Add(-time.Hour))
		fresh := write(t, dirB, "second")
		makeRepoReadable(repo, stampDir)

		if got := perm(t, fresh); got&0o044 != 0o044 {
			t.Errorf("a pack written after the last pass has perm %o and was not relaxed.\n"+
				"This is the case the pass exists for: adding a file updates its directory's\n"+
				"mtime, so the next pass must stat the files in that directory.", got)
		}
	})

	t.Run("a new file in a deeper directory is relaxed, because mtime does not propagate", func(t *testing.T) {
		// Writing data/bb/x updates bb but not data, so a pass that skipped an
		// old-looking data/ wholesale would never reach bb again.
		repo, stampDir, dirA, dirB := newRepo(t)
		write(t, dirA, "first")
		makeRepoReadable(repo, stampDir)

		hour := time.Now().Add(-time.Hour)
		setMtime(t, permStampPath(stampDir, repo), hour)
		deep := write(t, dirB, "second")
		// In a real repository data/ only changes when a new two-character prefix
		// appears. Backdate it after the write, which moved the parent's mtime.
		setMtime(t, filepath.Join(repo, "data"), hour.Add(-time.Hour))
		setMtime(t, repo, hour.Add(-time.Hour))

		makeRepoReadable(repo, stampDir)

		if got := perm(t, deep); got&0o044 != 0o044 {
			t.Errorf("perm %o: the pass never reached data/bb because data/ looked unchanged.\n"+
				"A directory's mtime says nothing about its subdirectories, so every directory\n"+
				"must be descended into and only its FILES may be skipped.", got)
		}
	})

	t.Run("an unchanged directory is skipped, which is the trade", func(t *testing.T) {
		repo, stampDir, dirA, _ := newRepo(t)
		p := write(t, dirA, "pack")
		makeRepoReadable(repo, stampDir)
		if got := perm(t, p); got&0o044 != 0o044 {
			t.Fatalf("the first pass must relax everything, perm %o", got)
		}

		// chmod does not change the directory's mtime, so the next pass cannot
		// see it and the file stays 0600.
		if err := os.Chmod(p, 0o600); err != nil {
			t.Fatal(err)
		}
		setMtime(t, dirA, time.Now().Add(-time.Hour))
		makeRepoReadable(repo, stampDir)
		if got := perm(t, p); got&0o044 == 0o044 {
			t.Error("the second pass relaxed a file in an UNCHANGED directory.\n" +
				"That means the mtime shortcut is not in effect and the pass is paying for\n" +
				"an lstat per entry again - the 592 ms this was measured to remove.")
		}
	})

	t.Run("an expired stamp forces a full pass", func(t *testing.T) {
		repo, stampDir, dirA, _ := newRepo(t)
		p := write(t, dirA, "pack")
		makeRepoReadable(repo, stampDir)
		if err := os.Chmod(p, 0o600); err != nil {
			t.Fatal(err)
		}
		setMtime(t, dirA, time.Now().Add(-time.Hour))

		// An expired stamp repairs what the previous subtest shows the shortcut
		// cannot see.
		old := time.Now().Add(-fullSweepAfter - time.Hour)
		setMtime(t, permStampPath(stampDir, repo), old)

		makeRepoReadable(repo, stampDir)
		if got := perm(t, p); got&0o044 != 0o044 {
			t.Errorf("perm %o: a stamp older than fullSweepAfter must be ignored, so the "+
				"pass repairs what the mtime shortcut cannot see", got)
		}
	})

	t.Run("a clean pass stamps and a failed one does not", func(t *testing.T) {
		repo, stampDir, dirA, _ := newRepo(t)
		write(t, dirA, "pack")
		makeRepoReadable(repo, stampDir)
		if _, err := os.Stat(permStampPath(stampDir, repo)); err != nil {
			t.Fatalf("a clean pass must leave a stamp, got %v", err)
		}

		// A pass over a missing repository fails. Stamping it would make the next
		// pass skip directories this one never reached.
		missing := filepath.Join(t.TempDir(), "gone")
		makeRepoReadable(missing, stampDir)
		if _, err := os.Stat(permStampPath(stampDir, missing)); err == nil {
			t.Error("a pass that hit an error still wrote a stamp.\n" +
				"The next pass would then skip directories this one never looked at.")
		}
	})

	t.Run("two repositories do not share a stamp", func(t *testing.T) {
		// A shared stamp would let a pass over one repository mark the other as
		// covered.
		one, stampDir, dirA, _ := newRepo(t)
		two := t.TempDir()
		if err := os.MkdirAll(filepath.Join(two, "data", "aa"), 0o700); err != nil {
			t.Fatal(err)
		}
		write(t, dirA, "pack")
		second := filepath.Join(two, "data", "aa", "pack")
		if err := os.WriteFile(second, []byte("pack"), 0o600); err != nil {
			t.Fatal(err)
		}

		makeRepoReadable(one, stampDir)
		makeRepoReadable(two, stampDir)

		if permStampPath(stampDir, one) == permStampPath(stampDir, two) {
			t.Fatal("two repositories resolved to the same stamp file")
		}
		if got := perm(t, second); got&0o044 != 0o044 {
			t.Errorf("perm %o: the second repository was never relaxed, so a pass over the "+
				"first one had marked it as done", got)
		}
	})
}
