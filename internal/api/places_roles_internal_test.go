package api

import (
	"context"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
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

func (f *placementFixture) adopt(placeID, rowID, domain string) map[string]any {
	f.t.Helper()
	return f.do(http.MethodPost, "/api/places/"+placeID+"/adopt", map[string]any{"rowId": rowID, "domain": domain})
}

func TestATargetAtThePlacesAddressJoinsItWithoutAProbe(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	loose := f.target("containers", "B2", "s3:https://s3.example.com/bucket/container")

	res := f.adopt(b2.ID, loose.ID, "")

	row, _, err := f.st.GetOffsiteTarget(loose.ID)
	if res["ok"] != true || err != nil || row.PlaceID != b2.ID || row.PlaceDomain != "containers" || row.Repo != loose.Repo {
		t.Fatalf("adopt = %v; row %+v, %v", res, row, err)
	}
	if len(f.eng.opened) != 0 {
		t.Fatalf("adopting at the same address opened %v", f.eng.opened)
	}
}

func TestATargetWithCopiesElsewhereIsNotMovedOntoThePlace(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	loose := f.target("containers", "Old", "s3:https://s3.example.com/old/container")
	f.listing("containers", loose.ID, 100, copiesRow("container:nginx", 6, 90))
	f.eng.opens["s3:https://s3.example.com/bucket/container"] = false

	res := f.adopt(b2.ID, loose.ID, "")

	if res["ok"] != false || res["code"] != "place-location-established" || res["snapshots"] != float64(6) ||
		!reflect.DeepEqual(res["domains"], []any{"containers"}) {
		t.Fatalf("adopt = %v, want place-location-established with 6 snapshots", res)
	}
	if row, _, err := f.st.GetOffsiteTarget(loose.ID); err != nil || row.PlaceID != "" || row.Repo != loose.Repo {
		t.Fatalf("row = %+v, %v, want it untouched", row, err)
	}
}

func TestATargetWithNothingElsewhereMovesToAnEmptyAddress(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	loose := f.target("vms", "Old", "s3:https://s3.example.com/old/vms")
	f.eng.opens["s3:https://s3.example.com/bucket/vms"] = false

	res := f.adopt(b2.ID, loose.ID, "")

	if row, _, err := f.st.GetOffsiteTarget(loose.ID); res["ok"] != true || err != nil || row.Repo != "s3:https://s3.example.com/bucket/vms" || row.PlaceID != b2.ID {
		t.Fatalf("adopt = %v; row %+v, %v", res, row, err)
	}
}

func TestADirectRepositoryJoinsOnlyThePlaceOfItsTarget(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	elsewhere := f.target("containers", "Other", "s3:https://s3.example.com/other/container")
	direct := f.direct(elsewhere)
	if res := f.adopt(b2.ID, direct.ID, ""); res["ok"] != false || res["code"] != "mirrored-field" {
		t.Fatalf("adopt = %v, want mirrored-field", res)
	}
}

func TestADirectRepositoryDoesNotJoinAPlaceThatIsARepository(t *testing.T) {
	f := newPlacementFixture(t)
	p := s3Place("B2 root", "s3:https://s3.example.com/bucket")
	p.Folders = map[string]string{"containers": "", "vms": ""}
	root := f.storePlace(p)
	target := f.placeTarget(root, "containers", "")
	direct, err := f.st.CreateCompanionRepo(target.ID, "B2 root direct", "s3:https://s3.example.com/other")
	if err != nil {
		t.Fatal(err)
	}
	f.eng.opens["s3:https://s3.example.com/bucket-direct"] = false

	if res := f.adopt(root.ID, direct.ID, ""); res["ok"] != false || res["code"] != "place-is-repository" {
		t.Fatalf("adopt = %v, want place-is-repository", res)
	}
	if row, err := f.st.GetNamedRepo(direct.ID); err != nil || row.PlaceID != "" || row.Repo != direct.Repo {
		t.Fatalf("row = %+v, %v, want it untouched", row, err)
	}
}

func TestARepositoryDoesNotJoinASwitchedOffPlace(t *testing.T) {
	f := newPlacementFixture(t)
	p := localPlace("NAS", "nas")
	p.Enabled = false
	nas := f.storePlace(p)
	named := f.namedRepo("Old NAS", "nas/containers")
	f.container("nginx", named.ID)
	direct := f.direct(f.placeTarget(nas, "vms", ""))

	for _, c := range []struct {
		row    store.OffsiteTarget
		domain string
	}{{named, "containers"}, {direct, ""}} {
		if res := f.adopt(nas.ID, c.row.ID, c.domain); res["ok"] != false || res["code"] != "place-off" {
			t.Fatalf("adopt %s = %v, want place-off", c.row.Name, res)
		}
		if row, err := f.st.GetNamedRepo(c.row.ID); err != nil || row.PlaceID != "" || !row.Enabled {
			t.Fatalf("%s = %+v, %v, want it on and without a place", c.row.Name, row, err)
		}
	}
}

func TestADomainPathWithoutAPlaceJoinsThePlaceAtItsAddress(t *testing.T) {
	f := newPlacementFixture(t)
	bv := f.storePlace(localPlace("Unraid", "user/bombvault"))

	res := f.adopt(bv.ID, "", "flash")

	homes, err := f.st.DomainPlaces()
	if res["ok"] != true || err != nil || homes["flash"] != bv.ID {
		t.Fatalf("adopt = %v; homes %v, %v", res, homes, err)
	}
	if settings, err := f.st.GetSettings(); err != nil || settings.FlashPath != "user/bombvault/flash" {
		t.Fatalf("flash path = %q, %v, want it unchanged", settings.FlashPath, err)
	}
}

func TestARowAtAPlaceIsNotAdoptedAgain(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	target := f.placeTarget(b2, "containers", "")
	eu := f.storePlace(s3Place("B2 EU", "s3:https://s3.example.com/eu"))
	if res := f.adopt(eu.ID, target.ID, ""); res["ok"] != false || res["code"] != nil {
		t.Fatalf("adopt = %v, want a plain refusal", res)
	}
}

func TestANamedRepositoryJoinsAPlaceAtTheFolderOfItsDomain(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.storePlace(localPlace("NAS", "nas"))
	old := f.namedRepo("Old NAS", "nas/containers")

	res := f.adopt(nas.ID, old.ID, "containers")

	row, err := f.st.GetNamedRepo(old.ID)
	if res["ok"] != true || err != nil || row.PlaceID != nas.ID || row.PlaceDomain != "containers" || row.Repo != "nas/containers" {
		t.Fatalf("adopt = %v; row %+v, %v", res, row, err)
	}
}

func TestARepositoryAdoptedAtAPlaceThatIsItselfOneServesEveryDomain(t *testing.T) {
	f := newPlacementFixture(t)
	p := s3Place("B2 root", "s3:https://s3.example.com/bucket")
	p.Folders = map[string]string{"containers": "", "vms": ""}
	root := f.storePlace(p)
	repo := f.namedRepo("Old root", "s3:https://s3.example.com/bucket")

	res := f.adopt(root.ID, repo.ID, "containers")

	row, err := f.st.GetNamedRepo(repo.ID)
	if res["ok"] != true || err != nil || row.PlaceID != root.ID || row.PlaceDomain != "" || row.Repo != p.Base {
		t.Fatalf("adopt = %v; row %+v, %v, want the place's one repository", res, row, err)
	}
	if vms := f.placeRepo(root.ID, "vms"); vms["repoId"] != repo.ID {
		t.Fatalf("POST repo for vms = %v, want the adopted repository", vms)
	}
}

func TestARowIsNotAdoptedOntoTheAddressOfAnotherRepository(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	f.namedRepo("Old repository", "s3:https://s3.example.com/bucket/vms")
	loose := f.target("vms", "Old", "s3:https://s3.example.com/old/vms")
	f.eng.opens["s3:https://s3.example.com/bucket/vms"] = false

	if res := f.adopt(b2.ID, loose.ID, ""); res["ok"] != false || res["code"] != "nested-location" {
		t.Fatalf("adopt = %v, want nested-location", res)
	}
	if row, _, err := f.st.GetOffsiteTarget(loose.ID); err != nil || row.PlaceID != "" || row.Repo != loose.Repo {
		t.Fatalf("row = %+v, %v, want it untouched", row, err)
	}
}

func TestADomainPathDoesNotJoinASwitchedOffPlace(t *testing.T) {
	f := newPlacementFixture(t)
	p := localPlace("Unraid", "user/bombvault")
	p.Enabled = false
	bv := f.storePlace(p)

	if res := f.adopt(bv.ID, "", "flash"); res["ok"] != false || res["code"] != "place-off" {
		t.Fatalf("adopt = %v, want place-off", res)
	}
	if homes, err := f.st.DomainPlaces(); err != nil || homes["flash"] != "" {
		t.Fatalf("homes = %v, %v, want flash without a home place", homes, err)
	}
}

func TestAnUnknownPlaceAdoptsNothing(t *testing.T) {
	f := newPlacementFixture(t)
	loose := f.target("vms", "Old", "s3:https://s3.example.com/old/vms")
	if code, res := f.doStatus(http.MethodPost, "/api/places/nosuchplace/adopt", map[string]any{"rowId": loose.ID}); code != http.StatusNotFound || res["ok"] != false {
		t.Fatalf("adopt at an unknown place = %d %v, want 404", code, res)
	}
}

func TestAnAdoptionWaitsForAnEditInProgress(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	loose := f.target("containers", "B2", "s3:https://s3.example.com/bucket/container")
	f.svc.placeEditMu.Lock()
	done := make(chan error, 1)
	go func() {
		done <- f.svc.adoptRow(context.Background(), b2.ID, adoptBody{RowID: loose.ID})
	}()
	select {
	case err := <-done:
		t.Fatalf("a row was adopted during an edit of its place: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	f.svc.placeEditMu.Unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the row was never adopted")
	}
}

// keepLocally sets the rule the rows without a place age by.
func (f *placementFixture) keepLocally(last int) {
	f.t.Helper()
	settings, err := f.st.GetSettings()
	if err != nil {
		f.t.Fatal(err)
	}
	settings.RetentionKeepLast = last
	if err := f.st.UpdateSettings(settings); err != nil {
		f.t.Fatal(err)
	}
}

func TestARowIsNotAdoptedByAPlaceThatKeepsFewerSnapshots(t *testing.T) {
	for _, c := range []struct {
		name string
		row  func(f *placementFixture, b2 store.Place) (store.OffsiteTarget, string)
	}{
		{"named repository", func(f *placementFixture, _ store.Place) (store.OffsiteTarget, string) {
			f.keepLocally(30)
			return f.namedRepo("Old B2", "s3:https://s3.example.com/bucket/container"), "containers"
		}},
		{"target", func(f *placementFixture, _ store.Place) (store.OffsiteTarget, string) {
			loose := f.target("containers", "Old B2", "s3:https://s3.example.com/bucket/container")
			loose.RetentionKeepLast = 30
			loose, err := f.st.UpsertOffsiteTarget(loose)
			if err != nil {
				t.Fatal(err)
			}
			return loose, ""
		}},
		{"direct repository", func(f *placementFixture, b2 store.Place) (store.OffsiteTarget, string) {
			direct := f.direct(f.placeTarget(b2, "containers", ""))
			if _, err := f.db.Exec(`UPDATE offsite_targets SET retention_keep_last = 30 WHERE id = ?`, direct.ID); err != nil {
				t.Fatal(err)
			}
			return direct, ""
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newPlacementFixture(t)
			p := s3Place("B2", "s3:https://s3.example.com/bucket")
			p.RetentionKeepLast = 3
			b2 := f.storePlace(p)
			row, domain := c.row(f, b2)

			if res := f.adopt(b2.ID, row.ID, domain); res["ok"] != false || res["code"] != "place-keeps-less" {
				t.Fatalf("adopt = %v, want place-keeps-less", res)
			}
			after, found, err := f.st.GetOffsiteTarget(row.ID)
			if !found {
				after, err = f.st.GetNamedRepo(row.ID)
			}
			if err != nil || after.PlaceID != "" || after.RetentionKeepLast == p.RetentionKeepLast {
				t.Fatalf("row = %+v, %v, want it without a place and on its old rules", after, err)
			}
		})
	}
}

func TestADomainPathIsNotAdoptedByAPlaceThatKeepsFewerSnapshots(t *testing.T) {
	f := newPlacementFixture(t)
	f.keepLocally(30)
	p := localPlace("Unraid", "user/bombvault")
	p.RetentionKeepLast = 3
	bv := f.storePlace(p)

	if res := f.adopt(bv.ID, "", "flash"); res["ok"] != false || res["code"] != "place-keeps-less" {
		t.Fatalf("adopt = %v, want place-keeps-less", res)
	}
	if homes, err := f.st.DomainPlaces(); err != nil || homes["flash"] != "" {
		t.Fatalf("homes = %v, %v, want flash without a home place", homes, err)
	}
}

func TestAnAppendOnlyRepositoryIsNotAdoptedByAPlaceWithoutIt(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	named := f.namedRepo("Old B2", "s3:https://s3.example.com/bucket/container")
	if _, err := f.db.Exec(`UPDATE offsite_targets SET immutable = 1 WHERE id = ?`, named.ID); err != nil {
		t.Fatal(err)
	}

	if res := f.adopt(b2.ID, named.ID, "containers"); res["ok"] != false || res["code"] != "place-append-only-off" {
		t.Fatalf("adopt = %v, want place-append-only-off", res)
	}
	if row, err := f.st.GetNamedRepo(named.ID); err != nil || row.PlaceID != "" || !row.Immutable {
		t.Fatalf("row = %+v, %v, want it append-only and without a place", row, err)
	}
}

func TestARepositoryAdoptedByAPlaceThatKeepsMoreTakesItsRules(t *testing.T) {
	f := newPlacementFixture(t)
	f.keepLocally(3)
	p := s3Place("B2", "s3:https://s3.example.com/bucket")
	p.RetentionKeepLast, p.Immutable = 30, true
	b2 := f.storePlace(p)
	named := f.namedRepo("Old B2", "s3:https://s3.example.com/bucket/container")

	res := f.adopt(b2.ID, named.ID, "containers")

	row, err := f.st.GetNamedRepo(named.ID)
	if res["ok"] != true || err != nil || row.PlaceID != b2.ID || row.RetentionKeepLast != 30 || !row.Immutable {
		t.Fatalf("adopt = %v; row %+v, %v, want the place's rules", res, row, err)
	}
}

func TestADirectRepositoryOnCredentialsOfItsOwnJoinsItsTargetsPlaceUnopened(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	direct := f.direct(f.placeTarget(b2, "containers", ""))
	if _, err := f.db.Exec(`UPDATE offsite_targets SET creds_ref = 'kept' WHERE id = ?`, direct.ID); err != nil {
		t.Fatal(err)
	}

	res := f.adopt(b2.ID, direct.ID, "")

	row, err := f.st.GetNamedRepo(direct.ID)
	if res["ok"] != true || err != nil || row.PlaceID != b2.ID || row.PlaceSuffix != "-direct" || row.Repo != direct.Repo || row.CredsRef != "kept" {
		t.Fatalf("adopt = %v; row %+v, %v, want it at the place on its own credentials", res, row, err)
	}
	if len(f.eng.opened) != 0 {
		t.Fatalf("adopting a direct repository at its own address opened %v", f.eng.opened)
	}
}
