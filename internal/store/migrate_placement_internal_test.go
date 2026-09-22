package store

import (
	"database/sql"
	"fmt"
	"maps"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// v8111Schema is the last migration a v8.11.1 database has recorded.
const v8111Schema = 108

func migrationNamed(t *testing.T, name string) migration {
	t.Helper()
	for _, m := range migrations {
		if m.name == name {
			return m
		}
	}
	t.Fatalf("no migration named %q", name)
	return migration{}
}

// withMigrations replaces the migration list for one test.
func withMigrations(t *testing.T, list []migration) {
	t.Helper()
	saved := migrations
	migrations = list
	t.Cleanup(func() { migrations = saved })
}

// migrateThrough runs Migrate with every migration up to and including version.
func migrateThrough(t *testing.T, db *sql.DB, version int) {
	t.Helper()
	saved := migrations
	defer func() { migrations = saved }()
	var list []migration
	for _, m := range saved {
		if m.version <= version {
			list = append(list, m)
		}
	}
	migrations = list
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate through v%d: %v", version, err)
	}
}

// seedV8111 builds a v8.11.1 database with a configuration that touches every
// table the placement migrations read.
func seedV8111(t *testing.T, db *sql.DB) {
	t.Helper()
	migrateThrough(t, db, v8111Schema)
	if _, err := db.Exec(`
UPDATE settings SET containers_offsite = 's3:b2/containers', files_offsite = 's3:b2/files' WHERE id = 1;
INSERT INTO offsite_targets (id, domain, name, repo, role, enabled, created_at, sort_order) VALUES
  ('c-field',   'containers', 'Primary',     's3:b2/containers',             'offsite', 1, 1000, 0),
  ('c-hetzner', 'containers', 'Hetzner',     'sftp:u1@hetzner:/containers',  'offsite', 1, 1100, 1),
  ('c-mesh',    'containers', 'mesh: tower', 'rest:http://tower:8000/c',     'offsite', 1, 1200, 0),
  ('f-field',   'files',      'Primary',     's3:b2/files',                  'offsite', 1, 1000, 0),
  ('r-nas',     '',           'NAS Keller',  'remotes/nas/bv',               'repo',    1, 1000, 0),
  ('r-box',     '',           'Storagebox',  'sftp:u1@storagebox:/bv',       'repo',    1, 1000, 0);
INSERT INTO targets (id, container_name, appdata_paths, created_at, repo) VALUES ('t-nginx', 'nginx', '[]', 1000, 'r-nas');
INSERT INTO vms (id, name, created_at) VALUES ('v-win11', 'win11', 1000);
INSERT INTO file_sets (id, name, path, created_at) VALUES ('fs-photos', 'Photos', 'user/photos', 1000);
INSERT INTO runs (id, target_id, kind, status, started_at, finished_at, completed)
  VALUES ('run-1', 't-nginx', 'backup', 'success', 2000, 2100, 1);
INSERT INTO offsite_runs (domain, started_at, finished_at, ok, offsite_target_id)
  VALUES ('containers', 2200, 2300, 1, 'c-field');`); err != nil {
		t.Fatalf("seed v8.11.1: %v", err)
	}
}

func sortOrders(t *testing.T, db *sql.DB) map[string]int {
	t.Helper()
	rows, err := db.Query(`SELECT id, sort_order FROM offsite_targets`)
	if err != nil {
		t.Fatalf("read sort_order: %v", err)
	}
	defer rows.Close() //nolint:errcheck // test cleanup
	got := map[string]int{}
	for rows.Next() {
		var id string
		var order int
		if err := rows.Scan(&id, &order); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got[id] = order
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	return got
}

func TestPrimarySlotMigrationMovesAV8111MeshRowBehind(t *testing.T) {
	db := OpenMem(t)
	seedV8111(t, db)
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	want := map[string]int{"c-field": 0, "c-hetzner": 1, "c-mesh": 2, "f-field": 0, "r-nas": 0, "r-box": 0}
	if got := sortOrders(t, db); !maps.Equal(got, want) {
		t.Fatalf("sort_order = %v, want %v", got, want)
	}
	field, ok, err := New(db).FieldOffsiteTarget("containers")
	if err != nil || !ok || field.ID != "c-field" {
		t.Fatalf("FieldOffsiteTarget(containers) = %q ok=%v err=%v, want c-field", field.ID, ok, err)
	}
}

// slotRow is one offsite_targets row a slotCase seeds.
type slotRow struct {
	id, domain, repo     string
	createdAt, sortOrder int
	enabled              int
}

// slotCase is one layout of a domain's targets before the primary slot is settled.
type slotCase struct {
	name  string
	field string
	rows  []slotRow
	want  map[string]int
}

var slotCases = []slotCase{
	{
		name:  "field row first, mesh row later on zero",
		field: "s3:b2",
		rows: []slotRow{
			{"a", "containers", "s3:b2", 1000, 0, 1},
			{"b", "containers", "sftp:h", 1100, 1, 1},
			{"m", "containers", "rest:m", 1200, 0, 1},
		},
		want: map[string]int{"a": 0, "b": 1, "m": 2},
	},
	{
		name:  "field row deleted by an earlier clear, mesh row alone on zero",
		field: "s3:b2",
		rows: []slotRow{
			{"b", "containers", "sftp:h", 1100, 1, 1},
			{"m", "containers", "rest:m", 1200, 0, 1},
		},
		want: map[string]int{"b": 1, "m": 2},
	},
	{
		name:  "field row moved behind by a PUT",
		field: "s3:b2",
		rows: []slotRow{
			{"m", "containers", "rest:m", 900, 0, 1},
			{"a", "containers", "s3:b2", 1000, 3, 1},
			{"n", "containers", "rest:n", 1300, 0, 1},
		},
		want: map[string]int{"a": 0, "m": 4, "n": 5},
	},
	{
		name:  "two rows name the field",
		field: "s3:b2",
		rows: []slotRow{
			{"late", "containers", "s3:b2", 2000, 0, 1},
			{"early", "containers", "s3:b2", 1000, 0, 1},
		},
		want: map[string]int{"early": 0, "late": 1},
	},
	{
		name:  "empty field, enabled row on zero moves away",
		field: "",
		rows: []slotRow{
			{"a", "containers", "s3:b2", 1000, 0, 1},
			{"b", "containers", "sftp:h", 1100, 1, 1},
		},
		want: map[string]int{"a": 2, "b": 1},
	},
	{
		name:  "empty field, switched-off row on zero keeps its slot",
		field: "",
		rows: []slotRow{
			{"a", "containers", "s3:b2", 1000, 0, 0},
			{"b", "containers", "sftp:h", 1100, 1, 1},
		},
		want: map[string]int{"a": 0, "b": 1},
	},
	{
		name:  "same creation time decided by id",
		field: "",
		rows: []slotRow{
			{"y", "containers", "rest:y", 1000, 0, 1},
			{"x", "containers", "rest:x", 1000, 0, 1},
			{"z", "containers", "sftp:z", 1000, 4, 1},
		},
		want: map[string]int{"x": 5, "y": 6, "z": 4},
	},
}

func seedSlotCase(t *testing.T, db *sql.DB, c slotCase) {
	t.Helper()
	if _, err := db.Exec(`UPDATE settings SET containers_offsite = ? WHERE id = 1`, c.field); err != nil {
		t.Fatal(err)
	}
	for _, row := range c.rows {
		if _, err := db.Exec(`INSERT INTO offsite_targets (id, domain, repo, created_at, sort_order, enabled) VALUES (?, ?, ?, ?, ?, ?)`,
			row.id, row.domain, row.repo, row.createdAt, row.sortOrder, row.enabled); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNormalizeOffsiteSortOrderMatchesMigration(t *testing.T) {
	slot := migrationNamed(t, "offsite_targets_primary_slot")
	for _, c := range slotCases {
		t.Run(c.name, func(t *testing.T) {
			migrated := OpenMem(t)
			migrateThrough(t, migrated, slot.version-1)
			seedSlotCase(t, migrated, c)
			if err := Migrate(migrated); err != nil {
				t.Fatalf("Migrate: %v", err)
			}

			normalized := OpenMem(t)
			if err := Migrate(normalized); err != nil {
				t.Fatalf("Migrate: %v", err)
			}
			seedSlotCase(t, normalized, c)
			if err := New(normalized).NormalizeOffsiteSortOrder("containers", c.field); err != nil {
				t.Fatalf("NormalizeOffsiteSortOrder: %v", err)
			}

			if got := sortOrders(t, migrated); !maps.Equal(got, c.want) {
				t.Errorf("migration: sort_order = %v, want %v", got, c.want)
			}
			if got := sortOrders(t, normalized); !maps.Equal(got, c.want) {
				t.Errorf("NormalizeOffsiteSortOrder: sort_order = %v, want %v", got, c.want)
			}
		})
	}
}

func TestPrimarySlotMigrationRunsOnceUnderAnyNumber(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	// A cleared field leaves its switched-off row on zero; running the body
	// again would move it and the next fill would mint a new target.
	if _, err := db.Exec(`INSERT INTO offsite_targets (id, domain, repo, enabled, created_at, sort_order)
		VALUES ('off', 'containers', 's3:b2', 0, 1000, 0), ('b', 'containers', 'sftp:h', 1, 1100, 1)`); err != nil {
		t.Fatal(err)
	}
	renumbered := migrationNamed(t, "offsite_targets_primary_slot")
	renumbered.version = len(migrations) + 1
	withMigrations(t, append(append([]migration(nil), migrations...), renumbered))
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if got := sortOrders(t, db)["off"]; got != 0 {
		t.Fatalf("switched-off field row moved to sort_order %d, want 0", got)
	}
	if name := appliedVersions(t, db)[renumbered.version]; name != "offsite_targets_primary_slot" {
		t.Fatalf("v%d recorded as %q, want offsite_targets_primary_slot", renumbered.version, name)
	}
}

// branchMigrations are this branch's migrations in order, each with a probe
// that is true once its body has taken effect.
var branchMigrations = []struct {
	name  string
	probe func(*sql.Tx) (bool, error)
}{
	{"offsite_targets_primary_slot", recordedAs("offsite_targets_primary_slot")},
	{"offsite_copy_rules", tablePresent("offsite_copy_rules")},
	{"placement_defaults", tablePresent("placement_defaults")},
	{"offsite_item_copies", tablePresent("offsite_item_copies")},
	{"offsite_observations", tablePresent("offsite_observations")},
	{"offsite_runs_aging_only", columnPresent("offsite_runs", "aging_only")},
	{"items_repo_chosen", columnPresent("targets", "repo_chosen")},
	{"placement_confirmed_manually", columnPresent("placement_defaults", "confirmed_manually")},
	{"offsite_targets_companion", columnPresent("offsite_targets", "companion_of")},
	{"offsite_targets_off_premises", columnPresent("offsite_targets", "off_premises")},
}

func probeOnce(t *testing.T, db *sql.DB, probe func(*sql.Tx) (bool, error)) bool {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback() //nolint:errcheck // a probe only reads
	ok, err := probe(tx)
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

// TestPlacementMigrationsSurviveRenumbering replays the rebase that puts two
// migrations of main in front of this branch: a database that ran the branch
// under the old numbers takes main's two and records every renumbered body
// without running it again.
func TestPlacementMigrationsSurviveRenumbering(t *testing.T) {
	db := OpenMem(t)
	seedV8111(t, db)
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	orders := sortOrders(t, db)
	first := migrationNamed(t, branchMigrations[0].name).version

	var list []migration
	for _, m := range migrations {
		if m.version < first {
			list = append(list, m)
		}
	}
	incoming := []migration{
		{version: first, name: "incoming_one", sql: `CREATE TABLE incoming_one (x INTEGER)`},
		{version: first + 1, name: "incoming_two", sql: `CREATE TABLE incoming_two (x INTEGER)`},
	}
	list = append(list, incoming...)
	for i, b := range branchMigrations {
		m := migrationNamed(t, b.name)
		m.version = first + len(incoming) + i
		list = append(list, m)
	}
	if _, err := db.Exec(`DELETE FROM schema_migrations
		WHERE version BETWEEN ? AND ? AND name NOT IN ('incoming_one', 'incoming_two')`,
		first, first+len(branchMigrations)-1); err != nil {
		t.Fatal(err)
	}
	withMigrations(t, list)
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate after the renumbering: %v", err)
	}

	applied := appliedVersions(t, db)
	for _, m := range incoming {
		if applied[m.version] != m.name || !probeOnce(t, db, tablePresent(m.name)) {
			t.Errorf("v%d recorded as %q, want %s run", m.version, applied[m.version], m.name)
		}
	}
	for i, b := range branchMigrations {
		v := first + len(incoming) + i
		if applied[v] != b.name {
			t.Errorf("v%d recorded as %q, want %s", v, applied[v], b.name)
		}
		if !probeOnce(t, db, b.probe) {
			t.Errorf("%s has not taken effect", b.name)
		}
	}
	if got := sortOrders(t, db); !maps.Equal(got, orders) {
		t.Errorf("sort_order = %v after the renumbering, want %v", got, orders)
	}
}

func TestAFreshDatabaseHasNoPlacementDefaults(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM placement_defaults`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("placement_defaults rows = %d (%v), want none", n, err)
	}
}

// TestAnExistingDatabaseKeepsItsReplicationThroughConfirmedDefaults pins the
// backfill against seedV8111's own shape: containers has a successful backup
// run and a successful off-site run, vms and files have only an item row and
// neither. Only containers may be grandfathered as confirmed; a domain that
// never actually replicated must not have the rebuild check retired for it.
func TestAnExistingDatabaseKeepsItsReplicationThroughConfirmedDefaults(t *testing.T) {
	db := OpenMem(t)
	seedV8111(t, db)
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	rows, err := db.Query(`SELECT domain, home, skip, confirmed_at FROM placement_defaults ORDER BY domain`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close() //nolint:errcheck // test cleanup
	var domains []string
	for rows.Next() {
		var domain, home, skip string
		var confirmed int64
		if err := rows.Scan(&domain, &home, &skip, &confirmed); err != nil {
			t.Fatal(err)
		}
		if home != "" || skip != "[]" || confirmed == 0 {
			t.Errorf("%s: home %q, skip %s, confirmed_at %d; want the domain path, every target, confirmed", domain, home, skip, confirmed)
		}
		domains = append(domains, domain)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(domains, []string{"containers"}) {
		t.Fatalf("defaults for %v, want containers alone: vms and files have no run of their own in seedV8111", domains)
	}
}

func TestExistingConfirmedDefaultsBackfillTheManualMarker(t *testing.T) {
	db := OpenMem(t)
	seedV8111(t, db)
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	rows, err := db.Query(`SELECT domain, confirmed_manually FROM placement_defaults`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close() //nolint:errcheck // test cleanup
	n := 0
	for rows.Next() {
		var domain string
		var manual int
		if err := rows.Scan(&domain, &manual); err != nil {
			t.Fatal(err)
		}
		if manual != 1 {
			t.Errorf("%s: confirmed_manually = %d, want 1: an install already replicating is not a rebuild to guard against", domain, manual)
		}
		n++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("seedV8111 left no placement_defaults rows to check")
	}
}

func TestThePlacementTablesStartEmpty(t *testing.T) {
	db := OpenMem(t)
	seedV8111(t, db)
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	for _, q := range []string{
		`SELECT count(*) FROM offsite_copy_rules`,
		`SELECT count(*) FROM offsite_item_copies`,
		`SELECT count(*) FROM offsite_observations`,
	} {
		var n int
		if err := db.QueryRow(q).Scan(&n); err != nil || n != 0 {
			t.Errorf("%s: %d rows (%v), want none", q, n, err)
		}
	}
	var agingOnly int
	if err := db.QueryRow(`SELECT aging_only FROM offsite_runs`).Scan(&agingOnly); err != nil || agingOnly != 0 {
		t.Errorf("aging_only of the seeded run = %d (%v), want 0", agingOnly, err)
	}
}

func TestItemsRepoChosenMarksEveryExistingRowChosen(t *testing.T) {
	db := OpenMem(t)
	seedV8111(t, db)
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	for _, table := range []string{"targets", "vms", "file_sets"} {
		var rows, chosen int
		if err := db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(repo_chosen = 1), 0) FROM `+table).Scan(&rows, &chosen); err != nil {
			t.Fatal(err)
		}
		if rows == 0 || chosen != rows {
			t.Errorf("%s: %d of %d rows chosen, want every row", table, chosen, rows)
		}
	}
}

func TestItemsRepoChosenDoesNotRunAgainUnderANewNumber(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO targets (id, container_name, appdata_paths, created_at) VALUES ('t1', 'nginx', '[]', 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE name = 'items_repo_chosen'`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate over a body that already ran: %v", err)
	}
	var choice int
	if err := db.QueryRow(`SELECT repo_chosen FROM targets WHERE id = 't1'`).Scan(&choice); err != nil {
		t.Fatal(err)
	}
	if choice != 0 {
		t.Fatalf("an open row reads %d: the body ran a second time", choice)
	}
}

func TestItemsRepoChosenGuardNeedsAllThreeTables(t *testing.T) {
	db := OpenMem(t)
	itemsRepoChosen := migrationNamed(t, "items_repo_chosen")
	migrateThrough(t, db, itemsRepoChosen.version-1)
	guard := itemsRepoChosen.alreadySatisfied
	if probeOnce(t, db, guard) {
		t.Fatal("the guard is satisfied before any of the three tables has the column")
	}
	if _, err := db.Exec(`ALTER TABLE targets ADD COLUMN repo_chosen INTEGER NOT NULL DEFAULT 1`); err != nil {
		t.Fatal(err)
	}
	if probeOnce(t, db, guard) {
		t.Fatal("the guard is satisfied with the column on targets alone")
	}
	if _, err := db.Exec(`
ALTER TABLE vms ADD COLUMN repo_chosen INTEGER NOT NULL DEFAULT 1;
ALTER TABLE file_sets ADD COLUMN repo_chosen INTEGER NOT NULL DEFAULT 1;`); err != nil {
		t.Fatal(err)
	}
	if !probeOnce(t, db, guard) {
		t.Fatal("the guard is not satisfied once every table has the column")
	}
}

func TestDatabaseBornAtIsTheFirstRecordedMigration(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := db.Exec(`UPDATE schema_migrations SET applied_at = 1000 WHERE version = 1`); err != nil {
		t.Fatal(err)
	}
	born, err := New(db).DatabaseBornAt()
	if err != nil || born.Unix() != 1000 {
		t.Fatalf("DatabaseBornAt = %v, %v, want 1000", born, err)
	}
}

func TestOffPremisesBackfillMatchesIsRemoteRepo(t *testing.T) {
	db := OpenMem(t)
	migrateThrough(t, db, migrationNamed(t, "offsite_targets_off_premises").version-1)
	locations := []string{
		"s3:https://s3.example.com/bv", "b2:bucket:bv", "rest:https://nas:8000/bv",
		"sftp:u@host:/bv", "rclone:remote:bv", "azure:container:/bv", "gs:bucket:/bv",
		"swift:container:/bv", "backups/named", "/mnt/remotes/nas/bv",
		"S3:https://s3.example.com/upper", "BackBlaze:bucket/cold",
	}
	for i, loc := range locations {
		if _, err := db.Exec(`INSERT INTO offsite_targets (id, domain, name, repo, role) VALUES (?, '', ?, ?, 'repo')`,
			fmt.Sprintf("repo-%d", i), loc, loc); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO offsite_targets (id, domain, name, repo, role)
		VALUES ('t1', 'containers', 'B2', 'b2:bucket:containers', 'offsite')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO offsite_targets (id, domain, name, repo, role, companion_of)
		VALUES ('d1', '', 'B2 direct', 'b2:bucket:containers-direct', 'repo', 't1')`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}

	rows, err := db.Query(`SELECT id, repo, role, companion_of, off_premises FROM offsite_targets`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close() //nolint:errcheck // test cleanup
	for rows.Next() {
		var id, repo, role, companion string
		var marked int
		if err := rows.Scan(&id, &repo, &role, &companion, &marked); err != nil {
			t.Fatal(err)
		}
		want := role == RoleRepo && companion == "" && restic.IsRemoteRepo(repo)
		if (marked != 0) != want {
			t.Errorf("%s (%s): off_premises = %d, want %v", id, repo, marked, want)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestOffPremisesMigrationMarksTheRemoteRepositoryOfAV8111Database(t *testing.T) {
	db := OpenMem(t)
	seedV8111(t, db)
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	for id, want := range map[string]int{"r-box": 1, "r-nas": 0, "c-hetzner": 0} {
		var marked int
		if err := db.QueryRow(`SELECT off_premises FROM offsite_targets WHERE id = ?`, id).Scan(&marked); err != nil {
			t.Fatal(err)
		}
		if marked != want {
			t.Errorf("%s: off_premises = %d, want %d", id, marked, want)
		}
	}
}

func TestOffPremisesMigrationLeavesASwitchedOffMarkWhenRenumbered(t *testing.T) {
	v := migrationNamed(t, "offsite_targets_off_premises").version
	db := OpenMem(t)
	migrateThrough(t, db, v)
	if _, err := db.Exec(`INSERT INTO offsite_targets (id, domain, name, repo, role, off_premises)
		VALUES ('lan', '', 'LAN rest', 'rest:http://nas:8000/bv', 'repo', 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE version = ?`, v); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("a body already applied under another number must not run again: %v", err)
	}
	var marked int
	if err := db.QueryRow(`SELECT off_premises FROM offsite_targets WHERE id = 'lan'`).Scan(&marked); err != nil {
		t.Fatal(err)
	}
	if marked != 0 {
		t.Fatal("the backfill ran again and marked a repository somebody had switched off")
	}
	if got := appliedVersions(t, db)[v]; got != "offsite_targets_off_premises" {
		t.Fatalf("version %d recorded as %q", v, got)
	}
}
