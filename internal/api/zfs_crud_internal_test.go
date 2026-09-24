package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// zfsRepoFixture is zfsRunFixture with a repository that really exists on disk,
// which is what the snapshot listings of discover and delete need.
func zfsRepoFixture(t *testing.T) (*Service, *store.Repo, *fakeZFSHost, *zfsFakeEngine) {
	t.Helper()
	s, st, host, eng := zfsRunFixture(t, zfsTwoDatasetTree())
	root := t.TempDir()
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
	return s, st, host, eng
}

func zfsCodeOf(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("expected a coded refusal, got nil")
	}
	code, ok := zfsRefusalCode(err)
	if !ok {
		t.Fatalf("error %v carries no reason code", err)
	}
	return code
}

func TestCreateZFSDatasetsPerItemResults(t *testing.T) {
	s, st, _, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	s.docker = newZFSFakeDocker("plex")

	results := s.CreateZFSDatasets(context.Background(), []ZFSCreateItem{
		{Dataset: zfsRoot, ExcludedChildren: []string{zfsChild}, StopContainers: []string{"plex"}},
		{Dataset: "cache/../etc"},
		{Dataset: "tank/media", ExcludedChildren: []string{"other/child"}},
		{Dataset: "tank/photos", StopContainers: []string{"ghost"}},
	})
	if len(results) != 4 {
		t.Fatalf("results = %d, want one per requested item", len(results))
	}
	want := []string{"ok", "invalid-name", "invalid-exclude", "container-unknown"}
	for i, res := range results {
		if res.Code != want[i] {
			t.Errorf("%s = %q, want %q (detail %q)", res.Dataset, res.Code, want[i], res.Detail)
		}
	}
	if results[0].ID == "" {
		t.Error("a stored item must come back with its id")
	}
	rows, err := st.ListZFSDatasets()
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	if len(rows) != 1 || rows[0].Dataset != zfsRoot {
		t.Fatalf("stored rows = %v, want only the accepted item", rows)
	}
	if len(rows[0].ExcludedChildren) != 1 || rows[0].ExcludedChildren[0] != zfsChild {
		t.Errorf("excluded children = %v, want the one that was sent", rows[0].ExcludedChildren)
	}
	if len(rows[0].StopContainers) != 1 {
		t.Errorf("stop list = %v, want the container that was sent", rows[0].StopContainers)
	}
}

func TestCreateZFSDatasetsRefusesOverlap(t *testing.T) {
	s, st, _, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	zfsSeedItem(t, st, zfsRoot)

	results := s.CreateZFSDatasets(context.Background(), []ZFSCreateItem{
		{Dataset: zfsChild},
		{Dataset: "cache"},
		{Dataset: zfsRoot},
		{Dataset: "tank/one"},
		{Dataset: "tank/one/two"},
	})
	want := []string{"overlaps-item", "overlaps-item", "overlaps-item", "ok", "overlaps-item"}
	for i, res := range results {
		if res.Code != want[i] {
			t.Errorf("%s = %q, want %q", res.Dataset, res.Code, want[i])
		}
	}
	rows, err := st.ListZFSDatasets()
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("stored rows = %d, want the seeded item plus the one that does not overlap", len(rows))
	}
}

func TestCreateZFSDatasetsRefusesMoreThanTheCap(t *testing.T) {
	s, st, _, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	items := make([]ZFSCreateItem, zfsMaxCreateItems+1)
	for i := range items {
		items[i] = ZFSCreateItem{Dataset: "tank/set" + strings.Repeat("x", i%3)}
	}

	if got := s.CreateZFSDatasets(context.Background(), items); len(got) != 0 {
		t.Fatalf("results = %d, want the whole batch refused", len(got))
	}
	rows, err := st.ListZFSDatasets()
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("stored rows = %d, want none", len(rows))
	}
}

func TestPatchZFSDatasetDisableNeverCallsHost(t *testing.T) {
	s, st, host, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	d := zfsSeedItem(t, st, zfsRoot)

	off := false
	if err := s.PatchZFSDataset(context.Background(), d.ID, ZFSDatasetPatch{Enabled: &off}); err != nil {
		t.Fatalf("switch the item off: %v", err)
	}
	if calls := host.recorded(); len(calls) != 0 {
		t.Fatalf("switching an item off reached the host: %v", calls)
	}
	row, err := st.GetZFSDataset(d.ID)
	if err != nil {
		t.Fatalf("reload the item: %v", err)
	}
	if row.Enabled {
		t.Fatal("the item is still enabled")
	}
}

func TestPatchZFSDatasetKeepsBothFieldsWhenExcludesAndEnabledChangeTogether(t *testing.T) {
	s, st, _, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	d := zfsSeedItem(t, st, zfsRoot)

	off := false
	patterns := []string{"*.tmp"}
	if err := s.PatchZFSDataset(context.Background(), d.ID, ZFSDatasetPatch{Enabled: &off, Excludes: &patterns}); err != nil {
		t.Fatalf("patch: %v", err)
	}
	row, err := st.GetZFSDataset(d.ID)
	if err != nil {
		t.Fatalf("reload the item: %v", err)
	}
	if row.Enabled {
		t.Fatal("the item is still enabled")
	}
	if len(row.Excludes) != 1 || row.Excludes[0] != "*.tmp" {
		t.Fatalf("excludes = %v, want the patched pattern", row.Excludes)
	}
}

func TestPatchZFSDatasetEnableRunsCheck(t *testing.T) {
	s, st, host, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	d, err := st.CreateZFSDataset(store.ZFSDataset{Dataset: zfsRoot})
	if err != nil {
		t.Fatalf("create the item: %v", err)
	}

	on := true
	if err := s.PatchZFSDataset(context.Background(), d.ID, ZFSDatasetPatch{Enabled: &on}); err != nil {
		t.Fatalf("switch the item on: %v", err)
	}
	if !hostDid(host, "list -r "+zfsRoot) {
		t.Fatalf("switching an item on did not look at its tree: %v", host.recorded())
	}
	row, err := st.GetZFSDataset(d.ID)
	if err != nil {
		t.Fatalf("reload the item: %v", err)
	}
	if row.LastCheckCode != "ok" || row.LastCheckAt == 0 {
		t.Fatalf("check = %q at %d, want the fresh verdict on the row", row.LastCheckCode, row.LastCheckAt)
	}
	if row.LastHostMountpoint != "/mnt/cache/appdata" {
		t.Errorf("host mountpoint = %q, want the root's", row.LastHostMountpoint)
	}
}

func TestPatchZFSDatasetRepoLockedAfterBackups(t *testing.T) {
	s, st, _, _ := zfsRepoFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)
	runID, err := st.StartRun(d.ID, "backup")
	if err != nil {
		t.Fatalf("start a run: %v", err)
	}
	if err := st.FinishRun(runID, "success", "snap", 1, ""); err != nil {
		t.Fatalf("finish the run: %v", err)
	}

	repo := "named-1"
	err = s.PatchZFSDataset(context.Background(), d.ID, ZFSDatasetPatch{Repo: &repo})
	if err == nil {
		t.Fatal("the repository of an item with backups must not move: its snapshots stay where they were written")
	}
	if !strings.Contains(err.Error(), "backups") {
		t.Fatalf("refusal = %q, want it to say why", err)
	}
	row, err := st.GetZFSDataset(d.ID)
	if err != nil {
		t.Fatalf("reload the item: %v", err)
	}
	if row.Repo != "" {
		t.Fatalf("repository = %q, want it unchanged", row.Repo)
	}
}

func TestPatchZFSDatasetStopContainersValidated(t *testing.T) {
	s, st, _, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	docker := newZFSFakeDocker("plex")
	docker.self = "bombvault"
	docker.containers["bombvault"] = &zfsFakeContainer{id: "id-bombvault", running: true, service: "bombvault"}
	s.docker = docker
	d := zfsSeedItem(t, st, zfsRoot)

	unknown := []string{"ghost"}
	if code := zfsCodeOf(t, s.PatchZFSDataset(context.Background(), d.ID, ZFSDatasetPatch{StopContainers: &unknown})); code != "container-unknown" {
		t.Fatalf("code = %q, want container-unknown", code)
	}
	itself := []string{"bombvault"}
	if code := zfsCodeOf(t, s.PatchZFSDataset(context.Background(), d.ID, ZFSDatasetPatch{StopContainers: &itself})); code != "container-is-self" {
		t.Fatalf("code = %q, want container-is-self", code)
	}
	good := []string{"plex"}
	if err := s.PatchZFSDataset(context.Background(), d.ID, ZFSDatasetPatch{StopContainers: &good}); err != nil {
		t.Fatalf("a running container must be accepted: %v", err)
	}
}

func TestPatchZFSDatasetExcludedChildrenValidated(t *testing.T) {
	s, st, _, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	d := zfsSeedItem(t, st, zfsRoot)
	if err := st.ReplaceZFSMembers(d.ID, []store.ZFSMember{
		{ItemID: d.ID, Dataset: zfsRoot},
		{ItemID: d.ID, Dataset: zfsChild},
	}); err != nil {
		t.Fatalf("record the tree: %v", err)
	}

	stranger := []string{"tank/other"}
	if code := zfsCodeOf(t, s.PatchZFSDataset(context.Background(), d.ID, ZFSDatasetPatch{ExcludedChildren: &stranger})); code != "invalid-exclude" {
		t.Fatalf("code = %q, want invalid-exclude for a dataset outside the tree", code)
	}
	itself := []string{zfsRoot}
	if code := zfsCodeOf(t, s.PatchZFSDataset(context.Background(), d.ID, ZFSDatasetPatch{ExcludedChildren: &itself})); code != "invalid-exclude" {
		t.Fatalf("code = %q, want invalid-exclude for the root itself", code)
	}
	// An exclude pattern that covers a whole member silently drops a dataset
	// the tree still snapshots, so it is refused in favour of the child list.
	wholeMember := []string{"/plex"}
	if code := zfsCodeOf(t, s.PatchZFSDataset(context.Background(), d.ID, ZFSDatasetPatch{Excludes: &wholeMember})); code != "invalid-exclude" {
		t.Fatalf("code = %q, want invalid-exclude for a pattern that names a member", code)
	}
	child := []string{zfsChild}
	if err := s.PatchZFSDataset(context.Background(), d.ID, ZFSDatasetPatch{ExcludedChildren: &child}); err != nil {
		t.Fatalf("a real descendant must be accepted: %v", err)
	}
	patterns := []string{"/plex/cache/**"}
	if err := s.PatchZFSDataset(context.Background(), d.ID, ZFSDatasetPatch{Excludes: &patterns}); err != nil {
		t.Fatalf("a pattern below a member must be accepted: %v", err)
	}
}

func TestZFSSettingsAreBoundedAtSaveTime(t *testing.T) {
	s, st, _, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	docker := newZFSFakeDocker("plex")
	docker.self = "bombvault"
	docker.containers["bombvault"] = &zfsFakeContainer{id: "id-bombvault", running: true, service: "bombvault"}
	s.docker = docker
	d := zfsSeedItem(t, st, zfsRoot)

	tooMany := make([]string, zfsMaxStopContainers+1)
	for i := range tooMany {
		tooMany[i] = "plex"
	}
	results := s.CreateZFSDatasets(context.Background(), []ZFSCreateItem{
		{Dataset: "tank/one", Repo: "no-such-repo"},
		{Dataset: "tank/two", StopContainers: tooMany},
	})
	for _, res := range results {
		if res.Code == "ok" {
			t.Errorf("%s was stored although its settings are out of bounds", res.Dataset)
		}
	}

	repo := "no-such-repo"
	longCommand := strings.Repeat("x", zfsMaxCommandBytes+1)
	self := "bombvault"
	for name, patch := range map[string]ZFSDatasetPatch{
		"an unknown repository":   {Repo: &repo},
		"too many containers":     {StopContainers: &tooMany},
		"a long pre command":      {PreSnapshot: &longCommand},
		"a long post command":     {PostSnapshot: &longCommand},
		"BombVault as hook place": {HookContainer: &self},
	} {
		if err := s.PatchZFSDataset(context.Background(), d.ID, patch); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	if code := zfsCodeOf(t, s.PatchZFSDataset(context.Background(), d.ID, ZFSDatasetPatch{HookContainer: &self})); code != "container-is-self" {
		t.Fatalf("code = %q, want container-is-self", code)
	}
	row, err := st.GetZFSDataset(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if row.Repo != "" || len(row.StopContainers) != 0 || row.PreSnapshot != "" || row.HookContainer != "" {
		t.Fatalf("row = %+v, want nothing of the refused patches stored", row)
	}
}

func TestPatchZFSDatasetCadenceFollowsTheDomainGrammar(t *testing.T) {
	s, st, _, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	d := zfsSeedItem(t, st, zfsRoot)

	interval := "everyN 3 02:00"
	if err := s.PatchZFSDataset(context.Background(), d.ID, ZFSDatasetPatch{ScheduleCadence: &interval}); err == nil {
		t.Fatal("an interval cadence has no per-item gate and must be refused")
	}
	daily := "daily 03:30"
	if err := s.PatchZFSDataset(context.Background(), d.ID, ZFSDatasetPatch{ScheduleCadence: &daily}); err != nil {
		t.Fatalf("a daily cadence: %v", err)
	}
	row, err := st.GetZFSDataset(d.ID)
	if err != nil {
		t.Fatalf("reload the item: %v", err)
	}
	if row.ScheduleCadence != daily {
		t.Fatalf("cadence = %q, want %q", row.ScheduleCadence, daily)
	}
}

func TestZFSRunDetailCarriesWindowAndHookDetail(t *testing.T) {
	s, st, _, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	d := zfsSeedItem(t, st, zfsRoot)
	runID, err := st.StartRun(d.ID, "backup")
	if err != nil {
		t.Fatalf("start a run: %v", err)
	}
	if err := st.RecordZFSRun(runID, d.ID, "bombvault-20260101000000", 42, "post-snapshot said: exit 1"); err != nil {
		t.Fatalf("record the run: %v", err)
	}
	for _, m := range []store.ZFSRunMember{
		{RunID: runID, Dataset: zfsRoot, Outcome: "backed-up", ResticSnapshot: "aaa"},
		{RunID: runID, Dataset: zfsChild, Outcome: "empty", IsNew: true},
	} {
		if err := st.AddZFSRunMember(m); err != nil {
			t.Fatalf("record a member: %v", err)
		}
	}

	detail, err := s.ZFSRunDetail(context.Background(), runID)
	if err != nil {
		t.Fatalf("ZFSRunDetail: %v", err)
	}
	if detail.WindowSeconds != 42 {
		t.Errorf("window = %d, want 42", detail.WindowSeconds)
	}
	if !strings.Contains(detail.HookDetail, "exit 1") {
		t.Errorf("hook detail = %q, want the command's own output", detail.HookDetail)
	}
	if len(detail.Members) != 2 || !detail.Members[1].IsNew {
		t.Fatalf("members = %+v, want both with the new mark kept", detail.Members)
	}

	empty, err := s.ZFSRunDetail(context.Background(), "no-such-run")
	if err != nil {
		t.Fatalf("a run without ZFS detail is not an error: %v", err)
	}
	if empty.WindowSeconds != -1 {
		t.Errorf("window = %d, want -1 when nothing was stopped", empty.WindowSeconds)
	}
}

func TestDeleteZFSDatasetSweepsFirstAndReportsRemaining(t *testing.T) {
	s, st, host, _ := zfsRepoFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)
	host.snaps = []zfs.SnapshotEntry{{Dataset: zfsRoot, Name: "bombvault-20260101000000"}}
	host.destroyErr = errors.New("dataset is busy")
	if err := st.UpsertZFSSafetySnapshot(store.ZFSSafetySnapshot{
		ItemID: d.ID, Dataset: zfsRoot, Name: "bombvault-prerestore-20260101000000", CreatedAt: 1,
	}); err != nil {
		t.Fatalf("record a safety snapshot: %v", err)
	}

	res, err := s.DeleteZFSDataset(context.Background(), d.ID, false)
	if err != nil {
		t.Fatalf("delete the item: %v", err)
	}
	if res.LeftoversRemaining != 1 {
		t.Errorf("leftovers = %d, want the stamp the sweep could not remove", res.LeftoversRemaining)
	}
	if res.SafetyRemaining != 1 {
		t.Errorf("safety snapshots = %d, want the one that was kept", res.SafetyRemaining)
	}
	if !hostDid(host, "destroy -r "+zfsRoot+"@bombvault-20260101000000") {
		t.Fatalf("the sweep did not run before the row went: %v", host.recorded())
	}
	if hostDid(host, "destroy "+zfsRoot+"@bombvault-prerestore-") {
		t.Fatal("the delete destroyed a safety snapshot that was not asked for")
	}
	if _, err := st.GetZFSDataset(d.ID); err == nil {
		t.Fatal("the row is still there")
	}
}

func TestDeleteZFSDatasetRemovesSafetySnapshotsOnRequest(t *testing.T) {
	s, st, host, _ := zfsRepoFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)
	if err := st.UpsertZFSSafetySnapshot(store.ZFSSafetySnapshot{
		ItemID: d.ID, Dataset: zfsRoot, Name: "bombvault-prerestore-20260101000000", CreatedAt: 1,
	}); err != nil {
		t.Fatalf("record a safety snapshot: %v", err)
	}

	res, err := s.DeleteZFSDataset(context.Background(), d.ID, true)
	if err != nil {
		t.Fatalf("delete the item: %v", err)
	}
	if res.SafetyRemaining != 0 {
		t.Errorf("safety snapshots = %d, want none left", res.SafetyRemaining)
	}
	if !hostDid(host, "destroy "+zfsRoot+"@bombvault-prerestore-20260101000000") {
		t.Fatalf("the safety snapshot was not destroyed: %v", host.recorded())
	}
}

func TestDeleteZFSDatasetWithoutAHostStillRemovesTheRow(t *testing.T) {
	s, st, _, _ := zfsRepoFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)
	s.zfs = nil

	if _, err := s.DeleteZFSDataset(context.Background(), d.ID, false); err != nil {
		t.Fatalf("delete the item: %v", err)
	}
	if _, err := st.GetZFSDataset(d.ID); err == nil {
		t.Fatal("the row survived a delete the host could not help with")
	}
}

func TestDeleteBackupsZFSDatasetForgetsEveryMemberTag(t *testing.T) {
	s, st, _, eng := zfsRepoFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)
	other := zfsSeedItem(t, st, "tank/media")
	eng.snaps = []restic.Snapshot{
		{ID: "s1", Tags: []string{"zfs:" + zfsRoot}},
		{ID: "s2", Tags: []string{"zfs:" + zfsChild}},
		{ID: "s3", Tags: []string{"zfs:" + zfsRoot + "/db"}},
		{ID: "s4", Tags: []string{"zfs:" + other.Dataset}},
		{ID: "s5", Tags: []string{"fileset:Photos"}},
	}

	if _, err := s.DeleteBackupsZFSDataset(context.Background(), d.ID, false); err != nil {
		t.Fatalf("delete the backups: %v", err)
	}
	got := strings.Join(eng.readForgotten(), ",")
	if got != "s1,s2,s3" {
		t.Fatalf("forgotten = %q, want every member of this tree and nothing else", got)
	}
	if _, err := st.GetZFSDataset(d.ID); err == nil {
		t.Fatal("the row survived its backups")
	}
}

func TestDeleteBackupsZFSDatasetAnswersWhatStaysOnThePool(t *testing.T) {
	for _, removeSafety := range []bool{false, true} {
		s, st, host, eng := zfsRepoFixture(t)
		d := zfsSeedItem(t, st, zfsRoot)
		eng.snaps = []restic.Snapshot{{ID: "s1", Tags: []string{"zfs:" + zfsRoot}}}
		if err := st.UpsertZFSSafetySnapshot(store.ZFSSafetySnapshot{
			ItemID: d.ID, Dataset: zfsRoot, Name: "bombvault-prerestore-20260101000000", CreatedAt: 1,
		}); err != nil {
			t.Fatalf("record a safety snapshot: %v", err)
		}

		target := "/api/zfs/datasets/" + d.ID + "/backups"
		if removeSafety {
			target += "?safety=true"
		}
		req := httptest.NewRequest(http.MethodDelete, target, nil)
		req.SetPathValue("id", d.ID)
		w := httptest.NewRecorder()
		h := NewHandler(s.cfg, st, nil, s, schedule.New(func(string) error { return nil }, st.ListTargets), nil)
		h.handleDeleteBackupsZFSDataset(w, req)

		var resp struct {
			OK                 bool `json:"ok"`
			LeftoversRemaining *int `json:"leftoversRemaining"`
			SafetyRemaining    *int `json:"safetyRemaining"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v (%s)", err, w.Body.String())
		}
		if !resp.OK || resp.LeftoversRemaining == nil || resp.SafetyRemaining == nil {
			t.Fatalf("answer = %s, want both counts", w.Body.String())
		}
		destroyed := hostDid(host, "destroy "+zfsRoot+"@bombvault-prerestore-20260101000000")
		want := 1
		if removeSafety {
			want = 0
		}
		if *resp.SafetyRemaining != want || destroyed != removeSafety {
			t.Fatalf("safety=%v: remaining %d, destroyed %v", removeSafety, *resp.SafetyRemaining, destroyed)
		}
	}
}

func TestDiscoverZFSDatasetsRebuildsMinimalRoots(t *testing.T) {
	s, st, _, eng := zfsRepoFixture(t)
	eng.snaps = []restic.Snapshot{
		{ID: "s1", Tags: []string{"zfs:cache/app data"}},
		{ID: "s2", Tags: []string{"zfs:cache/app data/plex"}},
		{ID: "s3", Tags: []string{"zfs:tank/media"}},
		{ID: "s4", Tags: []string{"zfs:cache/../etc"}},
		{ID: "s5", Tags: []string{"fileset:Photos"}},
	}

	found, skipped, err := s.DiscoverZFSDatasets(context.Background(), false)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(skipped) != 0 {
		t.Fatalf("skipped = %v, want none", skipped)
	}
	if found != 2 {
		t.Fatalf("found = %d, want the two minimal roots", found)
	}
	rows, err := st.ListZFSDatasets()
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	names := map[string]store.ZFSDataset{}
	for _, row := range rows {
		names[row.Dataset] = row
	}
	if len(names) != 2 {
		t.Fatalf("rows = %v, want cache/app data and tank/media", rows)
	}
	for name, row := range names {
		if row.Enabled {
			t.Errorf("%s came back enabled, but its stop list and commands are lost", name)
		}
	}
}

func TestDiscoverZFSDatasetsLeavesARootThatOverlapsAnItem(t *testing.T) {
	s, st, _, eng := zfsRepoFixture(t)
	child := zfsSeedItem(t, st, zfsChild)
	media := zfsSeedItem(t, st, "tank/media/photos")
	eng.snaps = []restic.Snapshot{
		{ID: "s1", Tags: []string{"zfs:" + zfsRoot}},
		{ID: "s2", Tags: []string{"zfs:" + zfsChild}},
		{ID: "s3", Tags: []string{"zfs:tank/media"}},
		{ID: "s4", Tags: []string{"zfs:tank/docs"}},
	}

	if _, _, err := s.DiscoverZFSDatasets(context.Background(), false); err != nil {
		t.Fatalf("discover: %v", err)
	}
	rows, err := st.ListZFSDatasets()
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	got := map[string]bool{}
	for _, row := range rows {
		got[row.Dataset] = true
	}
	if len(got) != 3 || !got[child.Dataset] || !got[media.Dataset] || !got["tank/docs"] {
		t.Fatalf("rows = %v, want the two items kept and only the tree that overlaps nothing rebuilt", got)
	}
}

func TestDiscoverZFSDatasetsKeepsNamedRepo(t *testing.T) {
	s, st, _, eng := zfsRepoFixture(t)
	named, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Name: "Cold", Repo: "named", Enabled: true, Role: store.RoleRepo})
	if err != nil {
		t.Fatalf("create a named repository: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(s.cfg.HostMountRoot, "named"), 0o750); err != nil {
		t.Fatalf("create the named repository: %v", err)
	}
	if err := os.WriteFile(filepath.Join(s.cfg.HostMountRoot, "named", "config"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("mark the named repository as established: %v", err)
	}
	eng.snapsByRepo = map[string][]restic.Snapshot{
		s.cfg.HostMountRoot + "/named": {{ID: "s1", Tags: []string{"zfs:tank/media"}}},
	}

	found, _, err := s.DiscoverZFSDatasets(context.Background(), false)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if found != 1 {
		t.Fatalf("found = %d, want the tree that lives in the named repository", found)
	}
	row, err := st.GetZFSDatasetByName("tank/media")
	if err != nil {
		t.Fatalf("load the rebuilt item: %v", err)
	}
	if row.Repo != named.ID {
		t.Fatalf("repository = %q, want the one its snapshots were found in (%s)", row.Repo, named.ID)
	}

	// A row that came back on the domain repository in an earlier pass is put
	// on the one holding its snapshots, or its next backup lands elsewhere.
	if err := st.SetZFSDatasetRepo(row.ID, ""); err != nil {
		t.Fatalf("clear the repository: %v", err)
	}
	if _, _, err := s.DiscoverZFSDatasets(context.Background(), false); err != nil {
		t.Fatalf("discover again: %v", err)
	}
	row, err = st.GetZFSDatasetByName("tank/media")
	if err != nil {
		t.Fatalf("load the item again: %v", err)
	}
	if row.Repo != named.ID {
		t.Fatalf("repository after a second pass = %q, want %s", row.Repo, named.ID)
	}
}

func TestDiscoverZFSDatasetsDryRunWritesNothing(t *testing.T) {
	s, st, _, eng := zfsRepoFixture(t)
	eng.snaps = []restic.Snapshot{{ID: "s1", Tags: []string{"zfs:tank/media"}}}

	found, _, err := s.DiscoverZFSDatasets(context.Background(), true)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if found != 1 {
		t.Fatalf("found = %d, want the root it would have rebuilt", found)
	}
	rows, err := st.ListZFSDatasets()
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("rows = %v, want a read-only pass", rows)
	}
}

// The domain's repository helpers have to know zfs, or a pass over the domain
// silently answers about the containers repository instead.
func TestZFSIsAKnownDomainOfTheRepositoryHelpers(t *testing.T) {
	s, st, _, _ := zfsRepoFixture(t)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	want := s.cfg.HostMountRoot + "/repo"

	repo, err := s.repoFor(settings, zfsDomain, "local")
	if err != nil || repo != want {
		t.Fatalf("repoFor = %q (%v), want %q", repo, err, want)
	}
	if got := s.DiscoverSource(zfsDomain); got != want {
		t.Fatalf("DiscoverSource = %q, want %q", got, want)
	}
	if got := domainTagPrefixes(zfsDomain); len(got) != 1 || got[0] != "zfs:" {
		t.Fatalf("tag prefixes = %v, want [zfs:]", got)
	}

	named, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Name: "Cold", Repo: "named", Enabled: true, Role: store.RoleRepo})
	if err != nil {
		t.Fatalf("create a named repository: %v", err)
	}
	d := zfsSeedItem(t, st, zfsRoot)
	if err := st.SetZFSDatasetRepo(d.ID, named.ID); err != nil {
		t.Fatalf("point the item at it: %v", err)
	}
	refs, _, err := s.domainReposInUse(settings, zfsDomain)
	if err != nil {
		t.Fatalf("domainReposInUse: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("repositories in use = %d, want the domain's own and the item's named one", len(refs))
	}
}

func zfsHandlerFor(t *testing.T, s *Service, st *store.Repo) *Handler {
	t.Helper()
	return NewHandler(s.cfg, st, nil, s, nil, nil)
}

// Both answers below hand out the store rows unchanged, so their wire names are
// the store struct's own and a rename there would reach the page unnoticed.
func TestZFSRunMembersAnswerNamesEveryFieldTheRunPanelReads(t *testing.T) {
	s, st, _, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	d := zfsSeedItem(t, st, zfsRoot)
	runID, err := st.StartRun(d.ID, "backup")
	if err != nil {
		t.Fatalf("start a run: %v", err)
	}
	if err := st.RecordZFSRun(runID, d.ID, "bombvault-20260101000000", 7, ""); err != nil {
		t.Fatalf("record the run: %v", err)
	}
	if err := st.AddZFSRunMember(store.ZFSRunMember{
		RunID: runID, Dataset: zfsRoot, Outcome: "backed-up", ResticSnapshot: "aaa",
		IsNew: true, BytesAdded: 11, FilesNew: 2, FilesChanged: 3, FilesUnmodified: 4, DurationMS: 55,
	}); err != nil {
		t.Fatalf("record a member: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/zfs/runs/"+runID+"/members", nil)
	req.SetPathValue("runId", runID)
	w := httptest.NewRecorder()
	zfsHandlerFor(t, s, st).handleZFSRunMembers(w, req)

	var resp struct {
		Members []map[string]any `json:"members"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	if len(resp.Members) != 1 {
		t.Fatalf("members = %v, want the one that was recorded", resp.Members)
	}
	for _, field := range []string{
		"dataset", "outcome", "resticSnapshot", "isNew",
		"bytesAdded", "filesNew", "filesChanged", "filesUnmodified", "durationMs",
	} {
		if _, ok := resp.Members[0][field]; !ok {
			t.Errorf("member has no %q: %v", field, resp.Members[0])
		}
	}
}

func TestZFSSafetySnapshotsAnswerNamesEveryFieldTheListReads(t *testing.T) {
	s, st, host, _ := zfsRepoFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)
	host.snaps = []zfs.SnapshotEntry{
		{Dataset: zfsRoot, Name: "bombvault-prerestore-20260101000000", Creation: 1_700_000_000, Used: 4096},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/zfs/datasets/"+d.ID+"/safety-snapshots", nil)
	req.SetPathValue("id", d.ID)
	w := httptest.NewRecorder()
	zfsHandlerFor(t, s, st).handleListZFSSafetySnapshots(w, req)

	var resp struct {
		Snapshots []map[string]any `json:"snapshots"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	if len(resp.Snapshots) != 1 {
		t.Fatalf("snapshots = %v, want the one the pool holds", resp.Snapshots)
	}
	for _, field := range []string{"dataset", "name", "createdAt", "usedBytes"} {
		if _, ok := resp.Snapshots[0][field]; !ok {
			t.Errorf("snapshot has no %q: %v", field, resp.Snapshots[0])
		}
	}
}
