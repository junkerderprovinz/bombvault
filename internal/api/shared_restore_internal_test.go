package api

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const atlasShared = "rest:http://atlas:8001/shared"

// memberOfAtlas makes the fixture a member of the shared repository Atlas
// holds, with Cube as the other member. Here, Cube and a host nobody knows
// have each backed up into it.
func memberOfAtlas(t *testing.T, f *placementFixture) store.SharedRepo {
	t.Helper()
	here := f.svc.resticHost()
	sr, err := f.st.PutSharedRepo(store.SharedRepo{Repo: atlasShared, Owner: "atlas-id", Members: []store.SharedMember{
		cube, {Host: here, Instance: "here-id", Name: "Here"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.st.SaveSharedJoin(store.SharedJoin{Direction: store.SharedJoinOut, MemberID: "atlas-id", MemberName: "Atlas", Host: here, State: store.SharedJoinJoined, Repo: atlasShared}); err != nil {
		t.Fatal(err)
	}
	for _, domain := range []string{"containers", "files"} {
		if _, err := f.st.UpsertOffsiteTarget(store.OffsiteTarget{Domain: domain, Name: "Shared with Atlas", Repo: atlasShared, Enabled: true, Immutable: true}); err != nil {
			t.Fatal(err)
		}
	}
	f.eng.snaps[atlasShared] = []restic.Snapshot{
		{ID: "aaaa000000000001", Hostname: here, Time: "2026-11-01T03:00:00Z", Tags: []string{"fileset:docs"}, Paths: []string{"/host/user/docs"}},
		{ID: "cccc000000000001", Hostname: cubeHost, Time: "2026-11-02T03:00:00Z", Tags: []string{"fileset:photos"}, Paths: []string{"/host/user/photos"}},
		{ID: "cccc000000000002", Hostname: cubeHost, Time: "2026-11-03T03:00:00Z", Tags: []string{"container:plex"}, Paths: []string{"/host/user/appdata/plex"}},
		{ID: "ffff000000000001", Hostname: strangerHost, Time: "2026-11-04T03:00:00Z", Tags: []string{"fileset:attic"}, Paths: []string{"/host/user/attic"}},
	}
	return sr
}

// decodeInto reads one field of a JSON answer into out.
func decodeInto(t *testing.T, answer map[string]any, field string, out any) {
	t.Helper()
	raw, err := json.Marshal(answer[field])
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatalf("%s: %v", field, err)
	}
}

func TestTheRestorePageNamesTheOtherServersOfASharedRepository(t *testing.T) {
	f := newPlacementFixture(t)
	sr := memberOfAtlas(t, f)

	res := f.do("GET", "/api/shared-repo/others", nil)
	if res["ok"] != true {
		t.Fatalf("others: %v", res)
	}
	var servers []sharedOtherServer
	decodeInto(t, res, "servers", &servers)
	if len(servers) != 2 {
		t.Fatalf("servers %+v", servers)
	}
	if s := servers[0]; s.Repo != sr.ID || s.Host != cubeHost || s.Name != "Cube" || s.Entries != 2 || s.Newest == 0 {
		t.Errorf("Cube reads %+v", s)
	}
	if s := servers[1]; s.Host != strangerHost || s.Name != "" || s.Entries != 1 {
		t.Errorf("the host nobody knows reads %+v", s)
	}
}

func TestAnotherServerOpensWithItsOwnBackupsAlone(t *testing.T) {
	f := newPlacementFixture(t)
	sr := memberOfAtlas(t, f)

	res := f.do("POST", "/api/shared-repo/others/open", map[string]any{"repo": sr.ID, "host": cubeHost})
	if res["ok"] != true {
		t.Fatalf("open: %v", res)
	}
	var inv ForeignInventory
	decodeInto(t, res, "inventory", &inv)
	if len(inv.FileSets) != 1 || inv.FileSets[0].Name != "photos" {
		t.Errorf("folder sets %+v", inv.FileSets)
	}
	if len(inv.Containers) != 1 || inv.Containers[0].Name != "plex" || !inv.Containers[0].NoDefinition {
		t.Errorf("containers %+v: Cube's definitions are sealed with Cube's key, so plex comes back as files", inv.Containers)
	}
	sess, err := f.svc.foreignSession(res["session"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if sess.mode.Host != cubeHost || !sess.mode.NoLock || sess.mode.Sharing != restic.SharingMember {
		t.Errorf("the session reads as host %q, no lock %v, sharing %v", sess.mode.Host, sess.mode.NoLock, sess.mode.Sharing)
	}
}

func TestThisServersOwnHostIsNotOpenedAsAnotherServer(t *testing.T) {
	f := newPlacementFixture(t)
	sr := memberOfAtlas(t, f)
	for _, host := range []string{f.svc.resticHost(), "", "-x", "bv-zz\n"} {
		code, out := f.call("POST", "/api/shared-repo/others/open", map[string]any{"repo": sr.ID, "host": host})
		if code == 200 && out["ok"] == true {
			t.Errorf("host %q opened", host)
		}
	}
	code, out := f.call("POST", "/api/shared-repo/others/open", map[string]any{"repo": "nope", "host": cubeHost})
	if code == 200 && out["ok"] == true {
		t.Error("a shared repository this server does not know opened")
	}
}

func TestAnUnreadableSharedRepositoryStillNamesItsMembers(t *testing.T) {
	f := newPlacementFixture(t)
	memberOfAtlas(t, f)
	f.eng.listErr[atlasShared] = context.DeadlineExceeded

	res := f.do("GET", "/api/shared-repo/others", nil)
	var servers []sharedOtherServer
	decodeInto(t, res, "servers", &servers)
	if len(servers) != 1 || servers[0].Name != "Cube" || servers[0].Error == "" {
		t.Fatalf("servers %+v", servers)
	}
}

func TestTheFileSearchReadsOnlyThisServersBackupsInASharedRepository(t *testing.T) {
	f := newPlacementFixture(t)
	memberOfAtlas(t, f)
	f.container("plex", "")
	here := f.svc.resticHost()
	f.eng.snaps[atlasShared] = []restic.Snapshot{
		{ID: "aaaa000000000001", Hostname: here, Time: "2026-11-01T03:00:00Z", Tags: []string{"container:plex"}, Paths: []string{"/host/user/appdata/plex"}},
		{ID: "cccc000000000001", Hostname: cubeHost, Time: "2026-11-02T03:00:00Z", Tags: []string{"container:plex"}, Paths: []string{"/host/user/appdata/plex"}},
	}
	f.holdsFiles("aaaa000000000001", foundAt("/appdata/plex/Preferences.xml", 10))
	f.holdsFiles("cccc000000000001", foundAt("/appdata/plex/Preferences.xml", 20))

	res := f.search("Preferences.xml", true, fileSearchItem{"containers", "plex"})

	if len(res.Files) != 1 || !slices.Equal(versionRows(res.Files[0]), []string{"aaaa000000000001"}) {
		t.Fatalf("found %+v, want this server's backup alone", res.Files)
	}
	for _, find := range f.eng.finds {
		if slices.Contains(find.IDs, "cccc000000000001") {
			t.Fatalf("Cube's backup was searched: %+v", find)
		}
	}
}
