package compose_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/compose"
)

func TestParseDependsOnEncodings(t *testing.T) {
	cases := []struct {
		name  string
		label string
		want  []string
	}{
		{"absent", "", nil},
		{"plain list", "db,cache", []string{"db", "cache"}},
		{"colon list", "db:service_healthy:false,cache:service_started:true", []string{"db", "cache"}},
		{"json object", `{"db":{"condition":"service_healthy"},"cache":{}}`, []string{"cache", "db"}}, // sorted
		{"json array", `["db","cache"]`, []string{"db", "cache"}},
		{"garbage json", `{not json`, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			labels := map[string]string{}
			if tc.label != "" {
				labels["com.docker.compose.depends_on"] = tc.label
			}
			got := compose.ParseDependsOn(labels)
			if len(got) != len(tc.want) {
				t.Fatalf("ParseDependsOn(%q) = %v, want %v", tc.label, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("ParseDependsOn(%q) = %v, want %v", tc.label, got, tc.want)
				}
			}
		})
	}
}

// StartOrder must place every dependency before the nodes that depend on it, for a
// db <- app <- web chain given in a scrambled input order.
func TestStartOrderDependenciesFirst(t *testing.T) {
	services := []string{"web", "app", "db"}
	deps := [][]string{{"app"}, {"db"}, nil}
	order := compose.StartOrder(services, deps)
	pos := make([]int, len(services))
	for p, idx := range order {
		pos[idx] = p
	}
	// indices: 0=web, 1=app, 2=db
	if pos[2] >= pos[1] || pos[1] >= pos[0] {
		t.Fatalf("StartOrder must be db(2) < app(1) < web(0), got positions %v (order %v)", pos, order)
	}
}

// A dependency cycle must not hang or drop nodes: every node is still returned
// exactly once (cycle members appended in original order).
func TestStartOrderCycleFallsBack(t *testing.T) {
	services := []string{"a", "b"}
	deps := [][]string{{"b"}, {"a"}} // a<->b cycle
	order := compose.StartOrder(services, deps)
	if len(order) != 2 {
		t.Fatalf("cycle must still return all nodes once, got %v", order)
	}
	seen := map[int]bool{}
	for _, i := range order {
		seen[i] = true
	}
	if !seen[0] || !seen[1] {
		t.Fatalf("cycle must return every node exactly once, got %v", order)
	}
}

// A depends_on that names a service outside the set produces no edge (it is
// ignored), and a shared service name resolves to every matching node.
func TestDepGraphExternalAndReplicaEdges(t *testing.T) {
	// two "db" replicas (0,1), one "app" (2) depending on db and on an out-of-set svc.
	services := []string{"db", "db", "app"}
	deps := [][]string{nil, nil, {"db", "external"}}
	g := compose.DepGraph(services, deps)
	if len(g[2]) != 2 {
		t.Fatalf("app must depend on both db replicas (external ignored), got %v", g[2])
	}
	if len(g[0]) != 0 || len(g[1]) != 0 {
		t.Fatalf("db replicas must have no deps, got %v / %v", g[0], g[1])
	}
}
