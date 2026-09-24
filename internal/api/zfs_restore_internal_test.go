package api

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

const (
	zfsStamp       = "bombvault-20260101000000"
	zfsRootSnapID  = "a1a1a1a1"
	zfsChildSnapID = "b2b2b2b2"
	// zfsMountedSub is the folder below the host mount that the fixture's mount
	// table reports as a mount of its own, so a restore may be written into it.
	zfsMountedSub = "pool"
)

// zfsRestoreFixture is a service whose repository, mount table and destination
// folders really exist, which is what a restore reads and writes.
func zfsRestoreFixture(t *testing.T) (*Service, *store.Repo, *fakeZFSHost, *zfsFakeEngine) {
	t.Helper()
	s, st, host, eng := zfsRunFixture(t, zfsTwoDatasetTree())
	root := filepath.ToSlash(t.TempDir())
	s.cfg.HostMountRoot = root
	if err := os.MkdirAll(filepath.Join(root, "repo"), 0o750); err != nil {
		t.Fatalf("create the repository: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "repo", "config"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("mark the repository as established: %v", err)
	}
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	settings.ZFSPath = "repo"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	zfsMountFixture(t, zfsRestoreRecords(root, "rw"), host, nil)
	zfsMountedFixture(t, root)
	eng.snaps = zfsRestorePointSnapshots(root)
	return s, st, host, eng
}

// zfsRestoreRecords is the container's view of the two datasets below root.
func zfsRestoreRecords(root, rootOptions string) []zfs.MountRecord {
	return []zfs.MountRecord{
		{MountPoint: root, Root: "/mnt", FSType: "rootfs", Source: "rootfs", Options: []string{"rw"}, Optional: []string{"master:5"}},
		{MountPoint: zfsMemberPath(root, zfsRoot), Root: "/", FSType: "zfs", Source: zfsRoot, Options: strings.Split(rootOptions, ","), Optional: []string{"master:7"}},
		{MountPoint: zfsMemberPath(root, zfsChild), Root: "/", FSType: "zfs", Source: zfsChild, Options: []string{"rw"}, Optional: []string{"master:8"}},
	}
}

// zfsMemberPath is where the container sees a dataset whose host mountpoint is
// /mnt/<dataset>.
func zfsMemberPath(root, dataset string) string { return root + "/" + dataset }

// zfsMountedFixture writes the mount table destinationMounted reads, with one
// mount below the host mount root that a restore folder may live on. The mount
// point is escaped the way the kernel escapes a path with spaces.
func zfsMountedFixture(t *testing.T, root string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "mountinfo")
	lines := []string{
		"30 1 0:28 / / rw,relatime - overlay overlay rw",
		"40 30 0:29 /mnt " + mountinfoEscape(root) + " rw,relatime master:5 - rootfs rootfs rw",
		"41 40 0:31 / " + mountinfoEscape(root+"/"+zfsMountedSub) + " rw,relatime master:6 - zfs pool rw",
	}
	if err := os.WriteFile(file, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatalf("write the mount table: %v", err)
	}
	previous := mountinfoPath
	t.Cleanup(func() { mountinfoPath = previous })
	mountinfoPath = file
}

func mountinfoEscape(p string) string { return strings.ReplaceAll(p, " ", `\040`) }

// zfsRestorePointSnapshots is one run instant of the two dataset tree, as
// restic reports it.
func zfsRestorePointSnapshots(root string) []restic.Snapshot {
	return []restic.Snapshot{
		{
			ID:    zfsRootSnapID,
			Tags:  []string{"zfs:" + zfsRoot},
			Paths: []string{zfsMemberPath(root, zfsRoot) + "/.zfs/snapshot/" + zfsStamp},
		},
		{
			ID:    zfsChildSnapID,
			Tags:  []string{"zfs:" + zfsChild},
			Paths: []string{zfsMemberPath(root, zfsChild) + "/.zfs/snapshot/" + zfsStamp},
		},
	}
}

// zfsUnmountedTree is the same tree with the root dataset no longer mounted.
func zfsUnmountedTree() []zfs.ListEntry {
	tree := zfsTwoDatasetTree()
	tree[0].Mounted = false
	return tree
}

// zfsAwaitRestore waits for the detached restore of an item to finish and
// returns its run row.
func zfsAwaitRestore(t *testing.T, st *store.Repo, itemID string) store.Run {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		runs, err := st.RecentRunsOfKind(itemID, "restore", 1)
		if err != nil {
			t.Fatalf("read the restore runs: %v", err)
		}
		if len(runs) > 0 && runs[0].Status != "running" {
			return runs[0]
		}
		if time.Now().After(deadline) {
			t.Fatal("the restore did not finish")
		}
		time.Sleep(time.Millisecond)
	}
}

func zfsRestoreRequest(dataset string) ZFSRestoreRequest {
	return ZFSRestoreRequest{Stamp: zfsStamp, Dataset: dataset, Confirm: true, SafetySnapshot: true}
}

func TestRestoreZFSInPlaceRequiresConfirm(t *testing.T) {
	s, st, host, eng := zfsRestoreFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)

	req := zfsRestoreRequest(zfsRoot)
	req.Confirm = false
	_, started, err := s.StartRestoreZFS(context.Background(), d.ID, "local", req)
	if started || !errors.Is(err, backup.ErrNotConfirmed) {
		t.Fatalf("started = %v, err = %v, want a refusal for the missing confirmation", started, err)
	}
	if hostDid(host, "snapshot ") {
		t.Fatalf("an unconfirmed restore touched the host: %v", host.recorded())
	}
	if len(eng.readRestores()) != 0 {
		t.Fatalf("an unconfirmed restore ran restic: %v", eng.readRestores())
	}
}

func TestRestoreZFSInPlaceRefusesUnmountedReadOnlyOrOutsideHostMountRoot(t *testing.T) {
	cases := []struct {
		name  string
		world func(t *testing.T, s *Service, host *fakeZFSHost)
		want  string
	}{
		{
			name: "the dataset is not mounted",
			world: func(_ *testing.T, _ *Service, host *fakeZFSHost) {
				host.tree = zfsUnmountedTree()
			},
			want: "not-mounted",
		},
		{
			name: "the container sees the dataset read only",
			world: func(t *testing.T, s *Service, host *fakeZFSHost) {
				zfsMountFixture(t, zfsRestoreRecords(s.cfg.HostMountRoot, "ro,relatime"), host, nil)
			},
			want: "read-only-mount",
		},
		{
			name: "the only mount lies outside the host mount root",
			world: func(t *testing.T, _ *Service, host *fakeZFSHost) {
				zfsMountFixture(t, []zfs.MountRecord{
					{MountPoint: "/mnt/cache/appdata", Root: "/", FSType: "zfs", Source: zfsRoot, Options: []string{"rw"}},
				}, host, nil)
			},
			want: "not-visible",
		},
		{
			name: "the host refuses BombVault's key",
			world: func(_ *testing.T, _ *Service, host *fakeZFSHost) {
				host.treeErr = &zfs.CmdError{Code: "ssh-auth", Stderr: "root@tower: Permission denied (publickey)."}
			},
			want: "ssh-auth",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, st, host, eng := zfsRestoreFixture(t)
			d := zfsSeedItem(t, st, zfsRoot)
			tc.world(t, s, host)

			_, started, err := s.StartRestoreZFS(context.Background(), d.ID, "local", zfsRestoreRequest(zfsRoot))
			if started {
				t.Fatal("the restore started although the dataset cannot be written")
			}
			if got := zfsCodeOf(t, err); got != tc.want {
				t.Fatalf("code = %q, want %q", got, tc.want)
			}
			if len(eng.readRestores()) != 0 {
				t.Fatalf("restic ran anyway: %v", eng.readRestores())
			}
		})
	}
}

func TestRestoreZFSInPlaceRechecksAfterLock(t *testing.T) {
	s, st, host, eng := zfsRestoreFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)
	listings := 0
	host.onTree = func() {
		listings++
		if listings == 2 {
			host.tree = zfsUnmountedTree()
		}
	}

	if _, started, err := s.StartRestoreZFS(context.Background(), d.ID, "local", zfsRestoreRequest(zfsRoot)); !started || err != nil {
		t.Fatalf("started = %v, err = %v, want the restore to be accepted", started, err)
	}
	run := zfsAwaitRestore(t, st, d.ID)
	if run.Status != "failed" || !strings.Contains(run.Error, "[not-mounted]") {
		t.Fatalf("run = %+v, want a failed run naming not-mounted", run)
	}
	if len(eng.readRestores()) != 0 {
		t.Fatalf("restic ran on a dataset that was unmounted in between: %v", eng.readRestores())
	}
}

func TestRestoreZFSInPlaceOneChildUsesRestoreAll(t *testing.T) {
	s, st, _, eng := zfsRestoreFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)

	if _, started, err := s.StartRestoreZFS(context.Background(), d.ID, "local", zfsRestoreRequest(zfsChild)); !started || err != nil {
		t.Fatalf("started = %v, err = %v", started, err)
	}
	if run := zfsAwaitRestore(t, st, d.ID); run.Status != "success" {
		t.Fatalf("run = %+v, want a successful restore", run)
	}
	want := "RestoreAll|" + zfsChildSnapID + "->" + zfsMemberPath(s.cfg.HostMountRoot, zfsChild)
	if got := strings.Join(eng.readRestores(), " "); got != want {
		t.Fatalf("restic calls = %q, want %q", got, want)
	}
}

func TestRestoreZFSSafetySnapshotCreatedBeforeAckAndRecorded(t *testing.T) {
	s, st, host, _ := zfsRestoreFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)

	ack, started, err := s.StartRestoreZFS(context.Background(), d.ID, "local", zfsRestoreRequest(zfsRoot))
	if !started || err != nil {
		t.Fatalf("started = %v, err = %v", started, err)
	}
	dataset, name, found := strings.Cut(ack.SafetySnapshot, "@")
	if !found || dataset != zfsRoot || !zfs.IsPreRestoreSnapshot(name) {
		t.Fatalf("safety snapshot = %q, want %s@bombvault-prerestore-<stamp>", ack.SafetySnapshot, zfsRoot)
	}
	if !hostDid(host, "snapshot "+ack.SafetySnapshot) {
		t.Fatalf("the safety snapshot was not taken before the answer: %v", host.recorded())
	}
	rows, err := st.ListZFSSafetySnapshots(d.ID)
	if err != nil {
		t.Fatalf("read the safety snapshots: %v", err)
	}
	if len(rows) != 1 || rows[0].Dataset != zfsRoot || rows[0].Name != name {
		t.Fatalf("stored safety snapshots = %+v, want the one the answer named", rows)
	}
	zfsAwaitRestore(t, st, d.ID)
}

func TestRestoreZFSSafetyOffNeedsSecondConfirm(t *testing.T) {
	s, st, host, _ := zfsRestoreFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)

	req := zfsRestoreRequest(zfsRoot)
	req.SafetySnapshot = false
	if _, started, err := s.StartRestoreZFS(context.Background(), d.ID, "local", req); started || err == nil {
		t.Fatalf("started = %v, err = %v, want a refusal without the second confirmation", started, err)
	}
	if hostDid(host, "snapshot ") {
		t.Fatalf("the refused restore touched the host: %v", host.recorded())
	}

	req.SafetyOffConfirm = true
	ack, started, err := s.StartRestoreZFS(context.Background(), d.ID, "local", req)
	if !started || err != nil {
		t.Fatalf("started = %v, err = %v, want the second confirmation to be enough", started, err)
	}
	if ack.SafetySnapshot != "" {
		t.Fatalf("safety snapshot = %q, want none", ack.SafetySnapshot)
	}
	zfsAwaitRestore(t, st, d.ID)
}

func TestRestoreZFSSelectedFilesInPlace(t *testing.T) {
	s, st, _, eng := zfsRestoreFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)

	req := zfsRestoreRequest(zfsRoot)
	req.SafetySnapshot = false
	req.SafetyOffConfirm = true
	req.Paths = []string{"/plex/config.xml"}
	if _, started, err := s.StartRestoreZFS(context.Background(), d.ID, "local", req); !started || err != nil {
		t.Fatalf("started = %v, err = %v", started, err)
	}
	zfsAwaitRestore(t, st, d.ID)
	want := "RestoreInclude|" + zfsRootSnapID + "|/plex/config.xml->" + zfsMemberPath(s.cfg.HostMountRoot, zfsRoot)
	if got := strings.Join(eng.readRestores(), " "); got != want {
		t.Fatalf("restic calls = %q, want %q", got, want)
	}
}

func TestRestoreZFSToFolderWithoutHost(t *testing.T) {
	s, st, _, eng := zfsRestoreFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)
	s.zfs = nil

	req := ZFSRestoreRequest{Stamp: zfsStamp, Dataset: zfsRoot, TargetPath: zfsMountedSub + "/restored"}
	ack, started, err := s.StartRestoreZFS(context.Background(), d.ID, "local", req)
	if !started || err != nil {
		t.Fatalf("started = %v, err = %v, want a restore that needs no host", started, err)
	}
	folder := s.cfg.HostMountRoot + "/" + zfsMountedSub + "/restored"
	if ack.Target != folder {
		t.Fatalf("target = %q, want %q", ack.Target, folder)
	}
	if _, statErr := os.Stat(folder); statErr != nil {
		t.Fatalf("the target folder was not created: %v", statErr)
	}
	zfsAwaitRestore(t, st, d.ID)
	want := "RestoreAll|" + zfsRootSnapID + "->" + folder
	if got := strings.Join(eng.readRestores(), " "); got != want {
		t.Fatalf("restic calls = %q, want %q", got, want)
	}
}

func TestRestoreZFSToFolderRefusesUnmountedDestination(t *testing.T) {
	s, st, _, eng := zfsRestoreFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)

	req := ZFSRestoreRequest{Stamp: zfsStamp, Dataset: zfsRoot, TargetPath: "unmounted/here"}
	_, started, err := s.StartRestoreZFS(context.Background(), d.ID, "local", req)
	if started {
		t.Fatal("the restore started into a folder on the container's own filesystem")
	}
	if got := zfsCodeOf(t, err); got != "destination-not-mounted" {
		t.Fatalf("code = %q, want destination-not-mounted", got)
	}
	if len(eng.readRestores()) != 0 {
		t.Fatalf("restic ran anyway: %v", eng.readRestores())
	}
}

func TestRestoreZFSToFolderRefusesProvenShortfall(t *testing.T) {
	s, st, _, eng := zfsRestoreFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)
	eng.restoreSizeBytes = 4096
	s.diskFree = func(string) (uint64, error) { return 512, nil }

	req := ZFSRestoreRequest{Stamp: zfsStamp, Dataset: zfsRoot, TargetPath: zfsMountedSub + "/restored"}
	_, started, err := s.StartRestoreZFS(context.Background(), d.ID, "local", req)
	if started {
		t.Fatal("the restore started although the destination is too small")
	}
	if got := zfsCodeOf(t, err); got != "not-enough-space" {
		t.Fatalf("code = %q, want not-enough-space", got)
	}

	// A probe that cannot answer is no proof of a shortfall and must not block.
	s.diskFree = func(string) (uint64, error) { return 0, errors.New("statfs: no such file") }
	if _, started, err := s.StartRestoreZFS(context.Background(), d.ID, "local", req); !started || err != nil {
		t.Fatalf("started = %v, err = %v, want an unmeasurable destination to pass", started, err)
	}
	zfsAwaitRestore(t, st, d.ID)
}

func TestRestoreZFSWholeTreeToFolder(t *testing.T) {
	s, st, _, eng := zfsRestoreFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)

	req := ZFSRestoreRequest{Stamp: zfsStamp, WholeTree: true, TargetPath: zfsMountedSub + "/tree"}
	if _, started, err := s.StartRestoreZFS(context.Background(), d.ID, "local", req); !started || err != nil {
		t.Fatalf("started = %v, err = %v", started, err)
	}
	zfsAwaitRestore(t, st, d.ID)
	folder := s.cfg.HostMountRoot + "/" + zfsMountedSub + "/tree"
	want := []string{
		"RestoreAll|" + zfsRootSnapID + "->" + folder,
		"RestoreAll|" + zfsChildSnapID + "->" + folder + "/plex",
	}
	if got := strings.Join(eng.readRestores(), " "); got != strings.Join(want, " ") {
		t.Fatalf("restic calls = %q, want %q", got, strings.Join(want, " "))
	}
}

func TestRestoreZFSRejectsTraversal(t *testing.T) {
	s, st, _, eng := zfsRestoreFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)

	selected := zfsRestoreRequest(zfsRoot)
	selected.Paths = []string{"../../etc/passwd"}
	if _, started, err := s.StartRestoreZFS(context.Background(), d.ID, "local", selected); started || err == nil {
		t.Fatalf("started = %v, err = %v, want a relative selection to be refused", started, err)
	}

	folder := ZFSRestoreRequest{Stamp: zfsStamp, Dataset: zfsRoot, TargetPath: "../escape"}
	if _, started, err := s.StartRestoreZFS(context.Background(), d.ID, "local", folder); started || err == nil {
		t.Fatalf("started = %v, err = %v, want a folder outside the host mount to be refused", started, err)
	}
	if len(eng.readRestores()) != 0 {
		t.Fatalf("restic ran anyway: %v", eng.readRestores())
	}
}

func TestRestoreZFSSnapshotMustBelongToItem(t *testing.T) {
	s, st, _, eng := zfsRestoreFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)

	foreign := zfsRestoreRequest("tank/media")
	if _, started, err := s.StartRestoreZFS(context.Background(), d.ID, "local", foreign); started || err == nil {
		t.Fatalf("started = %v, err = %v, want a dataset of another tree to be refused", started, err)
	}

	unknown := zfsRestoreRequest(zfsRoot)
	unknown.Stamp = "bombvault-20991231235959"
	if _, started, err := s.StartRestoreZFS(context.Background(), d.ID, "local", unknown); started || err == nil {
		t.Fatalf("started = %v, err = %v, want an unknown restore point to be refused", started, err)
	}

	malformed := zfsRestoreRequest(zfsRoot)
	malformed.Stamp = "autosnap_2026"
	if _, started, err := s.StartRestoreZFS(context.Background(), d.ID, "local", malformed); started || err == nil {
		t.Fatalf("started = %v, err = %v, want a foreign snapshot name to be refused", started, err)
	}
	if len(eng.readRestores()) != 0 {
		t.Fatalf("restic ran anyway: %v", eng.readRestores())
	}
}

func TestRestoreZFSNeverRollsBackOrReceives(t *testing.T) {
	s, st, host, _ := zfsRestoreFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)

	ack, started, err := s.StartRestoreZFS(context.Background(), d.ID, "local", zfsRestoreRequest(zfsRoot))
	if !started || err != nil {
		t.Fatalf("started = %v, err = %v", started, err)
	}
	zfsAwaitRestore(t, st, d.ID)
	for _, call := range host.recorded() {
		switch {
		case strings.HasPrefix(call, "list -r "):
		case call == "snapshot "+ack.SafetySnapshot:
		default:
			t.Fatalf("the restore ran %q on the host; only a listing and the safety snapshot are allowed", call)
		}
	}
}

func TestListZFSSafetySnapshotsReconciles(t *testing.T) {
	s, st, host, _ := zfsRestoreFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)
	const kept = "bombvault-prerestore-20260101000000"
	if err := st.UpsertZFSSafetySnapshot(store.ZFSSafetySnapshot{
		ItemID: d.ID, Dataset: zfsRoot, Name: "bombvault-prerestore-20250101000000", CreatedAt: 1,
	}); err != nil {
		t.Fatalf("seed a snapshot that is gone from the pool: %v", err)
	}
	host.snaps = []zfs.SnapshotEntry{
		{Dataset: zfsRoot, Name: kept, Creation: 1767225600, Used: 4096},
		{Dataset: zfsRoot, Name: zfsStamp, Creation: 1767225600},
		{Dataset: zfsChild, Name: "autosnap_2026", Creation: 1767225600},
	}

	got, err := s.ListZFSSafetySnapshots(context.Background(), d.ID)
	if err != nil {
		t.Fatalf("list the safety snapshots: %v", err)
	}
	if len(got) != 1 || got[0].Name != kept || got[0].UsedBytes != 4096 {
		t.Fatalf("listed = %+v, want only the pre-restore snapshot the pool still has", got)
	}
	stored, err := st.ListZFSSafetySnapshots(d.ID)
	if err != nil {
		t.Fatalf("read the stored snapshots: %v", err)
	}
	if len(stored) != 1 || stored[0].Name != kept {
		t.Fatalf("stored = %+v, want the listing to have replaced what the pool no longer has", stored)
	}
}

func TestDeleteZFSSafetySnapshotOnlyPreRestoreNames(t *testing.T) {
	s, st, host, _ := zfsRestoreFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)
	const name = "bombvault-prerestore-20260101000000"

	if err := s.DeleteZFSSafetySnapshot(context.Background(), d.ID, zfsRoot, zfsStamp); err == nil {
		t.Fatal("a backup stamp was accepted as a safety snapshot")
	}
	if err := s.DeleteZFSSafetySnapshot(context.Background(), d.ID, "tank/media", name); err == nil {
		t.Fatal("a dataset outside the item's tree was accepted")
	}
	if hostDid(host, "destroy ") {
		t.Fatalf("a refused delete reached the host: %v", host.recorded())
	}

	if err := st.UpsertZFSSafetySnapshot(store.ZFSSafetySnapshot{
		ItemID: d.ID, Dataset: zfsChild, Name: name, CreatedAt: 1,
	}); err != nil {
		t.Fatalf("seed the safety snapshot: %v", err)
	}
	if err := s.DeleteZFSSafetySnapshot(context.Background(), d.ID, zfsChild, name); err != nil {
		t.Fatalf("delete the safety snapshot: %v", err)
	}
	if !hostDid(host, "destroy "+zfsChild+"@"+name) {
		t.Fatalf("the snapshot was not destroyed: %v", host.recorded())
	}
	rows, err := st.ListZFSSafetySnapshots(d.ID)
	if err != nil {
		t.Fatalf("read the stored snapshots: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("stored = %+v, want the row to be gone", rows)
	}
}

func TestDeleteZFSSafetySnapshotWaitsForNothingWhileTheDomainIsBusy(t *testing.T) {
	s, st, host, _ := zfsRestoreFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)
	release := s.lockDomainFor(zfsDomain, "restore")
	defer release()

	err := s.DeleteZFSSafetySnapshot(context.Background(), d.ID, zfsRoot, "bombvault-prerestore-20260101000000")
	if !errors.Is(err, errDomainBusy) {
		t.Fatalf("err = %v, want the domain reported busy", err)
	}
	if hostDid(host, "destroy ") {
		t.Fatalf("the undo of a running restore was destroyed: %v", host.recorded())
	}
}

func TestListZFSRestorePointsGroupsMembersByRunInstant(t *testing.T) {
	s, st, _, _ := zfsRestoreFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)

	points, err := s.ListZFSRestorePoints(context.Background(), d.ID, "local")
	if err != nil {
		t.Fatalf("list the restore points: %v", err)
	}
	if len(points) != 1 || points[0].Stamp != zfsStamp {
		t.Fatalf("points = %+v, want the one run instant", points)
	}
	stamped, _ := zfs.StampTime(zfsStamp)
	if points[0].Time != stamped.Unix() {
		t.Fatalf("time = %d, want %d", points[0].Time, stamped.Unix())
	}
	got := fmt.Sprintf("%s %s %s|%s %s %s",
		points[0].Members[0].Dataset, points[0].Members[0].RelPath, points[0].Members[0].SnapshotID,
		points[0].Members[1].Dataset, points[0].Members[1].RelPath, points[0].Members[1].SnapshotID)
	want := zfsRoot + " / " + zfsRootSnapID + "|" + zfsChild + " /plex " + zfsChildSnapID
	if got != want {
		t.Fatalf("members = %q, want %q", got, want)
	}
}

func TestListSnapshotFilesZFSRefusesAnotherItemsSnapshot(t *testing.T) {
	s, st, _, eng := zfsRestoreFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)
	eng.lsEntries = []restic.FileEntry{{Path: "/plex/config.xml", Type: "file"}}

	entries, err := s.ListSnapshotFilesZFS(context.Background(), d.ID, zfsRootSnapID, "local")
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries = %v, err = %v, want the snapshot's files", entries, err)
	}
	if _, err := s.ListSnapshotFilesZFS(context.Background(), d.ID, "cccc3333", "local"); err == nil {
		t.Fatal("a snapshot of another item was listed")
	}
}

func TestZFSFolderTargetKeepsTheTreeShape(t *testing.T) {
	const folder = "/host/user/restored"
	if got := zfsFolderTarget(folder, "/"); got != folder {
		t.Fatalf("root = %q, want %q", got, folder)
	}
	if got := zfsFolderTarget(folder, "/plex/db"); got != path.Join(folder, "plex/db") {
		t.Fatalf("child = %q, want %q", got, path.Join(folder, "plex/db"))
	}
}
