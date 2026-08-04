// Package compose holds the pure, dependency-free helpers for reading the
// standard docker-compose identity labels off a container and ordering a set of
// containers by their depends_on relationships. It is shared by the stack
// restore (which starts a compose project in dependency order) and the
// backup restart phase (which restarts the containers it stopped in that same
// order), so both use one topological sort instead of two drifting copies.
//
// It imports only the standard library, so any package (backup, api, ...) can
// depend on it without an import cycle.
package compose

import (
	"encoding/json"
	"sort"
	"strings"
)

// Project reads the compose project name from a container's label map, or ""
// when the label is absent (not a compose container).
func Project(labels map[string]string) string { return labels["com.docker.compose.project"] }

// Service reads the compose service name from a container's label map, or ""
// when the label is absent (not a compose container).
func Service(labels map[string]string) string { return labels["com.docker.compose.service"] }

// ParseDependsOn extracts the compose service names a container depends on, from
// the com.docker.compose.depends_on label. That label's format has varied across
// compose versions, so all three encodings are handled:
//   - JSON object: {"svc":{"condition":"..."}}                          -> object keys
//   - colon list:  "svc:service_started:true,svc2:service_healthy:false" -> part before first ':'
//   - plain list:  "svc,svc2"                                           -> as-is
//
// Names are trimmed and empties dropped. Returns nil when the label is
// absent/blank.
func ParseDependsOn(labels map[string]string) []string {
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

// DepGraph maps each node to the indices of the OTHER nodes it depends on, given
// each node's compose service name (services[i]) and the service names it
// depends_on (deps[i]). A service name can resolve to MORE THAN ONE node
// (compose replicas / a shared service label), so every matching node becomes a
// dependency edge. Deps that name a service outside the set, and self-deps, are
// ignored; edges are de-duplicated. services and deps must be the same length.
func DepGraph(services []string, deps [][]string) [][]int {
	svcIndex := make(map[string][]int, len(services))
	for i, svc := range services {
		if svc != "" {
			svcIndex[svc] = append(svcIndex[svc], i)
		}
	}
	graph := make([][]int, len(services))
	for i := range services {
		seen := make(map[int]bool)
		for _, d := range deps[i] {
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

// StartOrder returns node indices in dependency order (a node's deps come before
// it) via Kahn's topological sort over the dependency graph derived from services
// and deps. If a cycle leaves nodes unresolved, they are appended in their
// original order so every node is still returned exactly once.
func StartOrder(services []string, deps [][]string) []int {
	graph := DepGraph(services, deps)
	indeg := make([]int, len(services))
	for i := range services {
		indeg[i] = len(graph[i])
	}
	// Kahn's algorithm: repeatedly emit a zero-in-degree node (lowest index
	// first, for determinism) and relax the nodes that depend on it.
	order := make([]int, 0, len(services))
	emitted := make([]bool, len(services))
	for len(order) < len(services) {
		progressed := false
		for i := range services {
			if emitted[i] || indeg[i] != 0 {
				continue
			}
			order = append(order, i)
			emitted[i] = true
			progressed = true
			// Relax dependents: any node that depends on i loses one in-degree.
			for k := range services {
				if emitted[k] {
					continue
				}
				for _, dj := range graph[k] {
					if dj == i {
						indeg[k]--
					}
				}
			}
		}
		if !progressed {
			break // a cycle remains — fall back to original order below
		}
	}
	// Append any leftover (cycle) nodes in original order.
	for i := range services {
		if !emitted[i] {
			order = append(order, i)
		}
	}
	return order
}
