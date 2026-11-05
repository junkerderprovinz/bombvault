package store

import (
	"database/sql"
	"testing"
)

func TestAnUnreadRepositoryLosesItsFailedVerdictOnUpdate(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := New(db)
	failed := sql.NullBool{Valid: true}
	verdicts := []struct {
		errText string
		kept    bool
	}{
		{"the received repository does not exist yet: it appears with the other instance's first copy there, unless the location is mistyped", false},
		{"could not reach the received repository: dial tcp: connection refused", false},
		{"could not open the received repository: wrong restic password, or the location is not a BombVault or restic repository", false},
		{"the rest-server rejected these credentials (401). Two things cause almost every one of these.", false},
		{"Fatal: repository contains errors", true},
	}
	ids := make([]string, len(verdicts))
	for i, v := range verdicts {
		rr, err := r.CreateReceivedRepo(ReceivedRepo{Name: "r", Repo: "user/r", LastCheckAt: 5, LastCheckOK: failed, LastCheckError: v.errText})
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = rr.ID
	}
	for _, m := range migrations {
		if m.version == receivedRepoUnreadVerdictMigration {
			if _, err := db.Exec(m.sql); err != nil {
				t.Fatal(err)
			}
		}
	}
	for i, v := range verdicts {
		got, _, err := r.GetReceivedRepo(ids[i])
		if err != nil {
			t.Fatal(err)
		}
		if got.LastCheckOK.Valid != v.kept {
			t.Errorf("%q: verdict %+v, kept should be %v", v.errText, got.LastCheckOK, v.kept)
		}
	}
}
