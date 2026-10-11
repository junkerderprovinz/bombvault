package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

var (
	errMoveSamePlace = errors.New("the section already keeps its copy at that destination")
	// errMoveSignIn is two places of the same kind under different sign-ins.
	// restic copies between two repositories in one process, and that process
	// takes one sign-in of each kind.
	errMoveSignIn  = errors.New("both places sign in the same way with different credentials, which one copy cannot do; move through a folder on this server instead")
	errMoveMissing = errors.New("the new place does not hold every backup of the old one after the copy, so the section stays where it is")
	errMoveRunning = errors.New("this section is already being moved")
)

// sectionMove is what moving a section's copy to another destination copies.
type sectionMove struct {
	Domain   string `json:"domain"`
	TargetID string `json:"targetId"`
	From     string `json:"from"`
	To       string `json:"to"`
	// Backups is how many backups of the section the old place holds, Pending
	// how many of them the new place does not hold yet.
	Backups int `json:"backups"`
	Pending int `json:"pending"`
	// Key is the progress key the running move reports under and is cancelled
	// by.
	Key string `json:"key"`
}

type movePlan struct {
	sectionMove
	target, dest store.OffsiteTarget
	src, dst     string
	// srcMode reads the old place. mode writes the new one and carries the
	// sign-ins of both, because the copy reaches both from one process.
	srcMode, mode restic.Mode
	lim           restic.Limits
}

// bothSignIns joins the backend credentials of two places. ok is false when
// they set the same variable to different values.
func bothSignIns(a, b []string) (env []string, ok bool) {
	set := map[string]string{}
	for _, kv := range a {
		name, value, _ := strings.Cut(kv, "=")
		set[name] = value
	}
	env = append(env, a...)
	for _, kv := range b {
		name, value, _ := strings.Cut(kv, "=")
		if held, taken := set[name]; taken {
			if held != value {
				return nil, false
			}
			continue
		}
		env = append(env, kv)
	}
	return env, true
}

// planMove checks a move and counts what it would copy. Nothing is written.
func (s *Service) planMove(ctx context.Context, targetID, destID string) (movePlan, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return movePlan{}, fmt.Errorf("read settings: %w", err)
	}
	t, ok, err := s.store.GetOffsiteTarget(targetID)
	if err != nil {
		return movePlan{}, err
	}
	if !ok {
		return movePlan{}, store.ErrNotOffsiteTarget
	}
	d, ok, err := s.store.GetDestination(destID)
	if err != nil {
		return movePlan{}, err
	}
	if !ok {
		return movePlan{}, store.ErrNotDestination
	}
	if _, direct, err := s.store.CompanionFor(t.ID); err != nil {
		return movePlan{}, err
	} else if direct {
		return movePlan{}, store.ErrTargetHasDirectRepo
	}
	held, err := s.store.DestinationTargets(d.ID)
	if err != nil {
		return movePlan{}, err
	}
	for _, other := range held {
		if other.Domain != t.Domain {
			continue
		}
		if other.ID == t.ID {
			return movePlan{}, errMoveSamePlace
		}
		return movePlan{}, store.ErrDomainHasDestinationTarget
	}

	to := destinationLocation(d.Repo, t.Domain)
	plan := movePlan{
		sectionMove: sectionMove{
			Domain: t.Domain, TargetID: t.ID, Key: "move:" + t.ID,
			From: scrubRepoLocation(t.Repo), To: scrubRepoLocation(to),
		},
		target: t, dest: d, lim: targetOffsiteLimits(t),
	}
	if plan.src, err = s.resolveRepo(t.Repo); err != nil {
		return movePlan{}, err
	}
	if plan.dst, err = s.resolveRepo(to); err != nil {
		return movePlan{}, err
	}
	if filepath.ToSlash(plan.src) == filepath.ToSlash(plan.dst) {
		return movePlan{}, errMoveSamePlace
	}
	// A copy on a home repository is no second copy, and one on or around
	// another target or destination is emptied with it.
	live, err := s.liveRepos(settings)
	if err != nil {
		return movePlan{}, err
	}
	if overlapsLive(s.placesNow(), plan.dst, live, t.ID, d.ID) {
		return movePlan{}, errNestedLocation
	}
	arrived := t
	arrived.Repo, arrived.CredsRef, arrived.StorageClass = to, d.CredsRef, d.StorageClass
	plan.srcMode, plan.mode = s.offsiteModeForTarget(settings, t), s.offsiteModeForTarget(settings, arrived)
	if plan.mode.Env, ok = bothSignIns(plan.mode.Env, plan.srcMode.Env); !ok {
		return movePlan{}, errMoveSignIn
	}

	pending, all, err := s.pendingMove(ctx, plan)
	if err != nil {
		return movePlan{}, err
	}
	plan.Backups, plan.Pending = all, len(pending)
	return plan, nil
}

// pendingMove lists both places and returns the section's backups the new one
// does not hold yet, and how many the old one holds. A new place that is no
// repository yet holds none.
func (s *Service) pendingMove(ctx context.Context, plan movePlan) (pending []restic.Snapshot, all int, err error) {
	src, err := s.listRepo(ctx, plan.src, plan.srcMode)
	if err != nil {
		return nil, 0, fmt.Errorf("list %s: %w", plan.From, err)
	}
	src = ofSection(src, plan.Domain)
	var dst []restic.Snapshot
	if s.engine.RepoOpens(ctx, plan.dst, plan.mode) {
		if dst, err = s.listRepo(ctx, plan.dst, plan.mode); err != nil {
			return nil, 0, fmt.Errorf("list %s: %w", plan.To, err)
		}
	}
	return pendingSnapshots(src, dst), len(src), nil
}

// MoveSection copies a section's backups from the place its target keeps them
// to the folder another destination has for the section, and points the
// target there once the new place holds every one of them and passes a check.
// The old place is only read.
//
// The bulk is copied while the section goes on being backed up and copied.
// Only what arrived in the meantime, the comparison, the check and the switch
// run under the section's lock.
func (s *Service) MoveSection(ctx context.Context, plan movePlan, startedAt int64) (store.Settings, error) {
	if err := s.EnsureRepo(ctx, plan.dst, plan.mode); err != nil {
		return store.Settings{}, fmt.Errorf("open %s: %w", plan.To, err)
	}
	if err := s.copyPending(ctx, plan, startedAt); err != nil {
		return store.Settings{}, err
	}

	unlock, err := s.lockDomainWhile(ctx, plan.Domain, "replicate")
	if err != nil {
		return store.Settings{}, err
	}
	defer unlock()
	if err := s.copyPending(ctx, plan, startedAt); err != nil {
		return store.Settings{}, err
	}
	left, _, err := s.pendingMove(ctx, plan)
	if err != nil {
		return store.Settings{}, err
	}
	if len(left) > 0 {
		return store.Settings{}, errMoveMissing
	}
	if err := s.engine.Check(ctx, plan.dst, plan.mode); err != nil {
		return store.Settings{}, fmt.Errorf("check %s: %w", plan.To, err)
	}

	to := destinationLocation(plan.dest.Repo, plan.Domain)
	moved, err := s.store.MoveTargetToDestination(plan.target.ID, plan.dest.ID, to, func(st *store.Settings) {
		setOffsiteRepoInSettings(st, plan.Domain, to)
		setOffsiteImmutableInSettings(st, plan.Domain, plan.dest.Immutable)
	})
	if err != nil {
		return store.Settings{}, err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return store.Settings{}, err
	}
	if moved.SortOrder == 0 {
		// The sync fills in the retention and limits the settings give the primary.
		if err := s.syncPrimaryOffsiteTarget(plan.Domain, settings); err != nil {
			return store.Settings{}, err
		}
	}
	return settings, nil
}

// copyPending copies what the new place lacks, a block of backups per restic
// run, so a move that is cancelled or fails keeps the blocks that landed and
// the next one goes on from there.
func (s *Service) copyPending(ctx context.Context, plan movePlan, startedAt int64) error {
	pending, _, err := s.pendingMove(ctx, plan)
	if err != nil {
		return err
	}
	last := -1.0
	ctx = progress.WithCopySink(ctx, func(cp progress.CopyProgress) {
		if cp.Percent < 100 && cp.Percent >= last && cp.Percent-last < 1 {
			return
		}
		last = cp.Percent
		s.publishProgress(progress.Event{
			Key: plan.Key, Phase: "replicate", Active: true, StartedAt: startedAt,
			Percent: cp.Percent, SnapshotIndex: cp.SnapshotIndex, SnapshotTotal: len(pending),
		})
	})
	if _, err := s.copyInChunks(ctx, plan.Domain, plan.dst, plan.src, pending, plan.lim, plan.mode, 0); err != nil {
		return fmt.Errorf("copy to %s: %w", plan.To, err)
	}
	return nil
}

// lockDomainWhile waits for the domain's lock until ctx ends.
func (s *Service) lockDomainWhile(ctx context.Context, domain, reason string) (func(), error) {
	for {
		if unlock, ok := s.tryLockDomainFor(domain, reason); ok {
			return unlock, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(drillLockPoll):
		}
	}
}

// StartSectionMove checks a move and runs it in the background as a run of
// kind "offsite" on the section. moved runs once the target points at the new
// place.
func (s *Service) StartSectionMove(ctx context.Context, targetID, destID string, moved func(store.Settings)) (sectionMove, error) {
	plan, err := s.planMove(ctx, targetID, destID)
	if err != nil {
		return sectionMove{}, err
	}
	bctx := context.WithoutCancel(ctx)
	rctx, cancel := context.WithTimeout(bctx, 7*24*time.Hour)
	s.cancelMu.Lock()
	if _, running := s.runCancels[plan.Key]; running {
		s.cancelMu.Unlock()
		cancel()
		return sectionMove{}, errMoveRunning
	}
	s.runCancels[plan.Key] = cancel
	s.cancelMu.Unlock()
	go func() {
		var runID string
		defer s.recoverOperation("move "+plan.Domain, nil, func(msg string) { s.finishMoveRun(runID, errors.New(msg)) })
		defer cancel()
		defer s.unregisterCancel(plan.Key)
		var rErr error
		if runID, rErr = s.startRun(bctx, domainRunTargetID(plan.Domain), "offsite"); rErr != nil {
			log.Printf("api: move %s: the run is not recorded: %v", plan.Domain, rErr) //nolint:gosec // G706: domain is a fixed literal
		}
		_, startedAt := s.progBegin(rctx, plan.Key, "replicate")
		settings, err := s.MoveSection(rctx, plan, startedAt)
		if err == nil {
			moved(settings)
		}
		s.progEnd(plan.Key, "replicate", err == nil, startedAt)
		s.finishMoveRun(runID, err)
		if err != nil {
			log.Printf("api: move %s: %v", plan.Domain, scrubError(err)) //nolint:gosec // G706: domain is a fixed literal and the error scrubbed
		}
	}()
	return plan.sectionMove, nil
}

func (s *Service) finishMoveRun(runID string, mErr error) {
	if runID == "" {
		return
	}
	status := "success"
	switch {
	case errors.Is(mErr, context.Canceled):
		status = "cancelled"
	case mErr != nil:
		status = "failed"
	}
	if err := s.store.FinishRun(runID, status, "", 0, truncateRunErr(mErr)); err != nil {
		log.Printf("api: move: the run is not closed: %v", err)
	}
}

func moveFail(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotOffsiteTarget) || errors.Is(err, store.ErrNotDestination) {
		writeJSON(w, http.StatusNotFound, codedFailEnvelope(err, "not-found"))
		return
	}
	placementFail(w, err, nil)
}

// handleSectionMovePreview says what moving a section's copy to another
// destination would copy.
// GET /api/offsite/targets/{id}/move?to={destination}
func (h *Handler) handleSectionMovePreview(w http.ResponseWriter, r *http.Request) {
	plan, err := h.svc.planMove(r.Context(), r.PathValue("id"), r.URL.Query().Get("to"))
	if err != nil {
		moveFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"move": plan.sectionMove}))
}

// handleMoveSection starts the move of a section's copy to the destination
// the body names.
// POST /api/offsite/targets/{id}/move
func (h *Handler) handleMoveSection(w http.ResponseWriter, r *http.Request) {
	var body struct {
		To string `json:"to"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	// A moved primary target changed the section's off-site settings, and the
	// scheduler reads those.
	moved := func(settings store.Settings) {
		if err := h.scheduler.ReloadWithGates(settings, h.dueGates()); err != nil {
			log.Printf("api: move: the scheduler did not reload: %v", err)
		}
	}
	move, err := h.svc.StartSectionMove(r.Context(), r.PathValue("id"), body.To, moved)
	if err != nil {
		moveFail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": true, "move": move}))
}
