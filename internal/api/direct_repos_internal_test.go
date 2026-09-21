package api

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
		fmt.Errorf("x: %w", errNestedLocation): "nested-location",
		errMirroredField:                       "mirrored-field",
		store.ErrCompanionTaken:                "companion-taken",
		store.ErrNotOffsiteTarget:              "unknown-target",
	} {
		if got := placementCode(err); got != want {
			t.Errorf("placementCode(%v) = %q, want %q", err, got, want)
		}
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
