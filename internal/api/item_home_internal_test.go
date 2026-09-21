package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

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

// TestPinningAnOpenItemToTheDomainPathIsAllowedWhenThatIsWhereItsBackupsAre
// pins that the has-backups guard judges "nothing moves" by the row's raw
// repo field, the same place itemBackups looks for an open item: pinning it
// to the domain path it is already open on moves nothing, so the guard lets
// it through despite the item's history.
func TestPinningAnOpenItemToTheDomainPathIsAllowedWhenThatIsWhereItsBackupsAre(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.setDefault("containers", nas.ID)
	f.openContainer("nginx")
	tg, err := f.st.GetTargetByContainer("nginx")
	if err != nil {
		t.Fatal(err)
	}
	f.backupRun(tg.ID, 100)
	res := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{"home": map[string]any{"repo": ""}})
	if res["ok"] != true {
		t.Fatalf("PATCH = %v, want it allowed since nothing moves", res)
	}
	if got := f.home(store.ItemRef{Domain: "containers", Key: "nginx"}); got.Repo != "" || got.Choice != store.RepoChosen {
		t.Fatalf("home = %+v, want the domain path chosen", got)
	}
}

// TestPinningAnOpenItemToTheDefaultIsRefusedWhenItsBackupsAreAtTheDomainPath
// is the mirror of the above: the default names a repository the item has
// never actually written to, so pinning it there strands the history that
// itemBackups finds at the domain path, and the guard must refuse it.
func TestPinningAnOpenItemToTheDefaultIsRefusedWhenItsBackupsAreAtTheDomainPath(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.setDefault("containers", nas.ID)
	f.openContainer("nginx")
	tg, err := f.st.GetTargetByContainer("nginx")
	if err != nil {
		t.Fatal(err)
	}
	f.backupRun(tg.ID, 100)
	res := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{"home": map[string]any{"repo": nas.ID}})
	if res["code"] != "has-backups" {
		t.Fatalf("PATCH = %v, want has-backups", res)
	}
	if got := f.home(store.ItemRef{Domain: "containers", Key: "nginx"}); got.Choice != store.RepoOpen {
		t.Fatalf("home = %+v, want it left open", got)
	}
}

// TestAForgottenAndReinstalledContainerKeepsItsHistoryOverTheDefault is the
// scenario the guard exists for: a container backed up before any default
// existed, was then removed and reinstalled (a fresh, open row), and only
// afterwards got a default pointing at a named repository it never wrote to.
// Its snapshots still sit at the domain path, found here by listing rather
// than a recorded run, and the guard must judge by that, not by the default.
func TestAForgottenAndReinstalledContainerKeepsItsHistoryOverTheDefault(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.hold(f.domainPath("containers"), snap("aaaa0001", 100, "container:nginx"))
	f.setDefault("containers", nas.ID)
	f.openContainer("nginx")

	toDefault := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{"home": map[string]any{"repo": nas.ID}})
	if toDefault["code"] != "has-backups" {
		t.Fatalf("pin to the default = %v, want has-backups", toDefault)
	}

	toHistory := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{"home": map[string]any{"repo": ""}})
	if toHistory["ok"] != true {
		t.Fatalf("pin to the domain path = %v, want it allowed", toHistory)
	}
	if got := f.home(store.ItemRef{Domain: "containers", Key: "nginx"}); got.Repo != "" || got.Choice != store.RepoChosen {
		t.Fatalf("home = %+v, want the domain path chosen", got)
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

// TestCopiesOnAnOpenItemAreCheckedAgainstItsEffectiveHome pins effectiveHome
// against nginx, which never had its home chosen: the domain's default sends
// it to Storagebox, a remote repository that takes no copies.
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

// TestHomeWriteIsRefusedOverARowChangedMeanwhile pins that writeItemPlacement's
// write compares against the same read checkHomeChange judged against: the
// hook here mutates the row from another write between that read and the
// write below it, exactly the window store.WritePlacement's expect argument
// closes. It calls writeItemPlacement directly, bypassing the handler's own
// preview check, so its one itemBackups listing is unambiguously the
// authoritative one the race belongs on.
func TestHomeWriteIsRefusedOverARowChangedMeanwhile(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	other := f.namedRepo("Backup2", "nas2")
	f.openContainer("nginx")
	item := store.ItemRef{Domain: "containers", Key: "nginx"}
	f.eng.onSnapshots = func() {
		if _, err := f.st.WritePlacement(item, &store.HomeWrite{Repo: other.ID, Choice: store.RepoChosen}, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	_, err := f.svc.writeItemPlacement(context.Background(), item, placementChange{Home: &homeChoice{Repo: &nas.ID}})
	if !errors.Is(err, errPlacementStale) {
		t.Fatalf("writeItemPlacement = %v, want errPlacementStale", err)
	}
	if got := f.home(item); got.Repo != other.ID {
		t.Fatalf("home = %+v, want the concurrent write left standing", got)
	}
}

// TestALaterFieldFailingLeavesHomeAndTheRuleUntouched pins that a PATCH writes
// placement no earlier than every other field in the same request: the
// schedule cadence here is invalid and refuses the request after home and
// copies would already have validated cleanly, so neither may land.
func TestALaterFieldFailingLeavesHomeAndTheRuleUntouched(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.openContainer("web")
	f.openVM("win11")
	docs := f.openFileSet("docs")
	for _, tc := range []struct {
		domain   string
		path     string
		item     store.ItemRef
		identity string
	}{
		{"containers", "/api/containers/web", store.ItemRef{Domain: "containers", Key: "web"}, "container:web"},
		{"vms", "/api/vms/win11", store.ItemRef{Domain: "vms", Key: "win11"}, "vm:win11"},
		{"files", "/api/files/sets/" + docs.ID, store.ItemRef{Domain: "files", Key: docs.ID}, "fileset:docs"},
	} {
		res := f.do(http.MethodPatch, tc.path, map[string]any{
			"home":            map[string]any{"repo": nas.ID},
			"copies":          map[string]any{"skip": []string{}},
			"scheduleCadence": "bogus",
		})
		if res["ok"] != false {
			t.Errorf("%s = %v, want the schedule refusal", tc.path, res)
		}
		if got := f.home(tc.item); got.Choice != store.RepoOpen {
			t.Errorf("%s moved home to %+v despite the later refusal", tc.path, got)
		}
		if _, found, err := f.st.CopyRuleFor(tc.domain, tc.identity); err != nil || found {
			t.Errorf("%s: rule found = %v, %v, want none", tc.path, found, err)
		}
	}
}

// TestAHomeRefusalLeavesOtherFieldsUntouched pins the other half of the PATCH
// ordering: home and copies are validated before anything else in the request
// is written, not after. An item that already has backups refuses the home
// change up front, so a field earlier in the same request body never lands.
func TestAHomeRefusalLeavesOtherFieldsUntouched(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	web := f.container("web", "")
	f.backupRun(web.ID, 100)
	win := f.vm("win11", "")
	f.backupRun(win.ID, 100)
	docs := f.fileSet("docs", "")
	f.backupRun(docs.ID, 100)

	res := f.do(http.MethodPatch, "/api/containers/web", map[string]any{
		"home": map[string]any{"repo": nas.ID}, "preHook": "echo hi",
	})
	if res["code"] != "has-backups" {
		t.Fatalf("containers PATCH = %v, want has-backups", res)
	}
	if tg, err := f.st.GetTargetByContainer("web"); err != nil || tg.PreHook != "" {
		t.Fatalf("PreHook = %q, %v, want untouched", tg.PreHook, err)
	}

	res = f.do(http.MethodPatch, "/api/vms/win11", map[string]any{
		"home": map[string]any{"repo": nas.ID}, "method": "acpi",
	})
	if res["code"] != "has-backups" {
		t.Fatalf("vms PATCH = %v, want has-backups", res)
	}
	if vm, err := f.st.GetVMTargetByName("win11"); err != nil || vm.Method != "graceful" {
		t.Fatalf("Method = %q, %v, want untouched", vm.Method, err)
	}

	res = f.do(http.MethodPatch, "/api/files/sets/"+docs.ID, map[string]any{
		"home": map[string]any{"repo": nas.ID}, "excludes": []string{"*.tmp"},
	})
	if res["code"] != "has-backups" {
		t.Fatalf("files PATCH = %v, want has-backups", res)
	}
	if got, err := f.st.GetFileSet(docs.ID); err != nil || len(got.Excludes) != 0 {
		t.Fatalf("Excludes = %v, %v, want untouched", got.Excludes, err)
	}
}

// TestABusyDomainLeavesOtherFieldsUntouched pins the busy half of the same
// ordering TestAHomeRefusalLeavesOtherFieldsUntouched pins for has-backups: a
// home change refuses up front while a backup holds the domain, so a field
// earlier in the same request body never lands either. The authoritative
// lock inside writeItemPlacement still catches the busy domain, but only
// after those fields would already have been written.
func TestABusyDomainLeavesOtherFieldsUntouched(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.openContainer("web")
	f.openVM("win11")
	docs := f.openFileSet("docs")

	unlock, ok := f.svc.tryLockDomainFor("containers", "backup")
	if !ok {
		t.Fatal("could not take the containers lock")
	}
	res := f.do(http.MethodPatch, "/api/containers/web", map[string]any{
		"home": map[string]any{"repo": nas.ID}, "preHook": "echo hi",
	})
	unlock()
	if res["code"] != "domain-busy" {
		t.Fatalf("containers PATCH = %v, want domain-busy", res)
	}
	if tg, err := f.st.GetTargetByContainer("web"); err != nil || tg.PreHook != "" {
		t.Fatalf("PreHook = %q, %v, want untouched", tg.PreHook, err)
	}

	unlock, ok = f.svc.tryLockDomainFor("vms", "backup")
	if !ok {
		t.Fatal("could not take the vms lock")
	}
	res = f.do(http.MethodPatch, "/api/vms/win11", map[string]any{
		"home": map[string]any{"repo": nas.ID}, "method": "acpi",
	})
	unlock()
	if res["code"] != "domain-busy" {
		t.Fatalf("vms PATCH = %v, want domain-busy", res)
	}
	if vm, err := f.st.GetVMTargetByName("win11"); err != nil || vm.Method != "graceful" {
		t.Fatalf("Method = %q, %v, want untouched", vm.Method, err)
	}

	unlock, ok = f.svc.tryLockDomainFor("files", "backup")
	if !ok {
		t.Fatal("could not take the files lock")
	}
	res = f.do(http.MethodPatch, "/api/files/sets/"+docs.ID, map[string]any{
		"home": map[string]any{"repo": nas.ID}, "excludes": []string{"*.tmp"},
	})
	unlock()
	if res["code"] != "domain-busy" {
		t.Fatalf("files PATCH = %v, want domain-busy", res)
	}
	if got, err := f.st.GetFileSet(docs.ID); err != nil || len(got.Excludes) != 0 {
		t.Fatalf("Excludes = %v, %v, want untouched", got.Excludes, err)
	}
}

func TestResetPreviewCountsTheNameInEveryCopySource(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	paperless := f.namedRepo("Paperless", "paperless")
	b2 := f.target("containers", "B2", "b2:bucket/containers")
	f.container("immich", nas.ID)
	f.container("paperless", paperless.ID)
	f.openContainer("vaultwarden")
	f.rule("containers", "container:vaultwarden", store.SkipAll)
	f.hold(f.root+"/nas", snap("aaaa0001", 100, "container:vaultwarden"), snap("aaaa0002", 200, "container:vaultwarden"))
	f.eng.listErr = map[string]error{f.root + "/paperless": errors.New("share not mounted")}
	f.listing("containers", b2.ID, 300)

	res := f.do(http.MethodPost, "/api/items/containers/vaultwarden/placement/preview", map[string]any{
		"home": map[string]any{"follow": true}, "copies": map[string]any{"follow": true},
	})
	added, _ := res["added"].([]any)
	if len(added) != 1 {
		t.Fatalf("added = %v, want B2", res)
	}
	a := added[0].(map[string]any)
	if a["targetId"] != b2.ID || a["snapshots"] != float64(2) {
		t.Fatalf("added = %v, want about 2 snapshots for B2 from NAS", a)
	}
	if !strings.Contains(fmt.Sprint(a["uncheckable"]), "Paperless") {
		t.Fatalf("uncheckable = %v, want the unreadable source named", a["uncheckable"])
	}
}

func TestAHomeOnAStorageboxDropsTheTargetsOfTheItem(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.namedRepo("Storagebox", "sftp:u1@box.example:/bv")
	b2 := f.target("containers", "B2", "b2:bucket/containers")
	f.openContainer("nginx")
	f.listing("containers", b2.ID, 300, copiesRow("container:nginx", 5, 290))

	res := f.do(http.MethodPost, "/api/items/containers/nginx/placement/preview", map[string]any{
		"home": map[string]any{"repo": box.ID}, "copies": map[string]any{"skip": []string{store.SkipAll}},
	})
	dropped, _ := res["dropped"].([]any)
	if len(dropped) != 1 {
		t.Fatalf("dropped = %v, want B2", res)
	}
	if d := dropped[0].(map[string]any); d["targetId"] != b2.ID || d["copies"] != float64(5) {
		t.Fatalf("dropped = %v, want B2 keeping 5 copies", d)
	}
	if len(res["added"].([]any)) != 0 {
		t.Fatalf("added = %v, want nothing", res["added"])
	}
}
