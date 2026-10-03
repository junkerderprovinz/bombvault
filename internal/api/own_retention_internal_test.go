package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// ownRetentionFixture is a Service and a Handler over an in-memory store whose
// containers domain has its own repository and a named one, nas, that nginx
// writes to.
type ownRetentionFixture struct {
	t    *testing.T
	svc  *Service
	h    *Handler
	st   *store.Repo
	eng  *ownRetentionEngine
	root string // host mount root, slash-spelled
}

func newOwnRetentionFixture(t *testing.T) *ownRetentionFixture {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := store.New(db)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	root := filepath.ToSlash(dir)
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: root}
	eng := &ownRetentionEngine{snaps: map[string][]restic.Snapshot{}}
	svc := NewService(cfg, st, nil, nil, eng)
	sched := schedule.New(func(string) error { return nil }, st.ListTargets)
	f := &ownRetentionFixture{t: t, svc: svc, h: NewHandler(cfg, st, nil, svc, sched, nil), st: st, eng: eng, root: root}

	f.makeRepo(f.root + "/backups/containers")
	f.makeRepo(f.root + "/nas")
	nas, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Role: store.RoleRepo, Name: "NAS", Repo: "nas", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertTarget(store.Target{ContainerName: "nginx"}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetTargetRepo("nginx", nas.ID); err != nil {
		t.Fatal(err)
	}
	return f
}

// makeRepo leaves a local repository the service treats as created.
func (f *ownRetentionFixture) makeRepo(loc string) {
	f.t.Helper()
	dir := filepath.FromSlash(loc)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config"), []byte("x"), 0o600); err != nil {
		f.t.Fatal(err)
	}
}

// hold sets what a listing of the location returns.
func (f *ownRetentionFixture) hold(location string, snaps ...restic.Snapshot) {
	f.eng.mu.Lock()
	defer f.eng.mu.Unlock()
	f.eng.snaps[filepath.ToSlash(location)] = snaps
}

// do sends one request through the router and returns its JSON body. Anything
// but 200 fails the test.
func (f *ownRetentionFixture) do(method, path string, body any) map[string]any {
	f.t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		f.t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	f.h.Router().ServeHTTP(rec, jsonReq(method, path, bytes.NewReader(b)))
	if rec.Code != http.StatusOK {
		f.t.Fatalf("%s %s = %d: %s", method, path, rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		f.t.Fatalf("%s %s: %v", method, path, err)
	}
	return out
}

type ownRetentionForget struct {
	Repo   string
	Tags   []string
	Policy restic.RetentionPolicy
	Prune  bool
}

// ownRetentionEngine answers listings per location and records the forgets
// and prunes a retention pass runs.
type ownRetentionEngine struct {
	ResticEngine
	mu      sync.Mutex
	snaps   map[string][]restic.Snapshot // by slash-spelled location
	forgets []ownRetentionForget
	prunes  []string
}

func (e *ownRetentionEngine) RepoOpens(context.Context, string, restic.Mode) bool { return true }

func (e *ownRetentionEngine) RepoOpensErr(context.Context, string, restic.Mode) error { return nil }

func (e *ownRetentionEngine) Snapshots(_ context.Context, repo string, _ restic.Mode) ([]restic.Snapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Clone(e.snaps[filepath.ToSlash(repo)]), nil
}

func (e *ownRetentionEngine) ForgetPolicy(_ context.Context, repo string, p restic.RetentionPolicy, _ restic.Mode, tags []string, prune bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.forgets = append(e.forgets, ownRetentionForget{Repo: filepath.ToSlash(repo), Tags: slices.Clone(tags), Policy: p, Prune: prune})
	return nil
}

func (e *ownRetentionEngine) Prune(_ context.Context, repo string, _ restic.Mode) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.prunes = append(e.prunes, filepath.ToSlash(repo))
	return nil
}

// ForgetPreview keeps every snapshot under the tag.
func (e *ownRetentionEngine) ForgetPreview(_ context.Context, repo string, _ restic.RetentionPolicy, _ restic.Mode, tag string) ([]restic.ForgetGroup, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return []restic.ForgetGroup{{Tags: []string{tag}, Keep: snapshotsTagged(e.snaps[filepath.ToSlash(repo)], tag)}}, nil
}

func (e *ownRetentionEngine) Unlock(context.Context, string, bool, restic.Mode) error { return nil }

func (e *ownRetentionEngine) Stats(context.Context, string, string, restic.Mode) (restic.StatsResult, error) {
	return restic.StatsResult{}, nil
}

func ownRetentionSnap(id string, at int64, tags ...string) restic.Snapshot {
	return restic.Snapshot{ID: id, Time: time.Unix(at, 0).UTC().Format(time.RFC3339), Tags: tags}
}

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
	if got := svc.retentionPolicyForSource(s, "vms", "offsite"); got != (restic.RetentionPolicy{KeepMonthly: 12}) {
		t.Fatalf("off-site policy = %+v, want the off-site keep-monthly 12", got)
	}
	if got := svc.retentionPolicyForSource(s, "vms", "local"); got != (restic.RetentionPolicy{KeepWeekly: 4}) {
		t.Fatalf("local policy = %+v, want the domain's own keep-weekly 4", got)
	}
}

func TestABackupAgesByItsDomainsOwnPolicy(t *testing.T) {
	f := newOwnRetentionFixture(t)
	settings := withOwnRetention(t, f.st, 3, map[string]store.RetentionKeep{"containers": {KeepDaily: 7}})
	f.svc.applyRetention(context.Background(), f.root+"/nas", settings, restic.Mode{}, tagIdentity("container:nginx"), "containers", anomalyScope{})
	want := []ownRetentionForget{{Repo: f.root + "/nas", Tags: []string{"container:nginx"}, Policy: restic.RetentionPolicy{KeepDaily: 7}, Prune: true}}
	if !reflect.DeepEqual(f.eng.forgets, want) {
		t.Fatalf("forgets = %+v, want %+v", f.eng.forgets, want)
	}
}

func TestAnotherDomainsOwnPolicyLeavesTheSharedOneInPlace(t *testing.T) {
	f := newOwnRetentionFixture(t)
	settings := withOwnRetention(t, f.st, 3, map[string]store.RetentionKeep{"vms": {KeepWeekly: 2}})
	f.svc.applyRetention(context.Background(), f.root+"/nas", settings, restic.Mode{}, tagIdentity("container:nginx"), "containers", anomalyScope{})
	if len(f.eng.forgets) != 1 || f.eng.forgets[0].Policy != (restic.RetentionPolicy{KeepLast: 3}) {
		t.Fatalf("forgets = %+v, want one with the shared keep-last 3", f.eng.forgets)
	}
}

func TestAnOwnPolicyThatKeepsEverythingSkipsTheBackupsRetention(t *testing.T) {
	f := newOwnRetentionFixture(t)
	settings := withOwnRetention(t, f.st, 3, map[string]store.RetentionKeep{"containers": {}})
	f.svc.applyRetention(context.Background(), f.root+"/nas", settings, restic.Mode{}, tagIdentity("container:nginx"), "containers", anomalyScope{})
	if len(f.eng.forgets) != 0 {
		t.Fatalf("forgets = %+v, want none under an own policy of all zero", f.eng.forgets)
	}
}

func TestADatabaseDumpAgesByTheContainersOwnPolicy(t *testing.T) {
	f := newOwnRetentionFixture(t)
	settings := withOwnRetention(t, f.st, 3, map[string]store.RetentionKeep{"containers": {KeepMonthly: 4}})
	f.svc.forgetDBDumpSeries(context.Background(), f.root+"/nas", settings, restic.Mode{}, "nginx", "t1")
	if len(f.eng.forgets) != 1 || f.eng.forgets[0].Policy != (restic.RetentionPolicy{KeepMonthly: 4}) {
		t.Fatalf("forgets = %+v, want one with the containers' own keep-monthly 4", f.eng.forgets)
	}
}

func TestZFSDatasetsAgeByTheirOwnPolicy(t *testing.T) {
	f := newOwnRetentionFixture(t)
	settings := withOwnRetention(t, f.st, 3, map[string]store.RetentionKeep{zfsDomain: {KeepWeekly: 8}})
	f.svc.applyRetentionTags(context.Background(), f.root+"/nas", settings, restic.Mode{}, []string{"zfs:tank/media"}, zfsDomain)
	if len(f.eng.forgets) != 1 || f.eng.forgets[0].Policy != (restic.RetentionPolicy{KeepWeekly: 8}) {
		t.Fatalf("forgets = %+v, want one with the ZFS domain's own keep-weekly 8", f.eng.forgets)
	}
}

func TestAManualPruneAgesByTheDomainsOwnPolicy(t *testing.T) {
	f := newOwnRetentionFixture(t)
	withOwnRetention(t, f.st, 3, map[string]store.RetentionKeep{"containers": {KeepLast: 10, KeepYearly: 1}})
	f.hold(f.root+"/backups/containers", ownRetentionSnap("a1", 100, "container:plex"))
	f.hold(f.root+"/nas", ownRetentionSnap("b1", 100, "container:nginx"))
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

// sharedWithAVM points the VM win11 at nas, the named repository nginx writes
// to, and leaves one snapshot of each item there and one of plex in the
// containers repository.
func (f *ownRetentionFixture) sharedWithAVM() {
	f.t.Helper()
	named, err := f.st.ListNamedRepos()
	if err != nil || len(named) != 1 {
		f.t.Fatalf("ListNamedRepos = %v, %v", named, err)
	}
	if _, err := f.st.UpsertVMTarget(store.VMTarget{Name: "win11"}); err != nil {
		f.t.Fatal(err)
	}
	if err := f.st.SetVMRepo("win11", named[0].ID); err != nil {
		f.t.Fatal(err)
	}
	f.hold(f.root+"/backups/containers", ownRetentionSnap("a1", 100, "container:plex"))
	f.hold(f.root+"/nas", ownRetentionSnap("b1", 100, "container:nginx"), ownRetentionSnap("c1", 100, "vm:win11"))
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
	f := newOwnRetentionFixture(t)
	f.sharedWithAVM()
	withOwnRetention(t, f.st, 3, map[string]store.RetentionKeep{"containers": {KeepDaily: 7}})
	if _, err := f.svc.pruneDomain(context.Background(), "containers", "local", false); err != nil {
		t.Fatalf("pruneDomain: %v", err)
	}
	want := []ownRetentionForget{
		{Repo: f.root + "/backups/containers", Tags: []string{"container:plex"}, Policy: restic.RetentionPolicy{KeepDaily: 7}},
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
	f := newOwnRetentionFixture(t)
	f.sharedWithAVM()
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
	f := newOwnRetentionFixture(t)

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
	f := newOwnRetentionFixture(t)
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
