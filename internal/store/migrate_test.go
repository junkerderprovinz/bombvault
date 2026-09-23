package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestMigrateIdempotent(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	for _, tbl := range []string{"settings", "targets", "runs", "schema_migrations", "vms"} {
		var n int
		row := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", tbl)
		if err := row.Scan(&n); err != nil || n != 1 {
			t.Fatalf("table %s missing", tbl)
		}
	}
}

func TestDBDumpColumnsMigrate(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO targets (id, container_name, appdata_paths, include_in_schedule, created_at)
		VALUES ('t1', 'pg', '[]', 0, 1)`); err != nil {
		t.Fatalf("insert target: %v", err)
	}
	var off int
	var engine string
	if err := db.QueryRow(`SELECT db_dump_off, db_dump_engine FROM targets WHERE id = 't1'`).Scan(&off, &engine); err != nil {
		t.Fatalf("read dump columns: %v", err)
	}
	if off != 0 || engine != "" {
		t.Fatalf("defaults are db_dump_off=%d db_dump_engine=%q, want 0 and empty", off, engine)
	}
	var enabled int
	if err := db.QueryRow(`SELECT db_dumps_enabled FROM settings WHERE id = 1`).Scan(&enabled); err != nil {
		t.Fatalf("read settings column: %v", err)
	}
	if enabled != 1 {
		t.Fatalf("db_dumps_enabled default = %d, want 1", enabled)
	}
}

// A database born under a different numbering already carries the columns, and
// SQLite has no idempotent ADD COLUMN, so the guards have to recognise them.
func TestDBDumpColumnsMigrateOverExistingColumns(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE name IN
		('targets_db_dump_off', 'settings_db_dumps_enabled', 'targets_db_dump_engine')`); err != nil {
		t.Fatalf("forget the dump migrations: %v", err)
	}
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate over existing columns: %v", err)
	}
}

func TestMigrateAddsZFSSettingsColumnsWithDefaults(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	var enabled, immutable int
	var path, schedule, offsite, offsiteSchedule string
	err := db.QueryRow(`SELECT zfs_enabled, zfs_path, zfs_schedule, zfs_offsite, zfs_offsite_schedule, zfs_offsite_immutable
		FROM settings WHERE id = 1`).Scan(&enabled, &path, &schedule, &offsite, &offsiteSchedule, &immutable)
	if err != nil {
		t.Fatalf("read the zfs settings columns: %v", err)
	}
	if enabled != 0 {
		t.Fatalf("zfs_enabled = %d, want the domain off on an existing install", enabled)
	}
	if path != "user/bombvault/zfs" {
		t.Fatalf("zfs_path = %q, want user/bombvault/zfs", path)
	}
	if schedule != "off" {
		t.Fatalf("zfs_schedule = %q, want off", schedule)
	}
	if offsite != "" || offsiteSchedule != "" {
		t.Fatalf("zfs_offsite = %q, zfs_offsite_schedule = %q, want both empty", offsite, offsiteSchedule)
	}
	if immutable != 0 {
		t.Fatalf("zfs_offsite_immutable = %d, want 0", immutable)
	}
}

func TestMigrateCreatesZFSTables(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO zfs_datasets (id, dataset) VALUES ('z1', 'cache/appdata')`); err != nil {
		t.Fatalf("insert dataset: %v", err)
	}
	var enabled int
	var excludes, excludedChildren, cadence, repo, stop, pending, hookContainer, pre, post string
	var code, detail, mountpoint string
	var checkedAt, leftovers, leftoverAt, createdAt int64
	err := db.QueryRow(`SELECT enabled, excludes, excluded_children, schedule_cadence, repo, stop_containers,
		restart_pending, hook_container, pre_snapshot, post_snapshot, last_check_code, last_check_detail,
		last_check_at, last_host_mountpoint, leftover_count, leftover_checked_at, created_at
		FROM zfs_datasets WHERE id = 'z1'`).Scan(&enabled, &excludes, &excludedChildren, &cadence, &repo, &stop,
		&pending, &hookContainer, &pre, &post, &code, &detail, &checkedAt, &mountpoint, &leftovers, &leftoverAt, &createdAt)
	if err != nil {
		t.Fatalf("read dataset defaults: %v", err)
	}
	if enabled != 1 {
		t.Fatalf("enabled = %d, want a new item switched on", enabled)
	}
	if excludes != "[]" || excludedChildren != "[]" || stop != "[]" || pending != "[]" {
		t.Fatalf("json columns default to %q/%q/%q/%q, want empty arrays", excludes, excludedChildren, stop, pending)
	}
	for name, got := range map[string]string{
		"schedule_cadence":     cadence,
		"repo":                 repo,
		"hook_container":       hookContainer,
		"pre_snapshot":         pre,
		"post_snapshot":        post,
		"last_check_code":      code,
		"last_check_detail":    detail,
		"last_host_mountpoint": mountpoint,
	} {
		if got != "" {
			t.Fatalf("%s = %q, want empty", name, got)
		}
	}
	if checkedAt != 0 || leftovers != 0 || leftoverAt != 0 || createdAt != 0 {
		t.Fatalf("numeric defaults are %d/%d/%d/%d, want zeroes", checkedAt, leftovers, leftoverAt, createdAt)
	}

	if _, err := db.Exec(`INSERT INTO zfs_datasets (id, dataset) VALUES ('z2', 'cache/appdata')`); err == nil {
		t.Fatal("a second item on the same root dataset must be refused")
	}

	if _, err := db.Exec(`INSERT INTO zfs_members (item_id, dataset) VALUES ('z1', 'cache/appdata')`); err != nil {
		t.Fatalf("insert member: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO zfs_members (item_id, dataset) VALUES ('z1', 'cache/appdata')`); err == nil {
		t.Fatal("a member is keyed by item and dataset and must not be inserted twice")
	}
	var memberOutcome string
	var firstSeen, lastBackup, used int64
	if err := db.QueryRow(`SELECT outcome, first_seen_at, last_backup_at, used_by_dataset
		FROM zfs_members WHERE item_id = 'z1'`).Scan(&memberOutcome, &firstSeen, &lastBackup, &used); err != nil {
		t.Fatalf("read member defaults: %v", err)
	}
	if memberOutcome != "" || firstSeen != 0 || lastBackup != 0 || used != 0 {
		t.Fatalf("member defaults are %q/%d/%d/%d, want empty and zero", memberOutcome, firstSeen, lastBackup, used)
	}

	if _, err := db.Exec(`INSERT INTO zfs_runs (run_id, item_id) VALUES ('r1', 'z1')`); err != nil {
		t.Fatalf("insert run: %v", err)
	}
	var snapName, hookDetail string
	var window int64
	if err := db.QueryRow(`SELECT snapshot_name, window_seconds, hook_detail FROM zfs_runs WHERE run_id = 'r1'`).
		Scan(&snapName, &window, &hookDetail); err != nil {
		t.Fatalf("read run defaults: %v", err)
	}
	if snapName != "" || hookDetail != "" {
		t.Fatalf("run defaults are %q/%q, want empty", snapName, hookDetail)
	}
	if window != -1 {
		t.Fatalf("window_seconds = %d, want -1 for a run with no stop list", window)
	}

	if _, err := db.Exec(`INSERT INTO zfs_run_members (run_id, dataset, outcome) VALUES ('r1', 'cache/appdata', 'backed-up')`); err != nil {
		t.Fatalf("insert run member: %v", err)
	}
	var isNew int
	var snapshot string
	var bytesAdded, filesNew, filesChanged, filesUnmodified, durationMS int64
	if err := db.QueryRow(`SELECT is_new, restic_snapshot, bytes_added, files_new, files_changed, files_unmodified, duration_ms
		FROM zfs_run_members WHERE run_id = 'r1'`).Scan(&isNew, &snapshot, &bytesAdded, &filesNew, &filesChanged,
		&filesUnmodified, &durationMS); err != nil {
		t.Fatalf("read run member defaults: %v", err)
	}
	if isNew != 0 || snapshot != "" || bytesAdded != 0 || filesNew != 0 || filesChanged != 0 || filesUnmodified != 0 || durationMS != 0 {
		t.Fatal("a run member's counters must start empty")
	}

	if _, err := db.Exec(`INSERT INTO zfs_safety_snapshots (item_id, dataset, name, created_at)
		VALUES ('z1', 'cache/appdata', 'bombvault-prerestore-20240102030405', 1700000000)`); err != nil {
		t.Fatalf("insert safety snapshot: %v", err)
	}
	var usedBytes int64
	if err := db.QueryRow(`SELECT used_bytes FROM zfs_safety_snapshots WHERE name = 'bombvault-prerestore-20240102030405'`).
		Scan(&usedBytes); err != nil {
		t.Fatalf("read safety snapshot: %v", err)
	}
	if usedBytes != 0 {
		t.Fatalf("used_bytes = %d, want 0 before the first listing", usedBytes)
	}
}

func TestMigrateCreatesVMsTable(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	_, err := db.Exec(`INSERT INTO vms (id, name, method, include_in_schedule, definition, created_at)
		VALUES ('test-id', 'testvm', 'graceful', 0, '', 1234567890)`)
	if err != nil {
		t.Fatalf("vms table not created or wrong schema: %v", err)
	}
	var name string
	if err := db.QueryRow(`SELECT name FROM vms WHERE id = 'test-id'`).Scan(&name); err != nil {
		t.Fatalf("cannot read back: %v", err)
	}
	if name != "testvm" {
		t.Fatalf("name = %q, want testvm", name)
	}
}
