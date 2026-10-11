package store_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestADatabaseANewerVersionMigratedIsRefusedAndLeftAlone(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("a fresh database: %v", err)
	}
	if err := store.Migrate(db); err != nil {
		t.Fatalf("a database this version migrated: %v", err)
	}
	if err := store.New(db).RecordWriter("v10.4.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO schema_migrations (version, name, applied_at) VALUES (99999, 'from_the_future', 1)`); err != nil {
		t.Fatal(err)
	}
	var before int
	if err := db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&before); err != nil {
		t.Fatal(err)
	}

	err := store.Migrate(db)
	var newer *store.NewerDatabaseError
	if !errors.As(err, &newer) || newer.Version != 99999 || newer.Name != "from_the_future" || newer.Writer != "v10.4.0" {
		t.Fatalf("migrating a database of a newer version: %v", err)
	}
	if !strings.Contains(err.Error(), "Start BombVault v10.4.0 or newer") {
		t.Fatalf("the refusal does not say what to do: %v", err)
	}
	var after int
	if err := db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&after); err != nil || after != before {
		t.Fatalf("the refused database was changed: %d migrations before, %d after, %v", before, after, err)
	}
}

func TestARefusedDatabaseFromBeforeTheWriterWasRecordedSaysANewerVersion(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO schema_migrations (version, name, applied_at) VALUES (99999, 'from_the_future', 1)`); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(db); err == nil || !strings.Contains(err.Error(), "a newer version of BombVault wrote it") {
		t.Fatalf("a newer database without its writer: %v", err)
	}
}

func TestAMigrationRecordedUnderAnotherNumberIsNoReasonToRefuse(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE schema_migrations SET version = 99999 WHERE name = 'declined_backups'`); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(db); err != nil {
		t.Fatalf("a known migration under a number of a merged branch: %v", err)
	}
}
