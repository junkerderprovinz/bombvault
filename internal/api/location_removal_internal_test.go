package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// backupsFolder is the section folder the fixture keeps containers, vms and
// files in.
var backupsFolder = "path:" + repoLocationKey("backups")

func (f *placementFixture) locationRemoval(id string) locationRemoval {
	f.t.Helper()
	res := f.do("GET", "/api/storage/locations/"+url.PathEscape(id)+"/removal", nil)
	if res["ok"] != true {
		f.t.Fatalf("removal preview: %v", res)
	}
	b, err := json.Marshal(res["removal"])
	if err != nil {
		f.t.Fatal(err)
	}
	var out locationRemoval
	if err := json.Unmarshal(b, &out); err != nil {
		f.t.Fatal(err)
	}
	return out
}

func (f *placementFixture) removeLocation(id string, expect locationRemoval, home string) map[string]any {
	f.t.Helper()
	return f.do("POST", "/api/storage/locations/"+url.PathEscape(id)+"/removal", map[string]any{"expect": expect, "home": home})
}

func (f *placementFixture) locationErasure(id string) destinationErasure {
	f.t.Helper()
	res := f.do("GET", "/api/storage/locations/"+url.PathEscape(id)+"/erasure", nil)
	if res["ok"] != true {
		f.t.Fatalf("erasure preview: %v", res)
	}
	return erasureOf(f.t, res)
}

func (f *placementFixture) eraseLocation(id string, expect locationRemoval, home string, lastCopies ...string) map[string]any {
	f.t.Helper()
	return f.do("POST", "/api/storage/locations/"+url.PathEscape(id)+"/erasure",
		map[string]any{"expect": expect, "lastCopies": lastCopies, "home": home})
}

func (f *placementFixture) sectionPaths() []string {
	f.t.Helper()
	s, err := f.st.GetSettings()
	if err != nil {
		f.t.Fatal(err)
	}
	return []string{s.ContainersPath, s.VMsPath, s.FilesPath}
}

func TestASectionFolderRemovalListsItsSectionsAndOffersAnotherFolder(t *testing.T) {
	f := newPlacementFixture(t)

	got := f.locationRemoval(backupsFolder)

	want := []removalSection{
		{Domain: "containers", Use: removalHome, Repo: "backups/containers", Folder: "containers"},
		{Domain: "vms", Use: removalHome, Repo: "backups/vms", Folder: "vms"},
		{Domain: "files", Use: removalHome, Repo: "backups/files", Folder: "files"},
	}
	if !slices.EqualFunc(got.Sections, want, func(a, b removalSection) bool { return a == b }) {
		t.Fatalf("sections = %+v, want %+v", got.Sections, want)
	}
	if got.Suggest == "" || got.Suggest == "backups" {
		t.Fatalf("suggest = %q, want another folder", got.Suggest)
	}
}

func TestRemovingASectionFolderMovesItsSectionsAndLeavesTheBackups(t *testing.T) {
	f := newPlacementFixture(t)
	f.hold(f.domainPath("vms"), snap("a1", 100, "vm:win"))
	preview := f.locationRemoval(backupsFolder)

	res := f.removeLocation(backupsFolder, preview, "moved/")

	if res["ok"] != true {
		t.Fatalf("answer = %v", res)
	}
	if got, want := f.sectionPaths(), []string{"moved/containers", "moved/vms", "moved/files"}; !slices.Equal(got, want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
	if len(f.held(f.domainPath("vms"))) != 1 || len(f.eng.deletes) != 0 {
		t.Fatalf("the removal touched the backups: deletes %+v", f.eng.deletes)
	}
}

func TestASectionFolderRemovalNeedsAFolderOtherThanItsOwn(t *testing.T) {
	f := newPlacementFixture(t)
	preview := f.locationRemoval(backupsFolder)

	for home, code := range map[string]string{"": "home-needed", " backups/ ": "home-same"} {
		if res := f.removeLocation(backupsFolder, preview, home); res["code"] != code {
			t.Errorf("home %q: answer = %v, want the refusal %s", home, res, code)
		}
	}
	if res := f.removeLocation(backupsFolder, preview, "/etc"); res["ok"] != false {
		t.Errorf("a folder outside the mount root was taken: %v", res)
	}
	if got := f.sectionPaths(); got[1] != "backups/vms" {
		t.Fatalf("a refused removal moved the sections: %v", got)
	}
}

func TestASectionFolderRemovalOverAChangedFolderIsStale(t *testing.T) {
	f := newPlacementFixture(t)
	preview := f.locationRemoval(backupsFolder)
	f.settings(func(s *store.Settings) { s.FilesPath = "elsewhere/files" })

	res := f.removeLocation(backupsFolder, preview, "moved")

	if res["code"] != "stale" || res["removal"] == nil {
		t.Fatalf("answer = %v, want stale with the sections as they are", res)
	}
	if got := f.sectionPaths(); got[0] != "backups/containers" {
		t.Fatalf("a stale removal moved the sections: %v", got)
	}
}

func TestErasingASectionFolderEmptiesItsRepositoriesBeforeTheSectionsMove(t *testing.T) {
	f := newPlacementFixture(t)
	f.signIn()
	f.hold(f.domainPath("vms"), snap("a1", 100, "vm:win"), snap("a2", 200, "vm:win"), snap("hand", 80))

	preview := f.locationErasure(backupsFolder)

	vms := slices.IndexFunc(preview.Sections, func(s erasureSection) bool { return s.Domain == "vms" })
	if vms < 0 || preview.Sections[vms].Use != removalHome || preview.Sections[vms].Backups != 2 || preview.Sections[vms].Kept != "" {
		t.Fatalf("sections = %+v, want vms emptied of its two backups", preview.Sections)
	}
	if last := []erasureEntry{{Domain: "vms", Identity: "vm:win", Backups: 2}}; !slices.Equal(preview.LastCopies, last) {
		t.Fatalf("last copies = %+v, want %+v", preview.LastCopies, last)
	}
	if preview.Key != "erase:"+repoLocationKey("backups") {
		t.Fatalf("key = %q", preview.Key)
	}

	if res := f.eraseLocation(backupsFolder, preview.Removal, "", "vm:win"); res["code"] != "home-needed" {
		t.Fatalf("an erasure without a folder answered %v", res)
	}
	res := f.eraseLocation(backupsFolder, preview.Removal, "moved", "vm:win")
	if res["ok"] != true {
		t.Fatalf("answer = %v", res)
	}
	f.waitForErasure("vms")

	if got := f.held(f.domainPath("vms")); !slices.Equal(got, []string{"hand"}) {
		t.Fatalf("the old folder holds %v, want only the snapshot written by hand", got)
	}
	if got := f.sectionPaths(); got[1] != "moved/vms" {
		t.Fatalf("paths = %v, want the sections moved", got)
	}
}

func TestRemovingANamedRepositorySendsWhatBacksUpThereBackToItsSection(t *testing.T) {
	f := newPlacementFixture(t)
	disk := f.namedRepo("Disk", "disk/bv")
	f.container("plex", disk.ID)
	f.container("sonarr", disk.ID)
	f.setDefault("vms", disk.ID)
	id := "repo:" + disk.ID

	preview := f.locationRemoval(id)

	want := []removalSection{
		{Domain: "containers", Use: removalRepo, Repo: "disk/bv", Items: 2},
		{Domain: "vms", Use: removalRepo, Repo: "disk/bv", Default: true},
	}
	if !slices.EqualFunc(preview.Sections, want, func(a, b removalSection) bool { return a == b }) {
		t.Fatalf("sections = %+v, want %+v", preview.Sections, want)
	}

	if res := f.removeLocation(id, preview, ""); res["ok"] != true {
		t.Fatalf("answer = %v", res)
	}
	if _, err := f.st.GetNamedRepo(disk.ID); err == nil {
		t.Fatal("the repository is still there")
	}
	if h := f.home(store.ItemRef{Domain: "containers", Key: "plex"}); h.Repo != "" || h.Choice != store.RepoOpen {
		t.Fatalf("plex = %+v, want it to follow its section", h)
	}
	if d, _, err := f.st.PlacementDefaultFor("vms"); err != nil || d.Home != "" {
		t.Fatalf("the vms default = %+v, %v; want the section's own folder", d, err)
	}
}

func TestANamedRepositoryRemovalOverNewEntriesIsStale(t *testing.T) {
	f := newPlacementFixture(t)
	disk := f.namedRepo("Disk", "disk/bv")
	f.container("plex", disk.ID)
	preview := f.locationRemoval("repo:" + disk.ID)
	f.container("sonarr", disk.ID)

	if res := f.removeLocation("repo:"+disk.ID, preview, ""); res["code"] != "stale" {
		t.Fatalf("answer = %v, want stale", res)
	}
	if _, err := f.st.GetNamedRepo(disk.ID); err != nil {
		t.Fatalf("a stale removal took the repository: %v", err)
	}
}

func TestErasingANamedRepositoryDeletesEachSectionsBackupsThere(t *testing.T) {
	f := newPlacementFixture(t)
	f.signIn()
	disk := f.namedRepo("Disk", "disk/bv")
	f.container("plex", disk.ID)
	at := f.root + "/disk/bv"
	f.hold(at, snap("p1", 100, "container:plex"), snap("w1", 110, "vm:win"), snap("hand", 80))
	id := "repo:" + disk.ID

	preview := f.locationErasure(id)

	if len(preview.Sections) != 2 || preview.Backups != 2 {
		t.Fatalf("sections = %+v, %d backups; want containers and vms with one backup each", preview.Sections, preview.Backups)
	}
	if res := f.eraseLocation(id, preview.Removal, "", "container:plex", "vm:win"); res["ok"] != true {
		t.Fatalf("answer = %v", res)
	}
	f.waitForErasure("containers")
	f.waitForErasure("vms")

	if got := f.held(at); !slices.Equal(got, []string{"hand"}) {
		t.Fatalf("the repository holds %v, want only the snapshot written by hand", got)
	}
	if _, err := f.st.GetNamedRepo(disk.ID); err == nil {
		t.Fatal("the repository is still there after its backups went")
	}
}

func TestRemovingATargetOfItsOwnClearsTheSectionsOffsiteField(t *testing.T) {
	f := newPlacementFixture(t)
	field := f.fieldTarget("vms", "rest:https://other:8000/vms")
	id := "target:" + field.ID

	preview := f.locationRemoval(id)

	if len(preview.Sections) != 1 || preview.Sections[0].Use != removalCopy || !preview.Sections[0].Primary {
		t.Fatalf("sections = %+v, want the vms copy from the off-site field", preview.Sections)
	}
	if res := f.removeLocation(id, preview, ""); res["ok"] != true {
		t.Fatalf("answer = %v", res)
	}
	if f.hasTarget(field.ID) {
		t.Fatal("the target is still there")
	}
	if s, err := f.st.GetSettings(); err != nil || s.VMsOffsite != "" {
		t.Fatalf("the off-site field reads %q, %v; want it cleared", s.VMsOffsite, err)
	}
}

func TestATargetThatFollowsADestinationIsNotALocationOfItsOwn(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.destination("Box", boxRepo)
	vms := f.tickDestination(box, "vms")

	code, _ := f.call("GET", "/api/storage/locations/"+url.PathEscape("target:"+vms.ID)+"/removal", nil)

	if code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", code)
	}
}
