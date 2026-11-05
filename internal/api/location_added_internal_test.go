package api

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const emptiedAt = "rest:http://192.168.98.2:8000/nas"

var aMode = restic.Mode{Encrypted: true, Password: "pw"}

// usedBefore leaves the record of a repository that was at loc and is not now,
// as a location used, removed and emptied there leaves it.
func (f *placementFixture) usedBefore(loc string) {
	f.t.Helper()
	f.eng.opens[loc] = false
	if err := f.st.MarkRepoEstablished(loc); err != nil {
		f.t.Fatal(err)
	}
}

func (f *placementFixture) addRestLocation(url string) store.OffsiteTarget {
	f.t.Helper()
	d, err := f.svc.SaveNewDestination(context.Background(), draftRequest{Provider: "rest", Name: "NAS", Settings: map[string]string{"url": url}})
	if err != nil {
		f.t.Fatal(err)
	}
	return d
}

func (f *placementFixture) wantsNewRepository(loc string) {
	f.t.Helper()
	if err := f.svc.EnsureRepo(context.Background(), loc, aMode); err != nil {
		f.t.Fatalf("EnsureRepo(%s) = %v, want a new repository", loc, err)
	}
	if !slices.Contains(f.eng.ensured, loc) {
		f.t.Fatalf("no repository was created at %s", loc)
	}
}

func (f *placementFixture) wantsRefusal(loc string) {
	f.t.Helper()
	if err := f.svc.EnsureRepo(context.Background(), loc, aMode); !errors.Is(err, errRemoteRepoGone) {
		f.t.Fatalf("EnsureRepo(%s) = %v, want the gone refusal", loc, err)
	}
}

func TestALocationAddedAgainStartsCleanInEverySection(t *testing.T) {
	f := newPlacementFixture(t)
	for _, folder := range []string{"containers", "vms", "flash", "selfbackup", "files", "zfs"} {
		f.usedBefore(emptiedAt + "/" + folder)
	}
	d := f.addRestLocation(emptiedAt)

	for _, domain := range offsiteConfigDomains {
		res := f.do("POST", "/api/offsite/destinations/"+d.ID+"/domains/"+domain, nil)
		if res["ok"] != true {
			t.Fatalf("tick %s = %v", domain, res)
		}
	}
	targets, err := f.st.DestinationTargets(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range targets {
		res := f.do("POST", "/api/offsite/targets/"+target.ID+"/test", nil)
		if res["ok"] != true || res["initialized"] != false {
			t.Errorf("test of %s = %v, want a place with no repository yet", target.Domain, res)
		}
		f.wantsNewRepository(target.Repo)
	}
	if f.goneLocation("destination:" + d.ID) {
		t.Error("the page says a repository this location never had is gone")
	}
}

func TestALocationAddedOverItsRepositoryKeepsTheGuard(t *testing.T) {
	f := newPlacementFixture(t)
	repo := emptiedAt + "/files"
	if err := f.st.MarkRepoEstablished(repo); err != nil {
		t.Fatal(err)
	}
	f.eng.snaps[repo] = []restic.Snapshot{{ID: "s1", Tags: []string{"fileset:photos"}}}
	d := f.addRestLocation(emptiedAt)
	f.svc.takeUpOldBackups(context.Background(), d)

	f.eng.opens[repo] = false
	f.wantsRefusal(repo)
}

func TestASectionTickedAgainStartsClean(t *testing.T) {
	f := newPlacementFixture(t)
	repo := emptiedAt + "/files"
	d := f.destination("NAS", emptiedAt)
	f.usedBefore(repo)

	if res := f.do("POST", "/api/offsite/destinations/"+d.ID+"/domains/files", nil); res["ok"] != true {
		t.Fatalf("tick = %v", res)
	}
	f.wantsNewRepository(repo)
}

func TestALocationThatStayedKeepsTheGuard(t *testing.T) {
	f := newPlacementFixture(t)
	repo := emptiedAt + "/files"
	d := f.destination("NAS", emptiedAt)
	f.tickDestination(d, "files")
	f.usedBefore(repo)

	if res := f.do("POST", "/api/offsite/destinations/"+d.ID+"/domains/files", nil); res["ok"] != true {
		t.Fatalf("tick = %v", res)
	}
	f.wantsRefusal(repo)
	if !f.goneLocation("destination:" + d.ID) {
		t.Error("the page does not say the repository of a location that stayed is gone")
	}
}

func TestATypedTargetStartsCleanWhereItIsAddedAndWhereItMoves(t *testing.T) {
	f := newPlacementFixture(t)
	added, moved := emptiedAt+"/typed", emptiedAt+"/moved"
	f.usedBefore(added)
	f.usedBefore(moved)

	body := map[string]any{"domain": "files", "name": "Box", "repo": added, "enabled": true, "sortOrder": 1}
	res := f.do("POST", "/api/offsite/targets", body)
	if res["ok"] != true {
		t.Fatalf("add = %v", res)
	}
	f.wantsNewRepository(added)

	id := res["target"].(map[string]any)["id"].(string)
	body["repo"] = moved
	if res := f.do("PUT", "/api/offsite/targets/"+id, body); res["ok"] != true {
		t.Fatalf("move = %v", res)
	}
	f.wantsNewRepository(moved)
}

func TestATypedTargetSavedInPlaceKeepsTheGuard(t *testing.T) {
	f := newPlacementFixture(t)
	repo := emptiedAt + "/typed"
	tgt := f.target("files", "Box", repo)
	f.usedBefore(repo)

	body := map[string]any{"domain": "files", "name": "Renamed", "repo": repo, "enabled": true}
	if res := f.do("PUT", "/api/offsite/targets/"+tgt.ID, body); res["ok"] != true {
		t.Fatalf("save = %v", res)
	}
	f.wantsRefusal(repo)
}

func TestARepositoryStartsCleanWhereItIsAddedAndWhereItMoves(t *testing.T) {
	f := newPlacementFixture(t)
	added, moved := emptiedAt+"/named", emptiedAt+"/elsewhere"
	f.usedBefore(added)
	f.usedBefore(moved)

	res := f.do("POST", "/api/repos", map[string]any{"name": "Bucket", "repo": added})
	if res["ok"] != true {
		t.Fatalf("add = %v", res)
	}
	f.wantsNewRepository(added)

	id := res["repo"].(map[string]any)["id"].(string)
	if res := f.do("PATCH", "/api/repos/"+id, map[string]any{"repo": moved}); res["ok"] != true {
		t.Fatalf("move = %v", res)
	}
	f.wantsNewRepository(moved)
}

func TestARepositorySavedInPlaceKeepsTheGuard(t *testing.T) {
	f := newPlacementFixture(t)
	repo := emptiedAt + "/named"
	named := f.namedRepo("Bucket", repo)
	f.usedBefore(repo)

	if res := f.do("PATCH", "/api/repos/"+named.ID, map[string]any{"name": "Renamed"}); res["ok"] != true {
		t.Fatalf("save = %v", res)
	}
	f.wantsRefusal(repo)
}

func TestASettingsFieldStartsCleanOnlyWhereItChanged(t *testing.T) {
	f := newPlacementFixture(t)
	kept, typed := emptiedAt+"/kept", emptiedAt+"/typed"
	f.fieldTarget("vms", kept)
	f.usedBefore(kept)
	f.usedBefore(typed)

	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	v := toView(settings)
	v.FilesOffsite = typed
	if res := f.do("PUT", "/api/settings", v); res["ok"] != true {
		t.Fatalf("save = %v", res)
	}
	f.wantsNewRepository(typed)
	f.wantsRefusal(kept)
}
