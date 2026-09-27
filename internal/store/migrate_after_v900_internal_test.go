package store

import (
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// BombVault 9.0.0 shipped every migration up to 153 except 109 to 118, which
// this release brings. Migrate keys on the number, so an instance on 9.0.0
// applies them on its next start, below versions it recorded long ago.

func isPlacementMigration(version int) bool { return version >= 109 && version <= 118 }

// bootV900 migrates db the way BombVault 9.0.0 did.
func bootV900(t *testing.T, db *sql.DB) {
	t.Helper()
	all := migrations
	defer func() { migrations = all }()
	var v900 []migration
	for _, m := range all {
		if !isPlacementMigration(m.version) && !isPlaceMigration(m.version) {
			v900 = append(v900, m)
		}
	}
	migrations = v900
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate as 9.0.0: %v", err)
	}
}

// seedV900 writes a 9.0.0 configuration touching every table the placement
// migrations alter or read: off-site targets of a placement domain and of the
// ZFS domain, named repositories on a local and a remote location, one item of
// each kind on its own repository, an alias and the runs a default is judged on.
func seedV900(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`
UPDATE settings SET containers_offsite = 's3:b2/containers', zfs_offsite = 's3:b2/zfs', db_dumps_enabled = 0 WHERE id = 1;
INSERT INTO offsite_targets (id, domain, name, repo, role, enabled, created_at, sort_order) VALUES
  ('c-field', 'containers', 'Primary',    's3:b2/containers',            'offsite', 1, 1000, 0),
  ('c-extra', 'containers', 'Hetzner',    'sftp:u1@hetzner:/containers', 'offsite', 1, 1100, 1),
  ('z-field', 'zfs',        'Primary',    's3:b2/zfs',                   'offsite', 1, 1000, 0),
  ('z-extra', 'zfs',        'Tower',      'rest:http://tower:8000/zfs',  'offsite', 1, 1100, 1),
  ('r-nas',   '',           'NAS',        'remotes/nas/bv',              'repo',    1, 1000, 0),
  ('r-box',   '',           'Storagebox', 'sftp:u1@storagebox:/bv',      'repo',    1, 1000, 0);
INSERT INTO targets (id, container_name, appdata_paths, created_at, repo, db_dump_off, db_dump_engine)
  VALUES ('t-pg', 'postgres', '[]', 1000, 'r-nas', 1, 'postgres');
INSERT INTO vms (id, name, created_at, uuid) VALUES ('v-win11', 'win11', 1000, 'b4f0c1d2');
INSERT INTO file_sets (id, name, path, created_at, repo) VALUES ('fs-photos', 'Photos', 'user/photos', 1000, 'r-box');
INSERT INTO zfs_datasets (id, dataset, repo, created_at) VALUES ('z-app', 'cache/appdata', 'r-nas', 1000);
INSERT INTO target_aliases (id, domain, old_name, target_id, linked_at, prev_definition)
  VALUES ('a-pg', 'container', 'pg', 't-pg', 1500, '');
INSERT INTO runs (id, target_id, kind, status, started_at, finished_at, completed)
  VALUES ('run-pg', 't-pg', 'backup', 'success', 2000, 2100, 1);
INSERT INTO offsite_runs (domain, started_at, finished_at, ok, offsite_target_id)
  VALUES ('zfs', 2200, 2300, 1, 'z-field');`); err != nil {
		t.Fatalf("seed 9.0.0: %v", err)
	}
}

// v900Tables are the tables seedV900 fills.
var v900Tables = []string{
	"settings", "offsite_targets", "targets", "vms", "file_sets", "zfs_datasets",
	"target_aliases", "runs", "offsite_runs",
}

func columnsOf(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?) ORDER BY cid`, table)
	if err != nil {
		t.Fatalf("columns of %s: %v", table, err)
	}
	defer rows.Close() //nolint:errcheck // test cleanup
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan column of %s: %v", table, err)
		}
		out = append(out, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("columns of %s: %v", table, err)
	}
	return out
}

// rowsOf renders a table's rows over cols, in rowid order.
func rowsOf(t *testing.T, db *sql.DB, table string, cols []string) []string {
	t.Helper()
	quoted := make([]string, len(cols))
	for i, c := range cols {
		quoted[i] = `"` + c + `"`
	}
	//nolint:gosec // G202: the table and column names come from this test and pragma_table_info
	rows, err := db.Query(`SELECT ` + strings.Join(quoted, ", ") + ` FROM ` + table + ` ORDER BY rowid`)
	if err != nil {
		t.Fatalf("rows of %s: %v", table, err)
	}
	defer rows.Close() //nolint:errcheck // test cleanup
	var out []string
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatalf("scan a row of %s: %v", table, err)
		}
		cells := make([]string, len(cols))
		for i, v := range vals {
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			cells[i] = fmt.Sprintf("%s=%v", cols[i], v)
		}
		out = append(out, strings.Join(cells, " "))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows of %s: %v", table, err)
	}
	return out
}

func intOf(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func TestPlacementMigrationsApplyToA900Database(t *testing.T) {
	db := OpenMem(t)
	bootV900(t, db)
	for v, name := range appliedVersions(t, db) {
		if isPlacementMigration(v) {
			t.Fatalf("the 9.0.0 seed already holds v%d (%s)", v, name)
		}
	}
	seedV900(t, db)

	cols := map[string][]string{}
	before := map[string][]string{}
	for _, table := range v900Tables {
		cols[table] = columnsOf(t, db, table)
		before[table] = rowsOf(t, db, table, cols[table])
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("a 9.0.0 database does not take the placement migrations, so the container would not boot: %v", err)
	}

	applied := appliedVersions(t, db)
	for _, m := range migrations {
		if applied[m.version] != m.name {
			t.Fatalf("v%d = %q, want %q recorded", m.version, applied[m.version], m.name)
		}
	}
	for _, table := range v900Tables {
		if after := rowsOf(t, db, table, cols[table]); !slices.Equal(after, before[table]) {
			t.Errorf("%s changed in the upgrade:\nbefore %v\nafter  %v", table, before[table], after)
		}
	}

	for _, table := range []string{"targets", "vms", "file_sets"} {
		//nolint:gosec // G202: the table names are the literals above
		if open := intOf(t, db, `SELECT count(*) FROM `+table+` WHERE repo_chosen <> 1`); open != 0 {
			t.Errorf("%s has %d rows left open; a row that existed before placement keeps its location", table, open)
		}
	}
	if got := intOf(t, db, `SELECT off_premises FROM offsite_targets WHERE id = 'r-box'`); got != 1 {
		t.Errorf("the remote repository reads off_premises=%d, want it counted as a site", got)
	}
	if got := intOf(t, db, `SELECT off_premises FROM offsite_targets WHERE id = 'r-nas'`); got != 0 {
		t.Errorf("the local repository reads off_premises=%d, want it on the premises", got)
	}
	if got := intOf(t, db, `SELECT count(*) FROM offsite_targets WHERE companion_of <> ''`); got != 0 {
		t.Errorf("%d rows became direct repositories in the upgrade", got)
	}
	var domains []string
	rows, err := db.Query(`SELECT domain FROM placement_defaults WHERE confirmed_at > 0 AND confirmed_manually = 1 ORDER BY domain`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close() //nolint:errcheck // test cleanup
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			t.Fatal(err)
		}
		domains = append(domains, d)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(domains, []string{"containers"}) {
		t.Errorf("confirmed defaults = %v, want containers alone: it is the one placement domain with a backup", domains)
	}

	fresh := OpenMem(t)
	if err := Migrate(fresh); err != nil {
		t.Fatalf("migrate a fresh database: %v", err)
	}
	if got, want := schemaFingerprint(t, db), schemaFingerprint(t, fresh); got != want {
		t.Errorf("the upgraded schema differs from a fresh install:\n got %s\nwant %s", got, want)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("second migrate after the upgrade: %v", err)
	}
}
