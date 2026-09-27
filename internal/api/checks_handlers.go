package api

import (
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// itemChecks is what the item cards show about the checks of one item. Domain
// and name are the ones the anomaly list uses, so a card finds its row the
// same way it finds its findings.
type itemChecks struct {
	TargetID string           `json:"targetId"`
	Domain   string           `json:"domain"`
	Name     string           `json:"name"`
	Probe    *store.ItemProbe `json:"probe,omitempty"`
	// StartTest and StartTestBlocked are for containers only: the newest
	// start test, and why the container cannot be tested when it cannot.
	StartTest        *store.StartTest `json:"startTest,omitempty"`
	StartTestBlocked string           `json:"startTestBlocked,omitempty"`
}

// ItemChecks returns every item with the newest results of its checks.
func (s *Service) ItemChecks() ([]itemChecks, error) {
	refs, err := s.anomalyItemRefs()
	if err != nil {
		return nil, err
	}
	probes, err := s.store.LatestItemProbes()
	if err != nil {
		return nil, err
	}
	tests, err := s.store.LatestStartTests()
	if err != nil {
		return nil, err
	}
	targets, err := s.store.ListTargets()
	if err != nil {
		return nil, err
	}
	blocked := make(map[string]string, len(targets))
	for _, tg := range targets {
		_, blocked[tg.ID] = startTestRecipe(tg)
	}
	out := []itemChecks{}
	for id, ref := range refs {
		row := itemChecks{TargetID: id, Domain: ref.Domain, Name: ref.Name, StartTestBlocked: blocked[id]}
		if p, ok := probes[id]; ok {
			row.Probe = &p
		}
		if t, ok := tests[id]; ok {
			row.StartTest = &t
		}
		out = append(out, row)
	}
	slices.SortFunc(out, func(a, b itemChecks) int { return strings.Compare(a.TargetID, b.TargetID) })
	return out, nil
}

// handleItemChecks answers GET /api/checks/items.
func (h *Handler) handleItemChecks(w http.ResponseWriter, _ *http.Request) {
	items, err := h.svc.ItemChecks()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"items": items}))
}

// handleProbeItem answers POST /api/checks/probe/{id}: a restore probe of the
// item's newest backup, run now.
func (h *Handler) handleProbeItem(w http.ResponseWriter, r *http.Request) {
	probe, err := h.svc.ProbeItem(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"probe": probe}))
}

// handleStartTest answers POST /api/checks/starttest/{id}: a start test of the
// container, run now.
func (h *Handler) handleStartTest(w http.ResponseWriter, r *http.Request) {
	test, err := h.svc.RunStartTest(r.Context(), r.PathValue("id"))
	if err != nil {
		body := failEnvelope(err)
		var blocked startTestBlocked
		if errors.As(err, &blocked) {
			body["blocked"] = blocked.Code
		}
		writeJSON(w, http.StatusOK, body)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"startTest": test}))
}
