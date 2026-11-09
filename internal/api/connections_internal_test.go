package api

import (
	"net/http"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestConnectionsSurviveASettingsRoundTrip(t *testing.T) {
	src := newPlacementFixture(t)
	box, err := src.st.SaveDestination(store.OffsiteTarget{Name: "Box", Repo: "rclone:box:BombVault", Provider: "box", Connections: 12})
	if err != nil {
		t.Fatal(err)
	}
	src.tickDestination(box, "containers")
	nas, err := src.st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "vms", Name: "NAS", Repo: "sftp:me@nas:/bv/vms", Enabled: true, Connections: 3})
	if err != nil {
		t.Fatal(err)
	}
	exp := src.do(http.MethodGet, "/api/settings/export", nil)

	dst := newPlacementFixture(t)
	if res := dst.do(http.MethodPost, "/api/settings/import?apply=true", exp); res["ok"] != true {
		t.Fatalf("import = %v", res)
	}
	got, _, err := dst.st.GetDestination(box.ID)
	if err != nil || got.Connections != 12 {
		t.Fatalf("destination = %+v, %v, want 12 connections", got, err)
	}
	derived, err := dst.st.DestinationTargets(box.ID)
	if err != nil || len(derived) != 1 || derived[0].Connections != 12 {
		t.Fatalf("derived targets = %+v, %v, want the destination's 12 connections", derived, err)
	}
	own, _, err := dst.st.GetOffsiteTarget(nas.ID)
	if err != nil || own.Connections != 3 {
		t.Fatalf("own target = %+v, %v, want 3 connections", own, err)
	}
}

func TestAnImportRefusesConnectionsOutOfRange(t *testing.T) {
	f := newPlacementFixture(t)
	f.destination("Box", "rclone:box:BombVault")
	res := importEdited(t, f, func(exp map[string]any) {
		exp["destinations"].([]any)[0].(map[string]any)["connections"] = 500
	})
	if res["ok"] == true || res["code"] != "import-row-connections" {
		t.Fatalf("import = %v, want the connections refusal", res)
	}
}

func TestADestinationTakesConnectionsWithinRangeOnly(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.destination("Box", "rclone:box:BombVault")
	tgt := f.tickDestination(box, "containers")

	if res := f.do(http.MethodPut, "/api/offsite/destinations/"+box.ID, map[string]any{"name": "Box", "connections": 33}); res["ok"] == true {
		t.Fatalf("33 connections were taken: %v", res)
	}
	if res := f.do(http.MethodPut, "/api/offsite/destinations/"+box.ID, map[string]any{"name": "Box", "connections": 10}); res["ok"] != true {
		t.Fatalf("save = %v", res)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	derived, _, err := f.st.GetOffsiteTarget(tgt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.svc.offsiteModeForTarget(settings, derived).Connections; got != 10 {
		t.Fatalf("a copy to the location opens %d connections, want 10", got)
	}
}

func TestAStorageLocationOffersConnectionsWhereItsBackendHasThem(t *testing.T) {
	f := newPlacementFixture(t)
	f.destination("Box", "rclone:box:BombVault")
	if _, err := f.st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "vms", Name: "Disk", Repo: "/mnt/disks/usb/bv", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	locations, err := f.svc.StorageLocations()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, loc := range locations {
		switch loc.Name {
		case "Box":
			seen[loc.Name] = loc.Connections != nil
		case "Disk":
			seen[loc.Name] = loc.Connections == nil
		}
	}
	if !seen["Box"] || !seen["Disk"] {
		t.Fatalf("connections offered right = %v, want the rclone location with them and the local folder without", seen)
	}
}
