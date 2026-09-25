package api

import (
	"maps"
	"net/http"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

const b2Base = "s3:https://s3.eu-central-003.backblazeb2.com/bv-offsite"

// seededPlaces is what seedPlaces leaves behind.
type seededPlaces struct {
	unraid, b2          store.Place
	field, repo, copies store.OffsiteTarget
}

// seedPlaces gives f a local place that is home to containers, VMs and
// folder sets, and a B2 place holding the containers field target, a named
// repository in its VMs folder and a VM copy target beside that one.
func seedPlaces(f *placementFixture) seededPlaces {
	f.t.Helper()
	var s seededPlaces
	s.unraid = f.storePlace(store.Place{
		ID: "place-unraid", Name: "Unraid", Provider: "unraid-folder", Kind: "local", Base: "backups",
		Folders: map[string]string{"containers": "containers", "vms": "vms", "files": "files"}, Enabled: true,
	}, "containers", "vms", "files")
	s.b2 = f.storePlace(store.Place{
		ID: "place-b2", Name: "B2", Provider: "b2", Kind: "s3", Base: b2Base,
		Folders:     map[string]string{"containers": "container", "vms": "vms"},
		OffPremises: true, RetentionKeepDaily: 30, Enabled: true,
	})
	s.field = f.fieldTarget("containers", b2Base+"/container")
	f.linkRow(s.field.ID, s.b2, "containers", "")
	s.repo = f.namedRepo("B2 VMs", b2Base+"/vms")
	f.linkRow(s.repo.ID, s.b2, "vms", "")
	s.copies = f.target("vms", "B2 copies", b2Base+"/vms-copies")
	f.linkRow(s.copies.ID, s.b2, "vms", "-copies")
	return s
}

// exportedRow finds a row of an exported block by id.
func exportedRow(t *testing.T, exp map[string]any, block, id string) map[string]any {
	t.Helper()
	rows, _ := exp[block].([]any)
	for _, row := range rows {
		if m := row.(map[string]any); m["id"] == id {
			return m
		}
	}
	t.Fatalf("%s has no row %s: %v", block, id, exp[block])
	return nil
}

func TestTheExportCarriesPlacesAndWhereEachRowSits(t *testing.T) {
	f := newPlacementFixture(t)
	seed := seedPlaces(f)
	exp := f.do(http.MethodGet, "/api/settings/export", nil)

	if exp["schemaVersion"] != float64(1) {
		t.Errorf("schemaVersion = %v, want 1: the new blocks are additions an older build skips", exp["schemaVersion"])
	}
	list, _ := exp["places"].([]any)
	if len(list) != 2 {
		t.Fatalf("places = %v, want the two seeded", exp["places"])
	}
	var b2 map[string]any
	for _, p := range list {
		if m := p.(map[string]any); m["id"] == seed.b2.ID {
			b2 = m
		}
	}
	wantKeys := []string{
		"base", "createdAt", "credsRef", "enabled", "folders", "growthBudgetGb", "id", "immutable", "kind",
		"limitDownload", "limitUpload", "name", "offPremises", "provider", "retentionKeepDaily", "retentionKeepLast",
		"retentionKeepMonthly", "retentionKeepWeekly", "sortOrder", "storageClass", "updatedAt",
	}
	if got := slices.Sorted(maps.Keys(b2)); !slices.Equal(got, wantKeys) {
		t.Errorf("place keys = %v, want %v", got, wantKeys)
	}
	if b2["base"] != b2Base || b2["retentionKeepDaily"] != float64(30) || b2["offPremises"] != true {
		t.Errorf("B2 place = %v", b2)
	}
	if folders, _ := b2["folders"].(map[string]any); len(folders) != 2 || folders["containers"] != "container" || folders["vms"] != "vms" {
		t.Errorf("B2 folders = %v", b2["folders"])
	}

	homes, _ := exp["storageDomainPlaces"].(map[string]any)
	want := map[string]any{"containers": seed.unraid.ID, "vms": seed.unraid.ID, "files": seed.unraid.ID}
	if !maps.Equal(homes, want) {
		t.Errorf("storageDomainPlaces = %v, want %v", homes, want)
	}

	for _, tc := range []struct{ block, id, domain, suffix string }{
		{"offsiteTargets", seed.field.ID, "containers", ""},
		{"offsiteTargets", seed.copies.ID, "vms", "-copies"},
		{"namedRepos", seed.repo.ID, "vms", ""},
	} {
		row := exportedRow(t, exp, tc.block, tc.id)
		suffix, _ := row["placeSuffix"].(string)
		if row["placeId"] != seed.b2.ID || row["placeDomain"] != tc.domain || suffix != tc.suffix {
			t.Errorf("%s %s = %v, want it on %s, %s%s", tc.block, tc.id, row, seed.b2.ID, tc.domain, tc.suffix)
		}
	}
}

func TestAnInstanceWithoutPlacesStillExportsBothBlocks(t *testing.T) {
	f := newPlacementFixture(t)
	exp := f.do(http.MethodGet, "/api/settings/export", nil)
	if list, ok := exp["places"].([]any); !ok || len(list) != 0 {
		t.Errorf("places = %v, want an empty list", exp["places"])
	}
	if homes, ok := exp["storageDomainPlaces"].(map[string]any); !ok || len(homes) != 0 {
		t.Errorf("storageDomainPlaces = %v, want an empty object", exp["storageDomainPlaces"])
	}
}
