package api

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// offsiteReadableFakeEngine embeds ResticEngine (left nil) and overrides only
// what copyToOffsiteTarget's path touches — same pattern as
// offsite_progress_heartbeat_internal_test.go. Copy writes a repo tree with the
// modes restic ACTUALLY produces on a local backend (verified against restic
// 0.17.3: dirs 0700, pack/config/key files 0400), which is the whole point of
// the fix under test.
type offsiteReadableFakeEngine struct {
	ResticEngine
	copyDest string
}

func (f *offsiteReadableFakeEngine) RepoOpens(context.Context, string, restic.Mode) bool { return true }

func (f *offsiteReadableFakeEngine) Unlock(context.Context, string, bool, restic.Mode) error {
	return nil
}

func (f *offsiteReadableFakeEngine) Snapshots(context.Context, string, restic.Mode) ([]restic.Snapshot, error) {
	return nil, nil
}

func (f *offsiteReadableFakeEngine) Copy(_ context.Context, dest, _ string, _ []string, _ restic.Limits, _ restic.Mode) error {
	f.copyDest = dest
	shard := filepath.Join(dest, "data", "00")
	if err := os.MkdirAll(shard, 0o700); err != nil { //nolint:gosec // G301: deliberately reproducing restic's root-only 0700 tree
		return err
	}
	for _, f := range []string{
		filepath.Join(dest, "config"),
		filepath.Join(shard, "cafebabe"),
	} {
		if err := os.WriteFile(f, []byte("restic"), 0o400); err != nil {
			return err
		}
	}
	return nil
}

// A replicated off-site repo that lands on a mounted share must end up readable
// by the share's OTHER clients, not just by the root process that wrote it.
//
// restic writes a local repo 0700/0400, and until this fix copyToOffsiteTarget
// left it that way — while every local backup has always run makeRepoReadable
// over the PRIMARY repo for exactly this reason. On an Unassigned-Devices NFS
// mount the asymmetry stays invisible (the host reads the share as uid 0 and
// walks straight through the 0700 dirs); switch the same share to SMB and the
// CIFS session authenticates as an ordinary user the far side refuses, so the
// repo folders list but their contents do not (bombvault#138 follow-up,
// reported by manilx). Pins the destination tree as group+other readable.
func TestCopyToOffsiteTargetMakesDestinationReadable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits are not modelled on windows")
	}

	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := store.New(db)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	// A local/mounted-share destination: a path RELATIVE to the Host Data mount,
	// which is what the v7.10.0 off-site wizard now accepts (#138).
	root := t.TempDir()
	fake := &offsiteReadableFakeEngine{}
	svc := &Service{store: st, engine: fake, progress: progress.NewStore()}
	svc.cfg.HostMountRoot = root

	target := store.OffsiteTarget{ID: "t1", Domain: "containers", Repo: "remotes/nas/bombvault", Enabled: true}
	if err := svc.copyToOffsiteTarget(context.Background(), "containers", settings, target, filepath.Join(root, "local"), false); err != nil {
		t.Fatalf("copyToOffsiteTarget: %v", err)
	}

	dest := filepath.Join(root, "remotes", "nas", "bombvault")
	if fake.copyDest != dest {
		t.Fatalf("copy destination = %q, want %q", fake.copyDest, dest)
	}
	if err := filepath.WalkDir(dest, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		perm := info.Mode().Perm()
		want := fs.FileMode(0o044) // group+other read
		if d.IsDir() {
			want |= 0o011 // group+other traverse
		}
		if perm&want != want {
			return &modeErr{path: p, perm: perm, want: want}
		}
		return nil
	}); err != nil {
		t.Fatalf("off-site replica must be readable off-box: %v", err)
	}
}

// modeErr reports the first entry of the replicated tree that stayed root-only.
type modeErr struct {
	path string
	perm fs.FileMode
	want fs.FileMode
}

func (e *modeErr) Error() string {
	return e.path + " has perm " + e.perm.String() + ", missing " + e.want.String()
}

// A REMOTE off-site destination has no local tree to relax; the guard must skip
// it rather than let filepath.WalkDir loose on a repo URL.
func TestMakeOffsiteRepoReadableSkipsRemote(t *testing.T) {
	for _, repo := range []string{
		"rest:http://192.168.1.2:8000/containers",
		"s3:s3.amazonaws.com/bucket/containers",
		"sftp:user@host:/srv/containers",
	} {
		if !restic.IsRemoteRepo(repo) {
			t.Fatalf("%q must be recognised as a remote repo", repo)
		}
		makeOffsiteRepoReadable(repo) // must not panic, must not touch the filesystem
	}
}

// A destination that is not there yet — EnsureRepo failed because the share had
// not mounted (#55) — must be a silent no-op, never a panic or a stray mkdir.
func TestMakeOffsiteRepoReadableToleratesMissingPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "never-mounted")
	makeOffsiteRepoReadable(missing)
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("relax pass must not create the destination, stat err = %v", err)
	}
}
