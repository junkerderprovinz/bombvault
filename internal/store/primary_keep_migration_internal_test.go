package store

import (
	"maps"
	"testing"
)

func TestTheUpgradeLetsAPrimaryWithItsLocationsKeepPolicyFollowIt(t *testing.T) {
	db := OpenMem(t)
	bootAs(t, db, primaryKeepMigration-1)
	if _, err := db.Exec(`
		INSERT INTO offsite_targets (id, domain, name, repo, role, enabled, created_at, sort_order,
		  retention_keep_daily, retention_keep_weekly) VALUES
		  ('d1', '', 'NAS', 'rest:http://nas/bv', 'destination', 1, 1, 0, 30, 4);
		INSERT INTO offsite_targets (id, domain, name, repo, role, enabled, created_at, sort_order, destination_id,
		  retention_keep_last, retention_keep_daily, retention_keep_weekly, retention_keep_monthly, retention_keep_yearly,
		  compression, own_settings) VALUES
		  ('same',   'containers', 'NAS', 'rest:http://nas/bv/containers', 'offsite', 1, 2, 0, 'd1', 0, 30, 4, 0, 0, 'max', 3),
		  ('differs','vms',        'NAS', 'rest:http://nas/bv/vms',        'offsite', 1, 2, 0, 'd1', 0, 30, 8, 0, 0, '',    0),
		  ('second', 'flash',      'NAS', 'rest:http://nas/bv/flash',      'offsite', 1, 2, 1, 'd1', 0, 30, 4, 0, 0, '',    1),
		  ('loose',  'files',      'Own', 's3:bucket/files',               'offsite', 1, 2, 0, '',   0, 7, 0, 0, 0, '',    0);`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	before := followedRows(t, db)

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if after := followedRows(t, db); !maps.Equal(before, after) {
		t.Fatalf("the upgrade changed a target's settings:\nbefore %v\nafter  %v", before, after)
	}
	r := New(db)
	for id, want := range map[string]OwnSettings{
		"same": OwnCompression, "differs": OwnRetention, "second": OwnRetention, "loose": 0,
	} {
		got, ok, err := r.GetOffsiteTarget(id)
		if err != nil || !ok {
			t.Fatalf("%s: found=%v err=%v", id, ok, err)
		}
		if got.Own != want {
			t.Errorf("%s keeps settings %d for itself, want %d", id, got.Own, want)
		}
	}
}
