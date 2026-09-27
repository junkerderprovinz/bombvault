package api

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/progress"
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

// movedHomeOf moves the database onto places and returns the home place the
// move gave domain.
func (f *placementFixture) movedHomeOf(domain string) string {
	f.t.Helper()
	if err := f.svc.MigrateToPlaces(); err != nil {
		f.t.Fatal(err)
	}
	homes, err := f.st.DomainPlaces()
	if err != nil || homes[domain] == "" {
		f.t.Fatalf("home place of %s = %q, %v", domain, homes[domain], err)
	}
	return homes[domain]
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

// The move onto places makes a place home to a remote domain path without
// putting the domain's primary row on it.
func TestAMovedHomePlaceTamperTestProbesItsDomainPath(t *testing.T) {
	var seen []string
	server := httptest.NewServer(deleteRecorder(http.StatusOK, &seen))
	defer server.Close()
	f := newPlacementFixture(t)
	f.settings(func(s *store.Settings) { s.VMsPath = "rest:" + server.URL + "/bv/vms" })
	primary, err := f.svc.SetPrimaryRemoteConfig("vms", store.OffsiteTarget{Immutable: true})
	if err != nil {
		t.Fatal(err)
	}
	home := f.movedHomeOf("vms")

	res := f.tamperTestPlace(home)

	if res["ok"] != true || res["protected"] != false || res["detail"] != "the server accepted a delete (HTTP 200)" {
		t.Fatalf("answer = %v, want deletes accepted at the vms path", res)
	}
	if len(seen) != 2 || !strings.HasPrefix(seen[0], "/bv/vms/") || !strings.HasPrefix(seen[1], "/bv/vms/") {
		t.Fatalf("probes = %v, want both at the vms path", seen)
	}
	if tt, found, err := f.st.LatestTamperTestForTarget("vms", primary.ID); err != nil || !found || tt.Protected {
		t.Fatalf("verdict of the vms path = %+v, found %v, %v, want deletes accepted under its primary row", tt, found, err)
	}
}

// Without a primary row the path has no id of its own to keep a verdict
// under, and the domain's empty one would compare it with another target's.
func TestAMovedDomainPathWithoutAPrimaryRowIsProbedWithoutKeepingAVerdict(t *testing.T) {
	var seen []string
	server := httptest.NewServer(deleteRecorder(http.StatusForbidden, &seen))
	defer server.Close()
	f := newPlacementFixture(t)
	f.settings(func(s *store.Settings) { s.VMsPath = "rest:" + server.URL + "/bv/vms" })
	home := f.movedHomeOf("vms")

	if res := f.tamperTestPlace(home); res["ok"] != true || res["testable"] != true || res["protected"] != true || len(seen) != 2 {
		t.Fatalf("answer = %v, probes %v, want deletes refused at the vms path", res, seen)
	}
	if tt, found, err := f.st.LatestTamperTest("vms"); err != nil || found {
		t.Fatalf("verdict under vms = %+v, found %v, %v, want none", tt, found, err)
	}
}

func TestAPlaceTamperTestLeavesADirectRepositoryThereAlone(t *testing.T) {
	var seen []string
	server := httptest.NewServer(deleteRecorder(http.StatusForbidden, &seen))
	defer server.Close()
	f := newPlacementFixture(t)
	garage := f.storePlace(restPlace("Garage", server.URL+"/bv"))
	copied := f.placeTarget(garage, "containers", "")
	direct := f.direct(copied)
	f.linkRow(direct.ID, garage, "containers", directSuffix)

	if res := f.tamperTestPlace(garage.ID); res["ok"] != true || res["protected"] != true {
		t.Fatalf("answer = %v", res)
	}
	if len(seen) != 2 || slices.ContainsFunc(seen, func(p string) bool { return strings.Contains(p, directSuffix) }) {
		t.Fatalf("probes = %v, want the copy's two and none at its direct repository", seen)
	}
}

func TestARepositoryPlaceHasNothingToTamperTest(t *testing.T) {
	var seen []string
	server := httptest.NewServer(deleteRecorder(http.StatusForbidden, &seen))
	defer server.Close()
	f := newPlacementFixture(t)
	p := restPlace("Vault", server.URL+"/vault")
	p.Folders = map[string]string{"containers": "", "vms": "", "files": ""}
	vault := f.storePlace(p)
	f.linkRow(f.namedRepo("Vault", vault.Base).ID, vault, "", "")

	if res := f.tamperTestPlace(vault.ID); res["ok"] != false || res["code"] != "place-nothing-to-test" || len(seen) != 0 {
		t.Fatalf("answer = %v, probes %v, want place-nothing-to-test without a probe", res, seen)
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

func TestAPlaceTamperTestLogsEachDomainItProbes(t *testing.T) {
	server := httptest.NewServer(deleteRecorder(http.StatusOK, new([]string)))
	defer server.Close()
	f := newPlacementFixture(t)
	friend := f.storePlace(restPlace("Friend", server.URL), "vms")
	f.placeTarget(friend, "containers", "")
	prog := progress.NewStore()
	f.svc.SetProgress(prog)
	events, cancel := prog.Subscribe()
	defer cancel()

	f.tamperTestPlace(friend.ID)

	ended := map[string]bool{}
	for len(events) > 0 {
		if e := <-events; !e.Active {
			ended[e.Key] = true
		}
	}
	if want := map[string]bool{"tamper:containers": true, "tamper:vms": true}; !maps.Equal(ended, want) {
		t.Fatalf("finished progress lines = %v, want %v", ended, want)
	}
	runs, err := f.st.ListRuns(10)
	if err != nil {
		t.Fatal(err)
	}
	logged := map[string]string{}
	for _, r := range runs {
		if r.Kind == "tamper" {
			logged[r.TargetID] = r.Status + ": " + r.Error
		}
	}
	accepted := "failed: the server accepted a delete (HTTP 200)"
	if want := map[string]string{"containers": accepted, "vms": accepted}; !maps.Equal(logged, want) {
		t.Fatalf("tamper runs = %v, want %v", logged, want)
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

	if res := f.tamperTestPlace(garage.ID); res["ok"] != false || res["code"] != "place-nothing-to-test" || len(seen) != 0 {
		t.Fatalf("answer = %v, probes %v, want place-nothing-to-test without a probe", res, seen)
	}
}

func TestARemoteDomainPathIsTamperTestedThroughItsPlaceOnly(t *testing.T) {
	f := newPlacementFixture(t)
	rec := httptest.NewRecorder()
	f.h.Router().ServeHTTP(rec, jsonReq(http.MethodPost, "/api/settings/primary-remote/vms/tamper-test", nil))
	if rec.Code == http.StatusOK {
		t.Fatalf("POST = %d %s, want no such route", rec.Code, rec.Body.String())
	}
}

func TestAnUnknownPlaceIsNotTamperTested(t *testing.T) {
	f := newPlacementFixture(t)
	if code, res := f.doStatus(http.MethodPost, "/api/places/nosuchplace/tamper-test", nil); code != http.StatusNotFound || res["ok"] != false {
		t.Fatalf("POST = %d %v, want 404", code, res)
	}
}
