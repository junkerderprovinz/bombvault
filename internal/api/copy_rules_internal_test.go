package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func ruleOf(t *testing.T, f *placementFixture, domain, identity string) ([]string, bool) {
	t.Helper()
	r, found, err := f.st.CopyRuleFor(domain, identity)
	if err != nil {
		t.Fatal(err)
	}
	return r.Skip, found
}

func TestTakingATargetAwayNamesTheCopiesItKeeps(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	hz := f.target("containers", "Hetzner", "sftp:u@box:/containers")
	f.container("nginx", "")
	f.listing("containers", b2.ID, 500)
	f.listing("containers", hz.ID, 500, copiesRow("container:nginx", 14, 400))

	res := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{"copies": map[string]any{"skip": []string{hz.ID}}})

	dropped, _ := res["dropped"].([]any)
	if res["ok"] != true || len(dropped) != 1 {
		t.Fatalf("PATCH = %v, want ok and one dropped target", res)
	}
	d := dropped[0].(map[string]any)
	if d["targetId"] != hz.ID || d["name"] != "Hetzner" || d["copies"] != float64(14) || d["appendOnly"] != false {
		t.Fatalf("dropped = %v, want Hetzner keeping 14", d)
	}
	if skip, found := ruleOf(t, f, "containers", "container:nginx"); !found || !slices.Equal(skip, []string{hz.ID}) {
		t.Fatalf("rule = %v found=%v, want [Hetzner]", skip, found)
	}
}

func TestATargetNeverListedIsListedInTheBackground(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.replicated("containers")
	f.container("nginx", "")

	res := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{"copies": map[string]any{"skip": []string{store.SkipAll}}})
	dropped, _ := res["dropped"].([]any)
	if len(dropped) != 1 || dropped[0].(map[string]any)["copies"] != nil {
		t.Fatalf("PATCH = %v, want B2 dropped with copies null", res)
	}
	waitForListings(t, f)
	if _, listed, err := f.st.TargetObservationFor("containers", b2.ID); err != nil || !listed {
		t.Fatalf("B2 was not listed in the background (listed=%v err=%v)", listed, err)
	}
}

func TestCopiesTakeFollowOrOneSkip(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	f.container("nginx", "")
	for _, c := range []struct {
		copies map[string]any
		code   string
	}{
		{map[string]any{}, "invalid-placement"},
		{map[string]any{"follow": false}, "invalid-placement"},
		{map[string]any{"follow": true, "skip": []string{}}, "invalid-placement"},
		{map[string]any{"skip": []string{store.SkipAll, "x"}}, "invalid-placement"},
		{map[string]any{"skip": []string{"not-a-target"}}, "unknown-target"},
	} {
		res := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{"copies": c.copies})
		if res["ok"] != false || res["code"] != c.code {
			t.Errorf("copies %v = %v, want code %s", c.copies, res, c.code)
		}
	}
	if _, found := ruleOf(t, f, "containers", "container:nginx"); found {
		t.Fatal("a refused PATCH wrote a rule")
	}
}

func TestAnItemOnARemoteRepositoryTakesNoCopies(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	box := f.namedRepo("Storagebox", "sftp:u@box:/bv")
	f.container("nginx", box.ID)

	res := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{
		"copies":            map[string]any{"skip": []string{}},
		"includeInSchedule": true,
	})
	if res["code"] != "copies-not-allowed" {
		t.Fatalf("PATCH = %v, want copies-not-allowed", res)
	}
	if tg, err := f.st.GetTargetByContainer("nginx"); err != nil || tg.IncludeInSchedule {
		t.Fatalf("the refused PATCH changed the schedule flag: %+v, %v", tg, err)
	}
	if res := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{"copies": map[string]any{"skip": []string{store.SkipAll}}}); res["ok"] != true {
		t.Fatalf("Local on a remote repository = %v, want ok", res)
	}
}

func TestFollowingTheDefaultDeletesTheRule(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	f.container("nginx", "")
	f.rule("containers", "container:nginx", store.SkipAll)

	if res := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{"copies": map[string]any{"follow": true}}); res["ok"] != true {
		t.Fatalf("PATCH = %v", res)
	}
	if _, found := ruleOf(t, f, "containers", "container:nginx"); found {
		t.Fatal("follow left the rule in place")
	}
}

func TestVMAndFileSetRoutesTakeCopies(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("vms", "B2", "b2:bucket:vms")
	f.target("files", "B2 files", "b2:bucket:files")
	docs := f.fileSet("Docs", "")
	body := map[string]any{"copies": map[string]any{"skip": []string{store.SkipAll}}}

	if res := f.do(http.MethodPatch, "/api/vms/Windows%2011", body); res["ok"] != true {
		t.Fatalf("VM PATCH = %v", res)
	}
	if res := f.do(http.MethodPatch, "/api/files/sets/"+docs.ID, body); res["ok"] != true {
		t.Fatalf("file set PATCH = %v", res)
	}
	for domain, identity := range map[string]string{"vms": "vm:Windows 11", "files": "fileset:Docs"} {
		if skip, found := ruleOf(t, f, domain, identity); !found || !slices.Equal(skip, []string{store.SkipAll}) {
			t.Errorf("rule of %s = %v found=%v, want [*]", identity, skip, found)
		}
	}
}

func TestUnreadableRulesRefuseTheChange(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	f.container("nginx", "")
	breakRule(t, f, "containers", "container:plex")

	res := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{"copies": map[string]any{"skip": []string{store.SkipAll}}})
	if res["code"] != "placement-unreadable" {
		t.Fatalf("PATCH = %v, want placement-unreadable", res)
	}
}

func TestThePreviewCountsTheUploadAndWritesNothing(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	nas := f.namedRepo("NAS", "nas")
	f.container("nginx", "")
	f.container("plex", nas.ID)
	f.rule("containers", "container:nginx", store.SkipAll)
	f.listing("containers", b2.ID, 500, copiesRow("container:nginx", 1, 100))
	f.hold(f.domainPath("containers"), snap("n1", 100, "container:nginx"), snap("n2", 200, "container:nginx"), snap("n3", 300, "container:nginx"))
	if err := f.st.MarkRepoEstablished(f.root + "/nas"); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(f.root + "/nas"); err != nil {
		t.Fatal(err)
	}

	res := f.do(http.MethodPost, "/api/items/containers/nginx/placement/preview", map[string]any{"copies": map[string]any{"skip": []string{}}})

	added, _ := res["added"].([]any)
	dropped, _ := res["dropped"].([]any)
	if res["ok"] != true || len(added) != 1 || len(dropped) != 0 {
		t.Fatalf("preview = %v, want B2 added and nothing dropped", res)
	}
	a := added[0].(map[string]any)
	if a["targetId"] != b2.ID || a["snapshots"] != float64(2) {
		t.Fatalf("added = %v, want about 2 snapshots for B2", a)
	}
	if u, _ := a["uncheckable"].([]any); len(u) != 1 || u[0] != "NAS" {
		t.Fatalf("uncheckable = %v, want [NAS]", a["uncheckable"])
	}
	if skip, _ := ruleOf(t, f, "containers", "container:nginx"); !slices.Equal(skip, []string{store.SkipAll}) {
		t.Fatalf("the preview changed the rule to %v", skip)
	}
}

func TestMalformedItemPathsAre400(t *testing.T) {
	f := newPlacementFixture(t)
	for _, c := range []struct{ method, path string }{
		{http.MethodPost, "/api/items/nope/x/placement/preview"},
		{http.MethodPost, "/api/items/containers/-rf/placement/preview"},
		// A project folder has no card and no rule; its name never reaches one.
		{http.MethodPatch, "/api/containers/stack:immich"},
	} {
		rec := httptest.NewRecorder()
		f.h.Router().ServeHTTP(rec, jsonReq(c.method, c.path, strings.NewReader(`{"copies":{"skip":["*"]}}`)))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s %s = %d, want 400", c.method, c.path, rec.Code)
		}
	}
}
