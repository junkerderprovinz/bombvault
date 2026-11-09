package restic

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

// needsRestic skips a test that runs the restic binary.
func needsRestic(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}
}

// otherInstance is a Restic whose binary runs in a PID namespace of its own,
// as the restic of a second BombVault container does. It has the same
// hostname and cannot see the processes of this one.
func otherInstance(t *testing.T) Restic {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("needs Linux namespaces")
	}
	needsRestic(t)
	var flags []string
	for _, try := range [][]string{{"--pid", "--fork"}, {"--user", "--map-root-user", "--pid", "--fork"}} {
		if exec.Command("unshare", append(try, "true")...).Run() == nil { //nolint:gosec // G204: fixed flags
			flags = try
			break
		}
	}
	if flags == nil {
		t.Skip("unshare cannot make a PID namespace here")
	}
	resticPath, err := exec.LookPath("restic")
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "restic-elsewhere")
	script := "#!/bin/sh\nexec unshare"
	for _, f := range flags {
		script += " " + f
	}
	script += " " + resticPath + " \"$@\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil { //nolint:gosec // G306: the test's own script has to run
		t.Fatal(err)
	}
	return Restic{Bin: bin}
}

// holdLock starts a backup that keeps its lock on repo until the returned
// function kills it, or until the test ends.
func holdLock(t *testing.T, repo string) (kill func()) {
	t.Helper()
	cmd := exec.Command("restic", "-r", repo, "--insecure-no-password", "backup", "--stdin", "--stdin-filename", "held") //nolint:gosec // G204: repo is the test's own temp dir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	kill = func() {
		once.Do(func() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			_ = stdin.Close()
		})
	}
	t.Cleanup(kill)
	waitForLocks(t, repo, 1)
	return kill
}

func waitForLocks(t *testing.T, repo string, n int) {
	t.Helper()
	for i := 0; lockFiles(t, filepath.Join(repo, "locks")) < n; i++ {
		if i > 100 {
			t.Fatal("restic took no lock")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func newRepo(t *testing.T) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	if err := (Restic{Bin: "restic"}).Init(context.Background(), repo, Mode{}); err != nil {
		t.Fatal(err)
	}
	return repo
}

// Two installs on one repository share the hostname bombvault. The second
// cannot see the first one's restic, and restic's own stale test would call
// that restic's lock left behind.
func TestUnlockLeavesALockThatAnotherInstanceStillHolds(t *testing.T) {
	other := otherInstance(t)
	repo := newRepo(t)
	holdLock(t, repo)

	if err := other.Unlock(context.Background(), repo, false, Mode{}); err != nil {
		t.Fatal(err)
	}
	if n := lockFiles(t, filepath.Join(repo, "locks")); n != 1 {
		t.Fatalf("the lock of a backup that is still running went: %d locks left", n)
	}
}

func TestUnlockRemovingEveryLockRefusesALockAnotherInstanceStillHolds(t *testing.T) {
	other := otherInstance(t)
	repo := newRepo(t)
	holdLock(t, repo)

	if err := other.Unlock(context.Background(), repo, true, Mode{}); !errors.Is(err, ErrLockHeld) {
		t.Fatalf("err = %v, want ErrLockHeld", err)
	}
	if n := lockFiles(t, filepath.Join(repo, "locks")); n != 1 {
		t.Fatalf("the lock of a backup that is still running went: %d locks left", n)
	}
}

// A lock that has gone 30 minutes without a refresh goes whoever wrote it,
// which is the one rule of restic's that needs no hostname.
func TestUnlockRemovesALockUnrefreshedForHalfAnHour(t *testing.T) {
	needsRestic(t)
	repo := newRepo(t)
	holdLock(t, repo)()

	r, ctx := Restic{Bin: "restic"}, context.Background()
	// Started before the lock was taken, so it is no orphan of this process.
	started := time.Now().Add(-time.Hour)
	if err := r.clearLocks(ctx, repo, false, Mode{}, started, time.Now().Add(staleLockTimeout-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if n := lockFiles(t, filepath.Join(repo, "locks")); n != 1 {
		t.Fatalf("a lock refreshed 29 minutes ago went: %d locks left", n)
	}
	if err := r.clearLocks(ctx, repo, false, Mode{}, started, time.Now().Add(staleLockTimeout+time.Minute)); err != nil {
		t.Fatal(err)
	}
	if n := lockFiles(t, filepath.Join(repo, "locks")); n != 0 {
		t.Fatalf("a lock unrefreshed for 31 minutes is still there: %d locks", n)
	}
}
