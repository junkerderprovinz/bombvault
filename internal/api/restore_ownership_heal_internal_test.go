package api

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// restic restores a subtree's contents with their recorded metadata but not the
// subtree root itself, so the heal reads the root's node via LsPath and applies
// its mode to the target. The chown goes to the test's own uid and gid, which
// needs no CAP_CHOWN, so this runs in CI without root.
func TestHealRestoreDirOwnershipAppliesSnapshotMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix mode/owner bits are not modelled on windows")
	}
	target := t.TempDir()
	if err := os.Chmod(target, 0o755); err != nil { //nolint:gosec // G302: test-only, seeding a starting mode the heal must then correct
		t.Fatalf("seed target mode: %v", err)
	}

	const subtree = "/host/user/zfs/appdata/SnapOtter/conf"
	// restic ls --json carries the type bits (ModeDir) next to the permission
	// bits, so the heal has to mask with Perm. 0o700 differs from the seeded
	// 0o755, so the assertion shows the chmod happened.
	const wantPerm = 0o700
	eng := &foreignRecordingEngine{lsPathEntries: []restic.FileEntry{
		{Path: subtree, Type: "dir", Uid: os.Getuid(), Gid: os.Getgid(), Mode: uint32(fs.ModeDir | wantPerm)},
	}}
	s := vmRestoreSvc(t, eng)

	s.healRestoreDirOwnership(context.Background(), "repo", "snap123", restic.Mode{}, []backup.RestoreDir{
		{Subtree: subtree, Target: target},
	})

	if len(eng.lsPathCalls) != 1 || eng.lsPathCalls[0] != subtree {
		t.Fatalf("LsPath calls = %v, want [%s]", eng.lsPathCalls, subtree)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}
	if got := info.Mode().Perm(); got != wantPerm {
		t.Fatalf("target mode = %o, want %o (heal did not apply the snapshot's recorded mode)", got, wantPerm)
	}
}

// The heal runs after the restore has succeeded, so it is best effort per
// directory: one failing LsPath must not stop the others.
func TestHealRestoreDirOwnershipContinuesPastAnErroringDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix mode/owner bits are not modelled on windows")
	}
	targetA := t.TempDir()
	targetB := t.TempDir()
	if err := os.Chmod(targetB, 0o755); err != nil { //nolint:gosec // G302: test-only, seeding a starting mode the heal must then correct
		t.Fatalf("seed target B mode: %v", err)
	}

	const subtreeA = "/host/user/zfs/appdata/First/conf"
	const subtreeB = "/host/user/zfs/appdata/Second/conf"
	const wantPermB = 0o750
	// The fake fails every LsPath call, so the check is that both directories
	// are still visited.
	eng := &foreignRecordingEngine{lsPathErr: errors.New("repo busy")}
	s := vmRestoreSvc(t, eng)

	s.healRestoreDirOwnership(context.Background(), "repo", "snap123", restic.Mode{}, []backup.RestoreDir{
		{Subtree: subtreeA, Target: targetA},
		{Subtree: subtreeB, Target: targetB},
	})

	if len(eng.lsPathCalls) != 2 {
		t.Fatalf("LsPath calls = %v, want 2 calls (one per dir, error on one must not skip the other)", eng.lsPathCalls)
	}
	info, err := os.Stat(targetB)
	if err != nil {
		t.Fatalf("stat target B: %v", err)
	}
	if got := info.Mode().Perm(); got == wantPermB {
		t.Fatalf("target B mode = %o, an all-erroring LsPath must never apply a mode", got)
	}
}

// Without a node for the subtree root itself the heal leaves the target alone
// instead of borrowing another entry's metadata.
func TestHealRestoreDirOwnershipSkipsWhenNoMatchingEntry(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix mode/owner bits are not modelled on windows")
	}
	target := t.TempDir()
	if err := os.Chmod(target, 0o755); err != nil { //nolint:gosec // G302: test-only, seeding a starting mode the heal must then correct
		t.Fatalf("seed target mode: %v", err)
	}
	const subtree = "/host/user/zfs/appdata/SnapOtter/conf"

	eng := &foreignRecordingEngine{lsPathEntries: []restic.FileEntry{
		{Path: filepath.ToSlash(subtree) + "/settings.ini", Type: "file", Uid: os.Getuid(), Gid: os.Getgid(), Mode: 0o600},
	}}
	s := vmRestoreSvc(t, eng)

	s.healRestoreDirOwnership(context.Background(), "repo", "snap123", restic.Mode{}, []backup.RestoreDir{
		{Subtree: subtree, Target: target},
	})

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Fatalf("target mode = %o, want unchanged 0755 (no matching entry, nothing should have been applied)", got)
	}
}
