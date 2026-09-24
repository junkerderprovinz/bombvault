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

	if err := r.UnlinkAlias("nginx", "", nil); err != nil {
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

	if err := r.UnlinkAlias("web", "", nil); err != nil {
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
	if err := r.UnlinkAlias("nginx", "", nil); err != nil {
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
	if err := r.UnlinkAlias("nginx", "", nil); err != nil {
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

	if err := r.UnlinkAlias("nginx", "", nil); !errors.Is(err, store.ErrCopyRuleTaken) {
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

	if err := r.DeleteTarget("app", nil); err != nil {
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

	if err := r.DeleteTarget("web", nil); err != nil {
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

	if err := r.DeleteTarget("web", nil); err != nil {
		t.Fatalf("DeleteTarget: %v", err)
	}
	mustNoRule(t, r, "containers", "container:nginx")
}

func TestAnUnlinkLeavesNoRuleOnAnInstalledNameItLeaves(t *testing.T) {
	r := newRepo(t)
	if _, err := r.UpsertTarget(store.Target{ContainerName: "nginx"}); err != nil {
		t.Fatal(err)
	}
	store.SeedCopyRule(t, r, "containers", "container:nginx", store.SkipAll)
	if err := r.RenameTargetWithAlias("nginx", "web", ""); err != nil {
		t.Fatal(err)
	}

	if err := r.UnlinkAlias("nginx", "", map[string]bool{"web": true}); err != nil {
		t.Fatalf("UnlinkAlias: %v", err)
	}
	mustRule(t, r, "containers", "container:nginx", store.SkipAll)
	mustNoRule(t, r, "containers", "container:web")
}

func TestADeletedEntryWritesNoRuleOnAnInstalledFormerName(t *testing.T) {
	r := newRepo(t)
	if _, err := r.UpsertTarget(store.Target{ContainerName: "nginx"}); err != nil {
		t.Fatal(err)
	}
	store.SeedCopyRule(t, r, "containers", "container:nginx")
	if err := r.RenameTargetWithAlias("nginx", "web", ""); err != nil {
		t.Fatal(err)
	}
	if err := r.RenameTargetWithAlias("web", "app", ""); err != nil {
		t.Fatal(err)
	}

	if err := r.DeleteTarget("app", map[string]bool{"nginx": true}); err != nil {
		t.Fatalf("DeleteTarget: %v", err)
	}
	mustNoRule(t, r, "containers", "container:nginx")
	mustRule(t, r, "containers", "container:web")
}

func TestADeletedEntryWithoutARuleLeavesItsFormerNamesFollowing(t *testing.T) {
	r := newRepo(t)
	if _, err := r.UpsertTarget(store.Target{ContainerName: "nginx"}); err != nil {
		t.Fatal(err)
	}
	if err := r.RenameTargetWithAlias("nginx", "web", ""); err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteTarget("web", nil); err != nil {
		t.Fatalf("DeleteTarget: %v", err)
	}
	mustNoRule(t, r, "containers", "container:nginx")
	mustNoRule(t, r, "containers", "container:web")
}

// linkedTo is an entry called name with each of olds linked to it as a former
// name, the first one oldest.
func linkedTo(t *testing.T, r *store.Repo, name string, olds ...string) {
	t.Helper()
	tg, err := r.UpsertTarget(store.Target{ContainerName: name})
	if err != nil {
		t.Fatal(err)
	}
	for i, old := range olds {
		if _, err := r.AddAliasAt("container", old, tg.ID, int64(100*(i+1))); err != nil {
			t.Fatal(err)
		}
	}
}

func importRules(t *testing.T, r *store.Repo, rules ...store.CopyRule) {
	t.Helper()
	if err := r.ImportPlacement(store.PlacementImport{HasRules: true, Rules: rules}, store.Installed{}); err != nil {
		t.Fatalf("ImportPlacement: %v", err)
	}
}

func TestAnImportedRuleOnAFormerNameMovesToTheEntry(t *testing.T) {
	r := newRepo(t)
	linkedTo(t, r, "web", "nginx")

	importRules(t, r, store.CopyRule{Domain: "containers", Identity: "container:nginx", Skip: []string{store.SkipAll}})
	mustRule(t, r, "containers", "container:web", store.SkipAll)
	mustNoRule(t, r, "containers", "container:nginx")
}

func TestAnImportedRuleOnAFormerNameStaysWithItsOwner(t *testing.T) {
	t.Run("the entry has a rule of its own", func(t *testing.T) {
		r := newRepo(t)
		linkedTo(t, r, "web", "nginx")
		importRules(t, r,
			store.CopyRule{Domain: "containers", Identity: "container:nginx", Skip: []string{store.SkipAll}},
			store.CopyRule{Domain: "containers", Identity: "container:web", Skip: []string{"t-b2"}})
		mustRule(t, r, "containers", "container:web", "t-b2")
		mustRule(t, r, "containers", "container:nginx", store.SkipAll)
	})
	t.Run("a row carries the former name", func(t *testing.T) {
		r := newRepo(t)
		linkedTo(t, r, "web", "nginx")
		if _, err := r.UpsertTarget(store.Target{ContainerName: "nginx"}); err != nil {
			t.Fatal(err)
		}
		importRules(t, r, store.CopyRule{Domain: "containers", Identity: "container:nginx", Skip: []string{store.SkipAll}})
		mustRule(t, r, "containers", "container:nginx", store.SkipAll)
		mustNoRule(t, r, "containers", "container:web")
	})
	t.Run("a container is installed under the former name", func(t *testing.T) {
		r := newRepo(t)
		linkedTo(t, r, "web", "nginx")
		rules := []store.CopyRule{{Domain: "containers", Identity: "container:nginx", Skip: []string{store.SkipAll}}}
		installed := store.Installed{Containers: map[string]bool{"nginx": true}}
		if err := r.ImportPlacement(store.PlacementImport{HasRules: true, Rules: rules}, installed); err != nil {
			t.Fatalf("ImportPlacement: %v", err)
		}
		mustRule(t, r, "containers", "container:nginx", store.SkipAll)
		mustNoRule(t, r, "containers", "container:web")
	})
	t.Run("a VM is defined under the former name", func(t *testing.T) {
		r := newRepo(t)
		vm, err := r.UpsertVMTarget(store.VMTarget{Name: "win11"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := r.AddVMAliasAt("windows-11", vm.ID, 200, ""); err != nil {
			t.Fatal(err)
		}
		rules := []store.CopyRule{{Domain: "vms", Identity: "vm:windows-11", Skip: []string{store.SkipAll}}}
		installed := store.Installed{VMs: map[string]bool{"windows-11": true}}
		if err := r.ImportPlacement(store.PlacementImport{HasRules: true, Rules: rules}, installed); err != nil {
			t.Fatalf("ImportPlacement: %v", err)
		}
		mustRule(t, r, "vms", "vm:windows-11", store.SkipAll)
		mustNoRule(t, r, "vms", "vm:win11")
	})
}

func TestTheRuleOfTheLatestFormerNameMovesToTheEntry(t *testing.T) {
	r := newRepo(t)
	linkedTo(t, r, "app", "nginx", "web")

	importRules(t, r,
		store.CopyRule{Domain: "containers", Identity: "container:nginx", Skip: []string{store.SkipAll}},
		store.CopyRule{Domain: "containers", Identity: "container:web", Skip: []string{"t-b2"}})
	mustRule(t, r, "containers", "container:app", "t-b2")
	mustRule(t, r, "containers", "container:nginx", store.SkipAll)
}

func TestAnImportedRuleOnAVMsFormerNameMovesToTheVM(t *testing.T) {
	r := newRepo(t)
	vm, err := r.UpsertVMTarget(store.VMTarget{Name: "win11"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.AddVMAliasAt("windows-11", vm.ID, 200, ""); err != nil {
		t.Fatal(err)
	}

	importRules(t, r, store.CopyRule{Domain: "vms", Identity: "vm:windows-11", Skip: []string{store.SkipAll}})
	mustRule(t, r, "vms", "vm:win11", store.SkipAll)
	mustNoRule(t, r, "vms", "vm:windows-11")
}

func TestARuleOnANewlyLinkedFormerNameMovesToTheEntry(t *testing.T) {
	r := newRepo(t)
	store.SeedCopyRule(t, r, "containers", "container:nginx", store.SkipAll)
	store.SeedCopyRule(t, r, "containers", "container:db", store.SkipAll)
	linkedTo(t, r, "web", "nginx")
	linkedTo(t, r, "postgres", "db")

	if err := r.CarryFormerNameRule("container", "nginx", nil); err != nil {
		t.Fatalf("CarryFormerNameRule: %v", err)
	}
	mustRule(t, r, "containers", "container:web", store.SkipAll)
	mustNoRule(t, r, "containers", "container:nginx")
	mustRule(t, r, "containers", "container:db", store.SkipAll)
}

func TestARuleOnANewlyLinkedFormerNameStaysWithTheContainerInstalledUnderIt(t *testing.T) {
	r := newRepo(t)
	store.SeedCopyRule(t, r, "containers", "container:nginx", store.SkipAll)
	linkedTo(t, r, "web", "nginx")

	if err := r.CarryFormerNameRule("container", "nginx", map[string]bool{"nginx": true}); err != nil {
		t.Fatalf("CarryFormerNameRule: %v", err)
	}
	mustRule(t, r, "containers", "container:nginx", store.SkipAll)
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

	if err := r.UnlinkVMAlias("windows-11", "", "", nil); err != nil {
		t.Fatalf("UnlinkVMAlias: %v", err)
	}
	mustRule(t, r, "vms", "vm:windows-11", store.SkipAll)
	mustRule(t, r, "vms", "vm:win11", store.SkipAll)
}

func TestAVMUnlinkLeavesNoRuleOnAnInstalledNameItLeaves(t *testing.T) {
	r := newRepo(t)
	if _, err := r.UpsertVMTarget(store.VMTarget{Name: "windows-11"}); err != nil {
		t.Fatal(err)
	}
	store.SeedCopyRule(t, r, "vms", "vm:windows-11", store.SkipAll)
	if err := r.RenameVMTargetWithAlias("windows-11", "win11", "", ""); err != nil {
		t.Fatal(err)
	}

	if err := r.UnlinkVMAlias("windows-11", "", "", map[string]bool{"win11": true}); err != nil {
		t.Fatalf("UnlinkVMAlias: %v", err)
	}
	mustRule(t, r, "vms", "vm:windows-11", store.SkipAll)
	mustNoRule(t, r, "vms", "vm:win11")
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

	if err := r.DeleteVMTarget("win11", nil); err != nil {
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

	if err := r.DeleteVMTarget("win11", nil); err != nil {
		t.Fatalf("DeleteVMTarget: %v", err)
	}
	mustNoRule(t, r, "vms", "vm:windows-11")
}

func TestADeletedVMWritesNoRuleOnAnInstalledFormerName(t *testing.T) {
	r := newRepo(t)
	if _, err := r.UpsertVMTarget(store.VMTarget{Name: "windows-11"}); err != nil {
		t.Fatal(err)
	}
	store.SeedCopyRule(t, r, "vms", "vm:windows-11", store.SkipAll)
	if err := r.RenameVMTargetWithAlias("windows-11", "win11", "", ""); err != nil {
		t.Fatal(err)
	}

	if err := r.DeleteVMTarget("win11", map[string]bool{"windows-11": true}); err != nil {
		t.Fatalf("DeleteVMTarget: %v", err)
	}
	mustNoRule(t, r, "vms", "vm:windows-11")
}
