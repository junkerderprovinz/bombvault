package api

import (
	"net/http"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func placementsByKey(t *testing.T, rows []any, key string) map[string]map[string]any {
	t.Helper()
	out := map[string]map[string]any{}
	for _, row := range rows {
		r := row.(map[string]any)
		placement, ok := r["placement"].(map[string]any)
		if !ok {
			t.Fatalf("row %v carries no placement", r[key])
		}
		out[r[key].(string)] = placement
	}
	return out
}

func TestTheContainerListCarriesThePlacementOfEveryRow(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	f.container("nginx", "")
	f.rule("containers", "container:nginx", store.SkipAll)
	f.dock.installed = map[string]bool{"fresh": true}

	got := placementsByKey(t, f.do(http.MethodGet, "/api/containers", nil)["containers"].([]any), "name")
	if got["nginx"]["segment"] != "local" || got["nginx"]["homeFollows"] != false {
		t.Errorf("nginx = %v, want local and chosen", got["nginx"])
	}
	if got["fresh"]["segment"] != "local-offsite" || got["fresh"]["homeFollows"] != true {
		t.Errorf("fresh = %v, want an open item on local + off-site", got["fresh"])
	}
}

func TestTheVMAndFileSetListsCarryThePlacement(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("vms", "B2", "b2:bucket:vms")
	f.vm("win11", "")
	set := f.fileSet("Photos", "")
	f.backupRun(set.ID, 1_758_000_000)

	vms := placementsByKey(t, f.do(http.MethodGet, "/api/vms", nil)["vms"].([]any), "libvirtName")
	if vms["win11"]["segment"] != "local-offsite" {
		t.Errorf("win11 = %v", vms["win11"])
	}
	sets := placementsByKey(t, f.do(http.MethodGet, "/api/files", nil)["fileSets"].([]any), "id")
	if p := sets[set.ID]; p["locked"] != true || p["lockReason"] != "first-backup" || p["segment"] != "local" {
		t.Errorf("Photos = %v, want locked on local (no files target)", p)
	}
}

func TestAnItemPatchAnswersWithTheNewPlacement(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("vms", "B2", "b2:bucket:vms")
	f.vm("win11", "")
	res := f.do(http.MethodPatch, "/api/vms/win11", map[string]any{"copies": map[string]any{"skip": []string{store.SkipAll}}})
	placement, ok := res["placement"].(map[string]any)
	if !ok || placement["segment"] != "local" || placement["copiesFollow"] != false {
		t.Fatalf("PATCH = %v, want the new placement on local", res)
	}

	set := f.fileSet("Scans", "")
	res = f.do(http.MethodPatch, "/api/files/sets/"+set.ID, map[string]any{"home": map[string]any{"follow": true}})
	if p := res["placement"].(map[string]any); p["homeFollows"] != true {
		t.Fatalf("PATCH = %v, want the set open again", res)
	}
}
