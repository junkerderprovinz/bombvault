package api

import (
	"net/http"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func (f *placementFixture) acceptDeleteRisk(locationID string, accepted bool) map[string]any {
	f.t.Helper()
	return f.do(http.MethodPut, "/api/storage/locations/"+locationID+"/risk-accepted", map[string]bool{"accepted": accepted})
}

func TestASectionWithoutDeleteProtectionIsAcceptedOnlyOnceItsLocationIs(t *testing.T) {
	f := newPlacementFixture(t)
	drive := f.target("containers", "OneDrive", "rclone:onedrive:bv/containers")
	if f.domainStatus("containers").DeleteRiskAccepted {
		t.Fatal("a location nobody accepted reads as accepted")
	}

	if res := f.acceptDeleteRisk("target:"+drive.ID, true); res["ok"] != true {
		t.Fatalf("accept = %v", res)
	}
	d := f.domainStatus("containers")
	if !d.DeleteRiskAccepted {
		t.Fatal("the section copies only to the accepted location, so its gap is accepted")
	}
	if d.OffsiteImmutable || d.TamperState != "" {
		t.Errorf("accepting claimed protection: immutable %v, tamper %q", d.OffsiteImmutable, d.TamperState)
	}
	if !locationByID(t, f.svc, "target:"+drive.ID).Protection.RiskAccepted {
		t.Error("the location page does not show the acceptance")
	}

	f.acceptDeleteRisk("target:"+drive.ID, false)
	if f.domainStatus("containers").DeleteRiskAccepted {
		t.Error("taking the acceptance back left the section accepted")
	}
}

func TestADestinationsAcceptanceCoversTheSectionsThatCopyThere(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.destination("OneDrive", "rclone:onedrive:BombVault")
	f.tickDestination(box, "containers")
	f.tickDestination(box, "flash")

	f.acceptDeleteRisk("destination:"+box.ID, true)
	for _, domain := range []string{"containers", "flash"} {
		if !f.domainStatus(domain).DeleteRiskAccepted {
			t.Errorf("%s copies to the accepted destination but is not accepted", domain)
		}
	}
}

func TestOneUnacceptedCopyKeepsTheSectionUnaccepted(t *testing.T) {
	f := newPlacementFixture(t)
	drive := f.target("containers", "OneDrive", "rclone:onedrive:bv/containers")
	f.target("containers", "Dropbox", "rclone:dropbox:bv/containers")
	f.acceptDeleteRisk("target:"+drive.ID, true)
	if f.domainStatus("containers").DeleteRiskAccepted {
		t.Fatal("a second copy nobody accepted still lacks delete protection")
	}
}

func TestACopyWithDeleteProtectionNeedsNoAcceptance(t *testing.T) {
	f := newPlacementFixture(t)
	drive := f.target("containers", "OneDrive", "rclone:onedrive:bv/containers")
	guarded, err := f.st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Vault", Repo: "rest:http://vault:8000/containers", Enabled: true, Immutable: true})
	if err != nil {
		t.Fatal(err)
	}
	f.acceptDeleteRisk("target:"+drive.ID, true)
	if !f.domainStatus("containers").DeleteRiskAccepted {
		t.Error("the only copy without delete protection is accepted")
	}

	code, res := f.call(http.MethodPut, "/api/storage/locations/target:"+guarded.ID+"/risk-accepted", map[string]bool{"accepted": true})
	if code != http.StatusOK || res["ok"] != false {
		t.Errorf("accepting a location with delete protection = %d %v, want a refusal", code, res)
	}
}

func TestOnlyALocationThatTakesCopiesCanBeAccepted(t *testing.T) {
	f := newPlacementFixture(t)
	var path string
	for _, l := range locationsOf(t, f.svc, locationPath) {
		path = l.ID
	}
	if path == "" {
		t.Fatal("the fixture has no folder of domain paths")
	}
	if _, res := f.call(http.MethodPut, "/api/storage/locations/"+path+"/risk-accepted", map[string]bool{"accepted": true}); res["ok"] != false {
		t.Errorf("accepting the folder the backups are written to = %v, want a refusal", res)
	}
}

func TestMovingATargetForgetsTheRiskAcceptedForItsOldPlace(t *testing.T) {
	f := newPlacementFixture(t)
	drive := f.target("containers", "OneDrive", "rclone:onedrive:bv/containers")
	if err := f.st.SetDeleteRiskAccepted(drive.ID, true); err != nil {
		t.Fatal(err)
	}
	drive.Repo = "rest:http://vault:8000/containers"
	moved, err := f.st.MoveOffsiteTarget(drive)
	if err != nil {
		t.Fatal(err)
	}
	if moved.DeleteRiskAccepted {
		t.Error("the new place inherited the acceptance of the old one")
	}
}

func TestASaveOfTheTargetKeepsItsAcceptance(t *testing.T) {
	f := newPlacementFixture(t)
	drive := f.target("containers", "OneDrive", "rclone:onedrive:bv/containers")
	f.acceptDeleteRisk("target:"+drive.ID, true)
	drive.Name = "OneDrive family"
	saved, err := f.st.UpsertOffsiteTarget(drive)
	if err != nil {
		t.Fatal(err)
	}
	if !saved.DeleteRiskAccepted {
		t.Error("renaming the target took the acceptance back")
	}
}

func TestTheStatusSaysWhenTheCheckOfDeleteProtectionIsOff(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "OneDrive", "rclone:onedrive:bv/containers")
	f.settings(func(s *store.Settings) { s.TamperTestSchedule = "daily 04:00" })
	if f.domainStatus("containers").TamperOff {
		t.Error("a scheduled check reads as off")
	}
	f.settings(func(s *store.Settings) { s.TamperTestSchedule = "off" })
	if !f.domainStatus("containers").TamperOff {
		t.Error("a check switched off reads as on")
	}
}

func TestTheAcceptanceSurvivesARoundTrip(t *testing.T) {
	src := newPlacementFixture(t)
	box := src.destination("OneDrive", "rclone:onedrive:BombVault")
	src.tickDestination(box, "containers")
	drive := src.target("flash", "Dropbox", "rclone:dropbox:bv/flash")
	src.acceptDeleteRisk("destination:"+box.ID, true)
	src.acceptDeleteRisk("target:"+drive.ID, true)
	exp := src.do(http.MethodGet, "/api/settings/export", nil)

	dst := newPlacementFixture(t)
	if res := dst.do(http.MethodPost, "/api/settings/import?apply=true", exp); res["ok"] != true {
		t.Fatalf("import = %v", res)
	}
	if !locationByID(t, dst.svc, "destination:"+box.ID).Protection.RiskAccepted {
		t.Error("the destination lost its acceptance on the way")
	}
	if !locationByID(t, dst.svc, "target:"+drive.ID).Protection.RiskAccepted {
		t.Error("the target lost its acceptance on the way")
	}

	src.acceptDeleteRisk("target:"+drive.ID, false)
	if res := dst.do(http.MethodPost, "/api/settings/import?apply=true", src.do(http.MethodGet, "/api/settings/export", nil)); res["ok"] != true {
		t.Fatalf("second import = %v", res)
	}
	if locationByID(t, dst.svc, "target:"+drive.ID).Protection.RiskAccepted {
		t.Error("a file that takes the acceptance back left it in place")
	}
}
