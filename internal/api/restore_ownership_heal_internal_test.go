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

// TestHealRestoreDirOwnershipAppliesSnapshotMode is the regression proof for the
// second #125 half: restic's restorer never re-applies a restored subtree
// root's OWN metadata (only its restored contents get the snapshot's mode), so
// a remapped restore's destination root was left at whatever EnsureDirReadable
// set it to (0o755) regardless of what the snapshot actually recorded. This
// proves healRestoreDirOwnership reads the snapshot's own node for the subtree
// root (via LsPath) and re-applies its mode to the real, already-restored
// target directory. Ownership (Lchown) is exercised too, chowning to the
// test's OWN current uid/gid — the one chown any process, privileged or not,
// is always permitted to perform (verified live: an unprivileged chown to the
// SAME uid/gid a file already has succeeds; only chowning to a DIFFERENT uid
// needs CAP_CHOWN) — so the assertion is deterministic in CI without root.
func TestHealRestoreDirOwnershipAppliesSnapshotMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix mode/owner bits are not modelled on windows")
	}
	target := t.TempDir()
	if err := os.Chmod(target, 0o755); err != nil {
		t.Fatalf("seed target mode: %v", err)
	}

	const subtree = "/host/user/zfs/appdata/SnapOtter/conf"
	// restic's own mode encoding: the type bits (ModeDir) ride alongside the
	// permission bits, exactly as observed live against restic 0.17.3's
	// `restic ls --json` output — a caller must mask with .Perm() to get a
	// plain chmod-able value. 0o700 here is deliberately DIFFERENT from the
	// 0o755 EnsureDirReadable seeded, so a passing Chmod is provable.
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

// TestHealRestoreDirOwnershipContinuesPastAnErroringDir proves the heal is
// best-effort per directory: a restore has ALREADY succeeded by the time this
// runs (it is only called after backup.RestoreContainer returns nil), so one
// directory's LsPath failing must not stop the remaining directories from
// being healed, and must never surface as an error to the caller (the
// function returns nothing to fail with).
func TestHealRestoreDirOwnershipContinuesPastAnErroringDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix mode/owner bits are not modelled on windows")
	}
	targetA := t.TempDir()
	targetB := t.TempDir()
	if err := os.Chmod(targetB, 0o755); err != nil {
		t.Fatalf("seed target B mode: %v", err)
	}

	const subtreeA = "/host/user/zfs/appdata/First/conf"
	const subtreeB = "/host/user/zfs/appdata/Second/conf"
	const wantPermB = 0o750
	// lsPathErr fires for EVERY call in this fake (it has no per-dirPath
	// selectivity), so this proves the loop tolerates a totally failing LsPath
	// for one entry and still visits the next — the interesting assertion is
	// that BOTH calls happen (the loop doesn't abort early) and the function
	// doesn't panic despite never getting a usable entry back.
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

// TestHealRestoreDirOwnershipSkipsWhenNoMatchingEntry proves the heal leaves
// the target untouched when the snapshot listing for that path came back but
// contains no node for the subtree root itself (an unexpected snapshot shape),
// rather than guessing at some other entry's metadata.
func TestHealRestoreDirOwnershipSkipsWhenNoMatchingEntry(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix mode/owner bits are not modelled on windows")
	}
	target := t.TempDir()
	if err := os.Chmod(target, 0o755); err != nil {
		t.Fatalf("seed target mode: %v", err)
	}
	const subtree = "/host/user/zfs/appdata/SnapOtter/conf"

	// Only a CHILD entry, no entry whose Path equals the subtree root itself.
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
