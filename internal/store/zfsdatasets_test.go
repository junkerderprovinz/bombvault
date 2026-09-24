package store_test

import (
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func zfsStore(t *testing.T) (*sql.DB, *store.Repo) {
	t.Helper()
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db, store.New(db)
}

func aZFSDataset(t *testing.T, r *store.Repo, name string) store.ZFSDataset {
	t.Helper()
	d, err := r.CreateZFSDataset(store.ZFSDataset{Dataset: name, Enabled: true})
	if err != nil {
		t.Fatalf("CreateZFSDataset(%s): %v", name, err)
	}
	return d
}

func TestZFSDatasetCRUD(t *testing.T) {
	_, r := zfsStore(t)

	created, err := r.CreateZFSDataset(store.ZFSDataset{
		Dataset:          "cache/appdata",
		Enabled:          true,
		Excludes:         []string{"/plex/Cache"},
		ExcludedChildren: []string{"cache/appdata/plex"},
		Repo:             "repo-1",
		StopContainers:   []string{"plex"},
		HookContainer:    "postgres",
		PreSnapshot:      "pg_backup_start",
		PostSnapshot:     "pg_backup_stop",
	})
	if err != nil {
		t.Fatalf("CreateZFSDataset: %v", err)
	}
	if created.ID == "" || created.CreatedAt == 0 {
		t.Fatalf("create returned id %q and created_at %d, want both filled", created.ID, created.CreatedAt)
	}

	got, err := r.GetZFSDataset(created.ID)
	if err != nil {
		t.Fatalf("GetZFSDataset: %v", err)
	}
	if got.Dataset != "cache/appdata" || !got.Enabled {
		t.Fatalf("stored item is %q enabled=%v", got.Dataset, got.Enabled)
	}
	if len(got.Excludes) != 1 || got.Excludes[0] != "/plex/Cache" {
		t.Fatalf("excludes = %v", got.Excludes)
	}
	if len(got.ExcludedChildren) != 1 || got.ExcludedChildren[0] != "cache/appdata/plex" {
		t.Fatalf("excluded children = %v", got.ExcludedChildren)
	}
	if got.Repo != "repo-1" {
		t.Fatalf("repo = %q, want the chosen repository to survive the create", got.Repo)
	}
	if len(got.StopContainers) != 1 || got.StopContainers[0] != "plex" {
		t.Fatalf("stop list = %v", got.StopContainers)
	}
	if got.HookContainer != "postgres" || got.PreSnapshot != "pg_backup_start" || got.PostSnapshot != "pg_backup_stop" {
		t.Fatalf("hooks = %q %q %q", got.HookContainer, got.PreSnapshot, got.PostSnapshot)
	}
	if len(got.RestartPending) != 0 {
		t.Fatalf("restart pending = %v, want empty on a fresh item", got.RestartPending)
	}

	byName, err := r.GetZFSDatasetByName("cache/appdata")
	if err != nil {
		t.Fatalf("GetZFSDatasetByName: %v", err)
	}
	if byName.ID != created.ID {
		t.Fatalf("lookup by name returned %q, want %q", byName.ID, created.ID)
	}

	second := aZFSDataset(t, r, "cache/system")
	if err := r.SetZFSDatasetEnabled(second.ID, false); err != nil {
		t.Fatalf("SetZFSDatasetEnabled: %v", err)
	}
	if err := r.SetZFSDatasetScheduleCadence(second.ID, "daily 04:00"); err != nil {
		t.Fatalf("SetZFSDatasetScheduleCadence: %v", err)
	}
	if err := r.SetZFSDatasetRepo(second.ID, "repo-2"); err != nil {
		t.Fatalf("SetZFSDatasetRepo: %v", err)
	}
	if err := r.SetZFSDatasetStopContainers(second.ID, []string{"sonarr", "radarr"}); err != nil {
		t.Fatalf("SetZFSDatasetStopContainers: %v", err)
	}
	if err := r.SetZFSLeftovers(second.ID, 3, 1700000000); err != nil {
		t.Fatalf("SetZFSLeftovers: %v", err)
	}

	list, err := r.ListZFSDatasets()
	if err != nil {
		t.Fatalf("ListZFSDatasets: %v", err)
	}
	if len(list) != 2 || list[0].Dataset != "cache/appdata" || list[1].Dataset != "cache/system" {
		t.Fatalf("list = %+v, want both items ordered by dataset", list)
	}
	if list[1].Enabled || list[1].ScheduleCadence != "daily 04:00" || list[1].Repo != "repo-2" {
		t.Fatalf("setters did not take: %+v", list[1])
	}
	if len(list[1].StopContainers) != 2 {
		t.Fatalf("stop list = %v", list[1].StopContainers)
	}
	if list[1].LeftoverCount != 3 || list[1].LeftoverCheckedAt != 1700000000 {
		t.Fatalf("leftovers = %d at %d", list[1].LeftoverCount, list[1].LeftoverCheckedAt)
	}

	if err := r.DeleteZFSDataset(second.ID); err != nil {
		t.Fatalf("DeleteZFSDataset: %v", err)
	}
	if _, err := r.GetZFSDataset(second.ID); err == nil {
		t.Fatal("the deleted item is still readable")
	}
}

func TestZFSDatasetNameUnique(t *testing.T) {
	_, r := zfsStore(t)
	aZFSDataset(t, r, "cache/appdata")

	if _, err := r.CreateZFSDataset(store.ZFSDataset{Dataset: "cache/appdata", Enabled: true}); err == nil {
		t.Fatal("two items on the same root dataset were accepted")
	}
}

// The setters own their columns so that a PATCH from a form that does not know
// about them cannot drop a stop list, a repository or a pending restart.
func TestUpdateZFSDatasetLeavesOwnedColumns(t *testing.T) {
	_, r := zfsStore(t)
	d, err := r.CreateZFSDataset(store.ZFSDataset{
		Dataset:          "cache/appdata",
		Enabled:          true,
		Excludes:         []string{"/old"},
		ExcludedChildren: []string{"cache/appdata/plex"},
		Repo:             "repo-1",
		StopContainers:   []string{"plex"},
		HookContainer:    "postgres",
		PreSnapshot:      "freeze",
		PostSnapshot:     "thaw",
	})
	if err != nil {
		t.Fatalf("CreateZFSDataset: %v", err)
	}
	if err := r.SetZFSDatasetScheduleCadence(d.ID, "daily 04:00"); err != nil {
		t.Fatalf("SetZFSDatasetScheduleCadence: %v", err)
	}
	if err := r.SetZFSRestartPending(d.ID, []string{"plex"}); err != nil {
		t.Fatalf("SetZFSRestartPending: %v", err)
	}

	err = r.UpdateZFSDataset(store.ZFSDataset{
		ID:               d.ID,
		Dataset:          "tank/other",
		Enabled:          false,
		Excludes:         []string{"/new"},
		ExcludedChildren: nil,
		Repo:             "",
		StopContainers:   nil,
		RestartPending:   nil,
		HookContainer:    "",
		ScheduleCadence:  "",
	})
	if err != nil {
		t.Fatalf("UpdateZFSDataset: %v", err)
	}

	got, err := r.GetZFSDataset(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Enabled {
		t.Fatal("the update must switch the item off")
	}
	if len(got.Excludes) != 1 || got.Excludes[0] != "/new" {
		t.Fatalf("excludes = %v, want the update's patterns", got.Excludes)
	}
	if got.Dataset != "cache/appdata" {
		t.Fatalf("dataset = %q; the root names the backups and must not change", got.Dataset)
	}
	if got.ScheduleCadence != "daily 04:00" {
		t.Fatalf("cadence = %q, want the per-item schedule kept", got.ScheduleCadence)
	}
	if got.Repo != "repo-1" {
		t.Fatalf("repo = %q, want the item to stay on its repository", got.Repo)
	}
	if len(got.StopContainers) != 1 || got.StopContainers[0] != "plex" {
		t.Fatalf("stop list = %v, want it kept", got.StopContainers)
	}
	if len(got.ExcludedChildren) != 1 {
		t.Fatalf("excluded children = %v, want them kept", got.ExcludedChildren)
	}
	if got.HookContainer != "postgres" || got.PreSnapshot != "freeze" || got.PostSnapshot != "thaw" {
		t.Fatalf("hooks = %q %q %q, want them kept", got.HookContainer, got.PreSnapshot, got.PostSnapshot)
	}
	if len(got.RestartPending) != 1 {
		t.Fatalf("restart pending = %v, want the marker kept", got.RestartPending)
	}
}

func TestZFSExcludedChildrenAndHooksRoundTrip(t *testing.T) {
	_, r := zfsStore(t)
	d := aZFSDataset(t, r, "cache/appdata")

	if err := r.SetZFSExcludedChildren(d.ID, []string{"cache/appdata/plex", "cache/appdata/docker"}); err != nil {
		t.Fatalf("SetZFSExcludedChildren: %v", err)
	}
	if err := r.SetZFSHooks(d.ID, "postgres", "pg_backup_start", "pg_backup_stop"); err != nil {
		t.Fatalf("SetZFSHooks: %v", err)
	}

	got, err := r.GetZFSDataset(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ExcludedChildren) != 2 || got.ExcludedChildren[1] != "cache/appdata/docker" {
		t.Fatalf("excluded children = %v", got.ExcludedChildren)
	}
	if got.HookContainer != "postgres" || got.PreSnapshot != "pg_backup_start" || got.PostSnapshot != "pg_backup_stop" {
		t.Fatalf("hooks = %q %q %q", got.HookContainer, got.PreSnapshot, got.PostSnapshot)
	}

	if err := r.SetZFSExcludedChildren(d.ID, nil); err != nil {
		t.Fatalf("SetZFSExcludedChildren(nil): %v", err)
	}
	got, err = r.GetZFSDataset(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ExcludedChildren) != 0 {
		t.Fatalf("excluded children = %v, want the list cleared", got.ExcludedChildren)
	}
}

func TestZFSRestartPendingRoundTrip(t *testing.T) {
	_, r := zfsStore(t)
	d := aZFSDataset(t, r, "cache/appdata")

	if err := r.SetZFSRestartPending(d.ID, []string{"plex", "sonarr"}); err != nil {
		t.Fatalf("SetZFSRestartPending: %v", err)
	}
	got, err := r.GetZFSDataset(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.RestartPending) != 2 || got.RestartPending[0] != "plex" {
		t.Fatalf("restart pending = %v", got.RestartPending)
	}

	if err := r.ClearZFSRestartPending(d.ID); err != nil {
		t.Fatalf("ClearZFSRestartPending: %v", err)
	}
	got, err = r.GetZFSDataset(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.RestartPending) != 0 {
		t.Fatalf("restart pending = %v after the restart", got.RestartPending)
	}
}

func TestListZFSRestartPending(t *testing.T) {
	_, r := zfsStore(t)
	waiting := aZFSDataset(t, r, "cache/appdata")
	aZFSDataset(t, r, "cache/system")

	if err := r.SetZFSRestartPending(waiting.ID, []string{"plex"}); err != nil {
		t.Fatalf("SetZFSRestartPending: %v", err)
	}

	rows, err := r.ListZFSRestartPending()
	if err != nil {
		t.Fatalf("ListZFSRestartPending: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != waiting.ID {
		t.Fatalf("pending rows = %+v, want only the item that stopped containers", rows)
	}
	if len(rows[0].RestartPending) != 1 || rows[0].RestartPending[0] != "plex" {
		t.Fatalf("pending containers = %v", rows[0].RestartPending)
	}

	if err := r.ClearZFSRestartPending(waiting.ID); err != nil {
		t.Fatal(err)
	}
	rows, err = r.ListZFSRestartPending()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("pending rows = %+v after the restart, want none", rows)
	}
}

func TestSetZFSCheckTruncatesDetailAndReturnsPrevious(t *testing.T) {
	_, r := zfsStore(t)
	d := aZFSDataset(t, r, "cache/appdata")

	prev, err := r.SetZFSCheck(d.ID, "ok", "", "/mnt/cache/appdata", 1700000000)
	if err != nil {
		t.Fatalf("SetZFSCheck: %v", err)
	}
	if prev != "" {
		t.Fatalf("previous code = %q, want empty for an item that was never checked", prev)
	}

	long := strings.Repeat("ä", 3000)
	prev, err = r.SetZFSCheck(d.ID, "snapshot-loop", long, "/mnt/cache/appdata", 1700000500)
	if err != nil {
		t.Fatalf("SetZFSCheck: %v", err)
	}
	if prev != "ok" {
		t.Fatalf("previous code = %q, want ok so a notification fires only on a change", prev)
	}

	got, err := r.GetZFSDataset(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.LastCheckCode != "snapshot-loop" || got.LastCheckAt != 1700000500 {
		t.Fatalf("check = %q at %d", got.LastCheckCode, got.LastCheckAt)
	}
	if got.LastHostMountpoint != "/mnt/cache/appdata" {
		t.Fatalf("host mountpoint = %q", got.LastHostMountpoint)
	}
	if len(got.LastCheckDetail) > 2000 {
		t.Fatalf("detail is %d bytes, want at most 2000", len(got.LastCheckDetail))
	}
	if !strings.HasPrefix(long, got.LastCheckDetail) || got.LastCheckDetail == "" {
		t.Fatal("the stored detail must be the beginning of the reported one")
	}
	for _, rn := range got.LastCheckDetail {
		if rn == '�' {
			t.Fatal("the cut must fall on a character boundary")
		}
	}
}

func TestDeleteZFSDatasetRemovesRunsAndDetailRows(t *testing.T) {
	db, r := zfsStore(t)
	doomed := aZFSDataset(t, r, "cache/appdata")
	kept := aZFSDataset(t, r, "cache/system")

	for _, d := range []store.ZFSDataset{doomed, kept} {
		runID, err := r.StartRun(d.ID, "backup")
		if err != nil {
			t.Fatalf("StartRun: %v", err)
		}
		if err := r.FinishRun(runID, "success", "snap", 10, ""); err != nil {
			t.Fatal(err)
		}
		if err := r.RecordZFSRun(runID, d.ID, "bombvault-20240102030405", 12, ""); err != nil {
			t.Fatalf("RecordZFSRun: %v", err)
		}
		if err := r.AddZFSRunMember(store.ZFSRunMember{RunID: runID, Dataset: d.Dataset, Outcome: "backed-up"}); err != nil {
			t.Fatalf("AddZFSRunMember: %v", err)
		}
		if err := r.ReplaceZFSMembers(d.ID, []store.ZFSMember{{ItemID: d.ID, Dataset: d.Dataset, Outcome: "backed-up"}}); err != nil {
			t.Fatalf("ReplaceZFSMembers: %v", err)
		}
		err = r.UpsertZFSSafetySnapshot(store.ZFSSafetySnapshot{
			ItemID: d.ID, Dataset: d.Dataset, Name: "bombvault-prerestore-20240102030405", CreatedAt: 1700000000,
		})
		if err != nil {
			t.Fatalf("UpsertZFSSafetySnapshot: %v", err)
		}
	}

	if err := r.DeleteZFSDataset(doomed.ID); err != nil {
		t.Fatalf("DeleteZFSDataset: %v", err)
	}

	for _, q := range []string{
		`SELECT count(*) FROM runs WHERE target_id = ?`,
		`SELECT count(*) FROM zfs_members WHERE item_id = ?`,
		`SELECT count(*) FROM zfs_runs WHERE item_id = ?`,
		`SELECT count(*) FROM zfs_safety_snapshots WHERE item_id = ?`,
	} {
		var gone, left int
		if err := db.QueryRow(q, doomed.ID).Scan(&gone); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		if gone != 0 {
			t.Fatalf("%s left %d rows behind", q, gone)
		}
		if err := db.QueryRow(q, kept.ID).Scan(&left); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		if left != 1 {
			t.Fatalf("%s: the other item has %d rows, want its own row untouched", q, left)
		}
	}

	var members int
	if err := db.QueryRow(`SELECT count(*) FROM zfs_run_members`).Scan(&members); err != nil {
		t.Fatal(err)
	}
	if members != 1 {
		t.Fatalf("zfs_run_members holds %d rows, want only the surviving item's", members)
	}
}

func TestDeleteZFSDatasetRemovesAnomalyState(t *testing.T) {
	_, r := zfsStore(t)
	doomed := aZFSDataset(t, r, "tank/doomed")
	kept := aZFSDataset(t, r, "tank/kept")
	seedAnomalyState(t, r, doomed.ID, "zfs")
	seedAnomalyState(t, r, kept.ID, "zfs")
	seedDomainAnomaly(t, r)

	if err := r.DeleteZFSDataset(doomed.ID); err != nil {
		t.Fatalf("DeleteZFSDataset: %v", err)
	}
	assertAnomalyStateGone(t, r, doomed.ID)

	rows, _, err := r.ListAnomalies(store.AnomalyFilter{TargetID: kept.ID})
	if err != nil {
		t.Fatalf("ListAnomalies: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("the other item kept %d of its findings", len(rows))
	}
	domainRows, _, err := r.ListAnomalies(store.AnomalyFilter{ScopeKind: "domain"})
	if err != nil {
		t.Fatalf("ListAnomalies domain: %v", err)
	}
	if len(domainRows) != 1 {
		t.Fatal("the domain finding went with the item")
	}
}

func TestLastSuccessfulZFSBackupScopedToDatasets(t *testing.T) {
	_, r := zfsStore(t)
	d := aZFSDataset(t, r, "cache/appdata")

	ts, err := r.LastSuccessfulZFSBackup()
	if err != nil {
		t.Fatalf("LastSuccessfulZFSBackup: %v", err)
	}
	if !ts.IsZero() {
		t.Fatalf("time = %v before any run, want the zero time", ts)
	}

	fs, err := r.CreateFileSet(store.FileSet{Name: "docs", Path: "user/documents", Enabled: true})
	if err != nil {
		t.Fatalf("CreateFileSet: %v", err)
	}
	runID, err := r.StartRun(fs.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.FinishRun(runID, "success", "snap", 1, ""); err != nil {
		t.Fatal(err)
	}
	ts, err = r.LastSuccessfulZFSBackup()
	if err != nil {
		t.Fatal(err)
	}
	if !ts.IsZero() {
		t.Fatalf("a folders backup satisfied the ZFS gate (%v)", ts)
	}

	runID, err = r.StartRun(d.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.FinishRun(runID, "success", "snap", 1, ""); err != nil {
		t.Fatal(err)
	}
	ts, err = r.LastSuccessfulZFSBackup()
	if err != nil {
		t.Fatal(err)
	}
	if ts.IsZero() {
		t.Fatal("a successful dataset backup must set the domain's last-success time")
	}
}

// A child that was there before is not news; one that turns up later is, and the
// run has to say so, so its first sighting survives the rewrite.
func TestReplaceZFSMembersKeepsFirstSeen(t *testing.T) {
	_, r := zfsStore(t)
	d := aZFSDataset(t, r, "cache/appdata")

	first := []store.ZFSMember{
		{ItemID: d.ID, Dataset: "cache/appdata", HostMountpoint: "/mnt/cache/appdata", Outcome: "backed-up", FirstSeenAt: 100, LastBackupAt: 100, UsedByDataset: 4096},
		{ItemID: d.ID, Dataset: "cache/appdata/plex", HostMountpoint: "/mnt/cache/appdata/plex", Outcome: "backed-up", FirstSeenAt: 100, LastBackupAt: 100},
	}
	if err := r.ReplaceZFSMembers(d.ID, first); err != nil {
		t.Fatalf("ReplaceZFSMembers: %v", err)
	}

	second := []store.ZFSMember{
		{ItemID: d.ID, Dataset: "cache/appdata", HostMountpoint: "/mnt/cache/appdata", Outcome: "backed-up", FirstSeenAt: 200, LastBackupAt: 200, UsedByDataset: 8192},
		{ItemID: d.ID, Dataset: "cache/appdata/sonarr", HostMountpoint: "/mnt/cache/appdata/sonarr", Outcome: "backed-up", FirstSeenAt: 200, LastBackupAt: 200},
	}
	if err := r.ReplaceZFSMembers(d.ID, second); err != nil {
		t.Fatalf("ReplaceZFSMembers: %v", err)
	}

	members, err := r.ListZFSMembers(d.ID)
	if err != nil {
		t.Fatalf("ListZFSMembers: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("members = %+v, want the tree as the last run found it", members)
	}
	byName := map[string]store.ZFSMember{}
	for _, m := range members {
		byName[m.Dataset] = m
	}
	root, ok := byName["cache/appdata"]
	if !ok {
		t.Fatal("the root is missing from the members")
	}
	if root.FirstSeenAt != 100 {
		t.Fatalf("root first seen at %d, want the original 100", root.FirstSeenAt)
	}
	if root.UsedByDataset != 8192 || root.LastBackupAt != 200 {
		t.Fatalf("root = %+v, want the newest size and backup time", root)
	}
	fresh, ok := byName["cache/appdata/sonarr"]
	if !ok {
		t.Fatal("the child picked up in the second run is missing")
	}
	if fresh.FirstSeenAt != 200 {
		t.Fatalf("new child first seen at %d, want 200", fresh.FirstSeenAt)
	}
	if _, gone := byName["cache/appdata/plex"]; gone {
		t.Fatal("a dataset the host no longer has must not stay in the members")
	}
}

func TestZFSRunMembersRoundTripAndByDataset(t *testing.T) {
	db, r := zfsStore(t)
	d := aZFSDataset(t, r, "cache/appdata")

	older, err := r.StartRun(d.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.FinishRun(older, "success", "snap1", 1, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE runs SET started_at = 100, finished_at = 200 WHERE id = ?`, older); err != nil {
		t.Fatalf("backdate the older run: %v", err)
	}
	newer, err := r.StartRun(d.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.FinishRun(newer, "success", "snap2", 1, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE runs SET started_at = 300, finished_at = 400 WHERE id = ?`, newer); err != nil {
		t.Fatalf("backdate the newer run: %v", err)
	}

	wrote := store.ZFSRunMember{
		RunID: newer, Dataset: "cache/appdata", Outcome: "backed-up", IsNew: true,
		ResticSnapshot: "abcd1234", BytesAdded: 512, FilesNew: 3, FilesChanged: 2, FilesUnmodified: 90, DurationMS: 1500,
	}
	if err := r.AddZFSRunMember(wrote); err != nil {
		t.Fatalf("AddZFSRunMember: %v", err)
	}
	if err := r.AddZFSRunMember(store.ZFSRunMember{RunID: newer, Dataset: "cache/appdata/plex", Outcome: "empty"}); err != nil {
		t.Fatal(err)
	}
	if err := r.AddZFSRunMember(store.ZFSRunMember{RunID: older, Dataset: "cache/appdata", Outcome: "backed-up", BytesAdded: 64}); err != nil {
		t.Fatal(err)
	}

	got, err := r.ListZFSRunMembers(newer)
	if err != nil {
		t.Fatalf("ListZFSRunMembers: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("run members = %+v, want both members of that run", got)
	}
	if got[0] != wrote {
		t.Fatalf("stored member = %+v, want %+v", got[0], wrote)
	}

	history, err := r.ListZFSRunMembersByDataset("cache/appdata", 10)
	if err != nil {
		t.Fatalf("ListZFSRunMembersByDataset: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("history = %+v, want one row per run", history)
	}
	if history[0].RunID != newer {
		t.Fatal("the newest run must come first")
	}
	limited, err := r.ListZFSRunMembersByDataset("cache/appdata", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(limited) != 1 || limited[0].RunID != newer {
		t.Fatalf("limited history = %+v", limited)
	}
}

func TestZFSRunKeepsWindowAndHookDetail(t *testing.T) {
	_, r := zfsStore(t)
	d := aZFSDataset(t, r, "cache/appdata")

	if _, err := r.GetZFSRun("no-such-run"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetZFSRun on an unknown run = %v, want sql.ErrNoRows", err)
	}

	quiet, err := r.StartRun(d.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.RecordZFSRun(quiet, d.ID, "bombvault-20240102030405", -1, ""); err != nil {
		t.Fatalf("RecordZFSRun: %v", err)
	}
	got, err := r.GetZFSRun(quiet)
	if err != nil {
		t.Fatalf("GetZFSRun: %v", err)
	}
	if got.ItemID != d.ID || got.SnapshotName != "bombvault-20240102030405" {
		t.Fatalf("run = %+v", got)
	}
	if got.WindowSeconds != -1 {
		t.Fatalf("window = %d, want -1 for a run that stopped nothing", got.WindowSeconds)
	}

	stopped, err := r.StartRun(d.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.RecordZFSRun(stopped, d.ID, "bombvault-20240102040506", 7, "pg_backup_stop: not in backup mode"); err != nil {
		t.Fatal(err)
	}
	got, err = r.GetZFSRun(stopped)
	if err != nil {
		t.Fatal(err)
	}
	if got.WindowSeconds != 7 {
		t.Fatalf("window = %d, want the measured seconds", got.WindowSeconds)
	}
	if got.HookDetail != "pg_backup_stop: not in backup mode" {
		t.Fatalf("hook detail = %q", got.HookDetail)
	}
}

func TestZFSSafetySnapshotsReconcile(t *testing.T) {
	_, r := zfsStore(t)
	d := aZFSDataset(t, r, "cache/appdata")

	for _, s := range []store.ZFSSafetySnapshot{
		{ItemID: d.ID, Dataset: "cache/appdata", Name: "bombvault-prerestore-20240102030405", CreatedAt: 100},
		{ItemID: d.ID, Dataset: "cache/appdata/plex", Name: "bombvault-prerestore-20240102030406", CreatedAt: 200},
	} {
		if err := r.UpsertZFSSafetySnapshot(s); err != nil {
			t.Fatalf("UpsertZFSSafetySnapshot: %v", err)
		}
	}

	list, err := r.ListZFSSafetySnapshots(d.ID)
	if err != nil {
		t.Fatalf("ListZFSSafetySnapshots: %v", err)
	}
	if len(list) != 2 || list[0].Name != "bombvault-prerestore-20240102030406" {
		t.Fatalf("list = %+v, want the newest first", list)
	}

	err = r.ReplaceZFSSafetySnapshots(d.ID, []store.ZFSSafetySnapshot{
		{ItemID: d.ID, Dataset: "cache/appdata", Name: "bombvault-prerestore-20240102030405", CreatedAt: 100, UsedBytes: 4096},
	})
	if err != nil {
		t.Fatalf("ReplaceZFSSafetySnapshots: %v", err)
	}
	list, err = r.ListZFSSafetySnapshots(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("list = %+v, want the snapshot the host no longer has dropped", list)
	}
	if list[0].UsedBytes != 4096 {
		t.Fatalf("used bytes = %d, want the size from the listing", list[0].UsedBytes)
	}

	if err := r.DeleteZFSSafetySnapshot("cache/appdata", "bombvault-prerestore-20240102030405"); err != nil {
		t.Fatalf("DeleteZFSSafetySnapshot: %v", err)
	}
	list, err = r.ListZFSSafetySnapshots(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("list = %+v after the delete", list)
	}
}
