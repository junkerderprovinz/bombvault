package store_test

import (
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// A container backed up by hand between two passes of an "every 3 days"
// schedule does not postpone the next pass for the others.
func TestADomainGateIsNotMovedByOneItemsBackup(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	now := time.Now()
	for _, name := range []string{"plex", "sonarr"} {
		if _, err := r.UpsertTarget(store.Target{ContainerName: name, IncludeInSchedule: true}); err != nil {
			t.Fatal(err)
		}
	}
	plex, err := r.GetTargetByContainer("plex")
	if err != nil {
		t.Fatal(err)
	}
	pass := now.AddDate(0, 0, -3)
	if err := r.RecordScheduleJobRun(store.ScheduleJobDomainPass("containers"), pass); err != nil {
		t.Fatal(err)
	}
	seedSuccess(t, db, r, plex.ID, now.Add(-13*time.Hour))

	last, err := schedule.ContainersDueGate(r)()
	if err != nil {
		t.Fatalf("gate: %v", err)
	}
	if !last.Equal(pass.Truncate(time.Second)) {
		t.Fatalf("gate = %v, want the pass three days ago", last)
	}
	if !schedule.EveryNDue(last, now, 3) {
		t.Fatal("three days after the last pass the domain must be due")
	}
}

// Before any pass has been recorded, as after an update from an older
// version, the gate keeps reading the items' own backups.
func TestADomainGateWithoutAPassReadsTheItems(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	tg, err := r.UpsertTarget(store.Target{ContainerName: "plex", IncludeInSchedule: true})
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().Add(-5 * time.Hour)
	seedSuccess(t, db, r, tg.ID, at)

	last, err := schedule.ContainersDueGate(r)()
	if err != nil {
		t.Fatalf("gate: %v", err)
	}
	if !last.Equal(at.Truncate(time.Second)) {
		t.Fatalf("gate = %v, want the item's backup at %v", last, at)
	}
}
