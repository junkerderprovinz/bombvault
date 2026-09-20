package restic_test

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// commandRepo builds an empty unencrypted repository for a --stdin-from-command
// test and skips when the real restic binary or a POSIX shell is missing.
func commandRepo(t *testing.T) (restic.Restic, string, restic.Mode) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the dump command is a POSIX shell line")
	}
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}
	repo := filepath.Join(resolved(t, t.TempDir()), "repo")
	r := restic.Restic{Bin: "restic"}
	m := restic.Mode{Encrypted: false}
	if err := r.Init(context.Background(), repo, m); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return r, repo, m
}

func snapshotCount(t *testing.T, r restic.Restic, repo string, m restic.Mode) int {
	t.Helper()
	snaps, err := r.Snapshots(context.Background(), repo, m)
	if err != nil {
		t.Fatalf("Snapshots: %v", err)
	}
	return len(snaps)
}

// TestBackupFromCommandFailingCommandWritesNoSnapshot pins the guarantee the
// dump mechanism rests on: restic saves no snapshot when the command it ran
// exits non-zero, even after that command has written part of its output. Exit
// 3 is the case to watch, because on a file backup restic treats it as a
// warning and keeps the snapshot.
func TestBackupFromCommandFailingCommandWritesNoSnapshot(t *testing.T) {
	r, repo, m := commandRepo(t)

	_, lines, err := r.BackupFromCommand(context.Background(), repo, "/dbdump/pg.sql", []string{"dbdump:pg"},
		[]string{"sh", "-c", "echo early >&2; printf partial; sleep 0.2; exit 3"}, m)
	if err == nil {
		t.Fatal("a command that exited 3 was reported as a successful backup")
	}
	if n := snapshotCount(t, r, repo, m); n != 0 {
		t.Fatalf("%d snapshots in the repository, want none", n)
	}
	// Only the early line is asserted: restic reaps the command as soon as its
	// stdout ends, which can drop whatever it wrote just before exiting.
	if !slicesContain(lines, "subprocess sh: early") {
		t.Fatalf("subprocess lines = %v, want the early stderr line among them", lines)
	}
}

func TestBackupFromCommandEmptyFailingCommandWritesNoSnapshot(t *testing.T) {
	r, repo, m := commandRepo(t)

	if _, _, err := r.BackupFromCommand(context.Background(), repo, "/dbdump/pg.sql", []string{"dbdump:pg"},
		[]string{"sh", "-c", "exit 2"}, m); err == nil {
		t.Fatal("a command that wrote nothing and exited 2 was reported as a successful backup")
	}
	if n := snapshotCount(t, r, repo, m); n != 0 {
		t.Fatalf("%d snapshots in the repository, want none", n)
	}
}

func TestBackupFromCommandRoundTrip(t *testing.T) {
	r, repo, m := commandRepo(t)
	payload := strings.Repeat("CREATE TABLE t (id int);\n", 400)

	sum, _, err := r.BackupFromCommand(context.Background(), repo, "/dbdump/pg.sql", []string{"dbdump:pg", "p1"},
		[]string{"sh", "-c", `printf "%s" "$1"`, "sh", payload}, m)
	if err != nil {
		t.Fatalf("BackupFromCommand: %v", err)
	}
	if sum.SnapshotID == "" {
		t.Fatal("no snapshot id in the summary")
	}
	if sum.TotalBytesProcessed != uint64(len(payload)) {
		t.Fatalf("TotalBytesProcessed = %d, want %d", sum.TotalBytesProcessed, len(payload))
	}

	snaps, err := r.Snapshots(context.Background(), repo, m)
	if err != nil {
		t.Fatalf("Snapshots: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("%d snapshots, want 1", len(snaps))
	}
	if got := snaps[0].Paths; len(got) != 1 || got[0] != "/dbdump/pg.sql" {
		t.Fatalf("paths = %v, want [/dbdump/pg.sql]", got)
	}
	for _, tag := range []string{"dbdump:pg", "p1"} {
		if !slicesContain(snaps[0].Tags, tag) {
			t.Fatalf("tags = %v, want %q among them", snaps[0].Tags, tag)
		}
	}
	if snaps[0].Summary == nil || snaps[0].Summary.TotalBytesProcessed != uint64(len(payload)) {
		t.Fatalf("snapshot summary = %+v, want %d bytes processed", snaps[0].Summary, len(payload))
	}

	var back bytes.Buffer
	if err := r.DumpRaw(context.Background(), repo, sum.SnapshotID, "/dbdump/pg.sql", &back, m); err != nil {
		t.Fatalf("DumpRaw: %v", err)
	}
	if back.String() != payload {
		t.Fatalf("dump came back with %d bytes, want the %d written", back.Len(), len(payload))
	}
}

// TestBackupFromCommandReturnsEarlySubprocessLines covers the line the orphan
// stop depends on: the pid the dump reports before any data flows must still be
// among the returned lines after a megabyte of output.
func TestBackupFromCommandReturnsEarlySubprocessLines(t *testing.T) {
	r, repo, m := commandRepo(t)

	_, lines, err := r.BackupFromCommand(context.Background(), repo, "/dbdump/pg.sql", []string{"dbdump:pg"},
		[]string{"sh", "-c", "echo bombvault-dbdump-pid 7 >&2; head -c 1048576 /dev/zero"}, m)
	if err != nil {
		t.Fatalf("BackupFromCommand: %v", err)
	}
	if !slicesContain(lines, "subprocess sh: bombvault-dbdump-pid 7") {
		t.Fatalf("subprocess lines = %v, want the pid line among them", lines)
	}
}

func slicesContain(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
