package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// A scheduled run visits never-backed-up targets first, then the oldest
// successful backup, then alphabetically, so an interrupted run cannot starve
// the same alphabetical tail every time. Only a successful run counts as a
// backup.
func TestListTargetsScheduleOrder(t *testing.T) {
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

	seedSuccess(ids["charlie"], 1000) // oldest successful backup
	seedSuccess(ids["alpha"], 9000)   // most recent successful backup

	// delta has only a failed run, so it still counts as never backed up.
	fr, err := r.StartRun(ids["delta"], "backup")
	if err != nil {
		t.Fatalf("StartRun(delta): %v", err)
	}
	if err := r.FinishRun(fr, "failed", "", 0, "boom"); err != nil {
		t.Fatalf("FinishRun(delta, failed): %v", err)
	}
	// bravo has no run at all.

	got, err := r.ListTargetsScheduleOrder()
	if err != nil {
		t.Fatalf("ListTargetsScheduleOrder: %v", err)
	}
	order := make([]string, 0, len(got))
	for _, tg := range got {
		order = append(order, tg.ContainerName)
	}

	want := []string{"bravo", "delta", "charlie", "alpha"}
	if len(order) != len(want) {
		t.Fatalf("schedule order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("schedule order = %v, want %v", order, want)
		}
	}

	// ListTargets feeds the UI and stays alphabetical.
	ui, err := r.ListTargets()
	if err != nil {
		t.Fatalf("ListTargets: %v", err)
	}
	uiOrder := make([]string, 0, len(ui))
	for _, tg := range ui {
		uiOrder = append(uiOrder, tg.ContainerName)
	}
	wantUI := []string{"alpha", "bravo", "charlie", "delta"}
	for i := range wantUI {
		if uiOrder[i] != wantUI[i] {
			t.Fatalf("ListTargets (UI) order = %v, want alphabetical %v", uiOrder, wantUI)
		}
	}
}
