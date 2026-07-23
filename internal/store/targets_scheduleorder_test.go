package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestListTargetsScheduleOrder pins the #95 anti-starvation ordering: a scheduled
// run must visit never-backed-up targets first, then the least-recently
// successfully backed-up, then alphabetically — so a slow or interrupted run can
// never perpetually starve the same alphabetical tail. Only a SUCCESSFUL backup run
// counts as "backed up"; a failed run leaves the target in the never-backed-up head.
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

	// seedSuccess records a successful backup and back-dates its finished_at so the
	// ordering by "oldest success first" is testable deterministically.
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

	// delta has ONLY a failed run — it must still count as never-backed-up.
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

	// never-backed-up first (bravo, delta — alphabetical among them), then by oldest
	// success (charlie at t=1000 before alpha at t=9000).
	want := []string{"bravo", "delta", "charlie", "alpha"}
	if len(order) != len(want) {
		t.Fatalf("schedule order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("schedule order = %v, want %v", order, want)
		}
	}

	// The UI list (ListTargets) must remain strictly alphabetical — the schedule
	// ordering is scoped to the scheduler and must not have leaked into it.
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
