package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// newCredSetID returns an id for a credential set the server creates itself,
// the one an accepted mesh offer brings or the one a direct repository keeps,
// in the form store.newID uses. Other credential set ids are minted by the SPA.
func newCredSetID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("newCredSetID: %v", err))
	}
	return hex.EncodeToString(b)
}

// meshOfferRequest is the body of POST /api/group/peer/mesh-offer: a member
// offering a rest-server it deployed as off-site storage. Only these
// connection details cross the group; backups replicate straight to the
// rest-server.
type meshOfferRequest struct {
	FromName        string `json:"fromName"`
	SuggestedDomain string `json:"suggestedDomain"`
	Repo            string `json:"repo"`
	RESTUser        string `json:"restUser"`
	RESTPassword    string `json:"restPassword"`
}

// meshOfferBodyMax caps an offer's body. A real offer is a few hundred bytes,
// and the cap keeps the largest one inside group.MaxCallBytes once it is
// sealed and encoded for the wire.
const meshOfferBodyMax = 64 << 10

// handlePeerMeshOffer stores a member's offer as a pending store.MeshOffer
// for the admin to review. POST /api/group/peer/mesh-offer
func (s *Service) handlePeerMeshOffer(w http.ResponseWriter, r *http.Request) {
	var in meshOfferRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, meshOfferBodyMax)).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "malformed mesh offer body"})
		return
	}
	repo := strings.TrimSpace(in.Repo)
	if repo == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "offer repo must not be empty"})
		return
	}
	enc, err := secret.Encrypt(s.cfg.AppKey, []byte(in.RESTPassword))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	stored, err := s.store.CreateMeshOffer(store.MeshOffer{
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

// meshOfferView is a received mesh offer as the SPA sees it. The password is
// left out; accepting uses the stored ciphertext.
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

// handleListMeshOffers lists received mesh offers in every status; the SPA
// filters them.
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

// meshOfferAcceptInput names the local domain the offered storage will back
// up.
type meshOfferAcceptInput struct {
	Domain      string              `json:"domain"`
	AlsoExclude *newTargetExclusion `json:"alsoExclude"`
}

// dropCredSet takes one credential set out of the stored list. It reads that
// list itself, because writing back a copy read earlier would drop whatever
// another request added or renamed in between, along with its password; the
// write merges blanked secrets back by id.
func (h *Handler) dropCredSet(id string) error {
	sets, err := h.svc.CloudCredSets()
	if err != nil {
		return err
	}
	return h.svc.SetCloudCredSets(slices.DeleteFunc(sets, func(s CloudCredSet) bool { return s.ID == id }))
}

// undoAcceptedOffer takes back the target and the credential set an accepted
// offer wrote, once a later write of the same accept failed, and returns the
// error to answer with. The credentials go only with the target, since a
// target that stays behind would otherwise point at a set that is gone.
func (h *Handler) undoAcceptedOffer(targetID, setID string, cause error) error {
	if err := h.store.DeleteOffsiteTarget(targetID); err != nil {
		return fmt.Errorf("%w; the target and its credentials are still there: %v", cause, err)
	}
	if err := h.dropCredSet(setID); err != nil {
		return fmt.Errorf("%w; the credential set is still there: %v", cause, err)
	}
	return cause
}

// handleAcceptMeshOffer turns a pending offer into a credential set holding
// the peer's REST credentials and an off-site target for the chosen domain
// that uses it. Neither is probed first, just as when an admin creates them
// by hand.
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
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": invalidOffsiteDomain})
		return
	}
	if in.AlsoExclude != nil {
		if err := checkExclusion(in.Domain, *in.AlsoExclude); err != nil {
			placementFail(w, err, nil)
			return
		}
	}
	password, err := secret.Decrypt(h.cfg.AppKey, offer.RESTPasswordEnc)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(fmt.Errorf("decrypt offer credential: %w", err)))
		return
	}
	sortOrder, err := h.svc.nextOffsiteSortOrder(in.Domain)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}

	label := offer.From
	if label == "" {
		label = "mesh peer"
	}
	setID := newCredSetID()
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
		Domain:    in.Domain,
		Name:      "mesh: " + label,
		Repo:      offer.Repo,
		CredsRef:  setID,
		Enabled:   true,
		SortOrder: sortOrder,
	}
	stored, err := h.store.CreateOffsiteTarget(target)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if in.AlsoExclude != nil {
		if err := h.svc.excludeFromTarget(stored.Domain, stored.ID, *in.AlsoExclude); err != nil {
			placementFail(w, h.undoAcceptedOffer(stored.ID, setID, err), nil)
			return
		}
	}
	if err := h.store.UpdateMeshOfferStatus(offer.ID, "accepted"); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(h.undoAcceptedOffer(stored.ID, setID, err)))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"target": offsiteTargetToView(stored)}))
}

// handleDeclineMeshOffer marks an offer declined. An unknown id answers 404.
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

// meshProposeInput is the domain the offered storage is for and the base URL
// the admin will deploy the rest-server at. BombVault cannot work out that
// URL; the rest-server may run on another host or port.
type meshProposeInput struct {
	Domain  string `json:"domain"`
	BaseURL string `json:"baseUrl"`
}

// meshProposeResponse is the deploy snippet plus the repo URL that was sent to
// the peer.
type meshProposeResponse struct {
	DeploySnippet
	Repo string `json:"repo"`
}

// handleProposeMeshOffer offers storage to a group member. It generates a
// one-time rest-server credential, sends the connection details to the
// member's mesh-offer inbox over the group, and returns the deploy snippet.
// Deploying the rest-server is left to the admin.
// POST /api/fleet/peers/{id}/mesh-offer
func (h *Handler) handleProposeMeshOffer(w http.ResponseWriter, r *http.Request) {
	peer, ok := h.lookupFleetPeer(w, r)
	if !ok {
		return
	}
	if peer.NeedsPairing() {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "pair this instance again first"})
		return
	}
	var in meshProposeInput
	if !decodeBody(w, r, &in) {
		return
	}
	if !validOffsiteDomain(in.Domain) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": invalidOffsiteDomain})
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

	settings, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if err := h.svc.callMember(r.Context(), peer.MemberID, http.MethodPost, "/api/group/peer/mesh-offer", meshOfferRequest{
		FromName:        instanceDisplayName(settings),
		SuggestedDomain: in.Domain,
		Repo:            repo,
		RESTUser:        snip.User,
		RESTPassword:    snip.Password,
	}, nil); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(fmt.Errorf("send offer to peer: %w", err)))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"snippet": meshProposeResponse{DeploySnippet: snip, Repo: repo}}))
}
