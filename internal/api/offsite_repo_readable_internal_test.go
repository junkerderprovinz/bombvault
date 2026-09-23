package api

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// offsiteReadableFakeEngine implements only what copyToOffsiteTarget reaches.
// Copy writes a tree with the modes restic 0.17.3 uses on a local backend:
// directories 0700, files 0400.
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
	if err := os.MkdirAll(shard, 0o700); err != nil { //nolint:gosec // G301: reproduces restic's root-only 0700 tree
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

// A replica on a mounted share must be readable by the share's other clients,
// as makeRepoReadable does for local backups. Over NFS the host reads as uid 0
// and never notices, but an SMB session authenticates as an ordinary user and
// cannot read into restic's 0700 directories.
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

	// A share destination, given relative to the host data mount.
	root := t.TempDir()
	fake := &offsiteReadableFakeEngine{}
	svc := &Service{store: st, engine: fake, progress: progress.NewStore()}
	svc.cfg.HostMountRoot = root

	target := store.OffsiteTarget{ID: "t1", Domain: "containers", Repo: "remotes/nas/bombvault", Enabled: true}
	if err := svc.copyToOffsiteTarget(context.Background(), "containers", settings, target, []domainRepoRef{ownRef(filepath.Join(root, "local"))}, nil, false, time.Now().Unix(), nil, targetVisit{}); err != nil {
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

// A remote destination has no local tree, so WalkDir must not run on its URL.
func TestMakeOffsiteRepoReadableSkipsRemote(t *testing.T) {
	for _, repo := range []string{
		"rest:http://192.168.1.2:8000/containers",
		"s3:s3.amazonaws.com/bucket/containers",
		"sftp:user@host:/srv/containers",
	} {
		if !restic.IsRemoteRepo(repo) {
			t.Fatalf("%q must be recognised as a remote repo", repo)
		}
		makeOffsiteRepoReadable(repo, t.TempDir())
	}
}

// When the share has not mounted, the destination does not exist and must not
// be created.
func TestMakeOffsiteRepoReadableToleratesMissingPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "never-mounted")
	makeOffsiteRepoReadable(missing, t.TempDir())
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("relax pass must not create the destination, stat err = %v", err)
	}
}
