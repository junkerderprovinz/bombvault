package api

import (
	"errors"
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

	if msg, _ := res["error"].(string); res["ok"] != false || !strings.Contains(msg, errOwnFolderOnTypedCopy.Error()) {
		t.Fatalf("answer = %v, want the own folder on the copy refused", res)
	}
	if s, _ := f.st.GetSettings(); s.FilesPath != settings.FilesPath {
		t.Fatalf("files path = %q, want it kept", s.FilesPath)
	}
}

func TestAnOwnFolderGoesWhereATypedInCopyWasOnceThatCopyIsRemoved(t *testing.T) {
	f := newPlacementFixture(t)
	moved := f.target("files", "Moved", "moved/files")
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	v := toView(settings)
	v.FilesPath = "moved/files"
	if res := f.do("PUT", "/api/settings", v); res["code"] != "own-folder-on-typed-copy" {
		t.Fatalf("answer = %v, want the typed-in copy named", res)
	}

	id := "target:" + moved.ID
	removal := f.do("GET", "/api/storage/locations/"+id+"/removal", nil)
	if res := f.do("POST", "/api/storage/locations/"+id+"/removal", map[string]any{"expect": removal["removal"]}); res["ok"] != true {
		t.Fatalf("removal = %v", res)
	}
	if res := f.do("PUT", "/api/settings", v); res["ok"] != true {
		t.Fatalf("answer after the copy went = %v, want the own folder there", res)
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

	if msg, _ := res["error"].(string); res["ok"] != false || !strings.Contains(msg, errOwnFolderOnTypedCopy.Error()) {
		t.Fatalf("answer = %v, want the move onto the copy refused", res)
	}
	if s, _ := f.st.GetSettings(); s.FilesPath != "backups/files" {
		t.Fatalf("files path = %q, want it kept", s.FilesPath)
	}
}

func TestALocationRefusalCarriesTheCodeTheInterfaceTranslates(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("files", "Moved", "moved/files")
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	onCopy := toView(settings)
	onCopy.FilesPath = "moved/files"
	onOwn := toView(settings)
	onOwn.FilesOffsite = settings.FilesPath
	box := f.destination("Box", "rest:https://box:8000/bv")
	f.tickDestination(box, "vms")
	onBox := toView(settings)
	onBox.VMsPath = "rest:https://box:8000/bv/vms"
	locations, err := f.svc.StorageLocations()
	if err != nil {
		t.Fatal(err)
	}
	id := locations[0].ID
	removal := f.do("GET", "/api/storage/locations/"+id+"/removal", nil)

	for name, c := range map[string]struct {
		res  map[string]any
		code string
	}{
		"own folder on a typed copy":  {f.do("PUT", "/api/settings", onCopy), "own-folder-on-typed-copy"},
		"own folder on a box's copy":  {f.do("PUT", "/api/settings", onBox), "own-folder-on-copy"},
		"off-site field on own":       {f.do("PUT", "/api/settings", onOwn), "copy-on-own-folder"},
		"section folder moved":        {f.do("POST", "/api/storage/locations/"+id+"/removal", map[string]any{"expect": removal["removal"], "home": "moved"}), "own-folder-on-typed-copy"},
		"repository in an own folder": {f.do("POST", "/api/repos", map[string]any{"name": "Inside", "repo": settings.FilesPath + "/inside"}), "nested-location"},
	} {
		if c.res["ok"] != false || c.res["code"] != c.code {
			t.Errorf("%s: answer = %v, want code %s", name, c.res, c.code)
		}
	}
}

func TestTheQuestionBeforeANewLocationRefusesWhatTheSaveWould(t *testing.T) {
	f := newPlacementFixture(t)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	for _, as := range []string{"field", "target"} {
		res := f.do("GET", "/api/placement/new-target-preview?domain=files&repo="+settings.FilesPath+"&as="+as, nil)
		if res["ok"] != false || res["saveRefused"] != true || res["code"] != "copy-on-own-folder" {
			t.Errorf("as %s: answer = %v, want the save refused before the question", as, res)
		}
	}
	if left, _ := f.st.OffsiteTargetsForDomain("files"); len(left) != 0 {
		t.Fatalf("files targets = %+v, want none", left)
	}
	if pv := f.newTargetPreview("domain=containers&repo=b2:bucket:containers&as=field"); pv["items"] == nil {
		t.Fatalf("preview = %v, want a free location previewed", pv)
	}
}

// leftoverCopy is the row the files section's off-site field leaves switched
// off at location once the field is cleared, with the sign-in credsRef.
func leftoverCopy(f *placementFixture, location, credsRef string) store.OffsiteTarget {
	f.t.Helper()
	left := f.fieldTarget("files", location)
	if credsRef != "" {
		left.CredsRef = credsRef
		if _, err := f.st.UpsertOffsiteTarget(left); err != nil {
			f.t.Fatal(err)
		}
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		f.t.Fatal(err)
	}
	settings.FilesOffsite = ""
	if err := f.st.UpdateSettings(settings); err != nil {
		f.t.Fatal(err)
	}
	if err := f.svc.syncPrimaryOffsiteTarget("files", settings); err != nil {
		f.t.Fatal(err)
	}
	return left
}

// ownFilesAt saves the files section's own folder at path the way the page does.
func ownFilesAt(f *placementFixture, path string) map[string]any {
	f.t.Helper()
	settings, err := f.st.GetSettings()
	if err != nil {
		f.t.Fatal(err)
	}
	v := toView(settings)
	v.FilesPath = path
	return f.do("PUT", "/api/settings", v)
}

func TestAnOwnFolderMayGoWhereASwitchedOffCopyWasLeftAndKeepsItsBackups(t *testing.T) {
	f := newPlacementFixture(t)
	left := leftoverCopy(f, "moved/files", "")
	f.hold(f.root+"/moved/files", snap("c1", 100, "fileset:iso"))
	locations, err := f.svc.StorageLocations()
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range locations {
		if l.ID == "target:"+left.ID {
			t.Fatalf("the switched-off copy is listed as %+v, want it hidden", l)
		}
	}

	if res := ownFilesAt(f, "moved/files"); res["ok"] != true {
		t.Fatalf("answer = %v, want the own folder where the switched-off copy was left", res)
	}
	if f.hasTarget(left.ID) {
		t.Fatal("the switched-off copy is still there after the own folder took its place")
	}
	if fixed, err := f.svc.RepairLocationModel(); err != nil || len(fixed) != 0 {
		t.Fatalf("a start repaired %v, %v, want nothing left to repair", fixed, err)
	}
	if len(f.eng.forgets) != 0 || len(f.eng.deletes) != 0 {
		t.Fatalf("forgot %+v and deleted %+v, want the backups there kept", f.eng.forgets, f.eng.deletes)
	}
	if got := f.snapsAt(f.root + "/moved/files"); len(got) != 1 {
		t.Fatalf("the own folder holds %d backups, want the copy's one", len(got))
	}
}

func TestAnOwnFolderStaysWhereItIsWhenItsKeyDoesNotOpenALeftCopy(t *testing.T) {
	for name, c := range map[string]struct {
		err  error
		code string
	}{
		"another key":      {errors.New("Fatal: wrong password or no key found"), "repo-wrong-key"},
		"a place far away": {errors.New("dial tcp: no route to host"), "place-unchecked"},
	} {
		t.Run(name, func(t *testing.T) {
			f := newPlacementFixture(t)
			left := leftoverCopy(f, "moved/files", "")
			f.eng.opens[f.root+"/moved/files"] = false
			f.eng.openErr[f.root+"/moved/files"] = c.err
			f.makeRepo(f.root + "/moved/files")
			before, _ := f.st.GetSettings()

			if res := ownFilesAt(f, "moved/files"); res["ok"] != false || res["code"] != c.code {
				t.Fatalf("answer = %v, want %s", res, c.code)
			}
			if !f.hasTarget(left.ID) {
				t.Fatal("the switched-off copy went although the save was refused")
			}
			if s, _ := f.st.GetSettings(); s.FilesPath != before.FilesPath {
				t.Fatalf("files path = %q, want it kept", s.FilesPath)
			}
		})
	}
}

func TestAnOwnFolderOnALeftRemoteCopyTakesItsSignIn(t *testing.T) {
	f := newPlacementFixture(t)
	const place = "s3:https://box.example/bv/files"
	leftoverCopy(f, place, "box-keys")

	if res := ownFilesAt(f, place); res["ok"] != true {
		t.Fatalf("answer = %v, want the own folder there", res)
	}
	row, ok, err := f.st.PrimaryRemoteTarget("files")
	if err != nil || !ok || row.CredsRef != "box-keys" {
		t.Fatalf("remote-primary row = %+v, %v, %v, want it to sign in with box-keys", row, ok, err)
	}
}

func TestAnOwnFolderOnALeftRemoteCopyKeepsTheSignInItHas(t *testing.T) {
	f := newPlacementFixture(t)
	const place = "s3:https://box.example/bv/files"
	leftoverCopy(f, place, "box-keys")
	if _, err := f.st.UpsertPrimaryRemoteTarget("files", store.OffsiteTarget{Repo: place, CredsRef: "own-keys", Enabled: true}); err != nil {
		t.Fatal(err)
	}

	if res := ownFilesAt(f, place); res["ok"] != true {
		t.Fatalf("answer = %v, want the own folder there", res)
	}
	if row, _, _ := f.st.PrimaryRemoteTarget("files"); row.CredsRef != "own-keys" {
		t.Fatalf("remote-primary row signs in with %q, want its own own-keys", row.CredsRef)
	}
}
