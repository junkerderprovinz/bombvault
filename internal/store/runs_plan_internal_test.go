package store

import (
	"strings"
	"testing"
)

// The list of items runs these on every load over a table that keeps every run
// of every item, so neither may fall back to reading the table row by row.
func TestItemListQueriesStayOnTheRunIndexes(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name, query string
		args        []any
	}{
		{"RecentRunsByTarget", recentRunsByTargetQuery, []any{14, 14}},
		{"LatestSourceBytesByTarget", latestSourceBytesByTargetQuery, nil},
	}
	for _, c := range cases {
		rows, err := db.Query("EXPLAIN QUERY PLAN "+c.query, c.args...)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		var plan []string
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			plan = append(plan, detail)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		_ = rows.Close()

		seeks := false
		for _, step := range plan {
			if strings.HasPrefix(step, "SCAN runs") && !strings.Contains(step, "COVERING INDEX") {
				t.Fatalf("%s reads the runs table itself: %q in\n%s", c.name, step, strings.Join(plan, "\n"))
			}
			if strings.HasPrefix(step, "SEARCH runs USING") && strings.Contains(step, "idx_runs_target_kind_started (target_id=? AND kind=?)") {
				seeks = true
			}
		}
		if !seeks {
			t.Fatalf("%s does not seek a target's runs by kind:\n%s", c.name, strings.Join(plan, "\n"))
		}
	}
}
