package store

import (
	"slices"
	"testing"
	"time"
)

func TestTheUpgradeForgetsRestartRecordsADayOld(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	migrateThrough(t, db, staleRestartsMigration-1)
	now := time.Now().Unix()
	if _, err := db.Exec(`INSERT INTO container_restart_pending (name, added_at) VALUES ('old', ?), ('fresh', ?)`, now-3*86400, now-60); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	left, err := New(db).ListContainersStopped()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(left, []string{"fresh"}) {
		t.Fatalf("left %v, want only the record of a backup that died just now", left)
	}
}
