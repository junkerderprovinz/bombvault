package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestSortTargetsForRun pins the #119 sequencing at the pure-function level:
// containers with an explicit backup order (>0) come first in ascending order,
// and the unordered rest (0) keep their INCOMING order — which callers pass
// most-overdue-first, so the existing #95 tiebreak survives untouched.
func TestSortTargetsForRun(t *testing.T) {
	// Incoming order is the overdue-first order the SQL produces. bravo/delta are
	// unordered; charlie and alpha carry explicit orders that must jump the queue.
	targets := []store.Target{
		{ContainerName: "bravo", BackupOrder: 0},
		{ContainerName: "delta", BackupOrder: 0},
		{ContainerName: "charlie", BackupOrder: 2},
		{ContainerName: "alpha", BackupOrder: 1},
	}
	store.SortTargetsForRun(targets)

	got := names(targets)
	// alpha(1), charlie(2) lead by explicit order; bravo, delta follow in the
	// incoming (overdue-first) order.
	want := []string{"alpha", "charlie", "bravo", "delta"}
	assertOrder(t, got, want)
}

// TestSortTargetsForRunNoExplicit proves the sort is a no-op when nothing has an
// explicit order: the incoming overdue-first order is returned verbatim (the
// stability guarantee that keeps #95 behavior EXACTLY as before).
func TestSortTargetsForRunNoExplicit(t *testing.T) {
	incoming := []string{"bravo", "delta", "charlie", "alpha"}
	targets := make([]store.Target, 0, len(incoming))
	for _, n := range incoming {
		targets = append(targets, store.Target{ContainerName: n})
	}
	store.SortTargetsForRun(targets)
	assertOrder(t, names(targets), incoming)
}

// TestListTargetsScheduleOrderWithBackupOrder proves the schedule/batch ordering
// combines both rules end to end through the DB: explicit backup order first,
// then the most-overdue-first tiebreak for the unordered remainder.
func TestListTargetsScheduleOrderWithBackupOrder(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	r := store.New(db)

	ids := map[string]string{}
	for _, n := range []string{"alpha", "bravo", "charlie", "delta"} {
		tg, err := r.UpsertTarget(store.Target{ContainerName: n})
		if err != nil {
			t.Fatalf("UpsertTarget %s: %v", n, err)
		}
		ids[n] = tg.ID
	}

	seedSuccess := func(id string, finishedAt int64) {
		runID, err := r.StartRun(id, "backup")
		if err != nil {
			t.Fatalf("StartRun: %v", err)
		}
		if err := r.FinishRun(runID, "success", "snap", 1, ""); err != nil {
			t.Fatalf("FinishRun: %v", err)
		}
		if _, err := db.Exec(`UPDATE runs SET finished_at = ? WHERE id = ?`, finishedAt, runID); err != nil {
			t.Fatalf("backdate finished_at: %v", err)
		}
	}
	// Without explicit orders the #95 order would be: bravo, delta (never), then
	// charlie (oldest success), then alpha (newest success).
	seedSuccess(ids["charlie"], 1000)
	seedSuccess(ids["alpha"], 9000)

	// Give delta and alpha explicit orders — they must lead, in that order.
	if err := r.SetBackupOrder("delta", 1); err != nil {
		t.Fatalf("SetBackupOrder(delta): %v", err)
	}
	if err := r.SetBackupOrder("alpha", 2); err != nil {
		t.Fatalf("SetBackupOrder(alpha): %v", err)
	}

	got, err := r.ListTargetsScheduleOrder()
	if err != nil {
		t.Fatalf("ListTargetsScheduleOrder: %v", err)
	}
	// delta(1), alpha(2) first by explicit order; then the unordered remainder in
	// overdue-first order: bravo (never), charlie (oldest success).
	want := []string{"delta", "alpha", "bravo", "charlie"}
	assertOrder(t, names(got), want)
}

// TestListTargetsScheduleOrderUnchangedWithoutOrder guards the no-regression
// promise: with no explicit backup order set, the schedule order is EXACTLY the
// pre-#119 most-overdue-first order.
func TestListTargetsScheduleOrderUnchangedWithoutOrder(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	r := store.New(db)

	ids := map[string]string{}
	for _, n := range []string{"alpha", "bravo", "charlie", "delta"} {
		tg, err := r.UpsertTarget(store.Target{ContainerName: n})
		if err != nil {
			t.Fatalf("UpsertTarget %s: %v", n, err)
		}
		ids[n] = tg.ID
	}
	seedSuccess := func(id string, finishedAt int64) {
		runID, err := r.StartRun(id, "backup")
		if err != nil {
			t.Fatalf("StartRun: %v", err)
		}
		if err := r.FinishRun(runID, "success", "snap", 1, ""); err != nil {
			t.Fatalf("FinishRun: %v", err)
		}
		if _, err := db.Exec(`UPDATE runs SET finished_at = ? WHERE id = ?`, finishedAt, runID); err != nil {
			t.Fatalf("backdate finished_at: %v", err)
		}
	}
	seedSuccess(ids["charlie"], 1000)
	seedSuccess(ids["alpha"], 9000)

	got, err := r.ListTargetsScheduleOrder()
	if err != nil {
		t.Fatalf("ListTargetsScheduleOrder: %v", err)
	}
	want := []string{"bravo", "delta", "charlie", "alpha"}
	assertOrder(t, names(got), want)
}

// TestBackupOrderRoundTrip proves setting and getting the ordering round-trips,
// that SetBackupOrders is authoritative (a dropped container returns to
// unordered), and that a positive order can be assigned to a container with no
// target row yet (create-on-miss).
func TestBackupOrderRoundTrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	r := store.New(db)

	// A never-seen container (no target row) can still be ordered.
	if err := r.SetBackupOrders([]store.ContainerOrder{
		{Container: "db", Order: 1},
		{Container: "app", Order: 2},
	}); err != nil {
		t.Fatalf("SetBackupOrders: %v", err)
	}
	got, err := r.BackupOrders()
	if err != nil {
		t.Fatalf("BackupOrders: %v", err)
	}
	if len(got) != 2 || got[0].Container != "db" || got[0].Order != 1 || got[1].Container != "app" || got[1].Order != 2 {
		t.Fatalf("BackupOrders round-trip = %+v, want db=1, app=2", got)
	}

	// Reapplying WITHOUT "app" must return it to unordered (authoritative replace).
	if err := r.SetBackupOrders([]store.ContainerOrder{{Container: "db", Order: 1}}); err != nil {
		t.Fatalf("SetBackupOrders (replace): %v", err)
	}
	got, err = r.BackupOrders()
	if err != nil {
		t.Fatalf("BackupOrders (after replace): %v", err)
	}
	if len(got) != 1 || got[0].Container != "db" {
		t.Fatalf("after replace BackupOrders = %+v, want only db", got)
	}
	tg, err := r.GetTargetByContainer("app")
	if err != nil {
		t.Fatalf("GetTargetByContainer(app): %v", err)
	}
	if tg.BackupOrder != 0 {
		t.Fatalf("app BackupOrder = %d, want 0 after being dropped from the order", tg.BackupOrder)
	}
}

// TestUpsertTargetPreservesBackupOrder guards that a backup's UpsertTarget (which
// refreshes appdata/definition) never clobbers the user's chosen order — same
// ownership contract as stop_containers/excludes.
func TestUpsertTargetPreservesBackupOrder(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	r := store.New(db)

	if _, err := r.UpsertTarget(store.Target{ContainerName: "app"}); err != nil {
		t.Fatalf("UpsertTarget: %v", err)
	}
	if err := r.SetBackupOrder("app", 5); err != nil {
		t.Fatalf("SetBackupOrder: %v", err)
	}
	// A later backup upserts the same container with fresh appdata/definition and
	// BackupOrder left at its zero value — the order must survive.
	if _, err := r.UpsertTarget(store.Target{ContainerName: "app", Definition: "{}"}); err != nil {
		t.Fatalf("UpsertTarget (refresh): %v", err)
	}
	tg, err := r.GetTargetByContainer("app")
	if err != nil {
		t.Fatalf("GetTargetByContainer: %v", err)
	}
	if tg.BackupOrder != 5 {
		t.Fatalf("BackupOrder = %d, want 5 preserved across Upsert", tg.BackupOrder)
	}
}

// TestOrderContainerNamesForRun proves the batch (multi-select) path sequences a
// SELECTED subset exactly like a scheduled run: explicit order first, overdue
// tiebreak after, and a selected name with no target row is never dropped.
func TestOrderContainerNamesForRun(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	r := store.New(db)

	ids := map[string]string{}
	for _, n := range []string{"alpha", "bravo", "charlie"} {
		tg, err := r.UpsertTarget(store.Target{ContainerName: n})
		if err != nil {
			t.Fatalf("UpsertTarget %s: %v", n, err)
		}
		ids[n] = tg.ID
	}
	// charlie has an old success, alpha a newer one, bravo never — overdue order
	// among the unordered is bravo, charlie, alpha.
	seed := func(id string, at int64) {
		runID, err := r.StartRun(id, "backup")
		if err != nil {
			t.Fatalf("StartRun: %v", err)
		}
		if err := r.FinishRun(runID, "success", "snap", 1, ""); err != nil {
			t.Fatalf("FinishRun: %v", err)
		}
		if _, err := db.Exec(`UPDATE runs SET finished_at = ? WHERE id = ?`, at, runID); err != nil {
			t.Fatalf("backdate: %v", err)
		}
	}
	seed(ids["charlie"], 1000)
	seed(ids["alpha"], 9000)
	if err := r.SetBackupOrder("alpha", 1); err != nil {
		t.Fatalf("SetBackupOrder(alpha): %v", err)
	}

	// Selection is passed in an arbitrary order and includes "ghost" (no target).
	got, err := r.OrderContainerNamesForRun([]string{"bravo", "ghost", "charlie", "alpha"})
	if err != nil {
		t.Fatalf("OrderContainerNamesForRun: %v", err)
	}
	// alpha(explicit 1) first; then unordered overdue-first bravo, charlie; then
	// the unknown "ghost" appended (never dropped).
	want := []string{"alpha", "bravo", "charlie", "ghost"}
	assertOrder(t, got, want)
}

// names extracts container names in slice order.
func names(targets []store.Target) []string {
	out := make([]string, 0, len(targets))
	for _, t := range targets {
		out = append(out, t.ContainerName)
	}
	return out
}

// assertOrder fails the test unless got matches want exactly (same length + order).
func assertOrder(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}
