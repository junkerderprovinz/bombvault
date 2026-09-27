package restic

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// restic's own stale test sends SIGHUP to the PID a lock names, so an orphan
// whose PID now belongs to someone else, this process included, has to be gone
// before `restic unlock` runs.
func TestUnlockRemovesOrphansBeforeResticSignalsTheirPID(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("reads /proc and runs a shell script as restic")
	}
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	id := strings.Repeat("ab", 32)
	if err := os.MkdirAll(filepath.Join(repo, "locks"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "locks", id), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	host, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(dir, "calls")
	lock := `{"time":"2020-01-01T00:00:00Z","hostname":"` + host + `","pid":` + strconv.Itoa(os.Getpid()) + `}`
	script := "#!/bin/sh\n" +
		"case \"$*\" in\n" +
		"*\"list locks\"*) echo list >> " + log + "; echo " + id + " ;;\n" +
		"*\"cat lock\"*) echo cat >> " + log + "; echo '" + lock + "' ;;\n" +
		"*unlock*) if [ -e " + filepath.Join(repo, "locks", id) + " ]; then echo unlock-with-orphan >> " + log + "; else echo unlock >> " + log + "; fi ;;\n" +
		"esac\n"
	bin := filepath.Join(dir, "restic")
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil { //nolint:gosec // G306: the fake restic has to be executable
		t.Fatal(err)
	}

	if err := (Restic{Bin: bin}).Unlock(context.Background(), repo, false, Mode{}); err != nil {
		t.Fatal(err)
	}
	calls, err := os.ReadFile(log) //nolint:gosec // G304: the test's own temp file
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Fields(string(calls)); strings.Join(got, " ") != "list cat unlock" {
		t.Fatalf("calls = %v, want the orphan removed before restic unlock", got)
	}
}
