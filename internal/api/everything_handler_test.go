package api_test

// Handler-level test for Task 5 of the "Backup Everything" plan
// (docs/superpowers/plans/2026-08-20-backup-everything.md): POST
// /api/backup-everything (internal/api/handlers.go's handleBackupEverything).
// Reuses everything_test.go's Task-4 harness (everythingTestService,
// waitForEverythingDone) so the concurrency guard is exercised through the
// SAME synchronization technique already used to test StartBackupEverything's
// re-entrancy contract directly (TestStartBackupEverythingRefusesConcurrent).

import (
	"net/http"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/spike"
)

// TestHandleBackupEverythingStartsAndRefusesConcurrent: the first POST starts
// the pass and returns {ok:true, started:true}; a second POST while the first
// pass is still in flight is refused with a 409 {ok:false} (handleBackupEverything
// mirrors handleBackupAll's exact response-shape/status-code convention). The
// fake engine's block channel holds the first pass inside the containers
// domain's real Restic.Backup call (see everythingTestService,
// everything_test.go: the "primary" target has a genuine, existing
// SelectedPaths folder), so the pass is deterministically still running when
// the second request is made.
func TestHandleBackupEverythingStartsAndRefusesConcurrent(t *testing.T) {
	eng := &fakeResticEngine{block: make(chan struct{})}
	svc, st, docker, _ := everythingTestService(t, eng)

	sched := schedule.New(func(string) error { return nil }, st.ListTargetsScheduleOrder)
	h := api.NewHandler(config.Config{}, st, docker, svc, sched, spike.DefaultProbes())
	router := h.Router()

	w, m := doJSON(t, router, http.MethodPost, "/api/backup-everything", "")
	if w.Code != http.StatusOK {
		t.Fatalf("first call status = %d, body=%s", w.Code, w.Body.String())
	}
	if m["ok"] != true || m["started"] != true {
		t.Fatalf("first call envelope = %v, want ok:true started:true", m)
	}

	w2, m2 := doJSON(t, router, http.MethodPost, "/api/backup-everything", "")
	if w2.Code != http.StatusConflict {
		t.Fatalf("second concurrent call status = %d, want %d, body=%s", w2.Code, http.StatusConflict, w2.Body.String())
	}
	if m2["ok"] != false {
		t.Fatalf("second concurrent call envelope = %v, want ok:false", m2)
	}

	close(eng.block) // let the first pass finish, then wait so cleanup is race-free
	waitForEverythingDone(t, svc)
}
