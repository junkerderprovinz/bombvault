package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
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

// TestPinningAnOpenItemToTheDomainPathIsRefusedWhenItHasBackups pins that the
// has-backups guard judges an open item by where its default actually sends
// it, not by its row's raw (always empty) repo field: a raw compare would see
// two empty strings and wave the pin through.
func TestPinningAnOpenItemToTheDomainPathIsRefusedWhenItHasBackups(t *testing.T) {
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
	if res["code"] != "has-backups" {
		t.Fatalf("PATCH = %v, want has-backups", res)
	}
	if got := f.home(store.ItemRef{Domain: "containers", Key: "nginx"}); got.Choice != store.RepoOpen {
		t.Fatalf("home = %+v, want it left open", got)
	}
}

// TestPinningAnOpenItemToItsEffectiveHomeIsAllowedEvenWithBackups is the
// mirror of the above: pinning the item to the repository its default already
// sends it to moves nothing, so the guard must let it through despite the
// item's history.
func TestPinningAnOpenItemToItsEffectiveHomeIsAllowedEvenWithBackups(t *testing.T) {
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
	if res["ok"] != true {
		t.Fatalf("PATCH = %v, want it allowed since nothing moves", res)
	}
	if got := f.home(store.ItemRef{Domain: "containers", Key: "nginx"}); got.Repo != nas.ID || got.Choice != store.RepoChosen {
		t.Fatalf("home = %+v, want NAS chosen", got)
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

// TestHomeWriteIsRefusedOverARowChangedMeanwhile pins that the final write
// compares against the same read checkHomeChange judged against: the hook
// here mutates the row from another write between that read and the write
// below it, exactly the window store.WritePlacement's expect argument closes.
// The PATCH now checks the home twice (the top-of-handler preview, then the
// authoritative pass right before the write), so the item is listed twice;
// the race is timed onto the second listing, the one the write's expect
// actually stands on.
func TestHomeWriteIsRefusedOverARowChangedMeanwhile(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	other := f.namedRepo("Backup2", "nas2")
	f.openContainer("nginx")
	item := store.ItemRef{Domain: "containers", Key: "nginx"}
	listings := 0
	f.eng.onSnapshots = func() {
		listings++
		if listings < 2 {
			return
		}
		if _, err := f.st.WritePlacement(item, &store.HomeWrite{Repo: other.ID, Choice: store.RepoChosen}, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	res := f.do(http.MethodPatch, "/api/containers/nginx", map[string]any{"home": map[string]any{"repo": nas.ID}})
	if res["code"] != "stale" {
		t.Fatalf("PATCH = %v, want stale", res)
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
