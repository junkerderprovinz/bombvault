package store_test

import (
	"database/sql"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestReceivedAlertStateCRUD expects an unrecorded source to read as (false,
// nil), an upsert on the same key to refresh in place, delete to remove one
// source, and the per-repo purge used when a repo is deleted to clear all of
// them.
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

	// A newer episode for the same source.
	if err := r.UpsertReceivedAlertState(store.ReceivedAlertState{ReceivedRepoID: "repo1", Source: "src-a", NotifiedAt: 200, BasedOn: 150}); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := r.GetReceivedAlertState("repo1", "src-a"); got.BasedOn != 150 || got.NotifiedAt != 200 {
		t.Fatalf("upsert did not refresh in place: %+v", got)
	}

	if err := r.DeleteReceivedAlertState("repo1", "src-a"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := r.GetReceivedAlertState("repo1", "src-a"); ok {
		t.Fatal("src-a still present after single delete")
	}
	if _, ok, _ := r.GetReceivedAlertState("repo1", "src-b"); !ok {
		t.Fatal("src-b must survive a src-a delete")
	}

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

// TestUpdateReceivedRepoCheckResult checks that the check-result writer touches
// just the last-check columns and stores a verdict as a non-NULL last_check_ok.
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

	// A failed deep check.
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

	// Then a passing structural check.
	if err := r.UpdateReceivedRepoCheckResult(created.ID, 1000, sql.NullBool{Bool: true, Valid: true}, "", false); err != nil {
		t.Fatal(err)
	}
	got2, _, _ := r.GetReceivedRepo(created.ID)
	if !got2.LastCheckOK.Valid || !got2.LastCheckOK.Bool || got2.LastCheckError != "" || got2.LastCheckReadData {
		t.Fatalf("second check result not persisted: %+v", got2)
	}
}
