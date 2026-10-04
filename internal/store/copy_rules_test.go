package store_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestACopyRuleIsStoredUnderItsSnapshotName(t *testing.T) {
	r := newRepo(t)
	if _, found, err := r.CopyRuleFor("containers", "container:nginx"); found || err != nil {
		t.Fatalf("before: found=%v err=%v, want nothing", found, err)
	}
	store.SeedCopyRule(t, r, "containers", "container:nginx", "t2", "t1", "t2")
	got, found, err := r.CopyRuleFor("containers", "container:nginx")
	if err != nil || !found || !slices.Equal(got.Skip, []string{"t1", "t2"}) || got.UpdatedAt == 0 {
		t.Fatalf("CopyRuleFor = %+v found=%v err=%v, want skip [t1 t2] with a time", got, found, err)
	}
	if err := r.DeleteCopyRule("containers", "container:nginx"); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.RuleSkip(t, r, "containers", "container:nginx"); ok {
		t.Fatal("the rule is still there after DeleteCopyRule")
	}
}

func TestAnEmptySkipMeansEveryTarget(t *testing.T) {
	r := newRepo(t)
	store.SeedCopyRule(t, r, "files", "fileset:Photos 2024")
	skip, ok := store.RuleSkip(t, r, "files", "fileset:Photos 2024")
	if !ok || skip == nil || len(skip) != 0 {
		t.Fatalf("skip = %#v ok=%v, want an empty list", skip, ok)
	}
}

func TestACopyRuleRefusesWhatItCannotMean(t *testing.T) {
	r := newRepo(t)
	for _, c := range []struct {
		domain, identity string
		skip             []string
		want             error
	}{
		{"containers", "container:nginx", []string{""}, store.ErrBadSkip},
		{"containers", "container:nginx", []string{store.SkipAll, "t1"}, store.ErrBadSkip},
		{"containers", "stack:immich", nil, store.ErrStackCopyRule},
		{"containers", "vm:win11", nil, store.ErrRuleDomain},
		{"vms", "vm:", nil, store.ErrRuleDomain},
		{"flash", "flash", nil, store.ErrRuleDomain},
	} {
		if err := r.SetCopyRule(c.domain, c.identity, c.skip); !errors.Is(err, c.want) {
			t.Errorf("SetCopyRule(%s, %s, %v) = %v, want %v", c.domain, c.identity, c.skip, err, c.want)
		}
	}
	if rules, err := r.ListCopyRules(); err != nil || len(rules) != 0 {
		t.Fatalf("ListCopyRules = %v, %v, want nothing stored", rules, err)
	}
}

func TestMovingARuleCarriesItToTheNewName(t *testing.T) {
	r := newRepo(t)
	store.SeedCopyRule(t, r, "files", "fileset:Photos", store.SkipAll)
	if err := r.MoveCopyRule("files", "fileset:Photos", "fileset:Pictures"); err != nil {
		t.Fatalf("MoveCopyRule: %v", err)
	}
	if _, ok := store.RuleSkip(t, r, "files", "fileset:Photos"); ok {
		t.Error("the old name kept its rule")
	}
	if skip, ok := store.RuleSkip(t, r, "files", "fileset:Pictures"); !ok || !slices.Equal(skip, []string{store.SkipAll}) {
		t.Errorf("new name: skip %v ok=%v, want [*]", skip, ok)
	}
	if err := r.MoveCopyRule("files", "fileset:Unknown", "fileset:Other"); err != nil {
		t.Errorf("moving a name without a rule = %v, want nothing to happen", err)
	}
}

func TestMovingOntoANameWithARuleIsRefused(t *testing.T) {
	r := newRepo(t)
	store.SeedCopyRule(t, r, "files", "fileset:Photos", store.SkipAll)
	store.SeedCopyRule(t, r, "files", "fileset:Pictures")
	if err := r.MoveCopyRule("files", "fileset:Photos", "fileset:Pictures"); !errors.Is(err, store.ErrCopyRuleTaken) {
		t.Fatalf("MoveCopyRule = %v, want ErrCopyRuleTaken", err)
	}
	if skip, _ := store.RuleSkip(t, r, "files", "fileset:Photos"); !slices.Equal(skip, []string{store.SkipAll}) {
		t.Errorf("the refused move changed the old rule to %v", skip)
	}
}

func TestCopyRulesForDomainAreKeyedByName(t *testing.T) {
	r := newRepo(t)
	rules, err := r.CopyRulesForDomain("vms")
	if err != nil || rules == nil || len(rules) != 0 {
		t.Fatalf("empty domain: %v, %v, want an empty map", rules, err)
	}
	store.SeedCopyRule(t, r, "vms", "vm:win11", store.SkipAll)
	store.SeedCopyRule(t, r, "containers", "container:nginx")
	rules, err = r.CopyRulesForDomain("vms")
	if err != nil || len(rules) != 1 || rules["vm:win11"].Domain != "vms" {
		t.Fatalf("CopyRulesForDomain(vms) = %v, %v", rules, err)
	}
}
