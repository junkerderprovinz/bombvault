package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"slices"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// sharedHolderEvery is how often a new holder tries again to reach the
// servers that have not heard of it yet.
const sharedHolderEvery = 5 * time.Minute

// sharedProofUser is the user of the throwaway key a server adds to a shared
// repository to check a server that says it holds the repository now. Every
// key can be added through the append-only server, and only a server at the
// repository's folder can remove one, which is the check.
const sharedProofUser = "bombvault-holder-check"

// sharedProofKeyAge is how old a key of a check has to be before the holder's
// upkeep removes it, so that a check still running keeps its key.
var sharedProofKeyAge = time.Hour

// sharedCheckTTL is how long a check stays open. It ends well before the
// upkeep may remove its key.
const sharedCheckTTL = 30 * time.Minute

// holderCheck is the key a server that says it holds a shared repository has
// to remove, and when it was added.
type holderCheck struct {
	key string
	at  time.Time
}

// restic writes the time a key was made in this layout.
const resticKeyTime = "2006-01-02 15:04:05"

var (
	errSharedRepoIDUnread = errors.New("this server could not read the id of its shared repositories")
	errNoSharedRepoWithID = errors.New("this server takes part in no shared repository with that id")
)

// repoIdentifier reads the id restic gave a repository.
type repoIdentifier interface {
	RepoID(ctx context.Context, repo string, m restic.Mode) (string, error)
}

// peerSharedHolder is a server's word that it holds the shared repository
// with RepoID now. Every key opens the repository's config, so a server that
// reads the same id with its own key knows the word is about its repository.
// Removed is the key of the check the server removed at the folder.
type peerSharedHolder struct {
	RepoID  string `json:"repoId"`
	Removed string `json:"removed,omitempty"`
	peerProof
}

func (c peerSharedHolder) signed() []byte { return callPayload(c.RepoID, c.Removed) }

// peerSharedHolderAnswer says whether the word was taken. Challenge is the key
// the server has to remove first. Later asks to be told again, when the
// repository could not be reached just then. OK says the call was heard.
type peerSharedHolderAnswer struct {
	OK        bool   `json:"ok"`
	Taken     bool   `json:"taken,omitempty"`
	Challenge string `json:"challenge,omitempty"`
	Later     bool   `json:"later,omitempty"`
	Error     string `json:"error,omitempty"`
}

// sharedRepoID reads the id of sr the way this server reaches it.
func (s *Service) sharedRepoID(ctx context.Context, sr store.SharedRepo) (string, error) {
	ider, ok := engineAs[repoIdentifier](s.engine)
	if !ok {
		return "", errors.New("this build of restic cannot read a repository's id")
	}
	repo, mode, err := s.sharedReach(sr)
	if err != nil {
		return "", err
	}
	mode.NoLock = true
	return ider.RepoID(ctx, repo, mode)
}

// handlePeerSharedHolder takes the word of a server of the group that it
// holds a shared repository now.
// POST /api/group/peer/shared-holder
func (s *Service) handlePeerSharedHolder(w http.ResponseWriter, r *http.Request) {
	var in peerSharedHolder
	if err := json.NewDecoder(io.LimitReader(r.Body, peerSharedBodyMax)).Decode(&in); err != nil || in.RepoID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "malformed holder call"})
		return
	}
	holder, ok := s.roleAsker(r)
	if !ok {
		refuseUnnamedPeer(w)
		return
	}
	if err := s.verifyMember(r.Context(), holder, keydeposit.ProofSharedHolder, in.signed(), in.peerProof); err != nil {
		writeJSON(w, http.StatusForbidden, failEnvelope(err))
		return
	}
	ans, err := s.takeSharedHolder(r.Context(), holder, in)
	switch {
	case errors.Is(err, errSharedRepoIDUnread):
		ans = peerSharedHolderAnswer{Later: true, Error: err.Error()}
	case err != nil:
		ans = peerSharedHolderAnswer{Error: err.Error()}
	}
	ans.OK = true
	writeJSON(w, http.StatusOK, ans)
}

// takeSharedHolder makes holder the holder of the shared repository whose id
// this server reads as in.RepoID, once holder removed the key of a check that
// only a server at the repository's folder can remove. A server that held it
// until then stops keeping it and remembers who took it over, so a person can
// take it back.
func (s *Service) takeSharedHolder(ctx context.Context, holder string, in peerSharedHolder) (peerSharedHolderAnswer, error) {
	sr, err := s.sharedRepoWithID(ctx, holder, in.RepoID)
	if err != nil {
		return peerSharedHolderAnswer{}, err
	}
	if sr.Owner == holder {
		return peerSharedHolderAnswer{Taken: true}, nil
	}
	check := sr.ID + "\x00" + holder
	var pending holderCheck
	if v, ok := s.holderChecks.Load(check); ok {
		pending = v.(holderCheck)
	}
	// A check runs out well before the holder's upkeep may remove its key, so
	// a key that is gone was removed by whoever was asked to.
	if pending.key != "" && time.Since(pending.at) >= sharedCheckTTL {
		s.holderChecks.Delete(check)
		s.dropProofKey(ctx, sr, pending.key)
		pending = holderCheck{}
	}
	if pending.key == "" {
		key, err := s.addProofKey(ctx, sr)
		if err != nil {
			return peerSharedHolderAnswer{Later: true, Error: scrubError(err)}, nil
		}
		s.holderChecks.Store(check, holderCheck{key: key, at: time.Now()})
		return peerSharedHolderAnswer{Challenge: key}, nil
	}
	// Asking again yields the same check, so a server that cannot pass it
	// adds no more keys to the repository than one.
	if in.Removed != pending.key {
		return peerSharedHolderAnswer{Challenge: pending.key}, nil
	}
	still, err := s.hasKey(ctx, sr, pending.key)
	if err != nil {
		return peerSharedHolderAnswer{Later: true, Error: scrubError(err)}, nil
	}
	if still {
		if s.dropProofKey(ctx, sr, pending.key) {
			s.holderChecks.Delete(check)
		}
		log.Printf("api: shared repository: %s says it holds %s, but the key of the check is still there, so nothing changes", holder, scrubRepoLocation(sr.Repo)) //nolint:gosec // G706: an instance id the group channel checked, the location scrubbed
		return peerSharedHolderAnswer{Error: "the key of the check is still there, so only a server at the repository's folder holds it"}, nil
	}
	s.holderChecks.Delete(check)
	if s.ownsSharedRepo(sr) {
		name := holder
		if m, ok := s.reachableMember(holder); ok && m.Name != "" {
			name = m.Name
		}
		if err := s.store.NoteSharedTakenOver(store.SharedTakenOver{RepoID: sr.ID, By: holder, Name: name, At: time.Now().Unix()}); err != nil {
			return peerSharedHolderAnswer{}, err
		}
		log.Printf("api: shared repository: %s holds %s now, so this server stops keeping it", name, scrubRepoLocation(sr.Repo)) //nolint:gosec // G706: the name is the member's own, the location scrubbed
	}
	sr.Owner = holder
	if _, err := s.store.PutSharedRepo(sr); err != nil {
		return peerSharedHolderAnswer{}, err
	}
	return peerSharedHolderAnswer{Taken: true}, nil
}

// sharedRepoWithID finds the shared repository whose id this server reads as
// id, other than one holder holds already.
func (s *Service) sharedRepoWithID(ctx context.Context, holder, id string) (store.SharedRepo, error) {
	all, err := s.store.ListSharedRepos()
	if err != nil {
		return store.SharedRepo{}, err
	}
	unread := false
	for _, sr := range all {
		got, err := s.sharedRepoID(ctx, sr)
		if err != nil {
			log.Printf("api: shared repository: could not read the id of %s: %v", scrubRepoLocation(sr.Repo), scrubError(err)) //nolint:gosec // G706: the location and the error are scrubbed
			unread = true
			continue
		}
		if got == id {
			return sr, nil
		}
	}
	if unread {
		return store.SharedRepo{}, errSharedRepoIDUnread
	}
	log.Printf("api: shared repository: %s says it holds a repository with an id none of this server's has, so nothing changes", holder) //nolint:gosec // G706: an instance id the group channel checked
	return store.SharedRepo{}, errNoSharedRepoWithID
}

// addProofKey adds a key with a password nobody keeps to sr, the way this
// server reaches it, and returns its id.
func (s *Service) addProofKey(ctx context.Context, sr store.SharedRepo) (string, error) {
	eng, err := s.sharedEngine()
	if err != nil {
		return "", err
	}
	repo, mode, err := s.sharedReach(sr)
	if err != nil {
		return "", err
	}
	password, err := randomDeployPassword()
	if err != nil {
		return "", err
	}
	if err := eng.AddKey(ctx, repo, mode, password, s.resticHost(), sharedProofUser); err != nil {
		return "", err
	}
	probe := mode
	probe.Password = password
	keys, err := eng.Keys(ctx, repo, probe)
	if err != nil {
		return "", err
	}
	if i := slices.IndexFunc(keys, func(k restic.Key) bool { return k.Current }); i >= 0 {
		return keys[i].ID, nil
	}
	return "", errors.New("restic did not name the key it added")
}

// hasKey reports whether the key id is in sr.
func (s *Service) hasKey(ctx context.Context, sr store.SharedRepo, id string) (bool, error) {
	eng, err := s.sharedEngine()
	if err != nil {
		return false, err
	}
	repo, mode, err := s.sharedReach(sr)
	if err != nil {
		return false, err
	}
	keys, err := eng.Keys(ctx, repo, mode)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(keys, func(k restic.Key) bool { return k.ID == id }), nil
}

// dropProofKey removes the key of a check this server made, which works only
// at the repository's folder. Elsewhere the key stays until the holder's
// upkeep removes it. It reports whether the key is gone.
func (s *Service) dropProofKey(ctx context.Context, sr store.SharedRepo, id string) bool {
	eng, err := s.sharedEngine()
	if err != nil {
		return false
	}
	repo, mode, err := s.sharedReach(sr)
	if err == nil {
		err = eng.RemoveKey(ctx, repo, mode, id)
	}
	if err != nil {
		log.Printf("api: shared repository: the key %s of a check stays in %s until the holder's upkeep removes it: %v", id, scrubRepoLocation(sr.Repo), scrubError(err)) //nolint:gosec // G706: a key id restic named, the location and the error scrubbed
		return false
	}
	return true
}

// removeProofKey removes the key of the check instance asked this server to
// remove, at the folder of sr. It removes nothing but a key of a check, and
// for a member only one under the member's own host.
func (s *Service) removeProofKey(ctx context.Context, sr store.SharedRepo, instance, id string) error {
	eng, err := s.sharedEngine()
	if err != nil {
		return err
	}
	dest, mode, _, err := s.sharedAccess(sr)
	if err != nil {
		return err
	}
	keys, err := eng.Keys(ctx, dest, mode)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(keys, func(k restic.Key) bool { return k.ID == id })
	if i < 0 {
		return nil
	}
	k := keys[i]
	host := ""
	if j := slices.IndexFunc(sr.Members, func(m store.SharedMember) bool { return m.Instance == instance }); j >= 0 {
		host = sr.Members[j].Host
	}
	if k.UserName != sharedProofUser || (host != "" && k.HostName != host) {
		return fmt.Errorf("key %s is not the key of a check by that server", id)
	}
	return eng.RemoveKey(ctx, dest, mode, id)
}

// clearProofKeys removes the keys of checks that were left in the shared
// repositories this server holds, where a member could not remove them
// through the append-only server.
func (s *Service) clearProofKeys(ctx context.Context) {
	eng, err := s.sharedEngine()
	if err != nil {
		return
	}
	owned, err := s.ownedSharedRepos()
	if err != nil {
		return
	}
	for _, sr := range owned {
		dest, mode, _, err := s.sharedAccess(sr)
		if err != nil {
			continue
		}
		keys, err := eng.Keys(ctx, dest, mode)
		if err != nil {
			continue
		}
		for _, k := range keys {
			if !leftProofKey(k, time.Now()) {
				continue
			}
			if err := eng.RemoveKey(ctx, dest, mode, k.ID); err != nil {
				log.Printf("api: shared repository: remove the key %s of a check: %v", k.ID, scrubError(err)) //nolint:gosec // G706: a key id restic named, the error scrubbed
			}
		}
	}
}

// leftProofKey reports whether k is the key of a check old enough that no
// check uses it any more. A time restic wrote in another layout counts as old.
func leftProofKey(k restic.Key, now time.Time) bool {
	if k.UserName != sharedProofUser || k.Current {
		return false
	}
	made, err := time.ParseInLocation(resticKeyTime, k.Created, time.Local)
	return err != nil || now.Sub(made) >= sharedProofKeyAge
}

// clearProofKeysSoon clears the keys of checks at most once an hour.
func (s *Service) clearProofKeysSoon(ctx context.Context) {
	now := time.Now().UnixNano()
	last := s.sharedProofsClearedAt.Load()
	if now-last < int64(time.Hour) || !s.sharedProofsClearedAt.CompareAndSwap(last, now) {
		return
	}
	s.clearProofKeys(ctx)
}

// tellSharedHolders tells every server still to hear of it that this server
// holds the shared repository it names. One that is away or could not look
// is told again later, one that refuses is not.
func (s *Service) tellSharedHolders(ctx context.Context) {
	notices, err := s.store.SharedHolderNotices()
	if err != nil || len(notices) == 0 {
		return
	}
	all, err := s.store.ListSharedRepos()
	if err != nil {
		return
	}
	ids := map[string]string{}
	for _, n := range notices {
		i := slices.IndexFunc(all, func(sr store.SharedRepo) bool { return sr.ID == n.RepoID })
		if i < 0 || !s.ownsSharedRepo(all[i]) {
			s.dropSharedHolderNotice(n)
			continue
		}
		id, ok := ids[n.RepoID]
		if !ok {
			if id, err = s.sharedRepoID(ctx, all[i]); err != nil {
				log.Printf("api: shared repository: could not read the id of %s, so the others hear of this server later: %v", scrubRepoLocation(all[i].Repo), scrubError(err)) //nolint:gosec // G706: the location and the error are scrubbed
				continue
			}
			ids[n.RepoID] = id
		}
		ans, err := s.tellSharedHolder(ctx, all[i], n.Instance, id)
		switch {
		case err != nil || ans.Later:
			continue
		case !ans.Taken:
			log.Printf("api: shared repository: %s did not take this server as the holder: %q", n.Instance, ans.Error) //nolint:gosec // G706: an instance id from the store and the member's own answer
		}
		s.dropSharedHolderNotice(n)
	}
}

// tellSharedHolder tells instance that this server holds sr, whose id it
// reads as id, and removes the key of the check instance answers with at the
// folder before it says so again.
func (s *Service) tellSharedHolder(ctx context.Context, sr store.SharedRepo, instance, id string) (peerSharedHolderAnswer, error) {
	ans, err := s.announceHolder(ctx, instance, id, "")
	if err != nil || ans.Challenge == "" {
		return ans, err
	}
	if err := s.removeProofKey(ctx, sr, instance, ans.Challenge); err != nil {
		log.Printf("api: shared repository: could not remove the key of the check %s asked for, so it hears of this server later: %v", instance, scrubError(err)) //nolint:gosec // G706: an instance id from the store, the error scrubbed
		return peerSharedHolderAnswer{Later: true}, nil
	}
	return s.announceHolder(ctx, instance, id, ans.Challenge)
}

func (s *Service) announceHolder(ctx context.Context, instance, id, removed string) (peerSharedHolderAnswer, error) {
	in := peerSharedHolder{RepoID: id, Removed: removed}
	var err error
	if in.peerProof, err = s.signCall(keydeposit.ProofSharedHolder, instance, in.signed()); err != nil {
		return peerSharedHolderAnswer{}, err
	}
	var ans peerSharedHolderAnswer
	err = s.callMember(ctx, instance, http.MethodPost, "/api/group/peer/shared-holder", in, &ans)
	return ans, err
}

func (s *Service) dropSharedHolderNotice(n store.SharedHolderNotice) {
	if err := s.store.DropSharedHolderNotice(n.RepoID, n.Instance); err != nil {
		log.Printf("api: shared repository: %v", err)
	}
}

// tellSharedHoldersSoon tells the servers still to hear of this server as a
// holder, at most every few minutes, those of them the group reaches.
func (s *Service) tellSharedHoldersSoon() {
	now := time.Now().UnixNano()
	last := s.sharedHolderToldAt.Load()
	if now-last < int64(sharedHolderEvery) || !s.sharedHolderToldAt.CompareAndSwap(last, now) {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(s.StopContext(), 5*time.Minute)
		defer cancel()
		s.tellSharedHolders(ctx)
	}()
}

// sharedHolding is this server's part as a holder in the shared repository
// its card shows: who took it over, and who has still to hear that this
// server holds it.
type sharedHolding struct {
	// ProofKeys counts the keys of checks still in a repository this server
	// holds, which its upkeep removes.
	ProofKeys   int      `json:"proofKeys,omitempty"`
	Repo        string   `json:"repo"`
	TakenBy     string   `json:"takenBy,omitempty"`
	TakenAt     int64    `json:"takenAt,omitempty"`
	StillToTell []string `json:"stillToTell,omitempty"`
}

// sharedHoldings lists what the holder's card says about each shared
// repository this server holds or held.
func (s *Service) sharedHoldings(ctx context.Context) ([]sharedHolding, error) {
	all, err := s.store.ListSharedRepos()
	if err != nil {
		return nil, err
	}
	notices, err := s.store.SharedHolderNotices()
	if err != nil {
		return nil, err
	}
	out := []sharedHolding{}
	for _, sr := range all {
		h := sharedHolding{Repo: sr.ID}
		if t, ok, err := s.store.SharedTakenOver(sr.ID); err != nil {
			return nil, err
		} else if ok {
			h.TakenBy, h.TakenAt = t.Name, t.At
		}
		for _, n := range notices {
			if n.RepoID != sr.ID {
				continue
			}
			name := n.Instance
			if i := slices.IndexFunc(sr.Members, func(m store.SharedMember) bool { return m.Instance == n.Instance && m.Name != "" }); i >= 0 {
				name = sr.Members[i].Name
			} else if m, ok := s.reachableMember(n.Instance); ok && m.Name != "" {
				name = m.Name
			}
			h.StillToTell = append(h.StillToTell, name)
		}
		if s.ownsSharedRepo(sr) {
			h.ProofKeys = s.proofKeysIn(ctx, sr)
		}
		if h.TakenBy != "" || len(h.StillToTell) > 0 || h.ProofKeys > 0 {
			out = append(out, h)
		}
	}
	return out, nil
}

// TakeBackSharedRepo makes this server the holder of the shared repository
// id again, after another server said it took it over, and tells the members
// and that server so.
func (s *Service) TakeBackSharedRepo(ctx context.Context, id string) error {
	taken, ok, err := s.store.SharedTakenOver(id)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("nobody took this shared repository over")
	}
	all, err := s.store.ListSharedRepos()
	if err != nil {
		return err
	}
	i := slices.IndexFunc(all, func(sr store.SharedRepo) bool { return sr.ID == id })
	if i < 0 {
		return errSharedRepoUnknown
	}
	g, err := s.store.GetGroupState()
	if err != nil {
		return err
	}
	sr := all[i]
	sr.Owner = g.InstanceID
	if _, err := s.store.PutSharedRepo(sr); err != nil {
		return err
	}
	if err := s.store.ClearSharedTakenOver(id); err != nil {
		return err
	}
	if err := s.store.QueueSharedHolderNotices(id, sharedHolderAudience(taken.By, sr.Members, g.InstanceID)); err != nil {
		return err
	}
	s.tellSharedHolders(ctx)
	return nil
}

// handleTakeBackSharedRepo serves POST /api/shared-repos/{id}/take-back.
func (h *Handler) handleTakeBackSharedRepo(w http.ResponseWriter, r *http.Request) {
	if !crossOriginGuard(w, r) {
		return
	}
	if err := h.svc.TakeBackSharedRepo(r.Context(), r.PathValue("id")); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// proofKeysIn counts the keys of checks in sr, which this server holds.
func (s *Service) proofKeysIn(ctx context.Context, sr store.SharedRepo) int {
	eng, err := s.sharedEngine()
	if err != nil {
		return 0
	}
	dest, mode, _, err := s.sharedAccess(sr)
	if err != nil {
		return 0
	}
	cctx, cancel := context.WithTimeout(ctx, sharedFactsWait)
	defer cancel()
	keys, err := eng.Keys(cctx, dest, mode)
	if err != nil {
		return 0
	}
	n := 0
	for _, k := range keys {
		if k.UserName == sharedProofUser {
			n++
		}
	}
	return n
}
