package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// newMeshCredSetID mints a fresh id for a newly accepted mesh offer's
// credential set. CloudCredSet.ID has no store-level generator
// (SetCloudCredSets is a pure replace-the-whole-list call, unlike the other
// CRUD tables here, and the SPA normally mints one client-side via
// crypto.randomUUID() — see Settings.tsx); this mirrors store's own newID()
// shape (16 random bytes, hex) since this call happens server-side.
func newMeshCredSetID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing is a fatal platform problem, not a recoverable
		// input error — matches store.newID()'s own panic-on-failure contract.
		panic(fmt.Sprintf("newMeshCredSetID: %v", err))
	}
	return hex.EncodeToString(b)
}

// ---------------------------------------------------------------------------
// Mesh off-site — a fleet peer OFFERS its own off-site storage (a rest-server
// it deploys itself) instead of the two admins exchanging a URL and password
// out of band. BombVault never hosts storage itself (see deploy.go); this only
// automates handing the connection details from the offering instance to the
// accepting instance's admin over the already-authenticated fleet channel, who
// then reviews and turns it into a normal named CloudCredSet + OffsiteTarget —
// both pre-existing mechanisms, unchanged. No off-site data ever flows through
// this path, only connection metadata; the actual backup still replicates
// straight to the deployed rest-server exactly as it does today.
// ---------------------------------------------------------------------------

// meshOfferRequest is the JSON body POSTed to the accepting instance's
// self-gated GET /api/fleet/status sibling, POST /api/fleet/mesh-offer (same
// fleetGate trust boundary: anyone holding this instance's fleet token may
// send an offer, exactly as anyone holding it may poll the status endpoint).
type meshOfferRequest struct {
	FromName        string `json:"fromName"`
	SuggestedDomain string `json:"suggestedDomain"`
	Repo            string `json:"repo"`
	RESTUser        string `json:"restUser"`
	RESTPassword    string `json:"restPassword"`
}

// handleFleetMeshOfferReceive handles POST /api/fleet/mesh-offer — a peer
// proposing its own off-site storage to this instance. Self-gated exactly
// like handleFleetStatus (same token, same allowlist entry); persists a
// pending store.MeshOffer for a human to review under Settings → Fleet.
func (h *Handler) handleFleetMeshOfferReceive(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.fleetGate(w, r); !ok {
		return
	}
	body := io.LimitReader(r.Body, fleetResponseMax)
	var in meshOfferRequest
	if err := json.NewDecoder(body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "malformed mesh offer body"})
		return
	}
	repo := strings.TrimSpace(in.Repo)
	if repo == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "offer repo must not be empty"})
		return
	}
	enc, err := secret.Encrypt(h.cfg.AppKey, []byte(in.RESTPassword))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	stored, err := h.store.CreateMeshOffer(store.MeshOffer{
		From:            strings.TrimSpace(in.FromName),
		SuggestedDomain: strings.TrimSpace(in.SuggestedDomain),
		Repo:            repo,
		RESTUser:        strings.TrimSpace(in.RESTUser),
		RESTPasswordEnc: enc,
	})
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"id": stored.ID}))
}

// meshOfferView is the JSON wire shape of a received mesh offer. RESTPassword
// is never returned — the accept endpoint uses the stored ciphertext
// server-side, the admin never needs to see or retype it.
type meshOfferView struct {
	ID              string `json:"id"`
	From            string `json:"from"`
	SuggestedDomain string `json:"suggestedDomain"`
	Repo            string `json:"repo"`
	RESTUser        string `json:"restUser"`
	Status          string `json:"status"`
	ReceivedAt      int64  `json:"receivedAt"`
}

func meshOfferToView(o store.MeshOffer) meshOfferView {
	return meshOfferView{
		ID:              o.ID,
		From:            o.From,
		SuggestedDomain: o.SuggestedDomain,
		Repo:            o.Repo,
		RESTUser:        o.RESTUser,
		Status:          o.Status,
		ReceivedAt:      o.ReceivedAt,
	}
}

// handleListMeshOffers lists every received mesh offer (pending, accepted and
// declined — the SPA filters by status). GET /api/fleet/mesh-offers,
// session-protected.
func (h *Handler) handleListMeshOffers(w http.ResponseWriter, _ *http.Request) {
	offers, err := h.store.ListMeshOffers()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	out := make([]meshOfferView, 0, len(offers))
	for _, o := range offers {
		out = append(out, meshOfferToView(o))
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"offers": out}))
}

// meshOfferAcceptInput is the accept request body: which of THIS instance's
// domains the accepted offer's storage should back up.
type meshOfferAcceptInput struct {
	Domain string `json:"domain"`
}

// handleAcceptMeshOffer turns a pending offer into a real, working off-site
// target: a new named CloudCredSet (holding the peer-generated REST
// credentials) plus a new OffsiteTarget for the chosen domain pointing at the
// offer's repo via that credential set. POST
// /api/fleet/mesh-offers/{id}/accept, session-protected. Neither the
// credential set nor the target is probed for reachability before creation —
// same contract as creating either directly (a Test button exists for that).
func (h *Handler) handleAcceptMeshOffer(w http.ResponseWriter, r *http.Request) {
	offer, ok, err := h.store.GetMeshOffer(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no such mesh offer"})
		return
	}
	if offer.Status != "pending" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "offer is not pending"})
		return
	}
	var in meshOfferAcceptInput
	if !decodeBody(w, r, &in) {
		return
	}
	if !validOffsiteDomain(in.Domain) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "invalid domain — must be one of containers, vms, flash, config, files"})
		return
	}
	password, err := secret.Decrypt(h.cfg.AppKey, offer.RESTPasswordEnc)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(fmt.Errorf("decrypt offer credential: %w", err)))
		return
	}

	label := offer.From
	if label == "" {
		label = "mesh peer"
	}
	setID := newMeshCredSetID()
	sets, err := h.svc.CloudCredSets()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	sets = append(sets, CloudCredSet{
		ID:   setID,
		Name: "mesh: " + label,
		CloudCreds: CloudCreds{
			RESTUser:     offer.RESTUser,
			RESTPassword: string(password),
		},
	})
	if err := h.svc.SetCloudCredSets(sets); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}

	target := store.OffsiteTarget{
		Domain:   in.Domain,
		Name:     "mesh: " + label,
		Repo:     offer.Repo,
		CredsRef: setID,
		Enabled:  true,
	}
	stored, err := h.store.UpsertOffsiteTarget(target)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if err := h.store.UpdateMeshOfferStatus(offer.ID, "accepted"); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"target": offsiteTargetToView(stored)}))
}

// handleDeclineMeshOffer marks a pending offer declined. POST
// /api/fleet/mesh-offers/{id}/decline, session-protected. A missing id
// answers 404; an already-decided offer is left untouched (idempotent
// decline of an already-declined offer just re-confirms ok).
func (h *Handler) handleDeclineMeshOffer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok, err := h.store.GetMeshOffer(id); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	} else if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no such mesh offer"})
		return
	}
	if err := h.store.UpdateMeshOfferStatus(id, "declined"); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// meshProposeInput is the propose request body: which of THIS instance's
// domains the offered storage is for, and the base URL this instance's admin
// will deploy the rest-server at (BombVault cannot know this — it may be a
// different box/port than BombVault itself, exactly like the existing
// deploy-snippet feature already assumes).
type meshProposeInput struct {
	Domain  string `json:"domain"`
	BaseURL string `json:"baseUrl"`
}

// meshProposeResponse mirrors DeploySnippet (same one-time credential the
// admin needs to actually deploy the rest-server) plus the concrete repo URL
// that was just sent to the peer, built from BaseURL instead of
// DeploySnippet's generic placeholder.
type meshProposeResponse struct {
	DeploySnippet
	Repo string `json:"repo"`
}

// handleProposeMeshOffer sends this instance's own off-site storage offer to
// one fleet peer. POST /api/fleet/peers/{id}/mesh-offer, session-protected.
// Generates a fresh one-time rest-server credential (the exact same
// generation buildDeploySnippet already uses) scoped to BaseURL instead of
// the generic placeholder, POSTs the connection details to the peer's
// self-gated mesh-offer inbox using the peer's stored token, and returns the
// same deploy recipe an admin needs to actually stand the rest-server up —
// nothing here deploys anything itself.
func (h *Handler) handleProposeMeshOffer(w http.ResponseWriter, r *http.Request) {
	peer, ok, err := h.store.GetFleetPeer(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no such fleet peer"})
		return
	}
	var in meshProposeInput
	if !decodeBody(w, r, &in) {
		return
	}
	if !validOffsiteDomain(in.Domain) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "invalid domain — must be one of containers, vms, flash, config, files"})
		return
	}
	base := strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	if base == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "the base URL where you will deploy the rest-server is required, e.g. http://192.168.1.50:8000"})
		return
	}

	snip, err := buildDeploySnippet(in.Domain)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	repo := fmt.Sprintf("rest:%s/%s/%s", base, snip.User, in.Domain)

	token, err := h.svc.decryptFleetPeerToken(peer)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(fmt.Errorf("decrypt peer token: %w", err)))
		return
	}
	settings, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if err := postMeshOffer(r.Context(), peer.URL, token, meshOfferRequest{
		FromName:        settings.InstanceName,
		SuggestedDomain: in.Domain,
		Repo:            repo,
		RESTUser:        snip.User,
		RESTPassword:    snip.Password,
	}); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(fmt.Errorf("send offer to peer: %w", err)))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"snippet": meshProposeResponse{DeploySnippet: snip, Repo: repo}}))
}

// postMeshOffer sends a mesh offer to a peer's POST /api/fleet/mesh-offer,
// mirroring pollFleetPeer's transport (bounded client, no redirects, skipped
// TLS verification for the same self-signed-cert reason).
func postMeshOffer(ctx context.Context, peerURL, token string, offer meshOfferRequest) error {
	ctx, cancel := context.WithTimeout(ctx, fleetPollTimeout)
	defer cancel()
	body, err := json.Marshal(offer)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(peerURL, "/")+"/api/fleet/mesh-offer", strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("build mesh offer request: %w", err)
	}
	req.Header.Set("X-Fleet-Token", token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := fleetHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // response body close error is not actionable
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, fleetResponseMax))
	if resp.StatusCode != http.StatusOK {
		return errors.New("peer refused the offer (check its fleet token)")
	}
	return nil
}
