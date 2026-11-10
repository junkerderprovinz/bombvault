package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// What the owner of a shared repository says to a member in its upkeep.
const (
	sharedHeld    = "held"
	sharedQuiet   = "quiet"
	sharedRelease = "release"
	sharedSpace   = "space"
)

// peerSharedBodyMax bounds a call about the upkeep. The measurement names
// every member, which a group keeps to a handful.
const peerSharedBodyMax = 64 << 10

var errNotSharedWithYou = errors.New("this server holds no shared repository of the asking one")

// peerSharedUpkeep is one call of an owner to a member about its upkeep.
type peerSharedUpkeep struct {
	Kind  string             `json:"kind"`
	Until int64              `json:"until,omitempty"`
	Space *store.SharedSpace `json:"space,omitempty"`
	peerProof
}

func (c peerSharedUpkeep) signed() []byte {
	space, _ := json.Marshal(c.Space) // a struct of plain fields always encodes
	return callPayload(c.Kind, strconv.FormatInt(c.Until, 10), string(space))
}

// peerSharedAnswer is a member's answer: the entries it holds, or whether a
// copy to the owner's repository still runs.
type peerSharedAnswer struct {
	OK   bool     `json:"ok"`
	Held []string `json:"held,omitempty"`
	Busy bool     `json:"busy,omitempty"`
}

// handlePeerSharedUpkeep answers the owner of a shared repository this server
// backs up into. Only the server that holds such a repository is heard, so
// no other member of the group can hold up this server's copies.
// POST /api/group/peer/shared-upkeep
func (s *Service) handlePeerSharedUpkeep(w http.ResponseWriter, r *http.Request) {
	var in peerSharedUpkeep
	if err := json.NewDecoder(io.LimitReader(r.Body, peerSharedBodyMax)).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "malformed upkeep call"})
		return
	}
	owner, ok := s.roleAsker(r)
	if !ok {
		refuseUnnamedPeer(w)
		return
	}
	if err := s.verifyMember(r.Context(), owner, keydeposit.ProofSharedUpkeep, in.signed(), in.peerProof); err != nil {
		writeJSON(w, http.StatusForbidden, failEnvelope(err))
		return
	}
	ans, err := s.answerSharedUpkeep(owner, in)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, ans)
}

// answerSharedUpkeep does what the owner owner asks of this server, whether
// it came through the group or from this server's own upkeep.
func (s *Service) answerSharedUpkeep(owner string, in peerSharedUpkeep) (peerSharedAnswer, error) {
	repos, err := s.reposSharedBy(owner)
	if err != nil {
		return peerSharedAnswer{}, err
	}
	if len(repos) == 0 {
		return peerSharedAnswer{}, errNotSharedWithYou
	}
	ans := peerSharedAnswer{OK: true}
	switch in.Kind {
	case sharedHeld:
		held, err := s.heldIdentityTags()
		if err != nil {
			return peerSharedAnswer{}, err
		}
		ans.Held = held.names()
	case sharedQuiet:
		ans.Busy = s.sharedCopies.hush(owner, repos, time.Unix(in.Until, 0))
	case sharedRelease:
		s.sharedCopies.lift(owner)
	case sharedSpace:
		if in.Space == nil {
			return peerSharedAnswer{}, errors.New("the measurement is missing")
		}
		err = s.store.SetSharedSpaceFrom(owner, *in.Space)
	default:
		err = errors.New("unknown upkeep call")
	}
	return ans, err
}

// reposSharedBy lists where this server reaches the shared repositories the
// server owner holds.
func (s *Service) reposSharedBy(owner string) ([]string, error) {
	all, err := s.store.ListSharedRepos()
	if err != nil {
		return nil, err
	}
	var repos []string
	for _, sr := range all {
		if sr.Owner == owner {
			repos = append(repos, sr.Repo)
		}
	}
	return repos, nil
}

// groupSharedMembers reaches the members of a shared repository through the
// group, and answers for the owner's own host itself.
type groupSharedMembers struct{ s *Service }

func (s *Service) groupMembers() sharedMembers { return groupSharedMembers{s} }

func (g groupSharedMembers) heldBy(ctx context.Context, m store.SharedMember) ([]string, error) {
	ans, err := g.ask(ctx, m, peerSharedUpkeep{Kind: sharedHeld})
	return ans.Held, err
}

func (g groupSharedMembers) quiet(ctx context.Context, m store.SharedMember, until time.Time) (bool, error) {
	ans, err := g.ask(ctx, m, peerSharedUpkeep{Kind: sharedQuiet, Until: until.Unix()})
	return ans.Busy, err
}

func (g groupSharedMembers) release(ctx context.Context, m store.SharedMember) {
	if _, err := g.ask(ctx, m, peerSharedUpkeep{Kind: sharedRelease}); err != nil {
		log.Printf("api: shared repository: %s did not hear that the prune is over, and goes on copying once its time is up: %v", m.Name, err) //nolint:gosec // G706: the name is the member's own
	}
}

func (g groupSharedMembers) tellSpace(ctx context.Context, m store.SharedMember, space store.SharedSpace) {
	if _, err := g.ask(ctx, m, peerSharedUpkeep{Kind: sharedSpace, Space: &space}); err != nil {
		log.Printf("api: shared repository: %s did not get the measurement: %v", m.Name, err) //nolint:gosec // G706: the name is the member's own
	}
}

func (g groupSharedMembers) ask(ctx context.Context, m store.SharedMember, in peerSharedUpkeep) (peerSharedAnswer, error) {
	gs, err := g.s.store.GetGroupState()
	if err != nil {
		return peerSharedAnswer{}, err
	}
	if m.Instance == gs.InstanceID {
		return g.s.answerSharedUpkeep(gs.InstanceID, in)
	}
	if in.peerProof, err = g.s.signCall(keydeposit.ProofSharedUpkeep, m.Instance, in.signed()); err != nil {
		return peerSharedAnswer{}, err
	}
	var ans peerSharedAnswer
	err = g.s.callMember(ctx, m.Instance, http.MethodPost, "/api/group/peer/shared-upkeep", in, &ans)
	return ans, err
}
