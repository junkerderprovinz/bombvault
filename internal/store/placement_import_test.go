package store_test

import (
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestAnImportWithoutPlacementBlocksChangesNothing(t *testing.T) {
	r := repoChoiceStore(t)
	store.SeedDefault(t, r, "containers", "", "t-b2")
	store.SeedCopyRule(t, r, "vms", "vm:win11", store.SkipAll)
	if err := r.ImportPlacement(store.PlacementImport{}, store.Installed{}); err != nil {
		t.Fatal(err)
	}
	if d, found, err := r.PlacementDefaultFor("containers"); err != nil || !found || len(d.Skip) != 1 {
		t.Fatalf("default = %+v, %v, %v, want it kept", d, found, err)
	}
	if _, found := store.RuleSkip(t, r, "vms", "vm:win11"); !found {
		t.Fatal("the rule went although the file carried no rules block")
	}
}

func TestAnEmptyRulesBlockClearsEveryRule(t *testing.T) {
	r := repoChoiceStore(t)
	store.SeedCopyRule(t, r, "vms", "vm:win11", store.SkipAll)
	if err := r.ImportPlacement(store.PlacementImport{HasRules: true}, store.Installed{}); err != nil {
		t.Fatal(err)
	}
	rules, err := r.ListCopyRules()
	if err != nil || len(rules) != 0 {
		t.Fatalf("rules = %v, %v, want none", rules, err)
	}
}

func TestImportedDefaultsReplaceTheConfirmedOnes(t *testing.T) {
	r := repoChoiceStore(t)
	store.SeedDefault(t, r, "containers", "", "t-b2")
	store.SeedDefault(t, r, "vms", "")
	err := r.ImportPlacement(store.PlacementImport{HasDefaults: true, Defaults: []store.PlacementDefault{
		{Domain: "containers", Home: "repo-nas", Skip: []string{store.SkipAll}},
	}}, store.Installed{})
	if err != nil {
		t.Fatal(err)
	}
	d, found, err := r.PlacementDefaultFor("containers")
	if err != nil || !found || d.Home != "repo-nas" || d.Paused() || len(d.Skip) != 1 || d.Skip[0] != store.SkipAll {
		t.Fatalf("containers = %+v, %v, %v, want the file's default, confirmed", d, found, err)
	}
	if _, found, _ := r.PlacementDefaultFor("vms"); found {
		t.Fatal("a default the file does not name survived")
	}
}

func TestAPausedDomainStaysPausedThroughAnImport(t *testing.T) {
	r := repoChoiceStore(t)
	if _, err := r.PausePlacement("files"); err != nil {
		t.Fatal(err)
	}
	err := r.ImportPlacement(store.PlacementImport{HasDefaults: true, Defaults: []store.PlacementDefault{
		{Domain: "files", Home: "repo-nas", Skip: []string{"t-b2"}},
	}}, store.Installed{})
	if err != nil {
		t.Fatal(err)
	}
	d, found, err := r.PlacementDefaultFor("files")
	if err != nil || !found || !d.Paused() || d.Home != "repo-nas" || len(d.Skip) != 1 {
		t.Fatalf("files = %+v, %v, %v, want the file's values and still paused", d, found, err)
	}
	if err := r.ImportPlacement(store.PlacementImport{HasDefaults: true}, store.Installed{}); err != nil {
		t.Fatal(err)
	}
	if d, found, _ := r.PlacementDefaultFor("files"); !found || !d.Paused() {
		t.Fatalf("an empty block removed or confirmed the paused row: %+v, %v", d, found)
	}
}

func TestAConfirmedDefaultKeepsItsManualMarkerThroughAnImportThatNamesIt(t *testing.T) {
	r := repoChoiceStore(t)
	if _, err := r.PausePlacement("containers"); err != nil {
		t.Fatal(err)
	}
	if err := r.ConfirmPlacement("containers", nil); err != nil {
		t.Fatal(err)
	}
	err := r.ImportPlacement(store.PlacementImport{HasDefaults: true, Defaults: []store.PlacementDefault{
		{Domain: "containers", Home: "repo-nas", Skip: []string{store.SkipAll}},
	}}, store.Installed{})
	if err != nil {
		t.Fatal(err)
	}
	d, found, err := r.PlacementDefaultFor("containers")
	if err != nil || !found || !d.ConfirmedManually {
		t.Fatalf("containers = %+v, %v, %v, want the manual marker kept", d, found, err)
	}
}

func TestAConfirmedDefaultIsDroppedByAnImportThatDoesNotNameIt(t *testing.T) {
	r := repoChoiceStore(t)
	if _, err := r.PausePlacement("containers"); err != nil {
		t.Fatal(err)
	}
	if err := r.ConfirmPlacement("containers", nil); err != nil {
		t.Fatal(err)
	}
	if err := r.ImportPlacement(store.PlacementImport{HasDefaults: true}, store.Installed{}); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := r.PlacementDefaultFor("containers"); found {
		t.Fatal("a confirmed default the file does not name survived")
	}
}

func TestAProjectFolderRuleRefusesTheWholeImport(t *testing.T) {
	r := repoChoiceStore(t)
	store.SeedCopyRule(t, r, "vms", "vm:win11", store.SkipAll)
	err := r.ImportPlacement(store.PlacementImport{
		HasDefaults: true,
		HasRules:    true,
		Rules:       []store.CopyRule{{Domain: "containers", Identity: "stack:immich", Skip: []string{}}},
	}, store.Installed{})
	if !errors.Is(err, store.ErrStackCopyRule) {
		t.Fatalf("err = %v, want ErrStackCopyRule", err)
	}
	if _, found := store.RuleSkip(t, r, "vms", "vm:win11"); !found {
		t.Fatal("a refused import still cleared the rules")
	}
}

func TestAMalformedSkipRefusesTheImport(t *testing.T) {
	r := repoChoiceStore(t)
	err := r.ImportPlacement(store.PlacementImport{HasRules: true, Rules: []store.CopyRule{
		{Domain: "vms", Identity: "vm:win11", Skip: []string{store.SkipAll, "t-b2"}},
	}}, store.Installed{})
	if !errors.Is(err, store.ErrBadSkip) {
		t.Fatalf("err = %v, want ErrBadSkip", err)
	}
}

func TestSkipIdsWithoutATargetStayInTheRule(t *testing.T) {
	r := repoChoiceStore(t)
	err := r.ImportPlacement(store.PlacementImport{HasRules: true, Rules: []store.CopyRule{
		{Domain: "vms", Identity: "vm:win11", Skip: []string{"t-gone"}},
	}}, store.Installed{})
	if err != nil {
		t.Fatal(err)
	}
	if skip, found := store.RuleSkip(t, r, "vms", "vm:win11"); !found || len(skip) != 1 || skip[0] != "t-gone" {
		t.Fatalf("rule = %v, %v, want [t-gone]", skip, found)
	}
}
