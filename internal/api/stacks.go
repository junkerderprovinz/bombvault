package api

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/compose"
)

// composeProject and composeService read the compose project and service labels,
// or "" for a container outside compose. The ordering logic lives in
// internal/compose, which the backup restart phase shares.
func composeProject(labels map[string]string) string { return compose.Project(labels) }
func composeService(labels map[string]string) string { return compose.Service(labels) }

// parseDependsOn returns the compose service names a container depends on.
func parseDependsOn(labels map[string]string) []string { return compose.ParseDependsOn(labels) }

// StackMemberResult is the per-container outcome of a stack restore.
type StackMemberResult struct {
	Name     string `json:"name"`
	Service  string `json:"service"`
	Restored bool   `json:"restored"`
	Started  bool   `json:"started"`
	Error    string `json:"error,omitempty"`
}

// StackRestoreResult is the result of RestoreStack: one entry per backed-up
// member, in enumeration order.
type StackRestoreResult struct {
	Members []StackMemberResult `json:"members"`
}

// stackMember is one container of a stack being restored.
type stackMember struct {
	name       string
	service    string
	deps       []string // compose service names this member depends_on
	wasRunning bool     // run-state captured at backup (def.Inspect.Running)
}

// prepareRestoreStack validates a stack restore and lists its members, so a bad
// request, including a stack with no backed-up containers, fails before
// anything long-running starts.
func (s *Service) prepareRestoreStack(project, source string, confirm bool) ([]stackMember, error) {
	if !confirm {
		return nil, backup.ErrNotConfirmed
	}
	if source != "local" && !isOffsiteSource(source) {
		return nil, fmt.Errorf("invalid source (must be local or offsite)")
	}
	// The name is only compared with labels, never used as a path, but
	// traversal tricks are rejected anyway.
	project = strings.TrimSpace(project)
	if project == "" {
		return nil, fmt.Errorf("stack name is required")
	}
	if strings.Contains(project, "/") || strings.Contains(project, "..") {
		return nil, fmt.Errorf("invalid stack name")
	}

	// ListTargets orders by container_name, so the member order is stable.
	targets, err := s.store.ListTargets()
	if err != nil {
		return nil, fmt.Errorf("list targets: %w", err)
	}
	var members []stackMember
	for _, tg := range targets {
		if tg.Definition == "" {
			continue
		}
		var def containerDefinition
		if json.Unmarshal([]byte(tg.Definition), &def) != nil {
			continue
		}
		labels := def.Inspect.Config.Labels
		if composeProject(labels) != project {
			continue
		}
		members = append(members, stackMember{
			name:       tg.ContainerName,
			service:    composeService(labels),
			deps:       parseDependsOn(labels),
			wasRunning: def.Inspect.Running,
		})
	}
	if len(members) == 0 {
		return nil, fmt.Errorf("no backed-up containers found in stack %q", project)
	}
	return members, nil
}

// RestoreStack restores every backed-up container of a compose project from its
// latest snapshot and leaves it stopped, so no dependent starts early. With
// startAfter, the members that restored are then started in depends_on order;
// dependencies outside the stack are ignored and a cycle falls back to
// enumeration order. A member's failure is recorded in its result and does not
// stop the others. stackDirSource says where the project folder comes from,
// empty for source. The HTTP layer uses StartRestoreStack, which runs detached.
func (s *Service) RestoreStack(ctx context.Context, project, source, stackDirSource string, startAfter, confirm bool) (StackRestoreResult, error) {
	members, err := s.prepareRestoreStack(project, source, confirm)
	if err != nil {
		return StackRestoreResult{}, err
	}
	return s.runRestoreStack(ctx, members, source, stackDirSource, startAfter), nil
}

// restoreStackMember restores one member and turns a panic into that member's
// error, so the members behind it and the start loop still run. Restore calls
// store.StartRun without a defer, so the handler also fails the run that would
// otherwise stay "running".
func (s *Service) restoreStackMember(ctx context.Context, name, source string) (err error) {
	// recoverOperation must be deferred directly (see backupOneForBatch).
	defer s.recoverOperation("restore stack: "+name, &err, func(msg string) {
		if tg, tErr := s.store.GetTargetByContainer(name); tErr == nil {
			s.failStuckRun(tg.ID, msg)
		}
	})
	return s.Restore(ctx, name, "latest", true, source, true)
}

// runRestoreStack restores the members, then with startAfter starts them in
// dependency order. Each member's restore records its own run, so the outcomes
// stay visible when this runs detached from the request.
func (s *Service) runRestoreStack(ctx context.Context, members []stackMember, source, stackDirSource string, startAfter bool) StackRestoreResult {
	// The project directory comes first: it holds the compose file and shared
	// files a member may read on start. Older backups have no stack snapshot and
	// bring the folder back with the first member instead.
	if len(members) > 0 {
		if project := s.projectOfMember(ctx, members[0].name); project != "" {
			if _, err := s.RestoreStackDir(ctx, project, cmp.Or(stackDirSource, source)); err != nil {
				// Not fatal: failing every member over the shared folder would
				// turn a partial problem into a total one.
				log.Printf("api: restore stack: %v", err)
			}
		}
	}

	// Leave every member stopped, so a dependent cannot come up before its
	// dependency is restored and started.
	results := make([]StackMemberResult, len(members))
	restoredOK := make([]bool, len(members))
	for i, m := range members {
		res := StackMemberResult{Name: m.name, Service: m.service}
		rErr := s.restoreStackMember(ctx, m.name, source)
		switch {
		case rErr == nil:
			res.Restored = true
			restoredOK[i] = true
		case errors.Is(rErr, context.Canceled):
			// A cancel stops the stack restore at this member. Its run is recorded
			// as cancelled; the remaining members and the start loop are skipped.
			res.Error = rErr.Error()
			results[i] = res
			return StackRestoreResult{Members: results[:i+1]}
		default:
			res.Error = rErr.Error()
		}
		results[i] = res
	}

	if startAfter {
		order := stackStartOrder(members)
		deps := stackDepGraph(members)
		// blocked[i] means member i is not started: its restore or start failed,
		// or one of its dependencies is blocked. The loop runs in dependency
		// order, so a member's dependencies are decided before it.
		blocked := make([]bool, len(members))
		for _, i := range order {
			if !restoredOK[i] {
				blocked[i] = true // the restore already recorded the error
				continue
			}
			if dep := firstBlockedDep(deps[i], blocked); dep >= 0 {
				blocked[i] = true
				if results[i].Error == "" {
					results[i].Error = fmt.Sprintf("not started: dependency %q was not restored/started", members[dep].name)
				}
				continue
			}
			// A member that was stopped at backup time stays stopped, as in a
			// single-container restore. It does not block its dependents.
			if !members[i].wasRunning {
				continue
			}
			if sErr := s.docker.Start(ctx, members[i].name); sErr != nil {
				blocked[i] = true // its failure holds back anything that depends on it
				if results[i].Error == "" {
					results[i].Error = sErr.Error()
				}
				continue
			}
			results[i].Started = true
		}
	}

	return StackRestoreResult{Members: results}
}

// StartRestoreStack validates a stack restore and runs it in a background
// goroutine detached from the request, so a long restore survives the browser
// or a proxy dropping the idle connection. A bad request fails before the
// goroutine starts. Each member's outcome lands in the run history.
//
// It shares batchActive with backups and the other restores and returns
// (false, nil) when one is already running.
func (s *Service) StartRestoreStack(ctx context.Context, project, source, stackDirSource string, startAfter, confirm bool) (bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	members, err := s.prepareRestoreStack(project, source, confirm)
	if err != nil {
		s.batchActive.Store(false)
		return false, err
	}
	bctx := context.WithoutCancel(ctx)
	// The frontend's cancel button stops the whole stack restore under this key.
	key := "stack:" + project
	go func() {
		// Catches a panic outside the member loop; each member has its own
		// recovery in restoreStackMember.
		defer s.recoverOperation("restore stack", nil, nil)
		defer s.batchActive.Store(false)
		tctx, tcancel := context.WithTimeout(bctx, restoreTimeout)
		defer tcancel()
		rctx, cancel := context.WithCancel(tctx)
		defer cancel()
		s.registerCancel(key, cancel)
		defer s.unregisterCancel(key)
		res := s.runRestoreStack(rctx, members, source, stackDirSource, startAfter)
		for _, m := range res.Members {
			if m.Error != "" {
				log.Printf("api: restore stack: member %q failed: %v", m.Name, m.Error) //nolint:gosec // G706: name is %q-quoted; the error is service/restic-generated
			}
		}
	}()
	return true, nil
}

// memberServicesAndDeps unpacks a member list into the parallel (services, deps)
// slices the shared compose ordering primitives consume.
func memberServicesAndDeps(members []stackMember) ([]string, [][]string) {
	services := make([]string, len(members))
	deps := make([][]string, len(members))
	for i, m := range members {
		services[i] = m.service
		deps[i] = m.deps
	}
	return services, deps
}

// stackDepGraph maps each member to the indices of the other members it depends
// on (see compose.DepGraph).
func stackDepGraph(members []stackMember) [][]int {
	return compose.DepGraph(memberServicesAndDeps(members))
}

// firstBlockedDep returns the index of the first dependency in deps that is
// blocked, or -1 when none is.
func firstBlockedDep(deps []int, blocked []bool) int {
	for _, j := range deps {
		if blocked[j] {
			return j
		}
	}
	return -1
}

// stackStartOrder returns member indices in dependency order, dependencies
// first (see compose.StartOrder).
func stackStartOrder(members []stackMember) []int {
	return compose.StartOrder(memberServicesAndDeps(members))
}

// stackParam reads {project}, a compose project name, which is laxer than a
// container name but must not carry a path.
func stackParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	project := r.PathValue("project")
	if project == "" || strings.Contains(project, "/") || strings.Contains(project, "..") {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid stack name"})
		return "", false
	}
	return project, true
}

// handleRestoreStack restores every backed-up member of a compose stack stopped,
// then optionally starts them in dependency order.
// POST /api/stacks/{project}/restore
//
// Like handleRestore it runs detached after validation and member enumeration,
// so a bad request or an empty stack still fails right away. Each member's
// restore records its own "restore" run.
func (h *Handler) handleRestoreStack(w http.ResponseWriter, r *http.Request) {
	project, ok := stackParam(w, r)
	if !ok {
		return
	}
	var body struct {
		StartAfter     bool   `json:"startAfter"`
		Confirm        bool   `json:"confirm"`
		StackDirSource string `json:"stackDirSource"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	dirSource := ""
	if body.StackDirSource != "" {
		dirSource = normalizeSource(body.StackDirSource)
	}
	started, err := h.svc.StartRestoreStack(r.Context(), project, sourceParam(r), dirSource, body.StartAfter, body.Confirm)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if !started {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a backup or restore is already running"})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": true}))
}
