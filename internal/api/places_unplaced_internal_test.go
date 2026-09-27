package api

import (
	"net/http"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// unplacedRow finds a row in the unplaced group of GET /api/places by its id,
// or a domain path by its domain.
func (f *placementFixture) unplacedRow(rowID, domain string) map[string]any {
	f.t.Helper()
	for _, r := range rowsOf(f.do(http.MethodGet, "/api/places", nil)["unplaced"]) {
		if r["rowId"] == rowID && (rowID != "" || r["domain"] == domain) {
			return r
		}
	}
	f.t.Fatalf("no row %q %q without a place", rowID, domain)
	return nil
}

// switchUnplaced sends the append-only switch of a row without a place.
func (f *placementFixture) switchUnplaced(rowID, domain string, on bool) (int, map[string]any) {
	f.t.Helper()
	return f.doStatus(http.MethodPatch, "/api/places/unplaced", map[string]any{"rowId": rowID, "domain": domain, "immutable": on})
}

func TestARowWithoutAPlaceTakesAnAppendOnlySwitchWhereTheFlagMeansSomething(t *testing.T) {
	f := newPlacementFixture(t)
	f.settings(func(s *store.Settings) { s.VMsPath = "b2:bucket:vms" })
	if _, err := f.st.UpsertPrimaryRemoteTarget("vms", store.OffsiteTarget{Repo: "b2:bucket:vms", Immutable: true, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.appendOnly(b2.ID)
	direct := f.direct(b2)
	disk := f.target("files", "Old disk", "remotes/old/files")
	flagged := f.target("files", "Older disk", "remotes/older/files")
	f.appendOnly(flagged.ID)
	archive := f.namedRepo("Archive", "gs:bucket:archive")

	for _, c := range []struct {
		name                   string
		rowID, domain          string
		immutable, protectable bool
	}{
		{"a remote domain path", "", "vms", true, true},
		{"a local domain path", "", "containers", false, false},
		{"a remote target", b2.ID, "", true, true},
		{"a direct repository, which follows its target", direct.ID, "", true, false},
		{"a local target", disk.ID, "", false, false},
		{"a local target flagged append-only, so the flag can go off", flagged.ID, "", true, true},
		{"a remote named repository", archive.ID, "", false, true},
	} {
		row := f.unplacedRow(c.rowID, c.domain)
		if row["immutable"] != c.immutable || row["protectable"] != c.protectable {
			t.Errorf("%s: immutable %v, protectable %v, want %v, %v", c.name, row["immutable"], row["protectable"], c.immutable, c.protectable)
		}
	}
}

func TestARowWithoutAPlaceCountsTheItemsWhoseBackupsGoThere(t *testing.T) {
	f := newPlacementFixture(t)
	archive := f.namedRepo("Archive", "gs:bucket:archive")
	f.container("web", "")
	f.container("db", "")
	f.container("nginx", archive.ID)
	if _, err := f.st.WritePlacement(store.ItemRef{Domain: "containers", Key: "cache"}, &store.HomeWrite{Choice: store.RepoOpen}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.PutPlacementDefault("containers", archive.ID, nil); err != nil {
		t.Fatal(err)
	}
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	if err := f.st.AdjustItemCopies("containers", b2.ID, 100, []store.ItemCopies{
		copiesRow("container:web", 3, 100), copiesRow("container:db", 1, 100),
	}); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		name          string
		rowID, domain string
		items         float64
	}{
		{"a domain path, without the item an open default sends elsewhere", "", "containers", 2},
		{"the flash path", "", "flash", 1},
		{"a named repository", archive.ID, "", 1},
		{"a target", b2.ID, "", 2},
	} {
		if got := f.unplacedRow(c.rowID, c.domain)["items"]; got != c.items {
			t.Errorf("%s: items %v, want %v", c.name, got, c.items)
		}
	}
}

func TestTheAppendOnlySwitchOfARowWithoutAPlaceWritesTheRowsFlag(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	archive := f.namedRepo("Archive", "gs:bucket:archive")

	if code, res := f.switchUnplaced(b2.ID, "", true); code != http.StatusOK || res["ok"] != true {
		t.Fatalf("switch a target = %d %v", code, res)
	}
	if !f.storedTarget(b2.ID).Immutable {
		t.Error("the target is not append-only")
	}
	if code, res := f.switchUnplaced(archive.ID, "", true); code != http.StatusOK || res["ok"] != true {
		t.Fatalf("switch a named repository = %d %v", code, res)
	}
	if got, _ := f.st.GetNamedRepo(archive.ID); !got.Immutable {
		t.Error("the named repository is not append-only")
	}
	if f.unplacedRow(b2.ID, "")["immutable"] != true {
		t.Error("the list does not show the switch on")
	}
}

func TestTheAppendOnlySwitchOfARemoteDomainPathGuardsItsRepository(t *testing.T) {
	f := newPlacementFixture(t)
	f.settings(func(s *store.Settings) { s.VMsPath = "b2:bucket:vms" })

	if code, res := f.switchUnplaced("", "vms", true); code != http.StatusOK || res["ok"] != true {
		t.Fatalf("switch on = %d %v", code, res)
	}
	if got := f.svc.primaryAppendOnly("vms", "b2:bucket:vms"); got != appendOnlyPrimaryRemote {
		t.Fatalf("after switching on: %v, want the primary row's flag", got)
	}
	if code, res := f.switchUnplaced("", "vms", false); code != http.StatusOK || res["ok"] != true {
		t.Fatalf("switch off = %d %v", code, res)
	}
	if got := f.svc.primaryAppendOnly("vms", "b2:bucket:vms"); got != appendOnlyNone {
		t.Fatalf("after switching off: %v, want none", got)
	}
}

func TestTheSwitchOfAFieldTargetSurvivesTheNextSettingsSave(t *testing.T) {
	f := newPlacementFixture(t)
	field := f.fieldTarget("files", "b2:bucket:files")

	if code, res := f.switchUnplaced(field.ID, "", true); code != http.StatusOK || res["ok"] != true {
		t.Fatalf("switch = %d %v", code, res)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.syncPrimaryOffsiteTarget("files", settings); err != nil {
		t.Fatal(err)
	}
	if !f.storedTarget(field.ID).Immutable {
		t.Fatal("a settings save switched the field target's append-only back off")
	}
}

func TestTheAppendOnlySwitchOfARowWithoutAPlaceRefusesWhatItCannotHold(t *testing.T) {
	f := newPlacementFixture(t)
	disk := f.target("files", "Old disk", "remotes/old/files")
	flagged := f.target("files", "Older disk", "remotes/older/files")
	f.appendOnly(flagged.ID)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	direct := f.direct(b2)

	for _, c := range []struct {
		name          string
		rowID, domain string
		on            bool
		code          string
	}{
		{"a local target", disk.ID, "", true, "place-no-append-only"},
		{"a local domain path", "", "containers", true, "place-no-append-only"},
		{"a direct repository", direct.ID, "", true, "mirrored-field"},
	} {
		code, res := f.switchUnplaced(c.rowID, c.domain, c.on)
		if code != http.StatusOK || res["ok"] != false || res["code"] != c.code {
			t.Errorf("%s: %d %v, want the refusal %s", c.name, code, res, c.code)
		}
	}
	if f.storedTarget(disk.ID).Immutable {
		t.Error("the refused local target became append-only")
	}
	if code, res := f.switchUnplaced(flagged.ID, "", false); code != http.StatusOK || res["ok"] != true || f.storedTarget(flagged.ID).Immutable {
		t.Errorf("switching a local target's flag off = %d %v", code, res)
	}
}

func TestARowAtAPlaceHasNoSwitchOfItsOwn(t *testing.T) {
	f := newPlacementFixture(t)
	bucket := f.storePlace(s3Place("Bucket", "s3:https://s3.example.com/bucket"), "containers")
	placed := f.placeTarget(bucket, "vms", "")

	for _, c := range []struct {
		name          string
		rowID, domain string
	}{
		{"a target at a place", placed.ID, ""},
		{"the path of a domain with a home place", "", "containers"},
		{"a row that does not exist", "nosuchrow", ""},
	} {
		if code, res := f.switchUnplaced(c.rowID, c.domain, true); code != http.StatusNotFound {
			t.Errorf("%s: %d %v, want 404", c.name, code, res)
		}
	}
	if f.storedTarget(placed.ID).Immutable {
		t.Error("the placed target took a flag of its own")
	}
}
