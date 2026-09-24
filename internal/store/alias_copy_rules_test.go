package store_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func mustRule(t *testing.T, r *store.Repo, domain, identity string, want ...string) {
	t.Helper()
	skip, ok := store.RuleSkip(t, r, domain, identity)
	if !ok || !slices.Equal(skip, want) {
		t.Fatalf("rule of %s = %v (found %v), want %v", identity, skip, ok, want)
	}
}

func mustNoRule(t *testing.T, r *store.Repo, domain, identity string) {
	t.Helper()
	if skip, ok := store.RuleSkip(t, r, domain, identity); ok {
		t.Fatalf("%s has the rule %v, want none", identity, skip)
	}
}

func TestARenamedContainerTakesItsCopyRuleAndGivesItBackOnUnlink(t *testing.T) {
	r := newRepo(t)
	if _, err := r.UpsertTarget(store.Target{ContainerName: "nginx"}); err != nil {
		t.Fatal(err)
	}
	store.SeedCopyRule(t, r, "containers", "container:nginx", "t-b2")

	if err := r.RenameTargetWithAlias("nginx", "web", ""); err != nil {
		t.Fatalf("RenameTargetWithAlias: %v", err)
	}
	mustRule(t, r, "containers", "container:web", "t-b2")
	mustNoRule(t, r, "containers", "container:nginx")

	if err := r.UnlinkAlias("nginx", ""); err != nil {
		t.Fatalf("UnlinkAlias: %v", err)
	}
	mustRule(t, r, "containers", "container:nginx", "t-b2")
	mustRule(t, r, "containers", "container:web", "t-b2")
}

func TestAnUnlinkInAChainLeavesTheRuleOnTheNameTheEntryLeaves(t *testing.T) {
	r := newRepo(t)
	if _, err := r.UpsertTarget(store.Target{ContainerName: "nginx"}); err != nil {
		t.Fatal(err)
	}
	store.SeedCopyRule(t, r, "containers", "container:nginx", store.SkipAll)
	if err := r.RenameTargetWithAlias("nginx", "web", ""); err != nil {
		t.Fatal(err)
	}
	if err := r.RenameTargetWithAlias("web", "app", ""); err != nil {
		t.Fatal(err)
	}

	if err := r.UnlinkAlias("web", ""); err != nil {
		t.Fatalf("UnlinkAlias: %v", err)
	}
	mustRule(t, r, "containers", "container:web", store.SkipAll)
	mustRule(t, r, "containers", "container:app", store.SkipAll)
	mustNoRule(t, r, "containers", "container:nginx")
}

func TestAContainerTakenOverAgainAfterAnUnlinkKeepsItsRule(t *testing.T) {
	r := newRepo(t)
	if _, err := r.UpsertTarget(store.Target{ContainerName: "nginx"}); err != nil {
		t.Fatal(err)
	}
	store.SeedCopyRule(t, r, "containers", "container:nginx", "t-b2")
	if err := r.RenameTargetWithAlias("nginx", "web", ""); err != nil {
		t.Fatal(err)
	}
	if err := r.UnlinkAlias("nginx", ""); err != nil {
		t.Fatal(err)
	}

	if err := r.RenameTargetWithAlias("nginx", "web", ""); err != nil {
		t.Fatalf("RenameTargetWithAlias: %v", err)
	}
	mustRule(t, r, "containers", "container:web", "t-b2")
	mustNoRule(t, r, "containers", "container:nginx")
}

func TestATakeoverOntoANameWithADifferentRuleLeftByAnUnlinkIsRefused(t *testing.T) {
	r := newRepo(t)
	if _, err := r.UpsertTarget(store.Target{ContainerName: "nginx"}); err != nil {
		t.Fatal(err)
	}
	store.SeedCopyRule(t, r, "containers", "container:nginx", "t-b2")
	if err := r.RenameTargetWithAlias("nginx", "web", ""); err != nil {
		t.Fatal(err)
	}
	if err := r.UnlinkAlias("nginx", ""); err != nil {
		t.Fatal(err)
	}
	store.SeedCopyRule(t, r, "containers", "container:nginx", store.SkipAll)

	if err := r.RenameTargetWithAlias("nginx", "web", ""); !errors.Is(err, store.ErrCopyRuleTaken) {
		t.Fatalf("err = %v, want ErrCopyRuleTaken", err)
	}
	mustRule(t, r, "containers", "container:web", "t-b2")
}

func TestARenameOntoANameWithACopyRuleIsRefused(t *testing.T) {
	r := newRepo(t)
	if _, err := r.UpsertTarget(store.Target{ContainerName: "nginx"}); err != nil {
		t.Fatal(err)
	}
	store.SeedCopyRule(t, r, "containers", "container:web", store.SkipAll)

	if err := r.RenameTargetWithAlias("nginx", "web", ""); !errors.Is(err, store.ErrCopyRuleTaken) {
		t.Fatalf("err = %v, want ErrCopyRuleTaken", err)
	}
	if _, err := r.GetTargetByContainer("nginx"); err != nil {
		t.Fatalf("the entry left its name: %v", err)
	}
	if aliases, err := r.ListAliases("container"); err != nil || len(aliases) != 0 {
		t.Fatalf("aliases = %v, %v, want none", aliases, err)
	}
}

func TestAnUnlinkOntoANameWithACopyRuleIsRefused(t *testing.T) {
	r := newRepo(t)
	if _, err := r.UpsertTarget(store.Target{ContainerName: "nginx"}); err != nil {
		t.Fatal(err)
	}
	if err := r.RenameTargetWithAlias("nginx", "web", ""); err != nil {
		t.Fatal(err)
	}
	store.SeedCopyRule(t, r, "containers", "container:nginx", store.SkipAll)

	if err := r.UnlinkAlias("nginx", ""); !errors.Is(err, store.ErrCopyRuleTaken) {
		t.Fatalf("err = %v, want ErrCopyRuleTaken", err)
	}
	if _, err := r.GetTargetByContainer("web"); err != nil {
		t.Fatalf("the entry left its name: %v", err)
	}
}

// A rename back onto the entry's own former name drops that alias, so the rule
// sitting on the name would be overwritten without this refusal.
func TestARenameBackOntoAFormerNameWithACopyRuleIsRefused(t *testing.T) {
	r := newRepo(t)
	if _, err := r.UpsertTarget(store.Target{ContainerName: "nginx"}); err != nil {
		t.Fatal(err)
	}
	if err := r.RenameTargetWithAlias("nginx", "web", ""); err != nil {
		t.Fatal(err)
	}
	store.SeedCopyRule(t, r, "containers", "container:nginx", store.SkipAll)

	if err := r.RenameTargetWithAlias("web", "nginx", ""); !errors.Is(err, store.ErrCopyRuleTaken) {
		t.Fatalf("err = %v, want ErrCopyRuleTaken", err)
	}
	if _, err := r.AliasByOldName("container", "nginx"); err != nil {
		t.Fatalf("the refused rename dropped the alias: %v", err)
	}
}

func TestADeletedEntryLeavesItsCopyRuleOnEveryFormerName(t *testing.T) {
	r := newRepo(t)
	if _, err := r.UpsertTarget(store.Target{ContainerName: "nginx"}); err != nil {
		t.Fatal(err)
	}
	store.SeedCopyRule(t, r, "containers", "container:nginx", store.SkipAll)
	if err := r.RenameTargetWithAlias("nginx", "web", ""); err != nil {
		t.Fatal(err)
	}
	if err := r.RenameTargetWithAlias("web", "app", ""); err != nil {
		t.Fatal(err)
	}

	if err := r.DeleteTarget("app"); err != nil {
		t.Fatalf("DeleteTarget: %v", err)
	}
	for _, id := range []string{"container:nginx", "container:web", "container:app"} {
		mustRule(t, r, "containers", id, store.SkipAll)
	}
}

func TestADeletedEntryLeavesTheRuleOfTheNamesHolderToday(t *testing.T) {
	r := newRepo(t)
	if _, err := r.UpsertTarget(store.Target{ContainerName: "nginx"}); err != nil {
		t.Fatal(err)
	}
	store.SeedCopyRule(t, r, "containers", "container:nginx", "t-b2")
	if err := r.RenameTargetWithAlias("nginx", "web", ""); err != nil {
		t.Fatal(err)
	}
	store.SeedCopyRule(t, r, "containers", "container:nginx", store.SkipAll)

	if err := r.DeleteTarget("web"); err != nil {
		t.Fatalf("DeleteTarget: %v", err)
	}
	mustRule(t, r, "containers", "container:nginx", store.SkipAll)
}

func TestADeletedEntryWritesNoRuleOnANameAnotherEntryCarries(t *testing.T) {
	r := newRepo(t)
	if _, err := r.UpsertTarget(store.Target{ContainerName: "nginx"}); err != nil {
		t.Fatal(err)
	}
	store.SeedCopyRule(t, r, "containers", "container:nginx")
	if err := r.RenameTargetWithAlias("nginx", "web", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := r.UpsertTarget(store.Target{ContainerName: "nginx"}); err != nil {
		t.Fatal(err)
	}

	if err := r.DeleteTarget("web"); err != nil {
		t.Fatalf("DeleteTarget: %v", err)
	}
	mustNoRule(t, r, "containers", "container:nginx")
}

func TestADeletedEntryWithoutARuleLeavesItsFormerNamesFollowing(t *testing.T) {
	r := newRepo(t)
	if _, err := r.UpsertTarget(store.Target{ContainerName: "nginx"}); err != nil {
		t.Fatal(err)
	}
	if err := r.RenameTargetWithAlias("nginx", "web", ""); err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteTarget("web"); err != nil {
		t.Fatalf("DeleteTarget: %v", err)
	}
	mustNoRule(t, r, "containers", "container:nginx")
	mustNoRule(t, r, "containers", "container:web")
}

func TestARenamedVMTakesItsCopyRuleAndGivesItBackOnUnlink(t *testing.T) {
	r := newRepo(t)
	if _, err := r.UpsertVMTarget(store.VMTarget{Name: "windows-11"}); err != nil {
		t.Fatal(err)
	}
	store.SeedCopyRule(t, r, "vms", "vm:windows-11", store.SkipAll)

	if err := r.RenameVMTargetWithAlias("windows-11", "win11", "", ""); err != nil {
		t.Fatalf("RenameVMTargetWithAlias: %v", err)
	}
	mustRule(t, r, "vms", "vm:win11", store.SkipAll)
	mustNoRule(t, r, "vms", "vm:windows-11")

	if err := r.UnlinkVMAlias("windows-11", "", ""); err != nil {
		t.Fatalf("UnlinkVMAlias: %v", err)
	}
	mustRule(t, r, "vms", "vm:windows-11", store.SkipAll)
	mustRule(t, r, "vms", "vm:win11", store.SkipAll)
}

func TestAVMRenameOntoANameWithACopyRuleIsRefused(t *testing.T) {
	r := newRepo(t)
	if _, err := r.UpsertVMTarget(store.VMTarget{Name: "windows-11"}); err != nil {
		t.Fatal(err)
	}
	store.SeedCopyRule(t, r, "vms", "vm:win11", store.SkipAll)

	if err := r.RenameVMTargetWithAlias("windows-11", "win11", "", ""); !errors.Is(err, store.ErrCopyRuleTaken) {
		t.Fatalf("err = %v, want ErrCopyRuleTaken", err)
	}
	if _, err := r.GetVMTargetByName("windows-11"); err != nil {
		t.Fatalf("the entry left its name: %v", err)
	}
}

func TestADeletedVMLeavesItsCopyRuleOnItsFormerName(t *testing.T) {
	r := newRepo(t)
	if _, err := r.UpsertVMTarget(store.VMTarget{Name: "windows-11"}); err != nil {
		t.Fatal(err)
	}
	store.SeedCopyRule(t, r, "vms", "vm:windows-11", store.SkipAll)
	if err := r.RenameVMTargetWithAlias("windows-11", "win11", "", ""); err != nil {
		t.Fatal(err)
	}

	if err := r.DeleteVMTarget("win11"); err != nil {
		t.Fatalf("DeleteVMTarget: %v", err)
	}
	mustRule(t, r, "vms", "vm:windows-11", store.SkipAll)
}

func TestADeletedVMWritesNoRuleOnANameAnotherVMCarries(t *testing.T) {
	r := newRepo(t)
	if _, err := r.UpsertVMTarget(store.VMTarget{Name: "windows-11"}); err != nil {
		t.Fatal(err)
	}
	store.SeedCopyRule(t, r, "vms", "vm:windows-11", store.SkipAll)
	if err := r.RenameVMTargetWithAlias("windows-11", "win11", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := r.UpsertVMTarget(store.VMTarget{Name: "windows-11"}); err != nil {
		t.Fatal(err)
	}

	if err := r.DeleteVMTarget("win11"); err != nil {
		t.Fatalf("DeleteVMTarget: %v", err)
	}
	mustNoRule(t, r, "vms", "vm:windows-11")
}
