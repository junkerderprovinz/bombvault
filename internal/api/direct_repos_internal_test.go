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
		store.ErrDirectRepo:                                    "direct-repo",
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

// A repository inside a domain path is a state an import can install, so the
// nesting rule may not lock the row against the edits it is not about.
func TestAnEditThatKeepsTheLocationPassesTheNestingCheck(t *testing.T) {
	f := newPlacementFixture(t)
	cold := f.namedRepo("Cold", "backups/containers/cold")
	if res := f.do("PATCH", "/api/repos/"+cold.ID, map[string]any{"enabled": false}); res["ok"] != true {
		t.Errorf("switching off a repository inside a domain path: %v", res)
	}
	if res := f.do("PATCH", "/api/repos/"+cold.ID, map[string]any{"name": "Cold store"}); res["ok"] != true {
		t.Errorf("renaming a repository inside a domain path: %v", res)
	}
	res := f.do("PATCH", "/api/repos/"+cold.ID, map[string]any{"repo": "backups/vms/cold"})
	if msg, _ := res["error"].(string); res["ok"] != false || !strings.Contains(msg, "lies inside") {
		t.Errorf("moving it to another nested place: %v", res)
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

func TestATargetInsideOrAroundAnotherPlaceIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bkt:containers")
	for _, loc := range []string{"b2:bkt:containers/inner", "b2:bkt", "backups/containers/offsite", "backups"} {
		res := f.do("POST", "/api/offsite/targets", map[string]any{"domain": "vms", "name": "x", "repo": loc, "enabled": true})
		if res["ok"] != false || res["code"] != "nested-location" {
			t.Errorf("new target at %s: %v", loc, res)
		}
	}
	if res := f.do("POST", "/api/offsite/targets", map[string]any{"domain": "vms", "name": "beside", "repo": "b2:bkt:vms", "enabled": true}); res["ok"] != true {
		t.Fatalf("a target beside another was refused: %v", res["error"])
	}
	v := offsiteTargetToView(b2)
	v.Repo = "backups/vms/offsite"
	if res := f.do("PUT", "/api/offsite/targets/"+b2.ID, v); res["code"] != "nested-location" {
		t.Errorf("moving a target into a domain path: %v", res)
	}
	v.Repo = b2.Repo
	v.Name = "B2 renamed"
	if res := f.do("PUT", "/api/offsite/targets/"+b2.ID, v); res["ok"] != true {
		t.Errorf("an edit that keeps the location: %v", res)
	}
}

// A row whose location does not resolve is not checked, and a clash nobody
// finds is a second repository over one place, so the pass says so.
func TestALocationThatDoesNotResolveIsReportedByTheClashCheck(t *testing.T) {
	f := newPlacementFixture(t)
	if _, err := f.st.UpsertOffsiteTarget(store.OffsiteTarget{
		Role: store.RoleRepo, Name: "Broken", Repo: "../escape", Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	logs := captureLog(t)
	if res := f.do("POST", "/api/repos", map[string]any{"name": "Cold", "repo": "backups/cold"}); res["ok"] != true {
		t.Fatalf("a repository beside the unresolvable one: %v", res)
	}
	if !strings.Contains(logs.String(), "Broken") {
		t.Errorf("the row that could not be resolved was not reported: %s", logs.String())
	}
}

// Only a named repository owns its place alone. Domain paths, off-site fields
// and targets may name one place between them, and only nesting is refused.
func TestTwoDomainsMayNameOneOffsiteDestination(t *testing.T) {
	f := newPlacementFixture(t)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	v := toView(settings)
	v.ContainersOffsite = "backups/shared-offsite"
	v.VMsOffsite = "backups/shared-offsite"
	if res := f.do("PUT", "/api/settings", v); res["ok"] != true {
		t.Fatalf("two domains on one off-site destination: %v", res["error"])
	}
	f.target("containers", "B2", "b2:bkt:shared")
	res := f.do("POST", "/api/offsite/targets", map[string]any{
		"domain": "vms", "name": "B2 too", "repo": "b2:bkt:shared", "enabled": true,
	})
	if res["ok"] != true {
		t.Fatalf("two targets on one place: %v", res["error"])
	}
	res = f.do("POST", "/api/offsite/targets", map[string]any{
		"domain": "vms", "name": "inner", "repo": "backups/containers/offsite", "enabled": true,
	})
	if res["ok"] != false || res["code"] != "nested-location" {
		t.Errorf("a target inside another domain's path: %v", res)
	}
}

func TestASettingsSaveRefusesAPathInsideOrAroundAnotherPlace(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("vms", "NAS", "backups/nas-vms")
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(v *settingsView){
		"a domain path inside another":            func(v *settingsView) { v.VMsPath = "backups/containers/vms" },
		"a domain path around the others":         func(v *settingsView) { v.FilesPath = "backups" },
		"a domain path inside a target":           func(v *settingsView) { v.FlashPath = "backups/nas-vms/flash" },
		"an off-site field inside its own domain": func(v *settingsView) { v.ContainersOffsite = "backups/containers/offsite" },
		"an off-site field inside a target":       func(v *settingsView) { v.VMsOffsite = "backups/nas-vms/inner" },
	} {
		v := toView(settings)
		change(&v)
		res := f.do("PUT", "/api/settings", v)
		if msg, _ := res["error"].(string); res["ok"] != false || !strings.Contains(msg, "lies inside") {
			t.Errorf("%s: %v", name, res)
		}
	}
	v := toView(settings)
	v.VMsPath = "backups/containers"
	if res := f.do("PUT", "/api/settings", v); res["ok"] != true {
		t.Fatalf("two domains sharing one repository: %v", res["error"])
	}
}

func TestAnImportWithATargetInsideADomainPathIsNotRefused(t *testing.T) {
	f := newPlacementFixture(t)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	exp := settingsExport{
		Settings:       toView(settings),
		OffsiteTargets: []offsiteTargetView{{Domain: "vms", Name: "inner", Repo: "backups/containers/offsite", Enabled: true}},
	}
	if msg := f.h.rejectImportCollisions(exp); msg != "" {
		t.Fatalf("an older file with a target inside a domain path must still import: %s", msg)
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

func TestTestingADirectLocationReportsAccessDenied(t *testing.T) {
	for name, msg := range map[string]string{
		"s3/b2 access denied": "restic cat failed: Fatal: unable to open config file: Stat: Access Denied.",
		"rest-server 401":     "restic cat failed: Fatal: unable to open config file: unexpected HTTP response (401): 401 Unauthorized",
		"rest-server 403":     "restic cat failed: Fatal: unable to open config file: unexpected HTTP response (403): 403 Forbidden",
	} {
		t.Run(name, func(t *testing.T) {
			f := newPlacementFixture(t)
			target := f.target("vms", "B2", "b2:bkt:vms")
			loc := "b2:bkt:vms-direct"
			f.eng.opens[loc] = false
			f.eng.openErr[loc] = errors.New(msg)
			res := f.do("POST", "/api/offsite/targets/"+target.ID+"/direct/test", map[string]any{"location": loc})
			if res["ok"] != false || res["code"] != "direct-access-denied" {
				t.Fatalf("%s = %v", name, res)
			}
		})
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

func TestConnectingARepositoryChecksTheTargetAndWhatUsesIt(t *testing.T) {
	f := newPlacementFixture(t)
	flash := f.target("flash", "Flash B2", "b2:bkt:flash")
	repo := f.namedRepo("NAS", "backups/nas")
	if res := f.do("POST", "/api/repos/"+repo.ID+"/connect", map[string]any{"targetId": flash.ID}); res["code"] != "unknown-target" {
		t.Errorf("connecting to a flash target = %v", res)
	}
	target := f.target("containers", "B2", "b2:bkt:containers")
	f.container("web", repo.ID)
	res := f.do("POST", "/api/repos/"+repo.ID+"/connect", map[string]any{"targetId": target.ID})
	if res["code"] != "repo-in-use" || res["items"] != float64(1) {
		t.Errorf("connecting a repository an item uses = %v", res)
	}
	if _, found, _ := f.st.CompanionFor(target.ID); found {
		t.Fatal("a repository in use was connected")
	}
	free := f.namedRepo("Cold", "backups/cold")
	if res := f.do("POST", "/api/repos/"+free.ID+"/connect", map[string]any{"targetId": target.ID}); res["ok"] != true {
		t.Fatalf("connecting a free repository = %v", res)
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

	f.svc.applyRetention(ctx, loc, settings, restic.Mode{}, tagIdentity("container:web"), "containers")
	want := restic.RetentionPolicy{KeepLast: 3, KeepDaily: 2, Direct: true}
	if got := forgetsAt(f, loc); len(got) != 1 || got[0].Policy != want {
		t.Fatalf("after a backup the direct repository forgot with %+v, want %+v", got, want)
	}
	f.svc.applyRetention(ctx, f.domainPath("containers"), settings, restic.Mode{}, tagIdentity("container:db"), "containers")
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
	f.svc.applyRetention(ctx, vLoc, settings, restic.Mode{}, tagIdentity("vm:win11"), "vms")
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

func (e keepTagEngine) ForgetPolicy(ctx context.Context, repo string, p restic.RetentionPolicy, mode restic.Mode, tags []string, prune bool) error {
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
	if err := e.placementEngine.ForgetPolicy(ctx, repo, p, mode, tags, prune); err != nil {
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

	f.svc.applyRetention(ctx, loc, settings, restic.Mode{}, tagIdentity("container:web"), "containers")
	if got := heldIDs(f, loc); !slices.Equal(got, []string{"d1", "d2", "d3", "n2"}) {
		t.Fatalf("the local rule on a repository that lost its link left %v, want every direct snapshot and the newest", got)
	}

	if err := f.st.ConnectCompanion(old.ID, target.ID); err != nil {
		t.Fatal(err)
	}
	f.svc.applyRetention(ctx, loc, settings, restic.Mode{}, tagIdentity("container:web"), "containers")
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

func TestDeletingADirectRepositoryOnTheRepositoriesCardIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	target := f.target("containers", "B2", "b2:bkt:containers")
	d := f.direct(target)
	res := f.do("DELETE", "/api/repos/"+d.ID, nil)
	if res["ok"] != false || res["code"] != "direct-repo" {
		t.Fatalf("deleting a direct repository = %v", res)
	}
	named, _ := res["target"].(map[string]any)
	if named["id"] != target.ID || named["name"] != placementTargetName(target) {
		t.Errorf("the refusal does not name the target: %v", res["target"])
	}
	if _, err := f.st.GetNamedRepo(d.ID); err != nil {
		t.Fatalf("a refused delete must leave the row: %v", err)
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

func TestAnImportLeavesADirectRepositoryWhereItsTargetWrites(t *testing.T) {
	f := newPlacementFixture(t)
	target := f.target("containers", "B2", "b2:bkt:containers")
	d := f.direct(target)
	logs := captureLog(t)
	view := offsiteTargetToView(d)
	view.Repo = "b2:bkt:somewhere-else"
	view.Name = "B2 direct renamed"
	view.Enabled = false
	if err := f.h.replaceNamedRepos([]offsiteTargetView{view}); err != nil {
		t.Fatal(err)
	}
	got, err := f.st.GetNamedRepo(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Repo != d.Repo {
		t.Errorf("location = %q, want the one its target writes to, %q", got.Repo, d.Repo)
	}
	if got.Name != "B2 direct renamed" || got.Enabled {
		t.Errorf("the fields the file may change were not applied: %+v", got)
	}
	if !strings.Contains(logs.String(), "B2 direct") {
		t.Errorf("the refused move was not logged: %s", logs.String())
	}
}

func TestDiscoverNamesAPlainRepositoryHoldingDirectSnapshots(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bkt:containers")
	other := f.target("containers", "Hetzner", "sftp:u@box:/containers")
	old := f.namedRepo("B2 old", "b2:bkt:containers-direct")
	linked := f.direct(f.target("vms", "B2 vms", "b2:bkt:vms"))
	f.hold("b2:bkt:containers-direct", snap("a1", 100, "container:web", restic.DirectTag))
	f.hold(linked.Repo, snap("b1", 100, "vm:win11", restic.DirectTag))
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, rows, err := f.svc.discoverNamesAcrossRepos(context.Background(), settings, "containers", "container:")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != old.ID {
		t.Fatalf("rows = %+v", rows)
	}
	findings, err := f.svc.directFindings("containers", rows)
	if err != nil || len(findings) != 1 {
		t.Fatalf("findings = %+v, %v", findings, err)
	}
	got := findings[0]
	if got.RepoID != old.ID || len(got.Candidates) != 2 || got.Candidates[0].ID != b2.ID || got.Candidates[1].ID != other.ID {
		t.Fatalf("finding = %+v", got)
	}
}

func TestTheDiscoverAnswerNamesDirectRepositories(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bkt:containers")
	old := f.namedRepo("B2 old", "b2:bkt:containers-direct")
	f.hold("b2:bkt:containers-direct", snap("a1", 100, "container:web", restic.DirectTag))
	res := f.do("POST", "/api/discover?probe=true", nil)
	list, ok := res["directRepos"].([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("directRepos = %v", res["directRepos"])
	}
	got := list[0].(map[string]any)
	targets := got["targets"].([]any)
	if got["repoId"] != old.ID || got["name"] != "B2 old" || targets[0].(map[string]any)["id"] != b2.ID || targets[0].(map[string]any)["name"] != "B2" {
		t.Fatalf("finding = %v", got)
	}
}

func TestRetentionLowered(t *testing.T) {
	p := func(last, daily int) restic.RetentionPolicy {
		return restic.RetentionPolicy{KeepLast: last, KeepDaily: daily}
	}
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
	f.container("web", d.ID)
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

	empty := f.target("vms", "B2 vms", "b2:bkt:vms")
	ed := f.direct(empty)
	ev := offsiteTargetToView(empty)
	ev.CredsRef = "set-3"
	f.eng.opens[ed.Repo] = false
	if codes := warningCodes(t, f.do("PUT", "/api/offsite/targets/"+empty.ID, ev)); len(codes) != 0 {
		t.Fatalf("a direct repository nothing uses warned about kept credentials: %v", codes)
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
	if err := f.svc.SetCloudCreds(CloudCreds{RESTUser: "bv", RESTPassword: "old"}); err != nil {
		t.Fatal(err)
	}
	d := f.direct(f.target("containers", "NAS", "rest:http://nas:8000/bv/containers"))
	f.container("web", d.ID)
	empty := f.direct(f.target("vms", "NAS vms", "rest:http://nas:8000/bv/vms"))
	f.eng.opens[d.Repo] = false
	f.eng.opens[empty.Repo] = false
	if codes := warningCodes(t, f.do("POST", "/api/cloud/creds-sets", map[string]any{"sets": []any{}})); len(codes) != 0 {
		t.Fatalf("a direct repository on the shared credentials was probed for a set change: %v", codes)
	}
	f.opensOnlyWith(d.Repo, "old")
	res := f.do("POST", "/api/cloud", map[string]any{"restUser": "bv", "restPassword": "new"})
	// empty is probed too (same shared creds) but nothing uses it, so it stays quiet.
	if codes := warningCodes(t, res); !slices.Equal(codes, []string{"direct-creds-kept"}) {
		t.Fatalf("warnings = %v", codes)
	}
}

// restDirect is a rest-server target on the credential set rest-creds, whose
// password is old, and its direct repository, which a container uses.
func (f *placementFixture) restDirect() (store.OffsiteTarget, store.OffsiteTarget) {
	f.t.Helper()
	if err := f.svc.SetCloudCredSets([]CloudCredSet{{ID: "rest-creds", Name: "Rest server", CloudCreds: CloudCreds{RESTUser: "bv", RESTPassword: "old"}}}); err != nil {
		f.t.Fatal(err)
	}
	target := f.target("containers", "NAS", "rest:http://nas:8000/bv/containers")
	target.CredsRef = "rest-creds"
	target, err := f.st.UpsertOffsiteTarget(target)
	if err != nil {
		f.t.Fatal(err)
	}
	d := f.direct(target)
	f.container("web", d.ID)
	return target, d
}

// runsWith is the environment a backup into a containers direct repository gets,
// built the way the backup builds it.
func (f *placementFixture) runsWith(direct store.OffsiteTarget) []string {
	f.t.Helper()
	settings, err := f.st.GetSettings()
	if err != nil {
		f.t.Fatal(err)
	}
	loc, err := f.svc.resolveRepo(direct.Repo)
	if err != nil {
		f.t.Fatal(err)
	}
	return f.svc.primaryModeFor(settings, "containers", loc).Env
}

func (f *placementFixture) storedCredSets() []CloudCredSet {
	f.t.Helper()
	settings, err := f.st.GetSettings()
	if err != nil {
		f.t.Fatal(err)
	}
	sets, err := f.svc.decodeCloudCredSets(settings)
	if err != nil {
		f.t.Fatal(err)
	}
	return sets
}

func (f *placementFixture) keptFor(directID string) []CloudCredSet {
	f.t.Helper()
	return slices.DeleteFunc(f.storedCredSets(), func(c CloudCredSet) bool { return c.KeptFor != directID })
}

// credSetDrafts is the list the Settings page posts back: every set as it read
// it, secrets blank, and without fields it does not know.
func (f *placementFixture) credSetDrafts() []map[string]any {
	f.t.Helper()
	listed, ok := f.do("GET", "/api/cloud/creds-sets", nil)["sets"].([]any)
	if !ok {
		f.t.Fatal("no sets list")
	}
	out := make([]map[string]any, 0, len(listed))
	for _, l := range listed {
		s := l.(map[string]any)
		out = append(out, map[string]any{
			"id": s["id"], "name": s["name"], "s3KeyId": s["s3KeyId"], "s3Secret": "", "s3Region": s["s3Region"],
			"restUser": s["restUser"], "restPassword": "", "s3StorageClass": s["s3StorageClass"],
		})
	}
	return out
}

func withPassword(drafts []map[string]any, id, password string) []map[string]any {
	for _, d := range drafts {
		if d["id"] == id {
			d["restPassword"] = password
		}
	}
	return drafts
}

// passwordEngine opens a location only with the REST password it holds for it.
type passwordEngine struct {
	*placementEngine
	passwords map[string]string
}

func (e *passwordEngine) RepoOpens(_ context.Context, repo string, mode restic.Mode) bool {
	return slices.Contains(mode.Env, "RESTIC_REST_PASSWORD="+e.passwords[repo])
}

// opensOnlyWith lets location open with password and nothing else.
func (f *placementFixture) opensOnlyWith(location, password string) *passwordEngine {
	eng := &passwordEngine{placementEngine: f.eng, passwords: map[string]string{location: password}}
	f.svc.engine = eng
	return eng
}

func TestADirectRepositoryKeepsTheSetValuesItOpenedWith(t *testing.T) {
	f := newPlacementFixture(t)
	target, d := f.restDirect()
	f.opensOnlyWith(d.Repo, "old")
	res := f.do("POST", "/api/cloud/creds-sets", map[string]any{"sets": withPassword(f.credSetDrafts(), "rest-creds", "wrong")})
	if codes := warningCodes(t, res); !slices.Equal(codes, []string{"direct-creds-kept"}) {
		t.Fatalf("warnings = %v", codes)
	}
	kept := f.keptFor(d.ID)
	if len(kept) != 1 || kept[0].RESTUser != "bv" || kept[0].RESTPassword != "old" || kept[0].Name != "NAS direct (kept credentials)" {
		t.Fatalf("kept sets = %+v, want one holding the old password", kept)
	}
	if got, err := f.st.GetNamedRepo(d.ID); err != nil || got.CredsRef != kept[0].ID {
		t.Fatalf("direct repository = %+v, %v; want it on the kept set %s", got, err, kept[0].ID)
	}
	if env := f.runsWith(d); !slices.Contains(env, "RESTIC_REST_PASSWORD=old") || !slices.Contains(env, "RESTIC_REST_USERNAME=bv") {
		t.Fatalf("a backup into the direct repository runs with %v, want the old password", env)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if env := f.svc.offsiteModeForTarget(settings, target).Env; !slices.Contains(env, "RESTIC_REST_PASSWORD=wrong") {
		t.Fatalf("the target runs with %v, want the saved password", env)
	}
}

func TestADirectRepositoryKeepsTheSharedValuesItOpenedWith(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCreds(CloudCreds{RESTUser: "bv", RESTPassword: "old"}); err != nil {
		t.Fatal(err)
	}
	d := f.direct(f.target("containers", "NAS", "rest:http://nas:8000/bv/containers"))
	f.container("web", d.ID)
	f.opensOnlyWith(d.Repo, "old")
	res := f.do("POST", "/api/cloud", map[string]any{"restUser": "bv", "restPassword": "wrong"})
	if codes := warningCodes(t, res); !slices.Equal(codes, []string{"direct-creds-kept"}) {
		t.Fatalf("warnings = %v", codes)
	}
	kept := f.keptFor(d.ID)
	if len(kept) != 1 || kept[0].RESTPassword != "old" {
		t.Fatalf("kept sets = %+v, want one holding the old password", kept)
	}
	if got, err := f.st.GetNamedRepo(d.ID); err != nil || got.CredsRef != kept[0].ID {
		t.Fatalf("direct repository = %+v, %v; want it on the kept set %s", got, err, kept[0].ID)
	}
	if env := f.runsWith(d); !slices.Contains(env, "RESTIC_REST_PASSWORD=old") {
		t.Fatalf("a backup into the direct repository runs with %v, want the old password", env)
	}
}

func TestCredentialsThatOpenADirectRepositoryAgainRetireItsKeptSet(t *testing.T) {
	f := newPlacementFixture(t)
	_, d := f.restDirect()
	spare := map[string]any{"id": "spare", "name": "Spare", "restUser": "u", "restPassword": "mine"}
	eng := f.opensOnlyWith(d.Repo, "old")
	f.do("POST", "/api/cloud/creds-sets", map[string]any{"sets": append(withPassword(f.credSetDrafts(), "rest-creds", "wrong"), spare)})
	if len(f.keptFor(d.ID)) != 1 {
		t.Fatalf("nothing kept: %+v", f.storedCredSets())
	}

	eng.passwords[d.Repo] = "right"
	res := f.do("POST", "/api/cloud/creds-sets", map[string]any{"sets": withPassword(f.credSetDrafts(), "rest-creds", "right")})
	if codes := warningCodes(t, res); len(codes) != 0 {
		t.Fatalf("warnings = %v", codes)
	}
	if got, err := f.st.GetNamedRepo(d.ID); err != nil || got.CredsRef != "rest-creds" {
		t.Fatalf("direct repository = %+v, %v; want it back on rest-creds", got, err)
	}
	if kept := f.keptFor(d.ID); len(kept) != 0 {
		t.Fatalf("kept sets = %+v, want none", kept)
	}
	ids := []string{}
	for _, s := range f.storedCredSets() {
		ids = append(ids, s.ID)
		if s.ID == "spare" && s.RESTPassword != "mine" {
			t.Fatalf("the user's set changed: %+v", s)
		}
	}
	if !slices.Equal(ids, []string{"rest-creds", "spare"}) {
		t.Fatalf("sets = %v, want rest-creds and spare", ids)
	}
	if env := f.runsWith(d); !slices.Contains(env, "RESTIC_REST_PASSWORD=right") {
		t.Fatalf("a backup into the direct repository runs with %v, want the new password", env)
	}
}

func TestSharedCredentialsThatOpenADirectRepositoryAgainRetireItsKeptSet(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCreds(CloudCreds{RESTUser: "bv", RESTPassword: "old"}); err != nil {
		t.Fatal(err)
	}
	d := f.direct(f.target("containers", "NAS", "rest:http://nas:8000/bv/containers"))
	f.container("web", d.ID)
	eng := f.opensOnlyWith(d.Repo, "old")
	f.do("POST", "/api/cloud", map[string]any{"restUser": "bv", "restPassword": "wrong"})
	if len(f.keptFor(d.ID)) != 1 {
		t.Fatalf("nothing kept: %+v", f.storedCredSets())
	}

	eng.passwords[d.Repo] = "right"
	if codes := warningCodes(t, f.do("POST", "/api/cloud", map[string]any{"restUser": "bv", "restPassword": "right"})); len(codes) != 0 {
		t.Fatalf("warnings = %v", codes)
	}
	if got, err := f.st.GetNamedRepo(d.ID); err != nil || got.CredsRef != "" {
		t.Fatalf("direct repository = %+v, %v; want it back on the shared credentials", got, err)
	}
	if sets := f.storedCredSets(); len(sets) != 0 {
		t.Fatalf("sets = %+v, want none", sets)
	}
}

func TestAKeptSetSomethingElseNamesStays(t *testing.T) {
	f := newPlacementFixture(t)
	_, d := f.restDirect()
	eng := f.opensOnlyWith(d.Repo, "old")
	f.do("POST", "/api/cloud/creds-sets", map[string]any{"sets": withPassword(f.credSetDrafts(), "rest-creds", "wrong")})
	kept := f.keptFor(d.ID)
	if len(kept) != 1 {
		t.Fatalf("nothing kept: %+v", f.storedCredSets())
	}
	other := f.target("vms", "NAS vms", "rest:http://nas:8000/bv/vms")
	other.CredsRef = kept[0].ID
	if _, err := f.st.UpsertOffsiteTarget(other); err != nil {
		t.Fatal(err)
	}

	eng.passwords[d.Repo] = "right"
	f.do("POST", "/api/cloud/creds-sets", map[string]any{"sets": withPassword(f.credSetDrafts(), "rest-creds", "right")})
	if got, err := f.st.GetNamedRepo(d.ID); err != nil || got.CredsRef != "rest-creds" {
		t.Fatalf("direct repository = %+v, %v; want it back on rest-creds", got, err)
	}
	if got := f.keptFor(d.ID); len(got) != 1 || got[0].ID != kept[0].ID || got[0].RESTPassword != "old" {
		t.Fatalf("kept sets = %+v, want the one another target names", got)
	}
}

func TestPostingTheSetsBackKeepsWhatASetWasKeptFor(t *testing.T) {
	f := newPlacementFixture(t)
	_, d := f.restDirect()
	f.opensOnlyWith(d.Repo, "old")
	f.do("POST", "/api/cloud/creds-sets", map[string]any{"sets": withPassword(f.credSetDrafts(), "rest-creds", "wrong")})
	kept := f.keptFor(d.ID)
	if len(kept) != 1 {
		t.Fatalf("nothing kept: %+v", f.storedCredSets())
	}
	f.do("POST", "/api/cloud/creds-sets", map[string]any{"sets": f.credSetDrafts()})
	if got := f.keptFor(d.ID); !slices.Equal(got, kept) {
		t.Fatalf("kept sets = %+v, want %+v", got, kept)
	}
}

func TestCredentialsThatOpenADirectRepositoryKeepNothing(t *testing.T) {
	f := newPlacementFixture(t)
	_, d := f.restDirect()
	res := f.do("POST", "/api/cloud/creds-sets", map[string]any{"sets": withPassword(f.credSetDrafts(), "rest-creds", "rotated")})
	if codes := warningCodes(t, res); len(codes) != 0 {
		t.Fatalf("warnings = %v", codes)
	}
	if sets := f.storedCredSets(); len(sets) != 1 || sets[0].ID != "rest-creds" {
		t.Fatalf("sets = %+v, want rest-creds alone", sets)
	}
	if env := f.runsWith(d); !slices.Contains(env, "RESTIC_REST_PASSWORD=rotated") {
		t.Fatalf("a backup into the direct repository runs with %v, want the new password", env)
	}
}

// A key rotated while the server is down opens nothing, the old one included,
// so the repository follows its set to the new key.
func TestOldValuesThatDoNotOpenADirectRepositoryAreNotKept(t *testing.T) {
	f := newPlacementFixture(t)
	_, d := f.restDirect()
	f.eng.opens[d.Repo] = false
	res := f.do("POST", "/api/cloud/creds-sets", map[string]any{"sets": withPassword(f.credSetDrafts(), "rest-creds", "rotated")})
	if codes := warningCodes(t, res); len(codes) != 0 {
		t.Fatalf("warnings = %v, want none: nothing was kept", codes)
	}
	if kept := f.keptFor(d.ID); len(kept) != 0 {
		t.Fatalf("kept sets = %+v, want none", kept)
	}
	if got, err := f.st.GetNamedRepo(d.ID); err != nil || got.CredsRef != "rest-creds" {
		t.Fatalf("direct repository = %+v, %v; want it still on rest-creds", got, err)
	}
	if env := f.runsWith(d); !slices.Contains(env, "RESTIC_REST_PASSWORD=rotated") {
		t.Fatalf("a backup into the direct repository runs with %v, want the new password", env)
	}
}

// A direct repository that kept the shared credentials when its target moved to
// a set of its own is probed with its own new values, not only the target's.
func TestADirectRepositoryOnItsOwnSelectorIsProbedWithItsNewValues(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCreds(CloudCreds{RESTUser: "bv", RESTPassword: "old"}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.SetCloudCredSets([]CloudCredSet{{ID: "other", Name: "Other", CloudCreds: CloudCreds{RESTUser: "bv", RESTPassword: "other"}}}); err != nil {
		t.Fatal(err)
	}
	target := f.target("containers", "NAS", "rest:http://nas:8000/bv/containers")
	d := f.direct(target)
	f.container("web", d.ID)
	eng := f.opensOnlyWith(d.Repo, "old")
	v := offsiteTargetToView(target)
	v.CredsRef = "other"
	f.do("PUT", "/api/offsite/targets/"+target.ID, v)
	if got, err := f.st.GetNamedRepo(d.ID); err != nil || got.CredsRef != "" {
		t.Fatalf("direct repository = %+v, %v; want it still on the shared credentials", got, err)
	}

	eng.passwords[d.Repo] = "rotated"
	res := f.do("POST", "/api/cloud", map[string]any{"restUser": "bv", "restPassword": "rotated"})
	if codes := warningCodes(t, res); len(codes) != 0 {
		t.Fatalf("warnings = %v, want none: the new shared values open it", codes)
	}
	if sets := f.storedCredSets(); len(sets) != 1 {
		t.Fatalf("sets = %+v, want only Other: the new shared values open it", sets)
	}
	if env := f.runsWith(d); !slices.Contains(env, "RESTIC_REST_PASSWORD=rotated") {
		t.Fatalf("a backup into the direct repository runs with %v, want the new shared password", env)
	}

	res = f.do("POST", "/api/cloud", map[string]any{"restUser": "bv", "restPassword": "typo"})
	if codes := warningCodes(t, res); !slices.Equal(codes, []string{"direct-creds-kept"}) {
		t.Fatalf("warnings = %v", codes)
	}
	kept := f.keptFor(d.ID)
	if len(kept) != 1 || kept[0].RESTPassword != "rotated" {
		t.Fatalf("kept sets = %+v, want one holding rotated", kept)
	}
}
