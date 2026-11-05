package store

import (
	"database/sql"
	"testing"
)

// establishedBeforeTheUpgrade is a database one step before
// staleEstablishedMigration. Added at 200, each target is named after what
// happened to it.
func establishedBeforeTheUpgrade(t *testing.T) *sql.DB {
	t.Helper()
	db := OpenMem(t)
	bootAs(t, db, staleEstablishedMigration-1)
	if _, err := db.Exec(`
		INSERT INTO offsite_targets (id, domain, name, repo, role, enabled, created_at) VALUES
		  ('readded',   'files', 'A', 's3:box/readded',   'offsite', 1, 200),
		  ('refound',   'files', 'B', 's3:box/refound',   'offsite', 1, 200),
		  ('stayed',    'files', 'C', 's3:box/stayed',    'offsite', 1, 200),
		  ('folder',    'files', 'D', '/mnt/old/folder',  'offsite', 1, 200),
		  ('named',     '',      'E', 's3:box/named',     'repo',    1, 200);
		INSERT INTO established_repos (repo, created_at, gone_at) VALUES
		  ('s3:box/readded', 100, 300),
		  ('s3:box/refound', 100, 0),
		  ('s3:box/stayed',  250, 300),
		  ('/mnt/old/folder', 100, 0),
		  ('s3:box/named',   100, 300),
		  ('s3:box/removed', 100, 0);
		INSERT INTO offsite_runs (domain, started_at, finished_at, ok, error, offsite_target_id) VALUES
		  ('files', 300, 301, 0, 'gone', 'readded'),
		  ('files', 210, 211, 1, '',     'refound');`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return db
}

func TestTheUpgradeForgetsARepositoryALocationNeverHad(t *testing.T) {
	db := establishedBeforeTheUpgrade(t)

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	r := New(db)
	for repo, want := range map[string]bool{
		"s3:box/readded":  false,
		"s3:box/refound":  true,
		"s3:box/stayed":   true,
		"/mnt/old/folder": true,
		"s3:box/named":    true,
		"s3:box/removed":  true,
	} {
		got, err := r.IsRepoEstablished(repo)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("%s established = %v, want %v", repo, got, want)
		}
	}
}
