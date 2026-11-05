package api

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/compose"
	"github.com/junkerderprovinz/bombvault/internal/dbdump"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const (
	// restoreStopTimeout is what a container gets to stop before files it uses are
	// written, the grace a backup gives it.
	restoreStopTimeout = 30 * time.Second
	// restoreRestartTimeout bounds starting containers again once a restore has
	// finished with them, after the request and any cancel.
	restoreRestartTimeout = 2 * time.Minute
)

// composeProject and composeService read the compose project and service labels,
// or "" for a container outside compose. The ordering logic lives in
// internal/compose, which the backup restart phase shares.
func composeProject(labels map[string]string) string { return compose.Project(labels) }
func composeService(labels map[string]string) string { return compose.Service(labels) }

// parseDependsOn returns the compose service names a container depends on.
func parseDependsOn(labels map[string]string) []string { return compose.ParseDependsOn(labels) }

// startsBefore reports whether a container stopped for in's backup comes up
// again before in: one in depends on by compose or a link, and a database
// next to an app that is none. Unraid templates on a custom network reach
// their database by name, which no label records.
func startsBefore(in model.Inspect, name string, other model.Inspect) bool {
	return dependsOnContainer(in, name, other) || (isDatabaseServer(other) && !isDatabaseServer(in))
}

// isDatabaseServer reports whether a container is recognised as a database
// server, as the database dumps recognise one.
func isDatabaseServer(in model.Inspect) bool {
	return dbdump.Resolve(in.Config.Image, envNames(in.Config.Env), in.Config.Labels, "").Engine != ""
}

// dependsOnContainer reports whether the container in needs other, named
// name: its compose file makes other's service a dependency, or a legacy link
// points at it.
func dependsOnContainer(in model.Inspect, name string, other model.Inspect) bool {
	project, service := composeProject(in.Config.Labels), composeService(other.Config.Labels)
	if project != "" && service != "" && composeProject(other.Config.Labels) == project &&
		slices.Contains(parseDependsOn(in.Config.Labels), service) {
		return true
	}
	for _, l := range in.HostConfig.Links {
		if linked, _, _ := strings.Cut(strings.TrimPrefix(l, "/"), ":"); linked == name {
			return true
		}
	}
	return false
}

// StackMemberResult is the per-container outcome of a stack restore.
type StackMemberResult struct {
	Name     string `json:"name"`
	Service  string `json:"service"`
	Restored bool   `json:"restored"`
	Started  bool   `json:"started"`
	Error    string `json:"error,omitempty"`
}

// StackRestoreResult is the result of RestoreStack: one entry per backed-up
// member, in enumeration order. FolderError says why the project folder did not
// come back.
type StackRestoreResult struct {
	Members     []StackMemberResult `json:"members"`
	FolderError string              `json:"folderError,omitempty"`
}

// stackMember is one container of a stack being restored.
type stackMember struct {
	name       string
	service    string
	deps       []string // compose service names this member depends_on
	wasRunning bool     // run-state captured at backup (def.Inspect.Running)
	// comesBackEmpty is a database a recreate brings back without its data,
	// see memberComesBackEmpty.
	comesBackEmpty bool
}

// volumeInspector finds a named volume on the host, with "" for one that is not
// there.
type volumeInspector interface {
	VolumeInspect(ctx context.Context, name string) (string, error)
}

// stackRestorePlan is a stack restore that passed its checks.
type stackRestorePlan struct {
	project string
	members []stackMember
	source  string
	// dirSource is where the project folder comes from, and hasDir whether it
	// holds one: older backups bring the folder back with the first member.
	dirSource string
	hasDir    bool
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

// planStackRestore runs everything that would make the restore of a member or
// of the project folder fail before anything is stopped or written, so a restore
// that cannot work leaves the stack as it is.
func (s *Service) planStackRestore(ctx context.Context, project string, members []stackMember, source, stackDirSource string) (stackRestorePlan, error) {
	ctx = planOnly(ctx)
	for i := range members {
		m := &members[i]
		plan, err := s.prepareRestore(ctx, m.name, "latest", true, source)
		if err == nil {
			err = s.guardStackMember(ctx, m.name, plan)
		}
		if err != nil {
			return stackRestorePlan{}, fmt.Errorf("%s: %w", m.name, err)
		}
		m.comesBackEmpty = s.memberComesBackEmpty(ctx, m.name, source, plan)
	}
	plan := stackRestorePlan{project: project, members: members, source: source, dirSource: cmp.Or(stackDirSource, source)}
	d, err := s.latestStackDir(ctx, project, plan.dirSource)
	if err != nil {
		return stackRestorePlan{}, fmt.Errorf("restore stack %s: %w", project, err)
	}
	plan.hasDir = d.found && len(d.snap.Paths) > 0
	if plan.hasDir {
		if err := s.guardStackDir(project, d); err != nil {
			return stackRestorePlan{}, err
		}
	}
	return plan, nil
}

// guardStackMember is what a member's restore refuses on once it has a plan.
func (s *Service) guardStackMember(ctx context.Context, name string, plan containerRestorePlan) error {
	if err := s.guardRecreateOverRunning(ctx, name, plan); err != nil {
		return err
	}
	if err := backup.CheckRestoreConflicts(ctx, s.docker, name, plan.inspect); err != nil {
		return err
	}
	return backup.CheckRestoreNetwork(ctx, s.docker, plan.inspect)
}

// memberComesBackEmpty reports a member a recreate brings back without its data:
// it has database dumps and no files backup, and neither the container nor a
// named volume of it is left on this host to hold the database. A dump only goes
// back through an import, which needs the started server, so such a member and
// what depends on it stay stopped and the run says so.
func (s *Service) memberComesBackEmpty(ctx context.Context, name, source string, plan containerRestorePlan) bool {
	if !plan.recreateOnly {
		return false
	}
	if _, found, err := s.liveContainer(ctx, name); err != nil || found {
		return false
	}
	if volumes, ok := s.docker.(volumeInspector); ok {
		for _, m := range plan.inspect.Mounts {
			if m.Name == "" {
				continue
			}
			if at, err := volumes.VolumeInspect(ctx, m.Name); err != nil || at != "" {
				return false
			}
		}
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		log.Printf("api: restore stack: read settings: %v", err)
		return false
	}
	dumps, _, err := s.dbDumpsOf(ctx, settings, name, source)
	if err != nil {
		log.Printf("api: restore stack: the database dumps of %q: %v", name, err) //nolint:gosec // G706: name is %q-quoted
		return false
	}
	return len(dumps) > 0
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
	plan, err := s.planStackRestore(ctx, strings.TrimSpace(project), members, source, stackDirSource)
	if err != nil {
		return StackRestoreResult{}, err
	}
	return s.runRestoreStack(ctx, plan, startAfter), nil
}

// restoreStackMember restores one member and turns a panic into that member's
// error, so the members behind it and the start loop still run. A restore calls
// store.StartRun without a defer, so the handler also fails the run that would
// otherwise stay "running".
func (s *Service) restoreStackMember(ctx context.Context, name, source string, notes []string) (err error) {
	// recoverOperation must be deferred directly (see backupOneForBatch).
	defer s.recoverOperation("restore stack: "+name, &err, func(msg string) {
		if tg, tErr := s.store.GetTargetByContainer(name); tErr == nil {
			s.failStuckRun(tg.ID, msg)
		}
	})
	plan, err := s.prepareRestore(ctx, name, "latest", true, source)
	if err != nil {
		return err
	}
	plan.notes = append(plan.notes, notes...)
	return s.executeRestore(ctx, name, plan, true)
}

// stopStackMembers stops the running members, the dependents first, and returns
// the ones it stopped. A member that keeps running is an error: restic must not
// write under it.
func (s *Service) stopStackMembers(ctx context.Context, members []stackMember) ([]string, error) {
	self := s.selfContainerName(ctx)
	order := stackStartOrder(members)
	slices.Reverse(order)
	var stopped []string
	for _, i := range order {
		name := members[i].name
		if name == self {
			continue
		}
		in, err := s.docker.Inspect(ctx, name)
		switch {
		case dockercli.IsNotFound(err):
			continue
		case err != nil:
			return stopped, fmt.Errorf("inspect %s: %w", name, err)
		case !in.Running:
			continue
		}
		stopped = append(stopped, name)
		if err := s.docker.Stop(ctx, name, restoreStopTimeout); err != nil {
			return stopped, fmt.Errorf("stop %s: %w", name, err)
		}
	}
	return stopped, nil
}

// startStoppedAgain starts stopped containers in the opposite order, when a
// restore ends before it wrote anything.
func (s *Service) startStoppedAgain(ctx context.Context, stopped []string) {
	cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), restoreRestartTimeout)
	defer cancel()
	for _, name := range slices.Backward(stopped) {
		if err := s.docker.Start(cctx, name); err != nil {
			log.Printf("api: restore stack: start %q again: %v", name, err) //nolint:gosec // G706: name is %q-quoted
		}
	}
}

// endStackBeforeMembers records err as the outcome of every member, whose
// restores never ran, so the run history and the open window hear about it.
func (s *Service) endStackBeforeMembers(members []stackMember, err error) StackRestoreResult {
	results := make([]StackMemberResult, len(members))
	for i, m := range members {
		results[i] = StackMemberResult{Name: m.name, Service: m.service, Error: err.Error()}
		s.finishRestoreRun(s.beginRestoreRun(m.name), "", err)
	}
	return StackRestoreResult{Members: results}
}

// runRestoreStack restores the project folder and the members, then with
// startAfter starts them in dependency order. Each member's restore records its
// own run, so the outcomes stay visible when this runs detached from the request.
func (s *Service) runRestoreStack(ctx context.Context, plan stackRestorePlan, startAfter bool) StackRestoreResult {
	members, source := plan.members, plan.source
	var folderErr error
	if plan.hasDir {
		// The folder can hold a member's data as a bind mount, which must not
		// change under a running database. It comes before the members: it holds
		// the compose file and shared files one may read on start.
		stopped, err := s.stopStackMembers(ctx, members)
		if err != nil {
			s.startStoppedAgain(ctx, stopped)
			return s.endStackBeforeMembers(members, fmt.Errorf("nothing was restored: %w", err))
		}
		if _, folderErr = s.RestoreStackDir(ctx, plan.project, plan.dirSource); folderErr != nil {
			if errors.Is(folderErr, context.Canceled) {
				return s.endStackBeforeMembers(members, folderErr)
			}
			log.Printf("api: restore stack: %v", folderErr)
		}
	}

	// Leave every member stopped, so a dependent cannot come up before its
	// dependency is restored and started. Dependencies are recreated first, so
	// a legacy link to one of them resolves.
	results := make([]StackMemberResult, len(members))
	restoredOK := make([]bool, len(members))
	for _, i := range stackStartOrder(members) {
		m := members[i]
		res := StackMemberResult{Name: m.name, Service: m.service}
		var notes []string
		if folderErr != nil {
			notes = append(notes, store.NoteStackFolderFailed+": "+truncateRunErr(folderErr))
		}
		if m.comesBackEmpty {
			notes = append(notes, store.NoteRestoredWithoutData)
		}
		rErr := s.restoreStackMember(ctx, m.name, source, notes)
		switch {
		case rErr == nil:
			res.Restored = true
			restoredOK[i] = true
		case errors.Is(rErr, context.Canceled):
			// A cancel stops the stack restore at this member. Its run is recorded
			// as cancelled; the remaining members and the start loop are skipped.
			res.Error = rErr.Error()
			results[i] = res
			return StackRestoreResult{Members: reached(results)}
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
			// Their runs already say why: the folder may be half written, or the
			// database has no data for the apps to run on.
			if folderErr != nil || members[i].comesBackEmpty {
				blocked[i] = true
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
		for i, r := range results {
			if restoredOK[i] && blocked[i] && r.Error != "" {
				s.noteOnRestoreRun(r.Name, store.NoteStackNotStarted+": "+r.Error)
			}
		}
	}

	out := StackRestoreResult{Members: results}
	if folderErr != nil {
		out.FolderError = truncateRunErr(folderErr)
	}
	return out
}

// noteOnRestoreRun puts a note on the newest restore run of a stack's member,
// which has closed by the time the start loop knows what became of it.
func (s *Service) noteOnRestoreRun(name, note string) {
	tg, err := s.store.GetTargetByContainer(name)
	if err != nil {
		log.Printf("api: restore stack: no target for %q to carry the note: %v", name, err) //nolint:gosec // G706: name is %q-quoted
		return
	}
	run, err := s.store.LastRestoreRun(tg.ID)
	if err != nil || run == nil {
		log.Printf("api: restore stack: no run of %q to carry the note: %v", name, err) //nolint:gosec // G706: name is %q-quoted
		return
	}
	if err := s.store.AddRunNote(run.ID, note); err != nil {
		log.Printf("api: restore stack: %v", err)
	}
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
	plan, err := s.planStackRestore(ctx, strings.TrimSpace(project), members, source, stackDirSource)
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
		res := s.runRestoreStack(rctx, plan, startAfter)
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

// reached keeps the results of the members a cancelled stack restore got to,
// in name order.
func reached(results []StackMemberResult) []StackMemberResult {
	var out []StackMemberResult
	for _, r := range results {
		if r.Name != "" {
			out = append(out, r)
		}
	}
	return out
}
