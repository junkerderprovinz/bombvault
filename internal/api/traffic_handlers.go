package api

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// streamingView is the "Streaming first" card. Saved with MediaServersAuto,
// the media servers go back to being chosen by image name.
type streamingView struct {
	Enabled          bool     `json:"enabled"`
	MediaServers     []string `json:"mediaServers"`
	MediaServersAuto bool     `json:"mediaServersAuto"`
	ThresholdMbit    int      `json:"thresholdMbit"`
	LimitKiB         int      `json:"limitKiB"`
	HoldMin          int      `json:"holdMin"`
}

// mediaCandidate is a container the card offers as a media server.
type mediaCandidate struct {
	Name  string `json:"name"`
	Image string `json:"image"`
	// HostNetwork marks a container whose traffic Docker cannot count.
	HostNetwork bool `json:"hostNetwork"`
}

// streamingToView is the stored card as a settings export carries it: the
// chosen media servers, or none while the image names decide.
func streamingToView(cfg store.TrafficSettings) *streamingView {
	return &streamingView{
		Enabled:          cfg.StreamThrottle,
		MediaServers:     cfg.MediaServers,
		MediaServersAuto: cfg.MediaServers == nil,
		ThresholdMbit:    cfg.StreamMbit,
		LimitKiB:         cfg.StreamLimitKiB,
		HoldMin:          cfg.StreamHoldMin,
	}
}

func applyStreamingView(cfg store.TrafficSettings, v streamingView) store.TrafficSettings {
	cfg.StreamThrottle = v.Enabled
	cfg.MediaServers = v.MediaServers
	if v.MediaServersAuto {
		cfg.MediaServers = nil
	}
	cfg.StreamMbit = v.ThresholdMbit
	cfg.StreamLimitKiB = v.LimitKiB
	cfg.StreamHoldMin = v.HoldMin
	return cfg
}

func validateStreaming(v streamingView) error {
	switch {
	case v.ThresholdMbit < 1 || v.ThresholdMbit > 10000:
		return errors.New("the streaming threshold has to be between 1 and 10000 Mbit/s")
	case v.LimitKiB < 1 || v.LimitKiB > 10_000_000:
		return errors.New("the upload limit while streaming has to be between 1 and 10000000 KiB/s")
	case v.HoldMin < 1 || v.HoldMin > 120:
		return errors.New("the wait after a stream has to be between 1 and 120 minutes")
	case len(v.MediaServers) > 200:
		return errors.New("too many media servers")
	}
	for _, n := range v.MediaServers {
		if strings.TrimSpace(n) == "" || len(n) > 255 {
			return errors.New("a media server needs a container name")
		}
	}
	return nil
}

// GET /api/settings/streaming
func (h *Handler) handleGetStreaming(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.store.TrafficSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	infos, err := h.svc.docker.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	self := h.svc.selfContainerName(r.Context())
	candidates := make([]mediaCandidate, 0, len(infos))
	for _, c := range infos {
		if c.Name == self {
			continue
		}
		candidates = append(candidates, mediaCandidate{Name: c.Name, Image: c.Image, HostNetwork: c.NetworkMode == "host"})
	}
	slices.SortFunc(candidates, func(a, b mediaCandidate) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	servers := h.svc.mediaServers(r.Context(), cfg, time.Now())
	if servers == nil {
		servers = []string{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"settings": streamingView{
			Enabled:          cfg.StreamThrottle,
			MediaServers:     servers,
			MediaServersAuto: cfg.MediaServers == nil,
			ThresholdMbit:    cfg.StreamMbit,
			LimitKiB:         cfg.StreamLimitKiB,
			HoldMin:          cfg.StreamHoldMin,
		},
		"candidates": candidates,
		"streaming":  h.svc.streamingServer(),
	}))
}

// PUT /api/settings/streaming
func (h *Handler) handleSetStreaming(w http.ResponseWriter, r *http.Request) {
	var v streamingView
	if !decodeBody(w, r, &v) {
		return
	}
	if err := validateStreaming(v); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	cfg, err := h.store.TrafficSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if err := h.store.SetTrafficSettings(applyStreamingView(cfg, v)); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}
