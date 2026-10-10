package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"slices"

	"github.com/junkerderprovinz/bombvault/internal/group"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

var errRoleNotAllowed = errors.New("that instance has not allowed this yet; the request waits on its Instances page")

var errPeerNotAllowed = errors.New("this instance has not allowed that for the asking instance")

var errPeerNotAsked = errors.New("this instance has not asked that one to keep or to fetch its backups")

var errPasswordHeld = errors.New("that instance has not released its restic password to this one: it first has to ask this instance to be its Receiver or Fetcher")

// peerCaller names the member behind a call to a route older than role
// requests. A member that states who it is counts as that member. One too old
// to say so on a call through a relay is recognised only while it is the one
// such member in the group, since any other of them could have sent the call.
func (s *Service) peerCaller(r *http.Request) (group.Member, bool) {
	members := s.pairing().Members()
	if sender, ok := s.roleAsker(r); ok {
		for _, m := range members {
			if m.ID == sender {
				return m, true
			}
		}
		return group.Member{ID: sender}, true
	}
	if peerSender(r) != "" {
		return group.Member{}, false
	}
	var older []group.Member
	for _, m := range members {
		if m.Kind == "" && predatesRoleRequests(m.Version) {
			older = append(older, m)
		}
	}
	if len(older) != 1 {
		return group.Member{}, false
	}
	return older[0], true
}

// noteOldReceiverAsk records a request for a member that asked for a receiver
// login the way instances did before role requests, so a person here finds it
// and can allow it. The member cannot say which sections it will send.
func (s *Service) noteOldReceiverAsk(caller group.Member, name string) {
	if !predatesRoleRequests(caller.Version) {
		return
	}
	_, err := s.store.AskRoleRequest(store.RoleRequest{
		MemberID: caller.ID, MemberName: clipRoleName(name), Role: store.RoleReceiver,
		Sections: allRoleSections, Store: store.RoleStoreRest,
	}, true)
	if err != nil {
		log.Printf("roles: record the receiver request of %q: %v", caller.ID, err) //nolint:gosec // G706: an instance id the group channel carried
	}
}

// pairingGrant is what this instance hands a member that asks how to open
// its repositories: the sections it may learn the locations of, and whether
// the restic password goes with them.
type pairingGrant struct {
	sections []string
	password bool
}

// pairingGrantFor reads the grant from the requests this instance sent the
// member. A request still waiting shows where the repositories are, so the
// other side can choose one while it answers; only an allowed one releases
// the password.
func (s *Service) pairingGrantFor(memberID string) (pairingGrant, error) {
	var g pairingGrant
	for _, role := range []string{store.RoleReceiver, store.RoleFetcher} {
		req, ok, err := s.store.FindRoleRequest(store.RoleRequestOut, memberID, role)
		if err != nil {
			return pairingGrant{}, err
		}
		if !ok || (req.State != store.RoleAsked && req.State != store.RoleAllowed) {
			continue
		}
		g.sections = append(g.sections, req.Sections...)
		g.password = g.password || req.State == store.RoleAllowed
	}
	return g, nil
}

// retellRoleAnswer tells a member once more that this instance allowed its
// request, in case the first telling did not arrive: the member releases its
// password only after it has heard.
func (s *Service) retellRoleAnswer(ctx context.Context, memberID, role string) {
	req, ok, err := s.store.FindRoleRequest(store.RoleRequestIn, memberID, role)
	if err == nil && ok && req.State == store.RoleAllowed {
		s.tellRoleAnswer(ctx, req)
	}
}

// grantOldMember records that this instance takes a role for a member older
// than role requests, which gave its password without being asked for leave.
func (s *Service) grantOldMember(memberID, role string) error {
	m, ok := s.reachableMember(memberID)
	if !ok || !predatesRoleRequests(m.Version) {
		return nil
	}
	return s.store.GrantRoleRequest(store.RoleRequest{
		Direction: store.RoleRequestIn, MemberID: m.ID, MemberName: m.Name, Role: role, Sections: allRoleSections,
	})
}

// ensureReceiverRole makes sure a member has allowed this instance to send
// to its receiver, asking it when there is no request yet.
func (s *Service) ensureReceiverRole(ctx context.Context, m group.Member) error {
	req, ok, err := s.store.FindRoleRequest(store.RoleRequestOut, m.ID, store.RoleReceiver)
	if err != nil {
		return err
	}
	if ok && req.State == store.RoleAllowed {
		return nil
	}
	sections := allRoleSections
	if ok {
		sections = req.Sections
	}
	req, err = s.askRole(ctx, m, store.RoleReceiver, sections, store.RoleStoreRest)
	if err != nil {
		return err
	}
	if req.State != store.RoleAllowed {
		return errRoleNotAllowed
	}
	return nil
}

// revokeReceiverOf takes a member's login off the receiver and, when the
// member holds the Receiver role, revokes the role with it, so asking again
// does not bring the login back.
func (s *Service) revokeReceiverOf(ctx context.Context, memberID string) error {
	req, ok, err := s.store.FindRoleRequest(store.RoleRequestIn, memberID, store.RoleReceiver)
	if err != nil {
		return err
	}
	if ok && req.State == store.RoleAllowed {
		_, _, err = s.decideRole(ctx, req.ID, store.RoleDecision{State: store.RoleRevoked})
		return err
	}
	return s.RevokeReceiverLogin(ctx, memberID)
}

// fetcherRoleOpen reports why a pull from a member must not run: this
// instance's agreement to fetch for it does not stand, or the member does
// not let it fetch that section. A source with no request behind it runs.
func (s *Service) fetcherRoleOpen(memberID, domain string) error {
	if memberID == "" {
		return nil
	}
	req, ok, err := s.store.FindRoleRequest(store.RoleRequestIn, memberID, store.RoleFetcher)
	if err != nil || !ok {
		return err
	}
	if req.State != store.RoleAllowed {
		return errors.New("this instance does not fetch for that one at the moment: the Fetcher role is " + req.State)
	}
	if !slices.Contains(req.Sections, domain) {
		return errors.New("that instance does not let this one fetch its " + domain + " backups")
	}
	return nil
}
