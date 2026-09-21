package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
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
