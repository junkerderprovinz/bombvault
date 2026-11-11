package api

import (
	"context"
	"errors"
	"slices"

	"github.com/junkerderprovinz/bombvault/internal/group"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

var errRoleNotAllowed = errors.New("that instance has not allowed this yet; the request waits on its Instances page")

var errPeerNotAllowed = errors.New("this instance has not allowed that for the asking instance")

var errPeerNotAsked = errors.New("this instance has not asked that one to keep or to fetch its backups")

// errMemberNotAsked is errPeerNotAsked as the instance on the other end says it.
var errMemberNotAsked = errors.New("that instance has not asked this one to keep or to fetch its backups. It asks on its own Instances page: switch on Receiver or Fetcher on this instance's card there")

var errPasswordHeld = errors.New("that instance has not released its restic password to this one: it first has to ask this instance to be its Receiver or Fetcher")

var errPasswordLocked = errors.New("that instance allows this, but cannot read its restic password right now: its backup key is locked there. Unlock it on that instance's Restore page under Emergency, then try again")

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

// memberAsked reports whether a member has a Receiver or Fetcher request with
// this instance that lets it hand out where its repositories are.
func (s *Service) memberAsked(memberID string) bool {
	for _, role := range []string{store.RoleReceiver, store.RoleFetcher} {
		req, ok, err := s.store.FindRoleRequest(store.RoleRequestIn, memberID, role)
		if err == nil && ok && (req.State == store.RoleAsked || req.State == store.RoleAllowed) {
			return true
		}
	}
	return false
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
