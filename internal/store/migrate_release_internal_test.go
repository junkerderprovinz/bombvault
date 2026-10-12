package store

import (
	"strings"
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

func TestATargetWhoseNameSpansLinesCanStillBeSaved(t *testing.T) {
	db := OpenMem(t)
	migrateThrough(t, db, oneLineMigration-1)
	r := New(db)
	stored, err := r.CreateOffsiteTarget(OffsiteTarget{Domain: "files", Name: "Two\nlines", Repo: "rest:https://box:8000/files", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("a second start: %v", err)
	}

	stored.Schedule = "daily 03:00"
	if _, err := r.UpsertOffsiteTarget(stored); err != nil {
		t.Fatalf("saving another field of the target: %v", err)
	}
	stored.Name = "Two\nother lines"
	if _, err := r.UpsertOffsiteTarget(stored); err == nil || !strings.Contains(err.Error(), ErrLineBreakText) {
		t.Fatalf("a changed name with a line break: %v", err)
	}
	if _, err := r.CreateOffsiteTarget(OffsiteTarget{Domain: "files", Name: "Two\nlines", Repo: "rest:https://box:8000/other", Enabled: true}); err == nil {
		t.Fatal("a new target took the stored two-line name")
	}
}

func TestTheFirstStartEndsEverySessionIssuedBefore(t *testing.T) {
	db := OpenMem(t)
	migrateThrough(t, db, legacyMembersMigration)
	if _, err := db.Exec(`UPDATE settings SET session_epoch = '', auth_password_hash = 'hash'`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := New(db)
	s, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.SessionEpoch == "" {
		t.Fatal("the session epoch did not move")
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	again, err := r.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if again.SessionEpoch != s.SessionEpoch {
		t.Fatal("a later start ended the sessions again")
	}
}
