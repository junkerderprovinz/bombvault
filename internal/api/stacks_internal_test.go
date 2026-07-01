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
}
