package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/junkerderprovinz/bombvault/internal/group"
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
// the paired phone phoneID: it opens the pages through the phone door, and
// what hands out a key asks for a sign-in on top. It is signed with the
// session epoch, which everything that ends the owner's sessions moves.
func (h *Handler) phoneSession(phoneID string) peerSessionResponse {
	if h.signInOpen() {
		return peerSessionResponse{OK: true}
	}
	if !h.phoneStands(phoneID) {
		return peerSessionResponse{}
	}
	s, err := h.store.GetSettings()
	if err != nil {
		return peerSessionResponse{}
	}
	c := h.newSessionCookie(secret.NewPairedSessionToken(h.cfg.AppKey, "", s.SessionEpoch, phoneID, sessionTTL), int(sessionTTL.Seconds()))
	return peerSessionResponse{OK: true, Needed: true, Name: c.Name, Value: c.Value, MaxAge: c.MaxAge, Secure: c.Secure}
}

// phoneSessionCookie is the cookie for a page phoneID opens through the
// relay with the session it holds. Without a login password none is needed.
// A session that is not this phone's, or that a sign-out everywhere or a
// reset ended, gives none.
func (h *Handler) phoneSessionCookie(phoneID, token string) (*http.Cookie, bool) {
	if h.signInOpen() {
		return nil, true
	}
	if !h.phoneSessionValid(phoneID, token) || !h.phoneStands(phoneID) {
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
	if peerSender(r) != body.InstanceID && !s.phoneNamesItself(r, body.InstanceID, body.Key) {
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

// phoneNamesItself takes the id a phone of the group names for itself, when
// its key was pinned under no id before the group ran on the kit: such a
// phone still signs as the id it had, and the key the call came with is the
// one it signs with.
func (s *Service) phoneNamesItself(r *http.Request, id string, key []byte) bool {
	sender, ok := group.SenderOf(r.Context())
	if !ok || sender.Kind != group.KindPhone || sender.ID != group.PhoneID(sender.Key) || !bytes.Equal(sender.Key, key) || len(id) > 64 {
		return false
	}
	if _, found, err := s.store.MemberKey(id); err != nil || found {
		return false
	}
	if err := s.store.PinMemberKey(id, key); err != nil {
		return false
	}
	return s.store.ForgetMemberKey(sender.ID) == nil
}

// admitPhone reports whether id is a phone the Instances page lists and lets
// in, and answers the call itself when it is not.
func (s *Service) admitPhone(w http.ResponseWriter, id string) bool {
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
