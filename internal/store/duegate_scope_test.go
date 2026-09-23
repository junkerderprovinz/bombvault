package store_test

// A domain's everyN due-gate asks whether the domain's scheduled pass has
// waited out its interval, so only items that pass runs may answer. With
// per-item schedules on, a container with its own daily cron entry is left out
// of the domain run. If its nightly success counted, a weekly containers gate
// would see a fresh timestamp every night and never back up the rest.

import (
	"database/sql"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// seedSuccess records a successful backup for targetID and back-dates it.
func seedSuccess(t *testing.T, db *sql.DB, r *store.Repo, targetID string, at time.Time) {
	t.Helper()
	runID, err := r.StartRun(targetID, "backup")
	if err != nil {
		t.Fatalf("StartRun(%s): %v", targetID, err)
	}
	if err := r.FinishRun(runID, "success", "snap", 1, ""); err != nil {
		t.Fatalf("FinishRun(%s): %v", targetID, err)
	}
	if _, err := db.Exec(`UPDATE runs SET finished_at = ?, started_at = ? WHERE id = ?`, at.Unix(), at.Unix(), runID); err != nil {
		t.Fatalf("backdate run: %v", err)
	}
}

func perItemOn(t *testing.T, r *store.Repo) {
	t.Helper()
	if _, err := r.MutateSettings(func(s *store.Settings) error {
		s.PerItemSchedules = true
		return nil
	}); err != nil {
		t.Fatalf("MutateSettings: %v", err)
	}
}

func TestContainersDueGateIgnoresItemsTheDomainRunSkips(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	perItemOn(t, r)

	now := time.Now()
	mk := func(name, cadence string, include bool) string {
		tg, err := r.UpsertTarget(store.Target{ContainerName: name})
		if err != nil {
			t.Fatalf("UpsertTarget %s: %v", name, err)
		}
		if err := r.SetInclude(name, include); err != nil {
			t.Fatalf("SetInclude %s: %v", name, err)
		}
		if cadence != "" {
			if err := r.SetScheduleCadence(name, cadence); err != nil {
				t.Fatalf("SetScheduleCadence %s: %v", name, err)
			}
		}
		return tg.ID
	}

	plex := mk("plex", "daily 01:00", true) // own cron entry, not in the domain run
	paused := mk("paused", "off", true)     // not scheduled at all
	excluded := mk("excluded", "", false)   // not included in the schedule
	sonarr := mk("sonarr", "", true)        // a plain member of the domain run
	seedSuccess(t, db, r, plex, now.Add(-2*time.Hour))
	seedSuccess(t, db, r, paused, now.Add(-3*time.Hour))
	seedSuccess(t, db, r, excluded, now.Add(-4*time.Hour)) // e.g. a manual backup
	seedSuccess(t, db, r, sonarr, now.AddDate(0, 0, -10))

	gate := schedule.ContainersDueGate(r)
	last, err := gate()
	if err != nil {
		t.Fatalf("gate: %v", err)
	}
	if age := now.Sub(last); age < 24*time.Hour {
		t.Fatalf("the gate measured a %v-old backup: only items the domain run covers may answer for it", age.Round(time.Minute))
	}
	if !schedule.EveryNDue(last, now, 7) {
		t.Fatal("the containers domain has not been backed up for 10 days and must be due")
	}

	// The dashboard's "newest success anywhere" query answers with plex's
	// two-hour-old run and would hold the gate shut.
	anyItem, err := r.LastSuccessfulContainerBackup()
	if err != nil {
		t.Fatal(err)
	}
	if schedule.EveryNDue(anyItem, now, 7) {
		t.Fatal("precondition: 'newest success anywhere' is expected to read not due here, which is the starvation under test")
	}
}

func TestVMsDueGateIgnoresItemsTheDomainRunSkips(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	perItemOn(t, r)

	now := time.Now()
	mk := func(name, cadence string, include bool) string {
		vm, err := r.UpsertVMTarget(store.VMTarget{Name: name, Method: "graceful"})
		if err != nil {
			t.Fatalf("UpsertVMTarget %s: %v", name, err)
		}
		if err := r.SetVMInclude(name, include); err != nil {
			t.Fatalf("SetVMInclude %s: %v", name, err)
		}
		if cadence != "" {
			if err := r.SetVMScheduleCadence(name, cadence); err != nil {
				t.Fatalf("SetVMScheduleCadence %s: %v", name, err)
			}
		}
		return vm.ID
	}

	nightly := mk("win11", "daily 01:00", true)
	member := mk("debian", "", true)
	seedSuccess(t, db, r, nightly, now.Add(-2*time.Hour))
	seedSuccess(t, db, r, member, now.AddDate(0, 0, -10))

	last, err := schedule.VMsDueGate(r)()
	if err != nil {
		t.Fatalf("gate: %v", err)
	}
	if !schedule.EveryNDue(last, now, 7) {
		t.Fatal("the VMs domain has not been backed up for 10 days and must be due")
	}
}

// TestFilesDueGateIgnoresDisabledSets checks that a disabled file set, which is
// not part of the domain run, cannot hold the gate closed for the sets that are
// still on.
func TestFilesDueGateIgnoresDisabledSets(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	now := time.Now()
	off, err := r.CreateFileSet(store.FileSet{Name: "archive", Path: "archive", Enabled: false})
	if err != nil {
		t.Fatalf("CreateFileSet: %v", err)
	}
	on, err := r.CreateFileSet(store.FileSet{Name: "photos", Path: "photos", Enabled: true})
	if err != nil {
		t.Fatalf("CreateFileSet: %v", err)
	}
	seedSuccess(t, db, r, off.ID, now.Add(-2*time.Hour))
	seedSuccess(t, db, r, on.ID, now.AddDate(0, 0, -10))

	last, err := schedule.FilesDueGate(r)()
	if err != nil {
		t.Fatalf("gate: %v", err)
	}
	if !schedule.EveryNDue(last, now, 7) {
		t.Fatal("the enabled file set has not been backed up for 10 days and must be due")
	}
}

// TestDueGateNeverBackedUpIsDue expects the zero time, which the scheduler
// reads as due, when there is nothing to measure: no error and no time from an
// item the pass does not run.
func TestDueGateNeverBackedUpIsDue(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	perItemOn(t, r)

	if _, err := r.UpsertTarget(store.Target{ContainerName: "fresh"}); err != nil {
		t.Fatal(err)
	}
	if err := r.SetInclude("fresh", true); err != nil {
		t.Fatal(err)
	}

	last, err := schedule.ContainersDueGate(r)()
	if err != nil {
		t.Fatalf("gate: %v", err)
	}
	if !last.IsZero() {
		t.Fatalf("expected the definite 'never ran' zero time, got %v", last)
	}
	if !schedule.EveryNDue(last, time.Now(), 7) {
		t.Fatal("a domain that has never been backed up must be due")
	}
}

// TestLastSuccessfulBackupAmongScopesAndChunks checks that only the given ids
// count, that an empty list gives the zero time and that a list longer than one
// IN (...) chunk still finds the newest.
func TestLastSuccessfulBackupAmongScopesAndChunks(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)
	now := time.Now()

	var ids []string
	for i := 0; i < 900; i++ {
		tg, err := r.UpsertTarget(store.Target{ContainerName: "c" + string(rune('a'+i%26)) + time.Duration(i).String()})
		if err != nil {
			t.Fatalf("UpsertTarget: %v", err)
		}
		ids = append(ids, tg.ID)
	}
	// The newest success in the list sits in the last chunk; a newer one lies
	// outside the list.
	seedSuccess(t, db, r, ids[len(ids)-1], now.AddDate(0, 0, -3))
	outsider, err := r.UpsertTarget(store.Target{ContainerName: "outsider"})
	if err != nil {
		t.Fatal(err)
	}
	seedSuccess(t, db, r, outsider.ID, now)

	got, err := r.LastSuccessfulBackupAmong(ids)
	if err != nil {
		t.Fatalf("LastSuccessfulBackupAmong: %v", err)
	}
	if d := now.Sub(got); d < 2*24*time.Hour || d > 4*24*time.Hour {
		t.Fatalf("got a %v-old timestamp, want the 3-day-old success inside the list", d.Round(time.Hour))
	}

	empty, err := r.LastSuccessfulBackupAmong(nil)
	if err != nil {
		t.Fatalf("empty list must not error: %v", err)
	}
	if !empty.IsZero() {
		t.Fatalf("an empty id list must report the definite zero time, got %v", empty)
	}
}
