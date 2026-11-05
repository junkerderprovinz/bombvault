package store

import (
	"database/sql"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func reusingMigrations(t *testing.T) {
	t.Helper()
	ReuseMigrations()
	t.Cleanup(func() { reuseMigrations.Store(false) })
}

// dumpDatabase reads the whole of db: its schema, and every row of every table,
// keyed by table and rowid.
func dumpDatabase(t *testing.T, db *sql.DB) map[string]map[string]string {
	t.Helper()
	dump := map[string]map[string]string{}
	schema := map[string]string{}
	rows, err := db.Query(`SELECT type || ' ' || name, COALESCE(sql, '') FROM sqlite_schema`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close() //nolint:errcheck // a completed query closes clean
	for rows.Next() {
		var name, text string
		if err := rows.Scan(&name, &text); err != nil {
			t.Fatal(err)
		}
		schema[name] = text
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	dump["sqlite_schema"] = schema

	var tables []string
	for name := range schema {
		if table, ok := strings.CutPrefix(name, "table "); ok && !strings.HasPrefix(table, "sqlite_") {
			tables = append(tables, table)
		}
	}
	for _, table := range tables {
		dump[table] = dumpTable(t, db, table)
	}
	return dump
}

func dumpTable(t *testing.T, db *sql.DB, table string) map[string]string {
	t.Helper()
	rows, err := db.Query(`SELECT * FROM "` + table + `"`) //nolint:gosec // G202: names from sqlite_schema
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close() //nolint:errcheck // a completed query closes clean
	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for n := 0; rows.Next(); n++ {
		values := make([]any, len(cols))
		targets := make([]any, len(cols))
		for i := range values {
			targets[i] = &values[i]
		}
		if err := rows.Scan(targets...); err != nil {
			t.Fatal(err)
		}
		for i, col := range cols {
			out[fmt.Sprintf("%d.%s", n, col)] = fmt.Sprintf("%v", values[i])
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// The image is older than the database it is compared with, so a date a
// migration writes shows up as a difference, and a random value does anyway. For
// a migration that adds one, startFromImage has to make it up again too, or two
// tests would share it.
func TestADatabaseFromTheImageHoldsWhatTheMigrationsBuild(t *testing.T) {
	if _, err := migratedImage(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)
	start := time.Now().Unix()

	real := OpenMem(t)
	if err := migrateSteps(real); err != nil {
		t.Fatal(err)
	}
	reusingMigrations(t)
	copied := OpenMem(t)
	if err := Migrate(copied); err != nil {
		t.Fatal(err)
	}
	want, got := dumpDatabase(t, real), dumpDatabase(t, copied)
	if !slices.Equal(slices.Sorted(maps.Keys(want)), slices.Sorted(maps.Keys(got))) {
		t.Fatalf("tables differ: %v against %v", slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(want)))
	}
	for table, rows := range want {
		for cell, value := range rows {
			if table == "schema_migrations" && strings.HasSuffix(cell, ".applied_at") || table == "group_state" && strings.HasSuffix(cell, ".instance_id") {
				continue
			}
			if other, ok := got[table][cell]; !ok || other != value {
				t.Errorf("%s %s = %q, the migrations build %q", table, cell, other, value)
			}
		}
		if len(rows) != len(got[table]) {
			t.Errorf("%s has %d cells, the migrations build %d", table, len(got[table]), len(rows))
		}
	}
	var oldest int64
	if err := copied.QueryRow(`SELECT MIN(applied_at) FROM schema_migrations`).Scan(&oldest); err != nil {
		t.Fatal(err)
	}
	if oldest < start {
		t.Fatalf("applied at %d, before this database existed at %d", oldest, start)
	}
}

func TestEachDatabaseFromTheImageIsItsOwn(t *testing.T) {
	reusingMigrations(t)
	one, other := OpenMem(t), OpenMem(t)
	for _, db := range []*sql.DB{one, other} {
		if err := Migrate(db); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := one.Exec(`INSERT INTO schema_migrations (version, name, applied_at) VALUES (999999, 'only here', 1)`); err != nil {
		t.Fatal(err)
	}
	var seen int
	if err := other.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version = 999999`).Scan(&seen); err != nil {
		t.Fatal(err)
	}
	if seen != 0 {
		t.Fatal("a write to one database reached the other")
	}
	third := OpenMem(t)
	if err := Migrate(third); err != nil {
		t.Fatal(err)
	}
	if err := third.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version = 999999`).Scan(&seen); err != nil {
		t.Fatal(err)
	}
	if seen != 0 {
		t.Fatal("a write to one database reached the image")
	}
	var a, b string
	if err := one.QueryRow(`SELECT instance_id FROM group_state`).Scan(&a); err != nil {
		t.Fatal(err)
	}
	if err := other.QueryRow(`SELECT instance_id FROM group_state`).Scan(&b); err != nil {
		t.Fatal(err)
	}
	if a == "" || a == b {
		t.Fatalf("instance IDs %q and %q, want one each", a, b)
	}
	var keys int
	if err := other.QueryRow(`PRAGMA foreign_keys`).Scan(&keys); err != nil {
		t.Fatal(err)
	}
	if keys != 1 {
		t.Fatal("the copy lost the foreign keys Open switched on")
	}
}

func TestADatabaseThatHoldsSomethingRunsTheMigrations(t *testing.T) {
	reusingMigrations(t)
	db := OpenMem(t)
	if _, err := db.Exec(`CREATE TABLE older (id INTEGER)`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	var older, applied int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_schema WHERE name = 'older'`).Scan(&older); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if older != 1 || applied != len(migrations) {
		t.Fatalf("older table %d, applied %d of %d, want it kept and every migration run", older, applied, len(migrations))
	}
}

func TestADatabaseInAFileRunsTheMigrations(t *testing.T) {
	reusingMigrations(t)
	path := filepath.Join(t.TempDir(), "bv.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close() //nolint:errcheck // read only
	var applied int
	if err := again.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != len(migrations) {
		t.Fatalf("applied %d of %d in the file", applied, len(migrations))
	}
}
