package api

import (
	"net/http"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func restForm(path string) map[string]string {
	return map[string]string{"url": "http://nas:8000", "user": "bvp", "password": "pw", "path": path}
}

func TestARestServerPathOfTwoFoldersIsRefusedByTheProbe(t *testing.T) {
	f := newPlacementFixture(t)
	eng := newEnvEngine(f)
	res := probeOf(t, f, "rest-server", restForm("/bvp/ao/"))
	if res.OK || res.Code != "place-rest-path-deep" {
		t.Fatalf("probe = %+v, want place-rest-path-deep", res)
	}
	if n := eng.openCount("rest:http://nas:8000/bvp/ao/files"); n != 0 {
		t.Fatalf("the probe opened a folder %d times below a path rest-server cannot hold", n)
	}
}

func TestARestServerRepositoryTwoFoldersDeepIsStillOffered(t *testing.T) {
	f := newPlacementFixture(t)
	eng := newEnvEngine(f)
	eng.ids["rest:http://nas:8000/bvp/ao"] = "id-ao"
	res := probeOf(t, f, "rest-server", restForm("bvp/ao"))
	if !res.OK || res.RepoIDs[""] != "id-ao" {
		t.Fatalf("probe = %+v, want the repository at the path", res)
	}
}

func TestARestServerPlaceOfTwoFoldersIsNotCreated(t *testing.T) {
	f := newPlacementFixture(t)
	newEnvEngine(f)
	res := f.do(http.MethodPost, "/api/places", map[string]any{
		"provider": "rest-server", "name": "rest", "offPremises": false, "fields": restForm("bvp/ao"),
	})
	probe, _ := res["probe"].(map[string]any)
	if res["ok"] != false || probe["code"] != "place-rest-path-deep" {
		t.Fatalf("POST /api/places = %v, want the probe's place-rest-path-deep", res)
	}
	if got := placesOf(t, f); len(got) != 0 {
		t.Fatalf("places = %+v, want none", got)
	}
}

func TestARestServerPlaceCannotMoveToAPathOfTwoFolders(t *testing.T) {
	f := newPlacementFixture(t)
	p := f.storePlace(store.Place{Name: "rest", Provider: "rest-server", Kind: string(places.KindREST),
		Base: "rest:http://nas:8000/bvp", Folders: places.DefaultFolders(), Enabled: true})

	res := f.do(http.MethodPatch, "/api/places/"+p.ID, map[string]any{"address": restForm("bvp/ao")})

	if res["ok"] != false || res["code"] != "place-rest-path-deep" {
		t.Fatalf("PATCH = %v, want place-rest-path-deep", res)
	}
	if got, err := f.st.GetPlace(p.ID); err != nil || got.Base != p.Base {
		t.Fatalf("place = %+v, %v, want its base unchanged", got, err)
	}
}
