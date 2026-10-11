package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/group"
	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
)

// maxCounterLead is how far ahead of this server's clock the counter of a
// signed call may run. A counter is a time in milliseconds, and one far in
// the future would leave the member's own later calls looking replayed.
const maxCounterLead = 24 * time.Hour

var errUnsignedCall = errors.New("the call is not signed with the key of the member it names")

// callPayload spells what a signed call says, for its claim.
func callPayload(parts ...string) []byte {
	b, _ := json.Marshal(parts)
	return b
}

// signCall signs what this instance says to a member in a call.
func (s *Service) signCall(action, memberID string, payload []byte) (peerProof, error) {
	signer, err := s.nextDepositSigner()
	if err != nil {
		return peerProof{}, err
	}
	return signer.prove(action, memberID, payload, nil), nil
}

// peerKeyAnswer is a member's signing key, with a proof that it holds it.
type peerKeyAnswer struct {
	OK    bool   `json:"ok"`
	Key   []byte `json:"key"`
	Proof []byte `json:"proof"`
}

// handlePeerKey names the key this instance signs its calls with.
// GET /api/group/peer/key
func (s *Service) handlePeerKey(w http.ResponseWriter, r *http.Request) {
	seed, err := s.depositSeed()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	g, err := s.store.GetGroupState()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	p := depositSigner{id: g.InstanceID, seed: seed}.prove(keydeposit.ProofKey, peerSender(r), nil, nil)
	writeJSON(w, http.StatusOK, peerKeyAnswer{OK: true, Key: p.Key, Proof: p.Proof})
}

// memberKey returns the key a member signs with. The first time, the member
// is asked for it through the group and the answer is pinned, so a caller
// that names the member brings no key of its own.
func (s *Service) memberKey(ctx context.Context, memberID string) ([]byte, error) {
	key, found, err := s.store.MemberKey(memberID)
	if err != nil || found {
		return key, err
	}
	var ans peerKeyAnswer
	if err := s.callMember(ctx, memberID, http.MethodGet, "/api/group/peer/key", nil, &ans); err != nil {
		return nil, err
	}
	g, err := s.store.GetGroupState()
	if err != nil {
		return nil, err
	}
	if !keydeposit.Proven(ans.Key, ans.Proof, keydeposit.Claim{Action: keydeposit.ProofKey, OwnerID: memberID, KeeperID: g.InstanceID}) {
		return nil, errMemberUnreadable
	}
	if err := s.store.PinMemberKey(memberID, ans.Key); err != nil {
		return nil, err
	}
	return ans.Key, nil
}

// verifyMember checks that a call in the name of memberID is signed with the
// key pinned for it and carries a counter newer than every call before.
func (s *Service) verifyMember(ctx context.Context, memberID, action string, payload []byte, p peerProof) error {
	if len(p.Key) == 0 || p.Counter > time.Now().Add(maxCounterLead).UnixMilli() {
		return errUnsignedCall
	}
	key, err := s.memberKey(ctx, memberID)
	if err != nil {
		return err
	}
	g, err := s.store.GetGroupState()
	if err != nil {
		return err
	}
	claim := keydeposit.Claim{Action: action, OwnerID: memberID, KeeperID: g.InstanceID, Counter: p.Counter, Deposit: payload}
	if !bytes.Equal(key, p.Key) || !keydeposit.Proven(key, p.Proof, claim) {
		return errUnsignedCall
	}
	return s.store.AdvanceMemberCounter(memberID, p.Counter)
}

// verifyCaller is verifyMember for the routes members older than role
// requests call as well. Such a member signs nothing, and its call counts as
// before while it is one this install knew before members signed.
func (s *Service) verifyCaller(ctx context.Context, caller group.Member, action string, payload []byte, p peerProof) error {
	if len(p.Key) == 0 {
		if old, err := s.legacyMember(caller); err != nil || old {
			return err
		}
	}
	return s.verifyMember(ctx, caller.ID, action, payload, p)
}

// legacyMember reports whether m is a member older than role requests. What
// the member says about its version only counts for one this install knew
// before members signed, so a member met later cannot pass for one.
func (s *Service) legacyMember(m group.Member) (bool, error) {
	if !predatesRoleRequests(m.Version) {
		return false, nil
	}
	return s.store.LegacyMember(m.ID)
}

// handleForgetMemberKey drops the key pinned for a member, for when it is not
// the member's own. The next signed call pins the key the member names
// through the group. DELETE /api/group/members/{id}/key
func (h *Handler) handleForgetMemberKey(w http.ResponseWriter, r *http.Request) {
	if err := h.store.ForgetMemberKey(r.PathValue("id")); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}
