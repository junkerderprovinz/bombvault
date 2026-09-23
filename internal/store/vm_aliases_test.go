package store_test

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestRenameVMTargetWithAlias(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	old, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetVMInclude("windows-11", true); err != nil {
		t.Fatal(err)
	}
	if err := st.RenameVMTargetWithAlias("windows-11", "win11", "", ""); err != nil {
		t.Fatalf("RenameVMTargetWithAlias: %v", err)
	}
	got, err := st.GetVMTargetByName("win11")
	if err != nil {
		t.Fatalf("the renamed entry must be found under the new name: %v", err)
	}
	if got.ID != old.ID {
		t.Fatalf("the row id must stay so the run history follows: %s != %s", got.ID, old.ID)
	}
	if !got.IncludeInSchedule {
		t.Fatal("settings must survive the rename")
	}
	names, _ := st.AliasNames("vm", old.ID)
	if len(names) != 1 || names[0] != "windows-11" {
		t.Fatalf("alias not written: %v", names)
	}
	if _, err := st.GetVMTargetByName("windows-11"); err == nil {
		t.Fatal("the old name must no longer be an entry of its own")
	}
}

// TestRenameVMTargetWithAliasKeepsTheDefinitionItHadUnderTheOldName: an unlink
// or a rename back needs the old disk paths, which the refreshed definition
// does not carry.
func TestRenameVMTargetWithAliasKeepsTheDefinitionItHadUnderTheOldName(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11", Definition: "as-windows-11"}); err != nil {
		t.Fatal(err)
	}
	if err := st.RenameVMTargetWithAlias("windows-11", "win11", "as-win11", ""); err != nil {
		t.Fatalf("RenameVMTargetWithAlias: %v", err)
	}
	a, err := st.AliasByOldName("vm", "windows-11")
	if err != nil || a.PrevDefinition != "as-windows-11" {
		t.Fatalf("alias = %+v, %v; want the definition from before the rename", a, err)
	}
}

// TestAddVMAliasAtKeepsTheDefinitionForAnUnlink: a link rebuilt from the
// repository keeps the definition the entry had under the old name.
func TestAddVMAliasAtKeepsTheDefinitionForAnUnlink(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	tg, err := st.UpsertVMTarget(store.VMTarget{Name: "win11"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddVMAliasAt("windows-11", tg.ID, 100, "as-windows-11"); err != nil {
		t.Fatalf("AddVMAliasAt: %v", err)
	}
	a, err := st.AliasByOldName("vm", "windows-11")
	if err != nil || a.TargetID != tg.ID || a.LinkedAt != 100 || a.PrevDefinition != "as-windows-11" {
		t.Fatalf("alias = %+v, %v; want windows-11 linked at 100 with its old definition", a, err)
	}
}

// The definition mirror records each alias with the definition it keeps, in
// link order, while the plain listing leaves the definitions out.
func TestTargetAliasesWithDefinitionsCarriesEachKeptDefinition(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	tg, err := st.UpsertVMTarget(store.VMTarget{Name: "win11"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddVMAliasAt("windows-11", tg.ID, 200, "as-windows-11"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddVMAliasAt("windows-10", tg.ID, 100, "as-windows-10"); err != nil {
		t.Fatal(err)
	}
	got, err := st.TargetAliasesWithDefinitions("vm", tg.ID)
	if err != nil || len(got) != 2 {
		t.Fatalf("TargetAliasesWithDefinitions = %+v, %v", got, err)
	}
	if got[0].OldName != "windows-10" || got[0].PrevDefinition != "as-windows-10" || got[1].OldName != "windows-11" || got[1].PrevDefinition != "as-windows-11" {
		t.Fatalf("aliases = %+v, want windows-10 then windows-11, each with its definition", got)
	}
	plain, err := st.TargetAliases("vm", tg.ID)
	if err != nil || len(plain) != 2 || plain[0].PrevDefinition != "" || plain[1].PrevDefinition != "" {
		t.Fatalf("TargetAliases = %+v, %v; want the definitions left out", plain, err)
	}
}

// TestRenameVMTargetWithAliasBackOntoItsOwnFormerName: renamed back, the entry
// owns its former name again, so that alias goes and the name it leaves
// becomes the alias, keeping the definition it had there.
func TestRenameVMTargetWithAliasBackOntoItsOwnFormerName(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	tg, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11"})
	if err != nil {
		t.Fatal(err)
	}
	c, err := st.UpsertTarget(store.Target{ContainerName: "windows-tools"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("container", "windows-11", c.ID, 100); err != nil {
		t.Fatal(err)
	}
	if err := st.RenameVMTargetWithAlias("windows-11", "win11", "as-win11", ""); err != nil {
		t.Fatalf("first rename: %v", err)
	}
	if err := st.RenameVMTargetWithAlias("win11", "windows-11", "as-windows-11", ""); err != nil {
		t.Fatalf("rename back: %v", err)
	}
	got, err := st.GetVMTargetByName("windows-11")
	if err != nil || got.ID != tg.ID || got.Definition != "as-windows-11" {
		t.Fatalf("entry on windows-11 = %+v, %v", got, err)
	}
	aliases, err := st.TargetAliases("vm", tg.ID)
	if err != nil || len(aliases) != 1 || aliases[0].OldName != "win11" {
		t.Fatalf("aliases = %+v, %v; want win11 only", aliases, err)
	}
	if a, err := st.AliasByOldName("vm", "win11"); err != nil || a.PrevDefinition != "as-win11" {
		t.Fatalf("win11 alias = %+v, %v; want the definition it had under win11", a, err)
	}
	if a, err := st.AliasByOldName("container", "windows-11"); err != nil || a.TargetID != c.ID {
		t.Fatalf("the container's alias of the same name must stay: %+v, %v", a, err)
	}
}

// TestRenameVMTargetWithAliasRefusesAnotherEntrysFormerName: a former name
// belongs to one entry, so no other entry is renamed onto it.
func TestRenameVMTargetWithAliasRefusesAnotherEntrysFormerName(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	owner, err := st.UpsertVMTarget(store.VMTarget{Name: "win11-gaming"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("vm", "win11", owner.ID, 100); err != nil {
		t.Fatal(err)
	}
	other, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11", Definition: "as-windows-11"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RenameVMTargetWithAlias("windows-11", "win11", "as-win11", ""); err == nil {
		t.Fatal("renaming onto another entry's former name must be refused")
	}
	if got, err := st.GetVMTargetByName("windows-11"); err != nil || got.ID != other.ID || got.Definition != "as-windows-11" {
		t.Fatalf("the refused entry must keep its name and definition: %+v, %v", got, err)
	}
	if a, err := st.AliasByOldName("vm", "win11"); err != nil || a.TargetID != owner.ID {
		t.Fatalf("the owner's alias must stay: %+v, %v", a, err)
	}
	if _, err := st.AliasByOldName("vm", "windows-11"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("no alias may be written on refusal: %v", err)
	}
}

// TestRenameVMTargetWithAliasIgnoresAContainersFormerNameOfTheSameName: the
// two domains keep separate former names, so a container once called win11
// does not stand in the way of a VM.
func TestRenameVMTargetWithAliasIgnoresAContainersFormerNameOfTheSameName(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	c, err := st.UpsertTarget(store.Target{ContainerName: "win11-tools"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("container", "win11", c.ID, 100); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11"}); err != nil {
		t.Fatal(err)
	}
	if err := st.RenameVMTargetWithAlias("windows-11", "win11", "", ""); err != nil {
		t.Fatalf("RenameVMTargetWithAlias: %v", err)
	}
	if a, err := st.AliasByOldName("container", "win11"); err != nil || a.TargetID != c.ID {
		t.Fatalf("the container's alias must stay: %+v, %v", a, err)
	}
}

func TestGetVMTargetByID(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "ubuntu"}); err != nil {
		t.Fatal(err)
	}
	tg, err := st.UpsertVMTarget(store.VMTarget{Name: "win11"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := st.GetVMTargetByID(tg.ID)
	if err != nil || got.Name != "win11" || got.ID != tg.ID {
		t.Fatalf("GetVMTargetByID = %+v, %v", got, err)
	}
	if _, err := st.GetVMTargetByID("does-not-exist"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("unknown id: want sql.ErrNoRows, got %v", err)
	}
}

func TestRenameVMTargetWithAliasWritesDefinitionAtomically(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11", Definition: "stale"}); err != nil {
		t.Fatal(err)
	}
	if err := st.RenameVMTargetWithAlias("windows-11", "win11", "rewritten", ""); err != nil {
		t.Fatalf("RenameVMTargetWithAlias: %v", err)
	}
	got, err := st.GetVMTargetByName("win11")
	if err != nil {
		t.Fatal(err)
	}
	if got.Definition != "rewritten" {
		t.Fatalf("Definition = %q, want %q", got.Definition, "rewritten")
	}
}

func TestRenameVMTargetWithAliasRefusesOccupiedName(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	oldEntry, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetVMInclude("windows-11", true); err != nil {
		t.Fatal(err)
	}
	newEntry, err := st.UpsertVMTarget(store.VMTarget{Name: "win11"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetVMInclude("win11", true); err != nil {
		t.Fatal(err)
	}

	if err := st.RenameVMTargetWithAlias("windows-11", "win11", "", ""); err == nil {
		t.Fatal("a name that still has its own entry must be refused")
	}

	oldAfter, err := st.GetVMTargetByName("windows-11")
	if err != nil {
		t.Fatalf("a refused rename must leave the old entry alone: %v", err)
	}
	if oldAfter.ID != oldEntry.ID {
		t.Fatalf("old entry ID must not change: %s != %s", oldAfter.ID, oldEntry.ID)
	}
	if !oldAfter.IncludeInSchedule {
		t.Fatal("old entry settings must survive the refusal")
	}

	newAfter, err := st.GetVMTargetByName("win11")
	if err != nil {
		t.Fatalf("the pre-existing entry of the new name must survive: %v", err)
	}
	if newAfter.ID != newEntry.ID {
		t.Fatalf("new entry ID must not change: %s != %s", newAfter.ID, newEntry.ID)
	}
	if !newAfter.IncludeInSchedule {
		t.Fatal("new entry settings must survive the refusal")
	}

	if _, err := st.AliasByOldName("vm", "windows-11"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("no alias must leak on refusal: want sql.ErrNoRows, got %v", err)
	}
	oldAliases, _ := st.AliasNames("vm", oldEntry.ID)
	if len(oldAliases) != 0 {
		t.Fatalf("old entry must have no aliases after refusal: %v", oldAliases)
	}
}

func TestUnlinkVMAlias(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	old, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetVMInclude("windows-11", true); err != nil {
		t.Fatal(err)
	}
	if err := st.RenameVMTargetWithAlias("windows-11", "win11", "", ""); err != nil {
		t.Fatalf("RenameVMTargetWithAlias: %v", err)
	}

	if err := st.UnlinkVMAlias("windows-11", "", ""); err != nil {
		t.Fatalf("UnlinkVMAlias: %v", err)
	}
	got, err := st.GetVMTargetByName("windows-11")
	if err != nil {
		t.Fatalf("the entry must be found under its old name again: %v", err)
	}
	if got.ID != old.ID {
		t.Fatalf("the row id must stay so the run history follows: %s != %s", got.ID, old.ID)
	}
	if !got.IncludeInSchedule {
		t.Fatal("settings must survive the unlink")
	}
	if _, err := st.GetVMTargetByName("win11"); err == nil {
		t.Fatal("the new name must no longer be an entry of its own")
	}
	if _, err := st.AliasByOldName("vm", "windows-11"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("the alias must be gone: %v", err)
	}
}

func TestUnlinkVMAliasWritesDefinitionAtomically(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11"}); err != nil {
		t.Fatal(err)
	}
	if err := st.RenameVMTargetWithAlias("windows-11", "win11", "as-win11", ""); err != nil {
		t.Fatalf("RenameVMTargetWithAlias: %v", err)
	}
	if err := st.UnlinkVMAlias("windows-11", "as-windows-11", ""); err != nil {
		t.Fatalf("UnlinkVMAlias: %v", err)
	}
	got, err := st.GetVMTargetByName("windows-11")
	if err != nil {
		t.Fatal(err)
	}
	if got.Definition != "as-windows-11" {
		t.Fatalf("Definition = %q, want %q", got.Definition, "as-windows-11")
	}
}

func TestUnlinkVMAliasRefusesOccupiedName(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	takenOver, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RenameVMTargetWithAlias("windows-11", "win11", "", ""); err != nil {
		t.Fatalf("RenameVMTargetWithAlias: %v", err)
	}
	// A different VM is defined under the freed name.
	reused, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11"})
	if err != nil {
		t.Fatal(err)
	}

	if err := st.UnlinkVMAlias("windows-11", "", ""); err == nil {
		t.Fatal("a name that has since been reused by a different entry must be refused")
	}

	stillRenamed, err := st.GetVMTargetByName("win11")
	if err != nil || stillRenamed.ID != takenOver.ID {
		t.Fatalf("the taken-over entry must be untouched: %+v, %v", stillRenamed, err)
	}
	stillReused, err := st.GetVMTargetByName("windows-11")
	if err != nil || stillReused.ID != reused.ID {
		t.Fatalf("the reused entry must be untouched: %+v, %v", stillReused, err)
	}
	if _, err := st.AliasByOldName("vm", "windows-11"); err != nil {
		t.Fatalf("the alias must survive the refusal: %v", err)
	}
}

// The uuid column names the VM the stored definition belongs to, so a rename
// and an unlink write it with the definition.
func TestRenameAndUnlinkWriteTheVMsUUID(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11", UUID: "uuid-of-windows-11"}); err != nil {
		t.Fatal(err)
	}
	if err := st.RenameVMTargetWithAlias("windows-11", "win11", "as-win11", "uuid-of-win11"); err != nil {
		t.Fatalf("RenameVMTargetWithAlias: %v", err)
	}
	if got, err := st.GetVMTargetByName("win11"); err != nil || got.UUID != "uuid-of-win11" {
		t.Fatalf("after the rename: %+v, %v; want uuid-of-win11", got, err)
	}
	if err := st.UnlinkVMAlias("windows-11", "as-windows-11", "uuid-of-windows-11"); err != nil {
		t.Fatalf("UnlinkVMAlias: %v", err)
	}
	if got, err := st.GetVMTargetByName("windows-11"); err != nil || got.UUID != "uuid-of-windows-11" {
		t.Fatalf("after the unlink: %+v, %v; want uuid-of-windows-11", got, err)
	}
}

func TestUnlinkVMAliasRefusesAnAliasWhoseEntryIsGone(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	if _, err := st.AddVMAliasAt("windows-11", "no-such-entry", 100, "as-windows-11"); err != nil {
		t.Fatal(err)
	}
	if err := st.UnlinkVMAlias("windows-11", "as-windows-11", ""); err == nil {
		t.Fatal("an unlink that moves no row back must fail")
	}
	if _, err := st.AliasByOldName("vm", "windows-11"); err != nil {
		t.Fatalf("the alias must stay: %v", err)
	}
}

func TestUnlinkVMAliasRefusesUnknownName(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	if err := st.UnlinkVMAlias("never-linked", "", ""); err == nil {
		t.Fatal("an unknown old name must be refused")
	}
}

func TestDeleteVMTargetRemovesItsOwnAliases(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11"}); err != nil {
		t.Fatal(err)
	}
	if err := st.RenameVMTargetWithAlias("windows-11", "win11", "", ""); err != nil {
		t.Fatalf("RenameVMTargetWithAlias: %v", err)
	}

	if err := st.DeleteVMTarget("win11"); err != nil {
		t.Fatalf("DeleteVMTarget: %v", err)
	}
	if _, err := st.GetVMTargetByName("win11"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("the target must be gone: %v", err)
	}
	if _, err := st.AliasByOldName("vm", "windows-11"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("the dangling alias must be gone too, got %v", err)
	}
	// The freed name must be usable as an old name again.
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11"}); err != nil {
		t.Fatal(err)
	}
	if err := st.RenameVMTargetWithAlias("windows-11", "win11-2", "", ""); err != nil {
		t.Fatalf("windows-11 must be reusable as an old name once the dangling alias is gone: %v", err)
	}
}

func TestDeleteVMTargetLeavesUnrelatedAliasAlone(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	renamed, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RenameVMTargetWithAlias("windows-11", "win11", "", ""); err != nil {
		t.Fatalf("RenameVMTargetWithAlias: %v", err)
	}
	// An unrelated VM reuses the freed name and is then deleted.
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11"}); err != nil {
		t.Fatal(err)
	}

	if err := st.DeleteVMTarget("windows-11"); err != nil {
		t.Fatalf("DeleteVMTarget: %v", err)
	}
	alias, err := st.AliasByOldName("vm", "windows-11")
	if err != nil {
		t.Fatalf("the unrelated alias must survive: %v", err)
	}
	if alias.TargetID != renamed.ID {
		t.Fatalf("alias.TargetID = %q, want the renamed original's id %q", alias.TargetID, renamed.ID)
	}
	if _, err := st.GetVMTargetByName("win11"); err != nil {
		t.Fatalf("the renamed original must be untouched: %v", err)
	}
}

// TestUnlinkVMAliasScopedToVMDomainNotContainer: a container alias may share
// the old_name. Picked up by mistake, its target_id matches no row in vms, so
// the rename back would update nothing without an error while the alias is
// still deleted.
func TestUnlinkVMAliasScopedToVMDomainNotContainer(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	// A container alias under the exact same old_name as the VM alias below.
	containerTg, err := st.UpsertTarget(store.Target{ContainerName: "shared-name"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RenameTargetWithAlias("shared-name", "container-new-name", ""); err != nil {
		t.Fatalf("RenameTargetWithAlias: %v", err)
	}

	vmTg, err := st.UpsertVMTarget(store.VMTarget{Name: "shared-name"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RenameVMTargetWithAlias("shared-name", "vm-new-name", "", ""); err != nil {
		t.Fatalf("RenameVMTargetWithAlias: %v", err)
	}

	if err := st.UnlinkVMAlias("shared-name", "", ""); err != nil {
		t.Fatalf("UnlinkVMAlias: %v", err)
	}

	got, err := st.GetVMTargetByName("shared-name")
	if err != nil {
		t.Fatalf("the vm must be found under its old name again: %v", err)
	}
	if got.ID != vmTg.ID {
		t.Fatalf("the row id must stay so the run history follows: %s != %s", got.ID, vmTg.ID)
	}
	if _, err := st.GetVMTargetByName("vm-new-name"); err == nil {
		t.Fatal("the vm's new name must no longer be an entry of its own")
	}

	a, err := st.AliasByOldName("container", "shared-name")
	if err != nil {
		t.Fatalf("the container alias must survive untouched: %v", err)
	}
	if a.TargetID != containerTg.ID {
		t.Fatalf("container alias TargetID = %q, want %q", a.TargetID, containerTg.ID)
	}
}
