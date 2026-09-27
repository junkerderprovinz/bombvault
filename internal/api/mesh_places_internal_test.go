package api

import (
	"maps"
	"net/http"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// offerFrom leaves a pending offer of a peer's rest-server at repo.
func (f *placementFixture) offerFrom(from, domain, repo string) store.MeshOffer {
	f.t.Helper()
	enc, err := secret.Encrypt(f.h.cfg.AppKey, []byte("peer-password"))
	if err != nil {
		f.t.Fatal(err)
	}
	offer, err := f.st.CreateMeshOffer(store.MeshOffer{From: from, SuggestedDomain: domain, Repo: repo, RESTUser: "bv", RESTPasswordEnc: enc})
	if err != nil {
		f.t.Fatal(err)
	}
	return offer
}

func (f *placementFixture) accept(offer store.MeshOffer, domain string) map[string]any {
	f.t.Helper()
	return f.do(http.MethodPost, "/api/fleet/mesh-offers/"+offer.ID+"/accept", map[string]any{"domain": domain})
}

func TestAnAcceptedOfferBringsAPlaceForItsDomainAlone(t *testing.T) {
	f := newPlacementFixture(t)
	offer := f.meshOffer()

	res := f.accept(offer, "containers")

	view, _ := res["place"].(map[string]any)
	if res["ok"] != true || view["name"] != "mesh: tower-a" || view["provider"] != "bombvault" || view["kind"] != "rest" ||
		view["base"] != "rest:http://192.0.2.10:8000/bv" || view["offPremises"] != true {
		t.Fatalf("accept = %v, want a place for the offer", res)
	}
	all, err := f.st.ListPlaces()
	if err != nil || len(all) != 1 || !maps.Equal(all[0].Folders, map[string]string{"containers": "containers"}) {
		t.Fatalf("places = %+v, %v, want one serving containers alone", all, err)
	}
	p := all[0]
	targetID, _ := res["target"].(map[string]any)["id"].(string)
	target, found, err := f.st.GetOffsiteTarget(targetID)
	if err != nil || !found || target.PlaceID != p.ID || target.PlaceDomain != "containers" || target.Repo != offer.Repo || target.CredsRef != p.CredsRef {
		t.Fatalf("target = %+v, %v, %v, want it at the place, on the offer's address", target, found, err)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	sets, err := f.svc.decodeCloudCredSets(settings)
	if err != nil || len(sets) != 1 || sets[0].ID != p.CredsRef || sets[0].Kind != "rest" ||
		sets[0].RESTUser != "bv" || sets[0].RESTPassword != "peer-password" {
		t.Fatalf("sets = %+v, %v, want the peer's login as the place's set", sets, err)
	}
}

func TestASecondOfferFromOnePeerGetsAPlaceOfItsOwn(t *testing.T) {
	f := newPlacementFixture(t)
	f.accept(f.meshOffer(), "containers")
	res := f.accept(f.offerFrom("tower-a", "vms", "rest:http://192.0.2.10:8000/bv-vms/vms"), "vms")
	if view, _ := res["place"].(map[string]any); res["ok"] != true || view["name"] != "mesh: tower-a 2" {
		t.Fatalf("accept = %v, want a second place with a name of its own", res)
	}
}

func TestAnOfferThatCannotBeMarkedAcceptedLeavesNoPlace(t *testing.T) {
	f := newPlacementFixture(t)
	offer := f.meshOffer()
	f.offerStatusStays()
	if res := f.accept(offer, "containers"); res["ok"] != false {
		t.Fatalf("accept = %v, want a refusal", res)
	}
	if all, err := f.st.ListPlaces(); err != nil || len(all) != 0 {
		t.Fatalf("places = %+v, %v, want none", all, err)
	}
	if s, err := f.st.GetSettings(); err != nil || s.ContainersOffsite != "" {
		t.Fatalf("the off-site field = %q, %v, want it empty again", s.ContainersOffsite, err)
	}
}

func TestAnOfferNoPlaceCanHoldIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	offer := f.offerFrom("tower-a", "containers", "rest:http://192.0.2.10:8000/bv/containers/")
	if res := f.accept(offer, "containers"); res["ok"] != false {
		t.Fatalf("accept = %v, want a refusal", res)
	}
	all, err := f.st.ListPlaces()
	targets, tErr := f.st.OffsiteTargetsForDomain("containers")
	again, _, gErr := f.st.GetMeshOffer(offer.ID)
	if err != nil || len(all) != 0 || tErr != nil || len(targets) != 0 || gErr != nil || again.Status != "pending" {
		t.Fatalf("places %v, targets %v, offer %q: want nothing written", all, targets, again.Status)
	}
}

func TestDroppingAnOffersSetKeepsTheSetsWrittenMeanwhile(t *testing.T) {
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCredSets([]CloudCredSet{
		{ID: "set-offer", Name: "mesh: tower-a", Kind: "rest", CloudCreds: CloudCreds{RESTUser: "bv", RESTPassword: "peer-password"}},
		{ID: "set-b2", Name: "B2", CloudCreds: CloudCreds{S3KeyID: "K005", S3Secret: "old-secret"}},
	}); err != nil {
		t.Fatal(err)
	}

	f.svc.credSetsMu.Lock()
	done := make(chan error, 1)
	go func() { done <- f.h.dropCredSet("set-offer") }()
	// Long enough for a drop that reads the sets before the lock to read them.
	time.Sleep(100 * time.Millisecond)
	// What a place write does under the lock: a new secret for one set and a
	// set of its own.
	if _, err := f.st.MutateSettings(func(s *store.Settings) error {
		sets, err := f.svc.decodeCloudCredSets(*s)
		if err != nil {
			return err
		}
		for i := range sets {
			if sets[i].ID == "set-b2" {
				sets[i].S3Secret = "new-secret"
			}
		}
		sets = append(sets, CloudCredSet{ID: "set-nas", Name: "NAS", WebDAVUser: "anna", WebDAVPass: "pw"})
		s.CloudCredSets, err = f.svc.encodeCloudCredSets(sets)
		return err
	}); err != nil {
		f.svc.credSetsMu.Unlock()
		t.Fatal(err)
	}
	f.svc.credSetsMu.Unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the drop never finished")
	}

	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	sets, err := f.svc.decodeCloudCredSets(settings)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]CloudCredSet{}
	for _, s := range sets {
		byID[s.ID] = s
	}
	if _, left := byID["set-offer"]; left {
		t.Error("the offer's set is still there")
	}
	if got := byID["set-b2"].S3Secret; got != "new-secret" {
		t.Errorf("B2 secret = %q, want the one written meanwhile", got)
	}
	if _, ok := byID["set-nas"]; !ok {
		t.Errorf("sets = %+v, want the set written meanwhile kept", sets)
	}
}
