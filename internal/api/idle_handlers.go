package api

import (
	"errors"
	"net/http"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// maxIdleWaitHours is the longest a scheduled backup may wait for its app.
const maxIdleWaitHours = 24

func setIdleWait(st *store.Repo, name string, hours int) error {
	if hours < 0 || hours > maxIdleWaitHours {
		return errors.New("the wait for an idle app can be 24 hours at most")
	}
	return st.SetContainerIdleWait(name, hours)
}

// idleView is the card that says when an app counts as idle.
type idleView struct {
	CPUPct   int `json:"cpuPct"`
	NetMbit  int `json:"netMbit"`
	QuietMin int `json:"quietMin"`
}

func validateIdle(v idleView) error {
	switch {
	case v.CPUPct < 1 || v.CPUPct > 3200:
		return errors.New("the CPU limit for an idle app has to be between 1 and 3200 percent")
	case v.NetMbit < 1 || v.NetMbit > 10000:
		return errors.New("the traffic limit for an idle app has to be between 1 and 10000 Mbit/s")
	case v.QuietMin < 1 || v.QuietMin > 60:
		return errors.New("the quiet time for an idle app has to be between 1 and 60 minutes")
	}
	return nil
}

func idleToView(cfg store.TrafficSettings) *idleView {
	return &idleView{CPUPct: cfg.IdleCPUPct, NetMbit: cfg.IdleNetMbit, QuietMin: cfg.IdleQuietMin}
}

func applyIdleView(cfg store.TrafficSettings, v idleView) store.TrafficSettings {
	cfg.IdleCPUPct, cfg.IdleNetMbit, cfg.IdleQuietMin = v.CPUPct, v.NetMbit, v.QuietMin
	return cfg
}

// GET /api/settings/idle
func (h *Handler) handleGetIdle(w http.ResponseWriter, _ *http.Request) {
	cfg, err := h.store.TrafficSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"settings": idleToView(cfg)}))
}

// PUT /api/settings/idle
func (h *Handler) handleSetIdle(w http.ResponseWriter, r *http.Request) {
	var v idleView
	if !decodeBody(w, r, &v) {
		return
	}
	if err := validateIdle(v); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	cfg, err := h.store.TrafficSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if err := h.store.SetTrafficSettings(applyIdleView(cfg, v)); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// GET /api/schedule/waiting lists the scheduled backups waiting for an idle
// app.
func (h *Handler) handleScheduleWaiting(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"waiting": h.svc.IdleWaits()}))
}
