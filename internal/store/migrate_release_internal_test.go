package store

import (
	"strings"
	"testing"
)

func TestTheListOfMembersAllowedToCallUnsignedGoes(t *testing.T) {
	db := OpenMem(t)
	migrateThrough(t, db, legacyMembersMigration+1)
	if _, err := db.Exec(`INSERT INTO legacy_members (member_id) VALUES ('old')`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("a second start: %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'legacy_members'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("legacy_members is still there: %d, %v", n, err)
	}
}

func TestATargetWhoseNameSpansLinesCanStillBeSaved(t *testing.T) {
	db := OpenMem(t)
	migrateThrough(t, db, oneLineMigration-1)
	if _, err := db.Exec(`INSERT INTO offsite_targets (id, domain, name, repo, role, enabled, created_at, sort_order)
		VALUES ('t1', 'files', 'Two' || char(10) || 'lines', 'rest:https://box:8000/files', 'offsite', 1, 1, 1)`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("a second start: %v", err)
	}
	r := New(db)
	stored, ok, err := r.GetOffsiteTarget("t1")
	if err != nil || !ok {
		t.Fatalf("the stored target: ok=%v err=%v", ok, err)
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
