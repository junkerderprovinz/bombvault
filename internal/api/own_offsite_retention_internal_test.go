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

// withOwnOffsiteRetention gives containers the built-in off-site repo
// offsite/containers and stores the shared off-site keep-last and the given
// per-domain off-site policies. It returns the settings as stored.
func withOwnOffsiteRetention(f *placementFixture, sharedKeepLast int, own map[string]store.RetentionKeep) store.Settings {
	f.t.Helper()
	if _, err := f.st.MutateSettings(func(s *store.Settings) error {
		s.OffsiteRetentionKeepLast = sharedKeepLast
		s.SetOwnOffsiteRetention(own)
		return nil
	}); err != nil {
		f.t.Fatal(err)
	}
	f.fieldTarget("containers", "offsite/containers")
	s, err := f.st.GetSettings()
	if err != nil {
		f.t.Fatal(err)
	}
	return s
}

// replicateContainers runs the copy pass of the containers domain from its own
// repository. The domain counts as replicated before, so the pass does not
// pause at its first look at a target that already holds its items.
func replicateContainers(f *placementFixture, settings store.Settings) {
	f.t.Helper()
	f.replicated("containers")
	local := []domainRepoRef{ownRef(f.domainPath("containers"))}
	if err := f.svc.copyToOffsite(context.Background(), "containers", settings, "", local, nil); err != nil {
		f.t.Fatalf("copyToOffsite: %v", err)
	}
}

func TestOffsiteRetentionPolicyFallsBackToTheSharedOne(t *testing.T) {
	var s store.Settings
	s.OffsiteRetentionKeepMonthly = 12
	s.SetOwnOffsiteRetention(map[string]store.RetentionKeep{
		"vms":   {KeepWeekly: 4},
		"flash": {},
	})
	s.SetOwnRetention(map[string]store.RetentionKeep{"files": {KeepDaily: 7}})
	svc := &Service{}
	for _, tc := range []struct {
		domain string
		want   restic.RetentionPolicy
	}{
		{"containers", restic.RetentionPolicy{KeepMonthly: 12}},
		{"vms", restic.RetentionPolicy{KeepWeekly: 4}},
		{"flash", restic.RetentionPolicy{}},
		{"files", restic.RetentionPolicy{KeepMonthly: 12}},
	} {
		if got := svc.offsiteRetentionPolicy(s, tc.domain); got != tc.want {
			t.Errorf("offsiteRetentionPolicy(%s) = %+v, want %+v", tc.domain, got, tc.want)
		}
	}
	if got := svc.retentionPolicy(s, "vms"); got != (restic.RetentionPolicy{}) {
		t.Errorf("the local policy of vms = %+v, want the shared local one, untouched by its off-site rules", got)
	}
}

func TestTheCopyPassAgesTheBuiltInOffsiteRepoByTheSourcesOwnPolicy(t *testing.T) {
	f := newPlacementFixture(t)
	settings := withOwnOffsiteRetention(f, 3, map[string]store.RetentionKeep{"containers": {KeepDaily: 14}})
	dest := f.root + "/offsite/containers"
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:plex"))
	f.hold(dest, snap("o1", 100, "container:plex"), snap("o2", 100, "vm:win11"))

	replicateContainers(f, settings)

	got := forgetsAt(f, dest)
	want := []forgetCall{{Repo: dest, Tags: []string{"container:plex"}, Policy: restic.RetentionPolicy{KeepDaily: 14}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("off-site forgets = %+v, want only the containers under their own off-site rules; win11 ages on the VMs' pass", got)
	}
}

func TestAnotherSourcesOwnOffsitePolicyLeavesTheSharedOneOnTheCopyPass(t *testing.T) {
	f := newPlacementFixture(t)
	settings := withOwnOffsiteRetention(f, 3, map[string]store.RetentionKeep{"vms": {KeepWeekly: 2}})
	dest := f.root + "/offsite/containers"
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:plex"))
	f.hold(dest, snap("o1", 100, "container:plex"))

	replicateContainers(f, settings)

	got := forgetsAt(f, dest)
	if len(got) != 1 || got[0].Policy != (restic.RetentionPolicy{KeepLast: 3}) {
		t.Fatalf("off-site forgets = %+v, want one with the shared off-site keep-last 3", got)
	}
}

func TestASettingsSaveCarriesTheOwnOffsitePolicyToTheCopyPass(t *testing.T) {
	f := newPlacementFixture(t)
	withOwnOffsiteRetention(f, 3, nil)
	extra, err := f.st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "B2", Repo: "extra/containers", Enabled: true, SortOrder: 1, RetentionKeepLast: 9,
	})
	if err != nil {
		t.Fatal(err)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	v := toView(settings)
	v.OwnOffsiteRetention = map[string]store.RetentionKeep{"containers": {KeepDaily: 14, KeepMonthly: -1}}
	if res := f.do("PUT", "/api/settings", v); res["ok"] != true {
		t.Fatalf("PUT: %v", res["error"])
	}

	targets, err := f.st.OffsiteTargetsForDomain("containers")
	if err != nil {
		t.Fatal(err)
	}
	var primary store.OffsiteTarget
	for _, tg := range targets {
		if tg.SortOrder == 0 {
			primary = tg
		}
	}
	if primary.Repo != "offsite/containers" || primary.RetentionKeepDaily != 14 || primary.RetentionKeepLast != 0 || primary.RetentionKeepMonthly != 0 {
		t.Fatalf("primary target = %+v, want offsite/containers on the containers' own daily 14", primary)
	}

	dest := f.root + "/offsite/containers"
	extraDest := f.root + "/extra/containers"
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:plex"))
	f.hold(dest, snap("o1", 100, "container:plex"))
	f.hold(extraDest, snap("e1", 100, "container:plex"))
	stored, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	replicateContainers(f, stored)

	if got := forgetsAt(f, dest); len(got) != 1 || got[0].Policy != (restic.RetentionPolicy{KeepDaily: 14}) {
		t.Fatalf("built-in off-site forgets = %+v, want the containers' own daily 14", got)
	}
	if got := forgetsAt(f, extraDest); len(got) != 1 || got[0].Policy != (restic.RetentionPolicy{KeepLast: 9}) {
		t.Fatalf("forgets on the additional target %s = %+v, want its own keep-last 9", extra.Name, got)
	}
}

func TestAManualOffsitePruneAgesByTheSourcesOwnOffsitePolicy(t *testing.T) {
	f := newPlacementFixture(t)
	withOwnOffsiteRetention(f, 3, map[string]store.RetentionKeep{"containers": {KeepWeekly: 8}})
	dest := f.root + "/offsite/containers"
	f.makeRepo(dest)
	f.hold(dest, snap("o1", 100, "container:plex"), snap("o2", 100, "vm:win11"))

	if _, err := f.svc.pruneDomain(context.Background(), "containers", "offsite", false); err != nil {
		t.Fatalf("pruneDomain: %v", err)
	}
	want := []forgetCall{{Repo: dest, Tags: []string{"container:plex"}, Policy: restic.RetentionPolicy{KeepWeekly: 8}}}
	if got := forgetsAt(f, dest); !reflect.DeepEqual(got, want) {
		t.Fatalf("forgets = %+v, want %+v", got, want)
	}
}

func TestAManualOffsitePruneOfAnotherSourceKeepsTheSharedOffsitePolicy(t *testing.T) {
	f := newPlacementFixture(t)
	withOwnOffsiteRetention(f, 3, map[string]store.RetentionKeep{"vms": {KeepWeekly: 8}})
	dest := f.root + "/offsite/containers"
	f.makeRepo(dest)
	f.hold(dest, snap("o1", 100, "container:plex"))

	if _, err := f.svc.pruneDomain(context.Background(), "containers", "offsite", false); err != nil {
		t.Fatalf("pruneDomain: %v", err)
	}
	if got := forgetsAt(f, dest); len(got) != 1 || got[0].Policy != (restic.RetentionPolicy{KeepLast: 3}) {
		t.Fatalf("forgets = %+v, want one with the shared off-site keep-last 3", got)
	}
}

func TestAManualPruneOfAnAdditionalTargetIgnoresTheSourcesOwnOffsitePolicy(t *testing.T) {
	f := newPlacementFixture(t)
	withOwnOffsiteRetention(f, 3, map[string]store.RetentionKeep{"containers": {KeepWeekly: 8}})
	extra, err := f.st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain: "containers", Name: "B2", Repo: "extra/containers", Enabled: true, SortOrder: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	dest := f.root + "/extra/containers"
	f.makeRepo(dest)
	f.hold(dest, snap("e1", 100, "container:plex"))

	if _, err := f.svc.pruneDomain(context.Background(), "containers", offsiteSourcePrefix+extra.ID, false); err != nil {
		t.Fatalf("pruneDomain: %v", err)
	}
	if got := forgetsAt(f, dest); len(got) != 1 || got[0].Policy != (restic.RetentionPolicy{KeepLast: 3}) {
		t.Fatalf("forgets = %+v, want the shared off-site keep-last 3", got)
	}
}

func TestTheOffsitePreviewNamesTheSourcesOwnPolicy(t *testing.T) {
	f := newPlacementFixture(t)
	withOwnOffsiteRetention(f, 3, map[string]store.RetentionKeep{"containers": {KeepDaily: 14}})
	dest := f.root + "/offsite/containers"
	f.makeRepo(dest)
	f.hold(dest, snap("o1", 100, "container:plex"), snap("o2", 100, "vm:win11"))

	got, err := f.svc.PreviewRetention(context.Background(), "containers", "offsite")
	if err != nil {
		t.Fatalf("PreviewRetention: %v", err)
	}
	want := RetentionPolicyView{On: true, KeepDaily: 14, Own: true}
	if got.Policy != want {
		t.Fatalf("policy = %+v, want %+v", got.Policy, want)
	}
	var tags []string
	for _, r := range got.Repos {
		for _, it := range r.Items {
			tags = append(tags, it.Tag)
		}
	}
	if !slices.Equal(tags, []string{"container:plex"}) {
		t.Fatalf("preview items = %v, want only the containers", tags)
	}

	local, err := f.svc.PreviewRetention(context.Background(), "containers", "local")
	if err != nil {
		t.Fatalf("PreviewRetention local: %v", err)
	}
	if local.Policy.Own {
		t.Fatal("the local preview called the shared local rules the source's own")
	}
}

func TestTheStartGuardReadsTheSourcesOwnOffsitePolicy(t *testing.T) {
	h, repo, _ := newMCPStartHandler(t)
	s, err := repo.MutateSettings(func(s *store.Settings) error {
		s.FilesOffsite = "offsite/files"
		s.ContainersOffsite = "offsite/containers"
		s.OffsiteRetentionKeepLast = 6
		s.SetOwnOffsiteRetention(map[string]store.RetentionKeep{"files": {KeepLast: 2}})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n, err := h.mcpKeepLast(s, "files"); err != nil || n != 2 {
		t.Fatalf("mcpKeepLast(files) = %d, %v; want the folder sets' own off-site keep-last 2", n, err)
	}
	if n, err := h.mcpKeepLast(s, "containers"); err != nil || n != 6 {
		t.Fatalf("mcpKeepLast(containers) = %d, %v; want the shared off-site keep-last 6", n, err)
	}
}

func TestASettingsSaveKeepsOwnOffsiteRetentionItDoesNotMention(t *testing.T) {
	f := newPlacementFixture(t)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	v := toView(settings)
	v.OwnOffsiteRetention = map[string]store.RetentionKeep{"vms": {KeepMonthly: 12}}
	if res := f.do("PUT", "/api/settings", v); res["ok"] != true {
		t.Fatalf("PUT: %v", res["error"])
	}
	stored := func() map[string]store.RetentionKeep {
		s, err := f.st.GetSettings()
		if err != nil {
			t.Fatal(err)
		}
		return s.OwnOffsiteRetention()
	}

	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var older map[string]any
	if err := json.Unmarshal(b, &older); err != nil {
		t.Fatal(err)
	}
	delete(older, "ownOffsiteRetention")
	if res := f.do("PUT", "/api/settings", older); res["ok"] != true {
		t.Fatalf("PUT without ownOffsiteRetention: %v", res["error"])
	}
	if got := stored(); !reflect.DeepEqual(got, map[string]store.RetentionKeep{"vms": {KeepMonthly: 12}}) {
		t.Fatalf("a client that does not send ownOffsiteRetention changed it: %+v", got)
	}

	v.OwnOffsiteRetention = map[string]store.RetentionKeep{}
	if res := f.do("PUT", "/api/settings", v); res["ok"] != true {
		t.Fatalf("PUT: %v", res["error"])
	}
	if got := stored(); len(got) != 0 {
		t.Fatalf("an empty map must put every domain back on the shared off-site policy, got %+v", got)
	}

	v.OwnOffsiteRetention = map[string]store.RetentionKeep{"photos": {KeepLast: 1}}
	if res := f.do("PUT", "/api/settings", v); res["ok"] != false {
		t.Fatalf("an off-site keep-policy for something that is not a domain was stored: %v", res)
	}
}

func TestASettingsSaveNotesAnOwnOffsitePolicyOnAnAppendOnlyRepo(t *testing.T) {
	f := newPlacementFixture(t)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	v := toView(settings)
	v.ContainersOffsite = "offsite/containers"
	v.ContainersOffsiteImmutable = true
	v.OwnOffsiteRetention = map[string]store.RetentionKeep{"containers": {KeepDaily: 14}}
	res := f.do("PUT", "/api/settings", v)
	if res["ok"] != true {
		t.Fatalf("PUT: %v", res["error"])
	}
	if n, _ := res["notes"].([]any); len(n) != 1 {
		t.Fatalf("notes = %v, want the append-only note for the containers' own off-site rules", res["notes"])
	}
}

func TestSettingsExportImportCarriesOwnOffsiteRetention(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	own := map[string]store.RetentionKeep{"containers": {KeepDaily: 14}, zfsDomain: {KeepYearly: 3}}
	if _, err := srcStore.MutateSettings(func(s *store.Settings) error {
		s.SetOwnOffsiteRetention(own)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	body, _ := doExport(t, src, "")

	dst, dstStore := newPortableHandler(t, appKeyB)
	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply failed: %v", env)
	}
	got, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.OwnOffsiteRetention(), own) {
		t.Fatalf("own off-site retention = %+v, want %+v", got.OwnOffsiteRetention(), own)
	}
}

func TestImportOfFileWithoutOwnOffsiteRetentionKeepsTheStoredOnes(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	body, _ := doExport(t, src, "")
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	delete(raw["settings"].(map[string]any), "ownOffsiteRetention")
	older, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}

	dst, dstStore := newPortableHandler(t, appKeyB)
	own := map[string]store.RetentionKeep{"vms": {KeepMonthly: 6}}
	if _, err := dstStore.MutateSettings(func(s *store.Settings) error {
		s.SetOwnOffsiteRetention(own)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if env := doImport(t, dst, older, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply failed: %v", env)
	}
	got, err := dstStore.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.OwnOffsiteRetention(), own) {
		t.Fatalf("own off-site retention = %+v, want the stored %+v", got.OwnOffsiteRetention(), own)
	}
}

func TestAnImportedOffsiteKeepPolicyForSomethingThatIsNotADomainIsRefused(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	body, _ := doExport(t, src, "")
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	raw["settings"].(map[string]any)["ownOffsiteRetention"] = map[string]any{"photos": map[string]int{"keepLast": 1}}
	bad, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	dst, _ := newPortableHandler(t, appKeyB)
	if env := doImport(t, dst, bad, "?apply=true"); env["ok"] == true {
		t.Fatal("an import stored an off-site keep-policy for something that is not a domain")
	}
}
