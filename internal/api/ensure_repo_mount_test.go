package api_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// hostMountRoot is the broad host bind BombVault runs under (HostSourceRoot
// /mnt). A real /proc/self/mountinfo always lists "/" and this bind, so every
// fixture here does too; neither may count as the backing mount.
const hostMountRoot = "/host/user"

// writeMountinfo writes a mountinfo fixture whose mount points are the given
// paths, verbatim, and points the api package at it until cleanup.
func writeMountinfo(t *testing.T, mountPoints ...string) {
	t.Helper()
	var b strings.Builder
	for i, mp := range mountPoints {
		// id parent major:minor root mountpoint opts... - fstype source superopts
		fmt.Fprintf(&b, "%d 1 0:%d / %s rw,relatime shared:%d - xfs /dev/sd%c rw\n", 36+i, 10+i, mp, i+1, 'a'+i)
	}
	path := filepath.Join(t.TempDir(), "mountinfo")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(api.SetMountinfoPath(path))
}

// slashRepo returns repo with forward slashes, as the kernel lists mount points.
func slashRepo(repo string) string { return filepath.ToSlash(repo) }

// newDiscriminatorSvc builds a Service for DestinationMounted, where only
// cfg.HostMountRoot matters.
func newDiscriminatorSvc(t *testing.T, mountRoot string) *api.Service {
	t.Helper()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: t.TempDir(), HostMountRoot: mountRoot}
	return api.NewService(cfg, newMemStore(t), &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})
}

// The parser collects the mount points (field 5) and unescapes octal spaces.
func TestParseMountedDirs(t *testing.T) {
	const fixture = "36 1 0:1 / / rw - xfs /dev/sda rw\n" +
		"41 36 0:2 / /host/user/disks/rolob-dev rw shared:1 - xfs /dev/sdb rw\n" +
		"42 36 0:3 / /host/user/disks/my\\040disk rw - xfs /dev/sdc rw\n" +
		"garbage line too short\n"
	got := api.ParseMountedDirs(strings.NewReader(fixture))
	for _, want := range []string{"/", "/host/user/disks/rolob-dev", "/host/user/disks/my disk"} {
		if !got[want] {
			t.Errorf("expected mount point %q in set %v", want, got)
		}
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 mount points, got %d: %v", len(got), got)
	}
}

// Only a mount strictly below HostMountRoot counts; "/" and the broad bind do
// not.
func TestDestinationMountedDiscriminator(t *testing.T) {
	svc := newDiscriminatorSvc(t, hostMountRoot)

	// (a) Unmounted share: only "/" and the broad bind are listed.
	writeMountinfo(t, "/", hostMountRoot)
	if svc.DestinationMounted(hostMountRoot + "/disks/X/container") {
		t.Error("(a) an unmounted share (only / and the broad bind present) must NOT count as mounted")
	}

	// (c) Array-default repo: the nearest mount is the broad bind, so it does
	// not count. That errs on the safe side.
	if svc.DestinationMounted(hostMountRoot + "/bombvault/container") {
		t.Error("(c) an array-default path whose nearest mount is the broad bind must NOT count as mounted")
	}
	// The broad bind itself and "/" must never count.
	if svc.DestinationMounted(hostMountRoot) {
		t.Error("HostMountRoot itself must NOT count as a backing mount")
	}

	// (b) Mounted Unassigned Devices share with its own mount below the bind.
	writeMountinfo(t, "/", hostMountRoot, hostMountRoot+"/disks/X")
	if !svc.DestinationMounted(hostMountRoot + "/disks/X/container") {
		t.Error("(b) a subdir of a mounted per-disk share (proper descendant of the broad bind) must count as mounted")
	}
	if !svc.DestinationMounted(hostMountRoot + "/disks/X") {
		t.Error("(b) the per-disk mount point itself must count as mounted")
	}
	// A sibling without a mount of its own stays unmounted.
	if svc.DestinationMounted(hostMountRoot + "/disks/Y/container") {
		t.Error("(b) a sibling share with no per-disk mount must NOT count as mounted")
	}
}

// On TrueNAS and other generic hosts HostSourceRoot and HostMountRoot are the
// same path. The discriminator reads only HostMountRoot, so such an identity
// root behaves like any other.
func TestDestinationMountedDiscriminatorIdentityRoot(t *testing.T) {
	const identityRoot = "/data"
	cfg := config.Config{
		AppKey:         strings.Repeat("a", 64),
		DataDir:        t.TempDir(),
		HostMountRoot:  identityRoot,
		HostSourceRoot: identityRoot,
	}
	svc := api.NewService(cfg, newMemStore(t), &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})

	// Only "/" and the identity bind are listed.
	writeMountinfo(t, "/", identityRoot)
	if svc.DestinationMounted(identityRoot + "/disks/X/container") {
		t.Error("an unmounted share under an identity root must NOT count as mounted")
	}
	if svc.DestinationMounted(identityRoot) {
		t.Error("the identity root itself must NOT count as a backing mount")
	}

	// A per-disk mount below the identity root counts.
	writeMountinfo(t, "/", identityRoot, identityRoot+"/disks/X")
	if !svc.DestinationMounted(identityRoot + "/disks/X/container") {
		t.Error("a subdir of a mounted per-disk share below the identity root must count as mounted")
	}
	if !svc.DestinationMounted(identityRoot + "/disks/X") {
		t.Error("the per-disk mount point itself must count as mounted")
	}
	if svc.DestinationMounted(identityRoot + "/disks/Y/container") {
		t.Error("a sibling share with no per-disk mount must NOT count as mounted")
	}
}

// An unreadable mount table counts as not mounted, so the guard against
// re-initializing a vanished repo still fires.
func TestDestinationMountedReadErrorIsNotMounted(t *testing.T) {
	svc := newDiscriminatorSvc(t, hostMountRoot)
	t.Cleanup(api.SetMountinfoPath(filepath.Join(t.TempDir(), "does-not-exist")))
	if svc.DestinationMounted(hostMountRoot + "/disks/X/container") {
		t.Error("an unreadable mount table must be treated as NOT mounted")
	}
}

// Once a repo is established, a missing `config` on an unmounted backing store
// returns ErrBackupPathNotMounted instead of initializing an empty repo over
// the real backups. A new location still initializes. The fixture lists "/"
// and the broad bind, as in production, but no mount for the repo.
func TestEnsureRepoRefusesReInitWhenEstablishedRepoVanishes(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	eng := &fakeResticEngine{}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)
	mode := restic.Mode{Encrypted: true, Password: "pw"}

	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	writeMountinfo(t, "/", slashRepo(dir))

	// With a `config` present RepoOpens succeeds and EnsureRepo records the repo.
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := svc.EnsureRepo(context.Background(), repo, mode); err != nil {
		t.Fatalf("establishing an existing repo should succeed: %v", err)
	}
	if len(eng.inited) != 0 {
		t.Fatalf("opening an existing repo must not init, got %v", eng.inited)
	}

	// The backing share has not mounted yet at boot, so the config is gone.
	if err := os.Remove(filepath.Join(repo, "config")); err != nil {
		t.Fatal(err)
	}
	if err := svc.EnsureRepo(context.Background(), repo, mode); !errors.Is(err, api.ErrBackupPathNotMounted) {
		t.Fatalf("a vanished established repo (not mounted) must return ErrBackupPathNotMounted, got %v", err)
	}
	if len(eng.inited) != 0 {
		t.Fatalf("must NOT re-init an established-but-unmounted repo, got inits %v", eng.inited)
	}
	if ok, _ := st.IsRepoEstablished(repo); !ok {
		t.Fatalf("the established marker must be kept while the store is unmounted")
	}

	// A location never established still initializes.
	fresh := filepath.Join(dir, "fresh")
	if err := svc.EnsureRepo(context.Background(), fresh, mode); err != nil {
		t.Fatalf("a fresh location should init, got %v", err)
	}
	if len(eng.inited) != 1 || eng.inited[0] != fresh {
		t.Fatalf("the fresh location should have been inited once, got %v", eng.inited)
	}
}

// A stale established marker on a mounted destination (an Unassigned Devices
// disk that mounted after Docker, hiding an init made before it) is cleared
// and the repo is initialized on the live disk.
func TestEnsureRepoReInitsWhenEstablishedRepoIsMountedButMissing(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	eng := &fakeResticEngine{initWritesConfig: true} // a successful init lands a config
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)
	mode := restic.Mode{Encrypted: true, Password: "pw"}

	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	// A marker, but no config on the live disk.
	if err := st.MarkRepoEstablished(repo); err != nil {
		t.Fatal(err)
	}
	// The repo has its own mount below the broad bind.
	writeMountinfo(t, "/", slashRepo(dir), slashRepo(repo))

	if err := svc.EnsureRepo(context.Background(), repo, mode); err != nil {
		t.Fatalf("a mounted disk with a stale marker must re-init, got %v", err)
	}
	if len(eng.inited) != 1 || eng.inited[0] != repo {
		t.Fatalf("the live disk should have been (re-)inited once, got %v", eng.inited)
	}
	// The init wrote a config, so the repo is marked established again.
	if ok, _ := st.IsRepoEstablished(repo); !ok {
		t.Fatalf("the repo should be established again after re-init on the live disk")
	}
}

// An established local repo without `config` lists no snapshots when its
// destination is mounted and returns ErrBackupPathNotMounted when it is not.
func TestSnapshotsGateEmptyWhenMountedErrorWhenNot(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})
	mode := restic.Mode{Encrypted: true, Password: "pw"}

	repo := filepath.Join(dir, "repo") // exists without a `config`
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := st.MarkRepoEstablished(repo); err != nil {
		t.Fatal(err)
	}

	// Mounted.
	writeMountinfo(t, "/", slashRepo(dir), slashRepo(repo))
	snaps, err := svc.SnapshotsForTag(context.Background(), repo, mode, "container:x")
	if err != nil {
		t.Fatalf("mounted destination must yield an empty list, got error %v", err)
	}
	if len(snaps) != 0 {
		t.Fatalf("expected no snapshots, got %v", snaps)
	}

	// Not mounted.
	writeMountinfo(t, "/", slashRepo(dir))
	if _, err := svc.SnapshotsForTag(context.Background(), repo, mode, "container:x"); !errors.Is(err, api.ErrBackupPathNotMounted) {
		t.Fatalf("unmounted destination must return ErrBackupPathNotMounted, got %v", err)
	}
}
