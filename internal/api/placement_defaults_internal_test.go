package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestDefaultRowsCountTheItemsOfEachDomain(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	done := f.container("done", "")
	f.backupRun(done.ID, 100)
	f.container("waiting", nas.ID)
	f.openContainer("fresh")
	f.openContainer("local-only")
	f.rule("containers", "container:local-only", store.SkipAll)

	res := f.do(http.MethodGet, "/api/placement/defaults", nil)
	rows, _ := res["defaults"].([]any)
	if len(rows) != 3 {
		t.Fatalf("defaults = %v, want one row per domain", res)
	}
	containers := rows[0].(map[string]any)
	if containers["domain"] != "containers" || containers["exists"] != false || containers["homeKind"] != "domain" {
		t.Fatalf("row = %v", containers)
	}
	counts := containers["counts"].(map[string]any)
	for key, want := range map[string]float64{"follow": 3, "own": 1, "open": 2, "chosenNoBackup": 1} {
		if counts[key] != want {
			t.Errorf("%s = %v, want %v", key, counts[key], want)
		}
	}
	if skip, ok := containers["skip"].([]any); !ok || len(skip) != 0 {
		t.Errorf("skip = %v, want an empty list", containers["skip"])
	}
}

// fourteenContainers is fourteen containers on the domain path, one project
// folder in it and one target that holds some of their copies.
func fourteenContainers(t *testing.T) (*placementFixture, store.OffsiteTarget) {
	t.Helper()
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket/containers")
	f.setDefault("containers", "")
	var held []restic.Snapshot
	for i := range 14 {
		name := fmt.Sprintf("app%02d", i)
		f.container(name, "")
		held = append(held, snap(fmt.Sprintf("aaaa%04d", i), 100, "container:"+name))
	}
	held = append(held, snap("bbbb0001", 100, "stack:immich"))
	f.hold(f.domainPath("containers"), held...)
	f.listing("containers", b2.ID, 200, copiesRow("container:app00", 3, 90), copiesRow("stack:immich", 2, 90))
	return f, b2
}

func TestAHomeOnlyDefaultChangeLeavesTheTargetsAlone(t *testing.T) {
	f, b2 := fourteenContainers(t)
	box := f.namedRepo("Storagebox", "sftp:u1@box.example:/bv")
	res := f.do(http.MethodPost, "/api/placement/default/containers/preview", map[string]any{"home": box.ID})
	impact := res["impact"].(map[string]any)
	if len(impact["dropped"].([]any)) != 0 || len(impact["added"].([]any)) != 0 {
		t.Fatalf("impact = %v, want no target to lose or gain anything", impact)
	}
	if n := f.eng.lists[b2.Repo]; n != 0 {
		t.Errorf("the preview listed the target %d times", n)
	}
}

func TestAnOffsiteOnlyHomeLeavesTheDefaultSkipAlone(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.namedRepo("Storagebox", "sftp:u1@box.example:/bv")
	f.target("containers", "B2", "b2:bucket/containers")
	f.setDefault("containers", "")
	f.container("web", "")
	body := map[string]any{"home": box.ID, "skip": []string{store.SkipAll}}
	impact := f.do(http.MethodPost, "/api/placement/default/containers/preview", body)["impact"].(map[string]any)
	if len(impact["dropped"].([]any)) != 0 {
		t.Fatalf("impact = %v, want no target to lose web", impact)
	}
	body["expect"] = impact
	if res := f.do(http.MethodPut, "/api/placement/default/containers", body); res["ok"] != true {
		t.Fatalf("PUT = %v", res)
	}
	d, found, err := f.st.PlacementDefaultFor("containers")
	if err != nil || !found || d.Home != box.ID || len(d.Skip) != 0 {
		t.Fatalf("default = %+v, %v, %v, want Storagebox with skip still []", d, found, err)
	}
}

func TestLocalForTheDefaultAsksForItemsAndProjectFolders(t *testing.T) {
	f, b2 := fourteenContainers(t)
	res := f.do(http.MethodPost, "/api/placement/default/containers/preview", map[string]any{"skip": []string{store.SkipAll}})
	dropped := res["impact"].(map[string]any)["dropped"].([]any)
	if len(dropped) != 1 {
		t.Fatalf("dropped = %v, want B2", dropped)
	}
	d := dropped[0].(map[string]any)
	if d["targetId"] != b2.ID || d["items"] != float64(15) || d["snapshots"] != float64(5) || d["unknown"] != false {
		t.Fatalf("dropped = %v, want B2 with 15 items and 5 copies that stay", d)
	}
}

func TestADefaultSkipChangeIgnoresAnItemHomedOnARemoteNamedRepositoryByDefault(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.namedRepo("Storagebox", "sftp:u1@box.example:/bv")
	f.target("containers", "B2", "b2:bucket/containers")
	f.setDefault("containers", box.ID)
	f.openContainer("web")

	res := f.do(http.MethodPost, "/api/placement/default/containers/preview", map[string]any{"skip": []string{store.SkipAll}})
	dropped := res["impact"].(map[string]any)["dropped"].([]any)
	if len(dropped) != 0 {
		t.Fatalf("dropped = %v, want none: web is homed on Storagebox by default, not on the domain path", dropped)
	}
}

func TestPutWithStaleNumbersIsRefusedWithTheNewOnes(t *testing.T) {
	f, _ := fourteenContainers(t)
	empty := map[string]any{"dropped": []any{}, "added": []any{}, "openTakeHome": 0}
	res := f.do(http.MethodPut, "/api/placement/default/containers", map[string]any{"skip": []string{store.SkipAll}, "expect": empty})
	if res["ok"] != false || res["code"] != "stale" {
		t.Fatalf("PUT = %v, want code stale", res)
	}
	res = f.do(http.MethodPut, "/api/placement/default/containers", map[string]any{"skip": []string{store.SkipAll}, "expect": res["impact"]})
	if res["ok"] != true {
		t.Fatalf("PUT with the new numbers = %v", res)
	}
	d, found, err := f.st.PlacementDefaultFor("containers")
	if err != nil || !found || !skipsEverything(d.Skip) {
		t.Fatalf("default = %+v, %v, %v, want skip [*]", d, found, err)
	}
}

func TestOpenItemsWithoutHistoryTakeTheNewHome(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.openContainer("fresh")
	f.openContainer("returning")
	f.hold(f.domainPath("containers"), snap("aaaa0001", 100, "container:returning"))
	res := f.do(http.MethodPost, "/api/placement/default/containers/preview", map[string]any{"home": nas.ID})
	if got := res["impact"].(map[string]any)["openTakeHome"]; got != float64(1) {
		t.Fatalf("openTakeHome = %v, want 1", got)
	}
}

func TestOnARemoteDomainPathEveryOpenItemMayTakeTheNewHome(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersPath = "s3:s3.example.com/bucket/containers"
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	f.openContainer("fresh")
	f.openContainer("returning")
	res := f.do(http.MethodPost, "/api/placement/default/containers/preview", map[string]any{"home": nas.ID})
	if got := res["impact"].(map[string]any)["openTakeHome"]; got != float64(2) {
		t.Fatalf("openTakeHome = %v, want 2", got)
	}
	if n := f.eng.lists[settings.ContainersPath]; n != 0 {
		t.Errorf("the remote domain path was listed %d times", n)
	}
}

func TestADefaultOnASwitchedOffRepositoryIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	nas.Enabled = false
	if _, err := f.st.UpsertOffsiteTarget(nas); err != nil {
		t.Fatal(err)
	}
	res := f.do(http.MethodPut, "/api/placement/default/containers", map[string]any{"home": nas.ID})
	if res["code"] != "repo-invalid" {
		t.Fatalf("PUT = %v, want repo-invalid", res)
	}
}

func TestADefaultSkipNamesOnlyTargetsOfItsDomain(t *testing.T) {
	f := newPlacementFixture(t)
	vms := f.target("vms", "B2 VMs", "b2:bucket/vms")
	res := f.do(http.MethodPut, "/api/placement/default/containers", map[string]any{"skip": []string{vms.ID}})
	if res["code"] != "unknown-target" {
		t.Fatalf("PUT = %v, want unknown-target", res)
	}
}

func TestADefaultRouteForAnotherDomainIsABadRequest(t *testing.T) {
	f := newPlacementFixture(t)
	rec := httptest.NewRecorder()
	f.h.Router().ServeHTTP(rec, jsonReq(http.MethodPut, "/api/placement/default/flash", strings.NewReader(`{}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
