package store

import (
	"database/sql"
	"maps"
	"testing"
)

// followedRows reads the settings a destination can hand down from every
// off-site target, as text, by id.
func followedRows(t *testing.T, db *sql.DB) map[string]string {
	t.Helper()
	rows, err := db.Query(`SELECT id, retention_keep_last || '/' || retention_keep_daily || '/' || retention_keep_weekly
		|| '/' || retention_keep_monthly || '/' || retention_keep_yearly || ' ' || compression
		|| ' ' || limit_upload || '/' || limit_download || ' ' || enabled
		FROM offsite_targets WHERE role = 'offsite'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close() //nolint:errcheck // test cleanup
	out := map[string]string{}
	for rows.Next() {
		var id, row string
		if err := rows.Scan(&id, &row); err != nil {
			t.Fatal(err)
		}
		out[id] = row
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestTheUpgradeLeavesEveryDerivedTargetWithItsSettings(t *testing.T) {
	db := OpenMem(t)
	bootAs(t, db, locationMigration-1)
	if _, err := db.Exec(`
		INSERT INTO offsite_targets (id, domain, name, repo, role, enabled, created_at, sort_order) VALUES
		  ('d1', '', 'Box', 'rclone:box:bv', 'destination', 1, 1, 0);
		INSERT INTO offsite_targets (id, domain, name, repo, role, enabled, created_at, sort_order, destination_id,
		  retention_keep_last, retention_keep_daily, retention_keep_weekly, retention_keep_monthly, retention_keep_yearly,
		  compression, limit_upload, limit_download) VALUES
		  ('plain',   'flash',      'Box', 'rclone:box:bv/flash',      'offsite', 1, 2, 1, 'd1', 0, 0, 0, 0,  0, '',     0,   0),
		  ('auto',    'config',     'Box', 'rclone:box:bv/selfbackup', 'offsite', 1, 2, 1, 'd1', 0, 0, 0, 0,  0, 'auto', 0,   0),
		  ('keeps',   'containers', 'Box', 'rclone:box:bv/containers', 'offsite', 1, 2, 1, 'd1', 3, 7, 4, 12, 2, '',     0,   0),
		  ('squeezed','vms',        'Box', 'rclone:box:bv/vms',        'offsite', 1, 2, 1, 'd1', 0, 0, 0, 0,  0, 'max',  512, 0),
		  ('off',     'files',      'Box', 'rclone:box:bv/files',      'offsite', 0, 2, 1, 'd1', 0, 0, 0, 0,  0, '',     0,   64),
		  ('primary', 'zfs',        'Box', 'rclone:box:bv/zfs',        'offsite', 1, 2, 0, 'd1', 0, 5, 0, 0,  0, '',     0,   0),
		  ('loose',   'vms',        'Own', 's3:bucket/vms',            'offsite', 1, 2, 2, '',   9, 9, 9, 9,  9, 'off',  1,   1),
		  ('orphan',  'flash',      'Old', 'rclone:gone:bv/flash',     'offsite', 1, 2, 2, 'no-such-destination', 1, 0, 0, 0, 0, '', 0, 0);`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	before := followedRows(t, db)

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	r := New(db)
	if after := followedRows(t, db); !maps.Equal(before, after) {
		t.Fatalf("the upgrade changed a target's settings:\nbefore %v\nafter  %v", before, after)
	}
	for id, want := range map[string]OwnSettings{
		"plain": 0, "auto": 0, "keeps": OwnRetention, "squeezed": OwnCompression | OwnLimits,
		"off": OwnLimits | OwnEnabled, "primary": OwnRetention, "loose": 0, "orphan": 0,
	} {
		got, ok, err := r.GetOffsiteTarget(id)
		if err != nil || !ok {
			t.Fatalf("%s: found=%v err=%v", id, ok, err)
		}
		if got.Own != want {
			t.Errorf("%s keeps settings %d for itself, want %d", id, got.Own, want)
		}
	}

	d, ok, err := r.GetDestination("d1")
	if err != nil || !ok || !d.OffPremises || !d.Enabled {
		t.Fatalf("destination = %+v, found=%v err=%v, want it enabled and off the premises", d, ok, err)
	}
	if _, err := r.SaveDestination(d); err != nil {
		t.Fatal(err)
	}
	if after := followedRows(t, db); !maps.Equal(before, after) {
		t.Fatalf("saving the destination as it is changed a target's settings:\nbefore %v\nafter  %v", before, after)
	}

	d.RetentionKeepDaily, d.Compression, d.LimitUpload, d.Enabled = 30, "off", 99, false
	if _, err := r.SaveDestination(d); err != nil {
		t.Fatal(err)
	}
	after := followedRows(t, db)
	for id, want := range map[string]string{
		"plain":    "0/30/0/0/0 off 99/0 0",
		"auto":     "0/30/0/0/0 off 99/0 0",
		"keeps":    "3/7/4/12/2 off 99/0 0",
		"squeezed": "0/30/0/0/0 max 512/0 0",
		"off":      "0/30/0/0/0 off 0/64 0",
		"primary":  before["primary"],
		"loose":    before["loose"],
		"orphan":   before["orphan"],
	} {
		if after[id] != want {
			t.Errorf("%s after the destination changed = %q, want %q", id, after[id], want)
		}
	}
}
