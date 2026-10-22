package api

import (
	"net/http"
	"net/url"
	"testing"
)

type deleteRoute struct {
	method, path string
	body         any
}

// deleteRoutes are the requests that delete stored backups or snapshots for
// good, each aimed at something nginxAtHomeAndB2 and the entries beside it hold.
func deleteRoutes(f *placementFixture, b2ID string) []deleteRoute {
	f.vm("win11", "")
	files := f.fileSet("docs", "")
	tank, _ := f.zfsItem("tank/apps")
	dest := f.destination("Disk", "disk1/backups")
	b2 := url.QueryEscape("offsite:" + b2ID)
	return []deleteRoute{
		{http.MethodDelete, "/api/items/containers/nginx/timeline/a1a1a1a1", map[string]any{"places": []map[string]any{{"place": "offsite:" + b2ID, "label": "B2", "snapshotIds": []string{"b1b1b1b1"}}}}},
		{http.MethodDelete, "/api/snapshots/containers/b1b1b1b1?source=" + b2, nil},
		{http.MethodPost, "/api/prune/containers", nil},
		{http.MethodDelete, "/api/containers/nginx/backups?source=" + b2, nil},
		{http.MethodDelete, "/api/vms/win11/backups", nil},
		{http.MethodDelete, "/api/files/sets/" + files.ID + "/backups", nil},
		{http.MethodDelete, "/api/zfs/datasets/" + tank.ID + "/backups", nil},
		{http.MethodDelete, "/api/zfs/datasets/" + tank.ID + "?safety=true", nil},
		{http.MethodDelete, "/api/zfs/datasets/" + tank.ID + "/safety-snapshots?dataset=tank/apps&name=bombvault-safety-1", nil},
		{http.MethodDelete, "/api/items/containers/nginx/offsite/" + b2ID + "/removal", map[string]any{"onlyThere": []string{"a7a7a7a7"}, "typedName": "nginx"}},
		{http.MethodGet, "/api/offsite/destinations/" + dest.ID + "/erasure", nil},
		{http.MethodPost, "/api/offsite/destinations/" + dest.ID + "/erasure", map[string]any{}},
	}
}

func TestDeletingBackupsForGoodNeedsALoginPassword(t *testing.T) {
	f, b2 := nginxAtHomeAndB2(t)
	routes := deleteRoutes(f, b2.ID)
	for _, r := range routes {
		code, out := f.call(r.method, r.path, r.body)
		if code != http.StatusForbidden || out["code"] != codeNoPassword {
			t.Errorf("%s %s without a password: %d %v", r.method, r.path, code, out)
		}
	}
	if len(f.eng.deletes) != 0 {
		t.Fatalf("forgotten without a password: %v", f.eng.deletes)
	}

	f.signIn()
	timeline := routes[0]
	if m := f.do(timeline.method, timeline.path, timeline.body); m["ok"] != true {
		t.Fatalf("delete signed in = %v", m)
	}
}

func TestAPairedPhoneCannotDeleteBackups(t *testing.T) {
	f, b2 := nginxAtHomeAndB2(t)
	routes := deleteRoutes(f, b2.ID)
	f.withPassword("a password of the owner")
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	f.session = listedPhoneSession(t, f.st, f.svc.cfg.AppKey, settings.AuthPasswordHash, settings.SessionEpoch)
	for _, r := range routes {
		code, out := f.call(r.method, r.path, r.body)
		if code != http.StatusForbidden || out["code"] != codeOwnLoginNeeded {
			t.Errorf("%s %s from a paired phone: %d %v", r.method, r.path, code, out)
		}
	}
	if len(f.eng.deletes) != 0 {
		t.Fatalf("forgotten for a paired phone: %v", f.eng.deletes)
	}
}

// The page turns the code into a sentence; without it the person reads only
// that the server refused.
func TestAZFSDeleteRefusedWhileARunHoldsTheSectionSaysSo(t *testing.T) {
	f := newPlacementFixture(t)
	tank, _ := f.zfsItem("tank/apps")
	f.svc.zfs = &fakeZFSHost{}
	f.signIn()
	unlock, ok := f.svc.tryLockDomainFor(zfsDomain, "backup")
	if !ok {
		t.Fatal("zfs is locked already")
	}
	defer unlock()
	for _, path := range []string{
		"/api/zfs/datasets/" + tank.ID,
		"/api/zfs/datasets/" + tank.ID + "/backups",
		"/api/zfs/datasets/" + tank.ID + "/safety-snapshots?dataset=tank/apps&name=bombvault-prerestore-20260101000000",
	} {
		if code, out := f.call(http.MethodDelete, path, nil); code != http.StatusConflict || out["code"] != codeDomainBusy {
			t.Errorf("DELETE %s while a backup runs: %d %v, want the busy code", path, code, out)
		}
	}
}

func TestRemovingAZFSEntryThatKeepsItsSnapshotsNeedsNoPassword(t *testing.T) {
	f := newPlacementFixture(t)
	tank, _ := f.zfsItem("tank/apps")
	if code, out := f.call(http.MethodDelete, "/api/zfs/datasets/"+tank.ID, nil); out["code"] == codeNoPassword {
		t.Fatalf("removing the entry alone: %d %v", code, out)
	}
}
