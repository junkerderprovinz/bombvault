package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestTheLastSuccessfulBackupOfADomainNamesItsRun(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	finish := func(target, status string) string {
		t.Helper()
		id, err := r.StartRun(target, "backup")
		if err != nil {
			t.Fatal(err)
		}
		if err := r.FinishRun(id, status, "snap-"+id, 1, ""); err != nil {
			t.Fatal(err)
		}
		return id
	}

	if _, ok, err := r.LastSuccessfulBackupRun("flash"); err != nil || ok {
		t.Fatalf("before any backup: ok=%v, %v", ok, err)
	}
	flash := finish(store.FlashTargetID, "success")
	finish(store.FlashTargetID, "failed")
	tg, err := r.UpsertTarget(store.Target{ContainerName: "sonarr", AppdataPaths: []string{"/data"}})
	if err != nil {
		t.Fatal(err)
	}
	sonarr := finish(tg.ID, "success")

	run, ok, err := r.LastSuccessfulBackupRun("flash")
	if err != nil || !ok || run.ID != flash || run.FinishedAt == nil {
		t.Fatalf("flash: %+v, ok=%v, %v, want run %s", run, ok, err, flash)
	}
	if run, ok, err := r.LastSuccessfulBackupRun("containers"); err != nil || !ok || run.ID != sonarr {
		t.Fatalf("containers: %+v, ok=%v, %v, want run %s", run, ok, err, sonarr)
	}
	if _, ok, err := r.LastSuccessfulBackupRun("vms"); err != nil || ok {
		t.Fatalf("vms without a backup: ok=%v, %v", ok, err)
	}
	if _, _, err := r.LastSuccessfulBackupRun("photos"); err == nil {
		t.Fatal("an unknown domain read as known")
	}
}
