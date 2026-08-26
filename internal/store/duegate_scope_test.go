package store_test

// What a domain's everyN due-gate is allowed to measure.
//
// The gate asks "has this domain's scheduled pass waited out its interval?".
// It used to be answered by "the newest successful backup of ANY row in the
// domain's table", which is a different question and starves the domain:
//
//	PerItemSchedules on, ContainersSchedule "everyN 7 03:00", 44 containers,
//	one of them ("plex") overridden to "daily 01:00". registerPerItemEntries
//	gives plex its own cron entry and DomainRunTargets REMOVES it from the
//	domain run. Every night plex writes a fresh successful run, so at 03:00 the
//	domain gate sees a two-hour-old timestamp and skips — every night, for
//	good. The other 43 containers are never backed up by the schedule again,
//	and the dashboard's RPO chip stays green because it reads the same query.
//
// These tests run the REAL gate (schedule.ContainersDueGate and friends,
// exactly as main.go and api.go wire them) against a real database, and pin the
// separation against the OLD feed in the same assertion, so the difference is
// visible without having to remember what the code used to do.

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

// TestContainersDueGateIgnoresItemsTheDomainRunSkips is the starvation case.
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

	plex := mk("plex", "daily 01:00", true) // own cron entry, NOT in the domain run
	paused := mk("paused", "off", true)     // deliberately not scheduled at all
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

	// The OLD feed, still present because the dashboard's protection currency
	// genuinely wants it, would have answered with plex's two-hour-old run and
	// held the gate shut — the defect, pinned here so the two questions cannot
	// be collapsed back into one query.
	anyItem, err := r.LastSuccessfulContainerBackup()
	if err != nil {
		t.Fatal(err)
	}
	if schedule.EveryNDue(anyItem, now, 7) {
		t.Fatal("precondition: 'newest success anywhere' is expected to read NOT due here — that is the starvation being guarded")
	}
}

// TestVMsDueGateIgnoresItemsTheDomainRunSkips is the VM counterpart.
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

// TestFilesDueGateIgnoresDisabledSets: a file set the user switched off is not
// part of the domain run, so its last backup must not hold the gate closed for
// the sets that are still on.
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

// TestDueGateNeverBackedUpIsDue: with nothing to measure, the gate reports the
// definite "never" (zero time), which the scheduler reads as due — the same
// answer a fresh install gets. It must not report an error, and it must not
// report a stale time from an item the pass does not run.
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

// TestLastSuccessfulBackupAmongScopesAndChunks pins the store primitive itself:
// only the given ids count, an empty list is a definite zero, and a list longer
// than one IN (...) chunk still finds the newest.
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
	// The newest success sits in the LAST chunk, an unrelated newer one outside
	// the list entirely.
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
