package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// vaultwardenAtB2 is a container on the domain path with two copies at B2: b1 of
// a1, which the home still holds, and b9 of a9, which exists only at B2.
func vaultwardenAtB2(t *testing.T) (*placementFixture, store.OffsiteTarget) {
	t.Helper()
	f := newPlacementFixture(t)
	f.container("vaultwarden", "")
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.hold(f.domainPath("containers"), snap("a1", 1_758_000_000, "container:vaultwarden"))
	f.hold("b2:bucket:containers",
		copied("b1", "a1", 1_758_000_000, "container:vaultwarden"),
		copied("b9", "a9", 1_756_700_000, "container:vaultwarden"),
		copied("c1", "x1", 1_758_000_000, "container:nginx"),
	)
	f.listing("containers", b2.ID, 1_758_100_000,
		copiesRow("container:vaultwarden", 2, 1_758_000_000),
		copiesRow("container:nginx", 1, 1_758_000_000),
	)
	return f, b2
}

func removalPath(targetID string) string {
	return "/api/items/containers/vaultwarden/offsite/" + targetID + "/removal"
}

func onlyThereIDs(m map[string]any) []string {
	var ids []string
	for _, row := range m["onlyThere"].([]any) {
		ids = append(ids, row.(map[string]any)["id"].(string))
	}
	slices.Sort(ids)
	return ids
}

func TestRemovalPreviewNamesWhatExistsOnlyAtTheTarget(t *testing.T) {
	f, b2 := vaultwardenAtB2(t)
	m := f.do("GET", removalPath(b2.ID), nil)
	if m["ok"] != true || m["count"] != float64(2) || m["homeUnreadable"] != false || m["homeLabel"] != "" {
		t.Fatalf("preview = %v", m)
	}
	if got := onlyThereIDs(m); !slices.Equal(got, []string{"b9"}) {
		t.Fatalf("only there = %v, want b9", got)
	}
	target := m["target"].(map[string]any)
	if target["id"] != b2.ID || target["name"] != "B2" || target["appendOnly"] != false {
		t.Fatalf("target = %v", target)
	}
	if f.eng.lists["b2:bucket:containers"] != 1 || f.eng.lists[f.domainPath("containers")] != 1 {
		t.Fatalf("the preview lists B2 once and the home once, got %v", f.eng.lists)
	}
}

func TestRemovalPreviewNamesTheItemTheTypedNameIsComparedWith(t *testing.T) {
	f, b2 := vaultwardenAtB2(t)
	if m := f.do("GET", removalPath(b2.ID), nil); m["name"] != "vaultwarden" {
		t.Fatalf("preview = %v, want the name out of the item's identity", m)
	}
}

func TestRemovalPreviewJudgesAnOpenItemAgainstItsEffectiveHome(t *testing.T) {
	f := newPlacementFixture(t)
	f.openContainer("vaultwarden")
	nas := f.namedRepo("NAS Keller", "nas/bv")
	f.setDefault("containers", nas.ID)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.hold(f.root+"/nas/bv", snap("a1", 1_758_000_000, "container:vaultwarden"))
	f.hold("b2:bucket:containers", copied("b1", "a1", 1_758_000_000, "container:vaultwarden"))

	m := f.do("GET", removalPath(b2.ID), nil)
	if m["homeLabel"] != "NAS Keller" {
		t.Fatalf("preview = %v, want the default's repository named", m)
	}
	if got := onlyThereIDs(m); len(got) != 0 {
		t.Fatalf("only there = %v, want none: the copy of a1 lies at NAS Keller", got)
	}
}

func TestRemovalPreviewCountsEveryCopyWhenTheHomeCannotBeRead(t *testing.T) {
	f, b2 := vaultwardenAtB2(t)
	f.eng.listErr[f.domainPath("containers")] = errors.New("permission denied")
	m := f.do("GET", removalPath(b2.ID), nil)
	if m["homeUnreadable"] != true {
		t.Fatalf("preview = %v, want homeUnreadable", m)
	}
	if got := onlyThereIDs(m); !slices.Equal(got, []string{"b1", "b9"}) {
		t.Fatalf("only there = %v, want every copy", got)
	}
}

func TestRemovalWithNothingOnlyThereNeedsNoName(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("vaultwarden", "")
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.hold(f.domainPath("containers"), snap("a1", 1_758_000_000, "container:vaultwarden"))
	f.hold("b2:bucket:containers", copied("b1", "a1", 1_758_000_000, "container:vaultwarden"))

	m := f.do("DELETE", removalPath(b2.ID), map[string]any{"onlyThere": []string{}, "typedName": ""})
	if m["ok"] != true || m["deleted"] != float64(1) {
		t.Fatalf("delete = %v", m)
	}
}

func TestRemovalOfSnapshotsFoundOnlyThereNeedsTheTypedName(t *testing.T) {
	f, b2 := vaultwardenAtB2(t)
	m := f.do("DELETE", removalPath(b2.ID), map[string]any{"onlyThere": []string{"b9"}, "typedName": ""})
	if m["ok"] != false || m["code"] != "name-mismatch" {
		t.Fatalf("delete without the name = %v, want name-mismatch", m)
	}
	if len(f.eng.deletes) != 0 {
		t.Fatalf("deleted without the name: %+v", f.eng.deletes)
	}

	m = f.do("DELETE", removalPath(b2.ID), map[string]any{"onlyThere": []string{"b9"}, "typedName": "vaultwarden"})
	if m["ok"] != true || m["deleted"] != float64(2) {
		t.Fatalf("delete = %v", m)
	}
	if got := forgottenAt(f, "b2:bucket:containers"); !slices.Equal(got, []string{"b1", "b9"}) {
		t.Fatalf("forgotten = %v, want b1 and b9", got)
	}
	copies, err := f.st.ItemCopiesForDomain("containers")
	if err != nil {
		t.Fatal(err)
	}
	if len(copies) != 1 || copies[0].Identity != "container:nginx" {
		t.Fatalf("observed copies = %+v, want only nginx", copies)
	}
}

// The window deletes everything the ownership rule gives the machine, disk
// images included, and its count says so before anything is deleted.
func TestRemovalOfAVMTakesItsDiskImages(t *testing.T) {
	f := newPlacementFixture(t)
	f.vm("win11", "")
	b2 := f.target("vms", "B2", "b2:bucket:vms")
	f.hold("b2:bucket:vms",
		copied("v1", "a1", 1_758_000_000, "vm:win11", "vmrun:r1"),
		copied("v2", "a2", 1_758_000_000, "vm:win11:zvol:sda", "vmrun:r1"),
	)
	path := "/api/items/vms/win11/offsite/" + b2.ID + "/removal"

	m := f.do("GET", path, nil)
	if m["count"] != float64(2) {
		t.Fatalf("preview = %v, want both snapshots counted", m)
	}
	if got := onlyThereIDs(m); !slices.Equal(got, []string{"v1", "v2"}) {
		t.Fatalf("only there = %v, want both", got)
	}
	m = f.do("DELETE", path, map[string]any{"onlyThere": []string{"v1", "v2"}, "typedName": "win11"})
	if m["ok"] != true || m["deleted"] != float64(2) {
		t.Fatalf("delete = %v", m)
	}
	if got := forgottenAt(f, "b2:bucket:vms"); !slices.Equal(got, []string{"v1", "v2"}) {
		t.Fatalf("forgotten = %v, want the machine and its disk", got)
	}
}

func TestRemovalRefusesAListThatGrew(t *testing.T) {
	f, b2 := vaultwardenAtB2(t)
	m := f.do("DELETE", removalPath(b2.ID), map[string]any{"onlyThere": []string{}, "typedName": "vaultwarden"})
	if m["ok"] != false || m["code"] != "removal-grown" {
		t.Fatalf("delete = %v, want removal-grown", m)
	}
	if got := onlyThereIDs(m["preview"].(map[string]any)); !slices.Equal(got, []string{"b9"}) {
		t.Fatalf("fresh preview = %v, want b9", got)
	}
	if len(f.eng.deletes) != 0 {
		t.Fatalf("a grown list was deleted: %+v", f.eng.deletes)
	}
}

func TestRemovalRefusesWhenTheHomeTurnedUnreadable(t *testing.T) {
	f, b2 := vaultwardenAtB2(t)
	f.do("GET", removalPath(b2.ID), nil)
	f.eng.listErr[f.domainPath("containers")] = errors.New("permission denied")
	m := f.do("DELETE", removalPath(b2.ID), map[string]any{"onlyThere": []string{"b9"}, "typedName": "vaultwarden"})
	if m["ok"] != false || m["code"] != "home-unreadable" {
		t.Fatalf("delete = %v, want home-unreadable", m)
	}
	if len(f.eng.deletes) != 0 {
		t.Fatalf("deleted while the home could not be read: %+v", f.eng.deletes)
	}
}

func TestRemovalAtAnAppendOnlyTargetIsRefused(t *testing.T) {
	f, b2 := vaultwardenAtB2(t)
	f.appendOnly(b2.ID)
	if m := f.do("GET", removalPath(b2.ID), nil); m["target"].(map[string]any)["appendOnly"] != true {
		t.Fatalf("preview = %v, want appendOnly", m)
	}
	m := f.do("DELETE", removalPath(b2.ID), map[string]any{"onlyThere": []string{"b9"}, "typedName": "vaultwarden"})
	if m["ok"] != false || m["code"] != "append-only" {
		t.Fatalf("delete = %v, want append-only", m)
	}
}

func TestRemovalWhileABackupRunsIsRefused(t *testing.T) {
	f, b2 := vaultwardenAtB2(t)
	unlock, ok := f.svc.tryLockDomainFor("containers", "backup")
	if !ok {
		t.Fatal("could not take the domain lock")
	}
	defer unlock()
	m := f.do("DELETE", removalPath(b2.ID), map[string]any{"onlyThere": []string{"b9"}, "typedName": "vaultwarden"})
	if m["ok"] != false || m["code"] != "domain-busy" {
		t.Fatalf("delete = %v, want domain-busy", m)
	}
}

func TestRemovalAtAnUnknownTargetIsRefused(t *testing.T) {
	f, _ := vaultwardenAtB2(t)
	m := f.do("GET", removalPath("0123456789abcdef0123456789abcdef"), nil)
	if m["ok"] != false || m["code"] != "unknown-target" {
		t.Fatalf("preview = %v, want unknown-target", m)
	}
}

func TestRemovalRouteRejectsAMalformedTargetID(t *testing.T) {
	f, _ := vaultwardenAtB2(t)
	rec := httptest.NewRecorder()
	f.h.Router().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, removalPath("not-hex"), nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
