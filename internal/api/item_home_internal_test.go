package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestPlacementCodesOfTheDefaultRefusals(t *testing.T) {
	for err, want := range map[error]string{
		errPlacementBusy:      "domain-busy",
		errHomeHasBackups:     "has-backups",
		errPlacementStale:     "stale",
		errRepoInvalid:        "repo-invalid",
		errRepoInUse:          "repo-in-use",
		errDefaultRepoMissing: "default-repo-missing",
	} {
		if got := placementCode(fmt.Errorf("wrapped: %w", err)); got != want {
			t.Errorf("placementCode(%v) = %q, want %q", err, got, want)
		}
	}
}

func TestHomeOnAnOpenItemIsWrittenChosen(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.openContainer("nginx")
	res := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{"home": map[string]any{"repo": nas.ID}})
	if res["ok"] != true {
		t.Fatalf("PATCH = %v", res)
	}
	if got := f.home(store.ItemRef{Domain: "containers", Key: "nginx"}); got.Repo != nas.ID || got.Choice != store.RepoChosen {
		t.Fatalf("home = %+v, want NAS chosen", got)
	}
}

func TestHomeCanBeChosenBeforeTheItemHasARow(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	res := f.do(http.MethodPatch, "/api/vms/win11", map[string]any{"home": map[string]any{"repo": nas.ID}})
	if res["ok"] != true {
		t.Fatalf("PATCH = %v", res)
	}
	if got := f.home(store.ItemRef{Domain: "vms", Key: "win11"}); got != (store.HomeState{Exists: true, Repo: nas.ID, Choice: store.RepoChosen}) {
		t.Fatalf("home = %+v, want a new row on NAS", got)
	}
}

func TestChoosingTheCurrentLocationStillMarksItChosen(t *testing.T) {
	f := newPlacementFixture(t)
	f.openContainer("nginx")
	res := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{"home": map[string]any{"repo": ""}})
	if res["ok"] != true {
		t.Fatalf("PATCH = %v", res)
	}
	if got := f.home(store.ItemRef{Domain: "containers", Key: "nginx"}); got.Repo != "" || got.Choice != store.RepoChosen {
		t.Fatalf("home = %+v, want the domain path chosen", got)
	}
}

func TestHomeChangeIsRefusedOnceAnItemHasBackups(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	web := f.container("web", "")
	f.backupRun(web.ID, 100)
	win := f.vm("win11", "")
	f.backupRun(win.ID, 100)
	docs := f.fileSet("docs", "")
	f.backupRun(docs.ID, 100)
	for _, tc := range []struct {
		path string
		item store.ItemRef
	}{
		{"/api/containers/web", store.ItemRef{Domain: "containers", Key: "web"}},
		{"/api/vms/win11", store.ItemRef{Domain: "vms", Key: "win11"}},
		{"/api/files/sets/" + docs.ID, store.ItemRef{Domain: "files", Key: docs.ID}},
	} {
		for _, body := range []map[string]any{
			{"home": map[string]any{"repo": nas.ID}},
			{"home": map[string]any{"follow": true}},
		} {
			res := f.do(http.MethodPatch, tc.path, body)
			if res["code"] != "has-backups" {
				t.Errorf("%s %v = %v, want has-backups", tc.path, body, res)
			}
		}
		if got := f.home(tc.item); got.Repo != "" || got.Choice != store.RepoChosen {
			t.Errorf("%s moved to %+v", tc.path, got)
		}
	}
}

func TestHomeIsRefusedWhileABackupHoldsTheDomain(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.openContainer("nginx")
	unlock, ok := f.svc.tryLockDomainFor("containers", "backup")
	if !ok {
		t.Fatal("could not take the containers lock")
	}
	choose := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{"home": map[string]any{"repo": nas.ID}})
	reset := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{"home": map[string]any{"follow": true}, "copies": map[string]any{"follow": true}})
	unlock()
	if choose["code"] != "domain-busy" || reset["code"] != "domain-busy" {
		t.Fatalf("PATCH during a backup = %v and %v, want domain-busy", choose, reset)
	}
	if got := f.home(store.ItemRef{Domain: "containers", Key: "nginx"}); got.Choice != store.RepoOpen {
		t.Fatalf("home = %+v, want it untouched", got)
	}
}

func TestResetReturnsBothAxesToTheDefault(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.container("nginx", nas.ID)
	f.rule("containers", "container:nginx", store.SkipAll)
	res := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{"home": map[string]any{"follow": true}, "copies": map[string]any{"follow": true}})
	if res["ok"] != true {
		t.Fatalf("PATCH = %v", res)
	}
	if got := f.home(store.ItemRef{Domain: "containers", Key: "nginx"}); got.Repo != "" || got.Choice != store.RepoOpen {
		t.Fatalf("home = %+v, want open", got)
	}
	if _, found, err := f.st.CopyRuleFor("containers", "container:nginx"); err != nil || found {
		t.Fatalf("rule found = %v, %v, want it gone", found, err)
	}
}

func TestTheRepoFieldActsAsHome(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.openContainer("nginx")
	if res := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{"repo": nas.ID}); res["ok"] != true {
		t.Fatalf("PATCH repo = %v", res)
	}
	if got := f.home(store.ItemRef{Domain: "containers", Key: "nginx"}); got.Repo != nas.ID || got.Choice != store.RepoChosen {
		t.Fatalf("home = %+v, want NAS chosen", got)
	}
	both := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{"repo": nas.ID, "home": map[string]any{"repo": nas.ID}})
	if both["code"] != "invalid-placement" {
		t.Fatalf("repo and home together = %v, want invalid-placement", both)
	}
}

func TestASwitchedOffRepositoryIsRefusedAsHome(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	nas.Enabled = false
	if _, err := f.st.UpsertOffsiteTarget(nas); err != nil {
		t.Fatal(err)
	}
	f.openContainer("nginx")
	res := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{"home": map[string]any{"repo": nas.ID}})
	if res["code"] != "repo-invalid" {
		t.Fatalf("PATCH = %v, want repo-invalid", res)
	}
}

func TestCopiesAreCheckedAgainstTheNewHome(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.namedRepo("Storagebox", "sftp:u1@box.example:/bv")
	f.target("containers", "B2", "b2:bucket/containers")
	f.openContainer("nginx")
	res := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{"home": map[string]any{"repo": box.ID}, "copies": map[string]any{"skip": []string{}}})
	if res["code"] != "copies-not-allowed" {
		t.Fatalf("PATCH = %v, want copies-not-allowed", res)
	}
	if got := f.home(store.ItemRef{Domain: "containers", Key: "nginx"}); got.Choice != store.RepoOpen {
		t.Fatalf("home = %+v, want nothing written", got)
	}
}

// TestCopiesOnAnOpenItemAreCheckedAgainstItsEffectiveHome pins that the copy
// side judges an open item by where its next backup actually lands, not by
// the row's own (empty) repo field. nginx never had its home chosen, so a raw
// read of its row says "domain path", but the domain's default sends it to
// Storagebox, a remote repository that takes no copies.
func TestCopiesOnAnOpenItemAreCheckedAgainstItsEffectiveHome(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.namedRepo("Storagebox", "sftp:u1@box.example:/bv")
	f.target("containers", "B2", "b2:bucket/containers")
	f.setDefault("containers", box.ID)
	f.openContainer("nginx")
	res := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{"copies": map[string]any{"skip": []string{}}})
	if res["code"] != "copies-not-allowed" {
		t.Fatalf("PATCH = %v, want copies-not-allowed", res)
	}
}
