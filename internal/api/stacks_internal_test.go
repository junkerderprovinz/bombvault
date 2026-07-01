package api

import (
	"reflect"
	"testing"
)

// TestParseDependsOn pins the three compose depends_on encodings (JSON object,
// colon-suffixed list, plain list) plus the empty/absent cases. parseDependsOn is
// unexported, so this lives in an internal (package api) test.
func TestParseDependsOn(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   []string
	}{
		{
			name:   "absent",
			labels: map[string]string{},
			want:   nil,
		},
		{
			name:   "blank",
			labels: map[string]string{"com.docker.compose.depends_on": "   "},
			want:   nil,
		},
		{
			name:   "plain list",
			labels: map[string]string{"com.docker.compose.depends_on": "db,cache"},
			want:   []string{"db", "cache"},
		},
		{
			name:   "colon list",
			labels: map[string]string{"com.docker.compose.depends_on": "db:service_started:true,cache:service_healthy:false"},
			want:   []string{"db", "cache"},
		},
		{
			name:   "json object",
			labels: map[string]string{"com.docker.compose.depends_on": `{"db":{"condition":"service_started"},"cache":{"condition":"service_healthy"}}`},
			want:   []string{"cache", "db"}, // sorted for determinism
		},
		{
			name:   "trims + drops empties",
			labels: map[string]string{"com.docker.compose.depends_on": " db , , cache "},
			want:   []string{"db", "cache"},
		},
		{
			name:   "json array of names",
			labels: map[string]string{"com.docker.compose.depends_on": `["db","cache"]`},
			want:   []string{"db", "cache"},
		},
		{
			name:   "bracketed but unparseable -> nil (not garbage)",
			labels: map[string]string{"com.docker.compose.depends_on": `{"db":`},
			want:   nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseDependsOn(tc.labels)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseDependsOn = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestStackStartOrder verifies the topological start order (deps start first) and
// the cycle fallback to enumeration order.
func TestStackStartOrder(t *testing.T) {
	// a depends_on b, b depends_on c, c none. Enumeration order is a,b,c;
	// dependency order must be c,b,a.
	members := []stackMember{
		{name: "a", service: "a", deps: []string{"b"}},
		{name: "b", service: "b", deps: []string{"c"}},
		{name: "c", service: "c"},
	}
	order := stackStartOrder(members)
	gotNames := make([]string, len(order))
	for i, idx := range order {
		gotNames[i] = members[idx].name
	}
	want := []string{"c", "b", "a"}
	if !reflect.DeepEqual(gotNames, want) {
		t.Fatalf("start order = %v, want %v", gotNames, want)
	}

	// External deps are ignored (dep on a service that is not a member).
	ext := []stackMember{
		{name: "x", service: "x", deps: []string{"not-in-stack"}},
		{name: "y", service: "y"},
	}
	if got := stackStartOrder(ext); len(got) != 2 {
		t.Fatalf("external-dep order length = %d, want 2 (%v)", len(got), got)
	}

	// A cycle (a<->b) must still return every member exactly once.
	cyc := []stackMember{
		{name: "a", service: "a", deps: []string{"b"}},
		{name: "b", service: "b", deps: []string{"a"}},
	}
	order = stackStartOrder(cyc)
	if len(order) != 2 {
		t.Fatalf("cycle order length = %d, want 2 (every member once)", len(order))
	}
	seen := map[int]bool{}
	for _, i := range order {
		if seen[i] {
			t.Fatalf("cycle order repeats index %d: %v", i, order)
		}
		seen[i] = true
	}

	// Diamond: d depends_on b + c, both depend_on a. Any valid order must place a
	// before b/c and b/c before d (exact positions of b vs c don't matter).
	diamond := []stackMember{
		{name: "d", service: "d", deps: []string{"b", "c"}},
		{name: "b", service: "b", deps: []string{"a"}},
		{name: "c", service: "c", deps: []string{"a"}},
		{name: "a", service: "a"},
	}
	assertDepsBeforeDependents(t, diamond, stackStartOrder(diamond))

	// Duplicate service label (compose replicas): two members share service "web",
	// and "app" depends_on "web" — so BOTH web members must start before app.
	dup := []stackMember{
		{name: "web-1", service: "web"},
		{name: "web-2", service: "web"},
		{name: "app", service: "app", deps: []string{"web"}},
	}
	assertDepsBeforeDependents(t, dup, stackStartOrder(dup))
}

// assertDepsBeforeDependents checks that in an acyclic graph every in-stack
// dependency of a member appears before it in the start order.
func assertDepsBeforeDependents(t *testing.T, members []stackMember, order []int) {
	t.Helper()
	pos := make(map[int]int, len(order))
	for p, idx := range order {
		pos[idx] = p
	}
	if len(pos) != len(members) {
		t.Fatalf("order must contain every member once, got %v", order)
	}
	deps := stackDepGraph(members)
	for i := range members {
		for _, j := range deps[i] {
			if pos[j] >= pos[i] {
				t.Fatalf("dependency %q must start before %q (order %v)", members[j].name, members[i].name, order)
			}
		}
	}
}
