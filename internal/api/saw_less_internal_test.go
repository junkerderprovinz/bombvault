package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/platform"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestARunThatSawLessSaysWhatAndThatOlderBackupsStay(t *testing.T) {
	s := &Service{cfg: config.Config{HostMountRoot: "/host/user", HostSourceRoot: "/mnt"}}
	cases := []struct {
		name string
		sum  backup.Summary
		want string
	}{
		{"missing folder", backup.Summary{Missing: "/host/user/data/pics"},
			store.NoteFoldersMissing + ": /mnt/data/pics; " + store.NoteOlderBackupsKept},
		{"empty run", backup.Summary{Emptied: true},
			store.NoteBackupEmpty + "; " + store.NoteOlderBackupsKept},
		{"a named folder says what an empty run would", backup.Summary{Missing: "/host/user/data/pics", Emptied: true},
			store.NoteFoldersMissing + ": /mnt/data/pics; " + store.NoteOlderBackupsKept},
		{"unreadable file", backup.Summary{Unreadable: "open /host/user/key.pem: permission denied"},
			store.NoteFilesUnreadable + ": open /mnt/key.pem: permission denied; " + store.NoteOlderBackupsKept},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := s.withUnreadableNote("success", "", tc.sum); got != tc.want {
				t.Fatalf("note = %q, want %q", got, tc.want)
			}
		})
	}
	if got := s.withUnreadableNote("failed", "", backup.Summary{Emptied: true}); got != "" {
		t.Fatalf("a failed run gets the note %q", got)
	}
}

// listingEngine answers a listing, and fails it when err is set.
type listingEngine struct {
	ResticEngine
	snaps []restic.Snapshot
	err   error
}

func (e listingEngine) Snapshots(context.Context, string, restic.Mode) ([]restic.Snapshot, error) {
	return e.snaps, e.err
}

func TestAnEmptyRunIsNewsOnlyAfterOneThatHeldData(t *testing.T) {
	measured := backup.Summary{Measured: true}
	empty := &restic.SnapshotSummary{}
	full := &restic.SnapshotSummary{TotalBytesProcessed: 10}
	cases := []struct {
		name   string
		engine listingEngine
		sum    backup.Summary
		want   bool
	}{
		{"first backup", listingEngine{}, measured, false},
		{"always empty", listingEngine{snaps: []restic.Snapshot{{Tags: []string{"flash"}, Summary: empty}}}, measured, false},
		{"held data before", listingEngine{snaps: []restic.Snapshot{{Tags: []string{"flash"}, Summary: full}}}, measured, true},
		{"from before restic counted", listingEngine{snaps: []restic.Snapshot{{Tags: []string{"flash"}}}}, measured, true},
		{"another item held data", listingEngine{snaps: []restic.Snapshot{{Tags: []string{"config"}, Summary: full}}}, measured, false},
		{"history cannot be read", listingEngine{err: errors.New("could not load snapshots")}, measured, true},
		{"read files", listingEngine{snaps: []restic.Snapshot{{Tags: []string{"flash"}, Summary: full}}}, backup.Summary{Measured: true, SourceFiles: 1}, false},
		{"not measured", listingEngine{snaps: []restic.Snapshot{{Tags: []string{"flash"}, Summary: full}}}, backup.Summary{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := &resticAdapter{engine: tc.engine, watchEmpty: true}
			if got := a.emptiedAfterData(context.Background(), "/repo", []string{"flash"}, tc.sum); got != tc.want {
				t.Fatalf("emptiedAfterData = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestThePreviewReportsAListingThatFailed(t *testing.T) {
	s := &Service{engine: listingEngine{err: errors.New("could not load snapshots")}, store: newTestStore(t)}
	groups, _, err := s.previewRetentionPerIdentity(context.Background(), "/repo", "files", restic.RetentionPolicy{KeepLast: 1}, restic.Mode{})
	if err == nil || !strings.Contains(err.Error(), "could not load snapshots") {
		t.Fatalf("preview = %+v, %v, want the failed listing", groups, err)
	}
}

func TestAnEmptiedAppdataIsRefusedWhenTheHistoryCannotBeRead(t *testing.T) {
	st := newTestStore(t)
	appdata, repo := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := &Service{engine: listingEngine{err: errors.New("could not load snapshots")}, store: st}
	err := s.refuseEmptiedAppdata(context.Background(), repo, restic.Mode{}, "app", []string{appdata})
	if err == nil || !strings.Contains(err.Error(), "could not be read") {
		t.Fatalf("refuseEmptiedAppdata = %v, want a refusal", err)
	}
}

func TestOldBackupsOfAnItemStayWhileItsNewestBackupSawLess(t *testing.T) {
	st := newTestStore(t)
	s := &Service{store: st}
	set, err := st.CreateFileSet(store.FileSet{Name: "photos", Path: "user/photos", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	finish := func(status, note string) {
		t.Helper()
		id, err := st.StartRun(set.ID, "backup")
		if err != nil {
			t.Fatal(err)
		}
		if err := st.FinishRun(id, status, "aa11", 1, note); err != nil {
			t.Fatal(err)
		}
	}
	hold := anomalyScope{Kind: anomalyScopeItem, ID: set.ID}
	held := func() bool {
		t.Helper()
		tags, err := s.heldIdentityTags()
		if err != nil {
			t.Fatal(err)
		}
		if tags.holds("fileset:photos") != s.lastBackupSawLess(hold) {
			t.Fatalf("the item's own pass and the passes over a repository disagree")
		}
		return tags.holds("fileset:photos")
	}

	finish("success", "")
	if held() {
		t.Fatal("a complete backup holds its old backups")
	}
	finish("success", store.NoteBackupEmpty+"; "+store.NoteOlderBackupsKept)
	if !held() {
		t.Fatal("a backup that saw less does not hold its old backups")
	}
	finish("failed", "disk full")
	if !held() {
		t.Fatal("a failed run wrote no snapshot, and it lifted the hold")
	}
}

func unraidService(t *testing.T, fixture string) *Service {
	t.Helper()
	old := mountinfoPath
	t.Cleanup(func() { mountinfoPath = old })
	mountinfoPath = filepath.Join(t.TempDir(), "mountinfo")
	if err := os.WriteFile(mountinfoPath, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	return &Service{cfg: config.Config{HostMountRoot: "/host/user", HostSourceRoot: "/mnt"}, platform: platform.Unraid{}}
}

const unraidMounts = `22 1 0:20 / / rw - overlay overlay rw
23 22 0:21 / /host/user rw,relatime - rootfs rootfs rw
24 23 8:1 / /host/user/cache rw,relatime - btrfs /dev/sdb1 rw
25 23 8:2 / /host/user/disks rw,relatime - tmpfs tmpfs rw
`

func TestAFolderOnNoMountedPoolIsInTheHostsMemory(t *testing.T) {
	s := unraidService(t, unraidMounts)
	for p, want := range map[string]bool{
		"/host/user/cache/repos/files": false,
		"/host/user/nvme/repos/files":  true,
		"/host/user/disks/old/repo":    true,
		"rest:http://nas:8000/files":   false,
	} {
		if got := s.inHostMemory(p); got != want {
			t.Errorf("inHostMemory(%s) = %v, want %v", p, got, want)
		}
	}
}

func TestOnlyUnraidsMntBindCanHoldAFolderInMemory(t *testing.T) {
	t.Run("another platform", func(t *testing.T) {
		s := unraidService(t, unraidMounts)
		s.SetPlatform(platform.Generic{})
		if s.inHostMemory("/host/user/nvme/repo") {
			t.Fatal("a plain host keeps its data on the bind")
		}
	})
	t.Run("a bind of one share", func(t *testing.T) {
		s := unraidService(t, unraidMounts)
		s.cfg.HostSourceRoot = "/mnt/user"
		if s.inHostMemory("/host/user/nvme/repo") {
			t.Fatal("a share bound as Host Data is storage itself")
		}
	})
	t.Run("no bind in the table", func(t *testing.T) {
		s := unraidService(t, "22 1 0:20 / / rw - overlay overlay rw\n")
		if s.inHostMemory("/host/user/nvme/repo") {
			t.Fatal("a table without the bind says nothing")
		}
	})
}

// opensEverywhere is a repository restic can open, a leftover one included.
type opensEverywhere struct{ ResticEngine }

func (opensEverywhere) RepoOpens(context.Context, string, restic.Mode) bool { return true }

func TestALeftoverRepositoryUnderAMountDoesNotTurnTheRefusalOff(t *testing.T) {
	s := unraidService(t, unraidMounts)
	s.engine = opensEverywhere{}
	s.store = newTestStore(t)
	s.cfg.AppKey = strings.Repeat("7c", 32)
	err := s.EnsureRepo(context.Background(), "/host/user/nvme/repos/files", restic.Mode{})
	if !errors.Is(err, ErrBackupPathNotMounted) || !strings.Contains(err.Error(), "/mnt/nvme/repos/files") {
		t.Fatalf("EnsureRepo = %v, want the refusal naming the folder", err)
	}
}

func TestASignInOnlyAFormerOwnFolderNamesStays(t *testing.T) {
	svc, st, _ := newConnectionSvc(t, opensWithPassword("old"))
	if err := svc.editCloudCredSets(func(sets []CloudCredSet) []CloudCredSet {
		return append(sets, CloudCredSet{ID: "old", Name: "Old box", CloudCreds: CloudCreds{RESTUser: "oldbox", RESTPassword: "pw"}})
	}); err != nil {
		t.Fatal(err)
	}
	const oldFolder = "rest:http://nas:8000/oldbox/files"
	if _, err := st.MutateSettings(func(s *store.Settings) error {
		s.FilesPath = oldFolder
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertPrimaryRemoteTarget("files", store.OffsiteTarget{Repo: oldFolder, CredsRef: "old", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.MutateSettings(func(s *store.Settings) error {
		s.FilesPath = "user/new/files"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.DeletePrimaryRemoteTarget("files"); err != nil {
		t.Fatal(err)
	}

	err := svc.RemoveSignIn("old")
	if err == nil || !strings.Contains(err.Error(), "oldbox/files") {
		t.Fatalf("RemoveSignIn = %v, want a refusal that names the old backups", err)
	}
}
