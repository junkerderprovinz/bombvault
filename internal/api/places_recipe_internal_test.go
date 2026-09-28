package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/places"
)

func TestTheRestServerRecipeMakesOneUserForThisBombVault(t *testing.T) {
	f := newPlacementFixture(t)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.InstanceName = "Tower 2"
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	res := f.do(http.MethodGet, "/api/places/rest-server-recipe", nil)

	snip, _ := res["snippet"].(map[string]any)
	htpasswd, _ := snip["htpasswd"].(string)
	if res["ok"] != true || snip["user"] != "tower-2" || snip["password"] == "" || !strings.HasPrefix(htpasswd, "tower-2:$2") {
		t.Fatalf("recipe = %v, want one user named after this BombVault", res)
	}
	for _, key := range []string{"dockerRun", "compose", "unraid"} {
		recipe, _ := snip[key].(string)
		if !strings.Contains(recipe, "--append-only") || !strings.Contains(recipe, htpasswd) ||
			!strings.Contains(recipe, "user tower-2") || strings.Contains(recipe, "bombvault-") {
			t.Errorf("%s = %q, want the append-only server with this one login and where to use it", key, recipe)
		}
	}
}

func TestARecipeUserIsTheInstanceNameInPlainLetters(t *testing.T) {
	for name, want := range map[string]string{"Tower 2": "tower-2", "  ": "bombvault", "NAS (Keller)": "nas-keller", "": "bombvault"} {
		if got := recipeUser(name); got != want {
			t.Errorf("recipeUser(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestAPlaceAddedFromTheRecipeStartsAppendOnly(t *testing.T) {
	f := newPlacementFixture(t)
	f.probeAnswers(places.ProbeResult{OK: true, Base: "rest:https://backup.example.com/tower"})

	res := f.do(http.MethodPost, "/api/places", map[string]any{
		"provider": "rest-server", "name": "Rest server", "offPremises": true, "immutable": true,
		"fields": map[string]string{"url": "https://backup.example.com", "user": "tower", "password": "pw"},
	})

	all, err := f.st.ListPlaces()
	if res["ok"] != true || err != nil || len(all) != 1 || !all[0].Immutable {
		t.Fatalf("POST /api/places = %v; places %+v, %v, want the place append-only", res, all, err)
	}
}

func TestAFolderPlaceCannotStartAppendOnly(t *testing.T) {
	f := newPlacementFixture(t)
	f.probeAnswers(places.ProbeResult{OK: true, Base: "nas"})

	_, res := f.doStatus(http.MethodPost, "/api/places", map[string]any{
		"provider": "unraid-folder", "name": "NAS", "immutable": true, "fields": map[string]string{"path": "nas"},
	})

	if res["ok"] == true || res["code"] != "place-no-append-only" {
		t.Fatalf("POST /api/places = %v, want the local append-only refusal", res)
	}
}
