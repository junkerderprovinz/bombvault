package api_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/spike"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func selectionRouter(t *testing.T, eng *fakeResticEngine) (http.Handler, *api.Service, *store.Repo, store.Target) {
	t.Helper()
	svc, st, docker, tg := everythingTestService(t, eng)
	sched := schedule.New(func(string) error { return nil }, st.ListTargetsScheduleOrder)
	return api.NewHandler(config.Config{}, st, docker, svc, sched, spike.DefaultProbes()).Router(), svc, st, tg
}

func waitForRun(t *testing.T, st *store.Repo, targetID string) *store.Run {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if run, err := st.LastRunForTarget(targetID); err == nil && run != nil && run.Status != "running" {
			return run
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no finished run for %s", targetID)
	return nil
}

// The server runs the kinds of a selection one after the other, with nothing
// from the page that started it.
func TestBackupSelectionRunsEveryKindOnTheServer(t *testing.T) {
	router, svc, st, tg := selectionRouter(t, &fakeResticEngine{})

	w, m := doJSON(t, router, http.MethodPost, "/api/backup/selection",
		`{"parts":[{"kind":"container","keys":["primary"]},{"kind":"config","keys":["config"]}]}`)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
	for _, id := range []string{tg.ID, store.ConfigTargetID} {
		if run := waitForRun(t, st, id); run.Status != "success" {
			t.Fatalf("run of %s = %+v, want success", id, run)
		}
	}
	for svc.BackupInProgress() {
		time.Sleep(10 * time.Millisecond)
	}
}

// A flash part with nowhere to go is named with its code, and the rest of the
// selection still runs.
func TestBackupSelectionNamesAFlashPartWithNoPlace(t *testing.T) {
	router, svc, st, tg := selectionRouter(t, &fakeResticEngine{})
	if err := st.SetLocal("flash", false); err != nil {
		t.Fatal(err)
	}

	w, m := doJSON(t, router, http.MethodPost, "/api/backup/selection",
		`{"parts":[{"kind":"flash","keys":["flash"]},{"kind":"container","keys":["primary"]}]}`)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
	refused, _ := m["refused"].([]any)
	if len(refused) != 1 || refused[0].(map[string]any)["kind"] != "flash" || refused[0].(map[string]any)["code"] != "no-place" {
		t.Fatalf("refused = %v, want flash with no-place", m["refused"])
	}
	if run := waitForRun(t, st, tg.ID); run.Status != "success" {
		t.Fatalf("container run = %+v, want success", run)
	}
	for svc.BackupInProgress() {
		time.Sleep(10 * time.Millisecond)
	}
}

// A second selection while one runs is refused as busy.
func TestBackupSelectionRefusesASecondOne(t *testing.T) {
	eng := &fakeResticEngine{block: make(chan struct{})}
	router, svc, st, _ := selectionRouter(t, eng)
	body := `{"parts":[{"kind":"container","keys":["primary"]},{"kind":"config","keys":["config"]}]}`

	if w, _ := doJSON(t, router, http.MethodPost, "/api/backup/selection", body); w.Code != http.StatusOK {
		t.Fatalf("first status %d, body %s", w.Code, w.Body.String())
	}
	w, m := doJSON(t, router, http.MethodPost, "/api/backup/selection", body)
	if w.Code != http.StatusConflict || m["code"] != nil {
		t.Fatalf("second status %d, body %s, want an uncoded 409", w.Code, w.Body.String())
	}
	close(eng.block)
	waitForRun(t, st, store.ConfigTargetID)
	for svc.BackupInProgress() {
		time.Sleep(10 * time.Millisecond)
	}
}
