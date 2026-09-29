package store

import (
	"fmt"
)

// StartTest is one start test of one container: its backup restored into an
// isolated copy, started and checked.
type StartTest struct {
	TargetID  string `json:"targetId"`
	Container string `json:"container"`
	At        int64  `json:"at"`
	OK        bool   `json:"ok"`
	Detail    string `json:"detail"`
	// Method is how the copy was judged: "health" (its Docker healthcheck),
	// "tcp" (a connection to its first exposed port) or "running" (neither
	// exists, so it had to stay up).
	Method     string `json:"method"`
	DurationMS int64  `json:"durationMs"`
	// Trigger is "schedule" or "manual".
	Trigger string `json:"trigger"`
}

// AddStartTest records a start test result.
func (r *Repo) AddStartTest(t StartTest) error {
	if t.Trigger == "" {
		t.Trigger = "manual"
	}
	_, err := r.db.Exec(`
		INSERT INTO start_tests (target_id, container, at, ok, detail, method, duration_ms, trigger)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		t.TargetID, t.Container, t.At, boolInt(t.OK), t.Detail, t.Method, t.DurationMS, t.Trigger,
	)
	if err != nil {
		return fmt.Errorf("AddStartTest: %w", err)
	}
	return nil
}

// LatestStartTests returns the newest start test of every container that has
// one, keyed by target id.
func (r *Repo) LatestStartTests() (map[string]StartTest, error) {
	rows, err := r.db.Query(`
		SELECT target_id, container, at, ok, detail, method, duration_ms, trigger
		FROM start_tests
		ORDER BY at, id`)
	if err != nil {
		return nil, fmt.Errorf("LatestStartTests: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	out := map[string]StartTest{}
	for rows.Next() {
		var t StartTest
		var ok int
		if err := rows.Scan(&t.TargetID, &t.Container, &t.At, &ok, &t.Detail, &t.Method, &t.DurationMS, &t.Trigger); err != nil {
			return nil, fmt.Errorf("LatestStartTests: %w", err)
		}
		t.OK = ok != 0
		out[t.TargetID] = t
	}
	return out, rows.Err()
}
