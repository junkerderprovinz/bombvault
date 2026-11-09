package restic

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fakeLockRestic writes a shell script that answers like restic for a
// repository holding the one lock id with the content lock, and records each
// call it gets in the returned log.
func fakeLockRestic(t *testing.T, id, lock string) (bin, log string) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("runs a shell script as restic")
	}
	dir := t.TempDir()
	log = filepath.Join(dir, "calls")
	script := "#!/bin/sh\n" +
		"case \"$*\" in\n" +
		"*\"list locks\"*) echo list >> " + log + "; echo " + id + " ;;\n" +
		"*\"cat lock\"*) echo cat >> " + log + "; echo '" + lock + "' ;;\n" +
		"*--remove-all*) echo unlock-all >> " + log + " ;;\n" +
		"*unlock*) echo unlock >> " + log + " ;;\n" +
		"esac\n"
	bin = filepath.Join(dir, "restic")
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil { //nolint:gosec // G306: the fake restic has to be executable
		t.Fatal(err)
	}
	return bin, log
}

func calls(t *testing.T, log string) string {
	t.Helper()
	raw, err := os.ReadFile(log) //nolint:gosec // G304: the test's own temp file
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return strings.Join(strings.Fields(string(raw)), " ")
}

func lockJSON(at time.Time, host string, pid int) string {
	return `{"time":"` + at.UTC().Format(time.RFC3339) + `","hostname":"` + host + `","pid":` + strconv.Itoa(pid) + `}`
}

// restic's own stale test would also take the lock of a restic it cannot see,
// so a local repository loses its stale locks as files and never to restic.
func TestUnlockOfALocalRepositoryRemovesStaleLocksWithoutResticUnlock(t *testing.T) {
	id := strings.Repeat("ab", 32)
	host, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	bin, log := fakeLockRestic(t, id, lockJSON(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), host, os.Getpid()))
	repo := filepath.Join(t.TempDir(), "repo")
	lockPath := filepath.Join(repo, "locks", id)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := (Restic{Bin: bin}).Unlock(context.Background(), repo, true, Mode{}); err != nil {
		t.Fatal(err)
	}
	if got := calls(t, log); got != "list cat" {
		t.Fatalf("calls = %q, want only the lock read", got)
	}
	if _, err := os.Stat(lockPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the stale lock is still there: %v", err)
	}
}

func TestUnlockOfARemoteRepositoryRunsResticOnlyWhenEveryLockIsStale(t *testing.T) {
	id := strings.Repeat("cd", 32)
	const repo = "rest:http://127.0.0.1:1/repo"
	now := time.Now()

	bin, log := fakeLockRestic(t, id, lockJSON(now.Add(-staleLockTimeout-time.Minute), "nas", 7))
	if err := (Restic{Bin: bin}).Unlock(context.Background(), repo, true, Mode{}); err != nil {
		t.Fatal(err)
	}
	if got := calls(t, log); got != "list cat unlock" {
		t.Fatalf("calls = %q, want restic unlock without --remove-all after the read", got)
	}

	bin, log = fakeLockRestic(t, id, lockJSON(now.Add(-time.Minute), "nas", 7))
	err := (Restic{Bin: bin}).Unlock(context.Background(), repo, true, Mode{})
	if !errors.Is(err, ErrLockHeld) {
		t.Fatalf("err = %v, want ErrLockHeld for a lock refreshed a minute ago", err)
	}
	if got := calls(t, log); got != "list cat" {
		t.Fatalf("calls = %q, want no restic unlock while a lock is held", got)
	}
}
