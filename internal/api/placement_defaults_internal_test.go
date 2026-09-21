package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestDefaultRowsCountTheItemsOfEachDomain(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	done := f.container("done", "")
	f.backupRun(done.ID, 100)
	f.container("waiting", nas.ID)
	f.openContainer("fresh")
	f.openContainer("local-only")
	f.rule("containers", "container:local-only", store.SkipAll)

	res := f.do(http.MethodGet, "/api/placement/defaults", nil)
	rows, _ := res["defaults"].([]any)
	if len(rows) != 3 {
		t.Fatalf("defaults = %v, want one row per domain", res)
	}
	containers := rows[0].(map[string]any)
	if containers["domain"] != "containers" || containers["exists"] != false || containers["homeKind"] != "domain" {
		t.Fatalf("row = %v", containers)
	}
	counts := containers["counts"].(map[string]any)
	for key, want := range map[string]float64{"follow": 3, "own": 1, "open": 2, "chosenNoRun": 1} {
		if counts[key] != want {
			t.Errorf("%s = %v, want %v", key, counts[key], want)
		}
	}
	if skip, ok := containers["skip"].([]any); !ok || len(skip) != 0 {
		t.Errorf("skip = %v, want an empty list", containers["skip"])
	}
}

// TestTheCardAndTheApplyButtonAgreeOnADiscoverRebuiltItem pins that the two
// checks no longer contradict each other. The card counts an item under
// chosenNoRun by the runs table alone; the apply button still refuses to
// reset it once its own, fuller check finds the snapshot that never got a
// run recorded, which is exactly what a row Discover rebuilds looks like.
func TestTheCardAndTheApplyButtonAgreeOnADiscoverRebuiltItem(t *testing.T) {
	f := newPlacementFixture(t)
	f.setDefault("containers", "")
	f.container("rebuilt", "")
	f.hold(f.domainPath("containers"), snap("aaaa0001", 100, "container:rebuilt"))

	res := f.do(http.MethodGet, "/api/placement/defaults", nil)
	rows, _ := res["defaults"].([]any)
	containers := rows[0].(map[string]any)
	counts := containers["counts"].(map[string]any)
	if counts["chosenNoRun"] != float64(1) {
		t.Fatalf("chosenNoRun = %v, want 1: the card has no recorded run for it", counts["chosenNoRun"])
	}

	_, kept := candidatesByKey(f.do(http.MethodGet, "/api/placement/default/containers/apply", nil))
	if kept["rebuilt"] != "has-backups" {
		t.Fatalf("kept = %v, want rebuilt refused as has-backups", kept)
	}
}

// fourteenContainers is fourteen containers on the domain path, one project
// folder in it and one target that holds some of their copies.
func fourteenContainers(t *testing.T) (*placementFixture, store.OffsiteTarget) {
	t.Helper()
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket/containers")
	f.setDefault("containers", "")
	var held []restic.Snapshot
	for i := range 14 {
		name := fmt.Sprintf("app%02d", i)
		f.container(name, "")
		held = append(held, snap(fmt.Sprintf("aaaa%04d", i), 100, "container:"+name))
	}
	held = append(held, snap("bbbb0001", 100, "stack:immich"))
	f.hold(f.domainPath("containers"), held...)
	f.listing("containers", b2.ID, 200, copiesRow("container:app00", 3, 90), copiesRow("stack:immich", 2, 90))
	return f, b2
}

func TestAHomeOnlyDefaultChangeLeavesTheTargetsAlone(t *testing.T) {
	f, b2 := fourteenContainers(t)
	box := f.namedRepo("Storagebox", "sftp:u1@box.example:/bv")
	res := f.do(http.MethodPost, "/api/placement/default/containers/preview", map[string]any{"home": box.ID})
	impact := res["impact"].(map[string]any)
	if len(impact["dropped"].([]any)) != 0 || len(impact["added"].([]any)) != 0 {
		t.Fatalf("impact = %v, want no target to lose or gain anything", impact)
	}
	if n := f.eng.lists[b2.Repo]; n != 0 {
		t.Errorf("the preview listed the target %d times", n)
	}
}

func TestAnOffsiteOnlyHomeLeavesTheDefaultSkipAlone(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.namedRepo("Storagebox", "sftp:u1@box.example:/bv")
	f.target("containers", "B2", "b2:bucket/containers")
	f.setDefault("containers", "")
	f.container("web", "")
	body := map[string]any{"home": box.ID, "skip": []string{store.SkipAll}}
	impact := f.do(http.MethodPost, "/api/placement/default/containers/preview", body)["impact"].(map[string]any)
	if len(impact["dropped"].([]any)) != 0 {
		t.Fatalf("impact = %v, want no target to lose web", impact)
	}
	body["expect"] = impact
	if res := f.do(http.MethodPut, "/api/placement/default/containers", body); res["ok"] != true {
		t.Fatalf("PUT = %v", res)
	}
	d, found, err := f.st.PlacementDefaultFor("containers")
	if err != nil || !found || d.Home != box.ID || len(d.Skip) != 0 {
		t.Fatalf("default = %+v, %v, %v, want Storagebox with skip still []", d, found, err)
	}
}

func TestLocalForTheDefaultAsksForItemsAndProjectFolders(t *testing.T) {
	f, b2 := fourteenContainers(t)
	res := f.do(http.MethodPost, "/api/placement/default/containers/preview", map[string]any{"skip": []string{store.SkipAll}})
	dropped := res["impact"].(map[string]any)["dropped"].([]any)
	if len(dropped) != 1 {
		t.Fatalf("dropped = %v, want B2", dropped)
	}
	d := dropped[0].(map[string]any)
	if d["targetId"] != b2.ID || d["items"] != float64(15) || d["snapshots"] != float64(5) || d["unknown"] != false {
		t.Fatalf("dropped = %v, want B2 with 15 items and 5 copies that stay", d)
	}
}

func TestADefaultSkipChangeIgnoresAnItemHomedOnARemoteNamedRepositoryByDefault(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.namedRepo("Storagebox", "sftp:u1@box.example:/bv")
	f.target("containers", "B2", "b2:bucket/containers")
	f.setDefault("containers", box.ID)
	f.openContainer("web")

	res := f.do(http.MethodPost, "/api/placement/default/containers/preview", map[string]any{"skip": []string{store.SkipAll}})
	dropped := res["impact"].(map[string]any)["dropped"].([]any)
	if len(dropped) != 0 {
		t.Fatalf("dropped = %v, want none: web is homed on Storagebox by default, not on the domain path", dropped)
	}
}

func TestPutWithStaleNumbersIsRefusedWithTheNewOnes(t *testing.T) {
	f, _ := fourteenContainers(t)
	empty := map[string]any{"dropped": []any{}, "added": []any{}, "openTakeHome": 0}
	res := f.do(http.MethodPut, "/api/placement/default/containers", map[string]any{"skip": []string{store.SkipAll}, "expect": empty})
	if res["ok"] != false || res["code"] != "stale" {
		t.Fatalf("PUT = %v, want code stale", res)
	}
	res = f.do(http.MethodPut, "/api/placement/default/containers", map[string]any{"skip": []string{store.SkipAll}, "expect": res["impact"]})
	if res["ok"] != true {
		t.Fatalf("PUT with the new numbers = %v", res)
	}
	d, found, err := f.st.PlacementDefaultFor("containers")
	if err != nil || !found || !skipsEverything(d.Skip) {
		t.Fatalf("default = %+v, %v, %v, want skip [*]", d, found, err)
	}
}

// TestASecondTabsHomeChangeIsRefusedOnceTheFirstHasLanded pins the gap
// sameCounts left open: a home-only change with no skip in the body always
// answers an empty impact regardless of which home it names, so two tabs
// that each preview a different home from the same starting point see the
// identical (empty) numbers. Comparing only those numbers would let the
// second tab's PUT win silently over the first's.
func TestASecondTabsHomeChangeIsRefusedOnceTheFirstHasLanded(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	cold := f.namedRepo("Cold", "cold")

	implA := f.do(http.MethodPost, "/api/placement/default/containers/preview", map[string]any{"home": nas.ID})["impact"]
	implB := f.do(http.MethodPost, "/api/placement/default/containers/preview", map[string]any{"home": cold.ID})["impact"]

	if res := f.do(http.MethodPut, "/api/placement/default/containers", map[string]any{"home": nas.ID, "expect": implA}); res["ok"] != true {
		t.Fatalf("first PUT = %v", res)
	}
	res := f.do(http.MethodPut, "/api/placement/default/containers", map[string]any{"home": cold.ID, "expect": implB})
	if res["ok"] != false || res["code"] != "stale" {
		t.Fatalf("second PUT = %v, want stale: NAS already replaced the home this tab previewed against", res)
	}
	d, _, err := f.st.PlacementDefaultFor("containers")
	if err != nil || d.Home != nas.ID {
		t.Fatalf("default = %+v, %v, want NAS to survive the second PUT", d, err)
	}
}

// TestAPutWithoutAnExpectBlockGetsItsOwnAnswer pins that a caller who never
// saw any numbers gets told to fetch them, not that its (nonexistent) numbers
// went stale.
func TestAPutWithoutAnExpectBlockGetsItsOwnAnswer(t *testing.T) {
	f := newPlacementFixture(t)
	res := f.do(http.MethodPut, "/api/placement/default/containers", map[string]any{"home": ""})
	if res["code"] != "expect-required" {
		t.Fatalf("PUT without expect = %v, want expect-required", res)
	}
}

func TestOpenItemsWithoutHistoryTakeTheNewHome(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.openContainer("fresh")
	f.openContainer("returning")
	f.hold(f.domainPath("containers"), snap("aaaa0001", 100, "container:returning"))
	res := f.do(http.MethodPost, "/api/placement/default/containers/preview", map[string]any{"home": nas.ID})
	if got := res["impact"].(map[string]any)["openTakeHome"]; got != float64(1) {
		t.Fatalf("openTakeHome = %v, want 1", got)
	}
}

func TestOnARemoteDomainPathEveryOpenItemMayTakeTheNewHome(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersPath = "s3:s3.example.com/bucket/containers"
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	f.openContainer("fresh")
	f.openContainer("returning")
	res := f.do(http.MethodPost, "/api/placement/default/containers/preview", map[string]any{"home": nas.ID})
	if got := res["impact"].(map[string]any)["openTakeHome"]; got != float64(2) {
		t.Fatalf("openTakeHome = %v, want 2", got)
	}
	if n := f.eng.lists[settings.ContainersPath]; n != 0 {
		t.Errorf("the remote domain path was listed %d times", n)
	}
}

func TestADefaultOnASwitchedOffRepositoryIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	nas.Enabled = false
	if _, err := f.st.UpsertOffsiteTarget(nas); err != nil {
		t.Fatal(err)
	}
	res := f.do(http.MethodPut, "/api/placement/default/containers", map[string]any{"home": nas.ID})
	if res["code"] != "repo-invalid" {
		t.Fatalf("PUT = %v, want repo-invalid", res)
	}
}

func TestADefaultSkipNamesOnlyTargetsOfItsDomain(t *testing.T) {
	f := newPlacementFixture(t)
	vms := f.target("vms", "B2 VMs", "b2:bucket/vms")
	res := f.do(http.MethodPut, "/api/placement/default/containers", map[string]any{"skip": []string{vms.ID}})
	if res["code"] != "unknown-target" {
		t.Fatalf("PUT = %v, want unknown-target", res)
	}
}

func TestADefaultRouteForAnotherDomainIsABadRequest(t *testing.T) {
	f := newPlacementFixture(t)
	rec := httptest.NewRecorder()
	f.h.Router().ServeHTTP(rec, jsonReq(http.MethodPut, "/api/placement/default/flash", strings.NewReader(`{}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func candidatesByKey(res map[string]any) (map[string]map[string]any, map[string]string) {
	reset := map[string]map[string]any{}
	for _, c := range res["reset"].([]any) {
		m := c.(map[string]any)
		reset[m["key"].(string)] = m
	}
	kept := map[string]string{}
	for _, k := range res["kept"].([]any) {
		m := k.(map[string]any)
		kept[m["key"].(string)] = m["reason"].(string)
	}
	return reset, kept
}

func TestApplyPreviewSortsResetFromKept(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	cold := f.namedRepo("Cold", "cold")
	f.target("containers", "B2", "b2:bucket/containers")
	f.setDefault("containers", "")
	f.container("moved", nas.ID)
	f.openContainer("excluded")
	f.rule("containers", "container:excluded", store.SkipAll)
	done := f.container("done", "")
	f.backupRun(done.ID, 100)
	f.container("unseen", cold.ID)
	f.eng.listErr = map[string]error{f.root + "/cold": errors.New("share not mounted")}
	f.openContainer("plain")

	reset, kept := candidatesByKey(f.do(http.MethodGet, "/api/placement/default/containers/apply", nil))
	if len(reset) != 2 || reset["moved"]["losesHome"] != true || reset["excluded"]["losesRule"] != true {
		t.Fatalf("reset = %v, want moved losing its home and excluded losing its rule", reset)
	}
	if len(kept) != 2 || kept["done"] != "has-backups" || kept["unseen"] != "unreadable" {
		t.Fatalf("kept = %v, want done with backups and unseen unreadable", kept)
	}
}

func TestApplyPreviewCountsARuledOutNameInEveryCopySource(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	paperless := f.namedRepo("Paperless", "paperless")
	b2 := f.target("containers", "B2", "b2:bucket/containers")
	f.setDefault("containers", "")
	f.container("immich", nas.ID)
	f.container("paperless", paperless.ID)
	f.openContainer("vaultwarden")
	f.rule("containers", "container:vaultwarden", store.SkipAll)
	f.hold(f.root+"/nas", snap("aaaa0001", 100, "container:vaultwarden"), snap("aaaa0002", 200, "container:vaultwarden"))
	f.eng.listErr = map[string]error{f.root + "/paperless": errors.New("share not mounted")}
	f.listing("containers", b2.ID, 300)

	reset, _ := candidatesByKey(f.do(http.MethodGet, "/api/placement/default/containers/apply", nil))
	uploads, _ := reset["vaultwarden"]["uploads"].([]any)
	if len(uploads) != 1 {
		t.Fatalf("vaultwarden = %v, want one upload", reset["vaultwarden"])
	}
	u := uploads[0].(map[string]any)
	if u["targetId"] != b2.ID || u["snapshots"] != float64(2) || !strings.Contains(fmt.Sprint(u["uncheckable"]), "Paperless") {
		t.Fatalf("upload = %v, want about 2 for B2 and Paperless named as not checkable", u)
	}
}

// TestApplyPreviewJudgesAnOpenItemsCopiesByItsEffectiveHome pins effectiveHome
// against vaultwarden, open under a remote default: the preview must not read
// its raw (empty) repo field and treat the domain path as a copy source.
func TestApplyPreviewJudgesAnOpenItemsCopiesByItsEffectiveHome(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.namedRepo("Storagebox", "sftp:u1@box.example:/bv")
	f.target("containers", "B2", "b2:bucket/containers")
	f.setDefault("containers", box.ID)
	f.openContainer("vaultwarden")
	f.rule("containers", "container:vaultwarden", store.SkipAll)

	reset, _ := candidatesByKey(f.do(http.MethodGet, "/api/placement/default/containers/apply", nil))
	if len(reset) != 1 {
		t.Fatalf("reset = %v, want vaultwarden alone", reset)
	}
	uploads, _ := reset["vaultwarden"]["uploads"].([]any)
	if len(uploads) != 0 {
		t.Fatalf("vaultwarden = %v, want no upload: it is already homed on Storagebox, not the domain path", reset["vaultwarden"])
	}
}

func TestApplyResetsBothAxesAndNamesWhatStays(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.setDefault("containers", "")
	f.container("moved", nas.ID)
	f.rule("containers", "container:moved", store.SkipAll)
	done := f.container("done", "")
	f.backupRun(done.ID, 100)

	res := f.do(http.MethodPost, "/api/placement/default/containers/apply", map[string]any{"keys": []string{"moved", "done", "gone"}})
	if res["ok"] != true {
		t.Fatalf("apply = %v", res)
	}
	if reset := res["reset"].([]any); len(reset) != 1 || reset[0] != "moved" {
		t.Fatalf("reset = %v, want moved", reset)
	}
	_, kept := candidatesByKey(map[string]any{"reset": []any{}, "kept": res["kept"]})
	if kept["done"] != "has-backups" || kept["gone"] != "changed" {
		t.Fatalf("kept = %v", kept)
	}
	if got := f.home(store.ItemRef{Domain: "containers", Key: "moved"}); got.Repo != "" || got.Choice != store.RepoOpen {
		t.Fatalf("home = %+v, want open", got)
	}
	if _, found, err := f.st.CopyRuleFor("containers", "container:moved"); err != nil || found {
		t.Fatalf("rule found = %v, %v, want it gone", found, err)
	}
}

func TestApplyIsRefusedWhileABackupHoldsTheDomain(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.container("moved", nas.ID)
	unlock, ok := f.svc.tryLockDomainFor("containers", "backup")
	if !ok {
		t.Fatal("could not take the containers lock")
	}
	res := f.do(http.MethodPost, "/api/placement/default/containers/apply", map[string]any{"keys": []string{"moved"}})
	unlock()
	if res["code"] != "domain-busy" {
		t.Fatalf("apply = %v, want domain-busy", res)
	}
	if got := f.home(store.ItemRef{Domain: "containers", Key: "moved"}); got.Repo != nas.ID {
		t.Fatalf("home = %+v, want it untouched", got)
	}
}

// pausedContainers is a rebuilt containers domain: paused, one row, and a name in
// the domain path that no row knows.
func pausedContainers(t *testing.T) (*placementFixture, store.OffsiteTarget) {
	t.Helper()
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket/containers")
	f.paused("containers")
	f.container("nginx", "")
	f.hold(f.domainPath("containers"),
		snap("aaaa0001", 100, "container:nginx"),
		snap("bbbb0001", 100, "container:old-app"),
		snap("bbbb0002", 200, "container:old-app"))
	return f, b2
}

func TestConfirmPreviewListsWhatCopiesAndNamesWithoutARow(t *testing.T) {
	f, b2 := pausedContainers(t)
	res := f.do(http.MethodGet, "/api/placement/default/containers/confirm", nil)
	if res["paused"] != true {
		t.Fatalf("confirm preview = %v, want paused", res)
	}
	targets := res["targets"].([]any)
	if len(targets) != 1 {
		t.Fatalf("targets = %v, want B2", targets)
	}
	row := targets[0].(map[string]any)
	preview := row["preview"].(map[string]any)
	if row["targetId"] != b2.ID || preview["items"] != float64(1) || preview["snapshots"] != float64(1) {
		t.Fatalf("B2 = %v, want one item with one snapshot", row)
	}
	unmatched := res["unmatched"].([]any)
	if len(unmatched) != 1 {
		t.Fatalf("unmatched = %v, want old-app", unmatched)
	}
	if u := unmatched[0].(map[string]any); u["identity"] != "container:old-app" || u["snapshots"] != float64(2) {
		t.Fatalf("unmatched = %v", u)
	}
}

func TestConfirmPreviewReportsAnUnpausedDomain(t *testing.T) {
	f := newPlacementFixture(t)
	f.setDefault("containers", "")
	res := f.do(http.MethodGet, "/api/placement/default/containers/confirm", nil)
	if res["paused"] != false {
		t.Fatalf("confirm preview = %v, want an unpaused domain", res)
	}
}

func TestConfirmEndsThePauseAndLeavesTheTickedNamesOut(t *testing.T) {
	f, b2 := pausedContainers(t)
	res := f.do(http.MethodPost, "/api/placement/default/containers/confirm", map[string]any{"exclude": []string{"container:old-app"}})
	if res["ok"] != true {
		t.Fatalf("confirm = %v", res)
	}
	d, found, err := f.st.PlacementDefaultFor("containers")
	if err != nil || !found || d.Paused() {
		t.Fatalf("default = %+v, %v, %v, want confirmed", d, found, err)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	p, err := f.svc.readPlacement(settings, "containers")
	if err != nil {
		t.Fatal(err)
	}
	if p.copiesTo(b2.ID, []string{"container:old-app"}) {
		t.Fatal("old-app is still copied to B2 after it was left out")
	}
	if !p.copiesTo(b2.ID, []string{"container:nginx"}) {
		t.Fatal("nginx is no longer copied to B2")
	}
}

func TestConfirmLeavesATickedNameOutOfTheNextRun(t *testing.T) {
	f, b2 := pausedContainers(t)
	// A first listing of B2 would find these old snapshots and pause the domain again.
	f.listing("containers", b2.ID, 300)
	if res := f.do(http.MethodPost, "/api/placement/default/containers/confirm", map[string]any{"exclude": []string{"container:old-app"}}); res["ok"] != true {
		t.Fatalf("confirm = %v", res)
	}
	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatalf("ReplicateOffsite: %v", err)
	}
	var copied []string
	for _, c := range f.eng.copies {
		if c.IDs == nil {
			t.Fatalf("a whole-repository copy from %s takes old-app along", c.Src)
		}
		copied = append(copied, c.IDs...)
	}
	if !slices.Contains(copied, "aaaa0001") {
		t.Fatalf("copied %v, want nginx's snapshot", copied)
	}
	if slices.Contains(copied, "bbbb0001") || slices.Contains(copied, "bbbb0002") {
		t.Fatalf("copied %v, want nothing of old-app", copied)
	}
}

func TestConfirmRefusesANameOfAnotherDomain(t *testing.T) {
	f, _ := pausedContainers(t)
	res := f.do(http.MethodPost, "/api/placement/default/containers/confirm", map[string]any{"exclude": []string{"vm:win11"}})
	if res["code"] != "invalid-placement" {
		t.Fatalf("confirm = %v, want invalid-placement", res)
	}
	if d, _, _ := f.st.PlacementDefaultFor("containers"); !d.Paused() {
		t.Fatal("a refused confirmation ended the pause")
	}
}

func TestConfirmPlacementResumesAPausedDomain(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:nginx"))

	// The first pass finds unreplicated history and pauses.
	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	if !pausedDefault(t, f, "containers") {
		t.Fatal("setup: the domain did not pause")
	}

	if res := f.do(http.MethodPost, "/api/placement/default/containers/confirm", map[string]any{}); res["ok"] != true {
		t.Fatalf("confirm = %v, want ok", res)
	}
	if pausedDefault(t, f, "containers") {
		t.Fatal("the domain is still paused after confirm")
	}
	f.eng.copies = nil
	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	if len(f.eng.copies) != 1 {
		t.Fatalf("copies = %+v, want one copy after the confirmation", f.eng.copies)
	}
}

func TestConfirmingAHealthyDomainDoesNotSilenceALaterFirstListingPause(t *testing.T) {
	f := newPlacementFixture(t)
	if res := f.do(http.MethodPost, "/api/placement/default/containers/confirm", map[string]any{}); res["ok"] != true {
		t.Fatalf("confirm = %v, want ok", res)
	}

	b2 := f.target("containers", "B2", "b2:bucket:containers")
	now := time.Now().Unix()
	f.hold(f.domainPath("containers"), snap("a9", now, "container:nginx"))
	f.hold("b2:bucket:containers", copied("b1", "a1", now-86400, "container:nginx"))

	for range 2 {
		if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
			t.Fatal(err)
		}
	}
	if !pausedDefault(t, f, "containers") {
		t.Fatal("confirming a domain that was never paused silenced its later first-listing pause")
	}
	if _, listed, err := f.st.TargetObservationFor("containers", b2.ID); err != nil || !listed {
		t.Fatalf("the listing that found it was not recorded (listed=%v err=%v)", listed, err)
	}
}

func TestConfirmPlacementRefusesAnUnknownDomain(t *testing.T) {
	f := newPlacementFixture(t)
	rec := httptest.NewRecorder()
	f.h.Router().ServeHTTP(rec, jsonReq(http.MethodPost, "/api/placement/default/flash/confirm", strings.NewReader(`{}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("confirm flash = %d, want 400", rec.Code)
	}
}

func TestConfirmPlacementAcceptsAnEmptyBody(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:nginx"))
	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	if !pausedDefault(t, f, "containers") {
		t.Fatal("setup: the domain did not pause")
	}

	rec := httptest.NewRecorder()
	f.h.Router().ServeHTTP(rec, jsonReq(http.MethodPost, "/api/placement/default/containers/confirm", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("confirm with no body = %d: %s", rec.Code, rec.Body.String())
	}
	if pausedDefault(t, f, "containers") {
		t.Fatal("the domain is still paused after a bodyless confirm")
	}
}

func TestConfirmPlacementIsIdempotentOnADomainNeverPaused(t *testing.T) {
	f := newPlacementFixture(t)
	if res := f.do(http.MethodPost, "/api/placement/default/containers/confirm", map[string]any{"exclude": []string{"container:old-app"}}); res["ok"] != true {
		t.Fatalf("confirm = %v, want ok", res)
	}
	if _, found, err := f.st.PlacementDefaultFor("containers"); err != nil {
		t.Fatal(err)
	} else if found {
		t.Fatal("confirming a domain that was never paused wrote a placement default")
	}
	if _, found := ruleOf(t, f, "containers", "container:old-app"); found {
		t.Fatal("confirming a domain that was never paused wrote a copy rule from its skip list")
	}
	if res := f.do(http.MethodPost, "/api/placement/default/containers/confirm", map[string]any{}); res["ok"] != true {
		t.Fatalf("second confirm = %v, want ok", res)
	}
}

// TestASecondConfirmationWritesItsExclusion pins the fix: the confirm route is
// the only way an exclusion rule is written, so a domain that is already
// confirmed must not silently drop what a later confirmation asks to exclude.
func TestASecondConfirmationWritesItsExclusion(t *testing.T) {
	f, _ := pausedContainers(t)
	if res := f.do(http.MethodPost, "/api/placement/default/containers/confirm", map[string]any{"exclude": []string{"container:old-app"}}); res["ok"] != true {
		t.Fatalf("first confirm = %v", res)
	}
	if pausedDefault(t, f, "containers") {
		t.Fatal("setup: the domain is still paused after the first confirm")
	}
	res := f.do(http.MethodPost, "/api/placement/default/containers/confirm", map[string]any{"exclude": []string{"container:nginx"}})
	if res["ok"] != true {
		t.Fatalf("second confirm = %v, want ok", res)
	}
	if _, found := ruleOf(t, f, "containers", "container:nginx"); !found {
		t.Fatal("a second confirmation dropped the exclusion it asked for")
	}
}

// TestASecondConfirmationDoesNotGrantAPutDefaultManualImmunity is the other
// half: a domain that has a default only because it was saved through the PUT
// route, never through an actual confirmation, must not gain the manual
// marker just because a later confirm call wrote its exclusion.
func TestASecondConfirmationDoesNotGrantAPutDefaultManualImmunity(t *testing.T) {
	f := newPlacementFixture(t)
	f.setDefault("containers", "")
	res := f.do(http.MethodPost, "/api/placement/default/containers/confirm", map[string]any{"exclude": []string{"container:old-app"}})
	if res["ok"] != true {
		t.Fatalf("confirm = %v, want ok", res)
	}
	if _, found := ruleOf(t, f, "containers", "container:old-app"); !found {
		t.Fatal("confirming a domain with a PUT default dropped the exclusion it asked for")
	}
	d, found, err := f.st.PlacementDefaultFor("containers")
	if err != nil || !found {
		t.Fatalf("default = %+v, %v, %v", d, found, err)
	}
	if d.ConfirmedManually {
		t.Fatal("confirming a domain that was never paused granted its PUT default manual-confirmation immunity")
	}
}

func TestDeletingARepositoryADefaultPointsAtNamesTheDomain(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.setDefault("containers", nas.ID)
	res := f.do(http.MethodDelete, "/api/repos/"+nas.ID, nil)
	if res["code"] != "repo-in-use" || res["items"] != float64(0) {
		t.Fatalf("DELETE = %v, want repo-in-use without items", res)
	}
	if domains, _ := res["defaultDomains"].([]any); len(domains) != 1 || domains[0] != "containers" {
		t.Fatalf("defaultDomains = %v, want containers", res["defaultDomains"])
	}
	if _, err := f.st.GetNamedRepo(nas.ID); err != nil {
		t.Fatalf("the repository is gone: %v", err)
	}
}

// TestMovingARepositoryADefaultPointsAtNamesTheDomain is the move's side of
// the above: a default that homes open items on this repository has no row in
// an item table, so the guard has to catch it the same way the delete does.
func TestMovingARepositoryADefaultPointsAtNamesTheDomain(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.setDefault("containers", nas.ID)
	res := f.do(http.MethodPatch, "/api/repos/"+nas.ID, map[string]any{"repo": "elsewhere"})
	if res["code"] != "repo-in-use" || res["items"] != float64(0) {
		t.Fatalf("PATCH = %v, want repo-in-use without items", res)
	}
	if domains, _ := res["defaultDomains"].([]any); len(domains) != 1 || domains[0] != "containers" {
		t.Fatalf("defaultDomains = %v, want containers", res["defaultDomains"])
	}
	back, err := f.st.GetNamedRepo(nas.ID)
	if err != nil || back.Repo != "nas" {
		t.Fatalf("repo = %+v, %v, want the location untouched", back, err)
	}
}
