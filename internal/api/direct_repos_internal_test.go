package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestRepoLocationsOverlapComparesPathElements(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"b2:bkt:containers", "b2:bkt:containers/", true},
		{"b2:bkt:containers", "b2:bkt:containers/sub", true},
		{"b2:bkt:containers", "b2:bkt:containers-direct", false},
		{"b2:bkt", "b2:bkt:containers-direct", true},
		{"b2:bkt", "b2:bkt-direct", false},
		{"b2:bkt:x", "gs:bkt:/x", false},
		{"s3:https://s3.example.com/bkt/containers", "s3:S3.example.com/bkt/containers/x", true},
		{"s3:s3.example.com/bkt", "s3:s3.example.com/bkt2", false},
		{"rest:https://u:p@host:8000/u/containers", "rest:http://host:8000/u", true},
		{"rest:http://host:8000/a", "rest:http://host:8001/a", false},
		{"sftp:u@nas:/srv/restic", "sftp://u@nas//srv/restic/x", true},
		{"rclone:remote:bkt/a", "rclone:remote:bkt/a/b", true},
		{"/mnt/user/backups", "/mnt/user/backups/containers", true},
		{"/mnt/user/backups", "/mnt/user/backups-direct", false},
		{"/mnt/user/Archiv", "/mnt/user/archiv", false},
		{"/mnt/b2", "b2:mnt", false},
	}
	for _, c := range cases {
		if got := repoLocationsOverlap(c.a, c.b); got != c.want {
			t.Errorf("repoLocationsOverlap(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
		if got := repoLocationsOverlap(c.b, c.a); got != c.want {
			t.Errorf("repoLocationsOverlap(%q, %q) = %v, want %v", c.b, c.a, got, c.want)
		}
	}
}

func TestDirectRepositoryRefusalsCarryTheirCodes(t *testing.T) {
	for err, want := range map[error]string{
		fmt.Errorf("x: %w", errNestedLocation):                 "nested-location",
		errMirroredField:                                       "mirrored-field",
		store.ErrCompanionTaken:                                "companion-taken",
		store.ErrNotOffsiteTarget:                              "unknown-target",
		errForeignDomain:                                       "foreign-domain",
		fmt.Errorf("%w: %w", errRepoInvalid, errForeignDomain): "foreign-domain",
		errTargetInUse:                                         "target-in-use",
	} {
		if got := placementCode(err); got != want {
			t.Errorf("placementCode(%v) = %q, want %q", err, got, want)
		}
	}
}

func TestADirectRepositoryServesOnlyItsTargetsDomain(t *testing.T) {
	f := newPlacementFixture(t)
	vms := f.direct(f.target("vms", "B2", "b2:bkt:vms"))
	files := f.direct(f.target("files", "B2 files", "b2:bkt:files"))
	if err := f.svc.validateItemRepoID("containers", vms.ID); !errors.Is(err, errForeignDomain) {
		t.Fatalf("a containers item on the VMs direct repository: %v", err)
	}
	if err := f.svc.validateItemRepoID("vms", vms.ID); err != nil {
		t.Fatalf("a VM on its own domain's direct repository: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(f.root, "photos"), 0o750); err != nil {
		t.Fatal(err)
	}
	if res := f.do("POST", "/api/files/sets", map[string]any{"name": "Photos", "path": "photos", "repo": vms.ID}); res["code"] != "foreign-domain" {
		t.Fatalf("folder set on the VMs direct repository = %v", res)
	}
	if res := f.do("POST", "/api/files/sets", map[string]any{"name": "Photos", "path": "photos", "repo": files.ID}); res["ok"] != true {
		t.Fatalf("folder set on the files direct repository = %v", res)
	}
	if res := f.do("PUT", "/api/placement/default/containers", map[string]any{"home": vms.ID}); res["code"] != "foreign-domain" {
		t.Fatalf("containers default on the VMs direct repository = %v", res)
	}
}

func TestADirectRepositoryIsItsOwnKindOfHomeWhereverItLies(t *testing.T) {
	f := newPlacementFixture(t)
	d := f.direct(f.target("containers", "NAS", "backups/nas-offsite"))
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	kind := f.svc.homeKindOf(settings, "containers", d.ID, map[string]store.OffsiteTarget{d.ID: d})
	if kind != homeDirect || kind.copySource() {
		t.Fatalf("homeKindOf = %q, copy source %v", kind, kind.copySource())
	}
}

func TestALocationInsideOrAroundAnotherIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bkt:containers")
	f.namedRepo("NAS", "backups/nas")
	for _, loc := range []string{"b2:bkt:containers/inner", "b2:bkt", "backups/nas/inner", "backups"} {
		res := f.do("POST", "/api/repos", map[string]any{"name": "x", "repo": loc})
		if msg, _ := res["error"].(string); res["ok"] != false || !strings.Contains(msg, "lies inside") {
			t.Errorf("named repository at %s: %v", loc, res)
		}
	}
	if res := f.do("POST", "/api/repos", map[string]any{"name": "beside", "repo": "b2:bkt:containers-direct"}); res["ok"] != true {
		t.Fatalf("a location beside the target was refused: %v", res["error"])
	}
	res := f.do("POST", "/api/offsite/targets", map[string]any{
		"domain": "vms", "name": "inner", "repo": "backups/nas/offsite", "enabled": true,
	})
	if msg, _ := res["error"].(string); res["ok"] != false || !strings.Contains(msg, "named repository") {
		t.Errorf("target inside a named repository: %v", res)
	}
	res = f.do("POST", "/api/repos", map[string]any{"name": "x", "repo": "backups/containers/inner"})
	if msg, _ := res["error"].(string); res["ok"] != false || !strings.Contains(msg, "domain's own repository") {
		t.Errorf("named repository inside a domain's own repository: %v", res)
	}
	f.fieldTarget("flash", "b2:bkt:flash-offsite")
	res = f.do("POST", "/api/repos", map[string]any{"name": "x", "repo": "b2:bkt:flash-offsite/inner"})
	if msg, _ := res["error"].(string); res["ok"] != false || !strings.Contains(msg, "domain's off-site destination") {
		t.Errorf("named repository inside a domain's off-site field: %v", res)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	v := toView(settings)
	v.ContainersPath = "backups/nas/containers"
	res = f.do("PUT", "/api/settings", v)
	if msg, _ := res["error"].(string); res["ok"] != false || !strings.Contains(msg, "Repositories") {
		t.Errorf("domain path inside a named repository: %v", res)
	}
}

func TestAnImportWithNestedLocationsIsNotRefused(t *testing.T) {
	f := newPlacementFixture(t)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	exp := settingsExport{
		Settings:       toView(settings),
		OffsiteTargets: []offsiteTargetView{{Domain: "containers", Name: "B2", Repo: "b2:bkt:c", Enabled: true}},
		NamedRepos:     []offsiteTargetView{{Name: "inner", Repo: "b2:bkt:c/inner", Enabled: true}},
	}
	if msg := f.h.rejectImportCollisions(exp); msg != "" {
		t.Fatalf("an older file with nested locations must still import: %s", msg)
	}
	exp.NamedRepos[0].Repo = "b2:bkt:c"
	if msg := f.h.rejectImportCollisions(exp); msg == "" {
		t.Fatal("the same place twice is still refused")
	}
}

func TestDirectLocationForSuggestsAPlaceBesideTheTarget(t *testing.T) {
	cases := []struct {
		repo string
		want directSuggestion
	}{
		{"b2:bkt:containers", directSuggestion{Location: "b2:bkt:containers-direct"}},
		{"b2:bkt:containers/", directSuggestion{Location: "b2:bkt:containers-direct"}},
		{"b2:bkt", directSuggestion{Location: "b2:bkt-direct", Note: "bucket-root"}},
		{"b2:bkt:", directSuggestion{Location: "b2:bkt-direct", Note: "bucket-root"}},
		{"gs:bkt:/", directSuggestion{Location: "gs:bkt-direct", Note: "bucket-root"}},
		{"s3:https://s3.example.com/bkt", directSuggestion{Location: "s3:https://s3.example.com/bkt-direct", Note: "bucket-root"}},
		{"s3:https://s3.example.com/bkt/vms", directSuggestion{Location: "s3:https://s3.example.com/bkt/vms-direct"}},
		{"rest:https://u:p@host:8000/u", directSuggestion{Note: "path-needed"}},
		{"rest:https://host:8000/", directSuggestion{Note: "path-needed"}},
		{"rest:https://u:p@host:8000/u/containers", directSuggestion{Location: "rest:https://u:p@host:8000/u/containers-direct"}}, //nolint:gosec // G101: a test-fixture literal, not a real credential
		{"sftp:u@nas:/srv/restic/files", directSuggestion{Location: "sftp:u@nas:/srv/restic/files-direct"}},
		{"sftp:u@nas:", directSuggestion{Note: "path-needed"}},
		{"rclone:remote:", directSuggestion{Note: "path-needed"}},
		{"rclone:remote:bkt/x", directSuggestion{Location: "rclone:remote:bkt/x-direct"}},
		{"backups/offsite", directSuggestion{Location: "backups/offsite-direct"}},
	}
	for _, c := range cases {
		if got := directLocationFor(store.OffsiteTarget{Repo: c.repo}); got != c.want {
			t.Errorf("directLocationFor(%q) = %+v, want %+v", c.repo, got, c.want)
		}
	}
}

func TestTestingADirectLocationCreatesNothing(t *testing.T) {
	f := newPlacementFixture(t)
	target := f.target("containers", "NAS", "backups/nas-offsite")
	res := f.do("GET", "/api/offsite/targets/"+target.ID+"/direct", nil)
	if res["ok"] != true || res["repo"] != nil {
		t.Fatalf("GET direct = %v", res)
	}
	sug := res["suggestion"].(map[string]any)
	if sug["location"] != "backups/nas-offsite-direct" || sug["note"] != "" {
		t.Fatalf("suggestion = %v", sug)
	}
	res = f.do("POST", "/api/offsite/targets/"+target.ID+"/direct/test", map[string]any{"location": "backups/nas-offsite-direct"})
	if res["ok"] != true || res["reachable"] != true || res["initialized"] != false {
		t.Fatalf("test of a path that does not exist yet = %v", res)
	}
	if _, err := os.Stat(filepath.Join(f.root, "backups", "nas-offsite-direct")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the test left something behind: %v", err)
	}
	rows, err := f.st.ListNamedRepos()
	if err != nil || len(rows) != 0 {
		t.Fatalf("the test wrote a repository row: %v, %v", rows, err)
	}
}

func TestTestingADirectLocationReportsWhatIsThere(t *testing.T) {
	f := newPlacementFixture(t)
	target := f.target("vms", "B2", "b2:bkt:vms")
	path := "/api/offsite/targets/" + target.ID + "/direct/test"
	if res := f.do("POST", path, map[string]any{"location": "b2:bkt:vms-direct"}); res["reachable"] != true || res["initialized"] != true {
		t.Fatalf("an existing repository = %v", res)
	}
	f.eng.opens["b2:bkt:vms-direct"] = false
	if res := f.do("POST", path, map[string]any{"location": "b2:bkt:vms-direct"}); res["reachable"] != true || res["initialized"] != false {
		t.Fatalf("an empty remote location = %v", res)
	}
	if res := f.do("POST", path, map[string]any{"location": "b2:bkt:vms/inner"}); res["ok"] != false || res["code"] != "nested-location" {
		t.Fatalf("a location inside the target = %v", res)
	}
	if res := f.do("POST", "/api/offsite/targets/unknown/direct/test", map[string]any{"location": "b2:bkt:x"}); res["code"] != "unknown-target" {
		t.Fatalf("an unknown target = %v", res)
	}
}

func TestCreatingADirectRepositoryEnsuresItOnlyThen(t *testing.T) {
	f := newPlacementFixture(t)
	target := f.target("containers", "NAS", "backups/nas-offsite")
	dir := filepath.Join(f.root, "backups", "nas-offsite-direct")
	f.eng.opens[filepath.ToSlash(dir)] = false
	res := f.do("POST", "/api/repos", map[string]any{"name": "", "repo": "backups/nas-offsite-direct", "companionOf": target.ID})
	if res["ok"] != true {
		t.Fatalf("create = %v", res)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("the repository was not set up: %v", err)
	}
	repo := res["repo"].(map[string]any)
	if repo["name"] != "NAS direct" || repo["companionOf"] != target.ID || repo["companionLost"] != false {
		t.Fatalf("repo = %v", repo)
	}
	res = f.do("POST", "/api/repos", map[string]any{"name": "again", "repo": "backups/nas-offsite-direct2", "companionOf": target.ID})
	if res["code"] != "companion-taken" {
		t.Fatalf("a second direct repository = %v", res)
	}
}

func TestCreatingADirectRepositoryRefusesWhatTheTargetDecides(t *testing.T) {
	f := newPlacementFixture(t)
	target := f.target("vms", "B2", "b2:bkt:vms")
	flash := f.target("flash", "B2 flash", "b2:bkt:flash")
	for _, c := range []struct {
		body map[string]any
		code string
	}{
		{map[string]any{"name": "x", "repo": "b2:bkt:vms-direct", "companionOf": target.ID, "immutable": true}, "mirrored-field"},
		{map[string]any{"name": "x", "repo": "b2:bkt:vms-direct", "companionOf": target.ID, "credsRef": "set-2"}, "mirrored-field"},
		{map[string]any{"name": "x", "repo": "b2:bkt:vms-direct", "companionOf": "missing"}, "unknown-target"},
		{map[string]any{"name": "x", "repo": "b2:bkt:flash-direct", "companionOf": flash.ID}, "unknown-target"},
		{map[string]any{"name": "x", "repo": "b2:bkt:vms/inside", "companionOf": target.ID}, "nested-location"},
	} {
		if res := f.do("POST", "/api/repos", c.body); res["ok"] != false || res["code"] != c.code {
			t.Errorf("%v = %v, want code %s", c.body, res, c.code)
		}
	}
	rows, err := f.st.ListNamedRepos()
	if err != nil || len(rows) != 0 {
		t.Fatalf("a refused create wrote %v, %v", rows, err)
	}
}

func TestConnectingARepositoryNeedsTheTargetsCredentialsToOpenIt(t *testing.T) {
	f := newPlacementFixture(t)
	target := f.target("containers", "B2", "b2:bkt:containers")
	old := f.namedRepo("B2 old", "b2:bkt:containers-direct")
	path := "/api/repos/" + old.ID + "/connect"
	f.eng.opens["b2:bkt:containers-direct"] = false
	if res := f.do("POST", path, map[string]any{"targetId": target.ID}); res["ok"] != false {
		t.Fatalf("connect without opening = %v", res)
	}
	if _, found, _ := f.st.CompanionFor(target.ID); found {
		t.Fatal("a repository that did not open was connected")
	}
	f.eng.opens["b2:bkt:containers-direct"] = true
	res := f.do("POST", path, map[string]any{"targetId": target.ID})
	if res["ok"] != true || res["repo"].(map[string]any)["companionOf"] != target.ID {
		t.Fatalf("connect = %v", res)
	}
	other := f.namedRepo("other", "b2:bkt:other")
	if res := f.do("POST", "/api/repos/"+other.ID+"/connect", map[string]any{"targetId": target.ID}); res["code"] != "companion-taken" {
		t.Fatalf("a second repository for one target = %v", res)
	}
}

func TestConnectingARepositoryAlreadyLinkedToAnotherTargetIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	first := f.target("containers", "B2", "b2:bkt:containers")
	second := f.target("containers", "B2 two", "b2:bkt:containers-two")
	repo := f.namedRepo("B2 direct", "b2:bkt:containers-direct")
	if res := f.do("POST", "/api/repos/"+repo.ID+"/connect", map[string]any{"targetId": first.ID}); res["ok"] != true {
		t.Fatalf("connect = %v", res)
	}
	if res := f.do("POST", "/api/repos/"+repo.ID+"/connect", map[string]any{"targetId": second.ID}); res["ok"] != false {
		t.Fatalf("connecting an already-linked repository to another target = %v", res)
	}
	if companion, found, _ := f.st.CompanionFor(first.ID); !found || companion.ID != repo.ID {
		t.Fatalf("the original link was not kept: %+v, %v", companion, found)
	}
}

func TestADirectRepositoryChangesOnlyItsNameAndSwitch(t *testing.T) {
	f := newPlacementFixture(t)
	target := f.target("containers", "B2", "b2:bkt:containers")
	d := f.direct(target)
	path := "/api/repos/" + d.ID
	if res := f.do("PATCH", path, map[string]any{"name": "B2 straight", "enabled": false}); res["ok"] != true {
		t.Fatalf("rename and switch off = %v", res)
	}
	for field, value := range map[string]any{
		"repo": "b2:bkt:elsewhere", "credsRef": "set-9", "storageClass": "GLACIER_IR",
		"limitUpload": 5, "limitDownload": 5, "immutable": !d.Immutable,
	} {
		if res := f.do("PATCH", path, map[string]any{field: value}); res["ok"] != false || res["code"] != "mirrored-field" {
			t.Errorf("%s = %v", field, res)
		}
	}
	if res := f.do("PATCH", path, map[string]any{"immutable": d.Immutable, "repo": d.Repo}); res["ok"] != true {
		t.Fatalf("sending the stored values back is no change: %v", res)
	}
	if res := f.do("PATCH", path, map[string]any{"companionOf": target.ID}); res["ok"] != false {
		t.Fatalf("a link set by an edit = %v", res)
	}
	got, err := f.st.GetNamedRepo(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "B2 straight" || got.Enabled || !got.MirroredEqual(target) {
		t.Fatalf("after the edits: %+v", got)
	}
}

func TestMirroredFieldRefusalNamesTheField(t *testing.T) {
	f := newPlacementFixture(t)
	target := f.target("containers", "B2", "b2:bkt:containers")
	d := f.direct(target)
	path := "/api/repos/" + d.ID

	res := f.do("PATCH", path, map[string]any{"repo": "b2:bkt:elsewhere"})
	if res["ok"] != false || res["code"] != "mirrored-field" {
		t.Fatalf("repo = %v", res)
	}
	if got, _ := res["fields"].([]any); len(got) != 1 || got[0] != "repo" {
		t.Fatalf("fields = %v, want [repo]", res["fields"])
	}

	res = f.do("PATCH", path, map[string]any{"credsRef": "set-9", "immutable": !d.Immutable})
	if res["ok"] != false || res["code"] != "mirrored-field" {
		t.Fatalf("credsRef+immutable = %v", res)
	}
	if got, _ := res["fields"].([]any); len(got) != 2 || got[0] != "credsRef" || got[1] != "immutable" {
		t.Fatalf("fields = %v, want [credsRef immutable]", res["fields"])
	}

	vms := f.target("vms", "B2 vms", "b2:bkt:vms")
	res = f.do("POST", "/api/repos", map[string]any{"name": "x", "repo": "b2:bkt:vms-direct", "companionOf": vms.ID, "credsRef": "set-2"})
	if res["ok"] != false || res["code"] != "mirrored-field" {
		t.Fatalf("create with credsRef = %v", res)
	}
	if got, _ := res["fields"].([]any); len(got) != 1 || got[0] != "credsRef" {
		t.Fatalf("fields = %v, want [credsRef]", res["fields"])
	}
}

func TestAlreadyOffSiteCountsADirectRepositoryWhereverItLies(t *testing.T) {
	direct := store.OffsiteTarget{ID: "d", CompanionOf: "t"}
	plain := store.OffsiteTarget{ID: "n"}
	cases := []struct {
		name string
		ref  domainRepoRef
		want bool
	}{
		{"local direct repository", namedRef("/mnt/user/nas-direct", direct), true},
		{"remote direct repository", namedRef("b2:bkt:x-direct", direct), true},
		{"local named repository", namedRef("/mnt/user/nas", plain), false},
		{"remote named repository", namedRef("b2:bkt:cold", plain), true},
		{"remote domain path", ownRef("s3:host/bkt/containers"), false},
	}
	for _, c := range cases {
		if got := alreadyOffSite(c.ref); got != c.want {
			t.Errorf("%s: alreadyOffSite = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestADirectRepositoryIsNeverACopySource(t *testing.T) {
	f := newPlacementFixture(t)
	d := f.direct(f.target("containers", "NAS", "backups/nas-offsite"))
	f.container("web", d.ID)
	f.container("db", "")
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	loc, err := f.svc.resolveRepo(d.Repo)
	if err != nil {
		t.Fatal(err)
	}
	refs, _ := f.svc.offsiteReplicationSources(settings, "containers")
	if len(refs) == 0 {
		t.Fatal("the domain path is no longer a source")
	}
	for _, r := range refs {
		if sameRepoLocation(r.Loc, loc) {
			t.Fatalf("the direct repository %s is a copy source", r.Loc)
		}
	}
}

func forgetsAt(f *placementFixture, loc string) []forgetCall {
	f.eng.mu.Lock()
	defer f.eng.mu.Unlock()
	var out []forgetCall
	for _, c := range f.eng.forgets {
		if filepath.ToSlash(c.Repo) == filepath.ToSlash(loc) {
			out = append(out, c)
		}
	}
	return out
}

func prunedAt(f *placementFixture, loc string) bool {
	f.eng.mu.Lock()
	defer f.eng.mu.Unlock()
	for _, p := range f.eng.prunes {
		if filepath.ToSlash(p) == filepath.ToSlash(loc) {
			return true
		}
	}
	return false
}

func directWithRules(t *testing.T, f *placementFixture, domain string, last, daily int, immutable bool) (store.OffsiteTarget, string) {
	t.Helper()
	target := f.target(domain, "NAS "+domain, "backups/nas-"+domain)
	target.RetentionKeepLast, target.RetentionKeepDaily, target.Immutable = last, daily, immutable
	target, err := f.st.UpsertOffsiteTarget(target)
	if err != nil {
		t.Fatal(err)
	}
	d := f.direct(target)
	loc, err := f.svc.resolveRepo(d.Repo)
	if err != nil {
		t.Fatal(err)
	}
	return d, loc
}

func TestADirectRepositoryAgesByItsTargetsRules(t *testing.T) {
	f := newPlacementFixture(t)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.RetentionKeepLast = 7
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	d, loc := directWithRules(t, f, "containers", 3, 2, false)
	f.container("web", d.ID)
	ctx := context.Background()

	f.svc.applyRetention(ctx, loc, settings, restic.Mode{}, "container:web", "containers")
	want := restic.RetentionPolicy{KeepLast: 3, KeepDaily: 2, Direct: true}
	if got := forgetsAt(f, loc); len(got) != 1 || got[0].Policy != want {
		t.Fatalf("after a backup the direct repository forgot with %+v, want %+v", got, want)
	}
	f.svc.applyRetention(ctx, f.domainPath("containers"), settings, restic.Mode{}, "container:db", "containers")
	if got := forgetsAt(f, f.domainPath("containers")); len(got) != 1 || got[0].Policy != (restic.RetentionPolicy{KeepLast: 7}) {
		t.Fatalf("the domain path forgot with %+v, want the local rule", got)
	}

	f.hold(loc, snap("a1", 100, "container:web", restic.DirectTag))
	if err := f.svc.pruneDomain(ctx, "containers", "local", true); err != nil {
		t.Fatal(err)
	}
	pruned := forgetsAt(f, loc)
	if len(pruned) < 2 {
		t.Fatalf("a manual prune did not forget in the direct repository: %+v", pruned)
	}
	for _, c := range pruned[1:] {
		if c.Policy != want {
			t.Fatalf("a manual prune forgot the direct repository with %+v, want %+v", c.Policy, want)
		}
	}

	v, vLoc := directWithRules(t, f, "vms", 3, 0, true)
	f.vm("win11", v.ID)
	f.svc.applyRetention(ctx, vLoc, settings, restic.Mode{}, "vm:win11", "vms")
	if got := forgetsAt(f, vLoc); len(got) != 0 {
		t.Fatalf("an append-only direct repository was forgotten: %+v", got)
	}
}

func TestABulkRunPrunesADirectRepositoryWithoutALocalRule(t *testing.T) {
	f := newPlacementFixture(t)
	d, loc := directWithRules(t, f, "containers", 3, 0, false)
	f.container("web", d.ID)
	f.svc.PruneAfterBulk(context.Background(), "containers")
	if !prunedAt(f, loc) {
		t.Fatal("the direct repository was not pruned after a bulk run although its rules forgot snapshots")
	}
}

func TestTheDirectTagIsNoIdentity(t *testing.T) {
	got := identityTags([]restic.Snapshot{{Tags: []string{"container:web", restic.DirectTag}}})
	if len(got) != 1 || got[0] != "container:web" {
		t.Fatalf("identityTags = %v", got)
	}
}

// keepTagEngine forgets the way restic does with --keep-tag bv:direct: a policy
// that is not a direct repository's own keeps every DirectTag snapshot.
type keepTagEngine struct {
	*placementEngine
}

func (e keepTagEngine) ForgetPolicy(ctx context.Context, repo string, p restic.RetentionPolicy, mode restic.Mode, tag string, prune bool) error {
	loc := filepath.ToSlash(repo)
	var kept []restic.Snapshot
	if !p.Direct {
		e.mu.Lock()
		for _, sn := range e.snaps[loc] {
			if slices.Contains(sn.Tags, restic.DirectTag) {
				kept = append(kept, sn)
			}
		}
		e.mu.Unlock()
	}
	if err := e.placementEngine.ForgetPolicy(ctx, repo, p, mode, tag, prune); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, sn := range kept {
		if !slices.ContainsFunc(e.snaps[loc], func(o restic.Snapshot) bool { return o.ID == sn.ID }) {
			e.snaps[loc] = append(e.snaps[loc], sn)
		}
	}
	return nil
}

func heldIDs(f *placementFixture, loc string) []string {
	f.eng.mu.Lock()
	defer f.eng.mu.Unlock()
	var ids []string
	for _, sn := range f.eng.snaps[filepath.ToSlash(loc)] {
		ids = append(ids, sn.ID)
	}
	slices.Sort(ids)
	return ids
}

func TestDirectSnapshotsOutliveTheLocalRuleUntilTheirTargetClaimsThem(t *testing.T) {
	f := newPlacementFixture(t)
	f.svc.engine = keepTagEngine{f.eng}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.RetentionKeepLast = 1
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	target := f.target("containers", "NAS", "backups/nas-containers")
	target.RetentionKeepLast = 2
	target, err = f.st.UpsertOffsiteTarget(target)
	if err != nil {
		t.Fatal(err)
	}
	old := f.namedRepo("NAS old", "backups/nas-containers-direct")
	f.container("web", old.ID)
	loc, err := f.svc.resolveRepo(old.Repo)
	if err != nil {
		t.Fatal(err)
	}
	f.hold(loc,
		snap("d1", 100, "container:web", restic.DirectTag),
		snap("d2", 200, "container:web", restic.DirectTag),
		snap("d3", 300, "container:web", restic.DirectTag),
		snap("n1", 400, "container:web"),
		snap("n2", 500, "container:web"),
	)
	ctx := context.Background()

	f.svc.applyRetention(ctx, loc, settings, restic.Mode{}, "container:web", "containers")
	if got := heldIDs(f, loc); !slices.Equal(got, []string{"d1", "d2", "d3", "n2"}) {
		t.Fatalf("the local rule on a repository that lost its link left %v, want every direct snapshot and the newest", got)
	}

	if err := f.st.ConnectCompanion(old.ID, target.ID); err != nil {
		t.Fatal(err)
	}
	f.svc.applyRetention(ctx, loc, settings, restic.Mode{}, "container:web", "containers")
	if got := heldIDs(f, loc); !slices.Equal(got, []string{"d3", "n2"}) {
		t.Fatalf("the target's rules left %v, want the two newest", got)
	}
}

type backupTagsEngine struct {
	ResticEngine
	tags [][]string
}

func (e *backupTagsEngine) Backup(_ context.Context, _ string, _, tags []string, _ restic.Mode, _ ...string) (restic.Summary, error) {
	e.tags = append(e.tags, tags)
	return restic.Summary{}, nil
}

func (e *backupTagsEngine) BackupStdin(_ context.Context, _ string, _ io.Reader, _ string, tags []string, _ restic.Mode) (restic.Summary, error) {
	e.tags = append(e.tags, tags)
	return restic.Summary{}, nil
}

func TestBackupsIntoADirectRepositoryCarryTheDirectTag(t *testing.T) {
	f := newPlacementFixture(t)
	d := f.direct(f.target("containers", "B2", "b2:bkt:containers"))
	plain := f.namedRepo("NAS", "backups/nas")
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	plainLoc, err := f.svc.resolveRepo(plain.Repo)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.svc.directTags(settings, "containers", d.Repo); !slices.Equal(got, []string{restic.DirectTag}) {
		t.Fatalf("direct repository: %v", got)
	}
	for _, loc := range []string{plainLoc, f.domainPath("containers")} {
		if got := f.svc.directTags(settings, "containers", loc); got != nil {
			t.Fatalf("%s: %v", loc, got)
		}
	}

	eng := &backupTagsEngine{}
	tags := make([]string, 2, 8)
	tags[0], tags[1] = "container:web", "p1"
	a := &resticAdapter{engine: eng, extraTags: []string{restic.DirectTag}}
	if _, err := a.Backup(context.Background(), "/repo", nil, tags); err != nil {
		t.Fatal(err)
	}
	z := &resticZvolAdapter{engine: eng, extraTags: []string{restic.DirectTag}}
	if _, err := z.BackupStdin(context.Background(), "/repo", nil, "/disk", tags); err != nil {
		t.Fatal(err)
	}
	for _, got := range eng.tags {
		if !slices.Equal(got, []string{"container:web", "p1", restic.DirectTag}) {
			t.Fatalf("tags = %v", got)
		}
	}
	if spare := tags[:3]; spare[2] != "" { //nolint:gosec // G602: 3 is within tags' cap of 8, not out of bounds
		t.Fatalf("the caller's slice was written past its length: %v", spare)
	}
}

func TestEveryBackupEntryPointAsksForTheDirectTag(t *testing.T) {
	src := mustReadService(t)
	for _, fn := range []string{"Backup", "BackupVM", "BackupFileSet"} {
		if !strings.Contains(funcBody(t, src, fn), "s.directTags(") {
			t.Errorf("%s writes into a direct repository without the %s tag, so its snapshots age by the local rule once the link is lost", fn, restic.DirectTag)
		}
	}
}

func TestDeletingATargetWhoseDirectRepositoryIsUsedIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	target := f.target("containers", "B2", "b2:bkt:containers")
	d := f.direct(target)
	f.container("web", d.ID)
	res := f.do("DELETE", "/api/offsite/targets/"+target.ID, nil)
	if res["ok"] != false || res["code"] != "target-in-use" {
		t.Fatalf("delete while in use = %v", res)
	}
	use := res["use"].(map[string]any)
	if use["directRepoId"] != d.ID || use["items"] != float64(1) || len(use["defaultDomains"].([]any)) != 0 {
		t.Fatalf("use = %v", use)
	}
	item := store.ItemRef{Domain: "containers", Key: "web"}
	if _, err := f.st.WritePlacement(item, &store.HomeWrite{Repo: "", Choice: store.RepoChosen}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if res := f.do("DELETE", "/api/offsite/targets/"+target.ID, nil); res["ok"] != true {
		t.Fatalf("delete once unused = %v", res)
	}
	if _, err := f.st.GetNamedRepo(d.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("the direct repository outlived its target: %v", err)
	}
}

func TestClearingTheOffsiteFieldKeepsItsDirectRepositoryLinked(t *testing.T) {
	f := newPlacementFixture(t)
	field := f.fieldTarget("containers", "b2:bkt:containers")
	d := f.direct(field)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersOffsite = ""
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.syncPrimaryOffsiteTarget("containers", settings); err != nil {
		t.Fatal(err)
	}
	got, found, err := f.st.CompanionFor(field.ID)
	if err != nil || !found || got.ID != d.ID || got.CompanionLost {
		t.Fatalf("after clearing the field: %+v, found %v, err %v", got, found, err)
	}
}

func TestAnImportKeepsTheDirectRepositoryOfATargetItCarries(t *testing.T) {
	f := newPlacementFixture(t)
	kept := f.target("containers", "B2", "b2:bkt:containers")
	gone := f.target("vms", "Hetzner", "sftp:u@box:/vms")
	dk := f.direct(kept)
	dg := f.direct(gone)
	if err := f.h.replaceOffsiteTargets([]offsiteTargetView{offsiteTargetToView(kept)}, settingsView{}); err != nil {
		t.Fatal(err)
	}
	k, err := f.st.GetNamedRepo(dk.ID)
	if err != nil || k.CompanionOf != kept.ID || k.CompanionLost {
		t.Fatalf("direct repository of a target in the file: %+v, %v", k, err)
	}
	g, err := f.st.GetNamedRepo(dg.ID)
	if err != nil || g.CompanionOf != "" || !g.CompanionLost {
		t.Fatalf("direct repository of a target missing from the file: %+v, %v", g, err)
	}
}

func TestAnImportKeepsWhatAKeptTargetHasObserved(t *testing.T) {
	f := newPlacementFixture(t)
	kept := f.target("containers", "B2", "b2:bkt:containers")
	gone := f.target("containers", "Hetzner", "sftp:u@box:/containers")
	f.listing("containers", kept.ID, 100, copiesRow("container:web", 3, 90))
	f.listing("containers", gone.ID, 100, copiesRow("container:web", 2, 90))
	if err := f.h.replaceOffsiteTargets([]offsiteTargetView{offsiteTargetToView(kept)}, settingsView{}); err != nil {
		t.Fatal(err)
	}
	copies, err := f.st.ItemCopiesForDomain("containers")
	if err != nil {
		t.Fatal(err)
	}
	if len(copies) != 1 || copies[0].TargetID != kept.ID || copies[0].SnapshotCount != 3 {
		t.Fatalf("offsite_item_copies after the import = %+v, want the kept target's row only", copies)
	}
	if _, listed, err := f.st.TargetObservationFor("containers", kept.ID); err != nil || !listed {
		t.Fatalf("the kept target lost its listing: listed %v, err %v", listed, err)
	}
	if _, listed, err := f.st.TargetObservationFor("containers", gone.ID); err != nil || listed {
		t.Fatalf("the dropped target kept its listing: listed %v, err %v", listed, err)
	}
}

func TestAnImportWithoutTargetsLeavesTheDirectRepositoryAsAPlainOne(t *testing.T) {
	f := newPlacementFixture(t)
	target := f.target("containers", "B2", "b2:bkt:containers")
	d := f.direct(target)
	file := f.do("GET", "/api/settings/export", nil)
	delete(file, "offsiteTargets")
	if res := f.do("POST", "/api/settings/import?apply=true", file); res["ok"] != true {
		t.Fatalf("import = %v", res)
	}
	got, err := f.st.GetNamedRepo(d.ID)
	if err != nil || got.CompanionOf != "" || !got.CompanionLost || !got.MirroredEqual(target) {
		t.Fatalf("direct repository after a file without targets: %+v, %v", got, err)
	}
	if _, found, _ := f.st.GetOffsiteTarget(target.ID); found {
		t.Fatal("a target the file does not carry is still there")
	}
}

func TestTheExportCarriesTheLinkOfADirectRepository(t *testing.T) {
	f := newPlacementFixture(t)
	target := f.target("containers", "B2", "b2:bkt:containers")
	d := f.direct(target)
	f.namedRepo("NAS", "backups/nas")
	res := f.do("GET", "/api/settings/export", nil)
	links := map[string]any{}
	for _, r := range res["namedRepos"].([]any) {
		row := r.(map[string]any)
		links[row["id"].(string)] = row["companionOf"]
	}
	if len(links) != 2 || links[d.ID] != target.ID {
		t.Fatalf("namedRepos links = %v", links)
	}
	for id, link := range links {
		if id != d.ID && link != nil {
			t.Fatalf("a plain repository carries a link: %v", link)
		}
	}
}

func TestAnImportedDirectRepositoryIsLinkedOrLabelledLost(t *testing.T) {
	f := newPlacementFixture(t)
	target := f.target("containers", "B2", "b2:bkt:containers")
	missing := "0123456789abcdef0123456789abcdef"
	views := []offsiteTargetView{
		{ID: "11111111111111111111111111111111", Name: "B2 direct", Repo: "b2:bkt:containers-direct", Enabled: true, CompanionOf: target.ID},
		{ID: "22222222222222222222222222222222", Name: "Gone direct", Repo: "b2:bkt:gone-direct", Enabled: true, CompanionOf: missing},
		{ID: "33333333333333333333333333333333", Name: "NAS", Repo: "backups/nas", Enabled: true},
	}
	if err := f.h.replaceNamedRepos(views); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]struct {
		link string
		lost bool
	}{
		views[0].ID: {target.ID, false},
		views[1].ID: {"", true},
		views[2].ID: {"", false},
	} {
		got, err := f.st.GetNamedRepo(id)
		if err != nil || got.CompanionOf != want.link || got.CompanionLost != want.lost {
			t.Errorf("%s = %+v, %v; want link %q lost %v", id, got, err, want.link, want.lost)
		}
	}
}

// TestAnOlderFileLeavesTheLinkOfAnExistingDirectRepository pins that an import
// never reads companionOf for a repository already stored here: the field is
// only ever consulted for a row the file introduces.
func TestAnOlderFileLeavesTheLinkOfAnExistingDirectRepository(t *testing.T) {
	f := newPlacementFixture(t)
	target := f.target("containers", "B2", "b2:bkt:containers")
	d := f.direct(target)
	file := f.do("GET", "/api/settings/export", nil)
	for _, row := range file["namedRepos"].([]any) {
		delete(row.(map[string]any), "companionOf")
	}
	if res := f.do("POST", "/api/settings/import?apply=true", file); res["ok"] != true {
		t.Fatalf("import = %v", res)
	}
	got, err := f.st.GetNamedRepo(d.ID)
	if err != nil || got.CompanionOf != target.ID || got.CompanionLost {
		t.Fatalf("direct repository after a file without companionOf: %+v, %v", got, err)
	}
}

func TestRetentionLowered(t *testing.T) {
	p := func(last, daily int) restic.RetentionPolicy { return restic.RetentionPolicy{KeepLast: last, KeepDaily: daily} }
	cases := []struct {
		name          string
		before, after restic.RetentionPolicy
		want          bool
	}{
		{"everything to a count", p(0, 0), p(5, 0), true},
		{"a count to everything", p(5, 0), p(0, 0), false},
		{"a smaller count", p(7, 0), p(3, 0), true},
		{"a larger count", p(7, 0), p(9, 0), false},
		{"a second dimension added", p(7, 0), p(7, 3), false},
		{"a dimension dropped", p(7, 3), p(7, 0), true},
		{"unchanged", p(7, 3), p(7, 3), false},
	}
	for _, c := range cases {
		if got := retentionLowered(c.before, c.after); got != c.want {
			t.Errorf("%s: retentionLowered = %v, want %v", c.name, got, c.want)
		}
	}
}

func warningCodes(t *testing.T, res map[string]any) []string {
	t.Helper()
	list, ok := res["warnings"].([]any)
	if !ok {
		t.Fatalf("no warnings list in %v", res)
	}
	codes := make([]string, 0, len(list))
	for _, w := range list {
		codes = append(codes, w.(map[string]any)["code"].(string))
	}
	return codes
}

func TestSavingATargetWarnsAboutItsUsedDirectRepository(t *testing.T) {
	f := newPlacementFixture(t)
	target := f.target("containers", "B2", "b2:bkt:containers")
	target.RetentionKeepLast, target.Immutable = 7, true
	target, err := f.st.UpsertOffsiteTarget(target)
	if err != nil {
		t.Fatal(err)
	}
	f.container("web", f.direct(target).ID)
	v := offsiteTargetToView(target)
	v.RetentionKeepLast, v.Immutable = 3, false
	res := f.do("PUT", "/api/offsite/targets/"+target.ID, v)
	if codes := warningCodes(t, res); !slices.Equal(codes, []string{"direct-retention-lowered", "direct-append-only-off"}) {
		t.Fatalf("warnings = %v", codes)
	}
	w := res["warnings"].([]any)[0].(map[string]any)
	if w["targetId"] != target.ID || w["targetName"] != "B2" || w["items"] != float64(1) {
		t.Fatalf("warning = %v", w)
	}
	v.RetentionKeepLast = 9
	if codes := warningCodes(t, f.do("PUT", "/api/offsite/targets/"+target.ID, v)); len(codes) != 0 {
		t.Fatalf("keeping more warned: %v", codes)
	}
}

func TestNewCredentialsReachADirectRepositoryOnlyWhenTheyOpenIt(t *testing.T) {
	f := newPlacementFixture(t)
	target := f.target("containers", "B2", "b2:bkt:containers")
	d := f.direct(target)
	v := offsiteTargetToView(target)
	v.CredsRef = "set-2"
	f.eng.opens[d.Repo] = false
	if codes := warningCodes(t, f.do("PUT", "/api/offsite/targets/"+target.ID, v)); !slices.Equal(codes, []string{"direct-creds-kept"}) {
		t.Fatalf("warnings = %v", codes)
	}
	if got, err := f.st.GetNamedRepo(d.ID); err != nil || got.CredsRef != "" {
		t.Fatalf("credentials that do not open it were mirrored: %+v, %v", got, err)
	}
	f.eng.opens[d.Repo] = true
	if codes := warningCodes(t, f.do("PUT", "/api/offsite/targets/"+target.ID, v)); len(codes) != 0 {
		t.Fatalf("warnings = %v", codes)
	}
	if got, err := f.st.GetNamedRepo(d.ID); err != nil || got.CredsRef != "set-2" {
		t.Fatalf("credentials that open it were not mirrored: %+v, %v", got, err)
	}
}

func TestASettingsSaveWarnsWhenTheFieldTargetsDirectRepositoryKeepsLess(t *testing.T) {
	f := newPlacementFixture(t)
	field := f.fieldTarget("containers", "b2:bkt:containers")
	f.container("web", f.direct(field).ID)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	v := toView(settings)
	v.OffsiteRetentionKeepLast = 5
	res := f.do("PUT", "/api/settings", v)
	if res["ok"] != true {
		t.Fatalf("save = %v", res)
	}
	if codes := warningCodes(t, res); !slices.Equal(codes, []string{"direct-retention-lowered"}) {
		t.Fatalf("warnings = %v", codes)
	}
	if _, ok := res["notes"].([]any); !ok {
		t.Fatalf("no notes list in %v", res)
	}
}

func TestSharedCloudCredentialsAreProbedOnDirectRepositories(t *testing.T) {
	f := newPlacementFixture(t)
	d := f.direct(f.target("containers", "B2", "b2:bkt:containers"))
	f.eng.opens[d.Repo] = false
	res := f.do("POST", "/api/cloud", map[string]any{
		"s3KeyId": "k", "s3Secret": "s", "s3Region": "", "restUser": "", "restPassword": "", "s3StorageClass": "",
	})
	if codes := warningCodes(t, res); !slices.Equal(codes, []string{"direct-creds-kept"}) {
		t.Fatalf("warnings = %v", codes)
	}
	if codes := warningCodes(t, f.do("POST", "/api/cloud/creds-sets", map[string]any{"sets": []any{}})); len(codes) != 0 {
		t.Fatalf("a direct repository on the shared credentials was probed for a set change: %v", codes)
	}
}
