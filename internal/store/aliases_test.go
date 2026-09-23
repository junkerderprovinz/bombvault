package store_test

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestAliasRoundTrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	tg, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAlias("container", "radarr-movies", tg.ID); err != nil {
		t.Fatalf("AddAlias: %v", err)
	}
	names, err := st.AliasNames("container", tg.ID)
	if err != nil || len(names) != 1 || names[0] != "radarr-movies" {
		t.Fatalf("AliasNames = %v, %v", names, err)
	}
	a, err := st.AliasByOldName("container", "radarr-movies")
	if err != nil || a.TargetID != tg.ID {
		t.Fatalf("AliasByOldName = %+v, %v", a, err)
	}
	if _, err := st.AddAlias("container", "radarr-movies", tg.ID); err == nil {
		t.Fatal("a second alias for the same old name must be refused")
	}
}

// TestAddAliasAtKeepsLinkedAt: Discover re-creates an alias with the
// time the rename happened, not the time it runs.
func TestAddAliasAtKeepsLinkedAt(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	tg, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
	if err != nil {
		t.Fatal(err)
	}
	const at = int64(1717200000)
	if _, err := st.AddAliasAt("container", "radarr-movies", tg.ID, at); err != nil {
		t.Fatalf("AddAliasAt: %v", err)
	}
	a, err := st.AliasByOldName("container", "radarr-movies")
	if err != nil || a.LinkedAt != at {
		t.Fatalf("AliasByOldName = %+v, %v; want linked_at %d", a, err, at)
	}
}

// TestTargetAliases: one target's aliases in one domain, with their
// linked_at, oldest link first.
func TestTargetAliases(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	tg, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := st.UpsertTarget(store.Target{ContainerName: "sonarr"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("container", "radarr-4k", tg.ID, 300); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("container", "radarr-movies", tg.ID, 100); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("container", "sonarr-tv", other.ID, 200); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("vm", "radarr-vm", tg.ID, 50); err != nil {
		t.Fatal(err)
	}
	got, err := st.TargetAliases("container", tg.ID)
	if err != nil {
		t.Fatalf("TargetAliases: %v", err)
	}
	if len(got) != 2 || got[0].OldName != "radarr-movies" || got[0].LinkedAt != 100 ||
		got[1].OldName != "radarr-4k" || got[1].LinkedAt != 300 {
		t.Fatalf("TargetAliases = %+v, want radarr-movies@100 then radarr-4k@300", got)
	}
}

func TestRenameTargetWithAlias(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	old, err := st.UpsertTarget(store.Target{ContainerName: "radarr-movies"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetInclude("radarr-movies", true); err != nil {
		t.Fatal(err)
	}
	if err := st.RenameTargetWithAlias("radarr-movies", "radarr", ""); err != nil {
		t.Fatalf("RenameTargetWithAlias: %v", err)
	}
	got, err := st.GetTargetByContainer("radarr")
	if err != nil {
		t.Fatalf("the renamed entry must be found under the new name: %v", err)
	}
	if got.ID != old.ID {
		t.Fatalf("the row id must stay so the run history follows: %s != %s", got.ID, old.ID)
	}
	if !got.IncludeInSchedule {
		t.Fatal("settings must survive the rename")
	}
	names, _ := st.AliasNames("container", old.ID)
	if len(names) != 1 || names[0] != "radarr-movies" {
		t.Fatalf("alias not written: %v", names)
	}
	if _, err := st.GetTargetByContainer("radarr-movies"); err == nil {
		t.Fatal("the old name must no longer be an entry of its own")
	}
}

// TestRenameTargetWithAliasWritesDefinitionAtomically: the new definition lands
// with the rename, so a restore right after a takeover cannot recreate the
// container under the name it just gave up.
func TestRenameTargetWithAliasWritesDefinitionAtomically(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	if _, err := st.UpsertTarget(store.Target{ContainerName: "radarr-movies", Definition: "stale"}); err != nil {
		t.Fatal(err)
	}
	if err := st.RenameTargetWithAlias("radarr-movies", "radarr", "rewritten"); err != nil {
		t.Fatalf("RenameTargetWithAlias: %v", err)
	}
	got, err := st.GetTargetByContainer("radarr")
	if err != nil {
		t.Fatal(err)
	}
	if got.Definition != "rewritten" {
		t.Fatalf("Definition = %q, want %q", got.Definition, "rewritten")
	}
}

func TestRenameTargetWithAliasRefusesOccupiedName(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	oldEntry, err := st.UpsertTarget(store.Target{ContainerName: "radarr-movies"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetInclude("radarr-movies", true); err != nil {
		t.Fatal(err)
	}
	newEntry, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetInclude("radarr", true); err != nil {
		t.Fatal(err)
	}

	if err := st.RenameTargetWithAlias("radarr-movies", "radarr", ""); err == nil {
		t.Fatal("a name that still has its own entry must be refused")
	}

	oldAfter, err := st.GetTargetByContainer("radarr-movies")
	if err != nil {
		t.Fatalf("a refused rename must leave the old entry alone: %v", err)
	}
	if oldAfter.ID != oldEntry.ID {
		t.Fatalf("old entry ID must not change: %s != %s", oldAfter.ID, oldEntry.ID)
	}
	if !oldAfter.IncludeInSchedule {
		t.Fatal("old entry settings must survive the refusal")
	}

	newAfter, err := st.GetTargetByContainer("radarr")
	if err != nil {
		t.Fatalf("the pre-existing entry of the new name must survive: %v", err)
	}
	if newAfter.ID != newEntry.ID {
		t.Fatalf("new entry ID must not change: %s != %s", newAfter.ID, newEntry.ID)
	}
	if !newAfter.IncludeInSchedule {
		t.Fatal("new entry settings must survive the refusal")
	}

	_, err = st.AliasByOldName("container", "radarr-movies")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("no alias must leak on refusal: want sql.ErrNoRows, got %v", err)
	}
	oldAliases, _ := st.AliasNames("container", oldEntry.ID)
	if len(oldAliases) != 0 {
		t.Fatalf("old entry must have no aliases after refusal: %v", oldAliases)
	}
}

// TestRenameTargetWithAliasBackOntoItsOwnFormerName: renamed back, the entry
// owns its former name again, so that alias goes and the name it leaves
// becomes the alias, in the same transaction.
func TestRenameTargetWithAliasBackOntoItsOwnFormerName(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	tg, err := st.UpsertTarget(store.Target{ContainerName: "radarr-hd"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("container", "radarr", tg.ID, 100); err != nil {
		t.Fatal(err)
	}
	if err := st.RenameTargetWithAlias("radarr-hd", "radarr", ""); err != nil {
		t.Fatalf("RenameTargetWithAlias: %v", err)
	}
	if got, err := st.GetTargetByContainer("radarr"); err != nil || got.ID != tg.ID {
		t.Fatalf("entry on radarr = %+v, %v", got, err)
	}
	aliases, err := st.TargetAliases("container", tg.ID)
	if err != nil || len(aliases) != 1 || aliases[0].OldName != "radarr-hd" {
		t.Fatalf("aliases = %+v, %v; want radarr-hd only", aliases, err)
	}
}

// TestRenameTargetWithAliasRefusesAnotherEntrysFormerName: a former name
// belongs to one entry, so no other entry is renamed onto it.
func TestRenameTargetWithAliasRefusesAnotherEntrysFormerName(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	owner, err := st.UpsertTarget(store.Target{ContainerName: "radarr-hd"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("container", "radarr", owner.ID, 100); err != nil {
		t.Fatal(err)
	}
	other, err := st.UpsertTarget(store.Target{ContainerName: "radarr-movies"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RenameTargetWithAlias("radarr-movies", "radarr", ""); err == nil {
		t.Fatal("renaming onto another entry's former name must be refused")
	}
	if got, err := st.GetTargetByContainer("radarr-movies"); err != nil || got.ID != other.ID {
		t.Fatalf("the refused entry must keep its name: %+v, %v", got, err)
	}
	if a, err := st.AliasByOldName("container", "radarr"); err != nil || a.TargetID != owner.ID {
		t.Fatalf("the owner's alias must stay: %+v, %v", a, err)
	}
	if _, err := st.AliasByOldName("container", "radarr-movies"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("no alias may be written on refusal: %v", err)
	}
}

func TestGetTargetByID(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	tg, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetInclude("radarr", true); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetTargetByID(tg.ID)
	if err != nil {
		t.Fatalf("GetTargetByID: %v", err)
	}
	if got.ContainerName != "radarr" || !got.IncludeInSchedule {
		t.Fatalf("GetTargetByID = %+v", got)
	}
	if _, err := st.GetTargetByID("does-not-exist"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("unknown id: want sql.ErrNoRows, got %v", err)
	}
}

// TestUnlinkAlias: the target moves back to its old name, keeps its id and
// settings, and the alias row is gone.
func TestUnlinkAlias(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	old, err := st.UpsertTarget(store.Target{ContainerName: "radarr-movies"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetInclude("radarr-movies", true); err != nil {
		t.Fatal(err)
	}
	if err := st.RenameTargetWithAlias("radarr-movies", "radarr", ""); err != nil {
		t.Fatalf("RenameTargetWithAlias: %v", err)
	}

	if err := st.UnlinkAlias("radarr-movies", ""); err != nil {
		t.Fatalf("UnlinkAlias: %v", err)
	}
	got, err := st.GetTargetByContainer("radarr-movies")
	if err != nil {
		t.Fatalf("the entry must be found under its old name again: %v", err)
	}
	if got.ID != old.ID {
		t.Fatalf("the row id must stay so the run history follows: %s != %s", got.ID, old.ID)
	}
	if !got.IncludeInSchedule {
		t.Fatal("settings must survive the unlink")
	}
	if _, err := st.GetTargetByContainer("radarr"); err == nil {
		t.Fatal("the new name must no longer be an entry of its own")
	}
	if _, err := st.AliasByOldName("container", "radarr-movies"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("the alias must be gone: %v", err)
	}
}

// TestUnlinkAliasWritesDefinitionAtomically: the definition lands with the
// rename back, so the row never points at the name it just gave up.
func TestUnlinkAliasWritesDefinitionAtomically(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	if _, err := st.UpsertTarget(store.Target{ContainerName: "radarr-movies"}); err != nil {
		t.Fatal(err)
	}
	if err := st.RenameTargetWithAlias("radarr-movies", "radarr", "as-radarr"); err != nil {
		t.Fatalf("RenameTargetWithAlias: %v", err)
	}
	if err := st.UnlinkAlias("radarr-movies", "as-radarr-movies"); err != nil {
		t.Fatalf("UnlinkAlias: %v", err)
	}
	got, err := st.GetTargetByContainer("radarr-movies")
	if err != nil {
		t.Fatal(err)
	}
	if got.Definition != "as-radarr-movies" {
		t.Fatalf("Definition = %q, want %q", got.Definition, "as-radarr-movies")
	}
}

// TestUnlinkAliasRefusesOccupiedName: an unrelated entry has reused the old
// name since the takeover, and unlinking must not merge the two into one row.
func TestUnlinkAliasRefusesOccupiedName(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	takenOver, err := st.UpsertTarget(store.Target{ContainerName: "radarr-movies"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RenameTargetWithAlias("radarr-movies", "radarr", ""); err != nil {
		t.Fatalf("RenameTargetWithAlias: %v", err)
	}
	// A different container is installed under the freed name.
	reused, err := st.UpsertTarget(store.Target{ContainerName: "radarr-movies"})
	if err != nil {
		t.Fatal(err)
	}

	if err := st.UnlinkAlias("radarr-movies", ""); err == nil {
		t.Fatal("a name that has since been reused by a different entry must be refused")
	}

	// Nothing changed: both entries keep their own id under their own name, and
	// the alias is still recorded.
	stillRenamed, err := st.GetTargetByContainer("radarr")
	if err != nil || stillRenamed.ID != takenOver.ID {
		t.Fatalf("the taken-over entry must be untouched: %+v, %v", stillRenamed, err)
	}
	stillReused, err := st.GetTargetByContainer("radarr-movies")
	if err != nil || stillReused.ID != reused.ID {
		t.Fatalf("the reused entry must be untouched: %+v, %v", stillReused, err)
	}
	if _, err := st.AliasByOldName("container", "radarr-movies"); err != nil {
		t.Fatalf("the alias must survive the refusal: %v", err)
	}
}

// UnlinkAlias unlinks containers only, so a VM's former name of the same name
// stays linked to its VM.
func TestUnlinkAliasLeavesAVMsFormerNameAlone(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	vm, err := st.UpsertVMTarget(store.VMTarget{Name: "windows-11"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RenameVMTargetWithAlias("windows-11", "win11", "", ""); err != nil {
		t.Fatalf("RenameVMTargetWithAlias: %v", err)
	}
	if err := st.UnlinkAlias("windows-11", ""); err == nil {
		t.Fatal("UnlinkAlias must not unlink a VM's former name")
	}
	if a, err := st.AliasByOldName("vm", "windows-11"); err != nil || a.TargetID != vm.ID {
		t.Fatalf("the VM alias must stay: %+v, %v", a, err)
	}
	if got, err := st.GetVMTargetByName("win11"); err != nil || got.ID != vm.ID {
		t.Fatalf("the VM must keep its name: %+v, %v", got, err)
	}
}

// An alias whose entry is gone moves no row back, so the unlink fails and the
// alias stays rather than vanish with nothing unlinked.
func TestUnlinkAliasRefusesAnAliasWhoseEntryIsGone(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	if _, err := st.AddAliasAt("container", "radarr-movies", "no-such-entry", 100); err != nil {
		t.Fatal(err)
	}
	if err := st.UnlinkAlias("radarr-movies", ""); err == nil {
		t.Fatal("an unlink that moves no row back must fail")
	}
	if _, err := st.AliasByOldName("container", "radarr-movies"); err != nil {
		t.Fatalf("the alias must stay: %v", err)
	}
}

// TestUnlinkAliasRefusesUnknownName: a name that was never taken over
// has no alias to unlink; refused rather than treated as a silent no-op.
func TestUnlinkAliasRefusesUnknownName(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	if err := st.UnlinkAlias("never-linked", ""); err == nil {
		t.Fatal("an unknown old name must be refused")
	}
}

// TestDeleteTargetRemovesItsOwnAliases: a deleted entry's aliases go with it,
// so the unique (domain, old_name) index does not keep those names from
// becoming aliases again.
func TestDeleteTargetRemovesItsOwnAliases(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	if _, err := st.UpsertTarget(store.Target{ContainerName: "radarr-movies"}); err != nil {
		t.Fatal(err)
	}
	if err := st.RenameTargetWithAlias("radarr-movies", "radarr", ""); err != nil {
		t.Fatalf("RenameTargetWithAlias: %v", err)
	}

	if err := st.DeleteTarget("radarr"); err != nil {
		t.Fatalf("DeleteTarget: %v", err)
	}
	if _, err := st.GetTargetByContainer("radarr"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("the target must be gone: %v", err)
	}
	if _, err := st.AliasByOldName("container", "radarr-movies"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("the dangling alias must be gone too, got %v", err)
	}
	if _, err := st.UpsertTarget(store.Target{ContainerName: "radarr-movies"}); err != nil {
		t.Fatal(err)
	}
	if err := st.RenameTargetWithAlias("radarr-movies", "radarr2", ""); err != nil {
		t.Fatalf("radarr-movies must be reusable as an old name once the dangling alias is gone: %v", err)
	}
}

// TestDeleteTargetLeavesUnrelatedAliasAlone: an alias whose old_name equals
// the deleted name but which points at another live target survives, since
// ownership follows target_id, not the name.
func TestDeleteTargetLeavesUnrelatedAliasAlone(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	renamed, err := st.UpsertTarget(store.Target{ContainerName: "radarr-movies"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RenameTargetWithAlias("radarr-movies", "radarr", ""); err != nil {
		t.Fatalf("RenameTargetWithAlias: %v", err)
	}
	// An unrelated container reuses the freed name and is then deleted.
	if _, err := st.UpsertTarget(store.Target{ContainerName: "radarr-movies"}); err != nil {
		t.Fatal(err)
	}

	if err := st.DeleteTarget("radarr-movies"); err != nil {
		t.Fatalf("DeleteTarget: %v", err)
	}
	alias, err := st.AliasByOldName("container", "radarr-movies")
	if err != nil {
		t.Fatalf("the unrelated alias must survive: %v", err)
	}
	if alias.TargetID != renamed.ID {
		t.Fatalf("alias.TargetID = %q, want the renamed original's id %q", alias.TargetID, renamed.ID)
	}
	if _, err := st.GetTargetByContainer("radarr"); err != nil {
		t.Fatalf("the renamed original must be untouched: %v", err)
	}
}

// A VM alias pointing at the same id as the deleted container row belongs to
// the VM domain, so deleting the container leaves it alone.
func TestDeleteTargetLeavesAVMAliasWithTheSameIDAlone(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)

	tg, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("vm", "windows-11", tg.ID, 100); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteTarget("radarr"); err != nil {
		t.Fatalf("DeleteTarget: %v", err)
	}
	if _, err := st.AliasByOldName("vm", "windows-11"); err != nil {
		t.Fatalf("the VM alias must survive: %v", err)
	}
}
