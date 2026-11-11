package api

import (
	"encoding/json"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func appImportOn(t *testing.T, f *placementFixture) bool {
	t.Helper()
	s, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	return s.AppImportEnabled
}

func TestTheSwitchForGivingBackupsBackIsSavedAndKept(t *testing.T) {
	f := newPlacementFixture(t)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	v := toView(settings)
	if v.AppImportEnabled == nil || *v.AppImportEnabled {
		t.Fatalf("the settings say %v, want the switch there and off", v.AppImportEnabled)
	}

	on := true
	v.AppImportEnabled = &on
	if res := f.do("PUT", "/api/settings", v); res["ok"] != true {
		t.Fatalf("PUT: %v", res["error"])
	}
	if !appImportOn(t, f) {
		t.Fatal("the switch did not go on")
	}

	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var older map[string]any
	if err := json.Unmarshal(b, &older); err != nil {
		t.Fatal(err)
	}
	delete(older, "appImportEnabled")
	if res := f.do("PUT", "/api/settings", older); res["ok"] != true {
		t.Fatalf("PUT without the switch: %v", res["error"])
	}
	if !appImportOn(t, f) {
		t.Fatal("a body without the switch turned it off")
	}
}

func TestSettingsExportImportCarriesTheSwitchForGivingBackupsBack(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	if _, err := srcStore.MutateSettings(func(s *store.Settings) error { s.AppImportEnabled = true; return nil }); err != nil {
		t.Fatal(err)
	}
	body, _ := doExport(t, src, "")

	dst, dstStore := newPortableHandler(t, appKeyB)
	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply failed: %v", env)
	}
	if got, err := dstStore.GetSettings(); err != nil || !got.AppImportEnabled {
		t.Fatalf("switch = %v with %v, want it carried over", got.AppImportEnabled, err)
	}

	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	delete(raw["settings"].(map[string]any), "appImportEnabled")
	older, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if env := doImport(t, dst, older, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply of a file without the switch failed: %v", env)
	}
	if got, err := dstStore.GetSettings(); err != nil || !got.AppImportEnabled {
		t.Fatalf("switch = %v with %v, want a file without it to keep it", got.AppImportEnabled, err)
	}
}
