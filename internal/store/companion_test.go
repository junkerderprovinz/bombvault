package store_test

import (
	"database/sql"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func migratedStore(t *testing.T) (*store.Repo, *sql.DB) {
	t.Helper()
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return store.New(db), db
}

func TestNamedRepoReadsItsCompanionLink(t *testing.T) {
	r, db := migratedStore(t)
	if _, err := db.Exec(`INSERT INTO offsite_targets (id, domain, repo, role, companion_of, companion_lost)
		VALUES ('d', '', 'b2:bkt:x-direct', 'repo', 't1', 0), ('l', '', 'b2:bkt:y-direct', 'repo', '', 1)`); err != nil {
		t.Fatal(err)
	}
	d, err := r.GetNamedRepo("d")
	if err != nil || d.CompanionOf != "t1" || d.CompanionLost {
		t.Fatalf("direct row = %+v, %v", d, err)
	}
	l, err := r.GetNamedRepo("l")
	if err != nil || l.CompanionOf != "" || !l.CompanionLost {
		t.Fatalf("lost row = %+v, %v", l, err)
	}
}

func TestOneTargetHasAtMostOneDirectRepository(t *testing.T) {
	_, db := migratedStore(t)
	insert := `INSERT INTO offsite_targets (id, domain, repo, role, companion_of) VALUES (?, '', ?, 'repo', ?)`
	if _, err := db.Exec(insert, "a", "b2:bkt:a", "t1"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(insert, "b", "b2:bkt:b", "t1"); err == nil {
		t.Fatal("a second direct repository for the same target was stored")
	}
	for _, id := range []string{"c", "d"} {
		if _, err := db.Exec(insert, id, "b2:bkt:"+id, ""); err != nil {
			t.Fatalf("plain repositories share the empty link: %v", err)
		}
	}
}

func TestCompanionMigrationIsRecordedWhereTheColumnsExist(t *testing.T) {
	_, db := migratedStore(t)
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE name = 'offsite_targets_companion'`); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(db); err != nil {
		t.Fatalf("a database that already has the columns must record the migration: %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE name = 'offsite_targets_companion'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("recorded %d times, err %v", n, err)
	}
}
