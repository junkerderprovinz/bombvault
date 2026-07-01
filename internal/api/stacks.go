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
	// JSON object form: keys are the service names.
	if strings.HasPrefix(raw, "{") {
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
		// Not valid JSON after all — fall through to the comma/colon parser.
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
	name    string
	service string
	deps    []string // compose service names this member depends_on
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
			name:    tg.ContainerName,
			service: composeService(labels),
			deps:    parseDependsOn(labels),
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
		for _, i := range order {
			if !restoredOK[i] {
				continue
			}
			if sErr := s.docker.Start(ctx, members[i].name); sErr != nil {
				// Don't overwrite a restore error (there shouldn't be one here, since
				// we only start restored members) — only record when Error is empty.
				if results[i].Error == "" {
					results[i].Error = sErr.Error()
				}
				continue // a start failure must NOT stop the loop
			}
			results[i].Started = true
		}
	}

	return StackRestoreResult{Members: results}, nil
}

// stackStartOrder returns member indices in dependency order (a member's deps
// start before it) via Kahn's topological sort over the in-stack compose service
// deps. Deps that name a service outside the stack are ignored. If a cycle leaves
// members unresolved, they are appended in their original enumeration order so
// every member is still returned exactly once.
func stackStartOrder(members []stackMember) []int {
	// Map compose service -> member index (only services that are members count).
	svcIndex := make(map[string]int, len(members))
	for i, m := range members {
		if m.service != "" {
			svcIndex[m.service] = i
		}
	}
	// Build the in-stack dependency edges + in-degrees.
	indeg := make([]int, len(members))
	deps := make([][]int, len(members)) // deps[i] = in-stack member indices i depends on
	for i, m := range members {
		seen := make(map[int]bool)
		for _, d := range m.deps {
			j, ok := svcIndex[d]
			if !ok || j == i || seen[j] {
				continue // dep outside the stack, self-dep, or duplicate
			}
			seen[j] = true
			deps[i] = append(deps[i], j)
			indeg[i]++
		}
	}
	// Kahn's algorithm: repeatedly emit a zero-in-degree member (lowest index
	// first, for determinism) and relax the members that depend on it.
	order := make([]int, 0, len(members))
	emitted := make([]bool, len(members))
	for len(order) < len(members) {
		progressed := false
		for i := 0; i < len(members); i++ {
			if emitted[i] || indeg[i] != 0 {
				continue
			}
			order = append(order, i)
			emitted[i] = true
			progressed = true
			// Relax dependents: any member that depends on i loses one in-degree.
			for k := 0; k < len(members); k++ {
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
	for i := 0; i < len(members); i++ {
		if !emitted[i] {
			order = append(order, i)
		}
	}
	return order
}
