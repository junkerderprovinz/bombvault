package api

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func (f *placementFixture) placeRepo(placeID, domain string) map[string]any {
	f.t.Helper()
	return f.do(http.MethodPost, "/api/places/"+placeID+"/repo", map[string]any{"domain": domain})
}

func TestAPlaceWithATargetOfTheDomainStandsForItsDirectRepository(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	target := f.placeTarget(b2, "containers", "")

	res := f.placeRepo(b2.ID, "containers")

	id, _ := res["repoId"].(string)
	row, err := f.st.GetNamedRepo(id)
	if res["ok"] != true || err != nil || row.CompanionOf != target.ID || row.Repo != target.Repo+"-direct" ||
		row.PlaceID != b2.ID || row.PlaceSuffix != "-direct" {
		t.Fatalf("POST repo = %v; row %+v, %v", res, row, err)
	}
	if again := f.placeRepo(b2.ID, "containers"); again["repoId"] != id {
		t.Fatalf("second POST = %v, want the same direct repository", again)
	}
}

func TestAPlaceWithoutARepositoryOfTheDomainGetsANamedOne(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.storePlace(localPlace("NAS", "nas"))

	res := f.placeRepo(nas.ID, "containers")

	id, _ := res["repoId"].(string)
	row, err := f.st.GetNamedRepo(id)
	if res["ok"] != true || err != nil || row.CompanionOf != "" || row.Repo != "nas/containers" ||
		row.PlaceID != nas.ID || row.PlaceDomain != "containers" || row.PlaceSuffix != "" {
		t.Fatalf("POST repo = %v; row %+v, %v", res, row, err)
	}
	if again := f.placeRepo(nas.ID, "containers"); again["repoId"] != id {
		t.Fatalf("second POST = %v, want the same repository", again)
	}
}

func TestTheHomePlaceStandsForTheDomainPath(t *testing.T) {
	f := newPlacementFixture(t)
	unraid := f.storePlace(localPlace("Unraid", "backups"), "containers")
	if res := f.placeRepo(unraid.ID, "containers"); res["ok"] != true || res["repoId"] != "" {
		t.Fatalf("POST repo = %v, want the domain path", res)
	}
}

func TestAPlaceThatIsARepositoryTakesNoSecondRole(t *testing.T) {
	f := newPlacementFixture(t)
	p := s3Place("B2 root", "s3:https://s3.example.com/bucket")
	p.Folders = map[string]string{"containers": "", "vms": ""}
	root := f.storePlace(p)
	f.placeTarget(root, "containers", "")
	for _, domain := range []string{"containers", "vms"} {
		if res := f.placeRepo(root.ID, domain); res["ok"] != false || res["code"] != "place-is-repository" {
			t.Fatalf("POST repo for %s = %v, want place-is-repository", domain, res)
		}
	}
}

func TestAPlaceThatIsARepositoryServesEveryDomainFromItsOneRepository(t *testing.T) {
	f := newPlacementFixture(t)
	p := s3Place("B2 root", "s3:https://s3.example.com/bucket")
	p.Folders = map[string]string{"containers": "", "vms": ""}
	root := f.storePlace(p)

	res := f.placeRepo(root.ID, "containers")

	id, _ := res["repoId"].(string)
	row, err := f.st.GetNamedRepo(id)
	if res["ok"] != true || err != nil || row.Repo != p.Base || row.PlaceID != root.ID || row.PlaceDomain != "" {
		t.Fatalf("POST repo = %v; row %+v, %v, want the place's own repository at its base", res, row, err)
	}
	if vms := f.placeRepo(root.ID, "vms"); vms["repoId"] != id {
		t.Fatalf("POST repo for vms = %v, want the same repository", vms)
	}
}

func TestAPlaceWithoutTheDomainsFolderStandsForNothing(t *testing.T) {
	f := newPlacementFixture(t)
	p := localPlace("NAS", "nas")
	p.Folders = map[string]string{"vms": "vms"}
	nas := f.storePlace(p)
	if res := f.placeRepo(nas.ID, "containers"); res["ok"] != false || res["code"] != "place-domain-unavailable" {
		t.Fatalf("POST repo = %v, want place-domain-unavailable", res)
	}
}

func TestASwitchedOffPlaceStandsForNothing(t *testing.T) {
	f := newPlacementFixture(t)
	p := localPlace("NAS", "nas")
	p.Enabled = false
	nas := f.storePlace(p)
	if res := f.placeRepo(nas.ID, "containers"); res["ok"] != false || res["code"] != "place-off" {
		t.Fatalf("POST repo = %v, want place-off", res)
	}
	if repos, err := f.st.ListNamedRepos(); err != nil || len(repos) != 0 {
		t.Fatalf("named repositories = %+v, %v, want none", repos, err)
	}
}

func TestAnUnknownPlaceStandsForNothing(t *testing.T) {
	f := newPlacementFixture(t)
	if code, res := f.doStatus(http.MethodPost, "/api/places/nosuchplace/repo", map[string]any{"domain": "containers"}); code != http.StatusNotFound || res["ok"] != false {
		t.Fatalf("POST repo at an unknown place = %d %v, want 404", code, res)
	}
}

func TestMakingAPlaceRepositoryWaitsForAnEditInProgress(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.storePlace(localPlace("NAS", "nas"))
	f.svc.placeEditMu.Lock()
	done := make(chan error, 1)
	go func() {
		_, err := f.svc.ensurePlaceRepo(context.Background(), nas.ID, "containers")
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("a repository was made at a place during an edit of it: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	f.svc.placeEditMu.Unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the repository was never made")
	}
}
