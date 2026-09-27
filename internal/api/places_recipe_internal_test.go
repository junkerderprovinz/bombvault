package api

import (
	"net/http"
	"strings"
	"testing"
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
