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

// writeMountinfo writes a /proc/self/mountinfo-format fixture whose mount points
// are the given directories, points the api package at it, and restores the
// previous source on cleanup. Paths are used verbatim as field 5 (the mount
// point) so a test can control exactly which directories look "mounted".
func writeMountinfo(t *testing.T, mountPoints ...string) {
	t.Helper()
	var b strings.Builder
	for i, mp := range mountPoints {
		// id parent major:minor root MOUNT-POINT opts... - fstype source superopts
		fmt.Fprintf(&b, "%d 1 0:%d / %s rw,relatime shared:%d - xfs /dev/sd%c rw\n", 36+i, 10+i, mp, i+1, 'a'+i)
	}
	path := filepath.Join(t.TempDir(), "mountinfo")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(api.SetMountinfoPath(path))
}

// slashRepo is the form destinationMounted compares against (kernel paths use
// forward slashes), so a mounted-fixture must list the repo in this form.
func slashRepo(repo string) string { return filepath.ToSlash(repo) }

// TestParseMountedDirs checks the mountinfo parser: it collects field-5 mount
// points and octal-unescapes spaces in them.
func TestParseMountedDirs(t *testing.T) {
	const fixture = "36 1 0:1 / / rw - xfs /dev/sda rw\n" +
		"41 36 0:2 / /mnt/disks/rolob-dev rw shared:1 - xfs /dev/sdb rw\n" +
		"42 36 0:3 / /mnt/disks/my\\040disk rw - xfs /dev/sdc rw\n" +
		"garbage line too short\n"
	got := api.ParseMountedDirs(strings.NewReader(fixture))
	for _, want := range []string{"/", "/mnt/disks/rolob-dev", "/mnt/disks/my disk"} {
		if !got[want] {
			t.Errorf("expected mount point %q in set %v", want, got)
		}
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 mount points, got %d: %v", len(got), got)
	}
}

// TestDestinationMountedAncestor checks the nearest-ancestor semantics: the repo
// itself or any ancestor being a mount point counts as mounted; a subdir of a
// mount counts; a path with no mounted ancestor does not.
func TestDestinationMountedAncestor(t *testing.T) {
	writeMountinfo(t, "/mnt/disks/rolob-dev")

	if !api.DestinationMounted("/mnt/disks/rolob-dev") {
		t.Error("the mount point itself must count as mounted")
	}
	if !api.DestinationMounted("/mnt/disks/rolob-dev/bombvault/containers") {
		t.Error("a subdir of a mount must count as mounted")
	}
	if api.DestinationMounted("/mnt/user/backups") {
		t.Error("a path with no mounted ancestor must NOT count as mounted")
	}
}

// TestDestinationMountedReadErrorIsNotMounted checks the conservative fallback:
// if the mount table cannot be read, the destination is treated as NOT mounted
// so the #55 protection still fires.
func TestDestinationMountedReadErrorIsNotMounted(t *testing.T) {
	t.Cleanup(api.SetMountinfoPath(filepath.Join(t.TempDir(), "does-not-exist")))
	if api.DestinationMounted("/mnt/disks/rolob-dev/backups") {
		t.Error("an unreadable mount table must be treated as NOT mounted")
	}
}

// #55: once a repo has been established at a local destination, a later failure
// to find its `config` while the backing store is genuinely NOT mounted must NOT
// trigger a re-init (that would write an empty repo shadowing the real backups).
// It must return ErrBackupPathNotMounted instead. A genuinely new location inits.
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
	// The backing store is NOT mounted: the fixture lists an unrelated mount only,
	// so neither the repo nor any ancestor is reported as a mount point.
	writeMountinfo(t, "/some/other/mount")

	// Establish it: a `config` marker makes RepoOpens true → EnsureRepo records it.
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := svc.EnsureRepo(context.Background(), repo, mode); err != nil {
		t.Fatalf("establishing an existing repo should succeed: %v", err)
	}
	if len(eng.inited) != 0 {
		t.Fatalf("opening an existing repo must not init, got %v", eng.inited)
	}

	// The backing share vanishes (late mount at boot): the config is gone.
	if err := os.Remove(filepath.Join(repo, "config")); err != nil {
		t.Fatal(err)
	}
	if err := svc.EnsureRepo(context.Background(), repo, mode); !errors.Is(err, api.ErrBackupPathNotMounted) {
		t.Fatalf("a vanished established repo (not mounted) must return ErrBackupPathNotMounted, got %v", err)
	}
	if len(eng.inited) != 0 {
		t.Fatalf("must NOT re-init an established-but-unmounted repo, got inits %v", eng.inited)
	}
	// The marker must survive (nothing to clear — the store is genuinely gone).
	if ok, _ := st.IsRepoEstablished(repo); !ok {
		t.Fatalf("the established marker must be kept while the store is unmounted")
	}

	// A genuinely new location (never established) still initialises normally.
	fresh := filepath.Join(dir, "fresh")
	if err := svc.EnsureRepo(context.Background(), fresh, mode); err != nil {
		t.Fatalf("a fresh location should init, got %v", err)
	}
	if len(eng.inited) != 1 || eng.inited[0] != fresh {
		t.Fatalf("the fresh location should have been inited once, got %v", eng.inited)
	}
}

// #120: a stale/phantom established marker on a destination that IS mounted (a UD
// disk that mounted after Docker, shadowing a pre-mount phantom init) must be
// cleared and the repo re-established on the live disk, not surface a spurious
// "not mounted" error.
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
	// A permanent phantom marker exists, but there is no config on the live disk.
	if err := st.MarkRepoEstablished(repo); err != nil {
		t.Fatal(err)
	}
	// The destination IS mounted (the repo itself is a mount point in the fixture).
	writeMountinfo(t, slashRepo(repo))

	if err := svc.EnsureRepo(context.Background(), repo, mode); err != nil {
		t.Fatalf("a mounted disk with a stale marker must re-init, got %v", err)
	}
	if len(eng.inited) != 1 || eng.inited[0] != repo {
		t.Fatalf("the live disk should have been (re-)inited once, got %v", eng.inited)
	}
	// It is established again (the fresh init wrote a config → RepoOpens → re-mark).
	if ok, _ := st.IsRepoEstablished(repo); !ok {
		t.Fatalf("the repo should be established again after re-init on the live disk")
	}
}

// The snapshot-list gate: a missing local repo that was established returns an
// empty list (not an error) when the destination is mounted, and
// ErrBackupPathNotMounted when it is not.
func TestSnapshotsGateEmptyWhenMountedErrorWhenNot(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})
	mode := restic.Mode{Encrypted: true, Password: "pw"}

	repo := filepath.Join(dir, "repo") // exists, but has no `config` → localRepoMissing
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := st.MarkRepoEstablished(repo); err != nil {
		t.Fatal(err)
	}

	// Mounted → empty list, no error.
	writeMountinfo(t, slashRepo(repo))
	snaps, err := svc.SnapshotsForTag(context.Background(), repo, mode, "container:x")
	if err != nil {
		t.Fatalf("mounted destination must yield an empty list, got error %v", err)
	}
	if len(snaps) != 0 {
		t.Fatalf("expected no snapshots, got %v", snaps)
	}

	// Not mounted → ErrBackupPathNotMounted.
	writeMountinfo(t, "/some/other/mount")
	if _, err := svc.SnapshotsForTag(context.Background(), repo, mode, "container:x"); !errors.Is(err, api.ErrBackupPathNotMounted) {
		t.Fatalf("unmounted destination must return ErrBackupPathNotMounted, got %v", err)
	}
}
