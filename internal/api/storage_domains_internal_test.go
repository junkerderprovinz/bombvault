package api

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func (f *placementFixture) domainRow(domain string) map[string]any {
	f.t.Helper()
	for _, row := range rowsOf(f.do(http.MethodGet, "/api/storage/domains", nil)["domains"]) {
		if row["domain"] == domain {
			return row
		}
	}
	f.t.Fatalf("no domain row %s", domain)
	return nil
}

// chipOf is the chip of a place in a domain row, nil when the row has none.
func chipOf(row map[string]any, placeID string) map[string]any {
	for _, c := range rowsOf(row["chips"]) {
		if c["placeId"] == placeID {
			return c
		}
	}
	return nil
}

func TestADomainRowOffersEveryPlaceButItsHomeAndThoseWithoutItsFolder(t *testing.T) {
	f := newPlacementFixture(t)
	unraid := f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	vmsOnly := localPlace("NAS", "nas")
	vmsOnly.Folders = map[string]string{"vms": "vms"}
	nas := f.storePlace(vmsOnly)

	containers := f.domainRow("containers")

	if containers["homePlace"] != unraid.ID || containers["storedIn"] != unraid.ID || chipOf(containers, unraid.ID) != nil ||
		chipOf(containers, b2.ID) == nil || chipOf(containers, nas.ID) != nil {
		t.Fatalf("containers row = %v", containers)
	}
	if vms := f.domainRow("vms"); chipOf(vms, nas.ID) == nil || chipOf(vms, b2.ID) == nil {
		t.Fatalf("vms row = %v", vms)
	}
	if len(f.eng.lists) != 0 || len(f.eng.opened) != 0 {
		t.Fatalf("the domain rows reached restic: lists %v, opened %v", f.eng.lists, f.eng.opened)
	}
}

func TestAChipIsOnWhileTheDefaultCopiesToItsTarget(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	target := f.placeTarget(b2, "containers", "")
	if chip := chipOf(f.domainRow("containers"), b2.ID); chip["on"] != true || chip["targetId"] != target.ID {
		t.Fatalf("chip = %v, want it on", chip)
	}
	f.setDefault("containers", "", target.ID)
	if chip := chipOf(f.domainRow("containers"), b2.ID); chip["on"] != false {
		t.Fatalf("chip = %v, want it off once the default skips the target", chip)
	}
}

func TestAChipOfASwitchedOffPlaceShowsItsStoredStateDimmed(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	f.placeTarget(b2, "containers", "")
	off, err := f.st.GetPlace(b2.ID)
	if err != nil {
		t.Fatal(err)
	}
	off.Enabled = false
	if _, err := f.st.WritePlace(store.PlaceWrite{Place: off}); err != nil {
		t.Fatal(err)
	}
	if chip := chipOf(f.domainRow("containers"), b2.ID); chip["on"] != true || chip["disabled"] != true || chip["reason"] != "off" {
		t.Fatalf("chip = %v, want it dimmed with its stored state", chip)
	}
}

func TestTheDefaultsPlaceIsNoChipUnlessTheDefaultIsItsTargetsDirectRepository(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	nas := f.storePlace(localPlace("NAS", "nas"))
	named, err := f.st.CreatePlaceRepo(nas.ID, "containers", "")
	if err != nil {
		t.Fatal(err)
	}
	f.setDefault("containers", named.ID)
	if row := f.domainRow("containers"); row["storedIn"] != nas.ID || chipOf(row, nas.ID) != nil {
		t.Fatalf("row = %v, want it stored at NAS without a NAS chip", row)
	}

	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	direct := f.direct(f.placeTarget(b2, "containers", ""))
	f.linkRow(direct.ID, b2, "containers", "-direct")
	f.setDefault("containers", direct.ID)
	if row := f.domainRow("containers"); row["storedIn"] != b2.ID || chipOf(row, b2.ID) == nil || chipOf(row, nas.ID) == nil {
		t.Fatalf("row = %v, want it stored at B2 with both chips", row)
	}
}

func TestAPlaceThatIsItselfTheDomainsRepositoryOffersNoChip(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	whole := localPlace("Shared", "shared")
	whole.Folders = map[string]string{"containers": "", "vms": "", "flash": "", "config": "", "files": ""}
	shared := f.storePlace(whole)
	if _, err := f.st.CreatePlaceRepo(shared.ID, "", ""); err != nil {
		t.Fatal(err)
	}
	if chip := chipOf(f.domainRow("containers"), shared.ID); chip != nil {
		t.Fatalf("chip = %v, want none: a target there would lie inside the repository", chip)
	}
}

func TestADomainRowListsItsExceptionsWithLinksToTheirCards(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	old := f.namedRepo("Old NAS", "oldnas")
	f.container("nginx", old.ID)
	f.container("plex", "")
	f.rule("containers", "container:plex", store.SkipAll)
	f.container("sonarr", "")

	got := rowsOf(f.domainRow("containers")["exceptions"])

	want := []map[string]any{
		{"identity": "container:nginx", "name": "nginx", "link": "/containers?item=nginx"},
		{"identity": "container:plex", "name": "plex", "link": "/containers?item=plex"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("exceptions = %v, want %v", got, want)
	}
}

func TestAPausedDomainRowSaysSoWithItsSchedule(t *testing.T) {
	f := newPlacementFixture(t)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.VMsSchedule = "daily 03:00"
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	f.paused("vms")
	if row := f.domainRow("vms"); row["paused"] != true || row["schedule"] != "daily 03:00" {
		t.Fatalf("vms row = %v", row)
	}
}

func TestAFlashChipFollowsItsTargetsSwitch(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	target := f.placeTarget(b2, "flash", "")
	if chip := chipOf(f.domainRow("flash"), b2.ID); chip["on"] != true {
		t.Fatalf("chip = %v, want it on", chip)
	}
	target.Enabled = false
	if _, err := f.st.UpsertOffsiteTarget(target); err != nil {
		t.Fatal(err)
	}
	if chip := chipOf(f.domainRow("flash"), b2.ID); chip["on"] != false || chip["disabled"] != false {
		t.Fatalf("chip = %v, want it off and selectable", chip)
	}
}

func TestAChipSaysWhenACopyWouldNeedTwoSetsOfTheSameKeys(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCredSets([]CloudCredSet{
		{ID: "set-a", Name: "A", Kind: "s3", CloudCreds: CloudCreds{S3KeyID: "ka", S3Secret: "sa"}},
		{ID: "set-b", Name: "B", Kind: "s3", CloudCreds: CloudCreds{S3KeyID: "kb", S3Secret: "sb"}},
	}); err != nil {
		t.Fatal(err)
	}
	home := s3Place("Home bucket", "s3:https://s3.example.com/home")
	home.CredsRef = "set-a"
	f.storePlace(home, "containers")
	other := s3Place("Other bucket", "s3:https://s3.example.com/other")
	other.CredsRef = "set-b"
	b := f.storePlace(other)
	if chip := chipOf(f.domainRow("containers"), b.ID); chip["reason"] != "creds-differ" || chip["disabled"] != false {
		t.Fatalf("chip = %v, want the creds-differ hint on a selectable chip", chip)
	}
}

func TestAnItemLeftOnTheDomainPathIsNoException(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	nas := f.storePlace(localPlace("NAS", "nas"))
	named, err := f.st.CreatePlaceRepo(nas.ID, "containers", "")
	if err != nil {
		t.Fatal(err)
	}
	f.container("plex", "")
	f.container("nginx", named.ID)
	f.setDefault("containers", named.ID)
	if got := rowsOf(f.domainRow("containers")["exceptions"]); len(got) != 0 {
		t.Fatalf("exceptions = %v, want none: plex backed up before the default moved and nginx follows it", got)
	}
}

func TestADomainRowWithoutPlacesAnswersEmptyListsAndNoBackupVerdict(t *testing.T) {
	f := newPlacementFixture(t)
	row := f.domainRow("config")
	if chips, ok := row["chips"].([]any); !ok || len(chips) != 0 {
		t.Fatalf("chips = %#v, want an empty list", row["chips"])
	}
	if exceptions, ok := row["exceptions"].([]any); !ok || len(exceptions) != 0 {
		t.Fatalf("exceptions = %#v, want an empty list", row["exceptions"])
	}
	if _, set := row["homeHasBackups"]; set || row["homePlace"] != "" || row["storedIn"] != "" || row["unreadable"] != false {
		t.Fatalf("config row = %v", row)
	}
}

// homePreview asks the "Stored in" preview and returns it ready to send back
// as expect.
func (f *placementFixture) homePreview(domain, placeID string) map[string]any {
	f.t.Helper()
	res := f.do(http.MethodPost, "/api/storage/domains/"+domain+"/home/preview", map[string]any{"placeId": placeID})
	if res["ok"] != true {
		f.t.Fatalf("home preview = %v", res)
	}
	delete(res, "ok")
	return res
}

func (f *placementFixture) homeRefusal(domain, placeID string) any {
	f.t.Helper()
	res := f.do(http.MethodPost, "/api/storage/domains/"+domain+"/home/preview", map[string]any{"placeId": placeID})
	if res["ok"] != false {
		f.t.Fatalf("home preview = %v, want a refusal", res)
	}
	return res["code"]
}

func TestADomainWithoutBackupsMovesItsHomePlace(t *testing.T) {
	f := newPlacementFixture(t)
	unraid := f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	nas := f.storePlace(localPlace("NAS", "nas"))
	if pv := f.homePreview("containers", nas.ID); pv["mode"] != "home-place" || pv["homePlace"] != unraid.ID || pv["homeHasBackups"] != false {
		t.Fatalf("preview = %v", pv)
	}
}

func TestADomainWithBackupsSetsItsDefaultInstead(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	nas := f.storePlace(localPlace("NAS", "nas"))
	f.container("nginx", "")
	f.openContainer("plex")
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:nginx"))

	pv := f.homePreview("containers", nas.ID)

	impact, _ := pv["impact"].(map[string]any)
	if pv["mode"] != "default" || pv["creates"] != "repository" || pv["homeHasBackups"] != true || impact["openTakeHome"] != float64(1) {
		t.Fatalf("preview = %v, want the default, a repository made on the choice, plex taking it", pv)
	}
	if rows, err := f.st.PlaceRows(nas.ID); err != nil || len(rows) != 0 {
		t.Fatalf("rows at NAS = %v, %v, want none: a preview makes nothing", rows, err)
	}
}

func TestADomainWithBackupsStoresBesideItsTargetAtThePlace(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	f.placeTarget(b2, "containers", "")
	f.container("nginx", "")
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:nginx"))

	if pv := f.homePreview("containers", b2.ID); pv["mode"] != "default" || pv["creates"] != "direct" {
		t.Fatalf("preview = %v, want the direct repository beside the B2 target", pv)
	}
}

func TestFlashWithBackupsMovesItsHomeAndCountsWhatStays(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.storePlace(localPlace("NAS", "nas"))
	f.hold(f.singletonRepo("flash"), snap("f1", 100, "flash"))
	if err := f.st.AddRepoStat(store.RepoStat{Domain: "flash", Source: "local", At: 100, Snapshots: 3}); err != nil {
		t.Fatal(err)
	}
	if pv := f.homePreview("flash", nas.ID); pv["mode"] != "home-move" || pv["backups"] != float64(3) {
		t.Fatalf("preview = %v, want a home move leaving 3 snapshots", pv)
	}
}

func TestAHomeWhereATargetOfTheDomainLiesIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.storePlace(localPlace("NAS", "nas"))
	f.placeTarget(nas, "flash", "")
	if code := f.homeRefusal("flash", nas.ID); code != "place-address-taken" {
		t.Fatalf("code = %v, want place-address-taken", code)
	}
}

func TestAPlaceWithoutTheDomainsFolderCannotBeItsHome(t *testing.T) {
	f := newPlacementFixture(t)
	p := localPlace("NAS", "nas")
	p.Folders = map[string]string{"vms": "vms"}
	nas := f.storePlace(p)
	if code := f.homeRefusal("containers", nas.ID); code != "place-domain-unavailable" {
		t.Fatalf("code = %v, want place-domain-unavailable", code)
	}
}

func TestASwitchedOffPlaceCannotBeAHome(t *testing.T) {
	f := newPlacementFixture(t)
	p := localPlace("NAS", "nas")
	p.Enabled = false
	nas := f.storePlace(p)
	if code := f.homeRefusal("containers", nas.ID); code != "place-off" {
		t.Fatalf("code = %v, want place-off", code)
	}
}

func TestAnUnreadableDomainPathKeepsItsHome(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	nas := f.storePlace(localPlace("NAS", "nas"))
	f.eng.listErr[f.domainPath("containers")] = errors.New("share not mounted")
	if code := f.homeRefusal("containers", nas.ID); code != "home-uncheckable" {
		t.Fatalf("code = %v, want home-uncheckable", code)
	}
}

func TestAHomePreviewOfAnUnknownPlaceIsNotFound(t *testing.T) {
	f := newPlacementFixture(t)
	code, res := f.doStatus(http.MethodPost, "/api/storage/domains/containers/home/preview", map[string]any{"placeId": "nope"})
	if code != http.StatusNotFound || res["ok"] != false {
		t.Fatalf("preview = %d %v, want 404", code, res)
	}
	if code, _ := f.doStatus(http.MethodPost, "/api/storage/domains/nope/home/preview", map[string]any{"placeId": "x"}); code != http.StatusBadRequest {
		t.Fatalf("unknown domain = %d, want 400", code)
	}
}

func (f *placementFixture) putHome(domain, placeID string, expect map[string]any, applyToOpen bool) map[string]any {
	f.t.Helper()
	return f.do(http.MethodPut, "/api/storage/domains/"+domain+"/home",
		map[string]any{"placeId": placeID, "expect": expect, "applyToOpen": applyToOpen})
}

func TestChoosingAPlaceForADomainWithoutBackupsMakesItTheHome(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	nas := f.storePlace(localPlace("NAS", "nas"))
	old := f.namedRepo("Old NAS", "oldnas")
	f.setDefault("containers", old.ID, store.SkipAll)

	res := f.putHome("containers", nas.ID, f.homePreview("containers", nas.ID), false)

	homes, err := f.st.DomainPlaces()
	if res["ok"] != true || err != nil || homes["containers"] != nas.ID {
		t.Fatalf("PUT = %v; homes %v, %v", res, homes, err)
	}
	if settings, err := f.st.GetSettings(); err != nil || settings.ContainersPath != "nas/containers" {
		t.Fatalf("containers path = %q, %v", settings.ContainersPath, err)
	}
	if d, _, err := f.st.PlacementDefaultFor("containers"); err != nil || d.Home != "" || !reflect.DeepEqual(d.Skip, []string{store.SkipAll}) {
		t.Fatalf("default = %+v, %v, want it back on the domain path with its skip", d, err)
	}
}

func TestChoosingAPlaceForADomainWithBackupsPointsItsDefaultThere(t *testing.T) {
	f := newPlacementFixture(t)
	unraid := f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	nas := f.storePlace(localPlace("NAS", "nas"))
	f.container("nginx", "")
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:nginx"))

	res := f.putHome("containers", nas.ID, f.homePreview("containers", nas.ID), false)

	d, _, err := f.st.PlacementDefaultFor("containers")
	if res["ok"] != true || err != nil || d.Home == "" {
		t.Fatalf("PUT = %v; default %+v, %v", res, d, err)
	}
	if repo, err := f.st.GetNamedRepo(d.Home); err != nil || repo.PlaceID != nas.ID || repo.Repo != "nas/containers" {
		t.Fatalf("default home = %+v, %v, want the NAS repository of containers", repo, err)
	}
	if homes, err := f.st.DomainPlaces(); err != nil || homes["containers"] != unraid.ID {
		t.Fatalf("homes = %v, %v, want Unraid kept", homes, err)
	}
}

func TestApplyToOpenResetsTheItemsWithoutBackups(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	nas := f.storePlace(localPlace("NAS", "nas"))
	f.container("nginx", "")
	f.container("plex", "")
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:nginx"))

	res := f.putHome("containers", nas.ID, f.homePreview("containers", nas.ID), true)

	plex := f.home(store.ItemRef{Domain: "containers", Key: "plex"})
	nginx := f.home(store.ItemRef{Domain: "containers", Key: "nginx"})
	if res["ok"] != true || !reflect.DeepEqual(res["reset"], []any{"plex"}) || plex.Choice != store.RepoOpen || nginx.Choice != store.RepoChosen {
		t.Fatalf("PUT = %v; plex %+v, nginx %+v", res, plex, nginx)
	}
}

func TestStoredInHoldsTheDomainFromItsPreviewToItsLastReset(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	nas := f.storePlace(localPlace("NAS", "nas"))
	f.container("nginx", "")
	f.container("plex", "")
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:nginx"))
	pv := f.homePreview("containers", nas.ID)
	free := 0
	f.eng.onSnapshots = func() {
		if unlock, ok := f.svc.tryLockDomainFor("containers", "backup"); ok {
			free++
			unlock()
		}
	}

	res := f.putHome("containers", nas.ID, pv, true)

	if res["ok"] != true || free != 0 {
		t.Fatalf("PUT = %v; a backup could have started at %d of its listings", res, free)
	}
}

func TestAStaleStoredInAnswerIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	unraid := f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	nas := f.storePlace(localPlace("NAS", "nas"))
	pv := f.homePreview("containers", nas.ID)
	pv["mode"] = "default"

	res := f.putHome("containers", nas.ID, pv, false)

	if res["ok"] != false || res["code"] != "stale" || res["preview"].(map[string]any)["mode"] != "home-place" {
		t.Fatalf("PUT = %v, want stale with a fresh preview", res)
	}
	if homes, err := f.st.DomainPlaces(); err != nil || homes["containers"] != unraid.ID {
		t.Fatalf("homes = %v, %v, want them unchanged", homes, err)
	}
}

func TestAStoredInAnswerWithStaleNumbersIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	nas := f.storePlace(localPlace("NAS", "nas"))
	f.container("nginx", "")
	f.openContainer("plex")
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:nginx"))
	pv := f.homePreview("containers", nas.ID)
	f.openContainer("immich")

	res := f.putHome("containers", nas.ID, pv, false)

	fresh, _ := res["preview"].(map[string]any)
	impact, _ := fresh["impact"].(map[string]any)
	if res["ok"] != false || res["code"] != "stale" || impact["openTakeHome"] != float64(2) {
		t.Fatalf("PUT = %v, want stale with plex and immich counted", res)
	}
	if d, found, err := f.st.PlacementDefaultFor("containers"); err != nil || (found && d.Home != "") {
		t.Fatalf("default = %+v, %v, %v, want it unchanged", d, found, err)
	}
}

func TestADefaultThatMovesDuringTheWriteAnswersStaleWithTheFreshPreview(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	nas := f.storePlace(localPlace("NAS", "nas"))
	old := f.namedRepo("Old NAS", "oldnas")
	f.container("nginx", "")
	f.openContainer("plex")
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:nginx"))
	pv := f.homePreview("containers", nas.ID)
	listings := 0
	f.eng.onSnapshots = func() {
		// The second listing of the write counts plex for its preview, which
		// has read the default by then.
		if listings++; listings == 2 {
			f.setDefault("containers", old.ID)
		}
	}

	res := f.putHome("containers", nas.ID, pv, false)

	fresh, _ := res["preview"].(map[string]any)
	impact, _ := fresh["impact"].(map[string]any)
	if res["ok"] != false || res["code"] != "stale" || impact["home"] != old.ID || fresh["repoId"] == nil || fresh["creates"] != nil {
		t.Fatalf("PUT = %v, want stale with the preview of the moved default", res)
	}
	if d, _, err := f.st.PlacementDefaultFor("containers"); err != nil || d.Home != old.ID {
		t.Fatalf("default = %+v, %v, want the one that moved in", d, err)
	}
}

func TestFlashMovesItsHomeAndLeavesItsBackupsBehind(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.storePlace(localPlace("NAS", "nas"))
	f.hold(f.singletonRepo("flash"), snap("f1", 100, "flash"))

	res := f.putHome("flash", nas.ID, f.homePreview("flash", nas.ID), false)

	homes, err := f.st.DomainPlaces()
	if res["ok"] != true || err != nil || homes["flash"] != nas.ID {
		t.Fatalf("PUT = %v; homes %v, %v", res, homes, err)
	}
	if settings, err := f.st.GetSettings(); err != nil || settings.FlashPath != "nas/flash" {
		t.Fatalf("flash path = %q, %v", settings.FlashPath, err)
	}
}

func TestAStoredInChangeWaitsForARunningBackup(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	nas := f.storePlace(localPlace("NAS", "nas"))
	pv := f.homePreview("containers", nas.ID)
	unlock, ok := f.svc.tryLockDomainFor("containers", "backup")
	if !ok {
		t.Fatal("the containers lock is taken")
	}
	defer unlock()
	if res := f.putHome("containers", nas.ID, pv, false); res["ok"] != false || res["code"] != "domain-busy" {
		t.Fatalf("PUT = %v, want domain-busy", res)
	}
}

func TestAStoredInChangeWaitsForAnEditOfThePlace(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	nas := f.storePlace(localPlace("NAS", "nas"))
	pv, err := f.svc.domainHomePreview(context.Background(), "containers", nas.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.svc.placeEditMu.Lock()
	done := make(chan error, 1)
	go func() {
		_, _, _, err := f.svc.setDomainHome(context.Background(), "containers", domainHomeBody{PlaceID: nas.ID, Expect: &pv})
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("the home moved during an edit of its place: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	f.svc.placeEditMu.Unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the home never moved")
	}
}

func TestAStoredInChangeToAnUnknownPlaceIsNotFound(t *testing.T) {
	f := newPlacementFixture(t)
	code, res := f.doStatus(http.MethodPut, "/api/storage/domains/containers/home",
		map[string]any{"placeId": "nope", "expect": map[string]any{"mode": "home-place", "placeId": "nope"}, "applyToOpen": false})
	if code != http.StatusNotFound || res["ok"] != false {
		t.Fatalf("PUT = %d %v, want 404", code, res)
	}
}

// copiesPreview asks a chip's preview and returns it ready to send back as expect.
func (f *placementFixture) copiesPreview(domain, placeID string, on bool) map[string]any {
	f.t.Helper()
	res := f.do(http.MethodPost, "/api/storage/domains/"+domain+"/copies/preview", map[string]any{"placeId": placeID, "on": on})
	if res["ok"] != true {
		f.t.Fatalf("copies preview = %v", res)
	}
	delete(res, "ok")
	return res
}

func (f *placementFixture) putCopies(domain, placeID string, on bool, expect map[string]any) map[string]any {
	f.t.Helper()
	return f.do(http.MethodPut, "/api/storage/domains/"+domain+"/copies", map[string]any{"placeId": placeID, "on": on, "expect": expect})
}

func (f *placementFixture) defaultSkip(domain string) []string {
	f.t.Helper()
	d, _, err := f.st.PlacementDefaultFor(domain)
	if err != nil {
		f.t.Fatal(err)
	}
	return d.Skip
}

func TestAChipOnWithoutATargetCreatesItAtThePlace(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	f.container("nginx", "")
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:nginx"))

	pv := f.copiesPreview("containers", b2.ID, true)
	if pv["targetId"] != nil || pv["newTarget"].(map[string]any)["items"] != float64(1) {
		t.Fatalf("preview = %v, want a new target that nginx goes to", pv)
	}
	if res := f.putCopies("containers", b2.ID, true, pv); res["ok"] != true {
		t.Fatalf("PUT = %v", res)
	}
	targets, err := f.st.OffsiteTargetsForDomain("containers")
	if err != nil || len(targets) != 1 || targets[0].PlaceID != b2.ID || targets[0].Repo != "s3:https://s3.example.com/bucket/container" || !targets[0].Enabled {
		t.Fatalf("targets = %+v, %v", targets, err)
	}
	if chip := chipOf(f.domainRow("containers"), b2.ID); chip["on"] != true || chip["targetId"] != targets[0].ID {
		t.Fatalf("chip = %v, want it on with its new target", chip)
	}
}

func TestAChipOnUnderSkipAllLeavesOutEveryOtherEnabledTarget(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	hz := f.target("containers", "Hetzner", "sftp:u1@hz.example:/bv")
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	f.placeTarget(b2, "containers", "")
	f.setDefault("containers", "", store.SkipAll)

	pv := f.copiesPreview("containers", b2.ID, true)
	if !reflect.DeepEqual(pv["skip"], []any{hz.ID}) {
		t.Fatalf("skip = %v, want only Hetzner left out", pv["skip"])
	}
	if res := f.putCopies("containers", b2.ID, true, pv); res["ok"] != true {
		t.Fatalf("PUT = %v", res)
	}
	if skip := f.defaultSkip("containers"); !slices.Equal(skip, []string{hz.ID}) {
		t.Fatalf("default skip = %v", skip)
	}
}

func TestTheLastChipOffSkipsEverything(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	hz := f.target("containers", "Hetzner", "sftp:u1@hz.example:/bv")
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	target := f.placeTarget(b2, "containers", "")
	f.setDefault("containers", "", hz.ID)

	pv := f.copiesPreview("containers", b2.ID, false)
	if !reflect.DeepEqual(pv["skip"], []any{store.SkipAll}) {
		t.Fatalf("skip = %v, want everything left out", pv["skip"])
	}
	if res := f.putCopies("containers", b2.ID, false, pv); res["ok"] != true {
		t.Fatalf("PUT = %v", res)
	}
	if skip := f.defaultSkip("containers"); !slices.Equal(skip, []string{store.SkipAll}) {
		t.Fatalf("default skip = %v", skip)
	}
	if row, _, err := f.st.GetOffsiteTarget(target.ID); err != nil || !row.Enabled {
		t.Fatalf("target = %+v, %v, want it kept and on", row, err)
	}
}

func TestAChipOffLeavesTheOtherTargetsIn(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	f.target("containers", "Hetzner", "sftp:u1@hz.example:/bv")
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	target := f.placeTarget(b2, "containers", "")

	pv := f.copiesPreview("containers", b2.ID, false)
	if !reflect.DeepEqual(pv["skip"], []any{target.ID}) || pv["impact"] == nil {
		t.Fatalf("preview = %v, want only B2 left out, with what that drops", pv)
	}
	if res := f.putCopies("containers", b2.ID, false, pv); res["ok"] != true {
		t.Fatalf("PUT = %v", res)
	}
	if skip := f.defaultSkip("containers"); !slices.Equal(skip, []string{target.ID}) {
		t.Fatalf("default skip = %v", skip)
	}
}

func TestAStaleChipIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	f.placeTarget(b2, "containers", "")
	pv := f.copiesPreview("containers", b2.ID, false)
	pv["skip"] = []string{}
	if res := f.putCopies("containers", b2.ID, false, pv); res["ok"] != false || res["code"] != "stale" || res["preview"] == nil {
		t.Fatalf("PUT = %v, want stale with the preview", res)
	}
	if _, found, err := f.st.PlacementDefaultFor("containers"); err != nil || found {
		t.Fatalf("a stale chip wrote the default: %v, %v", found, err)
	}
}

func TestAChipBesideTheDomainsRepositoryGetsTheCopiesEnding(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	nas := f.storePlace(localPlace("NAS", "nas"))
	if _, err := f.st.CreatePlaceRepo(nas.ID, "containers", ""); err != nil {
		t.Fatal(err)
	}

	pv := f.copiesPreview("containers", nas.ID, true)
	if pv["suffix"] != "-copies" {
		t.Fatalf("preview = %v, want the -copies ending", pv)
	}
	if res := f.putCopies("containers", nas.ID, true, pv); res["ok"] != true {
		t.Fatalf("PUT = %v", res)
	}
	targets, err := f.st.OffsiteTargetsForDomain("containers")
	if err != nil || len(targets) != 1 || targets[0].Repo != "nas/containers-copies" || targets[0].PlaceSuffix != "-copies" {
		t.Fatalf("targets = %+v, %v", targets, err)
	}
}

func (f *placementFixture) chipRefusal(domain, placeID string, on bool) any {
	f.t.Helper()
	res := f.do(http.MethodPost, "/api/storage/domains/"+domain+"/copies/preview", map[string]any{"placeId": placeID, "on": on})
	if res["ok"] != false {
		f.t.Fatalf("copies preview = %v, want a refusal", res)
	}
	return res["code"]
}

func TestTheHomePlaceTakesNoChip(t *testing.T) {
	f := newPlacementFixture(t)
	unraid := f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	if code := f.chipRefusal("containers", unraid.ID, true); code != "place-home-domain" {
		t.Fatalf("code = %v, want place-home-domain", code)
	}
}

func TestTheDefaultsPlaceTakesNoChip(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	nas := f.storePlace(localPlace("NAS", "nas"))
	named, err := f.st.CreatePlaceRepo(nas.ID, "containers", "")
	if err != nil {
		t.Fatal(err)
	}
	f.setDefault("containers", named.ID)
	if code := f.chipRefusal("containers", nas.ID, true); code != "place-home-domain" {
		t.Fatalf("code = %v, want place-home-domain", code)
	}
}

func TestASwitchedOffPlaceTakesNoNewChip(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	p := s3Place("B2", "s3:https://s3.example.com/bucket")
	p.Enabled = false
	b2 := f.storePlace(p)
	if code := f.chipRefusal("containers", b2.ID, true); code != "place-off" {
		t.Fatalf("code = %v, want place-off", code)
	}
}

func TestAPlaceWithoutTheDomainsFolderTakesNoChip(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	p := s3Place("B2", "s3:https://s3.example.com/bucket")
	p.Folders = map[string]string{"vms": "vms"}
	b2 := f.storePlace(p)
	if code := f.chipRefusal("containers", b2.ID, true); code != "place-domain-unavailable" {
		t.Fatalf("code = %v, want place-domain-unavailable", code)
	}
}

func TestAPlaceThatIsARepositoryTakesNoTargetBesideIt(t *testing.T) {
	f := newPlacementFixture(t)
	p := s3Place("B2 root", "s3:https://s3.example.com/bucket")
	p.Folders = map[string]string{"containers": ""}
	root := f.storePlace(p)
	if _, err := f.st.CreatePlaceRepo(root.ID, "containers", ""); err != nil {
		t.Fatal(err)
	}
	if code := f.chipRefusal("containers", root.ID, true); code != "place-is-repository" {
		t.Fatalf("code = %v, want place-is-repository", code)
	}
}

func TestAChipOnSwitchesItsStoppedTargetBackOn(t *testing.T) {
	f := newPlacementFixture(t)
	f.storePlace(localPlace("Unraid", "backups"), "containers", "vms", "files")
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	target := f.placeTarget(b2, "containers", "")
	target.Enabled = false
	if _, err := f.st.UpsertOffsiteTarget(target); err != nil {
		t.Fatal(err)
	}
	if chip := chipOf(f.domainRow("containers"), b2.ID); chip["on"] != false {
		t.Fatalf("chip = %v, want it off while its target is stopped", chip)
	}
	pv := f.copiesPreview("containers", b2.ID, true)
	if pv["newTarget"] == nil {
		t.Fatalf("preview = %v, want what the target receives once it runs again", pv)
	}
	if res := f.putCopies("containers", b2.ID, true, pv); res["ok"] != true {
		t.Fatalf("PUT = %v", res)
	}
	if row, _, err := f.st.GetOffsiteTarget(target.ID); err != nil || !row.Enabled {
		t.Fatalf("target = %+v, %v, want it switched on", row, err)
	}
	if chip := chipOf(f.domainRow("containers"), b2.ID); chip["on"] != true {
		t.Fatalf("chip = %v, want it on", chip)
	}
}

func TestAFlashChipSwitchesItsTargetRow(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	target := f.placeTarget(b2, "flash", "")

	if res := f.putCopies("flash", b2.ID, false, f.copiesPreview("flash", b2.ID, false)); res["ok"] != true {
		t.Fatalf("PUT off = %v", res)
	}
	if row, _, err := f.st.GetOffsiteTarget(target.ID); err != nil || row.Enabled {
		t.Fatalf("target = %+v, %v, want it off", row, err)
	}
	if res := f.putCopies("flash", b2.ID, true, f.copiesPreview("flash", b2.ID, true)); res["ok"] != true {
		t.Fatalf("PUT on = %v", res)
	}
	if row, _, err := f.st.GetOffsiteTarget(target.ID); err != nil || !row.Enabled {
		t.Fatalf("target = %+v, %v, want it on", row, err)
	}
}

func TestANewFlashTargetTakesTheFieldSlot(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	if res := f.putCopies("flash", b2.ID, true, f.copiesPreview("flash", b2.ID, true)); res["ok"] != true {
		t.Fatalf("PUT = %v", res)
	}
	targets, err := f.st.OffsiteTargetsForDomain("flash")
	if err != nil || len(targets) != 1 || targets[0].SortOrder != 0 {
		t.Fatalf("targets = %+v, %v", targets, err)
	}
	if settings, err := f.st.GetSettings(); err != nil || settings.FlashOffsite != targets[0].Repo {
		t.Fatalf("flash field = %q, %v, want the new target", settings.FlashOffsite, err)
	}
}

func TestAFlashChipSwitchedOffStaysOffWhenThePlaceIsEdited(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	target := f.placeTarget(b2, "flash", "")
	if res := f.putCopies("flash", b2.ID, false, f.copiesPreview("flash", b2.ID, false)); res["ok"] != true {
		t.Fatalf("PUT off = %v", res)
	}
	if res := f.do(http.MethodPatch, "/api/places/"+b2.ID, map[string]any{"name": "B2 EU"}); res["ok"] != true {
		t.Fatalf("PATCH = %v", res)
	}
	if row, _, err := f.st.GetOffsiteTarget(target.ID); err != nil || row.Enabled {
		t.Fatalf("target = %+v, %v, want it still off after an edit of its place", row, err)
	}
}

func TestAChipOfAnUnknownPlaceIsNotFound(t *testing.T) {
	f := newPlacementFixture(t)
	if code, res := f.doStatus(http.MethodPost, "/api/storage/domains/containers/copies/preview",
		map[string]any{"placeId": "nope", "on": true}); code != http.StatusNotFound || res["ok"] != false {
		t.Fatalf("preview = %d %v, want 404", code, res)
	}
	if code, res := f.doStatus(http.MethodPut, "/api/storage/domains/containers/copies",
		map[string]any{"placeId": "nope", "on": true, "expect": map[string]any{"placeId": "nope", "on": true}}); code != http.StatusNotFound || res["ok"] != false {
		t.Fatalf("PUT = %d %v, want 404", code, res)
	}
	if code, _ := f.doStatus(http.MethodPost, "/api/storage/domains/nope/copies/preview", map[string]any{"placeId": "x", "on": true}); code != http.StatusBadRequest {
		t.Fatalf("unknown domain = %d, want 400", code)
	}
}
