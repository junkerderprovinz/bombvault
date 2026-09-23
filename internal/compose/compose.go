// Package compose reads the docker-compose labels of a container and orders
// containers by depends_on. Stack restore and the restart after a backup both
// use it, so they start containers in the same order. It imports only the
// standard library, so any package can depend on it.
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

// ParseDependsOn returns the compose services a container depends on, from the
// com.docker.compose.depends_on label. The label's format differs between
// compose versions, so three encodings are handled:
//   - JSON object: {"svc":{"condition":"..."}}                          -> object keys
//   - colon list:  "svc:service_started:true,svc2:service_healthy:false" -> part before first ':'
//   - plain list:  "svc,svc2"                                           -> as-is
//
// Names are trimmed and empty ones dropped. A missing or blank label gives nil.
func ParseDependsOn(labels map[string]string) []string {
	raw := strings.TrimSpace(labels["com.docker.compose.depends_on"])
	if raw == "" {
		return nil
	}
	// A bracketed value that is not valid JSON gives nil; the comma parser
	// would turn it into bogus service names.
	if raw[0] == '{' || raw[0] == '[' {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal([]byte(raw), &obj); err == nil {
			deps := make([]string, 0, len(obj))
			for k := range obj {
				if k = strings.TrimSpace(k); k != "" {
					deps = append(deps, k)
				}
			}
			// Map iteration order is random.
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
	// Items may carry ":condition:restart" suffixes.
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

// DepGraph maps each node to the indices of the other nodes it depends on,
// given each node's service name (services[i]) and the services it depends on
// (deps[i]). A service name can match several nodes, such as replicas, and
// each match becomes an edge. Dependencies outside the set and on the node
// itself are ignored, and edges are not repeated. services and deps must have
// the same length.
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

// StartOrder returns node indices so that a node's dependencies come before it
// (Kahn's algorithm over DepGraph). Nodes left in a cycle are appended in their
// original order, so every node is returned exactly once.
func StartOrder(services []string, deps [][]string) []int {
	graph := DepGraph(services, deps)
	indeg := make([]int, len(services))
	for i := range services {
		indeg[i] = len(graph[i])
	}
	// Lowest index first keeps the order deterministic.
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
			break // a cycle remains
		}
	}
	for i := range services {
		if !emitted[i] {
			order = append(order, i)
		}
	}
	return order
}
