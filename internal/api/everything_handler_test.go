package api_test

import (
	"net/http"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/spike"
)

// The fake engine's block channel holds the first pass inside the containers
// backup, so it is still running when the second POST arrives.
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
