package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Containers with an explicit backup order (>0) come first in ascending order.
// The rest keep their incoming order, which callers pass most-overdue-first.
func TestSortTargetsForRun(t *testing.T) {
	targets := []store.Target{
		{ContainerName: "bravo", BackupOrder: 0},
		{ContainerName: "delta", BackupOrder: 0},
		{ContainerName: "charlie", BackupOrder: 2},
		{ContainerName: "alpha", BackupOrder: 1},
	}
	store.SortTargetsForRun(targets)

	got := names(targets)
	want := []string{"alpha", "charlie", "bravo", "delta"}
	assertOrder(t, got, want)
}

// Without explicit orders the sort is stable and changes nothing.
func TestSortTargetsForRunNoExplicit(t *testing.T) {
	incoming := []string{"bravo", "delta", "charlie", "alpha"}
	targets := make([]store.Target, 0, len(incoming))
	for _, n := range incoming {
		targets = append(targets, store.Target{ContainerName: n})
	}
	store.SortTargetsForRun(targets)
	assertOrder(t, names(targets), incoming)
}

// Explicit backup order first, then most-overdue-first for the rest.
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
	// Without explicit orders this would be bravo, delta (never backed up),
	// charlie, alpha.
	seedSuccess(ids["charlie"], 1000)
	seedSuccess(ids["alpha"], 9000)

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
	want := []string{"delta", "alpha", "bravo", "charlie"}
	assertOrder(t, names(got), want)
}

func TestListTargetsScheduleOrderWithoutBackupOrder(t *testing.T) {
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

// SetBackupOrders replaces the whole ordering: a container left out returns to
// unordered.
func TestBackupOrderRoundTrip(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	r := store.New(db)

	// Neither container has a target row yet.
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

	// Reapplying without app returns it to unordered.
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

// A backup's UpsertTarget refreshes appdata and definition but keeps the user's
// order, as it keeps stop_containers and excludes.
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
	// A backup upserts with BackupOrder left at zero.
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

// A multi-select batch is ordered like a scheduled run, and a selected name
// without a target row goes last instead of being dropped.
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
	// Overdue order: bravo (never backed up), charlie, alpha.
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

	// ghost has no target row.
	got, err := r.OrderContainerNamesForRun([]string{"bravo", "ghost", "charlie", "alpha"})
	if err != nil {
		t.Fatalf("OrderContainerNamesForRun: %v", err)
	}
	want := []string{"alpha", "bravo", "charlie", "ghost"}
	assertOrder(t, got, want)
}

func names(targets []store.Target) []string {
	out := make([]string, 0, len(targets))
	for _, t := range targets {
		out = append(out, t.ContainerName)
	}
	return out
}

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
