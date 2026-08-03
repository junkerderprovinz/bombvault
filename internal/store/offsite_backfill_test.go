package store

import (
	"database/sql"
	"testing"
	"time"
)

// migrateThrough applies migrations up to and including maxVersion, recording
// them in schema_migrations exactly as Migrate does, so a later Migrate() call
// applies only the remaining ones. It lets a test reconstruct a pre-v75 install
// (schema + data) and then run the v75 backfill against it.
func migrateThrough(t *testing.T, db *sql.DB, maxVersion int) {
	t.Helper()
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at INTEGER NOT NULL)`); err != nil {
		t.Fatalf("migrateThrough: schema_migrations: %v", err)
	}
	for _, m := range migrations {
		if m.version > maxVersion {
			continue
		}
		if _, err := db.Exec(m.sql); err != nil {
			t.Fatalf("migrateThrough: apply v%d (%s): %v", m.version, m.name, err)
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
			m.version, m.name, time.Now().Unix()); err != nil {
			t.Fatalf("migrateThrough: record v%d: %v", m.version, err)
		}
	}
}

func TestOffsiteBackfillV75(t *testing.T) {
	db := OpenMem(t)
	// Reconstruct a pre-v75 install: everything up to v74, no offsite_targets yet.
	migrateThrough(t, db, 74)
	r := New(db)

	// Configure a single off-site repo for the containers domain, plus the GLOBAL
	// retention / limits / growth-budget the backfill copies. Leave every other
	// domain's off-site repo empty. Written with direct SQL against the v74 schema:
	// GetSettings/UpdateSettings track the CURRENT settings columns (including the
	// later v76 receiver_enabled), which do not exist in this reconstructed pre-v75
	// install.
	if _, err := db.Exec(`UPDATE settings SET
		containers_offsite            = ?,
		containers_offsite_schedule   = ?,
		containers_offsite_immutable  = 1,
		offsite_retention_keep_last   = 5,
		offsite_retention_keep_daily  = 10,
		offsite_retention_keep_weekly = 6,
		offsite_retention_keep_monthly = 9,
		offsite_limit_upload          = 1500,
		offsite_limit_download        = 3000,
		offsite_growth_budget_gb      = 42
	  WHERE id = 1`, "s3:https://example/containers", "daily 04:00"); err != nil {
		t.Fatal(err)
	}

	// A couple of history rows for containers that must be stamped with the new
	// target id, plus off-site history for a domain with NO off-site repo (vms),
	// which must stay unstamped.
	if _, err := r.RecordOffsiteRun("containers", time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err := r.RecordOffsiteRun("containers", time.Now().Unix()+1); err != nil {
		t.Fatal(err)
	}
	if err := r.RecordTamperTest("containers", true, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := r.RecordOffsiteRun("vms", time.Now().Unix()); err != nil {
		t.Fatal(err)
	}

	// Apply the remaining migrations (v75 = the backfill).
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate v75: %v", err)
	}

	// Exactly ONE target overall, for containers, with the copied values.
	all, err := r.ListOffsiteTargets()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("ListOffsiteTargets len = %d, want 1 (only containers has an off-site repo)", len(all))
	}
	ct := all[0]
	if ct.Domain != "containers" || ct.Name != "Primary" || ct.Repo != "s3:https://example/containers" ||
		ct.Schedule != "daily 04:00" || !ct.Immutable || !ct.Enabled || ct.SortOrder != 0 ||
		ct.RetentionKeepLast != 5 || ct.RetentionKeepDaily != 10 ||
		ct.RetentionKeepWeekly != 6 || ct.RetentionKeepMonthly != 9 ||
		ct.LimitUpload != 1500 || ct.LimitDownload != 3000 || ct.GrowthBudgetGB != 42 {
		t.Fatalf("backfilled target mismatch: %+v", ct)
	}
	if ct.CreatedAt == 0 {
		t.Fatalf("backfilled target created_at not stamped: %+v", ct)
	}
	// storage_class is intentionally left empty (lives in the encrypted cloud_conf
	// blob the pure-SQL migration cannot decode).
	if ct.StorageClass != "" {
		t.Fatalf("backfilled storage_class = %q, want empty (stage 2 copies it)", ct.StorageClass)
	}

	// The containers off-site history now carries the new target id.
	assertStamped := func(query, wantID string) {
		t.Helper()
		rows, err := db.Query(query)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close() //nolint:errcheck // completed query
		var seen int
		for rows.Next() {
			var got string
			if err := rows.Scan(&got); err != nil {
				t.Fatal(err)
			}
			if got != wantID {
				t.Fatalf("history offsite_target_id = %q, want %q (query %s)", got, wantID, query)
			}
			seen++
		}
		if seen == 0 {
			t.Fatalf("no rows for query %s", query)
		}
	}
	assertStamped(`SELECT offsite_target_id FROM offsite_runs WHERE domain='containers'`, ct.ID)
	assertStamped(`SELECT offsite_target_id FROM tamper_tests WHERE domain='containers'`, ct.ID)

	// The vms off-site run had no target (empty repo) and stays at the '' default.
	var vmsID string
	if err := db.QueryRow(`SELECT offsite_target_id FROM offsite_runs WHERE domain='vms'`).Scan(&vmsID); err != nil {
		t.Fatal(err)
	}
	if vmsID != "" {
		t.Fatalf("vms offsite_run should be unstamped, got %q", vmsID)
	}

	// Migrate is idempotent: a second call neither errors nor double-backfills.
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate idempotent: %v", err)
	}
	all2, err := r.ListOffsiteTargets()
	if err != nil {
		t.Fatal(err)
	}
	if len(all2) != 1 {
		t.Fatalf("after re-Migrate ListOffsiteTargets len = %d, want 1", len(all2))
	}
}
