package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/junkerderprovinz/bombvault/internal/secret"
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

// phoneSession mints a session without the password, marked as the one of a
// paired phone: it opens the pages, and what hands out a key asks for a
// sign-in on top.
func (h *Handler) phoneSession() peerSessionResponse {
	hash, epoch, on := h.authEnabled()
	if !on {
		return peerSessionResponse{OK: true}
	}
	c := h.newSessionCookie(secret.NewPairedSessionToken(h.cfg.AppKey, hash, epoch, sessionTTL), int(sessionTTL.Seconds()))
	return peerSessionResponse{OK: true, Needed: true, Name: c.Name, Value: c.Value, MaxAge: c.MaxAge, Secure: c.Secure}
}

// handlePeerSession gives a session to a member that is a phone. The phone
// names itself inside the sealed call and has to be the member the call came
// from, so a BombVault instance of the group never comes away with one.
func (s *Service) handlePeerSession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InstanceID string `json:"instanceId"`
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
	if on, err := s.store.PhonePairing(); err != nil || !on {
		writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "error": "pairing phones is switched off on this instance", "code": codePhonePairingOff})
		return
	}
	// Synced first: a member that joined since anybody last opened the
	// Instances page has no row yet.
	peers, err := s.syncFleetPeers()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	for _, p := range peers {
		if p.MemberID == body.InstanceID && p.Kind == "android" && p.Enabled {
			writeJSON(w, http.StatusOK, s.peerSession())
			return
		}
	}
	writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "error": "no phone of this group by that name"})
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
