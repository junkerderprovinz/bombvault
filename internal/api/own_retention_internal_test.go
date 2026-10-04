package api

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// withOwnRetention stores the shared keep-last and the given per-domain
// policies, and returns the settings as stored.
func withOwnRetention(t *testing.T, st *store.Repo, sharedKeepLast int, own map[string]store.RetentionKeep) store.Settings {
	t.Helper()
	s, err := st.MutateSettings(func(s *store.Settings) error {
		s.RetentionKeepLast = sharedKeepLast
		s.SetOwnRetention(own)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRetentionPolicyFallsBackToTheSharedOne(t *testing.T) {
	var s store.Settings
	s.RetentionKeepLast = 5
	s.RetentionKeepDaily = 2
	s.SetOwnRetention(map[string]store.RetentionKeep{
		"vms":   {KeepWeekly: 4, KeepMonthly: 6},
		"flash": {},
	})
	svc := &Service{}
	for _, tc := range []struct {
		domain string
		want   restic.RetentionPolicy
	}{
		{"containers", restic.RetentionPolicy{KeepLast: 5, KeepDaily: 2}},
		{"vms", restic.RetentionPolicy{KeepWeekly: 4, KeepMonthly: 6}},
		{"flash", restic.RetentionPolicy{}},
		{zfsDomain, restic.RetentionPolicy{KeepLast: 5, KeepDaily: 2}},
	} {
		if got := svc.retentionPolicy(s, tc.domain); got != tc.want {
			t.Errorf("retentionPolicy(%s) = %+v, want %+v", tc.domain, got, tc.want)
		}
	}
}

func TestAnUndecodableOverrideFallsBackToTheSharedPolicy(t *testing.T) {
	s := store.Settings{RetentionKeepLast: 5, RetentionOverrides: "{not json"}
	if got := (&Service{}).retentionPolicy(s, "vms"); got != (restic.RetentionPolicy{KeepLast: 5}) {
		t.Fatalf("retentionPolicy = %+v, want the shared keep-last 5", got)
	}
}

func TestTheOffsitePolicyIgnoresTheDomainsOwnOne(t *testing.T) {
	s := store.Settings{RetentionKeepLast: 5, OffsiteRetentionKeepMonthly: 12}
	s.SetOwnRetention(map[string]store.RetentionKeep{"vms": {KeepWeekly: 4}})
	svc := &Service{}
	if got := svc.retentionPolicyForSource(s, "vms", "offsite"); got != (restic.RetentionPolicy{KeepMonthly: 12, Direct: true}) {
		t.Fatalf("off-site policy = %+v, want the off-site keep-monthly 12", got)
	}
	if got := svc.retentionPolicyForSource(s, "vms", "local"); got != (restic.RetentionPolicy{KeepWeekly: 4}) {
		t.Fatalf("local policy = %+v, want the domain's own keep-weekly 4", got)
	}
}

func TestABackupAgesByItsDomainsOwnPolicy(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.container("nginx", nas.ID)
	settings := withOwnRetention(t, f.st, 3, map[string]store.RetentionKeep{"containers": {KeepDaily: 7}})
	f.svc.applyRetention(context.Background(), f.root+"/nas", settings, restic.Mode{}, tagIdentity("container:nginx"), "containers", anomalyScope{})
	want := []forgetCall{{Repo: f.root + "/nas", Tags: []string{"container:nginx"}, Policy: restic.RetentionPolicy{KeepDaily: 7}, Prune: true}}
	if !reflect.DeepEqual(f.eng.forgets, want) {
		t.Fatalf("forgets = %+v, want %+v", f.eng.forgets, want)
	}
}

func TestAnotherDomainsOwnPolicyLeavesTheSharedOneInPlace(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.container("nginx", nas.ID)
	settings := withOwnRetention(t, f.st, 3, map[string]store.RetentionKeep{"vms": {KeepWeekly: 2}})
	f.svc.applyRetention(context.Background(), f.root+"/nas", settings, restic.Mode{}, tagIdentity("container:nginx"), "containers", anomalyScope{})
	if len(f.eng.forgets) != 1 || f.eng.forgets[0].Policy != (restic.RetentionPolicy{KeepLast: 3}) {
		t.Fatalf("forgets = %+v, want one with the shared keep-last 3", f.eng.forgets)
	}
}

func TestAnOwnPolicyThatKeepsEverythingSkipsTheBackupsRetention(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.container("nginx", nas.ID)
	settings := withOwnRetention(t, f.st, 3, map[string]store.RetentionKeep{"containers": {}})
	f.svc.applyRetention(context.Background(), f.root+"/nas", settings, restic.Mode{}, tagIdentity("container:nginx"), "containers", anomalyScope{})
	if len(f.eng.forgets) != 0 {
		t.Fatalf("forgets = %+v, want none under an own policy of all zero", f.eng.forgets)
	}
}

func TestADatabaseDumpAgesByTheContainersOwnPolicy(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.container("nginx", nas.ID)
	settings := withOwnRetention(t, f.st, 3, map[string]store.RetentionKeep{"containers": {KeepMonthly: 4}})
	f.svc.forgetDBDumpSeries(context.Background(), f.root+"/nas", settings, restic.Mode{}, "nginx", "t1")
	if len(f.eng.forgets) != 1 || f.eng.forgets[0].Policy != (restic.RetentionPolicy{KeepMonthly: 4}) {
		t.Fatalf("forgets = %+v, want one with the containers' own keep-monthly 4", f.eng.forgets)
	}
}

func TestZFSDatasetsAgeByTheirOwnPolicy(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.container("nginx", nas.ID)
	settings := withOwnRetention(t, f.st, 3, map[string]store.RetentionKeep{zfsDomain: {KeepWeekly: 8}})
	f.svc.applyRetentionTags(context.Background(), f.root+"/nas", settings, restic.Mode{}, []string{"zfs:tank/media"}, zfsDomain)
	if len(f.eng.forgets) != 1 || f.eng.forgets[0].Policy != (restic.RetentionPolicy{KeepWeekly: 8}) {
		t.Fatalf("forgets = %+v, want one with the ZFS domain's own keep-weekly 8", f.eng.forgets)
	}
}

func TestAManualPruneAgesByTheDomainsOwnPolicy(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.container("nginx", nas.ID)
	withOwnRetention(t, f.st, 3, map[string]store.RetentionKeep{"containers": {KeepLast: 10, KeepYearly: 1}})
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:plex"))
	f.hold(f.root+"/nas", snap("b1", 100, "container:nginx"))
	if _, err := f.svc.pruneDomain(context.Background(), "containers", "local", false); err != nil {
		t.Fatalf("pruneDomain: %v", err)
	}
	if len(f.eng.forgets) != 2 {
		t.Fatalf("forgets = %+v, want one per repository", f.eng.forgets)
	}
	for _, c := range f.eng.forgets {
		if c.Policy != (restic.RetentionPolicy{KeepLast: 10, KeepYearly: 1}) {
			t.Errorf("%s forgot with %+v, want the domain's own policy", c.Repo, c.Policy)
		}
	}
}

// sharedWithAVM places nginx and the VM win11 on the named repository nas and
// leaves one snapshot of each item there and one of plex in the containers
// repository.
func sharedWithAVM(f *placementFixture) {
	f.t.Helper()
	nas := f.namedRepo("NAS", "nas")
	f.container("nginx", nas.ID)
	f.vm("win11", nas.ID)
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:plex"))
	f.hold(f.root+"/nas", snap("b1", 100, "container:nginx"), snap("c1", 100, "vm:win11"))
}

func TestEveryIdentityTagBelongsToItsDomain(t *testing.T) {
	for tag, want := range map[string]string{
		"container:plex":     "containers",
		"dbdump:immich":      "containers",
		"stack:immich":       "containers",
		"vm:win11":           "vms",
		"vm:win11:zvol:vda":  "vms",
		"fileset:docs":       "files",
		"zfs:tank/media":     zfsDomain,
		"flash":              "flash",
		"config":             "config",
		"engine:postgres":    "",
		"containerless:plex": "",
	} {
		if got := identityDomain(tag); got != want {
			t.Errorf("identityDomain(%q) = %q, want %q", tag, got, want)
		}
	}
}

func TestAManualPruneLeavesAnotherDomainsBackupsInASharedRepository(t *testing.T) {
	f := newPlacementFixture(t)
	sharedWithAVM(f)
	withOwnRetention(t, f.st, 3, map[string]store.RetentionKeep{"containers": {KeepDaily: 7}})
	if _, err := f.svc.pruneDomain(context.Background(), "containers", "local", false); err != nil {
		t.Fatalf("pruneDomain: %v", err)
	}
	want := []forgetCall{
		{Repo: f.domainPath("containers"), Tags: []string{"container:plex"}, Policy: restic.RetentionPolicy{KeepDaily: 7}},
		{Repo: f.root + "/nas", Tags: []string{"container:nginx"}, Policy: restic.RetentionPolicy{KeepDaily: 7}},
	}
	if !reflect.DeepEqual(f.eng.forgets, want) {
		t.Fatalf("forgets = %+v, want only the containers under their own policy; win11 ages by the VMs' rules", f.eng.forgets)
	}
	if !slices.Contains(f.eng.prunes, f.root+"/nas") {
		t.Fatalf("prunes = %v, want the shared repository's space reclaimed", f.eng.prunes)
	}
}

func TestThePreviewOfAPruneLeavesAnotherDomainsBackupsOut(t *testing.T) {
	f := newPlacementFixture(t)
	sharedWithAVM(f)
	withOwnRetention(t, f.st, 3, map[string]store.RetentionKeep{"containers": {KeepDaily: 7}})
	got, err := f.svc.PreviewRetention(context.Background(), "containers", "local")
	if err != nil {
		t.Fatalf("PreviewRetention: %v", err)
	}
	var tags []string
	for _, r := range got.Repos {
		for _, it := range r.Items {
			tags = append(tags, it.Tag)
		}
	}
	slices.Sort(tags)
	if want := []string{"container:nginx", "container:plex"}; !slices.Equal(tags, want) {
		t.Fatalf("preview items = %v, want %v", tags, want)
	}
}

func TestTheBatchedPruneFollowsTheDomainsOwnPolicy(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.container("nginx", nas.ID)

	withOwnRetention(t, f.st, 3, map[string]store.RetentionKeep{"containers": {}})
	f.svc.PruneAfterBulk(context.Background(), "containers")
	if len(f.eng.prunes) != 0 {
		t.Fatalf("an own policy that keeps everything still pruned %v", f.eng.prunes)
	}

	withOwnRetention(t, f.st, 0, map[string]store.RetentionKeep{"containers": {KeepDaily: 7}})
	f.svc.PruneAfterBulk(context.Background(), "containers")
	if len(f.eng.prunes) != 2 {
		t.Fatalf("with only an own policy the batched prune reached %v, want both repositories", f.eng.prunes)
	}
}

func TestTheStartGuardReadsTheDomainsOwnPolicy(t *testing.T) {
	h, repo, _ := newMCPStartHandler(t)
	s := withOwnRetention(t, repo, 0, map[string]store.RetentionKeep{"files": {KeepYearly: 2}})
	s.RetentionKeepDaily = 7
	if n, err := h.mcpKeepLast(s, "files"); err != nil || n != 1 {
		t.Fatalf("mcpKeepLast(files) = %d, %v; a yearly-only own policy keeps a single recent restore point", n, err)
	}
	if n, err := h.mcpKeepLast(s, "containers"); err != nil || n != 0 {
		t.Fatalf("mcpKeepLast(containers) = %d, %v; the shared daily rule has no count-only window", n, err)
	}
}

func TestASettingsSaveKeepsOwnRetentionItDoesNotMention(t *testing.T) {
	f := newPlacementFixture(t)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	v := toView(settings)
	v.OwnRetention = map[string]store.RetentionKeep{"vms": {KeepWeekly: 4, KeepMonthly: -2}}
	if res := f.do("PUT", "/api/settings", v); res["ok"] != true {
		t.Fatalf("PUT: %v", res["error"])
	}
	stored := func() map[string]store.RetentionKeep {
		s, err := f.st.GetSettings()
		if err != nil {
			t.Fatal(err)
		}
		return s.OwnRetention()
	}
	if got := stored(); !reflect.DeepEqual(got, map[string]store.RetentionKeep{"vms": {KeepWeekly: 4}}) {
		t.Fatalf("stored %+v, want vms on weekly 4 with the negative count clamped", got)
	}

	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var older map[string]any
	if err := json.Unmarshal(b, &older); err != nil {
		t.Fatal(err)
	}
	delete(older, "ownRetention")
	if res := f.do("PUT", "/api/settings", older); res["ok"] != true {
		t.Fatalf("PUT without ownRetention: %v", res["error"])
	}
	if got := stored(); len(got) != 1 {
		t.Fatalf("a client that does not send ownRetention wiped it: %+v", got)
	}

	v.OwnRetention = map[string]store.RetentionKeep{}
	if res := f.do("PUT", "/api/settings", v); res["ok"] != true {
		t.Fatalf("PUT: %v", res["error"])
	}
	if got := stored(); len(got) != 0 {
		t.Fatalf("an empty map must put every domain back on the shared policy, got %+v", got)
	}

	v.OwnRetention = map[string]store.RetentionKeep{"photos": {KeepLast: 1}}
	if res := f.do("PUT", "/api/settings", v); res["ok"] != false {
		t.Fatalf("a keep-policy for something that is not a domain was stored: %v", res)
	}
}

func TestSettingsExportImportCarriesOwnRetention(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	own := map[string]store.RetentionKeep{"containers": {KeepDaily: 7}, zfsDomain: {KeepLast: 2, KeepYearly: 1}}
	withOwnRetention(t, srcStore, 3, own)
	body, _ := doExport(t, src, "")

	dst, dstStore := newPortableHandler(t, appKeyB)
	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply failed: %v", env)
	}
	got, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.OwnRetention(), own) {
		t.Fatalf("own retention = %+v, want %+v", got.OwnRetention(), own)
	}
}

func TestImportOfFileWithoutOwnRetentionKeepsTheStoredOnes(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	body, _ := doExport(t, src, "")
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	delete(raw["settings"].(map[string]any), "ownRetention")
	older, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}

	dst, dstStore := newPortableHandler(t, appKeyB)
	own := map[string]store.RetentionKeep{"vms": {KeepMonthly: 6}}
	withOwnRetention(t, dstStore, 0, own)
	if env := doImport(t, dst, older, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply failed: %v", env)
	}
	got, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.OwnRetention(), own) {
		t.Fatalf("own retention = %+v, want the stored %+v", got.OwnRetention(), own)
	}
}

func TestAnImportedKeepPolicyForSomethingThatIsNotADomainIsRefused(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	body, _ := doExport(t, src, "")
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	raw["settings"].(map[string]any)["ownRetention"] = map[string]any{"photos": map[string]int{"keepLast": 1}}
	bad, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	dst, _ := newPortableHandler(t, appKeyB)
	if env := doImport(t, dst, bad, "?apply=true"); env["ok"] == true {
		t.Fatal("an import stored a keep-policy for something that is not a domain")
	}
}
