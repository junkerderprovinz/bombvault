package api

import (
	"context"
	"encoding/json"
	"errors"
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
type peerSharedHolder struct {
	RepoID string `json:"repoId"`
	peerProof
}

func (c peerSharedHolder) signed() []byte { return callPayload(c.RepoID) }

// peerSharedHolderAnswer says whether the word was taken. Later asks to be
// told again, when the server could not read its repositories just then.
type peerSharedHolderAnswer struct {
	OK    bool   `json:"ok"`
	Later bool   `json:"later,omitempty"`
	Error string `json:"error,omitempty"`
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
	err := s.takeSharedHolder(r.Context(), holder, in.RepoID)
	switch {
	case errors.Is(err, errSharedRepoIDUnread):
		writeJSON(w, http.StatusOK, peerSharedHolderAnswer{Later: true, Error: err.Error()})
	case err != nil:
		writeJSON(w, http.StatusOK, peerSharedHolderAnswer{Error: err.Error()})
	default:
		writeJSON(w, http.StatusOK, peerSharedHolderAnswer{OK: true})
	}
}

// takeSharedHolder makes holder the holder of the shared repository whose id
// this server reads as id. A server that held it until now stops keeping it
// and remembers who took it over, so a person can take it back.
func (s *Service) takeSharedHolder(ctx context.Context, holder, id string) error {
	all, err := s.store.ListSharedRepos()
	if err != nil {
		return err
	}
	unread := false
	for _, sr := range all {
		got, err := s.sharedRepoID(ctx, sr)
		if err != nil {
			log.Printf("api: shared repository: could not read the id of %s: %v", scrubRepoLocation(sr.Repo), scrubError(err)) //nolint:gosec // G706: the location and the error are scrubbed
			unread = true
			continue
		}
		if got != id {
			continue
		}
		if sr.Owner == holder {
			return nil
		}
		if s.ownsSharedRepo(sr) {
			name := holder
			if m, ok := s.reachableMember(holder); ok && m.Name != "" {
				name = m.Name
			}
			if err := s.store.NoteSharedTakenOver(store.SharedTakenOver{RepoID: sr.ID, By: holder, Name: name, At: time.Now().Unix()}); err != nil {
				return err
			}
			log.Printf("api: shared repository: %s holds %s now, so this server stops keeping it", name, scrubRepoLocation(sr.Repo)) //nolint:gosec // G706: the name is the member's own, the location scrubbed
		}
		sr.Owner = holder
		_, err = s.store.PutSharedRepo(sr)
		return err
	}
	if unread {
		return errSharedRepoIDUnread
	}
	log.Printf("api: shared repository: %s says it holds a repository with an id none of this server's has, so nothing changes", holder) //nolint:gosec // G706: an instance id the group channel checked
	return errNoSharedRepoWithID
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
		ans, err := s.tellSharedHolder(ctx, n.Instance, id)
		switch {
		case err != nil || ans.Later:
			continue
		case !ans.OK:
			log.Printf("api: shared repository: %s did not take this server as the holder: %q", n.Instance, ans.Error) //nolint:gosec // G706: an instance id from the store and the member's own answer
		}
		s.dropSharedHolderNotice(n)
	}
}

func (s *Service) tellSharedHolder(ctx context.Context, instance, id string) (peerSharedHolderAnswer, error) {
	in := peerSharedHolder{RepoID: id}
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
	Repo        string   `json:"repo"`
	TakenBy     string   `json:"takenBy,omitempty"`
	TakenAt     int64    `json:"takenAt,omitempty"`
	StillToTell []string `json:"stillToTell,omitempty"`
}

// sharedHoldings lists what the holder's card says about each shared
// repository this server holds or held.
func (s *Service) sharedHoldings() ([]sharedHolding, error) {
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
		if h.TakenBy != "" || len(h.StillToTell) > 0 {
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
