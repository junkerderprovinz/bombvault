package store

import (
	"encoding/json"
	"reflect"
	"testing"
)

// An instance updated from the release before the replica keeps its ZFS items,
// which do not replicate yet.
func TestReplicaMigrationsKeepItems(t *testing.T) {
	db := OpenMem(t)
	bootAs(t, db, zfsReplicaMigration-1)
	if _, err := db.Exec(`INSERT INTO zfs_datasets (id, dataset, repo, created_at) VALUES ('z1', 'cache/appdata', 'repo-1', 1700000000)`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	d, err := New(db).GetZFSDataset("z1")
	if err != nil {
		t.Fatal(err)
	}
	if d.Repo != "repo-1" || !reflect.DeepEqual(d.Replica, DefaultZFSReplica()) {
		t.Fatalf("item after the upgrade: repo %q, replica %+v; want its repo and the default replica", d.Repo, d.Replica)
	}

	want, err := json.Marshal(DefaultZFSReplicaKeep)
	if err != nil {
		t.Fatal(err)
	}
	for _, col := range []struct{ table, column string }{
		{"zfs_datasets", "replica_keep"}, {"zfs_receive_slots", "proposed_keep"}, {"zfs_receive_slots", "keep"},
	} {
		var def string
		if err := db.QueryRow(`SELECT dflt_value FROM pragma_table_info(?) WHERE name = ?`, col.table, col.column).Scan(&def); err != nil {
			t.Fatal(err)
		}
		if def != "'"+string(want)+"'" {
			t.Fatalf("%s.%s defaults to %s, want DefaultZFSReplicaKeep %s", col.table, col.column, def, want)
		}
	}
	for _, table := range []string{"zfs_replica_servers", "zfs_replica_state", "zfs_replica_runs", "zfs_receive_slots"} {
		if !hasTable(t, db, table) {
			t.Fatalf("%s missing after the upgrade", table)
		}
	}
}

func TestReplicaColumnMigrationIsSatisfiedWhenAlreadyApplied(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO zfs_datasets (id, dataset, replica_cadence, replica_peer_slot)
		VALUES ('z1', 'cache/appdata', 'daily 05:00', 'slot-1')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE name = 'zfs_datasets_replica'`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate over a database that already has the replica columns: %v", err)
	}
	if applied := appliedVersions(t, db); applied[zfsReplicaMigration+1] != "zfs_datasets_replica" {
		t.Fatalf("the guarded migration was not recorded again: %v", applied)
	}
	d, err := New(db).GetZFSDataset("z1")
	if err != nil || d.Replica.Cadence != "daily 05:00" || d.Replica.Peer.Slot != "slot-1" {
		t.Fatalf("item after the second run = %+v, %v", d.Replica, err)
	}
}
