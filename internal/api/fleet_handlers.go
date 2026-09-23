package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// fleetPeerView is a registered fleet peer with its last poll result. It
// reports only whether a token is stored, never the token.
type fleetPeerView struct {
	ID                   string              `json:"id"`
	Name                 string              `json:"name"`
	URL                  string              `json:"url"`
	Enabled              bool                `json:"enabled"`
	LastPollAt           int64               `json:"lastPollAt"`
	LastPollOK           *bool               `json:"lastPollOk"` // null = never polled
	LastPollError        string              `json:"lastPollError"`
	LastPollInstanceName string              `json:"lastPollInstanceName"`
	LastPollVersion      string              `json:"lastPollVersion"`
	LastPollDomains      []DomainStatusEntry `json:"lastPollDomains"`
	CreatedAt            int64               `json:"createdAt"`
	SortOrder            int                 `json:"sortOrder"`
	HasToken             bool                `json:"hasToken"`
}

// fleetPeerInput is the create and update request body. Token is the peer's
// fleet token; on update an empty Token keeps the stored one. Enabled is a
// pointer so that leaving it out differs from sending false.
type fleetPeerInput struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Token     string `json:"token"`
	Enabled   *bool  `json:"enabled"`
	SortOrder int    `json:"sortOrder"`
}

func fleetPeerToView(p store.FleetPeer) fleetPeerView {
	var ok *bool
	if p.LastPollOK.Valid {
		v := p.LastPollOK.Bool
		ok = &v
	}
	var domains []DomainStatusEntry
	if p.LastPollDomainsJSON != "" {
		_ = json.Unmarshal([]byte(p.LastPollDomainsJSON), &domains) // on failure the view shows no cached domains
	}
	return fleetPeerView{
		ID:                   p.ID,
		Name:                 p.Name,
		URL:                  p.URL,
		Enabled:              p.Enabled,
		LastPollAt:           p.LastPollAt,
		LastPollOK:           ok,
		LastPollError:        p.LastPollError,
		LastPollInstanceName: p.LastPollInstanceName,
		LastPollVersion:      p.LastPollVersion,
		LastPollDomains:      domains,
		CreatedAt:            p.CreatedAt,
		SortOrder:            p.SortOrder,
		HasToken:             len(p.TokenEnc) > 0,
	}
}

// buildFleetPeer validates in and applies it to existing (the zero value on
// create). It returns the row to store, with the token encrypted under this
// instance's app key, or a message for the user. Identity and last-poll
// fields are kept from existing.
func buildFleetPeer(cfgAppKey string, in fleetPeerInput, existing store.FleetPeer, isCreate bool) (store.FleetPeer, string) {
	peerURL := strings.TrimSpace(in.URL)
	if peerURL == "" {
		return store.FleetPeer{}, "peer URL must not be empty"
	}

	p := existing
	p.Name = strings.TrimSpace(in.Name)
	p.URL = peerURL

	token := strings.TrimSpace(in.Token)
	switch {
	case token == "" && isCreate:
		return store.FleetPeer{}, "the peer's fleet token is required (generated on that instance's Settings page)"
	case token == "":
		p.TokenEnc = existing.TokenEnc
	default:
		enc, err := secret.Encrypt(cfgAppKey, []byte(token))
		if err != nil {
			return store.FleetPeer{}, "could not encrypt the peer token"
		}
		p.TokenEnc = enc
	}

	p.SortOrder = in.SortOrder
	if in.Enabled != nil {
		p.Enabled = *in.Enabled
	} else if isCreate {
		p.Enabled = true
	}
	return p, ""
}

// handleListFleetPeers lists the fleet peers with their last stored poll
// result. It does not poll them: a poll is a round-trip to another site, so
// fresh results come from the scheduled sweep and the poll-now button.
// GET /api/fleet/peers
func (h *Handler) handleListFleetPeers(w http.ResponseWriter, _ *http.Request) {
	peers, err := h.store.ListFleetPeers()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	out := make([]fleetPeerView, 0, len(peers))
	for _, p := range peers {
		out = append(out, fleetPeerToView(p))
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"peers": out}))
}

// handleCreateFleetPeer registers a fleet peer. The peer must answer with the
// given token before it is saved, so a mistyped URL or token is caught here,
// and the first poll is recorded right away.
// POST /api/fleet/peers
func (h *Handler) handleCreateFleetPeer(w http.ResponseWriter, r *http.Request) {
	var in fleetPeerInput
	if !decodeBody(w, r, &in) {
		return
	}
	p, msg := buildFleetPeer(h.cfg.AppKey, in, store.FleetPeer{}, true)
	if msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}
	if _, err := pollFleetPeer(r.Context(), p.URL, strings.TrimSpace(in.Token)); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	stored, err := h.store.CreateFleetPeer(p)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	_, _ = h.svc.pollAndRecordFleetPeer(r.Context(), stored) //nolint:errcheck,gosec // the freshly-recorded row is what the response reports, not this call's own error
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"peer": fleetPeerToView(refetchedOrStored(h, stored))}))
}

// handleUpdateFleetPeer edits a fleet peer. As on create, the peer must answer
// before the edit is saved.
// PUT /api/fleet/peers/{id}
func (h *Handler) handleUpdateFleetPeer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, ok, err := h.store.GetFleetPeer(id)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no such fleet peer"})
		return
	}
	var in fleetPeerInput
	if !decodeBody(w, r, &in) {
		return
	}
	p, msg := buildFleetPeer(h.cfg.AppKey, in, existing, false)
	if msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}
	token := strings.TrimSpace(in.Token)
	if token == "" {
		if dec, dErr := h.svc.decryptFleetPeerToken(p); dErr == nil {
			token = dec
		}
	}
	if _, err := pollFleetPeer(r.Context(), p.URL, token); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if err := h.store.UpdateFleetPeer(p); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	_, _ = h.svc.pollAndRecordFleetPeer(r.Context(), p) //nolint:errcheck,gosec // the freshly-recorded row is what the response reports, not this call's own error
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"peer": fleetPeerToView(refetchedOrStored(h, p))}))
}

// handleDeleteFleetPeer removes a fleet peer without contacting it. An
// unknown id is not an error.
// DELETE /api/fleet/peers/{id}
func (h *Handler) handleDeleteFleetPeer(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteFleetPeer(r.PathValue("id")); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleFleetPeerPoll polls one peer now and returns the recorded result.
// POST /api/fleet/peers/{id}/poll
func (h *Handler) handleFleetPeerPoll(w http.ResponseWriter, r *http.Request) {
	p, ok, err := h.store.GetFleetPeer(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no such fleet peer"})
		return
	}
	_, _ = h.svc.pollAndRecordFleetPeer(r.Context(), p) //nolint:errcheck,gosec // the persisted result is what the response reports, not this call's own error
	fresh := refetchedOrStored(h, p)
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"peer": fleetPeerToView(fresh)}))
}

// refetchedOrStored re-reads a fleet peer so the response shows the poll just
// recorded. If the read fails it returns fallback instead.
func refetchedOrStored(h *Handler, fallback store.FleetPeer) store.FleetPeer {
	if fresh, ok, err := h.store.GetFleetPeer(fallback.ID); err == nil && ok {
		return fresh
	}
	return fallback
}

// handleFleetTokenGenerate creates a random token for peers to poll this
// instance with, replaces any previous one and returns it once. Peers that
// used the old token have to be given the new one.
// POST /api/fleet/token
func (h *Handler) handleFleetTokenGenerate(w http.ResponseWriter, _ *http.Request) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	token := hex.EncodeToString(buf)
	if _, err := h.store.MutateSettings(func(s *store.Settings) error {
		s.FleetToken = token
		return nil
	}); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"token": token}))
}

// handleFleetTokenDisable clears the fleet token, so every peer polling this
// instance gets 403 from then on.
// DELETE /api/fleet/token
func (h *Handler) handleFleetTokenDisable(w http.ResponseWriter, _ *http.Request) {
	if _, err := h.store.MutateSettings(func(s *store.Settings) error {
		s.FleetToken = ""
		return nil
	}); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}
