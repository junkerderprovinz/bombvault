package api

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// zfsDomainFixture is a service whose store carries one enabled ZFS item and
// nothing else, for the sites that only need the domain to exist.
func zfsDomainFixture(t *testing.T) (*Service, *store.Repo, store.Settings) {
	t.Helper()
	st := newTestStore(t)
	s := &Service{
		store:  st,
		engine: &zfsFakeEngine{},
		cfg: config.Config{
			HostMountRoot: "/host/user",
			DataDir:       t.TempDir(),
			AppKey:        strings.Repeat("a", 64),
		},
		repoMu: map[string]*sync.Mutex{zfsDomain: {}},
	}
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	settings.ContainersEnabled = false
	settings.VMsEnabled = false
	settings.FlashEnabled = false
	settings.FilesEnabled = false
	settings.ZFSEnabled = true
	settings.ZFSPath = "bombvault/zfs"
	settings.ZFSSchedule = "daily 03:00"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	return s, st, settings
}

func TestIdentityTagsRecognizesZFS(t *testing.T) {
	tags := identityTags([]restic.Snapshot{{Tags: []string{"zfs:cache/appdata", "p1"}}})
	if len(tags) != 1 || tags[0] != "zfs:cache/appdata" {
		t.Fatalf("identityTags = %v, want [zfs:cache/appdata]", tags)
	}
}

func TestDomainTagPrefixZFS(t *testing.T) {
	got := domainTagPrefixes(zfsDomain)
	if len(got) != 1 || got[0] != "zfs:" {
		t.Fatalf("domainTagPrefixes(zfs) = %v, want [zfs:]", got)
	}
}

func TestRepoForZFS(t *testing.T) {
	s, _, settings := zfsDomainFixture(t)
	repo, err := s.repoFor(settings, zfsDomain, "local")
	if err != nil {
		t.Fatalf("repoFor: %v", err)
	}
	if !strings.HasSuffix(repo, "bombvault/zfs") {
		t.Fatalf("repoFor(zfs) = %q, want the ZFS path", repo)
	}
}

func TestOffsiteSettingsForZFS(t *testing.T) {
	settings := store.Settings{
		ZFSOffsite:            "rest:https://box:8000/zfs",
		ZFSOffsiteSchedule:    "weekly SUN 02:00",
		ZFSOffsiteImmutable:   true,
		FilesOffsite:          "rest:https://box:8000/files",
		FilesOffsiteImmutable: false,
	}
	if got := offsiteRepoFromSettings(zfsDomain, settings); got != settings.ZFSOffsite {
		t.Fatalf("offsiteRepoFromSettings(zfs) = %q", got)
	}
	if got := offsiteScheduleFromSettings(zfsDomain, settings); got != settings.ZFSOffsiteSchedule {
		t.Fatalf("offsiteScheduleFromSettings(zfs) = %q", got)
	}
	if !offsiteImmutableFor(zfsDomain, settings) {
		t.Fatal("offsiteImmutableFor(zfs) = false, want true")
	}
}

func TestDomainStatusIncludesZFS(t *testing.T) {
	s, _, settings := zfsDomainFixture(t)
	entries, err := s.domainStatusFrom(settings)
	if err != nil {
		t.Fatalf("domainStatusFrom: %v", err)
	}
	for _, e := range entries {
		if e.Domain == zfsDomain {
			if !e.Enabled || e.Schedule != "daily 03:00" {
				t.Fatalf("zfs entry = %+v, want it enabled on its own cadence", e)
			}
			return
		}
	}
	t.Fatalf("no zfs entry in %+v", entries)
}

func TestBucketRunsByDayCountsZFS(t *testing.T) {
	const at = int64(1_700_000_000)
	days := bucketRunsByDay(
		[]store.Run{
			{TargetID: "item", Kind: "backup", Status: "success", StartedAt: at},
			{TargetID: "item", Kind: "backup", Status: "failed", StartedAt: at},
		},
		map[string]string{"item": zfsDomain}, at, at,
	)
	if len(days) != 1 {
		t.Fatalf("got %d days, want 1", len(days))
	}
	if days[0].ZFS.OK != 1 || days[0].ZFS.Failed != 1 {
		t.Fatalf("zfs day = %+v, want one ok and one failed", days[0].ZFS)
	}
}

func TestRunDomainsMapsZFSItems(t *testing.T) {
	s, st, _ := zfsDomainFixture(t)
	d, err := st.CreateZFSDataset(store.ZFSDataset{Dataset: "cache/appdata", Enabled: true})
	if err != nil {
		t.Fatalf("create item: %v", err)
	}
	if got := s.runDomains()[d.ID]; got != zfsDomain {
		t.Fatalf("runDomains[%s] = %q, want zfs", d.ID, got)
	}
}

func TestPickDRSnapshotZFSNothingToDrill(t *testing.T) {
	s, _, settings := zfsDomainFixture(t)
	eng := s.engine.(*zfsFakeEngine)
	eng.snaps = []restic.Snapshot{{ID: "aaa", Tags: []string{"fileset:docs"}}}

	if _, err := s.pickDRSnapshot(context.Background(), zfsDomain, settings, "repo", restic.Mode{}); err == nil {
		t.Fatal("a repository without a zfs: snapshot must not offer a drill target")
	}

	eng.snaps = append(eng.snaps, restic.Snapshot{ID: "bbb", Tags: []string{"zfs:cache/appdata"}})
	id, err := s.pickDRSnapshot(context.Background(), zfsDomain, settings, "repo", restic.Mode{})
	if err != nil {
		t.Fatalf("pickDRSnapshot: %v", err)
	}
	if id != "bbb" {
		t.Fatalf("picked %q, want the zfs-tagged snapshot", id)
	}
}

func TestCoverageZFSItemsAndSkippedMembers(t *testing.T) {
	s, st, _ := zfsDomainFixture(t)
	scheduled, err := st.CreateZFSDataset(store.ZFSDataset{Dataset: "cache/appdata", Enabled: true})
	if err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := st.CreateZFSDataset(store.ZFSDataset{Dataset: "cache/media", Enabled: false}); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if err := st.ReplaceZFSMembers(scheduled.ID, []store.ZFSMember{
		{ItemID: scheduled.ID, Dataset: "cache/appdata", Outcome: "backed-up"},
		{ItemID: scheduled.ID, Dataset: "cache/appdata/fresh", Outcome: ""},
		{ItemID: scheduled.ID, Dataset: "cache/appdata/secret", Outcome: "key-not-loaded"},
		{ItemID: scheduled.ID, Dataset: "cache/appdata/broken", Outcome: "backup-failed"},
		{ItemID: scheduled.ID, Dataset: "cache/appdata/disk", Outcome: "zvol"},
		{ItemID: scheduled.ID, Dataset: "cache/appdata/struct", Outcome: "canmount-off", UsedByDataset: 128 << 10},
		{ItemID: scheduled.ID, Dataset: "cache/appdata/hidden", Outcome: "canmount-off", UsedByDataset: 5 << 30},
		{ItemID: scheduled.ID, Dataset: "cache/appdata/cachey", Outcome: "excluded"},
	}); err != nil {
		t.Fatalf("write members: %v", err)
	}

	report, err := s.Coverage(context.Background())
	if err != nil {
		t.Fatalf("coverage: %v", err)
	}
	var domain CoverageDomain
	for _, d := range report.Domains {
		if d.Domain == zfsDomain {
			domain = d
		}
	}
	if domain.Domain == "" {
		t.Fatalf("no zfs domain in %+v", report.Domains)
	}
	codes := map[string]string{}
	for _, item := range domain.Unprotected {
		codes[item.Name] = item.Code
		if item.Code != "" && item.Reason != CoverageZFSMemberSkipped {
			t.Fatalf("%s carries code %q under reason %q", item.Name, item.Code, item.Reason)
		}
	}
	if _, ok := codes["cache/media"]; !ok {
		t.Fatalf("the item nothing schedules is missing from %v", codes)
	}
	for dataset, code := range map[string]string{
		"cache/appdata/secret": "key-not-loaded",
		"cache/appdata/broken": "backup-failed",
		"cache/appdata/hidden": "canmount-off",
	} {
		if codes[dataset] != code {
			t.Fatalf("%s has code %q in %v, want %q", dataset, codes[dataset], codes, code)
		}
	}
	for _, unwanted := range []string{
		"cache/appdata/fresh",
		"cache/appdata",
		"cache/appdata/disk",
		"cache/appdata/struct",
		"cache/appdata/cachey",
	} {
		if _, ok := codes[unwanted]; ok {
			t.Fatalf("%s must not count as unprotected", unwanted)
		}
	}
}

func TestCoverageCountsAnItemWithASkippedMemberAsUnprotected(t *testing.T) {
	s, st, _ := zfsDomainFixture(t)
	gappy, err := st.CreateZFSDataset(store.ZFSDataset{Dataset: "cache/appdata", Enabled: true})
	if err != nil {
		t.Fatalf("create item: %v", err)
	}
	whole, err := st.CreateZFSDataset(store.ZFSDataset{Dataset: "cache/media", Enabled: true})
	if err != nil {
		t.Fatalf("create item: %v", err)
	}
	if err := st.ReplaceZFSMembers(gappy.ID, []store.ZFSMember{
		{ItemID: gappy.ID, Dataset: "cache/appdata", Outcome: "backed-up"},
		{ItemID: gappy.ID, Dataset: "cache/appdata/nextcloud", Outcome: "not-mounted"},
	}); err != nil {
		t.Fatalf("write members: %v", err)
	}
	if err := st.ReplaceZFSMembers(whole.ID, []store.ZFSMember{
		{ItemID: whole.ID, Dataset: "cache/media", Outcome: "backed-up"},
	}); err != nil {
		t.Fatalf("write members: %v", err)
	}

	report, err := s.Coverage(context.Background())
	if err != nil {
		t.Fatalf("coverage: %v", err)
	}
	for _, d := range report.Domains {
		if d.Domain != zfsDomain {
			continue
		}
		if d.Total != 2 || d.Protected != 1 {
			t.Fatalf("the ZFS domain counts %d of %d protected, want 1 of 2", d.Protected, d.Total)
		}
		return
	}
	t.Fatalf("no zfs domain in %+v", report.Domains)
}

func TestWatchdogIncludesZFS(t *testing.T) {
	s, st, _ := zfsDomainFixture(t)
	bodies := zfsCaptureNotifications(t, s)
	item, err := st.CreateZFSDataset(store.ZFSDataset{Dataset: "cache/appdata", Enabled: true})
	if err != nil {
		t.Fatalf("create item: %v", err)
	}
	runID, err := st.StartRun(item.ID, "backup")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	if err := st.FinishRun(runID, "success", "snap", 1, ""); err != nil {
		t.Fatalf("finish run: %v", err)
	}

	if err := s.runWatchdogAt(context.Background(), time.Now().Add(72*time.Hour).Unix()); err != nil {
		t.Fatalf("watchdog: %v", err)
	}
	if _, have, err := st.GetWatchdogState(zfsDomain); err != nil || !have {
		t.Fatalf("no watchdog episode recorded for zfs (have=%v, err=%v)", have, err)
	}
	if !zfsAnyMessageContains(bodies(), "zfs") {
		t.Fatalf("no overdue message names the domain: %v", bodies())
	}
}

func TestReceiverItemTagZFS(t *testing.T) {
	got := receiverItemTag(restic.Snapshot{Tags: []string{"p1", "zfs:cache/appdata"}})
	if got != "zfs:cache/appdata" {
		t.Fatalf("receiverItemTag = %q", got)
	}
	if bare := receiverItemTag(restic.Snapshot{Tags: []string{"zfs:"}}); bare != "untagged" {
		t.Fatalf("a prefix without a dataset = %q, want untagged", bare)
	}
}

func TestRecoveryKitHasZFSSection(t *testing.T) {
	s, st, _ := zfsDomainFixture(t)
	item, err := st.CreateZFSDataset(store.ZFSDataset{
		Dataset:          "cache/appdata",
		Enabled:          true,
		ExcludedChildren: []string{"cache/appdata/cachey"},
		StopContainers:   []string{"plex"},
	})
	if err != nil {
		t.Fatalf("create item: %v", err)
	}
	if _, err := st.SetZFSCheck(item.ID, "ok", "", "/mnt/cache/appdata", 1); err != nil {
		t.Fatalf("record mountpoint: %v", err)
	}
	if err := st.ReplaceZFSMembers(item.ID, []store.ZFSMember{
		{ItemID: item.ID, Dataset: "cache/appdata", Outcome: "backed-up"},
		{ItemID: item.ID, Dataset: "cache/appdata/plex", Outcome: "backed-up"},
	}); err != nil {
		t.Fatalf("write members: %v", err)
	}

	kit, err := s.RecoveryKit()
	if err != nil {
		t.Fatalf("recovery kit: %v", err)
	}
	for _, want := range []string{
		"## ZFS datasets",
		"- zfs (local):",
		"- cache/appdata: repository ",
		"mounted at /mnt/cache/appdata",
		"datasets: cache/appdata, cache/appdata/plex",
		"left out: cache/appdata/cachey",
		"stopped for the snapshot: plex",
		"restic -r '<repo>' snapshots --tag 'zfs:<dataset>'",
		"zfs snapshot '<dataset>@before-restore'",
	} {
		if !strings.Contains(kit, want) {
			t.Fatalf("recovery kit is missing %q", want)
		}
	}
}

func TestRepoSharedWithAnotherDomainSeesZFS(t *testing.T) {
	s, st, settings := zfsDomainFixture(t)
	named, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Role: store.RoleRepo, Name: "pool", Repo: "bombvault/pool", Enabled: true,
	})
	if err != nil {
		t.Fatalf("create named repository: %v", err)
	}
	if _, err := st.CreateZFSDataset(store.ZFSDataset{Dataset: "cache/appdata", Enabled: true, Repo: named.ID}); err != nil {
		t.Fatalf("create item: %v", err)
	}

	ref := domainRepoRef{Named: named}
	if !s.repoSharedWithAnotherDomain(settings, "files", ref) {
		t.Fatal("a repository a ZFS item writes to is shared with the ZFS domain")
	}
}

// A tree whose root was always skipped comes back from Discover as its
// children, so the result has to say the roots want a look.
func TestDiscoverCountsRootsThatMayBeChildren(t *testing.T) {
	_, st, _ := zfsDomainFixture(t)
	h := &Handler{store: st}

	for _, name := range []string{"cache/appdata/plex", "cache/appdata/db"} {
		if _, err := st.CreateZFSDataset(store.ZFSDataset{Dataset: name}); err != nil {
			t.Fatalf("create %q: %v", name, err)
		}
	}
	if got := h.zfsRootsToCheck(); got != 2 {
		t.Fatalf("roots to check = %d, want both children of the missing root", got)
	}

	if _, err := st.CreateZFSDataset(store.ZFSDataset{Dataset: "tank/media"}); err != nil {
		t.Fatalf("create tank/media: %v", err)
	}
	if got := h.zfsRootsToCheck(); got != 2 {
		t.Fatalf("roots to check = %d, an item alone under its parent is not in doubt", got)
	}
}
