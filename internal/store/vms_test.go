package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestUpsertVMTargetRoundtrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	tg, err := r.UpsertVMTarget(store.VMTarget{
		Name:              "win10",
		Method:            "graceful",
		IncludeInSchedule: false,
		Definition:        `{"xml":"<domain/>"}`,
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if tg.ID == "" {
		t.Fatal("ID must be assigned")
	}
	if tg.Name != "win10" {
		t.Fatalf("name = %q", tg.Name)
	}
	if tg.CreatedAt == 0 {
		t.Fatal("created_at must be set")
	}

	// Re-read.
	got, err := r.GetVMTargetByName("win10")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != tg.ID {
		t.Fatalf("id mismatch: %q vs %q", got.ID, tg.ID)
	}
	if got.Definition != `{"xml":"<domain/>"}` {
		t.Fatalf("definition mismatch: %q", got.Definition)
	}
}

func TestUpsertVMTargetConflictPreservesID(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	first, err := r.UpsertVMTarget(store.VMTarget{Name: "ubuntu", Method: "graceful"})
	if err != nil {
		t.Fatal(err)
	}
	// Upsert again with updated definition — ID and created_at must be preserved.
	second, err := r.UpsertVMTarget(store.VMTarget{Name: "ubuntu", Method: "graceful", Definition: "updated"})
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Fatalf("conflict must preserve original ID: %q vs %q", second.ID, first.ID)
	}
	if second.Definition != "updated" {
		t.Fatalf("definition not updated: %q", second.Definition)
	}
}

func TestUpsertVMTargetStoresUUID(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	tg, err := r.UpsertVMTarget(store.VMTarget{
		Name:   "win10",
		Method: "graceful",
		UUID:   "4a9b3fa1-4e77-4f2a-9c1f-2b6e9d1a7c33",
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if tg.UUID != "4a9b3fa1-4e77-4f2a-9c1f-2b6e9d1a7c33" {
		t.Fatalf("UUID = %q", tg.UUID)
	}

	got, err := r.GetVMTargetByName("win10")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.UUID != "4a9b3fa1-4e77-4f2a-9c1f-2b6e9d1a7c33" {
		t.Fatalf("UUID after re-read = %q", got.UUID)
	}
}

func TestUpsertVMTargetEmptyUUIDDoesNotEraseStoredUUID(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	if _, err := r.UpsertVMTarget(store.VMTarget{
		Name:   "ubuntu",
		Method: "graceful",
		UUID:   "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
	}); err != nil {
		t.Fatal(err)
	}

	second, err := r.UpsertVMTarget(store.VMTarget{Name: "ubuntu", Method: "graceful", Definition: "updated"})
	if err != nil {
		t.Fatal(err)
	}
	if second.UUID != "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" {
		t.Fatalf("an empty incoming UUID must not erase the stored one: got %q", second.UUID)
	}
}

func TestListVMTargets(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	for _, name := range []string{"vmB", "vmA", "vmC"} {
		if _, err := r.UpsertVMTarget(store.VMTarget{Name: name, Method: "graceful"}); err != nil {
			t.Fatal(err)
		}
	}
	list, err := r.ListVMTargets()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3, got %d", len(list))
	}
	// ORDER BY name
	if list[0].Name != "vmA" || list[1].Name != "vmB" || list[2].Name != "vmC" {
		t.Fatalf("order wrong: %v", list)
	}
}

func TestSetVMMethod(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	if _, err := r.UpsertVMTarget(store.VMTarget{Name: "fedora", Method: "graceful"}); err != nil {
		t.Fatal(err)
	}
	if err := r.SetVMMethod("fedora", "graceful"); err != nil {
		t.Fatal(err)
	}
	tg, err := r.GetVMTargetByName("fedora")
	if err != nil {
		t.Fatal(err)
	}
	if tg.Method != "graceful" {
		t.Fatalf("method = %q", tg.Method)
	}
}

func TestSetVMUUID(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	if _, err := r.UpsertVMTarget(store.VMTarget{Name: "fedora", Method: "graceful"}); err != nil {
		t.Fatal(err)
	}
	if err := r.SetVMUUID("fedora", "4a9b3fa1-4e77-4f2a-9c1f-2b6e9d1a7c33"); err != nil {
		t.Fatal(err)
	}
	tg, err := r.GetVMTargetByName("fedora")
	if err != nil {
		t.Fatal(err)
	}
	if tg.UUID != "4a9b3fa1-4e77-4f2a-9c1f-2b6e9d1a7c33" {
		t.Fatalf("UUID = %q", tg.UUID)
	}
}

func TestSetVMInclude(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	if _, err := r.UpsertVMTarget(store.VMTarget{Name: "archvm", Method: "graceful"}); err != nil {
		t.Fatal(err)
	}
	if err := r.SetVMInclude("archvm", true); err != nil {
		t.Fatal(err)
	}
	tg, err := r.GetVMTargetByName("archvm")
	if err != nil {
		t.Fatal(err)
	}
	if !tg.IncludeInSchedule {
		t.Fatal("include must be true after SetVMInclude(true)")
	}
}

func TestDeleteVMTarget(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	tg, err := r.UpsertVMTarget(store.VMTarget{Name: "deleteme", Method: "graceful"})
	if err != nil {
		t.Fatal(err)
	}
	// Seed a run referencing this VM target.
	runID, err := r.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.FinishRun(runID, "success", "abc123", 1024, ""); err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteVMTarget("deleteme"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := r.GetVMTargetByName("deleteme"); err == nil {
		t.Fatal("target must be gone after delete")
	}
	// Runs must also be gone (cascade in tx).
	runs, _ := r.ListRuns(100)
	for _, run := range runs {
		if run.TargetID == tg.ID {
			t.Fatalf("run for deleted VM target must be removed: %+v", run)
		}
	}
}

func TestDeleteVMTargetNotFoundIsNoop(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	// Deleting a non-existent VM must not error.
	if err := r.DeleteVMTarget("ghost"); err != nil {
		t.Fatalf("delete non-existent: %v", err)
	}
}

func TestDeleteVMTargetRemovesAnomalyState(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	tg, err := r.UpsertVMTarget(store.VMTarget{Name: "deleteme", Method: "graceful"})
	if err != nil {
		t.Fatal(err)
	}
	seedAnomalyState(t, r, tg.ID, "vm")

	if err := r.DeleteVMTarget("deleteme"); err != nil {
		t.Fatalf("DeleteVMTarget: %v", err)
	}
	assertAnomalyStateGone(t, r, tg.ID)
}
