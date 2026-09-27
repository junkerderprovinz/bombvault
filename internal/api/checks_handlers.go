package api

import (
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
	out := []itemChecks{}
	for id, ref := range refs {
		row := itemChecks{TargetID: id, Domain: ref.Domain, Name: ref.Name}
		if p, ok := probes[id]; ok {
			row.Probe = &p
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
