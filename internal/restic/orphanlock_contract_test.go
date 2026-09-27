package restic

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// A restic killed while it holds a lock leaves the lock file behind. Once this
// process counts as started after it, the lock is an earlier run's and goes.
func TestRemoveOrphanLocksClearsTheLockOfAKilledRestic(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("reads /proc")
	}
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}
	repo := filepath.Join(t.TempDir(), "repo")
	r := Restic{Bin: "restic"}
	m := Mode{}
	ctx := context.Background()
	if err := r.Init(ctx, repo, m); err != nil {
		t.Fatal(err)
	}

	// backup --stdin holds its lock for as long as its input stays open.
	cmd := exec.Command("restic", "-r", repo, "--insecure-no-password", "backup", "--stdin") //nolint:gosec // G204: repo is the test's own temp dir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	locks := filepath.Join(repo, "locks")
	for i := 0; ; i++ {
		if entries, _ := os.ReadDir(locks); len(entries) > 0 {
			break
		}
		if i > 100 {
			t.Fatal("restic took no lock")
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	_ = stdin.Close()

	if err := r.removeOrphanLocks(ctx, repo, m, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(locks); len(entries) != 1 {
		t.Fatalf("a lock taken after the cut-off must stay, locks dir has %d entries", len(entries))
	}
	if err := r.removeOrphanLocks(ctx, repo, m, time.Now()); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(locks); len(entries) != 0 {
		t.Fatalf("the killed restic's lock is still there: %d entries", len(entries))
	}
}
