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

// ---------------------------------------------------------------------------
// Fleet peer CRUD + manual poll endpoints
// ---------------------------------------------------------------------------
//
// This box watches a list of PEER BombVault instances here, read-only: their
// protection scorecard, nothing else. Every endpoint is under the existing
// authGate. The stored peer token is encrypted at rest (internal/secret) and
// NEVER returned in the clear — the view only reports whether one is stored
// (hasToken). Modeled closely on receiver_handlers.go.

// fleetPeerView is the JSON wire shape of a registered fleet peer's
// configuration + persisted last-poll status. It deliberately carries NO
// token (only hasToken) so the peer's secret never leaves the box.
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
	HasToken             bool                `json:"hasToken"` // a peer token is stored (the token itself is never returned)
}

// fleetPeerInput is the create/update request body. Token is the PEER's
// fleet_token; on update an empty Token keeps the stored one (so an edit need
// not resend the secret). Enabled is a pointer so its absence is
// distinguishable from an explicit false.
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
		_ = json.Unmarshal([]byte(p.LastPollDomainsJSON), &domains) // best-effort: a decode failure just shows no cached domains
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

// buildFleetPeer validates in and folds it onto existing (the zero value on
// create), returning the row to persist or a user-facing error message. The
// token is encrypted at rest with THIS instance's APP_KEY before it ever
// reaches the store; on update an empty Token preserves existing.TokenEnc.
// Identity and last-poll columns are carried from existing so an edit never
// re-stamps them.
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
		// Update with no new token: keep the stored ciphertext untouched.
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

// handleListFleetPeers lists every registered fleet peer with its persisted
// last-poll status (including the cached scorecard). GET /api/fleet/peers.
// Unlike the receiver dashboard's list endpoint, this does NOT live-probe
// every peer on each page load — a peer poll is a real network round-trip to
// another site, so freshness comes from the scheduled sweep (RunFleetPolls)
// and the explicit poll-now button, never an implicit one from just opening
// the page. Never returns a decrypted token.
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

// handleCreateFleetPeer registers a fleet peer. POST /api/fleet/peers. The
// peer must actually answer its /api/fleet/status with the given token before
// the row is saved — a mistyped URL/token never lands silently in the list —
// and the first poll's result is recorded immediately so the new row never
// shows a misleading "never polled" for a peer that IS reachable.
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

// handleUpdateFleetPeer edits a fleet peer in place. PUT /api/fleet/peers/{id}.
// The id comes from the path; identity + last-poll columns are preserved. As
// with create, the (possibly token-changed) peer must answer before the edit
// is saved.
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
		// No new token presented: probe with the stored one so an edit that only
		// changes e.g. the name still verifies the peer is reachable.
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

// handleDeleteFleetPeer removes a fleet peer's DB row. DELETE
// /api/fleet/peers/{id}. It NEVER contacts the peer (this box only ever
// polled it read-only). A missing id is a harmless no-op.
func (h *Handler) handleDeleteFleetPeer(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteFleetPeer(r.PathValue("id")); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleFleetPeerPoll polls one peer now and returns the fresh persisted
// result. POST /api/fleet/peers/{id}/poll.
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

// refetchedOrStored re-reads a fleet peer by id so the response reflects the
// just-recorded poll result; on a read error it falls back to fallback (the
// pre-poll row) rather than failing the whole request over a cosmetic re-read.
func refetchedOrStored(h *Handler, fallback store.FleetPeer) store.FleetPeer {
	if fresh, ok, err := h.store.GetFleetPeer(fallback.ID); err == nil && ok {
		return fresh
	}
	return fallback
}

// handleFleetTokenGenerate handles POST /api/fleet/token — generates a fresh
// random 32-hex token, stores it (replacing any previous one — regenerate ==
// revoke old + issue new) and returns it ONCE. Every peer instance that had
// this instance configured with the OLD token must be updated with the new
// one, exactly like rotating a widget token breaks existing embeds. Session-
// protected via authGate: only a logged-in admin can mint or rotate it.
func (h *Handler) handleFleetTokenGenerate(w http.ResponseWriter, _ *http.Request) {
	s, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	token := hex.EncodeToString(buf)
	s.FleetToken = token
	if err := h.store.UpdateSettings(s); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"token": token}))
}

// handleFleetTokenDisable handles DELETE /api/fleet/token — clears the stored
// token, so GET /api/fleet/status immediately fails closed (403) again for
// every peer polling this instance. Session-protected like the generate
// endpoint.
func (h *Handler) handleFleetTokenDisable(w http.ResponseWriter, _ *http.Request) {
	s, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	s.FleetToken = ""
	if err := h.store.UpdateSettings(s); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}
