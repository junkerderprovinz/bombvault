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

// phoneSession mints the session a login would, without the password: the
// group key already gives a member every backup of this instance.
func (h *Handler) phoneSession() peerSessionResponse {
	hash, epoch, on := h.authEnabled()
	if !on {
		return peerSessionResponse{OK: true}
	}
	c := h.newSessionCookie(secret.NewSessionToken(h.cfg.AppKey, hash, epoch, sessionTTL), int(sessionTTL.Seconds()))
	return peerSessionResponse{OK: true, Needed: true, Name: c.Name, Value: c.Value, MaxAge: c.MaxAge, Secure: c.Secure}
}

// handlePeerSession gives a session to a member that is a phone. The phone
// names itself inside the sealed call, so a BombVault instance of the group
// never comes away with one.
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
