package api

import (
	"errors"
	"log"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// upkeepTurn lets one upkeep of the shared repositories run at a time and
// tells the page whether one does.
type upkeepTurn struct {
	mu   sync.Mutex
	busy atomic.Bool
}

func (t *upkeepTurn) take() bool {
	if !t.mu.TryLock() {
		return false
	}
	t.busy.Store(true)
	return true
}

func (t *upkeepTurn) done() {
	t.busy.Store(false)
	t.mu.Unlock()
}

func (t *upkeepTurn) running() bool { return t.busy.Load() }

// startSharedUpkeep forgets, prunes, checks and measures the shared
// repository id in the background, as a person asked. A prune can run for
// hours, so the answer comes at once and the page follows the pass. A
// shutdown ends it, and restic leaves the repository as a cut-off prune does.
func (s *Service) startSharedUpkeep(id string, members sharedMembers) error {
	if !s.sharedUpkeep.take() {
		return errSharedUpkeepBusy
	}
	sr, err := s.ownedSharedRepo(id)
	if err != nil {
		s.sharedUpkeep.done()
		return err
	}
	go func() {
		defer s.sharedUpkeep.done()
		defer s.recoverOperation("upkeep of a shared repository", nil, nil)
		if err := s.keepSharedRepo(s.StopContext(), sr, members, time.Now(), true); err != nil {
			log.Printf("api: shared repository: the upkeep asked for failed: %v", err)
		}
	}()
	return nil
}

// ownedSharedRepo is the shared repository id, if this server holds it.
func (s *Service) ownedSharedRepo(id string) (store.SharedRepo, error) {
	owned, err := s.ownedSharedRepos()
	if err != nil {
		return store.SharedRepo{}, err
	}
	i := slices.IndexFunc(owned, func(sr store.SharedRepo) bool { return sr.ID == id })
	if i < 0 {
		return store.SharedRepo{}, errNotSharedOwner
	}
	return owned[i], nil
}

// sharedUpkeepView is a shared repository as the page shows its upkeep. A
// member sees what the owner measured; the owner also sees its rules, its
// last passes and when the next window opens.
type sharedUpkeepView struct {
	ID         string                    `json:"id"`
	Location   string                    `json:"location"`
	Role       string                    `json:"role"`
	Members    []sharedMemberView        `json:"members"`
	Rules      *sharedRulesView          `json:"rules,omitempty"`
	Forget     *store.SharedForgetReport `json:"forget,omitempty"`
	Prune      *store.SharedPruneReport  `json:"prune,omitempty"`
	Space      *store.SharedSpace        `json:"space,omitempty"`
	NextWindow int64                     `json:"nextWindow,omitempty"`
	Running    bool                      `json:"running"`
}

// sharedRulesView is the part of the upkeep a person sets.
type sharedRulesView struct {
	Keep         store.RetentionKeep `json:"keep"`
	KeepSet      bool                `json:"keepSet"`
	PruneCadence string              `json:"pruneCadence"`
	PruneHours   int                 `json:"pruneHours"`
	MaxUnused    int                 `json:"maxUnused"`
	CheckSubset  int                 `json:"checkSubset"`
}

// sharedUpkeepViews lists every shared repository this server is in.
func (s *Service) sharedUpkeepViews(now time.Time) ([]sharedUpkeepView, error) {
	all, err := s.store.ListSharedRepos()
	if err != nil {
		return nil, err
	}
	out := make([]sharedUpkeepView, 0, len(all))
	for _, sr := range all {
		v := sharedUpkeepView{ID: sr.ID, Location: scrubRepoLocation(sr.Repo), Role: "member", Members: []sharedMemberView{}}
		for _, m := range sr.Members {
			v.Members = append(v.Members, sharedMemberView{MemberID: m.Instance, Name: m.Name, Host: m.Host})
		}
		if !s.ownsSharedRepo(sr) {
			space, ok, err := s.store.SharedSpaceFrom(sr.Owner)
			if err != nil {
				return nil, err
			}
			if ok {
				v.Space = &space
			}
			out = append(out, v)
			continue
		}
		u, err := s.store.SharedUpkeepOf(sr.ID)
		if err != nil {
			return nil, err
		}
		v.Role = "owner"
		v.Rules = &sharedRulesView{Keep: u.Keep, KeepSet: u.KeepSet, PruneCadence: u.PruneCadence, PruneHours: u.PruneHours, MaxUnused: u.MaxUnused, CheckSubset: u.CheckSubset}
		v.Forget, v.Prune, v.Space = u.Forget, u.Prune, u.Space
		if cad, err := schedule.ParseCadence(u.PruneCadence); err == nil {
			if next, ok := cad.NextDue(time.Time{}, now); ok {
				v.NextWindow = next.Unix()
			}
		}
		v.Running = s.sharedUpkeep.running()
		out = append(out, v)
	}
	return out, nil
}

// checkSharedRules says what is wrong with rules a person sent, if anything.
// A window that never opens would leave the repository to grow for good.
func checkSharedRules(in sharedRulesView) error {
	cad, err := schedule.ParseCadence(in.PruneCadence)
	switch {
	case err != nil || !cad.Enabled:
		return errors.New("the prune window needs a day and a time")
	case in.PruneHours < 1 || in.PruneHours > 24:
		return errors.New("the prune window lasts between 1 and 24 hours")
	case in.MaxUnused < 0 || in.MaxUnused > 100:
		return errors.New("the unused space a prune leaves is between 0 and 100 percent")
	case in.CheckSubset < 1 || in.CheckSubset > 100:
		return errors.New("the check after a prune reads between 1 and 100 percent of the data")
	case min(in.Keep.KeepLast, in.Keep.KeepDaily, in.Keep.KeepWeekly, in.Keep.KeepMonthly, in.Keep.KeepYearly) < 0:
		return errors.New("a keep rule cannot be negative")
	}
	return nil
}

// handleSharedUpkeep lists the shared repositories this server is in, with
// their upkeep. GET /api/shared-repos/upkeep
func (h *Handler) handleSharedUpkeep(w http.ResponseWriter, _ *http.Request) {
	views, err := h.svc.sharedUpkeepViews(time.Now())
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"repos": views}))
}

// handlePutSharedUpkeep saves the rules of a shared repository this server
// holds. PUT /api/shared-repos/{id}/upkeep
func (h *Handler) handlePutSharedUpkeep(w http.ResponseWriter, r *http.Request) {
	var in sharedRulesView
	if !decodeBody(w, r, &in) {
		return
	}
	sr, err := h.svc.ownedSharedRepo(r.PathValue("id"))
	if err == nil {
		err = checkSharedRules(in)
	}
	if err == nil {
		err = h.store.PutSharedUpkeepRules(store.SharedUpkeep{RepoID: sr.ID, Keep: in.Keep, KeepSet: in.KeepSet,
			PruneCadence: in.PruneCadence, PruneHours: in.PruneHours, MaxUnused: in.MaxUnused, CheckSubset: in.CheckSubset})
	}
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleRunSharedUpkeep starts the upkeep of a shared repository this server
// holds at once. POST /api/shared-repos/{id}/upkeep/run
func (h *Handler) handleRunSharedUpkeep(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.startSharedUpkeep(r.PathValue("id"), h.svc.groupMembers()); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}
