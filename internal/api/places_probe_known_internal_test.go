package api

import (
	"context"
	"maps"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestANewPlaceWhoseAddressIsARepositoryIsOfferedAsOne(t *testing.T) {
	f := newPlacementFixture(t)
	eng := newEnvEngine(f)
	eng.ids["rest:https://nas:8000/tower"] = "id-tower"
	res := probeOf(t, f, "rest-server", map[string]string{"url": "https://nas:8000", "user": "tower", "password": "pw"})
	if !res.OK || res.Folders != nil || res.RepoIDs[""] != "id-tower" {
		t.Fatalf("probe = %+v, want the address offered as one repository", res)
	}
	if !slices.ContainsFunc(res.Facts, func(f places.ProbeFact) bool { return f.Key == places.FactBaseIsRepository }) {
		t.Errorf("facts = %v", res.Facts)
	}
}

func TestAStoredPlaceIsProbedWithItsSecretsUnlessTheFormHoldsNewOnes(t *testing.T) {
	f := newPlacementFixture(t)
	eng := newEnvEngine(f)
	if err := f.svc.SetCloudCredSets([]CloudCredSet{{ID: "nas", Name: "NAS", Kind: "rest", CloudCreds: CloudCreds{RESTUser: "tower", RESTPassword: "stored"}}}); err != nil {
		t.Fatal(err)
	}
	place, err := f.st.WritePlace(store.PlaceWrite{Place: store.Place{
		Name: "NAS", Provider: "rest-server", Kind: "rest", Base: "rest:https://nas:8000/tower",
		Folders: map[string]string{"containers": "container"}, CredsRef: "nas", Enabled: true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	addr := "rest:https://nas:8000/tower/container"
	f.eng.opens[addr] = false

	res, err := f.svc.ProbePlace(context.Background(), ProbeRequest{PlaceID: place.ID})
	if err != nil {
		t.Fatal(err)
	}
	if res.Base != place.Base || !slices.Equal(slices.Sorted(maps.Keys(res.Folders)), []string{"containers"}) {
		t.Fatalf("probe = %+v, want the stored address and folders", res)
	}
	if env := eng.env(addr); !slices.Contains(env, "RESTIC_REST_PASSWORD=stored") {
		t.Fatalf("env = %v, want the stored password", env)
	}
	if _, err := f.svc.ProbePlace(context.Background(), ProbeRequest{PlaceID: place.ID, Fields: map[string]string{"password": "typed"}}); err != nil {
		t.Fatal(err)
	}
	if env := eng.env(addr); !slices.Contains(env, "RESTIC_REST_PASSWORD=typed") || !slices.Contains(env, "RESTIC_REST_USERNAME=tower") {
		t.Fatalf("env = %v, want the typed password and the stored user", env)
	}
}
