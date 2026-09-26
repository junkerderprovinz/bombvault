package api

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// restPlace is a rest-server place with append-only on, offering containers,
// vms and files.
func restPlace(name, url string) store.Place {
	return store.Place{Name: name, Provider: "rest-server", Kind: string(places.KindREST), Base: "rest:" + url,
		Folders: map[string]string{"containers": "container", "vms": "vms", "files": "files"}, Immutable: true, OffPremises: true, Enabled: true}
}

func (f *placementFixture) tamperTestPlace(id string) map[string]any {
	f.t.Helper()
	return f.do(http.MethodPost, "/api/places/"+id+"/tamper-test", nil)
}

func TestAPlaceTamperTestLeavesTheSameDomainsCopiesAtOtherPlacesAlone(t *testing.T) {
	var guarded, plain []string
	garageServer := httptest.NewServer(deleteRecorder(http.StatusForbidden, &guarded))
	defer garageServer.Close()
	friendServer := httptest.NewServer(deleteRecorder(http.StatusOK, &plain))
	defer friendServer.Close()
	f := newPlacementFixture(t)
	garage := f.storePlace(restPlace("Garage", garageServer.URL))
	friend := f.storePlace(restPlace("Friend", friendServer.URL))
	atGarage := f.placeTarget(garage, "containers", "")
	atFriend := f.placeTarget(friend, "containers", "")

	res := f.tamperTestPlace(garage.ID)

	if res["ok"] != true || res["testable"] != true || res["protected"] != true {
		t.Fatalf("answer = %v, want deletes refused at the garage", res)
	}
	if len(guarded) == 0 || len(plain) != 0 {
		t.Fatalf("probes: garage %v, friend %v, want the garage's server alone", guarded, plain)
	}
	if tt, found, err := f.st.LatestTamperTestForTarget("containers", atGarage.ID); err != nil || !found || !tt.Protected {
		t.Fatalf("verdict of the garage copy = %+v, found %v, %v", tt, found, err)
	}
	if _, found, err := f.st.LatestTamperTestForTarget("containers", atFriend.ID); err != nil || found {
		t.Fatalf("the friend's copy got a verdict: found %v, %v", found, err)
	}
}

func TestAPlaceTamperTestProbesTheDomainPathsAndCopiesItHolds(t *testing.T) {
	var seen []string
	server := httptest.NewServer(deleteRecorder(http.StatusForbidden, &seen))
	defer server.Close()
	f := newPlacementFixture(t)
	garage := f.storePlace(restPlace("Garage", server.URL), "vms")
	f.placeTarget(garage, "containers", "")
	off := f.placeTarget(garage, "files", "")
	off.Enabled = false
	if _, err := f.st.UpsertOffsiteTarget(off); err != nil {
		t.Fatal(err)
	}

	if res := f.tamperTestPlace(garage.ID); res["ok"] != true || res["protected"] != true {
		t.Fatalf("answer = %v", res)
	}

	folders := map[string]int{}
	for _, p := range seen {
		folders[strings.Split(strings.TrimPrefix(p, "/"), "/")[0]]++
	}
	if want := map[string]int{"vms": 2, "container": 2}; !maps.Equal(folders, want) {
		t.Fatalf("probed folders = %v, want both probes at the vms path and the containers copy, none at the switched-off files copy", folders)
	}
}

func TestAPlaceThatAcceptsDeletesSaysSoOnce(t *testing.T) {
	server := httptest.NewServer(deleteRecorder(http.StatusOK, new([]string)))
	defer server.Close()
	f := newPlacementFixture(t)
	friend := f.storePlace(restPlace("Friend", server.URL))
	f.placeTarget(friend, "containers", "")
	f.placeTarget(friend, "vms", "")

	res := f.tamperTestPlace(friend.ID)

	if res["ok"] != true || res["testable"] != true || res["protected"] != false || res["detail"] != "the server accepted a delete (HTTP 200)" {
		t.Fatalf("answer = %v, want deletes accepted, said once", res)
	}
}

func TestAPlaceTamperTestAtABucketCannotTellAnything(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	f.placeTarget(b2, "containers", "")

	if res := f.tamperTestPlace(b2.ID); res["ok"] != true || res["testable"] != false {
		t.Fatalf("answer = %v, want a verdict that is not testable", res)
	}
}

func TestAFolderOnThisServerHasNoAppendOnlyToTest(t *testing.T) {
	f := newPlacementFixture(t)
	unraid := f.storePlace(localPlace("Unraid", "backups"), "containers")

	if res := f.tamperTestPlace(unraid.ID); res["ok"] != false || res["code"] != "place-no-append-only" {
		t.Fatalf("answer = %v, want place-no-append-only", res)
	}
}

func TestASwitchedOffPlaceIsNotTamperTested(t *testing.T) {
	var seen []string
	server := httptest.NewServer(deleteRecorder(http.StatusForbidden, &seen))
	defer server.Close()
	f := newPlacementFixture(t)
	p := restPlace("Garage", server.URL)
	p.Enabled = false
	garage := f.storePlace(p)
	f.placeTarget(garage, "containers", "")

	if res := f.tamperTestPlace(garage.ID); res["ok"] != false || res["code"] != "place-off" || len(seen) != 0 {
		t.Fatalf("answer = %v, probes %v, want place-off without a probe", res, seen)
	}
}

func TestAnUnusedPlaceHasNothingToTamperTest(t *testing.T) {
	var seen []string
	server := httptest.NewServer(deleteRecorder(http.StatusForbidden, &seen))
	defer server.Close()
	f := newPlacementFixture(t)
	garage := f.storePlace(restPlace("Garage", server.URL))

	if res := f.tamperTestPlace(garage.ID); res["ok"] != false || res["error"] == "" || len(seen) != 0 {
		t.Fatalf("answer = %v, probes %v, want a refusal without a probe", res, seen)
	}
}

func TestAnUnknownPlaceIsNotTamperTested(t *testing.T) {
	f := newPlacementFixture(t)
	if code, res := f.doStatus(http.MethodPost, "/api/places/nosuchplace/tamper-test", nil); code != http.StatusNotFound || res["ok"] != false {
		t.Fatalf("POST = %d %v, want 404", code, res)
	}
}
