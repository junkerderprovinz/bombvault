package store_test

import (
	"database/sql"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestReceivedAlertStateCRUD exercises the dead-mans-switch episode memory: an
// unrecorded source is (false, nil); an upsert records it; a second upsert on the
// same key refreshes in place; delete removes one source; and the per-repo purge
// clears every episode for a repo (the DELETE-repo cleanup path).
func TestReceivedAlertStateCRUD(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	if _, ok, err := r.GetReceivedAlertState("repo1", "src-a"); ok || err != nil {
		t.Fatalf("unrecorded source = ok:%v err:%v, want (false, nil)", ok, err)
	}

	if err := r.UpsertReceivedAlertState(store.ReceivedAlertState{ReceivedRepoID: "repo1", Source: "src-a", NotifiedAt: 100, BasedOn: 50}); err != nil {
		t.Fatal(err)
	}
	if err := r.UpsertReceivedAlertState(store.ReceivedAlertState{ReceivedRepoID: "repo1", Source: "src-b", NotifiedAt: 100, BasedOn: 60}); err != nil {
		t.Fatal(err)
	}
	got, ok, err := r.GetReceivedAlertState("repo1", "src-a")
	if err != nil || !ok {
		t.Fatalf("GetReceivedAlertState: ok=%v err=%v", ok, err)
	}
	if got.NotifiedAt != 100 || got.BasedOn != 50 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}

	// Refresh in place (a newer episode for the same source).
	if err := r.UpsertReceivedAlertState(store.ReceivedAlertState{ReceivedRepoID: "repo1", Source: "src-a", NotifiedAt: 200, BasedOn: 150}); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := r.GetReceivedAlertState("repo1", "src-a"); got.BasedOn != 150 || got.NotifiedAt != 200 {
		t.Fatalf("upsert did not refresh in place: %+v", got)
	}

	// Delete a single source leaves the other.
	if err := r.DeleteReceivedAlertState("repo1", "src-a"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := r.GetReceivedAlertState("repo1", "src-a"); ok {
		t.Fatal("src-a still present after single delete")
	}
	if _, ok, _ := r.GetReceivedAlertState("repo1", "src-b"); !ok {
		t.Fatal("src-b must survive a src-a delete")
	}

	// Per-repo purge clears everything for the repo (the DELETE-repo path).
	if err := r.UpsertReceivedAlertState(store.ReceivedAlertState{ReceivedRepoID: "repo1", Source: "src-a", NotifiedAt: 1, BasedOn: 1}); err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteReceivedAlertStatesForRepo("repo1"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := r.GetReceivedAlertState("repo1", "src-a"); ok {
		t.Fatal("per-repo purge left src-a")
	}
	if _, ok, _ := r.GetReceivedAlertState("repo1", "src-b"); ok {
		t.Fatal("per-repo purge left src-b")
	}
}

// TestUpdateReceivedRepoCheckResult pins the focused check-result writer: it
// updates ONLY the last-check columns (name/repo/cadence untouched) and records a
// definite verdict as a non-NULL last_check_ok.
func TestUpdateReceivedRepoCheckResult(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	created, err := r.CreateReceivedRepo(store.ReceivedRepo{Name: "A", Repo: "rest:https://box/vault", CheckCadence: "daily 04:00", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	// Record a FAILED deep check.
	if err := r.UpdateReceivedRepoCheckResult(created.ID, 999, sql.NullBool{Bool: false, Valid: true}, "boom", true); err != nil {
		t.Fatal(err)
	}
	got, _, err := r.GetReceivedRepo(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "A" || got.Repo != "rest:https://box/vault" || got.CheckCadence != "daily 04:00" {
		t.Fatalf("check-result writer clobbered config fields: %+v", got)
	}
	if !got.LastCheckOK.Valid || got.LastCheckOK.Bool || got.LastCheckAt != 999 ||
		got.LastCheckError != "boom" || !got.LastCheckReadData {
		t.Fatalf("check result not persisted: %+v", got)
	}

	// Record a subsequent PASSING structural check.
	if err := r.UpdateReceivedRepoCheckResult(created.ID, 1000, sql.NullBool{Bool: true, Valid: true}, "", false); err != nil {
		t.Fatal(err)
	}
	got2, _, _ := r.GetReceivedRepo(created.ID)
	if !got2.LastCheckOK.Valid || !got2.LastCheckOK.Bool || got2.LastCheckError != "" || got2.LastCheckReadData {
		t.Fatalf("second check result not persisted: %+v", got2)
	}
}
