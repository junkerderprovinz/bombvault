package api

import (
	"encoding/json"
	"net/http"

	"github.com/junkerderprovinz/bombvault/internal/group"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// fleetPeerView is a Fleet row with its last poll result and how the member
// is reachable right now.
type fleetPeerView struct {
	ID                   string              `json:"id"`
	MemberID             string              `json:"memberId"`
	Name                 string              `json:"name"`
	URL                  string              `json:"url"`
	Enabled              bool                `json:"enabled"`
	NeedsPairing         bool                `json:"needsPairing"`
	Direct               bool                `json:"direct"`
	Relay                bool                `json:"relay"`
	LastPollAt           int64               `json:"lastPollAt"`
	LastPollOK           *bool               `json:"lastPollOk"` // null = never polled
	LastPollError        string              `json:"lastPollError"`
	LastPollInstanceName string              `json:"lastPollInstanceName"`
	LastPollVersion      string              `json:"lastPollVersion"`
	LastPollDomains      []DomainStatusEntry `json:"lastPollDomains"`
	// RemoteViewEnabled is whether the peer's remote-view switch was on as of
	// the last poll. The Fleet card's Open button reads this rather than
	// calling out just to find out.
	RemoteViewEnabled bool  `json:"remoteViewEnabled"`
	CreatedAt         int64 `json:"createdAt"`
	SortOrder         int   `json:"sortOrder"`
}

// fleetPeerInput is the update request body. The pointers tell a field left
// out from one sent as its zero value.
type fleetPeerInput struct {
	Enabled   *bool `json:"enabled"`
	SortOrder *int  `json:"sortOrder"`
}

func fleetPeerToView(p store.FleetPeer, members []group.Member) fleetPeerView {
	var ok *bool
	if p.LastPollOK.Valid {
		v := p.LastPollOK.Bool
		ok = &v
	}
	var domains []DomainStatusEntry
	if p.LastPollDomainsJSON != "" {
		_ = json.Unmarshal([]byte(p.LastPollDomainsJSON), &domains) // on failure the view shows no cached domains
	}
	v := fleetPeerView{
		ID:                   p.ID,
		MemberID:             p.MemberID,
		Name:                 p.Name,
		URL:                  p.URL,
		Enabled:              p.Enabled,
		NeedsPairing:         p.NeedsPairing(),
		LastPollAt:           p.LastPollAt,
		LastPollOK:           ok,
		LastPollError:        p.LastPollError,
		LastPollInstanceName: p.LastPollInstanceName,
		LastPollVersion:      p.LastPollVersion,
		LastPollDomains:      domains,
		RemoteViewEnabled:    p.LastPollRemoteViewEnabled,
		CreatedAt:            p.CreatedAt,
		SortOrder:            p.SortOrder,
	}
	for _, m := range members {
		if m.ID == p.MemberID && p.MemberID != "" {
			v.Direct, v.Relay = m.Direct, m.Relay
		}
	}
	return v
}

// handleListFleetPeers lists every member with its last stored poll result,
// plus rows from before pairing. It does not poll: fresh results come from
// the scheduled sweep and the poll-now button. GET /api/fleet/peers
func (h *Handler) handleListFleetPeers(w http.ResponseWriter, _ *http.Request) {
	peers, err := h.svc.syncFleetPeers()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	members := h.svc.pairing().Members()
	out := make([]fleetPeerView, 0, len(peers))
	for _, p := range peers {
		out = append(out, fleetPeerToView(p, members))
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"peers": out}))
}

// lookupFleetPeer loads the row named in the path or writes the error.
func (h *Handler) lookupFleetPeer(w http.ResponseWriter, r *http.Request) (store.FleetPeer, bool) {
	p, ok, err := h.store.GetFleetPeer(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return store.FleetPeer{}, false
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no such fleet peer"})
		return store.FleetPeer{}, false
	}
	return p, true
}

// handleUpdateFleetPeer switches a row's polling on or off and moves it.
// PUT /api/fleet/peers/{id}
func (h *Handler) handleUpdateFleetPeer(w http.ResponseWriter, r *http.Request) {
	p, ok := h.lookupFleetPeer(w, r)
	if !ok {
		return
	}
	var in fleetPeerInput
	if !decodeBody(w, r, &in) {
		return
	}
	if in.Enabled != nil {
		p.Enabled = *in.Enabled
	}
	if in.SortOrder != nil {
		p.SortOrder = *in.SortOrder
	}
	if err := h.store.UpdateFleetPeer(p); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"peer": fleetPeerToView(p, h.svc.pairing().Members())}))
}

// handleDeleteFleetPeer removes a row without contacting the member. A
// member still in the group comes back on the next list. An unknown id is not
// an error. DELETE /api/fleet/peers/{id}
func (h *Handler) handleDeleteFleetPeer(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteFleetPeer(r.PathValue("id")); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleFleetPeerPoll polls one member now and returns the recorded result.
// POST /api/fleet/peers/{id}/poll
func (h *Handler) handleFleetPeerPoll(w http.ResponseWriter, r *http.Request) {
	p, ok := h.lookupFleetPeer(w, r)
	if !ok {
		return
	}
	_, _ = h.svc.pollAndRecordFleetPeer(r.Context(), p) //nolint:errcheck,gosec // the persisted result is what the response reports
	fresh, ok, err := h.store.GetFleetPeer(p.ID)
	if err != nil || !ok {
		fresh = p
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"peer": fleetPeerToView(fresh, h.svc.pairing().Members())}))
}

// handleFleetPeerCheck asks a member to check one domain's repository now.
// The member answers once the check has started; its result shows in the
// member's next scorecard. POST /api/fleet/peers/{id}/check/{domain}
func (h *Handler) handleFleetPeerCheck(w http.ResponseWriter, r *http.Request) {
	p, ok := h.lookupFleetPeer(w, r)
	if !ok {
		return
	}
	domain := r.PathValue("domain")
	if !checkableDomain(domain) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown domain"})
		return
	}
	if p.NeedsPairing() {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "pair this instance again first"})
		return
	}
	if err := h.svc.callMember(r.Context(), p.MemberID, http.MethodPost, "/api/group/peer/check/"+domain, nil, nil); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// checkableDomain reports whether a check can run on domain, the same set
// POST /api/check/{domain} accepts.
func checkableDomain(domain string) bool {
	switch domain {
	case "containers", "vms", "flash", "files", "zfs":
		return true
	}
	return false
}
