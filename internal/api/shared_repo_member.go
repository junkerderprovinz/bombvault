package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"sync"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

var (
	errNoSharedOffer    = errors.New("that instance runs no shared repository")
	errSharedNotAMember = errors.New("this instance is in no shared repository there")
)

// askShared asks the owner about its shared repository, with join set when a
// person here asks to join it.
func (s *Service) askShared(ctx context.Context, ownerID string, join bool) (peerSharedOffer, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return peerSharedOffer{}, err
	}
	ask := peerSharedAsk{Name: instanceDisplayName(settings), Host: s.resticHost(), Join: join}
	if ask.peerProof, err = s.signCall(keydeposit.ProofSharedJoin, ownerID, ask.signed()); err != nil {
		return peerSharedOffer{}, err
	}
	var ans peerSharedOffer
	if err := s.callMember(ctx, ownerID, http.MethodPost, "/api/group/peer/shared-repo/join", ask, &ans); err != nil {
		return peerSharedOffer{}, err
	}
	return ans, nil
}

// JoinSharedRepo asks the owner to let this server into its shared repository
// and returns where the ask stands. One the owner allowed already is finished
// at once.
func (s *Service) JoinSharedRepo(ctx context.Context, ownerID string) (string, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return "", err
	}
	if !settings.EncryptionEnabled {
		return "", errSharedNeedsEncryption
	}
	owner, ok := s.reachableMember(ownerID)
	if !ok {
		return "", errMemberGone
	}
	ans, err := s.askShared(ctx, ownerID, true)
	if err != nil {
		return "", err
	}
	if ans.Server == nil {
		return "", errNoSharedOffer
	}
	s.sharedMu.Lock()
	j, _, err := s.store.GetSharedJoin(store.SharedJoinOut, ownerID)
	if err == nil {
		j.Direction, j.MemberID, j.MemberName, j.Host = store.SharedJoinOut, ownerID, owner.Name, s.resticHost()
		if j.State != store.SharedJoinJoined || ans.State != store.SharedJoinJoined {
			j.State = ans.State
		}
		err = s.store.SaveSharedJoin(j)
	}
	s.sharedMu.Unlock()
	if err != nil {
		return "", err
	}
	if ans.State == store.SharedJoinAllowed {
		if err := s.finishSharedJoin(ctx, ownerID); err != nil {
			return ans.State, err
		}
		return store.SharedJoinJoined, nil
	}
	return ans.State, nil
}

// finishSharedJoin adds this server's own key with the one-time key the owner
// handed out, takes up the repository as a copy target for every section,
// switched off until a person picks the sections that go there, and tells the
// owner, which then removes the one-time key. A step that went through before
// is not repeated.
func (s *Service) finishSharedJoin(ctx context.Context, ownerID string) error {
	s.sharedMu.Lock()
	defer s.sharedMu.Unlock()
	j, ok, err := s.store.GetSharedJoin(store.SharedJoinOut, ownerID)
	if err != nil {
		return err
	}
	if !ok || j.State == store.SharedJoinJoined || j.State == store.SharedJoinRemoved || j.State == store.SharedJoinExpired {
		return nil
	}
	if j.Repo == "" {
		if err := s.addOwnSharedKey(ctx, ownerID, &j); err != nil {
			return err
		}
	}
	proof, err := s.signCall(keydeposit.ProofSharedJoined, ownerID, callPayload(j.Host))
	if err != nil {
		return err
	}
	if err := s.callMember(ctx, ownerID, http.MethodPost, "/api/group/peer/shared-repo/joined", peerSharedWord{Host: j.Host, peerProof: proof}, nil); err != nil {
		return err
	}
	j.State = store.SharedJoinJoined
	return s.store.SaveSharedJoin(j)
}

func (s *Service) addOwnSharedKey(ctx context.Context, ownerID string, j *store.SharedJoin) error {
	owner, ok := s.reachableMember(ownerID)
	if !ok {
		return errMemberGone
	}
	ans, err := s.askShared(ctx, ownerID, false)
	if err != nil {
		return err
	}
	if ans.Server == nil {
		return errNoSharedOffer
	}
	if ans.State != store.SharedJoinAllowed || ans.Once == "" || ans.User == "" || ans.Login == "" {
		j.State = ans.State
		return s.store.SaveSharedJoin(*j)
	}
	at := groupReceiverFor(owner, *ans.Server)
	switch {
	case at.NeedsHost:
		return errReceiverNeedsHost
	case at.NeedsAddress:
		return errReceiverNeedsAddress
	}
	repo, err := sharedRepoLocation(at, ans.Path)
	if err != nil {
		return err
	}
	eng, err := s.sharedEngine()
	if err != nil {
		return err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return err
	}
	signIn := cloudEnv(CloudCreds{RESTUser: ans.User, RESTPassword: ans.Login})
	once := restic.Mode{Encrypted: true, Password: ans.Once, Env: signIn, Host: s.resticHost(), Sharing: restic.SharingMember}
	if err := eng.AddKey(ctx, repo, once, s.backupPassword(), s.resticHost(), defaultReceiverUser(instanceDisplayName(settings))); err != nil {
		return fmt.Errorf("add this server's key: %w", err)
	}
	own := once
	own.Password = s.backupPassword()
	if err := s.engine.RepoOpensErr(ctx, repo, own); err != nil {
		return fmt.Errorf("this server's key does not open the shared repository: %w", err)
	}
	loginEnc, err := secret.Encrypt(s.cfg.AppKey, []byte(ans.Login))
	if err != nil {
		return err
	}
	if err := s.takeUpSharedRepo(repo, ownerID, owner.Name, ans); err != nil {
		return err
	}
	j.Repo, j.User, j.LoginEnc, j.OnceEnc, j.DecidedAt = repo, ans.User, loginEnc, nil, time.Now().Unix()
	return s.store.SaveSharedJoin(*j)
}

// sharedFolderRe is a folder name the owner may give its shared repository.
var sharedFolderRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// sharedRepoLocation is where restic reaches the shared repository the owner
// named at, refused unless the address is a bare server and the folder a
// plain name, since this server hands its own password to whatever is there.
func sharedRepoLocation(at groupReceiverView, folder string) (string, error) {
	u, err := url.Parse(at.URL)
	if err != nil || u.Scheme != "http" || !receiverHostRe.MatchString(u.Hostname()) || u.Port() == "" || (u.Path != "" && u.Path != "/") || u.User != nil || u.RawQuery != "" {
		return "", errors.New("the owner named no usable address for its shared repository")
	}
	if !sharedFolderRe.MatchString(folder) || slices.Contains(restReservedFolders, folder) {
		return "", errors.New("the owner named no usable folder for its shared repository")
	}
	repo := "rest:http://" + u.Host + "/" + folder
	return repo, restic.CheckLocation(repo)
}

// takeUpSharedRepo records the shared repository with its members and gives
// every section a copy target there under the member's own login. A target is
// append-only and has no keep policy: only the owner forgets there.
func (s *Service) takeUpSharedRepo(repo, ownerID, ownerName string, ans peerSharedOffer) error {
	sr, _, err := s.store.SharedRepoAt(repo)
	if err != nil {
		return err
	}
	g, err := s.store.GetGroupState()
	if err != nil {
		return err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return err
	}
	sr.Repo, sr.Owner, sr.Members = repo, ownerID, nil
	for _, m := range ans.Members {
		if m.MemberID != g.InstanceID && m.Host != s.resticHost() {
			sr.Members = append(sr.Members, store.SharedMember{Host: m.Host, Instance: m.MemberID, Name: m.Name})
		}
	}
	sr.Members = append(sr.Members, store.SharedMember{Host: s.resticHost(), Instance: g.InstanceID, Name: instanceDisplayName(settings)})
	if _, err := s.store.PutSharedRepo(sr); err != nil {
		return err
	}
	name := "Shared with " + ownerName
	ref, err := s.addCredSet(name, CloudCreds{RESTUser: ans.User, RESTPassword: ans.Login})
	if err != nil {
		return err
	}
	targets, err := s.store.ListOffsiteTargets()
	if err != nil {
		return err
	}
	for _, domain := range offsiteConfigDomains {
		if i := slices.IndexFunc(targets, func(t store.OffsiteTarget) bool { return t.Domain == domain && t.Repo == repo }); i >= 0 {
			t := targets[i]
			t.CredsRef, t.Immutable = ref, true
			if _, err := s.store.UpsertOffsiteTarget(t); err != nil {
				return err
			}
			continue
		}
		if _, err := s.store.CreateOffsiteTarget(store.OffsiteTarget{
			Domain: domain, Name: name, Repo: repo, CredsRef: ref, Immutable: true, Enabled: false,
		}); err != nil {
			return err
		}
	}
	return nil
}

// LeaveSharedRepo takes this server out of the owner's shared repository: the
// owner removes its key and login. Its copy targets there are switched off.
func (s *Service) LeaveSharedRepo(ctx context.Context, ownerID string) error {
	s.sharedMu.Lock()
	defer s.sharedMu.Unlock()
	j, ok, err := s.store.GetSharedJoin(store.SharedJoinOut, ownerID)
	if err != nil {
		return err
	}
	if !ok {
		return errSharedNotAMember
	}
	if _, reachable := s.reachableMember(ownerID); !reachable {
		return errMemberGone
	}
	proof, err := s.signCall(keydeposit.ProofSharedLeave, ownerID, callPayload(j.Host))
	if err != nil {
		return err
	}
	if err := s.callMember(ctx, ownerID, http.MethodPost, "/api/group/peer/shared-repo/leave", peerSharedWord{Host: j.Host, peerProof: proof}, nil); err != nil {
		return err
	}
	return s.leftSharedRepo(j, store.SharedJoinRemoved)
}

// leftSharedRepo notes that this server is out of the shared repository, as
// state says. The repository stays recorded with this server's part as a
// member's, so nothing here ever clears its locks or forgets there.
func (s *Service) leftSharedRepo(j store.SharedJoin, state string) error {
	if j.Repo != "" {
		targets, err := s.store.ListOffsiteTargets()
		if err != nil {
			return err
		}
		for _, t := range targets {
			if t.Repo == j.Repo && t.Enabled {
				t.Enabled = false
				if _, err := s.store.UpsertOffsiteTarget(t); err != nil {
					return err
				}
			}
		}
		sr, ok, err := s.store.SharedRepoAt(j.Repo)
		if err != nil {
			return err
		}
		if ok {
			sr.Members = slices.DeleteFunc(sr.Members, func(m store.SharedMember) bool { return m.Host == j.Host })
			if _, err := s.store.PutSharedRepo(sr); err != nil {
				return err
			}
		}
	}
	j.State, j.LoginEnc, j.OnceEnc, j.DecidedAt = state, nil, nil, time.Now().Unix()
	return s.store.SaveSharedJoin(j)
}

// handlePeerSharedAnswer takes the owner's answer to this server's ask. An
// allow is finished in the background, since it reads the key over the
// network.
// POST /api/group/peer/shared-repo/answer
func (s *Service) handlePeerSharedAnswer(w http.ResponseWriter, r *http.Request) {
	var in peerSharedNote
	if !decodePeerRole(w, r, &in) {
		return
	}
	owner, ok := s.roleAsker(r)
	if !ok {
		refuseUnnamedPeer(w)
		return
	}
	if err := s.verifyMember(r.Context(), owner, keydeposit.ProofSharedAnswer, callPayload(in.State), in.peerProof); err != nil {
		writeJSON(w, http.StatusForbidden, failEnvelope(err))
		return
	}
	if err := s.takeSharedAnswer(owner, in.State); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

func (s *Service) takeSharedAnswer(ownerID, state string) error {
	s.sharedMu.Lock()
	defer s.sharedMu.Unlock()
	j, ok, err := s.store.GetSharedJoin(store.SharedJoinOut, ownerID)
	if err != nil {
		return err
	}
	if !ok {
		return errSharedNotAMember
	}
	switch state {
	case store.SharedJoinAllowed:
		j.State = state
		if err := s.store.SaveSharedJoin(j); err != nil {
			return err
		}
		go s.finishSharedJoinLater(ownerID)
		return nil
	case store.SharedJoinRefused:
		j.State, j.DecidedAt = state, time.Now().Unix()
		return s.store.SaveSharedJoin(j)
	case store.SharedJoinRemoved, store.SharedJoinExpired:
		return s.leftSharedRepo(j, state)
	}
	return fmt.Errorf("unknown answer %q", state)
}

func (s *Service) finishSharedJoinLater(ownerID string) {
	ctx, cancel := context.WithTimeout(s.StopContext(), 5*time.Minute)
	defer cancel()
	if err := s.finishSharedJoin(ctx, ownerID); err != nil {
		log.Printf("shared repository: finish joining %q: %v", ownerID, scrubError(err)) //nolint:gosec // G706: an instance id from the store, the error scrubbed
	}
}

// finishSharedJoinsSoon picks up an allowed join whose finish broke off, as
// when the owner's answer arrived while this server was away.
func (s *Service) finishSharedJoinsSoon() {
	now := time.Now().UnixNano()
	last := s.sharedRefreshedAt.Load()
	if now-last < int64(roleRefreshEvery) || !s.sharedRefreshedAt.CompareAndSwap(last, now) {
		return
	}
	joins, err := s.store.ListSharedJoins(store.SharedJoinOut)
	if err != nil {
		return
	}
	for _, j := range joins {
		if j.State == store.SharedJoinAllowed || (j.State == store.SharedJoinAsked && j.Repo == "") {
			if _, ok := s.reachableMember(j.MemberID); ok {
				go s.refreshSharedJoin(j.MemberID)
			}
		}
	}
}

// refreshSharedJoin asks the owner where an ask stands and finishes it once
// allowed.
func (s *Service) refreshSharedJoin(ownerID string) {
	ctx, cancel := context.WithTimeout(s.StopContext(), 5*time.Minute)
	defer cancel()
	ans, err := s.askShared(ctx, ownerID, false)
	if err != nil || ans.Server == nil || ans.State == "" {
		return
	}
	if ans.State == store.SharedJoinAllowed || ans.State == store.SharedJoinRefused || ans.State == store.SharedJoinExpired {
		if err := s.takeSharedAnswer(ownerID, ans.State); err != nil {
			log.Printf("shared repository: %v", err)
		}
	}
}

// sharedMembershipView is a shared repository this server asked to join or
// copies into, as its card shows it.
type sharedMembershipView struct {
	OwnerID   string `json:"ownerId"`
	OwnerName string `json:"ownerName"`
	State     string `json:"state"`
	// Repo is where the repository is reached, without the login.
	Repo     string             `json:"repo,omitempty"`
	AskedAt  int64              `json:"askedAt"`
	JoinedAt int64              `json:"joinedAt,omitempty"`
	Together []sharedMemberView `json:"together"`
	// LastCopyAt is when the last copy there that went through began, and
	// DeferredAt when a later one began that waited for the owner's prune.
	LastCopyAt int64 `json:"lastCopyAt,omitempty"`
	DeferredAt int64 `json:"deferredAt,omitempty"`
	// Space is what the owner last measured, NextWindow when it said its
	// next prune window opens.
	Space      *store.SharedSpace `json:"space,omitempty"`
	NextWindow int64              `json:"nextWindow,omitempty"`
}

func (s *Service) sharedMemberships() ([]sharedMembershipView, error) {
	joins, err := s.store.ListSharedJoins(store.SharedJoinOut)
	if err != nil {
		return nil, err
	}
	targets, err := s.store.ListOffsiteTargets()
	if err != nil {
		return nil, err
	}
	out := []sharedMembershipView{}
	for _, j := range joins {
		v := sharedMembershipView{OwnerID: j.MemberID, OwnerName: j.MemberName, State: j.State, Repo: scrubRepoLocation(j.Repo), AskedAt: j.CreatedAt, Together: []sharedMemberView{}}
		if j.State == store.SharedJoinJoined {
			v.JoinedAt = j.DecidedAt
			if v.Space, v.NextWindow, err = s.sharedSpaceHeard(j.MemberID); err != nil {
				return nil, err
			}
		}
		if m, ok := s.reachableMember(j.MemberID); ok && m.Name != "" {
			v.OwnerName = m.Name
		}
		if j.Repo != "" {
			if sr, ok, err := s.store.SharedRepoAt(j.Repo); err == nil && ok {
				for _, m := range sr.Members {
					if m.Host != j.Host {
						v.Together = append(v.Together, sharedMemberView{MemberID: m.Instance, Name: m.Name, Host: m.Host})
					}
				}
			}
			var ids []string
			for _, t := range targets {
				if t.Repo == j.Repo {
					ids = append(ids, t.ID)
				}
			}
			if v.LastCopyAt, v.DeferredAt, err = s.store.CopiesToTargets(ids); err != nil {
				return nil, err
			}
		}
		out = append(out, v)
	}
	return out, nil
}

// sharedOfferView is a member's shared repository as the destination wizard
// offers it.
type sharedOfferView struct {
	groupReceiverView
	State string `json:"state,omitempty"`
}

// SharedRepoOffers asks every instance of the group whether it runs a shared
// repository. A member that does not answer is left out.
func (s *Service) SharedRepoOffers(ctx context.Context) []sharedOfferView {
	members := s.pairing().Members()
	found := make([]*sharedOfferView, len(members))
	var wg sync.WaitGroup
	for i, m := range members {
		if m.Kind != "" {
			continue
		}
		wg.Go(func() {
			cctx, cancel := context.WithTimeout(ctx, groupReceiverWait)
			defer cancel()
			ans, err := s.askShared(cctx, m.ID, false)
			if err != nil || ans.Server == nil {
				return
			}
			found[i] = &sharedOfferView{groupReceiverView: groupReceiverFor(m, *ans.Server), State: ans.State}
		})
	}
	wg.Wait()
	out := []sharedOfferView{}
	for _, v := range found {
		if v != nil {
			out = append(out, *v)
		}
	}
	return out
}
