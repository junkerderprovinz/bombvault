package api

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/backup"
)

// composeProject / composeService read the standard compose identity labels off a
// container's label map. "" when the label is absent (not a compose container).
func composeProject(labels map[string]string) string { return labels["com.docker.compose.project"] }
func composeService(labels map[string]string) string { return labels["com.docker.compose.service"] }

// parseDependsOn extracts the compose service names a container depends on, from
// the com.docker.compose.depends_on label. That label's format has varied across
// compose versions, so all three encodings are handled:
//   - JSON object: {"svc":{"condition":"..."}}                         -> object keys
//   - colon list:  "svc:service_started:true,svc2:service_healthy:false" -> part before first ':'
//   - plain list:  "svc,svc2"                                          -> as-is
//
// Names are trimmed and empties dropped. Returns nil when the label is
// absent/blank.
func parseDependsOn(labels map[string]string) []string {
	raw := strings.TrimSpace(labels["com.docker.compose.depends_on"])
	if raw == "" {
		return nil
	}
	// JSON forms start with a bracket: the modern object encoding
	// ({"svc":{...}}), or an array of names (["svc",...]). Parse those directly;
	// a bracketed-but-unparseable value returns nil rather than being fed to the
	// comma parser (which would turn "{...}"/"[...]" into garbage service names).
	if raw[0] == '{' || raw[0] == '[' {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal([]byte(raw), &obj); err == nil {
			deps := make([]string, 0, len(obj))
			for k := range obj {
				if k = strings.TrimSpace(k); k != "" {
					deps = append(deps, k)
				}
			}
			// Deterministic order (map iteration is random) so callers are stable.
			sort.Strings(deps)
			return deps
		}
		var arr []string
		if err := json.Unmarshal([]byte(raw), &arr); err == nil {
			deps := make([]string, 0, len(arr))
			for _, svc := range arr {
				if svc = strings.TrimSpace(svc); svc != "" {
					deps = append(deps, svc)
				}
			}
			return deps
		}
		return nil
	}
	// Comma-separated list; each item may carry ":condition:restart" suffixes, so
	// keep only the part before the first ':'. Covers the plain-list form too.
	var deps []string
	for _, part := range strings.Split(raw, ",") {
		svc := part
		if i := strings.IndexByte(svc, ':'); i >= 0 {
			svc = svc[:i]
		}
		if svc = strings.TrimSpace(svc); svc != "" {
			deps = append(deps, svc)
		}
	}
	return deps
}

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

// RestoreStack restores every backed-up container in the compose project: each is
// restored from its LATEST snapshot with leaveStopped=true (nothing starts during
// restore, so a dependent container can't start prematurely). When startAfter is
// true, members that restored OK are then started in dependency order
// (topological sort over com.docker.compose.depends_on; deps outside the stack are
// ignored; any cycle/unknown falls back to stable enumeration order). A single
// member's failure is recorded in its result and does NOT abort the others.
// Confirm must be true.
func (s *Service) RestoreStack(ctx context.Context, project, source string, startAfter, confirm bool) (StackRestoreResult, error) {
	if !confirm {
		return StackRestoreResult{}, backup.ErrNotConfirmed
	}
	if source != "local" && source != "offsite" {
		return StackRestoreResult{}, fmt.Errorf("invalid source (must be local or offsite)")
	}
	// Defense-in-depth: the project name flows into the member enumeration only
	// (never a filesystem path), but reject the obvious traversal tricks anyway.
	project = strings.TrimSpace(project)
	if project == "" {
		return StackRestoreResult{}, fmt.Errorf("stack name is required")
	}
	if strings.Contains(project, "/") || strings.Contains(project, "..") {
		return StackRestoreResult{}, fmt.Errorf("invalid stack name")
	}

	// Enumerate the members from the stored targets. ListTargets orders by
	// container_name, so this enumeration order is stable and alphabetical.
	targets, err := s.store.ListTargets()
	if err != nil {
		return StackRestoreResult{}, fmt.Errorf("list targets: %w", err)
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
		return StackRestoreResult{}, fmt.Errorf("no backed-up containers found in stack %q", project)
	}

	// Restore every member from its latest snapshot, leaving it stopped so a
	// dependent can't come up before its dependency is restored + started.
	results := make([]StackMemberResult, len(members))
	restoredOK := make([]bool, len(members))
	for i, m := range members {
		res := StackMemberResult{Name: m.name, Service: m.service}
		if rErr := s.Restore(ctx, m.name, "latest", true, source, true); rErr != nil {
			res.Error = rErr.Error()
		} else {
			res.Restored = true
			restoredOK[i] = true
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

	return StackRestoreResult{Members: results}, nil
}

// stackDepGraph maps each member to the indices of the OTHER in-stack members it
// depends on (via com.docker.compose.depends_on service names). A service name can
// resolve to MORE THAN ONE member (compose replicas / a shared service label), so
// every matching member becomes a dependency edge. Deps that name a service
// outside the stack, and self-deps, are ignored; edges are de-duplicated.
func stackDepGraph(members []stackMember) [][]int {
	svcIndex := make(map[string][]int, len(members))
	for i, m := range members {
		if m.service != "" {
			svcIndex[m.service] = append(svcIndex[m.service], i)
		}
	}
	graph := make([][]int, len(members))
	for i, m := range members {
		seen := make(map[int]bool)
		for _, d := range m.deps {
			for _, j := range svcIndex[d] {
				if j == i || seen[j] {
					continue // self-dep or duplicate edge
				}
				seen[j] = true
				graph[i] = append(graph[i], j)
			}
		}
	}
	return graph
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
// start before it) via Kahn's topological sort over the in-stack dependency graph.
// If a cycle leaves members unresolved, they are appended in their original
// enumeration order so every member is still returned exactly once.
func stackStartOrder(members []stackMember) []int {
	deps := stackDepGraph(members)
	indeg := make([]int, len(members))
	for i := range members {
		indeg[i] = len(deps[i])
	}
	// Kahn's algorithm: repeatedly emit a zero-in-degree member (lowest index
	// first, for determinism) and relax the members that depend on it.
	order := make([]int, 0, len(members))
	emitted := make([]bool, len(members))
	for len(order) < len(members) {
		progressed := false
		for i := range members {
			if emitted[i] || indeg[i] != 0 {
				continue
			}
			order = append(order, i)
			emitted[i] = true
			progressed = true
			// Relax dependents: any member that depends on i loses one in-degree.
			for k := range members {
				if emitted[k] {
					continue
				}
				for _, dj := range deps[k] {
					if dj == i {
						indeg[k]--
					}
				}
			}
		}
		if !progressed {
			break // a cycle remains — fall back to enumeration order below
		}
	}
	// Append any leftover (cycle) members in enumeration order.
	for i := range members {
		if !emitted[i] {
			order = append(order, i)
		}
	}
	return order
}
