package api

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/spike"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// resticFixture is a Service and a Handler over an in-memory store and the
// restic binary, with every domain's repository below one folder.
type resticFixture struct {
	t    *testing.T
	svc  *Service
	h    *Handler
	st   *store.Repo
	db   *sql.DB
	root string // host mount root, slash-spelled
}

func newResticFixture(t *testing.T) *resticFixture {
	t.Helper()
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("restic not on PATH")
	}
	dir := t.TempDir()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
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
	settings.EncryptionEnabled = true
	settings.ContainersPath, settings.VMsPath, settings.FilesPath = "backups/containers", "backups/vms", "backups/files"
	settings.FlashPath, settings.ConfigPath, settings.ZFSPath = "backups/flash", "backups/config", "backups/zfs"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	root := filepath.ToSlash(dir)
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: root}
	dock := &placementDocker{installed: map[string]bool{}}
	svc := NewService(cfg, st, dock, &placementVirsh{}, restic.Restic{Bin: "restic", CacheDir: filepath.Join(dir, "cache")})
	sched := schedule.New(func(string) error { return nil }, st.ListTargets)
	return &resticFixture{
		t: t, svc: svc, h: NewHandler(cfg, st, dock, svc, sched, spike.DefaultProbes()),
		st: st, db: db, root: root,
	}
}

// do sends one request through the router and returns its JSON body. Anything
// but 200 fails the test.
func (f *resticFixture) do(method, path string, body any) map[string]any {
	f.t.Helper()
	return doJSON(f.t, f.h, method, path, body)
}

// write puts files below the source folder dir, which is relative to the
// fixture's sources.
func (f *resticFixture) write(dir string, files map[string]string) {
	f.t.Helper()
	for name, body := range files {
		p := filepath.Join(filepath.FromSlash(f.root), "src", filepath.FromSlash(dir), filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			f.t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			f.t.Fatal(err)
		}
	}
}

// repoOf is the domain's own repository and the mode that opens it.
func (f *resticFixture) repoOf(domain string) (string, restic.Mode) {
	f.t.Helper()
	settings, err := f.st.GetSettings()
	if err != nil {
		f.t.Fatal(err)
	}
	repo, err := f.svc.repoFor(settings, domain, "local")
	if err != nil {
		f.t.Fatal(err)
	}
	return repo, f.svc.repoModeFor(settings, domain, "local", repo)
}

// backup takes a snapshot of the source folder dir into the domain's
// repository, with the folder's entries at the snapshot root.
func (f *resticFixture) backup(domain, dir string, tags ...string) restic.Snapshot {
	f.t.Helper()
	ctx := context.Background()
	repo, mode := f.repoOf(domain)
	if localRepoMissing(repo) {
		if err := f.svc.engine.Init(ctx, repo, mode); err != nil {
			f.t.Fatalf("init %s: %v", domain, err)
		}
	}
	src := filepath.Join(filepath.FromSlash(f.root), "src", filepath.FromSlash(dir))
	sum, err := f.svc.engine.BackupDir(ctx, repo, src, tags, mode)
	if err != nil {
		f.t.Fatalf("backup %s: %v", dir, err)
	}
	return f.snapshot(domain, sum.SnapshotID)
}

// snapshot reads one snapshot of the domain's repository as it is listed. A tag
// rewrites a snapshot under a new id, so key is matched against the id it had
// first as well.
func (f *resticFixture) snapshot(domain, key string) restic.Snapshot {
	f.t.Helper()
	repo, mode := f.repoOf(domain)
	snaps, err := f.svc.engine.Snapshots(context.Background(), repo, mode)
	if err != nil {
		f.t.Fatal(err)
	}
	for _, sn := range snaps {
		if restic.Identity(sn) == key || sn.ID == key {
			return sn
		}
	}
	f.t.Fatalf("no snapshot %s in the %s repository", key, domain)
	return restic.Snapshot{}
}

// zfsEntry adds a ZFS entry for the root dataset.
func (f *resticFixture) zfsEntry(dataset string) store.ZFSDataset {
	f.t.Helper()
	d, err := f.st.CreateZFSDataset(store.ZFSDataset{Dataset: dataset, Enabled: true})
	if err != nil {
		f.t.Fatal(err)
	}
	return d
}

// zfsBackup takes one run's snapshot of a dataset: a folder named like the
// host snapshot the run read, tagged with the dataset.
func (f *resticFixture) zfsBackup(dataset, stamp string, files map[string]string) restic.Snapshot {
	f.t.Helper()
	dir := dataset + "/.zfs/snapshot/" + stamp
	f.write(dir, files)
	return f.backup(zfsDomain, dir, "zfs:"+dataset)
}

// skipWithoutSnapshotStamps skips a test that tells ZFS runs apart. restic
// records the folder a snapshot was read from in the spelling of the host, and
// a run's stamp is read from the slash-spelled one.
func skipWithoutSnapshotStamps(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("a snapshot path recorded on Windows carries no ZFS snapshot stamp")
	}
}
