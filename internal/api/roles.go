package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"

	"golang.org/x/mod/semver"

	"github.com/junkerderprovinz/bombvault/internal/group"
	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// peerRoleBodyMax caps a role call, which is a role, a name and a few
// section names.
const peerRoleBodyMax = 4 << 10

// roleNameMax is how much of an instance's name a request keeps.
const roleNameMax = 200

// roleRequestsSince is the first release line whose instances ask for and
// answer role requests. An older member hands out a receiver login and its
// restic password to whoever asks.
const roleRequestsSince = "v10.0"

// allRoleSections are the backup domains a role can cover, sorted the way
// every stored request keeps them.
var allRoleSections = []string{"config", "containers", "files", "flash", "vms", "zfs"}

var errRoleNeedsReceiver = errors.New("set up a receiver first: it is where the other instance's copies go")

// predatesRoleRequests reports whether a member said it runs a release from
// before role requests. An unstamped build counts as current, and so does a
// build between releases: it carries the last release's number with its branch
// appended, which says nothing about what the build can do.
func predatesRoleRequests(version string) bool {
	return semver.IsValid(version) && semver.Build(version) == "" && semver.Compare(semver.MajorMinor(version), roleRequestsSince) < 0
}

type peerSenderKey struct{}

// peerSender is the instance id of the member a peer call came from as the
// group channel carried it, sealed into the call or signed in its header. It
// is empty for a call through a relay from a member older than role requests.
func peerSender(r *http.Request) string {
	id, _ := r.Context().Value(peerSenderKey{}).(string)
	return id
}

// roleAsker names the member behind a call to the request routes, which only
// instances that state who they are reach. Another instance's id is never
// taken from the body, and nobody asks in this instance's own name.
func (s *Service) roleAsker(r *http.Request) (string, bool) {
	sender := peerSender(r)
	if !zfs.ValidSourceID(sender) {
		return "", false
	}
	g, err := s.store.GetGroupState()
	if err != nil || sender == g.InstanceID {
		return "", false
	}
	return sender, true
}

// checkRoleAsk normalises what a request asks for and says what is wrong
// with it, if anything.
func checkRoleAsk(role string, sections []string, where string) ([]string, string, string) {
	if role != store.RoleReceiver && role != store.RoleFetcher {
		return nil, "", "the role must be receiver or fetcher"
	}
	sections = slices.Clone(sections)
	slices.Sort(sections)
	sections = slices.Compact(sections)
	if len(sections) == 0 {
		return nil, "", "choose at least one section"
	}
	for _, sec := range sections {
		if !slices.Contains(allRoleSections, sec) {
			return nil, "", "unknown section " + strings.ToValidUTF8(sec, "")
		}
	}
	switch {
	case role == store.RoleFetcher:
		where = ""
	case where == "":
		where = store.RoleStoreRest
	case where != store.RoleStoreRest && where != store.RoleStoreShare:
		return nil, "", "a receiver keeps the copies in its rest-server or on a share"
	}
	return sections, where, ""
}

func clipRoleName(name string) string {
	name = strings.TrimSpace(strings.ToValidUTF8(name, ""))
	if len(name) <= roleNameMax {
		return name
	}
	cut := roleNameMax
	for cut > 0 && !utf8.RuneStart(name[cut]) {
		cut--
	}
	return name[:cut]
}

// peerRoleAsk is what an instance sends to ask this one for a role. Renew is
// set when a person there asked, which is the only request that reopens a
// revoked one.
type peerRoleAsk struct {
	Name     string   `json:"name"`
	Role     string   `json:"role"`
	Sections []string `json:"sections"`
	Store    string   `json:"store,omitempty"`
	Renew    bool     `json:"renew"`
}

// peerRoleState is the answer to an ask: where the request stands here.
type peerRoleState struct {
	OK        bool     `json:"ok"`
	State     string   `json:"state"`
	Sections  []string `json:"sections"`
	DirectURL string   `json:"directUrl,omitempty"`
}

// peerRoleNote is what the two short role calls carry: the answer a person
// gave to the receiving instance's request, or the role a request is
// withdrawn for.
type peerRoleNote struct {
	Role  string `json:"role"`
	State string `json:"state,omitempty"`
	peerProof
}

func decodePeerRole(w http.ResponseWriter, r *http.Request, into any) bool {
	if err := json.NewDecoder(io.LimitReader(r.Body, peerRoleBodyMax)).Decode(into); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "malformed role request"})
		return false
	}
	return true
}

func refuseUnnamedPeer(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "the asking instance did not say who it is"})
}

// handlePeerRoleAsk records a member's request to be its Receiver or Fetcher
// and answers with the request's state at once. A person answers later; the
// asking instance is told then and can also ask again to find out.
// POST /api/group/peer/roles/ask
func (s *Service) handlePeerRoleAsk(w http.ResponseWriter, r *http.Request) {
	var in peerRoleAsk
	if !decodePeerRole(w, r, &in) {
		return
	}
	asker, ok := s.roleAsker(r)
	if !ok {
		refuseUnnamedPeer(w)
		return
	}
	sections, where, msg := checkRoleAsk(in.Role, in.Sections, in.Store)
	if msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}
	req, err := s.store.AskRoleRequest(store.RoleRequest{
		MemberID: asker, MemberName: clipRoleName(in.Name), Role: in.Role, Sections: sections, Store: where,
	}, in.Renew)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, peerRoleState{OK: true, State: req.State, Sections: req.Sections, DirectURL: s.selfDirectURL()})
}

// handlePeerRoleAnswer takes the answer a member gave to a request this
// instance sent it. Only the member that was asked can answer, since the
// request is looked up under the caller's id.
// POST /api/group/peer/roles/answer
func (s *Service) handlePeerRoleAnswer(w http.ResponseWriter, r *http.Request) {
	var in peerRoleNote
	if !decodePeerRole(w, r, &in) {
		return
	}
	asked, ok := s.roleAsker(r)
	if !ok {
		refuseUnnamedPeer(w)
		return
	}
	if !slices.Contains([]string{store.RoleAllowed, store.RoleRefused, store.RoleRevoked}, in.State) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "the answer must be allowed, refused or revoked"})
		return
	}
	if err := s.verifyMember(r.Context(), asked, keydeposit.ProofRoleAnswer, callPayload(in.Role, in.State), in.peerProof); err != nil {
		writeJSON(w, http.StatusForbidden, failEnvelope(err))
		return
	}
	known, err := s.store.SetSentRoleRequestState(asked, in.Role, in.State)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"known": known}))
}

// handlePeerRoleWithdraw ends a request the calling member sent here. An
// allowed Receiver loses its login, as it would when a person here revoked it.
// POST /api/group/peer/roles/withdraw
func (s *Service) handlePeerRoleWithdraw(w http.ResponseWriter, r *http.Request) {
	var in peerRoleNote
	if !decodePeerRole(w, r, &in) {
		return
	}
	asker, ok := s.roleAsker(r)
	if !ok {
		refuseUnnamedPeer(w)
		return
	}
	if err := s.verifyMember(r.Context(), asker, keydeposit.ProofRoleWithdraw, callPayload(in.Role), in.peerProof); err != nil {
		writeJSON(w, http.StatusForbidden, failEnvelope(err))
		return
	}
	prev, found, err := s.store.WithdrawRoleRequest(asker, in.Role)
	if err == nil && found && prev.State == store.RoleAllowed && prev.Role == store.RoleReceiver {
		err = s.dropReceiverLogin(r.Context(), asker)
	}
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// dropReceiverLogin takes a member's login off the receiver, if this instance
// runs one.
func (s *Service) dropReceiverLogin(ctx context.Context, memberID string) error {
	if err := s.RevokeReceiverLogin(ctx, memberID); !errors.Is(err, errNoReceiver) {
		return err
	}
	return nil
}

// reachableMember returns the instance with the given id while the group
// reaches it. A phone running the app takes no roles.
func (s *Service) reachableMember(memberID string) (group.Member, bool) {
	for _, m := range s.pairing().Members() {
		if m.ID == memberID && m.Kind == "" {
			return m, true
		}
	}
	return group.Member{}, false
}

// askRole asks a member to take a role for this instance, as a person here
// wants it, and records what the member answered. A member older than role
// requests cannot be asked: its receiver hands a login to whoever asks, and
// it fetches whatever it is given the password for, so the request counts as
// allowed the moment it is made here.
func (s *Service) askRole(ctx context.Context, m group.Member, role string, sections []string, where string) (store.RoleRequest, error) {
	s.roleSendMu.Lock()
	defer s.roleSendMu.Unlock()
	sent := store.RoleRequest{MemberID: m.ID, MemberName: m.Name, Role: role, Sections: sections, Store: where}
	if predatesRoleRequests(m.Version) {
		if role == store.RoleReceiver {
			offer, err := s.askReceiver(ctx, m.ID, false)
			if err != nil {
				return store.RoleRequest{}, err
			}
			if offer == nil {
				return store.RoleRequest{}, errNoMemberReceiver
			}
		}
		sent.State, sent.DecidedBy = store.RoleAllowed, store.RoleDecidedAutomatic
	} else {
		state, err := s.sendRoleAsk(ctx, sent, true)
		if err != nil {
			return store.RoleRequest{}, err
		}
		sent.State = state
	}
	saved, err := s.store.SaveSentRoleRequest(sent)
	if err != nil {
		return store.RoleRequest{}, err
	}
	// The new request replaces one withdrawn while the member was away, so
	// there is nothing left to tell it.
	if err := s.store.ForgetRoleWithdrawal(m.ID, role); err != nil {
		return store.RoleRequest{}, err
	}
	return saved, nil
}

// sendRoleAsk puts a request to the member it is for and returns the state
// the member holds it in.
func (s *Service) sendRoleAsk(ctx context.Context, req store.RoleRequest, renew bool) (string, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return "", err
	}
	ask := peerRoleAsk{Name: instanceDisplayName(settings), Role: req.Role, Sections: req.Sections, Store: req.Store, Renew: renew}
	var ans peerRoleState
	if err := s.callMember(ctx, req.MemberID, http.MethodPost, "/api/group/peer/roles/ask", ask, &ans); err != nil {
		return "", err
	}
	if !slices.Contains([]string{store.RoleAsked, store.RoleAllowed, store.RoleRefused, store.RoleRevoked}, ans.State) {
		return "", errMemberUnreadable
	}
	return ans.State, nil
}

// withdrawRole takes back a request this instance sent and reports whether
// the member was told. The request is forgotten here before anything else,
// so the member loses what this instance gave it for the role even while it
// is out of reach or another call to it is under way. A member that does not
// hear still holds the request as allowed, so the withdrawal is noted and
// told again until it arrives.
func (s *Service) withdrawRole(ctx context.Context, memberID, role string) (bool, error) {
	if err := s.store.DeleteRoleRequest(store.RoleRequestOut, memberID, role); err != nil {
		return false, err
	}
	s.roleSendMu.Lock()
	defer s.roleSendMu.Unlock()
	// An ask that was under way has written its answer by now.
	if err := s.store.DeleteRoleRequest(store.RoleRequestOut, memberID, role); err != nil {
		return false, err
	}
	if err := s.store.NoteRoleWithdrawal(memberID, role); err != nil {
		return false, err
	}
	return s.tellRoleWithdrawn(ctx, memberID, role), nil
}

// tellRoleWithdrawn tells the member that the request for role is withdrawn
// and forgets the note once it heard. The caller holds roleSendMu. A member
// older than role requests holds no request to take back.
func (s *Service) tellRoleWithdrawn(ctx context.Context, memberID, role string) bool {
	m, ok := s.reachableMember(memberID)
	if !ok {
		return false
	}
	if !predatesRoleRequests(m.Version) {
		proof, err := s.signCall(keydeposit.ProofRoleWithdraw, memberID, callPayload(role))
		if err != nil {
			log.Printf("roles: sign the withdrawal of the %s role for %q: %v", role, memberID, err) //nolint:gosec // G706: a role from a fixed set and an instance id from the store
			return false
		}
		if s.callMember(ctx, memberID, http.MethodPost, "/api/group/peer/roles/withdraw", peerRoleNote{Role: role, peerProof: proof}, nil) != nil {
			return false
		}
	}
	if err := s.store.ForgetRoleWithdrawal(memberID, role); err != nil {
		log.Printf("roles: %v", err)
	}
	return !predatesRoleRequests(m.Version)
}

// retellRoleWithdrawals tells every member in reach again about a withdrawal
// it has not heard of.
func (s *Service) retellRoleWithdrawals(ctx context.Context) {
	pending, err := s.store.ListRoleWithdrawals()
	if err != nil {
		log.Printf("roles: %v", err)
		return
	}
	for _, w := range pending {
		if _, ok := s.reachableMember(w.MemberID); !ok {
			continue
		}
		s.roleSendMu.Lock()
		// Read under the lock, so a request asked for again meanwhile is not
		// withdrawn after all.
		if still, err := s.store.RoleWithdrawn(w.MemberID, w.Role); err == nil && still {
			s.tellRoleWithdrawn(ctx, w.MemberID, w.Role)
		}
		s.roleSendMu.Unlock()
	}
}

// forgetMemberRoles ends every Receiver and Fetcher request between this
// instance and a member removed while out of reach, since those requests alone
// would keep its card, and a server taken out of service keeps no login here.
// Should the member come back, it hears of the ones this instance sent. One in
// reach is listed again anyway and keeps its requests.
func (s *Service) forgetMemberRoles(ctx context.Context, memberID string) error {
	if _, ok := s.reachableMember(memberID); ok {
		return nil
	}
	reqs, err := s.store.ListRoleRequests()
	if err != nil {
		return err
	}
	s.roleSendMu.Lock()
	defer s.roleSendMu.Unlock()
	for _, req := range reqs {
		if req.MemberID != memberID {
			continue
		}
		if err := s.store.DeleteRoleRequest(req.Direction, memberID, req.Role); err != nil {
			return err
		}
		switch {
		case req.Direction == store.RoleRequestOut && (req.State == store.RoleAsked || req.State == store.RoleAllowed):
			err = s.store.NoteRoleWithdrawal(memberID, req.Role)
		case req.Role == store.RoleReceiver && req.State == store.RoleAllowed:
			err = s.dropReceiverLogin(ctx, memberID)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// tellRoleAnswer tells the member that sent a request what a person here
// answered, and reports whether it heard. A member that did not hear learns
// the answer the next time it asks.
func (s *Service) tellRoleAnswer(ctx context.Context, req store.RoleRequest) bool {
	m, ok := s.reachableMember(req.MemberID)
	if !ok || predatesRoleRequests(m.Version) {
		return false
	}
	proof, err := s.signCall(keydeposit.ProofRoleAnswer, req.MemberID, callPayload(req.Role, req.State))
	if err != nil {
		return false
	}
	note := peerRoleNote{Role: req.Role, State: req.State, peerProof: proof}
	return s.callMember(ctx, req.MemberID, http.MethodPost, "/api/group/peer/roles/answer", note, nil) == nil
}

// decideRole gives a member's request a person's answer, takes the receiver
// login away with a revoke, and tells the member. The second result reports
// whether the member heard.
func (s *Service) decideRole(ctx context.Context, id string, d store.RoleDecision) (store.RoleRequest, bool, error) {
	req, ok, err := s.store.GetRoleRequest(id)
	if err != nil {
		return store.RoleRequest{}, false, err
	}
	if !ok || req.Direction != store.RoleRequestIn {
		return store.RoleRequest{}, false, sql.ErrNoRows
	}
	if d.State == store.RoleAllowed && req.State != store.RoleAllowed && req.Role == store.RoleReceiver && req.Store != store.RoleStoreShare {
		if _, running, err := s.store.GetReceiverServer(); err != nil {
			return store.RoleRequest{}, false, err
		} else if !running {
			return store.RoleRequest{}, false, errRoleNeedsReceiver
		}
	}
	req, err = s.store.DecideRoleRequest(id, d)
	if err != nil {
		return store.RoleRequest{}, false, err
	}
	if req.State == store.RoleRevoked && req.Role == store.RoleReceiver {
		if err := s.dropReceiverLogin(ctx, req.MemberID); err != nil {
			return store.RoleRequest{}, false, err
		}
	}
	return req, s.tellRoleAnswer(ctx, req), nil
}

// refreshSentRoles asks every member again that this instance has a request
// with, so an answer or a revoke the member could not deliver arrives anyway,
// and tells the members a withdrawal they missed. It asks as the instance, not
// as a person, which leaves a revoked request revoked.
func (s *Service) refreshSentRoles(ctx context.Context) {
	s.retellRoleWithdrawals(ctx)
	reqs, err := s.store.ListRoleRequests()
	if err != nil {
		log.Printf("roles: %v", err)
		return
	}
	for _, req := range reqs {
		if req.Direction == store.RoleRequestOut {
			s.refreshSentRole(ctx, req.MemberID, req.Role)
		}
	}
}

func (s *Service) refreshSentRole(ctx context.Context, memberID, role string) {
	m, ok := s.reachableMember(memberID)
	if !ok || predatesRoleRequests(m.Version) {
		return
	}
	s.roleSendMu.Lock()
	defer s.roleSendMu.Unlock()
	// Read under the lock, so a request withdrawn meanwhile is not sent again.
	req, ok, err := s.store.FindRoleRequest(store.RoleRequestOut, memberID, role)
	if err != nil || !ok || (req.State != store.RoleAsked && req.State != store.RoleAllowed) {
		return
	}
	state, err := s.sendRoleAsk(ctx, req, false)
	if err == nil {
		_, err = s.store.SetSentRoleRequestState(memberID, role, state)
	}
	if err != nil {
		log.Printf("roles: asking %q again about the %s role: %v", memberID, role, err) //nolint:gosec // G706: an instance id from the store and a role from a fixed set
	}
}
