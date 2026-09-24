package api

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

const (
	zfsRoot      = "cache/appdata"
	zfsChild     = "cache/appdata/plex"
	zfsRootPath  = "/host/user/cache/appdata"
	zfsChildPath = "/host/user/cache/appdata/plex"
)

// The container's mount table for the tree the tests back up: the Host Data
// bind of /mnt plus the two datasets below it.
const zfsRunMountinfo = `
30 1 0:28 / / rw,relatime - overlay overlay rw
40 30 0:29 /mnt /host/user rw,relatime master:5 - rootfs rootfs rw
41 40 0:31 / /host/user/cache rw,relatime master:6 - zfs cache rw,xattr
42 41 0:32 / /host/user/cache/appdata rw,relatime master:7 - zfs cache/appdata rw,xattr
43 42 0:33 / /host/user/cache/appdata/plex rw,relatime master:8 - zfs cache/appdata/plex rw,xattr
`

func zfsEntry(name, mountpoint string) zfs.ListEntry {
	return zfs.ListEntry{
		Name: name, Type: "filesystem", Mountpoint: mountpoint, Mounted: true,
		Canmount: "on", Encryption: "off", Keystatus: "-", Snapdir: "hidden",
		Referenced: 4096, UsedByDataset: 4096,
	}
}

func zfsTwoDatasetTree() []zfs.ListEntry {
	return []zfs.ListEntry{
		zfsEntry(zfsRoot, "/mnt/cache/appdata"),
		zfsEntry(zfsChild, "/mnt/cache/appdata/plex"),
	}
}

// zfsRunFixture builds a service whose repository, host, mount table and
// snapshot directories are all answered by fakes.
func zfsRunFixture(t *testing.T, tree []zfs.ListEntry) (*Service, *store.Repo, *fakeZFSHost, *zfsFakeEngine) {
	t.Helper()
	st := newTestStore(t)
	eng := &zfsFakeEngine{}
	host := &fakeZFSHost{tree: tree}
	s := &Service{
		store:  st,
		engine: eng,
		zfs:    host,
		cfg: config.Config{
			HostMountRoot:  "/host/user",
			HostSourceRoot: "/mnt",
			DataDir:        t.TempDir(),
			AppKey:         strings.Repeat("a", 64),
		},
		repoMu: map[string]*sync.Mutex{"zfs": {}, "containers": {}},
	}
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	settings.ZFSEnabled = true
	settings.ZFSPath = "bombvault/zfs"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	zfsMountFixture(t, zfsTestRecords(t, zfsRunMountinfo), host, map[string]string{
		zfsRoot:  zfsRootPath,
		zfsChild: zfsChildPath,
	})
	previousStat, previousEmpty := zfsStat, zfsDirEmpty
	previousPoll, previousWait := zfsSnapshotPoll, zfsSnapshotWait
	t.Cleanup(func() {
		zfsStat, zfsDirEmpty = previousStat, previousEmpty
		zfsSnapshotPoll, zfsSnapshotWait = previousPoll, previousWait
	})
	zfsStat = func(string) (fs.FileInfo, error) { return nil, nil }
	zfsDirEmpty = func(string) (bool, error) { return false, nil }
	zfsSnapshotPoll, zfsSnapshotWait = time.Millisecond, 20*time.Millisecond
	return s, st, host, eng
}

func zfsSeedItem(t *testing.T, st *store.Repo, root string) store.ZFSDataset {
	t.Helper()
	d, err := st.CreateZFSDataset(store.ZFSDataset{Dataset: root, Enabled: true})
	if err != nil {
		t.Fatalf("create item %q: %v", root, err)
	}
	return d
}

func zfsLastRun(t *testing.T, st *store.Repo, targetID string) store.Run {
	t.Helper()
	runs, err := st.RecentRunsOfKind(targetID, "backup", 1)
	if err != nil || len(runs) == 0 {
		t.Fatalf("no run recorded for %s (err %v)", targetID, err)
	}
	return runs[0]
}

func hostDid(host *fakeZFSHost, prefix string) bool {
	for _, c := range host.recorded() {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

func TestZFSBackupWithoutHostRecordsFailedRun(t *testing.T) {
	s, st, _, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	s.zfs = nil
	d := zfsSeedItem(t, st, zfsRoot)

	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err == nil {
		t.Fatal("a backup without a host must fail")
	}
	run := zfsLastRun(t, st, d.ID)
	if run.Status != "failed" {
		t.Fatalf("run status = %q, want failed", run.Status)
	}
	if !strings.HasSuffix(run.Error, "[ssh-missing]") {
		t.Fatalf("run error = %q, want it to end in [ssh-missing]", run.Error)
	}
}

func TestZFSBackupPreflightRefusals(t *testing.T) {
	longChild := zfsRoot + "/" + strings.Repeat("n", 240)

	cases := []struct {
		name  string
		code  string
		setUp func(t *testing.T, st *store.Repo, host *fakeZFSHost)
	}{
		{
			name: "the root is gone",
			code: "not-found",
			setUp: func(_ *testing.T, _ *store.Repo, host *fakeZFSHost) {
				host.treeErr = &zfs.CmdError{Code: "not-found",
					Stderr: "cannot open 'cache/appdata': dataset does not exist"}
			},
		},
		{
			name: "the host refuses BombVault's key",
			code: "ssh-auth",
			setUp: func(_ *testing.T, _ *store.Repo, host *fakeZFSHost) {
				host.treeErr = &zfs.CmdError{Code: "ssh-auth", Stderr: "root@tower: Permission denied (publickey)."}
			},
		},
		{
			name: "the root is a volume",
			code: "not-filesystem",
			setUp: func(_ *testing.T, _ *store.Repo, host *fakeZFSHost) {
				host.tree[0].Type = "volume"
			},
		},
		{
			name: "no member can be read",
			code: "nothing-readable",
			setUp: func(_ *testing.T, _ *store.Repo, host *fakeZFSHost) {
				host.tree[0].Canmount = "off"
				host.tree[1].Keystatus = "unavailable"
			},
		},
		{
			name: "the tree is Docker's own storage",
			code: "docker-storage",
			setUp: func(_ *testing.T, _ *store.Repo, host *fakeZFSHost) {
				for i := 0; i < 21; i++ {
					e := zfsEntry(fmt.Sprintf("%s/layer%02d", zfsRoot, i), "legacy")
					e.Mountpoint = "legacy"
					host.tree = append(host.tree, e)
				}
			},
		},
		{
			name: "a child's snapshot name would not fit",
			code: "name-too-long",
			setUp: func(_ *testing.T, _ *store.Repo, host *fakeZFSHost) {
				host.tree = append(host.tree, zfsEntry(longChild, "/mnt/cache/appdata/long"))
			},
		},
		{
			name: "another item already covers part of the tree",
			code: "overlaps-item",
			setUp: func(t *testing.T, st *store.Repo, _ *fakeZFSHost) {
				zfsSeedItem(t, st, zfsChild)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, st, host, eng := zfsRunFixture(t, zfsTwoDatasetTree())
			d := zfsSeedItem(t, st, zfsRoot)
			tc.setUp(t, st, host)

			if _, err := s.BackupZFSDataset(context.Background(), d.ID); err == nil {
				t.Fatal("a refused item must fail the backup")
			}
			run := zfsLastRun(t, st, d.ID)
			if run.Status != "failed" {
				t.Fatalf("run status = %q, want failed", run.Status)
			}
			if !strings.HasSuffix(run.Error, "["+tc.code+"]") {
				t.Fatalf("run error = %q, want it to end in [%s]", run.Error, tc.code)
			}
			if !strings.Contains(run.Error, zfsRoot) {
				t.Fatalf("run error = %q, want the dataset name to survive the scrubber", run.Error)
			}
			if hostDid(host, "snapshot -r") {
				t.Fatal("a refused item must not be snapshotted")
			}
			if len(eng.readBackupDirs()) != 0 {
				t.Fatal("a refused item must not reach restic")
			}
			row, err := st.GetZFSDataset(d.ID)
			if err != nil {
				t.Fatal(err)
			}
			if row.LastCheckCode != tc.code {
				t.Fatalf("last check code = %q, want %q", row.LastCheckCode, tc.code)
			}
		})
	}
}

func TestZFSBackupPreflightScrubsTheHostsOutput(t *testing.T) {
	s, st, host, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	d := zfsSeedItem(t, st, zfsRoot)
	host.treeErr = &zfs.CmdError{Code: "zfs-error", Stderr: "/etc/profile.d/motd.sh: \x1b[1mwelcome\x1b[0m\ninternal error"}

	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err == nil {
		t.Fatal("a failed listing must fail the run")
	}
	row, err := st.GetZFSDataset(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{zfsLastRun(t, st, d.ID).Error, row.LastCheckDetail} {
		if strings.Contains(text, "/etc/profile.d") || strings.ContainsRune(text, '\x1b') {
			t.Fatalf("stored %q, want the host's paths and control characters out", text)
		}
		if !strings.Contains(text, "internal error") {
			t.Fatalf("stored %q, want the reason kept", text)
		}
	}
}

func TestZFSBackupOrchestratorRefusalsKeepDatasetName(t *testing.T) {
	t.Run("the snapshot itself fails", func(t *testing.T) {
		s, st, host, _ := zfsRunFixture(t, zfsTwoDatasetTree())
		d := zfsSeedItem(t, st, zfsRoot)
		host.snapshotErr = &zfs.CmdError{Code: "exists",
			Stderr: "cannot create snapshot 'cache/appdata@bombvault-20260101000000': dataset already exists"}

		if _, err := s.BackupZFSDataset(context.Background(), d.ID); err == nil {
			t.Fatal("a failed snapshot must fail the run")
		}
		run := zfsLastRun(t, st, d.ID)
		if run.Status != "failed" || !strings.Contains(run.Error, zfsRoot) {
			t.Fatalf("run = %q / %q, want a failure naming the dataset", run.Status, run.Error)
		}
		if !strings.Contains(run.Error, "snapshot-failed") {
			t.Fatalf("run error = %q, want the snapshot-failed code", run.Error)
		}
	})

	t.Run("the host's own output is scrubbed", func(t *testing.T) {
		s, st, host, _ := zfsRunFixture(t, zfsTwoDatasetTree())
		d := zfsSeedItem(t, st, zfsRoot)
		host.snapshotErr = &zfs.CmdError{Code: "zfs-error",
			Stderr: "/root/.bashrc: line 4: \x1b[31mwarning\x1b[0m\ncannot create snapshot: out of space"}

		if _, err := s.BackupZFSDataset(context.Background(), d.ID); err == nil {
			t.Fatal("a failed snapshot must fail the run")
		}
		run := zfsLastRun(t, st, d.ID)
		if strings.Contains(run.Error, "/root/.bashrc") || strings.ContainsRune(run.Error, '\x1b') {
			t.Fatalf("run error = %q, want the host's paths and control characters out", run.Error)
		}
		if !strings.Contains(run.Error, zfsRoot) || !strings.Contains(run.Error, "out of space") {
			t.Fatalf("run error = %q, want the dataset and the reason kept", run.Error)
		}
	})

	t.Run("the snapshot never reaches the container", func(t *testing.T) {
		s, st, _, eng := zfsRunFixture(t, zfsTwoDatasetTree())
		d := zfsSeedItem(t, st, zfsRoot)
		zfsStat = func(name string) (fs.FileInfo, error) {
			return nil, &fs.PathError{Op: "stat", Path: name, Err: syscall.ELOOP}
		}

		if _, err := s.BackupZFSDataset(context.Background(), d.ID); err == nil {
			t.Fatal("a member that cannot be read must fail the run")
		}
		run := zfsLastRun(t, st, d.ID)
		if !strings.Contains(run.Error, zfsRoot) || !strings.Contains(run.Error, "snapshot-loop") {
			t.Fatalf("run error = %q, want the dataset name and the snapshot-loop code", run.Error)
		}
		if len(eng.readBackupDirs()) != 0 {
			t.Fatal("an invisible snapshot must not be handed to restic")
		}
	})
}

func TestZFSBackupHappyPath(t *testing.T) {
	s, st, host, eng := zfsRunFixture(t, zfsTwoDatasetTree())
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "zfs", Repo: "b2:bucket/offsite", Enabled: true,
	}); err != nil {
		t.Fatalf("create off-site target: %v", err)
	}
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.RetentionKeepDaily = 7
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	d := zfsSeedItem(t, st, zfsRoot)

	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err != nil {
		t.Fatalf("backup: %v", err)
	}

	snaps := host.snapshotNames()
	calls := host.recorded()
	if len(snaps) != 0 {
		t.Fatalf("the run left %v behind, want the snapshot destroyed", snaps)
	}
	var taken, destroyed string
	for _, c := range calls {
		if strings.HasPrefix(c, "snapshot -r ") {
			taken = strings.TrimPrefix(c, "snapshot -r ")
		}
		if strings.HasPrefix(c, "destroy -r ") {
			destroyed = strings.TrimPrefix(c, "destroy -r ")
		}
	}
	if taken == "" || taken != destroyed {
		t.Fatalf("host saw snapshot %q and destroy %q, want the same name", taken, destroyed)
	}
	snap := strings.TrimPrefix(taken, zfsRoot+"@")

	wantDirs := []string{
		zfsRootPath + "/.zfs/snapshot/" + snap,
		zfsChildPath + "/.zfs/snapshot/" + snap,
	}
	gotDirs := eng.readBackupDirs()
	if len(gotDirs) != len(wantDirs) {
		t.Fatalf("restic read %v, want one run per member: %v", gotDirs, wantDirs)
	}
	for i, want := range wantDirs {
		if gotDirs[i] != want {
			t.Fatalf("restic read %q, want %q", gotDirs[i], want)
		}
	}
	for i, tags := range eng.backupTags {
		if len(tags) != 1 {
			t.Fatalf("member %d carried %v, want exactly one identity tag", i, tags)
		}
	}
	if eng.backupTags[0][0] != "zfs:"+zfsRoot || eng.backupTags[1][0] != "zfs:"+zfsChild {
		t.Fatalf("tags = %v, want one per dataset", eng.backupTags)
	}

	gotTags := eng.readForgetTags()
	wantTags := []string{"zfs:" + zfsRoot, "zfs:" + zfsChild}
	if strings.Join(gotTags, ",") != strings.Join(wantTags, ",") {
		t.Fatalf("forget tags = %v, want %v", gotTags, wantTags)
	}
	for _, pruned := range eng.forgetPruned {
		if pruned {
			t.Fatal("a member's forget must not prune; the run prunes once at the end")
		}
	}
	if eng.prunes != 1 {
		t.Fatalf("prune ran %d times, want once", eng.prunes)
	}
	if len(eng.copies) == 0 {
		t.Fatal("the run did not replicate off site")
	}

	run := zfsLastRun(t, st, d.ID)
	if run.Status != "success" {
		t.Fatalf("run status = %q (%s), want success", run.Status, run.Error)
	}
	members, err := st.ListZFSRunMembers(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 2 {
		t.Fatalf("run members = %v, want one per dataset", members)
	}
	for _, m := range members {
		if m.Outcome != "backed-up" {
			t.Fatalf("member %s outcome = %q, want backed-up", m.Dataset, m.Outcome)
		}
	}
	zfsRun, err := st.GetZFSRun(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if zfsRun.SnapshotName != snap {
		t.Fatalf("recorded snapshot = %q, want %q", zfsRun.SnapshotName, snap)
	}
	if zfsRun.WindowSeconds != -1 {
		t.Fatalf("window = %d, want -1 for an item that stops nothing", zfsRun.WindowSeconds)
	}
}

func TestZFSBackupPicksUpNewChildAndReportsIt(t *testing.T) {
	s, st, host, eng := zfsRunFixture(t, []zfs.ListEntry{zfsEntry(zfsRoot, "/mnt/cache/appdata")})
	messages := zfsCaptureNotifications(t, s)
	d := zfsSeedItem(t, st, zfsRoot)

	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err != nil {
		t.Fatalf("first backup: %v", err)
	}
	host.tree = zfsTwoDatasetTree()
	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err != nil {
		t.Fatalf("second backup: %v", err)
	}

	if len(eng.readBackupDirs()) != 3 {
		t.Fatalf("restic ran %d times, want one for the root and two for the second run", len(eng.readBackupDirs()))
	}
	run := zfsLastRun(t, st, d.ID)
	members, err := st.ListZFSRunMembers(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	var child store.ZFSRunMember
	for _, m := range members {
		if m.Dataset == zfsChild {
			child = m
		}
	}
	if child.Dataset == "" || !child.IsNew {
		t.Fatalf("second run's members = %v, want %s marked new", members, zfsChild)
	}
	if !zfsAnyMessageContains(messages(), zfsChild+" is new") {
		t.Fatalf("notifications = %v, want the new child named", messages())
	}
}

func TestZFSBackupFirstRunReportsNothingAsNew(t *testing.T) {
	s, st, _, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	messages := zfsCaptureNotifications(t, s)
	d := zfsSeedItem(t, st, zfsRoot)

	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err != nil {
		t.Fatalf("backup: %v", err)
	}
	members, err := st.ListZFSRunMembers(zfsLastRun(t, st, d.ID).ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range members {
		if m.IsNew {
			t.Fatalf("%s marked new on the item's first run", m.Dataset)
		}
	}
	if zfsAnyMessageContains(messages(), "changed") {
		t.Fatalf("notifications = %v, want no change message for a new item", messages())
	}
}

func TestZFSBackupUnchangedTreeSendsNoChangeMessage(t *testing.T) {
	tree := append(zfsTwoDatasetTree(), zfsEntry(zfsRoot+"/off", "/mnt/cache/appdata/off"))
	tree[2].Canmount = "off"
	s, st, _, _ := zfsRunFixture(t, tree)
	messages := zfsCaptureNotifications(t, s)
	d := zfsSeedItem(t, st, zfsRoot)

	for i := 0; i < 2; i++ {
		if _, err := s.BackupZFSDataset(context.Background(), d.ID); err != nil {
			t.Fatalf("backup %d: %v", i+1, err)
		}
	}
	if zfsAnyMessageContains(messages(), "changed") {
		t.Fatalf("notifications = %v, want no change message for the same tree", messages())
	}
}

func TestZFSBackupRecordsEachMembersResultOnTheTree(t *testing.T) {
	s, st, host, eng := zfsRunFixture(t, zfsTwoDatasetTree())
	host.onSnapshot = func(snap string) {
		eng.backupErr = map[string]error{zfsChildPath + "/.zfs/snapshot/" + snap: errors.New("restic: read error")}
	}
	d := zfsSeedItem(t, st, zfsRoot)

	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err == nil {
		t.Fatal("a member that failed must fail the run")
	}
	members, err := st.ListZFSMembers(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]store.ZFSMember{}
	for _, m := range members {
		got[m.Dataset] = m
	}
	if root := got[zfsRoot]; root.Outcome != "backed-up" || root.LastBackupAt == 0 {
		t.Fatalf("root = %+v, want it backed up with its time", root)
	}
	if child := got[zfsChild]; child.Outcome != "backup-failed" || child.LastBackupAt != 0 {
		t.Fatalf("child = %+v, want the run's failure and no backup time", child)
	}
}

func TestZFSBackupCountsALeftoverOnTheSweptCount(t *testing.T) {
	s, st, host, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	d := zfsSeedItem(t, st, zfsRoot)
	if err := st.SetZFSLeftovers(d.ID, 3, time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	host.destroyErr = &zfs.CmdError{Code: "zfs-permission", Stderr: "cannot destroy snapshot: permission denied"}

	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err != nil {
		t.Fatalf("backup: %v", err)
	}
	row, err := st.GetZFSDataset(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if row.LeftoverCount != 1 {
		t.Fatalf("leftover count = %d, want the one snapshot the sweep did not find", row.LeftoverCount)
	}
}

func TestZFSBackupSkipsUnreadableChildrenVisibly(t *testing.T) {
	tree := zfsTwoDatasetTree()
	unreadable := map[string]func(e *zfs.ListEntry){
		"cache/appdata/off":    func(e *zfs.ListEntry) { e.Canmount = "off" },
		"cache/appdata/locked": func(e *zfs.ListEntry) { e.Keystatus = "unavailable" },
		"cache/appdata/noauto": func(e *zfs.ListEntry) { e.Mounted = false },
		"cache/appdata/legacy": func(e *zfs.ListEntry) { e.Mountpoint = "legacy" },
		"cache/appdata/vol":    func(e *zfs.ListEntry) { e.Type = "volume" },
	}
	wantCodes := map[string]string{
		"cache/appdata/off":    "canmount-off",
		"cache/appdata/locked": "key-not-loaded",
		"cache/appdata/noauto": "not-mounted",
		"cache/appdata/legacy": "legacy-mount",
		"cache/appdata/vol":    "zvol",
	}
	for name, mutate := range unreadable {
		e := zfsEntry(name, "/mnt/"+strings.TrimPrefix(name, "cache/"))
		mutate(&e)
		tree = append(tree, e)
	}

	s, st, _, eng := zfsRunFixture(t, tree)
	d := zfsSeedItem(t, st, zfsRoot)
	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err != nil {
		t.Fatalf("backup: %v", err)
	}

	if len(eng.readBackupDirs()) != 2 {
		t.Fatalf("restic read %v, want only the two readable datasets", eng.readBackupDirs())
	}
	run := zfsLastRun(t, st, d.ID)
	if run.Status != "success" {
		t.Fatalf("run status = %q (%s), want a skipped member not to fail the run", run.Status, run.Error)
	}
	members, err := st.ListZFSRunMembers(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, m := range members {
		got[m.Dataset] = m.Outcome
	}
	for name, code := range wantCodes {
		if got[name] != code {
			t.Fatalf("%s recorded as %q, want %q", name, got[name], code)
		}
	}
}

func TestZFSBackupExcludedChildIsLeftOut(t *testing.T) {
	tree := append(zfsTwoDatasetTree(), zfsEntry(zfsChild+"/cache", "/mnt/cache/appdata/plex/cache"))
	s, st, _, eng := zfsRunFixture(t, tree)
	d := zfsSeedItem(t, st, zfsRoot)
	if err := st.SetZFSExcludedChildren(d.ID, []string{zfsChild}); err != nil {
		t.Fatalf("exclude child: %v", err)
	}

	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err != nil {
		t.Fatalf("backup: %v", err)
	}
	dirs := eng.readBackupDirs()
	if len(dirs) != 1 || !strings.HasPrefix(dirs[0], zfsRootPath+"/") {
		t.Fatalf("restic read %v, want only the root", dirs)
	}
	run := zfsLastRun(t, st, d.ID)
	members, err := st.ListZFSRunMembers(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range members {
		if m.Dataset == zfsRoot {
			continue
		}
		if m.Outcome != "excluded" {
			t.Fatalf("%s recorded as %q, want excluded for the child and its subtree", m.Dataset, m.Outcome)
		}
	}
}

func TestZFSBackupUsesDatasetRepoOverride(t *testing.T) {
	s, st, _, eng := zfsRunFixture(t, zfsTwoDatasetTree())
	named, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Role: store.RoleRepo, Name: "cold", Repo: "b2:bucket/cold", Enabled: true,
	})
	if err != nil {
		t.Fatalf("create named repo: %v", err)
	}
	d := zfsSeedItem(t, st, zfsRoot)
	if err := st.SetZFSDatasetRepo(d.ID, named.ID); err != nil {
		t.Fatalf("point the item at the named repo: %v", err)
	}

	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err != nil {
		t.Fatalf("backup: %v", err)
	}
	for _, repo := range eng.backupRepos {
		if repo != "b2:bucket/cold" {
			t.Fatalf("member backed up to %q, want the item's own repository", repo)
		}
	}
}

func TestZFSBackupCancelKeyIsRootKey(t *testing.T) {
	s, st, host, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	d := zfsSeedItem(t, st, zfsRoot)

	cancelled := make(chan bool, 1)
	host.onSnapshot = func(string) {
		cancelled <- s.CancelBackupRun("zfs:" + zfsRoot)
	}
	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err == nil {
		t.Fatal("a cancelled backup must not report success")
	}
	if !<-cancelled {
		t.Fatalf("CancelBackupRun did not find the run under zfs:%s", zfsRoot)
	}
	run := zfsLastRun(t, st, d.ID)
	if run.Status != "cancelled" {
		t.Fatalf("run status = %q, want cancelled", run.Status)
	}
}

func TestZFSSweepOnlyExactBombVaultStampsOnFilesystems(t *testing.T) {
	tree := []zfs.ListEntry{
		zfsEntry("bvtest/p", "/mnt/bvtest/p"),
		zfsEntry("bvtest/p/c", "/mnt/bvtest/p/c"),
	}
	vol := zfsEntry("bvtest/p/vol", "-")
	vol.Type = "volume"
	tree = append(tree, vol)

	s, st, host, _ := zfsRunFixture(t, tree)
	host.snaps = []zfs.SnapshotEntry{
		{Dataset: "bvtest/p", Name: "bombvault-20260101000000"},
		{Dataset: "bvtest/p/c", Name: "bombvault-20260101000000"},
		{Dataset: "bvtest/p", Name: "bombvault-prerestore-20260101000000"},
		{Dataset: "bvtest/p", Name: "autosnap_2026"},
		{Dataset: "bvtest/p", Name: "bombvault-2026"},
		{Dataset: "bvtest/p/vol", Name: "bombvault-20260202000000"},
	}
	d := zfsSeedItem(t, st, "bvtest/p")

	remaining, err := s.SweepZFSLeftovers(context.Background(), d.ID)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("remaining = %d, want 0", remaining)
	}
	var destroys []string
	for _, c := range host.recorded() {
		if strings.HasPrefix(c, "destroy") {
			destroys = append(destroys, c)
		}
	}
	want := []string{"destroy -r bvtest/p@bombvault-20260101000000"}
	if strings.Join(destroys, "|") != strings.Join(want, "|") {
		t.Fatalf("destroys = %v, want %v", destroys, want)
	}
}

func TestZFSSweepCoversDeletedRowsAndDomainOff(t *testing.T) {
	s, st, host, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ZFSEnabled = false
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	d := zfsSeedItem(t, st, zfsRoot)
	if err := st.SetZFSDatasetEnabled(d.ID, false); err != nil {
		t.Fatal(err)
	}
	host.snaps = []zfs.SnapshotEntry{{Dataset: zfsRoot, Name: "bombvault-20260101000000"}}

	s.SweepZFSLeftoversOnStartup(context.Background())

	if !hostDid(host, "destroy -r "+zfsRoot+"@bombvault-20260101000000") {
		t.Fatalf("the startup sweep left the stamp behind: %v", host.recorded())
	}
	row, err := st.GetZFSDataset(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if row.LeftoverCount != 0 || row.LeftoverCheckedAt == 0 {
		t.Fatalf("leftovers = %d at %d, want a fresh count of zero", row.LeftoverCount, row.LeftoverCheckedAt)
	}
}

func TestZFSBackupLeavesTheDestroyToTheSweepWhenItFails(t *testing.T) {
	s, st, host, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	d := zfsSeedItem(t, st, zfsRoot)
	host.destroyErr = &zfs.CmdError{Code: "zfs-permission", Stderr: "cannot destroy snapshot: permission denied"}

	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err != nil {
		t.Fatalf("a failed destroy must not fail the backup: %v", err)
	}
	run := zfsLastRun(t, st, d.ID)
	if run.Status != "success" {
		t.Fatalf("run status = %q, want the backup to stand", run.Status)
	}
	row, err := st.GetZFSDataset(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if row.LeftoverCount != 1 {
		t.Fatalf("leftover count = %d, want the stamp counted on the row", row.LeftoverCount)
	}
}

func TestZFSRefusalSentenceNamesTheCode(t *testing.T) {
	got := zfsRefusalSentence("backup", zfsRoot, &backup.ZFSRefusal{Code: "not-found",
		Detail: "cannot open 'cache/appdata': dataset does not exist"})
	if !strings.HasSuffix(got, "[not-found]") || !strings.Contains(got, zfsRoot) {
		t.Fatalf("sentence = %q", got)
	}
	if bare := zfsRefusalSentence("backup", zfsRoot, &backup.ZFSRefusal{Code: "ssh-missing"}); !strings.HasSuffix(bare, "[ssh-missing]") {
		t.Fatalf("sentence without a detail = %q", bare)
	}
	if !errors.Is(&backup.ZFSRefusal{Code: "x"}, backup.ErrZFSRefusal) {
		t.Fatal("a refusal must pass the scrubber bypass")
	}
}
