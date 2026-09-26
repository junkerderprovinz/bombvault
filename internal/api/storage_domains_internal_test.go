package api

import (
	"net/http"
	"reflect"
	"testing"

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
