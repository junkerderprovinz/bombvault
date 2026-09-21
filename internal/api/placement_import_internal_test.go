package api

import (
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func importEdited(t *testing.T, f *placementFixture, edit func(exp map[string]any)) map[string]any {
	t.Helper()
	exp := f.do(http.MethodGet, "/api/settings/export", nil)
	edit(exp)
	return f.do(http.MethodPost, "/api/settings/import?apply=true", exp)
}

func TestTheExportAlwaysCarriesBothLists(t *testing.T) {
	f := newPlacementFixture(t)
	exp := f.do(http.MethodGet, "/api/settings/export", nil)
	if list, ok := exp["placementDefaults"].([]any); !ok || len(list) != 0 {
		t.Errorf("placementDefaults = %v, want an empty list", exp["placementDefaults"])
	}
	if list, ok := exp["copyRules"].([]any); !ok || len(list) != 0 {
		t.Errorf("copyRules = %v, want an empty list", exp["copyRules"])
	}
}

func TestDefaultsAndRulesSurviveARoundTrip(t *testing.T) {
	src := newPlacementFixture(t)
	nas := src.namedRepo("NAS", "nas")
	b2 := src.target("containers", "B2", "b2:bucket/containers")
	src.setDefault("containers", nas.ID, b2.ID)
	src.rule("containers", "container:plex", store.SkipAll)
	exp := src.do(http.MethodGet, "/api/settings/export", nil)
	if len(exp["placementDefaults"].([]any)) != 1 || len(exp["copyRules"].([]any)) != 1 {
		t.Fatalf("export = %v, want one default and one rule", exp)
	}
	dst := newPlacementFixture(t)
	if res := dst.do(http.MethodPost, "/api/settings/import?apply=true", exp); res["ok"] != true {
		t.Fatalf("import = %v", res)
	}
	back := dst.do(http.MethodGet, "/api/settings/export", nil)
	if !reflect.DeepEqual(back["placementDefaults"], exp["placementDefaults"]) || !reflect.DeepEqual(back["copyRules"], exp["copyRules"]) {
		t.Fatalf("round trip changed the blocks:\n got %v %v\nwant %v %v", back["placementDefaults"], back["copyRules"], exp["placementDefaults"], exp["copyRules"])
	}
}

func TestAFileWithoutPlacementBlocksLeavesThemAlone(t *testing.T) {
	f := newPlacementFixture(t)
	f.setDefault("containers", "")
	f.rule("vms", "vm:win11", store.SkipAll)
	res := importEdited(t, f, func(exp map[string]any) {
		delete(exp, "placementDefaults")
		delete(exp, "copyRules")
	})
	if res["ok"] != true {
		t.Fatalf("import = %v", res)
	}
	if _, found, _ := f.st.CopyRuleFor("vms", "vm:win11"); !found {
		t.Error("the rule went although the file carried no rules block")
	}
	if _, found, _ := f.st.PlacementDefaultFor("containers"); !found {
		t.Error("the default went although the file carried no defaults block")
	}
}

func TestAnEmptyCopyRulesBlockClearsTheRules(t *testing.T) {
	f := newPlacementFixture(t)
	f.rule("vms", "vm:win11", store.SkipAll)
	res := importEdited(t, f, func(exp map[string]any) { exp["copyRules"] = []any{} })
	if res["ok"] != true {
		t.Fatalf("import = %v", res)
	}
	if _, found, _ := f.st.CopyRuleFor("vms", "vm:win11"); found {
		t.Error("an empty rules block left the rule in place")
	}
}

func TestAnImportKeepsAPausedDomainPaused(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("files", "B2", "b2:bucket/files")
	f.paused("files")
	res := importEdited(t, f, func(exp map[string]any) {
		exp["placementDefaults"] = []any{map[string]any{"domain": "files", "home": "", "skip": []any{b2.ID}}}
	})
	if res["ok"] != true {
		t.Fatalf("import = %v", res)
	}
	d, found, err := f.st.PlacementDefaultFor("files")
	if err != nil || !found || !d.Paused() || len(d.Skip) != 1 || d.Skip[0] != b2.ID {
		t.Fatalf("files = %+v, %v, %v, want the file's skip and still paused", d, found, err)
	}
	res = importEdited(t, f, func(exp map[string]any) { exp["placementDefaults"] = []any{} })
	if res["ok"] != true {
		t.Fatalf("import = %v", res)
	}
	if d, found, _ := f.st.PlacementDefaultFor("files"); !found || !d.Paused() {
		t.Fatalf("an empty block ended the pause: %+v, %v", d, found)
	}
}

func TestADefaultOnAnUnknownRepositoryRefusesTheImport(t *testing.T) {
	f := newPlacementFixture(t)
	f.rule("vms", "vm:win11", store.SkipAll)
	res := importEdited(t, f, func(exp map[string]any) {
		exp["placementDefaults"] = []any{map[string]any{"domain": "containers", "home": "no-such-repo", "skip": []any{}}}
		exp["copyRules"] = []any{}
	})
	if res["code"] != "default-repo-missing" {
		t.Fatalf("import = %v, want default-repo-missing", res)
	}
	if _, found, _ := f.st.CopyRuleFor("vms", "vm:win11"); !found {
		t.Error("a refused import still cleared the rules")
	}
}

func TestADefaultSkipNamingAnUnknownTargetRefusesTheImport(t *testing.T) {
	f := newPlacementFixture(t)
	f.rule("vms", "vm:win11", store.SkipAll)
	res := importEdited(t, f, func(exp map[string]any) {
		exp["placementDefaults"] = []any{map[string]any{"domain": "containers", "home": "", "skip": []any{"no-such-target"}}}
	})
	if res["code"] != "unknown-target" {
		t.Fatalf("import = %v, want unknown-target", res)
	}
	if _, found, _ := f.st.CopyRuleFor("vms", "vm:win11"); !found {
		t.Error("a refused import still cleared the rules")
	}
}

// TestADefaultSkipNamingATargetTheFileItselfBringsIsAccepted pins that a skip
// entry is checked against the targets the import is about to create, not
// only the ones already stored: dst has none of its own yet, and B2 arrives
// in the same file as the default that excludes it.
func TestADefaultSkipNamingATargetTheFileItselfBringsIsAccepted(t *testing.T) {
	src := newPlacementFixture(t)
	b2 := src.target("containers", "B2", "b2:bucket/containers")
	src.setDefault("containers", "", b2.ID)
	exp := src.do(http.MethodGet, "/api/settings/export", nil)

	dst := newPlacementFixture(t)
	if res := dst.do(http.MethodPost, "/api/settings/import?apply=true", exp); res["ok"] != true {
		t.Fatalf("import = %v", res)
	}
	d, found, err := dst.st.PlacementDefaultFor("containers")
	if err != nil || !found || len(d.Skip) != 1 || d.Skip[0] != b2.ID {
		t.Fatalf("containers default = %+v, %v, %v, want the file's own target kept in its skip", d, found, err)
	}
}

// TestADefaultSkipNamingATargetOnlyTheDatabaseHasRefusesTheImport is the
// mirror of TestADefaultSkipNamingATargetTheFileItselfBringsIsAccepted: the
// import always replaces every off-site target with the file's own set, so a
// skip entry naming a target that only the current database still has, and
// the file no longer carries, would exclude nothing once the import lands.
func TestADefaultSkipNamingATargetOnlyTheDatabaseHasRefusesTheImport(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket/containers")
	f.setDefault("containers", "", b2.ID)
	res := importEdited(t, f, func(exp map[string]any) {
		exp["offsiteTargets"] = []any{}
	})
	if res["code"] != "unknown-target" {
		t.Fatalf("import = %v, want unknown-target", res)
	}
}

// TestTwoDefaultsForTheSameDomainRefuseTheImport pins that the pre-flight
// catches a duplicate domain itself: ImportPlacement's insert refuses the
// second row only after settings and the off-site targets have already been
// replaced, which is too late.
func TestTwoDefaultsForTheSameDomainRefuseTheImport(t *testing.T) {
	f := newPlacementFixture(t)
	res := importEdited(t, f, func(exp map[string]any) {
		exp["placementDefaults"] = []any{
			map[string]any{"domain": "containers", "home": "", "skip": []any{}},
			map[string]any{"domain": "containers", "home": "", "skip": []any{}},
		}
	})
	if res["code"] != "invalid-placement" {
		t.Fatalf("import = %v, want invalid-placement", res)
	}
	if _, found, _ := f.st.PlacementDefaultFor("containers"); found {
		t.Error("a refused import still wrote a default")
	}
}

// TestADefaultOnADisabledRepositoryRefusesTheImport pins that the home is
// checked the way every other writer checks a repository choice: it must be
// able to take a backup, not merely exist. NAS is dropped from the file's own
// namedRepos block so the check falls back to the stored row, which is
// switched off.
func TestADefaultOnADisabledRepositoryRefusesTheImport(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	nas.Enabled = false
	if _, err := f.st.UpsertOffsiteTarget(nas); err != nil {
		t.Fatal(err)
	}
	f.setDefault("containers", nas.ID)
	res := importEdited(t, f, func(exp map[string]any) {
		var keep []any
		for _, row := range exp["namedRepos"].([]any) {
			if row.(map[string]any)["name"] != "NAS" {
				keep = append(keep, row)
			}
		}
		exp["namedRepos"] = keep
	})
	if res["code"] != "repo-invalid" {
		t.Fatalf("import = %v, want repo-invalid", res)
	}
}

// TestADefaultOnAFileOwnDisabledRepositoryRefusesTheImport is the mirror of
// TestADefaultOnADisabledRepositoryRefusesTheImport: NAS is not yet in the
// database, so the check has only the file's own namedRepos block to judge
// it by, and that block carries whether a repository is switched off too.
func TestADefaultOnAFileOwnDisabledRepositoryRefusesTheImport(t *testing.T) {
	src := newPlacementFixture(t)
	nas := src.namedRepo("NAS", "nas")
	src.setDefault("containers", nas.ID)
	exp := src.do(http.MethodGet, "/api/settings/export", nil)
	for _, row := range exp["namedRepos"].([]any) {
		row.(map[string]any)["enabled"] = false
	}

	dst := newPlacementFixture(t)
	res := dst.do(http.MethodPost, "/api/settings/import?apply=true", exp)
	if res["code"] != "repo-invalid" {
		t.Fatalf("import = %v, want repo-invalid", res)
	}
}

func TestAnImportRefusesARuleWhoseIdentityMismatchesItsDomain(t *testing.T) {
	f := newPlacementFixture(t)
	exp := settingsExport{CopyRules: []copyRuleExport{{Domain: "containers", Identity: "vm:win11"}}}
	if err := f.h.checkImportedPlacement(exp); !errors.Is(err, store.ErrRuleDomain) {
		t.Fatalf("checkImportedPlacement = %v, want store.ErrRuleDomain", err)
	}
}

// TestAnImportJudgesTheFilesOwnDirectRepositoryByItsFileTarget pins that the
// shortcut for a repository the file itself carries also judges the domain a
// direct repository serves, using the target the same file describes, not
// only whether the repository is switched off.
func TestAnImportJudgesTheFilesOwnDirectRepositoryByItsFileTarget(t *testing.T) {
	f := newPlacementFixture(t)
	exp := settingsExport{
		OffsiteTargets: []offsiteTargetView{{ID: "target1", Domain: "vms", Name: "B2", Repo: "b2:bkt:vms", Enabled: true}},
		NamedRepos:     []offsiteTargetView{{ID: "repo1", Name: "VMs direct", Repo: "b2:bkt:vms-direct", Enabled: true, CompanionOf: "target1"}},
	}
	exp.PlacementDefaults = []placementDefaultExport{{Domain: "containers", Home: "repo1", Skip: []string{}}}
	if err := f.h.checkImportedPlacement(exp); !errors.Is(err, errForeignDomain) {
		t.Fatalf("a containers default on the file's own VMs direct repository = %v, want errForeignDomain", err)
	}
	exp.PlacementDefaults = []placementDefaultExport{{Domain: "vms", Home: "repo1", Skip: []string{}}}
	if err := f.h.checkImportedPlacement(exp); err != nil {
		t.Fatalf("a vms default on the file's own VMs direct repository = %v", err)
	}
}

func TestAProjectFolderRuleRefusesTheImport(t *testing.T) {
	f := newPlacementFixture(t)
	res := importEdited(t, f, func(exp map[string]any) {
		exp["copyRules"] = []any{map[string]any{"domain": "containers", "identity": "stack:immich", "skip": []any{}}}
	})
	if res["code"] != "stack-rule" {
		t.Fatalf("import = %v, want stack-rule", res)
	}
}

func TestAnImportKeepsARepositoryADefaultPointsAt(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	f.namedRepo("Other", "other")
	f.setDefault("containers", nas.ID)
	res := importEdited(t, f, func(exp map[string]any) {
		var keep []any
		for _, row := range exp["namedRepos"].([]any) {
			if row.(map[string]any)["name"] != "NAS" {
				keep = append(keep, row)
			}
		}
		exp["namedRepos"] = keep
	})
	if res["ok"] != true {
		t.Fatalf("import = %v", res)
	}
	if _, err := f.st.GetNamedRepo(nas.ID); err != nil {
		t.Fatalf("the repository the default points at is gone: %v", err)
	}
}

func TestAnImportKeepsAConfirmedDomainConfirmed(t *testing.T) {
	f := newPlacementFixture(t)
	nas := f.namedRepo("NAS", "nas")
	b2 := f.target("containers", "B2", "b2:bucket/containers")
	f.setDefault("containers", nas.ID, b2.ID)
	f.paused("containers")
	if res := f.do(http.MethodPost, "/api/placement/default/containers/confirm", map[string]any{}); res["ok"] != true {
		t.Fatalf("confirm = %v", res)
	}
	if d, found, err := f.st.PlacementDefaultFor("containers"); err != nil || !found || !d.ConfirmedManually || d.Paused() {
		t.Fatalf("containers after confirm = %+v, %v, %v, want the manual marker set and the pause ended", d, found, err)
	}

	exp := f.do(http.MethodGet, "/api/settings/export", nil)
	if res := f.do(http.MethodPost, "/api/settings/import?apply=true", exp); res["ok"] != true {
		t.Fatalf("import = %v", res)
	}

	d, found, err := f.st.PlacementDefaultFor("containers")
	if err != nil || !found || !d.ConfirmedManually || d.Paused() {
		t.Fatalf("containers after import = %+v, %v, %v, want the manual marker kept and the domain still unpaused", d, found, err)
	}
}

func TestTheImportPreviewCountsBothBlocks(t *testing.T) {
	f := newPlacementFixture(t)
	f.setDefault("containers", "")
	f.rule("vms", "vm:win11", store.SkipAll)
	exp := f.do(http.MethodGet, "/api/settings/export", nil)
	summary := f.do(http.MethodPost, "/api/settings/import", exp)["summary"].(map[string]any)
	if summary["placementDefaults"] != float64(1) || summary["copyRules"] != float64(1) {
		t.Fatalf("summary = %v, want one default and one rule", summary)
	}
	delete(exp, "copyRules")
	summary = f.do(http.MethodPost, "/api/settings/import", exp)["summary"].(map[string]any)
	if summary["copyRules"] != nil {
		t.Fatalf("copyRules = %v, want null for a missing block", summary["copyRules"])
	}
}
