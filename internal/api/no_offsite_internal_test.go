package api

import (
	"encoding/json"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func (f *placementFixture) setNoOffsiteByChoice(on bool) {
	f.t.Helper()
	if _, err := f.st.MutateSettings(func(s *store.Settings) error {
		s.ContainersEnabled = true
		s.NoOffsiteByChoice = on
		return nil
	}); err != nil {
		f.t.Fatal(err)
	}
}

func noOffsiteByChoice(t *testing.T, st *store.Repo) bool {
	t.Helper()
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	return s.NoOffsiteByChoice
}

func TestNoOffsiteByChoiceIsSavedAndKept(t *testing.T) {
	f := newPlacementFixture(t)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	v := toView(settings)
	if v.NoOffsiteByChoice == nil || *v.NoOffsiteByChoice {
		t.Fatalf("the settings say %v, want the switch there and off", v.NoOffsiteByChoice)
	}

	on := true
	v.NoOffsiteByChoice = &on
	if res := f.do("PUT", "/api/settings", v); res["ok"] != true {
		t.Fatalf("PUT: %v", res["error"])
	}
	if !noOffsiteByChoice(t, f.st) {
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
	delete(older, "noOffsiteByChoice")
	if res := f.do("PUT", "/api/settings", older); res["ok"] != true {
		t.Fatalf("PUT without the switch: %v", res["error"])
	}
	if !noOffsiteByChoice(t, f.st) {
		t.Fatal("a body without the switch turned it off")
	}
}

func TestSettingsExportImportCarriesNoOffsiteByChoice(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	if _, err := srcStore.MutateSettings(func(s *store.Settings) error { s.NoOffsiteByChoice = true; return nil }); err != nil {
		t.Fatal(err)
	}
	body, _ := doExport(t, src, "")

	dst, dstStore := newPortableHandler(t, appKeyB)
	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply failed: %v", env)
	}
	if !noOffsiteByChoice(t, dstStore) {
		t.Fatal("the switch was not carried over")
	}

	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	delete(raw["settings"].(map[string]any), "noOffsiteByChoice")
	older, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if env := doImport(t, dst, older, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply of a file without the switch failed: %v", env)
	}
	if !noOffsiteByChoice(t, dstStore) {
		t.Fatal("a file without the switch turned it off")
	}
}

func TestASectionKeptOnThePremisesByChoiceIsProtected(t *testing.T) {
	f := newPlacementFixture(t)
	f.setNoOffsiteByChoice(true)
	d := f.domainStatus("containers")
	if !d.NoOffsiteByChoice || d.OffsiteConfigured {
		t.Fatalf("by choice %v, off-site %v, want the choice and no off-site", d.NoOffsiteByChoice, d.OffsiteConfigured)
	}
	if d.Protection != "green" {
		t.Errorf("protection = %q, want green with the other checks clear", d.Protection)
	}

	f.setNoOffsiteByChoice(false)
	d = f.domainStatus("containers")
	if d.NoOffsiteByChoice || d.Protection != "red" {
		t.Errorf("by choice %v, protection %q, want the missing copy red again", d.NoOffsiteByChoice, d.Protection)
	}
}

func TestAnOffsiteCopyAnywhereEndsTheChoice(t *testing.T) {
	f := newPlacementFixture(t)
	f.setNoOffsiteByChoice(true)
	f.target("flash", "Storage Box", "rest:http://box:8000/flash")

	d := f.domainStatus("containers")
	if d.NoOffsiteByChoice {
		t.Error("a section without an off-site copy still reads as by choice while flash copies off-site")
	}
	if d.Protection != "red" {
		t.Errorf("protection = %q, want red for the section without a copy", d.Protection)
	}
	if f.domainStatus("flash").NoOffsiteByChoice {
		t.Error("the section with the copy reads as by choice")
	}
}
