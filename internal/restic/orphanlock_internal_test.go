package restic

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestOrphanLockNeedsAnEarlierLockOfThisHostWhoseOwnerIsGone(t *testing.T) {
	started := time.Date(2026, 9, 27, 18, 0, 0, 0, time.UTC)
	before := started.Add(-time.Minute)
	gone := func(int) bool { return false }
	alive := func(int) bool { return true }
	cases := []struct {
		name  string
		lock  lockFile
		owner func(int) bool
		want  bool
	}{
		{"an earlier run's lock whose owner is gone", lockFile{Hostname: "bv", PID: 43, Time: before}, gone, true},
		{"a lock whose owner may still run", lockFile{Hostname: "bv", PID: 43, Time: before}, alive, false},
		{"a lock taken since this run started", lockFile{Hostname: "bv", PID: 43, Time: started.Add(time.Second)}, gone, false},
		{"a lock from another host", lockFile{Hostname: "nas", PID: 43, Time: before}, gone, false},
	}
	for _, c := range cases {
		if got := orphanLock(c.lock, "bv", started, c.owner); got != c.want {
			t.Errorf("%s: orphan = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestOwnerMayLiveTellsAReusedPIDFromItsOwner(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("reads /proc")
	}
	if !ownerMayLive(os.Getppid()) {
		t.Fatal("the parent started before this process, so it may own a lock")
	}
	if ownerMayLive(os.Getpid()) {
		t.Fatal("this process runs restic as children and holds no lock under its own PID")
	}
	if ownerMayLive(1 << 30) {
		t.Fatal("a PID nothing runs under owns nothing")
	}
	// The shell leaves the sleep behind, so it is younger than this process
	// without being its child.
	laterTick()
	out, err := exec.Command("sh", "-c", "sleep 5 >/dev/null 2>&1 & echo $!").Output()
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatal(err)
	}
	if younger, err := os.FindProcess(pid); err == nil {
		defer func() { _ = younger.Kill() }()
	}
	if ownerMayLive(pid) {
		t.Fatal("a process that started after this one cannot own a lock from before it")
	}
}

// restic writes a lock's time from the wall clock, so a clock stepped back
// after this process started dates the lock of a restic it runs right now
// before the start.
func TestOwnerMayLiveKeepsTheLockOfAResticThisProcessRuns(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("reads /proc")
	}
	laterTick()
	child := exec.Command("sleep", "5")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = child.Process.Kill(); _ = child.Wait() }()
	if !ownerMayLive(child.Process.Pid) {
		t.Fatal("a child of this process may hold its lock")
	}
}

func TestRemoveOrphanLocksLeavesTheLockOfAResticThisProcessRuns(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("reads /proc and runs a shell script as restic")
	}
	laterTick()
	child := exec.Command("sleep", "5")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = child.Process.Kill(); _ = child.Wait() }()

	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	id := strings.Repeat("cd", 32)
	lockPath := filepath.Join(repo, "locks", id)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	host, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	lock := `{"time":"2020-01-01T00:00:00Z","hostname":"` + host + `","pid":` + strconv.Itoa(child.Process.Pid) + `}`
	script := "#!/bin/sh\n" +
		"case \"$*\" in\n" +
		"*\"list locks\"*) echo " + id + " ;;\n" +
		"*\"cat lock\"*) echo '" + lock + "' ;;\n" +
		"esac\n"
	bin := filepath.Join(dir, "restic")
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil { //nolint:gosec // G306: the fake restic has to be executable
		t.Fatal(err)
	}

	if err := (Restic{Bin: bin}).removeOrphanLocks(context.Background(), repo, Mode{}, processStart); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("the lock of a running child is gone: %v", err)
	}
}

// laterTick lets a clock tick pass, the unit /proc counts start times in, so a
// process started next reads as younger than this one.
func laterTick() {
	time.Sleep(30 * time.Millisecond)
}
