package restic

import (
	"os"
	"os/exec"
	"runtime"
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
	younger := exec.Command("sleep", "5")
	if err := younger.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = younger.Process.Kill(); _ = younger.Wait() }()
	if ownerMayLive(younger.Process.Pid) {
		t.Fatal("a process that started after this one cannot own a lock from before it")
	}
}
