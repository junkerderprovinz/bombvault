package api

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func (f *placementFixture) item(domain, key string, lastSuccess int64) placementItem {
	f.t.Helper()
	ref := store.ItemRef{Domain: domain, Key: key}
	home, err := f.st.ItemHome(ref)
	if err != nil {
		f.t.Fatal(err)
	}
	identity, err := f.svc.itemIdentity(ref)
	if err != nil {
		f.t.Fatal(err)
	}
	return placementItem{Key: key, Identity: identity, Home: home, LastSuccess: lastSuccess}
}

func (f *placementFixture) views(domain string, items ...placementItem) map[string]placementView {
	f.t.Helper()
	settings, err := f.st.GetSettings()
	if err != nil {
		f.t.Fatal(err)
	}
	views, err := f.svc.placementViews(settings, domain, items)
	if err != nil {
		f.t.Fatal(err)
	}
	return views
}

func (f *placementFixture) cardOf(domain, key string, lastSuccess int64) placementView {
	f.t.Helper()
	return f.views(domain, f.item(domain, key, lastSuccess))[key]
}

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

func TestTheSegmentFollowsTheStoredRule(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	box := f.namedRepo("Storagebox", "sftp:u1@box.example:/bv")
	f.container("everywhere", "")
	f.container("nowhere", "")
	f.rule("containers", "container:nowhere", store.SkipAll)
	f.container("elsewhere", "")
	f.rule("containers", "container:elsewhere", b2.ID)
	f.container("boxed", box.ID)

	views := f.views("containers",
		f.item("containers", "everywhere", 0), f.item("containers", "nowhere", 0),
		f.item("containers", "elsewhere", 0), f.item("containers", "boxed", 0))
	got := map[string]string{}
	for key, v := range views {
		got[key] = v.Segment
	}
	want := map[string]string{"everywhere": "local-offsite", "nowhere": "local", "elsewhere": "local-offsite", "boxed": "offsite-only"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("segments = %v, want %v", got, want)
	}
}

func TestADomainWithoutTargetsStandsOnLocalWithBothOtherSegmentsLocked(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	v := f.cardOf("containers", "nginx", 0)
	want := map[string]string{"local-offsite": "no-target", "offsite-only": "no-target"}
	if v.Segment != "local" || !reflect.DeepEqual(v.SegmentLocks, want) {
		t.Fatalf("view = %+v, want local with %v", v, want)
	}
}

func TestARemoteRepositoryOpensOffsiteOnlyWithoutATarget(t *testing.T) {
	f := newPlacementFixture(t)
	f.namedRepo("Storagebox", "sftp:u1@box.example:/bv")
	f.container("nginx", "")
	got := f.cardOf("containers", "nginx", 0).SegmentLocks
	if !reflect.DeepEqual(got, map[string]string{"local-offsite": "no-target"}) {
		t.Fatalf("locks = %v, want only local-offsite locked", got)
	}
}

// A direct repository is offered through its target, never on its own, so a
// switched off target leaves Send to empty and the segment has to stay locked.
func TestADirectRepositoryOfASwitchedOffTargetLeavesOffsiteOnlyLocked(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.direct(b2)
	b2.Enabled = false
	if _, err := f.st.UpsertOffsiteTarget(b2); err != nil {
		t.Fatal(err)
	}
	f.container("nginx", "")

	if got := rowsOf(f.options("containers")["sendTo"]); len(got) != 0 {
		t.Fatalf("sendTo = %v, want nothing to send to", got)
	}
	want := map[string]string{"local-offsite": "no-target", "offsite-only": "no-target"}
	if got := f.cardOf("containers", "nginx", 0).SegmentLocks; !reflect.DeepEqual(got, want) {
		t.Fatalf("locks = %v, want %v", got, want)
	}
}

// Copies go to the target itself, off-site only to its direct repository, so
// switching that repository off takes one segment away and leaves the other.
func TestASwitchedOffDirectRepositoryLocksOffsiteOnlyAndLeavesCopies(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	direct := f.direct(b2)
	direct.Enabled = false
	if _, err := f.st.UpsertOffsiteTarget(direct); err != nil {
		t.Fatal(err)
	}
	f.container("nginx", "")

	if got := rowsOf(f.options("containers")["sendTo"]); len(got) != 0 {
		t.Fatalf("sendTo = %v, want nothing to send to", got)
	}
	want := map[string]string{"offsite-only": "no-target"}
	if got := f.cardOf("containers", "nginx", 0).SegmentLocks; !reflect.DeepEqual(got, want) {
		t.Fatalf("locks = %v, want %v", got, want)
	}
}

func TestAHomeWithItsOwnCredentialsOrAtTheTargetTakesNoCopies(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	box := f.namedRepo("Storagebox", "sftp:u1@box.example:/bv")
	direct := f.direct(b2)
	f.container("boxed", box.ID)
	f.container("direct", direct.ID)

	views := f.views("containers", f.item("containers", "boxed", 0), f.item("containers", "direct", 0))
	if got := views["boxed"].SegmentLocks["local-offsite"]; got != "own-credentials" {
		t.Errorf("boxed locks = %v, want own-credentials on local-offsite", views["boxed"].SegmentLocks)
	}
	if v := views["direct"]; v.Segment != "offsite-only" || v.RepoKind != homeDirect || v.SegmentLocks["local-offsite"] != "at-target" {
		t.Errorf("direct = %+v, want offsite-only, direct, at-target", v)
	}
}

func TestTheFirstBackupFixesTheLocation(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	nas := f.namedRepo("NAS Keller", "nas")
	box := f.namedRepo("Storagebox", "sftp:u1@box.example:/bv")
	f.container("kept", nas.ID)
	f.container("boxed", box.ID)

	views := f.views("containers", f.item("containers", "kept", 1_758_000_000), f.item("containers", "boxed", 1_758_000_000))
	kept := views["kept"]
	if !kept.Locked || kept.LockReason != "first-backup" || !reflect.DeepEqual(kept.SegmentLocks, map[string]string{"offsite-only": "home-fixed"}) {
		t.Errorf("kept = %+v, want locked with offsite-only fixed", kept)
	}
	want := map[string]string{"local": "home-fixed", "local-offsite": "own-credentials"}
	if got := views["boxed"].SegmentLocks; !reflect.DeepEqual(got, want) {
		t.Errorf("boxed locks = %v, want %v", got, want)
	}
}

func TestAnOpenItemShowsTheDefaultsLocationAndFollowsBothAxes(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS Keller", "nas")
	f.target("containers", "B2", "b2:bucket:containers")
	f.setDefault("containers", nas.ID)
	f.openContainer("fresh")
	f.container("chosen", "")
	f.rule("containers", "container:chosen", store.SkipAll)

	views := f.views("containers", f.item("containers", "fresh", 0), f.item("containers", "chosen", 0))
	if v := views["fresh"]; !v.HomeFollows || !v.CopiesFollow || v.Repo != nas.ID || v.RepoLabel != "NAS Keller" || v.RepoKind != homeLocal {
		t.Errorf("fresh = %+v, want the default's NAS, following both axes", v)
	}
	if v := views["chosen"]; v.HomeFollows || v.CopiesFollow || !reflect.DeepEqual(v.Skip, []string{store.SkipAll}) {
		t.Errorf("chosen = %+v, want its own rule", v)
	}
}

func TestAContainerWithoutARowIsOpen(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	v := f.views("containers", placementItem{Key: "fresh", Identity: "container:fresh"})["fresh"]
	if !v.HomeFollows || !v.CopiesFollow || v.Segment != "local-offsite" || v.Skip == nil {
		t.Fatalf("view = %+v, want an open item that follows the default", v)
	}
}

func TestASwitchedOffHomeIsMarked(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS Keller", "nas")
	f.container("nginx", nas.ID)
	nas.Enabled = false
	if _, err := f.st.UpsertOffsiteTarget(nas); err != nil {
		t.Fatal(err)
	}
	if v := f.cardOf("containers", "nginx", 0); !v.RepoOff || v.RepoLabel != "NAS Keller" {
		t.Fatalf("view = %+v, want NAS Keller marked off", v)
	}
}

func TestAPausedDomainSaysSoOnEveryCard(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	f.container("nginx", "")
	f.paused("containers")
	if v := f.cardOf("containers", "nginx", 0); !v.Paused {
		t.Fatalf("view = %+v, want paused", v)
	}
}

func TestTargetsKnownOnlyFromTheSettingsFieldShowTheCardUnreadable(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersOffsite = "b2:bucket:containers"
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	v := f.cardOf("containers", "nginx", 0)
	if !v.Unreadable || v.Segment != "" || v.Skip == nil || len(v.Skip) != 0 || v.SegmentLocks == nil {
		t.Fatalf("view = %+v, want unreadable with empty lists", v)
	}
}
