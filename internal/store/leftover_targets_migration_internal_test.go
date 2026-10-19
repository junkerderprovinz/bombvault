package store

import (
	"database/sql"
	"maps"
	"testing"
)

func offsiteRows(t *testing.T, db *sql.DB) map[string]string {
	t.Helper()
	rows, err := db.Query(`SELECT id, destination_id FROM offsite_targets WHERE role IN ('offsite', 'repo')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close() //nolint:errcheck // test cleanup
	out := map[string]string{}
	for rows.Next() {
		var id, dest string
		if err := rows.Scan(&id, &dest); err != nil {
			t.Fatal(err)
		}
		out[id] = dest
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestTheUpgradeDropsOnlyLeftoverTargetsThatHoldNothing(t *testing.T) {
	db := OpenMem(t)
	bootAs(t, db, leftoverTargetsMigration-1)
	if _, err := db.Exec(`
		UPDATE settings SET vms_offsite = '', files_offsite = '', containers_offsite = 's3:box/containers',
		  flash_offsite = 'rest:https://gone:8000/flash', flash_offsite_immutable = 1 WHERE id = 1;
		INSERT INTO offsite_targets (id, domain, name, repo, role, enabled, created_at, sort_order, destination_id, companion_of) VALUES
		  ('box',      '',           'Box',  's3:box',             'destination', 1, 1, 0, '', ''),
		  ('empty',    'vms',        'Box',  's3:box/vms',         'offsite', 0, 2, 0, '', ''),
		  ('ran',      'files',      'Box',  's3:box/files',       'offsite', 0, 2, 0, '', ''),
		  ('filled',   'containers', 'Box',  's3:box/containers',  'offsite', 0, 2, 0, '', ''),
		  ('attached', 'zfs',        'Box',  's3:box/zfs',         'offsite', 0, 2, 0, 'box', ''),
		  ('dangling', 'config',     'Old',  's3:old/selfbackup',  'offsite', 1, 2, 1, 'gone', ''),
		  ('unused',   '',           'Box direct', 's3:box/vms-direct', 'repo', 1, 2, 0, '', 'empty');
		INSERT INTO offsite_runs (domain, started_at, ok, offsite_target_id) VALUES ('files', 5, 1, 'ran');`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	want := map[string]string{"ran": "", "filled": "", "attached": "box", "dangling": ""}
	if got := offsiteRows(t, db); !maps.Equal(got, want) {
		t.Fatalf("rows after the upgrade = %v, want %v", got, want)
	}
	s, err := New(db).GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.FlashOffsite != "" || s.FlashOffsiteImmutable {
		t.Fatalf("flash off-site field = %q, append-only %v, want it cleared with no row behind it", s.FlashOffsite, s.FlashOffsiteImmutable)
	}
	if s.ContainersOffsite != "s3:box/containers" {
		t.Fatalf("containers off-site field = %q, want the one its row holds", s.ContainersOffsite)
	}
}

func TestTheUpgradeKeepsALeftoverWhoseDirectRepositoryIsInUse(t *testing.T) {
	db := OpenMem(t)
	bootAs(t, db, leftoverTargetsMigration-1)
	if _, err := db.Exec(`
		UPDATE settings SET vms_offsite = '' WHERE id = 1;
		INSERT INTO offsite_targets (id, domain, name, repo, role, enabled, created_at, sort_order, companion_of) VALUES
		  ('left',   'vms', 'Box',        's3:box/vms',        'offsite', 0, 2, 0, ''),
		  ('direct', '',    'Box direct', 's3:box/vms-direct', 'repo',    1, 2, 0, 'left');
		INSERT INTO vms (id, name, created_at, repo) VALUES ('v1', 'win', 1, 'direct');`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	if got, want := offsiteRows(t, db), map[string]string{"left": "", "direct": ""}; !maps.Equal(got, want) {
		t.Fatalf("rows after the upgrade = %v, want %v", got, want)
	}
}
