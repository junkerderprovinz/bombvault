package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/compose"
)

// composeProject / composeService read the standard compose identity labels off a
// container's label map. "" when the label is absent (not a compose container).
// The depends_on ordering primitives live in internal/compose so the backup
// restart phase shares this exact logic (one topological sort, no drift).
func composeProject(labels map[string]string) string { return compose.Project(labels) }
func composeService(labels map[string]string) string { return compose.Service(labels) }

// parseDependsOn extracts the compose service names a container depends on. See
// compose.ParseDependsOn for the label-encoding details it handles.
func parseDependsOn(labels map[string]string) []string { return compose.ParseDependsOn(labels) }

// StackMemberResult is the per-container outcome of a stack restore.
type StackMemberResult struct {
	Name     string `json:"name"`
	Service  string `json:"service"`
	Restored bool   `json:"restored"`
	Started  bool   `json:"started"`
	Error    string `json:"error,omitempty"`
}

// StackRestoreResult is the full result of RestoreStack: one entry per backed-up
// member, in stable (enumeration) order.
type StackRestoreResult struct {
	Members []StackMemberResult `json:"members"`
}

// stackMember is the internal working record for one enumerated stack member.
type stackMember struct {
	name       string
	service    string
	deps       []string // compose service names this member depends_on
	wasRunning bool     // run-state captured at backup (def.Inspect.Running)
}

// prepareRestoreStack performs ALL of a stack restore's validation and member
// enumeration synchronously — confirmation, source and project-name checks and
// the stored-target enumeration — so a bad request (including "no backed-up
// containers found in stack") fails immediately with a clear error, BEFORE
// anything long-running starts. The returned member list is everything
// runRestoreStack needs.
func (s *Service) prepareRestoreStack(project, source string, confirm bool) ([]stackMember, error) {
	if !confirm {
		return nil, backup.ErrNotConfirmed
	}
	if source != "local" && source != "offsite" {
		return nil, fmt.Errorf("invalid source (must be local or offsite)")
	}
	// Defense-in-depth: the project name flows into the member enumeration only
	// (never a filesystem path), but reject the obvious traversal tricks anyway.
	project = strings.TrimSpace(project)
	if project == "" {
		return nil, fmt.Errorf("stack name is required")
	}
	if strings.Contains(project, "/") || strings.Contains(project, "..") {
		return nil, fmt.Errorf("invalid stack name")
	}

	// Enumerate the members from the stored targets. ListTargets orders by
	// container_name, so this enumeration order is stable and alphabetical.
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

// RestoreStack restores every backed-up container in the compose project: each is
// restored from its LATEST snapshot with leaveStopped=true (nothing starts during
// restore, so a dependent container can't start prematurely). When startAfter is
// true, members that restored OK are then started in dependency order
// (topological sort over com.docker.compose.depends_on; deps outside the stack are
// ignored; any cycle/unknown falls back to stable enumeration order). A single
// member's failure is recorded in its result and does NOT abort the others.
// Confirm must be true.
//
// This is the SYNC composition (prepareRestoreStack + runRestoreStack) — the
// HTTP layer uses StartRestoreStack, which runs the loops detached.
func (s *Service) RestoreStack(ctx context.Context, project, source string, startAfter, confirm bool) (StackRestoreResult, error) {
	members, err := s.prepareRestoreStack(project, source, confirm)
	if err != nil {
		return StackRestoreResult{}, err
	}
	return s.runRestoreStack(ctx, members, source, startAfter), nil
}

// runRestoreStack drives the long-running part of a stack restore over an
// already-enumerated member list: the per-member restore loop, then (when
// startAfter) the dependency-ordered start loop. Each member's in-place restore
// records its own kindRestore run via the orchestrator, so per-member outcomes
// stay discoverable even when this runs detached from the request.
func (s *Service) runRestoreStack(ctx context.Context, members []stackMember, source string, startAfter bool) StackRestoreResult {
	// Restore every member from its latest snapshot, leaving it stopped so a
	// dependent can't come up before its dependency is restored + started.
	results := make([]StackMemberResult, len(members))
	restoredOK := make([]bool, len(members))
	for i, m := range members {
		res := StackMemberResult{Name: m.name, Service: m.service}
		rErr := s.Restore(ctx, m.name, "latest", true, source, true)
		switch {
		case rErr == nil:
			res.Restored = true
			restoredOK[i] = true
		case errors.Is(rErr, context.Canceled):
			// A user cancel aborts the whole stack restore at the current member: the
			// member's own run is recorded "cancelled" by the orchestrator, and the
			// remaining members are left untouched (their runs are never started, and
			// the start loop below is skipped).
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
		// blocked[i] = member i could not (and must not) be started: it failed to
		// restore, its own start failed, or a dependency it needs is itself blocked.
		// Processed in dependency order, so a member's deps are decided before it — a
		// dependent is never started ahead of a dependency that isn't up.
		blocked := make([]bool, len(members))
		for _, i := range order {
			if !restoredOK[i] {
				blocked[i] = true // the restore already recorded the error
				continue
			}
			// Hold back a member whose dependency did not come up (exactly the race
			// the stack restore exists to avoid).
			if dep := firstBlockedDep(deps[i], blocked); dep >= 0 {
				blocked[i] = true
				if results[i].Error == "" {
					results[i].Error = fmt.Sprintf("not started: dependency %q was not restored/started", members[dep].name)
				}
				continue
			}
			// Respect the captured run-state: a member stopped when it was backed up
			// is restored but not started (mirrors the single-container restore). It is
			// NOT blocked — a stopped-at-backup dependency doesn't hold back dependents.
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

// StartRestoreStack launches a stack restore in a background goroutine and
// returns immediately, mirroring StartRestore: the per-member restore + start
// loops run ON THE SERVER, detached from the request, so a multi-hour stack
// restore can't be killed by the browser/proxy dropping the idle HTTP
// connection. ALL validation (confirm, source, project, member enumeration)
// runs synchronously first, so a bad request — including an empty stack —
// still fails immediately with a clear error and no goroutine is started.
// Per-member outcomes land in the run history (each member's in-place restore
// records a kindRestore run via the orchestrator).
//
// Shares batchActive with backups and the other restores; returns (false, nil)
// when one is already running.
func (s *Service) StartRestoreStack(ctx context.Context, project, source string, startAfter, confirm bool) (bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	members, err := s.prepareRestoreStack(project, source, confirm)
	if err != nil {
		s.batchActive.Store(false)
		return false, err
	}
	// Detach so the run is independent of the request that started it, capped by
	// restoreTimeout (see its comment for why the restore cap is far more
	// generous than the backup one).
	bctx := context.WithoutCancel(ctx)
	// A stack restore has no aggregate progress bar; it is cancellable as a whole
	// under this synthetic key (the frontend cancel button targets it). Cancelling
	// aborts the member loop at the current member.
	key := "stack:" + project
	go func() {
		defer s.batchActive.Store(false)
		tctx, tcancel := context.WithTimeout(bctx, restoreTimeout)
		defer tcancel()
		rctx, cancel := context.WithCancel(tctx)
		defer cancel()
		s.registerCancel(key, cancel)
		defer s.unregisterCancel(key)
		res := s.runRestoreStack(rctx, members, source, startAfter)
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

// stackDepGraph maps each member to the indices of the OTHER in-stack members it
// depends on (via com.docker.compose.depends_on service names). Thin adapter over
// compose.DepGraph — see it for the edge/replica/self-dep semantics.
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

// stackStartOrder returns member indices in dependency order (a member's deps
// start before it). Thin adapter over compose.StartOrder — see it for the
// topological-sort and cycle-fallback semantics.
func stackStartOrder(members []stackMember) []int {
	return compose.StartOrder(memberServicesAndDeps(members))
}
