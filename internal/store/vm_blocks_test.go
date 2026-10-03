package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestVMBlockBackupKeepsSwitchAndLastRunApart(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	tg, err := r.UpsertVMTarget(store.VMTarget{Name: "win"})
	if err != nil {
		t.Fatal(err)
	}
	if b, err := r.GetVMBlockBackup(tg.ID); err != nil || b.Enabled || b.LastMode != "" {
		t.Fatalf("a VM without a row = %+v, %v; want off", b, err)
	}
	if err := r.RecordVMBlockBackupRun(tg.ID, "full", "first", 10); err != nil {
		t.Fatal(err)
	}
	if err := r.SetVMBlockBackupEnabled(tg.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := r.RecordVMBlockBackupRun(tg.ID, "changed", "", 20); err != nil {
		t.Fatal(err)
	}
	all, err := r.ListVMBlockBackups()
	if err != nil {
		t.Fatal(err)
	}
	want := store.VMBlockBackup{TargetID: tg.ID, Enabled: true, LastMode: "changed", LastAt: 20}
	if all[tg.ID] != want {
		t.Fatalf("row = %+v, want %+v", all[tg.ID], want)
	}
	if err := r.DeleteVMTarget("win", nil); err != nil {
		t.Fatal(err)
	}
	if b, _ := r.GetVMBlockBackup(tg.ID); b.Enabled {
		t.Fatal("the switch outlived the VM entry")
	}
}
