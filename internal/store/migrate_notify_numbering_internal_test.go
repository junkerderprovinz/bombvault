package store

import "testing"

// A build of the notification channels recorded notify_before_channels as 850,
// the number shared_repo_joins holds.
func TestADatabaseWithNotifyBeforeChannelsAt850StillGetsTheSharedRepoJoins(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("seed install: %v", err)
	}
	for _, q := range []string{
		`DROP TABLE shared_repo_joins`,
		`DROP TABLE shared_repo_server`,
		`DELETE FROM schema_migrations WHERE version IN (850, 900)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("unseed %q: %v", q, err)
		}
	}
	record(t, db, 850, "notify_before_channels")

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, table := range []string{"shared_repo_joins", "shared_repo_server", "notify_before_channels"} {
		if !hasTable(t, db, table) {
			t.Errorf("%s is missing", table)
		}
	}
	var name string
	if err := db.QueryRow(`SELECT name FROM schema_migrations WHERE version = 850`).Scan(&name); err != nil || name != "shared_repo_joins" {
		t.Errorf("850 is recorded as %q (%v), want shared_repo_joins", name, err)
	}
	if !applied(t, db, notifyChannelsMigration) {
		t.Errorf("v%d was not recorded", notifyChannelsMigration)
	}
}
