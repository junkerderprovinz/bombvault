package api

import (
	"net/http"

	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
)

// activityRuns is how many recent runs a member gets, enough for the tail of
// the activity log the Android app shows.
const activityRuns = 50

// peerActivityResponse is what GET /api/group/peer/activity answers: the
// three inputs of the dashboard's activity log, in the shapes of
// GET /api/runs, GET /api/progress and GET /api/schedule/next.
type peerActivityResponse struct {
	OK       bool               `json:"ok"`
	Runs     []runView          `json:"runs"`
	Progress []progress.Event   `json:"progress"`
	Next     []schedule.NextRun `json:"next"`
}

// activity reads what peerActivityResponse carries. Runs and the scheduler
// belong to the Handler, so the Service's peer route asks through this.
func (h *Handler) activity() (peerActivityResponse, error) {
	runs, err := h.store.ListRuns(activityRuns)
	if err != nil {
		return peerActivityResponse{}, err
	}
	resp := peerActivityResponse{OK: true, Runs: h.runViews(runs), Progress: []progress.Event{}, Next: []schedule.NextRun{}}
	if h.progress != nil {
		resp.Progress = h.progress.Snapshot()
	}
	if h.scheduler != nil {
		resp.Next = h.scheduler.NextRuns()
	}
	return resp, nil
}

func (s *Service) handlePeerActivity(w http.ResponseWriter, _ *http.Request) {
	if s.peerActivity == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no activity here"})
		return
	}
	resp, err := s.peerActivity()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
