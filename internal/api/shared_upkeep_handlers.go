package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// upkeepTurn lets one upkeep of the shared repositories run at a time and
// tells the page whether one does, and which former member's backups it
// forgets.
type upkeepTurn struct {
	mu         sync.Mutex
	busy       atomic.Bool
	forgetting atomic.Pointer[string]
}

func (t *upkeepTurn) take() bool {
	if !t.mu.TryLock() {
		return false
	}
	t.busy.Store(true)
	return true
}

func (t *upkeepTurn) done() {
	t.forgetting.Store(nil)
	t.busy.Store(false)
	t.mu.Unlock()
}

func (t *upkeepTurn) running() bool { return t.busy.Load() }

// forgets reports whether the turn forgets the backups of the former member
// under host.
func (t *upkeepTurn) forgets(host string) bool {
	h := t.forgetting.Load()
	return h != nil && *h == host
}

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

var (
	errNoSuchFormer = errors.New("no server that left this shared repository wrote under that host")
	errFormerName   = errors.New("type the server's name as the card shows it to forget its backups")
)

// startForgetFormer forgets in the background every snapshot the server that
// wrote under host left in the shared repository id after it went, once a
// person typed its name. It takes the upkeep's turn, so it never meets a
// prune of this server's, and the space comes back with the next window's.
func (s *Service) startForgetFormer(id, host, typed string) error {
	sr, err := s.ownedSharedRepo(id)
	if err != nil {
		return err
	}
	f, err := s.formerMember(sr, host)
	if err != nil {
		return err
	}
	if strings.TrimSpace(typed) != f.Name {
		return errFormerName
	}
	if !s.sharedUpkeep.take() {
		return errSharedUpkeepBusy
	}
	s.sharedUpkeep.forgetting.Store(&f.Host)
	go func() {
		defer s.sharedUpkeep.done()
		defer s.recoverOperation("forget a former member of a shared repository", nil, nil)
		s.forgetFormer(s.StopContext(), sr, f)
	}()
	return nil
}

// formerMember is the former member of sr under host. A host a member writes
// under again, or this server's own, is nobody's that left.
func (s *Service) formerMember(sr store.SharedRepo, host string) (store.SharedFormer, error) {
	if host == s.resticHost() || slices.ContainsFunc(sr.Members, func(m store.SharedMember) bool { return m.Host == host }) {
		return store.SharedFormer{}, errNoSuchFormer
	}
	former, err := s.store.ListSharedFormer(sr.ID)
	if err != nil {
		return store.SharedFormer{}, err
	}
	i := slices.IndexFunc(former, func(f store.SharedFormer) bool { return f.Host == host })
	if i < 0 {
		return store.SharedFormer{}, errNoSuchFormer
	}
	return former[i], nil
}

// forgetFormer forgets the snapshots restic lists under the host of the
// former member f, and keeps why when that fails so the card can say so.
func (s *Service) forgetFormer(ctx context.Context, sr store.SharedRepo, f store.SharedFormer) {
	err := s.forgetFormerSnapshots(ctx, sr.Repo, f.Host)
	if err != nil {
		log.Printf("api: shared repository: forgetting what %s left failed: %v", f.Name, scrubError(err)) //nolint:gosec // G706: the name is the former member's own, the error scrubbed
		f.ForgetError = scrubError(err)
		if nErr := s.store.NoteSharedFormer(sr.ID, f); nErr != nil {
			log.Printf("api: shared repository: could not keep why: %v", nErr)
		}
		return
	}
	if err := s.store.DropSharedFormer(sr.ID, f.Host); err != nil {
		log.Printf("api: shared repository: forgot what %s left, but could not note it: %v", f.Name, err) //nolint:gosec // G706: the name is the former member's own
	}
}

func (s *Service) forgetFormerSnapshots(ctx context.Context, repo, host string) error {
	// The server may have come back while the forget waited for its turn.
	sr, ok, err := s.store.SharedRepoAt(repo)
	if err != nil {
		return err
	}
	if !ok {
		return errNotSharedOwner
	}
	if _, err := s.formerMember(sr, host); err != nil {
		return err
	}
	dest, mode, _, err := s.sharedAccess(sr)
	if err != nil {
		return err
	}
	snaps, err := s.engine.SnapshotsOfEveryHost(ctx, dest, mode)
	if err != nil {
		return err
	}
	var ids []string
	for _, sn := range snaps {
		if sn.Hostname == host {
			ids = append(ids, sn.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return s.retryAfterUnlock(ctx, dest, mode, func() error { return s.engine.Forget(ctx, dest, ids, false, mode) })
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
			if v.Space, v.NextWindow, err = s.sharedSpaceHeard(sr.Owner); err != nil {
				return nil, err
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
		v.NextWindow = nextSharedWindow(u, now)
		v.Running = s.sharedUpkeep.running()
		out = append(out, v)
	}
	return out, nil
}

// sharedSpaceHeard is what the owner last told this server about its shared
// repository: the numbers once it measured them, and its next window.
func (s *Service) sharedSpaceHeard(owner string) (*store.SharedSpace, int64, error) {
	space, ok, err := s.store.SharedSpaceFrom(owner)
	if err != nil || !ok {
		return nil, 0, err
	}
	if space.At == 0 {
		return nil, space.NextWindow, nil
	}
	return &space, space.NextWindow, nil
}

// tellSharedWindowLater tells the members of sr the window new rules open
// next. Some of them answer slowly over the relay, so the person saving the
// rules does not wait for it.
func (s *Service) tellSharedWindowLater(sr store.SharedRepo) {
	ctx, cancel := context.WithTimeout(s.StopContext(), time.Minute)
	defer cancel()
	s.tellSharedWindow(ctx, sr, s.groupMembers(), time.Now())
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
	go h.svc.tellSharedWindowLater(sr)
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleForgetSharedFormer forgets what a server that left a shared
// repository this server holds left there, once a person typed its name.
// POST /api/shared-repos/{id}/former/{host}/forget
func (h *Handler) handleForgetSharedFormer(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	if err := h.svc.startForgetFormer(r.PathValue("id"), r.PathValue("host"), in.Name); err != nil {
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
