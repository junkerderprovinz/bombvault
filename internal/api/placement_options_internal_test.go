package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func (f *placementFixture) newTargetPreview(query string) map[string]any {
	f.t.Helper()
	res := f.do(http.MethodGet, "/api/placement/new-target-preview?"+query, nil)
	pv, ok := res["preview"].(map[string]any)
	if !ok {
		f.t.Fatalf("preview = %v", res)
	}
	return pv
}

func TestANewTargetReceivesEveryItemNotSetToLocalAndTheProjectFolders(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	f.container("plex", "")
	f.rule("containers", "container:plex", store.SkipAll)
	f.hold(f.domainPath("containers"),
		snap("aaaa0001", 100, "container:nginx"), snap("aaaa0002", 200, "container:nginx"),
		snap("aaaa0003", 100, "container:plex"), snap("aaaa0004", 100, "stack:immich"))
	if err := f.st.AddRepoStat(store.RepoStat{Domain: "containers", Source: "local", At: 300, RawSize: 81_604_378_624, RestoreSize: 90_000_000_000, Snapshots: 4}); err != nil {
		t.Fatal(err)
	}

	pv := f.newTargetPreview("domain=containers&repo=b2:bucket:containers")
	if pv["items"] != float64(2) || pv["snapshots"] != float64(3) || pv["bytes"] != float64(81_604_378_624) {
		t.Fatalf("preview = %v, want nginx and immich, 3 snapshots, the measured size", pv)
	}
}

func TestANewTargetCountsItemsOnARemoteDomainPath(t *testing.T) {
	f := newPlacementFixture(t)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.VMsPath = "s3:https://s3.example.com/vms"
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	f.vm("win11", "")
	if pv := f.newTargetPreview("domain=vms&repo=b2:bucket:vms"); pv["items"] != float64(1) {
		t.Fatalf("preview = %v, want the VM on the remote domain path", pv)
	}
}

func TestAMovedTargetLeavesOutWhatAlreadySkipsIt(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	hz := f.target("containers", "Hetzner", "sftp:u1@hz.example:/bv")
	f.container("nginx", "")
	f.container("plex", "")
	f.rule("containers", "container:plex", b2.ID)
	f.container("sonarr", "")
	f.rule("containers", "container:sonarr", hz.ID)
	f.setDefault("containers", "", hz.ID)

	pv := f.newTargetPreview("domain=containers&repo=b2:other:containers&target=" + b2.ID)
	if pv["items"] != float64(2) {
		t.Errorf("items = %v, want nginx and sonarr", pv["items"])
	}
	formerly := rowsOf(pv["formerlyExcluded"])
	if len(formerly) != 1 || formerly[0]["identity"] != "container:sonarr" {
		t.Errorf("formerlyExcluded = %v, want sonarr only", formerly)
	}
	if pv["defaultExcludes"] != true {
		t.Errorf("defaultExcludes = %v, want true", pv["defaultExcludes"])
	}
}

func TestTheSizeIsLeftOutWhenANamedRepositoryHoldsCountedItems(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS Keller", "nas")
	f.container("nginx", nas.ID)
	if err := f.st.AddRepoStat(store.RepoStat{Domain: "containers", Source: "local", At: 300, RawSize: 1_000, RestoreSize: 2_000, Snapshots: 1}); err != nil {
		t.Fatal(err)
	}
	pv := f.newTargetPreview("domain=containers&repo=b2:bucket:containers")
	if pv["items"] != float64(1) || pv["bytes"] != nil {
		t.Fatalf("preview = %v, want one item and no size", pv)
	}
}

func TestANewTargetPreviewNamesSourcesItCouldNotList(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	f.eng.listErr = map[string]error{f.domainPath("containers"): errors.New("wrong password or no key found")}
	pv := f.newTargetPreview("domain=containers&repo=b2:bucket:containers")
	if unreadable := pv["unreadable"].([]any); len(unreadable) != 1 || pv["items"] != float64(1) {
		t.Fatalf("preview = %v, want nginx counted and the domain path named as unreadable", pv)
	}
}

func TestANewTargetPreviewRefusesATargetOfAnotherDomain(t *testing.T) {
	f := newPlacementFixture(t)
	vms := f.target("vms", "B2", "b2:bucket:vms")
	res := f.do(http.MethodGet, "/api/placement/new-target-preview?domain=containers&repo=b2:x&target="+vms.ID, nil)
	if res["code"] != "unknown-target" {
		t.Fatalf("preview = %v, want unknown-target", res)
	}
}

func TestANewTargetPreviewWithoutALocationIsABadRequest(t *testing.T) {
	f := newPlacementFixture(t)
	rec := httptest.NewRecorder()
	f.h.Router().ServeHTTP(rec, jsonReq(http.MethodGet, "/api/placement/new-target-preview?domain=containers", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// rowsOf turns a JSON array of objects into typed rows, nil for anything else.
func rowsOf(v any) []map[string]any {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	rows := make([]map[string]any, 0, len(arr))
	for _, e := range arr {
		if m, ok := e.(map[string]any); ok {
			rows = append(rows, m)
		}
	}
	return rows
}

func (f *placementFixture) options(domain string) map[string]any {
	f.t.Helper()
	res := f.do(http.MethodGet, "/api/placement/options?domain="+domain, nil)
	opts, ok := res["options"].(map[string]any)
	if !ok {
		f.t.Fatalf("options = %v", res)
	}
	return opts
}

func TestOptionsOfferTheDomainPathThenMountedRepositories(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS Keller", "nas")
	cold := f.namedRepo("Cold", "cold")
	cold.Enabled = false
	if _, err := f.st.UpsertOffsiteTarget(cold); err != nil {
		t.Fatal(err)
	}
	f.namedRepo("Storagebox", "sftp:u1@box.example:/bv")
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	homes := rowsOf(f.options("vms")["homes"])
	want := []map[string]any{
		{"id": "", "name": "", "location": settings.VMsPath, "kind": "domain", "scheme": ""},
		{"id": nas.ID, "name": "NAS Keller", "location": "nas", "kind": "local", "scheme": ""},
	}
	if !reflect.DeepEqual(homes, want) {
		t.Fatalf("homes = %v, want %v", homes, want)
	}
}

func TestATargetWithOtherCredentialsThanARemoteDomainPathSaysSo(t *testing.T) {
	f := newPlacementFixture(t)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersPath = "s3:https://s3.example.com/containers"
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.UpsertPrimaryRemoteTarget("containers", store.OffsiteTarget{Repo: settings.ContainersPath, CredsRef: "set-a", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	same := f.target("containers", "Same", "b2:bucket:same")
	same.CredsRef = "set-a"
	if _, err := f.st.UpsertOffsiteTarget(same); err != nil {
		t.Fatal(err)
	}
	f.target("containers", "Other", "b2:bucket:other")

	opts := f.options("containers")
	if home := rowsOf(opts["homes"])[0]; home["kind"] != "domain-remote" || home["scheme"] != "s3" {
		t.Errorf("domain path = %v, want domain-remote s3", home)
	}
	hints := map[string]any{}
	for _, tg := range rowsOf(opts["targets"]) {
		hints[tg["name"].(string)] = tg["hint"]
	}
	if !reflect.DeepEqual(hints, map[string]any{"Same": "", "Other": "creds-differ"}) {
		t.Errorf("hints = %v", hints)
	}
}

func TestTargetsComeInSortOrderWithThePrimaryFirst(t *testing.T) {
	f := newPlacementFixture(t)
	extra := f.target("containers", "Hetzner", "sftp:u1@hz.example:/bv")
	field := f.fieldTarget("containers", "b2:bucket:containers")
	targets := rowsOf(f.options("containers")["targets"])
	if len(targets) != 2 || targets[0]["id"] != field.ID || targets[0]["primary"] != true || targets[1]["id"] != extra.ID || targets[1]["primary"] != false {
		t.Fatalf("targets = %v, want the field target first and primary", targets)
	}
}

func TestSendToOffersEachTargetsDirectRepositoryThenRemoteOnes(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	hz := f.target("containers", "Hetzner", "sftp:u1@hz.example:/bv")
	off := f.target("containers", "Wasabi", "s3:https://s3.wasabi.example/bv")
	off.Enabled = false
	if _, err := f.st.UpsertOffsiteTarget(off); err != nil {
		t.Fatal(err)
	}
	direct := f.direct(hz)
	box := f.namedRepo("Storagebox", "sftp:u1@box.example:/bv")

	got := rowsOf(f.options("containers")["sendTo"])
	want := []map[string]any{
		{"kind": "direct", "repoId": "", "targetId": b2.ID, "name": "B2", "location": ""},
		{"kind": "direct", "repoId": direct.ID, "targetId": hz.ID, "name": "Hetzner", "location": direct.Repo},
		{"kind": "remote", "repoId": box.ID, "targetId": "", "name": "Storagebox", "location": "sftp:u1@box.example:/bv"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sendTo = %v, want %v", got, want)
	}
}

func TestOptionsLockWhatTheDomainCannotOffer(t *testing.T) {
	f := newPlacementFixture(t)
	want := map[string]any{"local-offsite": "no-target", "offsite-only": "no-target"}
	if got := f.options("files")["segmentLocks"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("segmentLocks = %v, want %v", got, want)
	}
	f.target("files", "B2", "b2:bucket:files")
	if got := f.options("files")["segmentLocks"]; !reflect.DeepEqual(got, map[string]any{}) {
		t.Fatalf("segmentLocks = %v, want none with a target", got)
	}
}

func TestOptionsCarryThePauseAndTheDefault(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("vms", "B2", "b2:bucket:vms")
	f.paused("vms")
	opts := f.options("vms")
	if opts["paused"] != true || opts["unreadable"] != false {
		t.Errorf("options = %v, want paused and readable", opts)
	}
	if d := opts["default"].(map[string]any); d["domain"] != "vms" || d["paused"] != true {
		t.Errorf("default = %v", d)
	}
}

func TestOptionsOfADomainWithUncertainTargetsAreUnreadable(t *testing.T) {
	f := newPlacementFixture(t)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.VMsOffsite = "b2:bucket:vms"
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	opts := f.options("vms")
	if opts["unreadable"] != true || len(opts["homes"].([]any)) != 0 || len(opts["targets"].([]any)) != 0 {
		t.Fatalf("options = %v, want unreadable with empty lists", opts)
	}
}

func TestOptionsForAnotherDomainAreABadRequest(t *testing.T) {
	f := newPlacementFixture(t)
	rec := httptest.NewRecorder()
	f.h.Router().ServeHTTP(rec, jsonReq(http.MethodGet, "/api/placement/options?domain=flash", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
