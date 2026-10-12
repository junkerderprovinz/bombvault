package store

import (
	"testing"
)

func TestAMemberThatPinnedAKeyIsNoLegacyMember(t *testing.T) {
	db := OpenMem(t)
	migrateThrough(t, db, legacyMembersMigration-1)
	if _, err := db.Exec(`INSERT INTO fleet_peers (id, member_id) VALUES ('p1', 'signs'), ('p2', 'old')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO member_keys (member_id, key, pinned_at) VALUES ('signs', x'01', 1)`); err != nil {
		t.Fatal(err)
	}
	migrateThrough(t, db, legacyMembersMigration)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("a second start: %v", err)
	}
	r := New(db)
	if legacy, err := r.LegacyMember("signs"); err != nil || legacy {
		t.Errorf("the member with a pinned key is legacy=%v (%v)", legacy, err)
	}
	if legacy, err := r.LegacyMember("old"); err != nil || !legacy {
		t.Errorf("the member without a key is legacy=%v (%v)", legacy, err)
	}
}
