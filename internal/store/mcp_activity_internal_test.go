package store

import "testing"

// A key's log written before reads had a cap of their own keeps its starts,
// cancels and refusals under the larger cap, and only the reads that went
// through count as routine.
func TestUpgradeSortsAKeysLogIntoReadsAndTheRest(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`DROP INDEX idx_mcp_key_events_routine`,
		`ALTER TABLE mcp_key_events DROP COLUMN routine`,
		`DELETE FROM schema_migrations WHERE version > ?`,
	} {
		if _, err := db.Exec(stmt, mcpActivityMigration); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	for _, row := range [][3]string{
		{"get_activity", "ok", ""},
		{"start_backup", "ok", ""},
		{"cancel_backup", "ok", "run1"},
		{"get_status", "failed", ""},
		{"", "rate_limited", ""},
	} {
		if _, err := db.Exec(`INSERT INTO mcp_key_events (key_id, at, tool, outcome, run_id) VALUES ('k1', 2000, ?, ?, ?)`,
			row[0], row[1], row[2]); err != nil {
			t.Fatal(err)
		}
	}

	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}

	rows, err := db.Query(`SELECT tool, outcome FROM mcp_key_events WHERE routine = 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite
	var routine []string
	for rows.Next() {
		var tool, outcome string
		if err := rows.Scan(&tool, &outcome); err != nil {
			t.Fatal(err)
		}
		routine = append(routine, tool+" "+outcome)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(routine) != 1 || routine[0] != "get_activity ok" {
		t.Fatalf("routine rows = %v, want only the read that went through", routine)
	}
}
