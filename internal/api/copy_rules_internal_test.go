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

	res := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{"copies": map[string]any{"skip": []string{}}})
	if res["code"] != "copies-not-allowed" {
		t.Fatalf("PATCH = %v, want copies-not-allowed", res)
	}
	if _, found := ruleOf(t, f, "containers", "container:nginx"); found {
		t.Fatal("a refused PATCH wrote a rule")
	}
	if res := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{"copies": map[string]any{"skip": []string{store.SkipAll}}}); res["ok"] != true {
		t.Fatalf("Local on a remote repository = %v, want ok", res)
	}
}

func TestAPatchWhoseLaterFieldFailsLeavesNoRuleAndNoChangedField(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	tg := f.container("nginx", "")
	f.backupRun(tg.ID, 100)
	nas := f.namedRepo("NAS", "nas")

	res := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{
		"copies": map[string]any{"skip": []string{b2.ID}},
		"repo":   nas.ID,
	})
	if res["ok"] != false {
		t.Fatalf("PATCH = %v, want a refusal (the container already has backups)", res)
	}
	if _, found := ruleOf(t, f, "containers", "container:nginx"); found {
		t.Fatal("a PATCH that failed on a later field still wrote the rule")
	}
	if got, err := f.st.GetTargetByContainer("nginx"); err != nil || strings.TrimSpace(got.Repo) != "" {
		t.Fatalf("repo = %+v, %v, want it unchanged", got, err)
	}
}

func TestAMoveToARemoteNamedRepositoryWithCopiesIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.namedRepo("Storagebox", "sftp:u@box:/bv")
	f.container("nginx", "")

	res := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{
		"repo":   box.ID,
		"copies": map[string]any{"skip": []string{}},
	})
	if res["code"] != "copies-not-allowed" {
		t.Fatalf("PATCH = %v, want copies-not-allowed", res)
	}
	if got, err := f.st.GetTargetByContainer("nginx"); err != nil || strings.TrimSpace(got.Repo) != "" {
		t.Fatalf("repo = %+v, %v, want it unchanged by the refusal", got, err)
	}
}

func TestAMoveToAnUnknownRepositoryWithCopiesGetsTheRepositoryError(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")

	res := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{
		"repo":   "no-such-repo",
		"copies": map[string]any{"skip": []string{}},
	})
	if res["ok"] != false || res["code"] != "repo-invalid" {
		t.Fatalf("PATCH = %v, want repo-invalid, not a coded copies refusal", res)
	}
	errText, _ := res["error"].(string)
	if !strings.Contains(errText, "no such repository") {
		t.Fatalf("error = %q, want the repository message, not copies-not-allowed", errText)
	}
	if _, found := ruleOf(t, f, "containers", "container:nginx"); found {
		t.Fatal("the refused PATCH wrote a rule")
	}
	if got, err := f.st.GetTargetByContainer("nginx"); err != nil || strings.TrimSpace(got.Repo) != "" {
		t.Fatalf("repo = %+v, %v, want it unchanged", got, err)
	}
}

func TestAMoveToTheDomainPathWithCopiesIsAccepted(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.namedRepo("Storagebox", "sftp:u@box:/bv")
	f.container("nginx", box.ID)

	res := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{
		"repo":   "",
		"copies": map[string]any{"skip": []string{}},
	})
	if res["ok"] != true {
		t.Fatalf("PATCH = %v, want ok", res)
	}
	if got, err := f.st.GetTargetByContainer("nginx"); err != nil || strings.TrimSpace(got.Repo) != "" {
		t.Fatalf("repo = %+v, %v, want the domain path", got, err)
	}
	if skip, found := ruleOf(t, f, "containers", "container:nginx"); !found || len(skip) != 0 {
		t.Fatalf("rule = %v found=%v, want an empty skip", skip, found)
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
	waitForListings(t, f)
}

// TestCopiesOnlyPATCHSurvivesTheHomeGuards pins the part's headline rule:
// copies may change at any time, even on an item with backups and while a
// backup holds the domain, because only a home change is ever refused then.
func TestCopiesOnlyPATCHSurvivesTheHomeGuards(t *testing.T) {
	f := newPlacementFixture(t)
	web := f.container("web", "")
	f.backupRun(web.ID, 100)
	win := f.vm("win11", "")
	f.backupRun(win.ID, 100)
	docs := f.fileSet("docs", "")
	f.backupRun(docs.ID, 100)
	body := map[string]any{"copies": map[string]any{"skip": []string{}}}
	for _, tc := range []struct{ domain, path string }{
		{"containers", "/api/containers/web"},
		{"vms", "/api/vms/win11"},
		{"files", "/api/files/sets/" + docs.ID},
	} {
		if res := f.do(http.MethodPatch, tc.path, body); res["ok"] != true {
			t.Errorf("%s with backups = %v, want ok", tc.path, res)
		}
		unlock, ok := f.svc.tryLockDomainFor(tc.domain, "backup")
		if !ok {
			t.Fatalf("%s: could not take the domain lock", tc.domain)
		}
		res := f.do(http.MethodPatch, tc.path, body)
		unlock()
		if res["ok"] != true {
			t.Errorf("%s while the domain is locked = %v, want ok", tc.path, res)
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

func TestARenamedFileSetCarriesItsRule(t *testing.T) {
	f := newPlacementFixture(t)
	docs := f.fileSet("Docs", "")
	f.rule("files", "fileset:Docs", store.SkipAll)

	if res := f.do(http.MethodPatch, "/api/files/sets/"+docs.ID, map[string]any{"name": "Papers"}); res["ok"] != true {
		t.Fatalf("rename = %v", res)
	}
	if skip, found := ruleOf(t, f, "files", "fileset:Papers"); !found || !slices.Equal(skip, []string{store.SkipAll}) {
		t.Fatalf("rule of the new name = %v found=%v, want [*]", skip, found)
	}
	if _, found := ruleOf(t, f, "files", "fileset:Docs"); found {
		t.Fatal("the old name kept its rule")
	}
}

func TestARenameThatAlsoSetsCopiesKeepsTheRuleUnderTheNewName(t *testing.T) {
	f := newPlacementFixture(t)
	docs := f.fileSet("Docs", "")

	res := f.do(http.MethodPatch, "/api/files/sets/"+docs.ID, map[string]any{
		"name":   "Papers",
		"copies": map[string]any{"skip": []string{store.SkipAll}},
	})
	if res["ok"] != true {
		t.Fatalf("rename with copies = %v", res)
	}
	if skip, found := ruleOf(t, f, "files", "fileset:Papers"); !found || !slices.Equal(skip, []string{store.SkipAll}) {
		t.Fatalf("rule of the new name = %v found=%v, want [*]", skip, found)
	}
	if _, found := ruleOf(t, f, "files", "fileset:Docs"); found {
		t.Fatal("the rule the same request wrote stayed under the old name")
	}
}

func TestARenameOntoANameWithARuleIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	docs := f.fileSet("Docs", "")
	f.rule("files", "fileset:Docs", store.SkipAll)
	f.rule("files", "fileset:Papers")

	res := f.do(http.MethodPatch, "/api/files/sets/"+docs.ID, map[string]any{"name": "Papers"})
	if res["code"] != "copy-rule-taken" {
		t.Fatalf("rename = %v, want copy-rule-taken", res)
	}
	if fs, err := f.st.GetFileSet(docs.ID); err != nil || fs.Name != "Docs" {
		t.Fatalf("set = %+v, %v, want the name unchanged", fs, err)
	}
}

func TestAFailedRenamePutsTheRuleBack(t *testing.T) {
	f := newPlacementFixture(t)
	docs := f.fileSet("Docs", "")
	f.fileSet("Papers", "")
	f.rule("files", "fileset:Docs", store.SkipAll)

	res := f.do(http.MethodPatch, "/api/files/sets/"+docs.ID, map[string]any{"name": "Papers"})
	if res["ok"] != false {
		t.Fatalf("rename onto an existing set = %v, want a refusal", res)
	}
	if _, found := ruleOf(t, f, "files", "fileset:Docs"); !found {
		t.Fatal("the rule did not come back to Docs")
	}
	if _, found := ruleOf(t, f, "files", "fileset:Papers"); found {
		t.Fatal("Papers kept a rule it never had")
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
