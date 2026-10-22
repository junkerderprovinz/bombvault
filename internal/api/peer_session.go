package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// peerSessionResponse is what POST /api/group/peer/session answers: the
// session cookie a paired phone sets before it opens this instance, or
// Needed false when the instance has no password to sign in with.
type peerSessionResponse struct {
	OK     bool   `json:"ok"`
	Needed bool   `json:"needed"`
	Name   string `json:"name,omitempty"`
	Value  string `json:"value,omitempty"`
	MaxAge int    `json:"maxAge,omitempty"`
	Secure bool   `json:"secure,omitempty"`
}

// phoneSession mints a session without the password, marked as the one of
// the paired phone phoneID: it opens the pages, and what hands out a key asks
// for a sign-in on top.
func (h *Handler) phoneSession(phoneID string) peerSessionResponse {
	hash, epoch, on := h.authEnabled()
	if !on {
		return peerSessionResponse{OK: true}
	}
	c := h.newSessionCookie(secret.NewPairedSessionToken(h.cfg.AppKey, hash, epoch, phoneID, sessionTTL), int(sessionTTL.Seconds()))
	return peerSessionResponse{OK: true, Needed: true, Name: c.Name, Value: c.Value, MaxAge: c.MaxAge, Secure: c.Secure}
}

// phoneSessionCookie is the cookie for a page phoneID opens through the
// relay with the session it holds. Without a login password none is needed.
// A session that is not this phone's, or that a sign-out everywhere or a
// reset ended, gives none.
func (h *Handler) phoneSessionCookie(phoneID, token string) (*http.Cookie, bool) {
	hash, epoch, on := h.authEnabled()
	if !on {
		return nil, true
	}
	if phone, ok := secret.PairedSessionPhone(token); !ok || phone != phoneID || !secret.ValidSessionToken(h.cfg.AppKey, hash, epoch, token) {
		return nil, false
	}
	return &http.Cookie{Name: h.newSessionCookie("", 0).Name, Value: token}, true //nolint:gosec // G124: a cookie on a request this instance builds for itself
}

// handlePeerSession gives a session to a member that is a phone. The phone
// names itself, has to be the member the call came from and signs the ask
// with its own key, so a BombVault instance of the group never comes away
// with one.
func (s *Service) handlePeerSession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InstanceID string `json:"instanceId"`
		peerProof
	}
	if s.peerSession == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no sessions here"})
		return
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil || body.InstanceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "no instance named"})
		return
	}
	if peerSender(r) != body.InstanceID {
		writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "error": "a session is only given to the member that asks for it"})
		return
	}
	if !s.admitPhone(w, body.InstanceID) {
		return
	}
	if err := s.verifyPhone(r.Context(), body.InstanceID, keydeposit.ProofSession, callPayload(body.InstanceID), body.peerProof); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "error": errUnsignedCall.Error()})
		return
	}
	writeJSON(w, http.StatusOK, s.peerSession(body.InstanceID))
}

// admitPhone reports whether id is a phone the Instances page lists and lets
// in while pairing phones is on, and answers the call itself when it is not.
func (s *Service) admitPhone(w http.ResponseWriter, id string) bool {
	if on, err := s.store.PhonePairing(); err != nil || !on {
		writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "error": "pairing phones is switched off on this instance", "code": codePhonePairingOff})
		return false
	}
	if id != "" {
		// A page asks many times a minute, so the rows are read before they
		// are synced. A member that joined since anybody last opened the
		// Instances page has no row until they are.
		if rows, err := s.store.ListFleetPeers(); err == nil && listsPhone(rows, id) {
			return true
		}
		peers, err := s.syncFleetPeers()
		if err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return false
		}
		if listsPhone(peers, id) {
			return true
		}
	}
	writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "error": "no phone of this group by that name"})
	return false
}

// pairedPhoneListed reports whether the phone a paired session was made for
// is still one admitPhone lets in. A phone whose row is gone or switched off
// is refused at this instance's own address too, not only through the relay.
func (h *Handler) pairedPhoneListed(token string) bool {
	id, ok := secret.PairedSessionPhone(token)
	if !ok {
		return false
	}
	rows, err := h.store.ListFleetPeers()
	return err == nil && listsPhone(rows, id)
}

func listsPhone(peers []store.FleetPeer, id string) bool {
	for _, p := range peers {
		if p.MemberID == id && p.Kind == "android" && p.Enabled {
			return true
		}
	}
	return false
}

// handleGetPhonePairing says whether a phone of the group may pair.
// GET /api/auth/phone-pairing
func (h *Handler) handleGetPhonePairing(w http.ResponseWriter, _ *http.Request) {
	on, err := h.store.PhonePairing()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"on": on}))
}

// handleSetPhonePairing switches pairing of phones on or off. Off, a phone
// gets no session and the sessions phones hold are refused.
// PUT /api/auth/phone-pairing
func (h *Handler) handleSetPhonePairing(w http.ResponseWriter, r *http.Request) {
	var body struct {
		On bool `json:"on"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	// Switching it on lets the group's words in, so a session a reset ended
	// while this request was under way must not do it.
	release, ok := h.holdSignIn(w, r, "switching pairing phones on or off")
	if !ok {
		return
	}
	defer release()
	if err := h.store.SetPhonePairing(body.On); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"on": body.On}))
}
