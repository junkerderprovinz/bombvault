package store

import (
	"fmt"
)

// MCPKeyEvent is one tool call a key made, or one of its requests the endpoint
// refused before any tool ran, which leaves Tool empty. Outcome is "ok" or the
// refusal code, and RunID the run a cancel named. Routine marks a read that
// went through, which is kept under a cap of its own. The arguments of a call
// are never kept, and neither is anything about the key but its id.
type MCPKeyEvent struct {
	At      int64  `json:"at"`
	Tool    string `json:"tool"`
	Outcome string `json:"outcome"`
	RunID   string `json:"runId"`
	Routine bool   `json:"-"`
}

// How much of a key's activity is kept: the newest MCPKeyReadsKept routine
// reads and the newest MCPKeyEventsKept other events of each key, none older
// than MCPKeyEventsMaxAge seconds. An assistant polling a running backup makes
// hundreds of reads, and a cap they shared would push the backup's start and
// cancel out within minutes. Both stay small enough that a looping client costs
// a fixed number of rows.
const (
	MCPKeyEventsKept   = 500
	MCPKeyReadsKept    = 200
	MCPKeyEventsMaxAge = 30 * 24 * 60 * 60
)

// The calls of a key are also counted in quarter-hour slots, which outlive the
// event cap, so "calls today" stays right for a busy key. A quarter hour is the
// finest offset a time zone's midnight has. MCPKeyCallsCovered is how far back
// the slots reach, two days, which holds today in any zone.
const (
	mcpCallSlot        = 15 * 60
	MCPKeyCallsCovered = 2 * 24 * 60 * 60
)

// RecordMCPKeyEvent stores one event and prunes in the same step: the key's
// events of the same kind beyond their cap, and everyone's events and call
// slots past their age.
func (r *Repo) RecordMCPKeyEvent(keyID string, e MCPKeyEvent) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("RecordMCPKeyEvent: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after a successful commit is a no-op

	kept := MCPKeyEventsKept
	if e.Routine {
		kept = MCPKeyReadsKept
	}
	if _, err := tx.Exec(`INSERT INTO mcp_key_events (key_id, at, tool, outcome, run_id, routine) VALUES (?, ?, ?, ?, ?, ?)`,
		keyID, e.At, e.Tool, e.Outcome, e.RunID, e.Routine); err != nil {
		return fmt.Errorf("RecordMCPKeyEvent: %w", err)
	}
	if e.Tool != "" {
		if _, err := tx.Exec(`INSERT INTO mcp_key_calls (key_id, slot, calls) VALUES (?, ?, 1)
			ON CONFLICT (key_id, slot) DO UPDATE SET calls = calls + 1`,
			keyID, e.At-e.At%mcpCallSlot); err != nil {
			return fmt.Errorf("RecordMCPKeyEvent calls: %w", err)
		}
	}
	if _, err := tx.Exec(`DELETE FROM mcp_key_events WHERE key_id = ? AND routine = ? AND id NOT IN (
			SELECT id FROM mcp_key_events WHERE key_id = ? AND routine = ? ORDER BY id DESC LIMIT ?)`,
		keyID, e.Routine, keyID, e.Routine, kept); err != nil {
		return fmt.Errorf("RecordMCPKeyEvent cap: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM mcp_key_events WHERE at < ?`, e.At-MCPKeyEventsMaxAge); err != nil {
		return fmt.Errorf("RecordMCPKeyEvent age: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM mcp_key_calls WHERE slot < ?`, e.At-MCPKeyCallsCovered); err != nil {
		return fmt.Errorf("RecordMCPKeyEvent slots: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("RecordMCPKeyEvent commit: %w", err)
	}
	return nil
}

// MCPKeyEvents returns the events kept for one key, newest first.
func (r *Repo) MCPKeyEvents(keyID string) ([]MCPKeyEvent, error) {
	rows, err := r.db.Query(`SELECT at, tool, outcome, run_id FROM mcp_key_events
		WHERE key_id = ? ORDER BY id DESC`, keyID)
	if err != nil {
		return nil, fmt.Errorf("MCPKeyEvents: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	out := []MCPKeyEvent{}
	for rows.Next() {
		var e MCPKeyEvent
		if err := rows.Scan(&e.At, &e.Tool, &e.Outcome, &e.RunID); err != nil {
			return nil, fmt.Errorf("MCPKeyEvents: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("MCPKeyEvents: %w", err)
	}
	return out, nil
}

// MCPKeyCallsSince counts each key's tool calls in the slots that begin at or
// after since.
func (r *Repo) MCPKeyCallsSince(since int64) (map[string]int, error) {
	rows, err := r.db.Query(`SELECT key_id, sum(calls) FROM mcp_key_calls WHERE slot >= ? GROUP BY key_id`, since)
	if err != nil {
		return nil, fmt.Errorf("MCPKeyCallsSince: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	out := map[string]int{}
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, fmt.Errorf("MCPKeyCallsSince: %w", err)
		}
		out[id] = n
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("MCPKeyCallsSince: %w", err)
	}
	return out, nil
}

// MCPKeyEventTotals counts the events kept across all keys and how many of
// them were refusals, which is all the diagnostics bundle says about them.
func (r *Repo) MCPKeyEventTotals() (events, refusals int, err error) {
	err = r.db.QueryRow(`SELECT count(*), coalesce(sum(outcome != 'ok'), 0) FROM mcp_key_events`).Scan(&events, &refusals)
	if err != nil {
		return 0, 0, fmt.Errorf("MCPKeyEventTotals: %w", err)
	}
	return events, refusals, nil
}

// RunsStartedByMCPKey returns the newest runs a key started, running ones
// included.
func (r *Repo) RunsStartedByMCPKey(keyID string, limit int) ([]Run, error) {
	rows, err := r.db.Query(`SELECT `+runCols+` FROM runs
		WHERE started_via_key = ? ORDER BY started_at DESC, rowid DESC LIMIT ?`, keyID, limit)
	if err != nil {
		return nil, fmt.Errorf("RunsStartedByMCPKey: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	out := []Run{}
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, fmt.Errorf("RunsStartedByMCPKey: %w", err)
		}
		out = append(out, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("RunsStartedByMCPKey: %w", err)
	}
	return out, nil
}
