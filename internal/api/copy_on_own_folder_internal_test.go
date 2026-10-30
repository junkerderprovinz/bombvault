package api

import (
	"reflect"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestACopyOnItsOwnFolderIsRemovedAtStart(t *testing.T) {
	f := newPlacementFixture(t)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	own := settings.ContainersPath
	self := f.target("containers", "Local", own+"/")
	box := f.target("containers", "Box", "s3:box/containers")
	vms := f.target("vms", "Elsewhere", own)
	f.setDefault("containers", "", self.ID, box.ID)
	f.rule("containers", "container:plex", self.ID)

	if _, err := f.svc.RepairLocationModel(); err != nil {
		t.Fatal(err)
	}

	left, err := f.st.OffsiteTargetsForDomain("containers")
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 1 || left[0].ID != box.ID {
		t.Fatalf("containers targets = %+v, want only Box", left)
	}
	if others, _ := f.st.OffsiteTargetsForDomain("vms"); len(others) != 1 || others[0].ID != vms.ID {
		t.Errorf("vms targets = %+v, want the copy that sits on another section's folder kept", others)
	}
	if d, _, _ := f.st.PlacementDefaultFor("containers"); !reflect.DeepEqual(d.Skip, []string{box.ID}) {
		t.Errorf("default skip = %v, want only %s", d.Skip, box.ID)
	}
	if r, found, _ := f.st.CopyRuleFor("containers", "container:plex"); !found || len(r.Skip) != 0 {
		t.Errorf("rule = %+v, %v, want an empty skip", r, found)
	}
}

func TestACopyOnItsOwnFolderTakesTheOffsiteFieldAlongWhenItWasThePrimary(t *testing.T) {
	f := newPlacementFixture(t)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	own := settings.ContainersPath
	settings.ContainersOffsite = own
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Local", Repo: own, Enabled: true}); err != nil {
		t.Fatal(err)
	}

	if _, err := f.svc.RepairLocationModel(); err != nil {
		t.Fatal(err)
	}

	if left, _ := f.st.OffsiteTargetsForDomain("containers"); len(left) != 0 {
		t.Errorf("containers targets = %+v, want none", left)
	}
	if s, _ := f.st.GetSettings(); s.ContainersOffsite != "" {
		t.Errorf("containers off-site field = %q, want it cleared", s.ContainersOffsite)
	}
}

func TestASettingsSaveRefusesAnOffsiteFieldOnTheOwnFolder(t *testing.T) {
	f := newPlacementFixture(t)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	v := toView(settings)
	v.FilesOffsite = settings.FilesPath

	res := f.do("PUT", "/api/settings", v)

	if msg, _ := res["error"].(string); res["ok"] != false || !strings.Contains(msg, errCopyOnOwnFolder.Error()) {
		t.Fatalf("answer = %v, want the copy onto the own folder refused", res)
	}
	if left, _ := f.st.OffsiteTargetsForDomain("files"); len(left) != 0 {
		t.Fatalf("files targets = %+v, want none", left)
	}
}

func TestASettingsSaveRefusesAnOwnFolderWhereACopyOfTheSectionLies(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("files", "Moved", "moved/files")
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	v := toView(settings)
	v.FilesPath = "moved/files"

	res := f.do("PUT", "/api/settings", v)

	if msg, _ := res["error"].(string); res["ok"] != false || !strings.Contains(msg, errOwnFolderOnCopy.Error()) {
		t.Fatalf("answer = %v, want the own folder on the copy refused", res)
	}
	if s, _ := f.st.GetSettings(); s.FilesPath != settings.FilesPath {
		t.Fatalf("files path = %q, want it kept", s.FilesPath)
	}
}

func TestMovingASectionFolderOntoACopyOfTheSectionIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("files", "Moved", "moved/files")
	locations, err := f.svc.StorageLocations()
	if err != nil {
		t.Fatal(err)
	}
	id := locations[0].ID
	removal := f.do("GET", "/api/storage/locations/"+id+"/removal", nil)

	res := f.do("POST", "/api/storage/locations/"+id+"/removal", map[string]any{"expect": removal["removal"], "home": "moved"})

	if msg, _ := res["error"].(string); res["ok"] != false || !strings.Contains(msg, errOwnFolderOnCopy.Error()) {
		t.Fatalf("answer = %v, want the move onto the copy refused", res)
	}
	if s, _ := f.st.GetSettings(); s.FilesPath != "backups/files" {
		t.Fatalf("files path = %q, want it kept", s.FilesPath)
	}
}
