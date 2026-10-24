package store

import (
	"database/sql"
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
	assertReplicaSchema(t, db)
}

func TestAFreshDatabaseNumbersTheReplicaBlockWithoutGaps(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	applied := appliedVersions(t, db)
	want := []string{"zfs_replica_servers", "zfs_datasets_replica", "zfs_replica_state", "zfs_replica_runs", "zfs_receive_slots"}
	for i, name := range want {
		if got := applied[zfsReplicaMigration+i]; got != name {
			t.Errorf("v%d = %q, want %q", zfsReplicaMigration+i, got, name)
		}
	}
	for v := zfsReplicaMigration + len(want); v < locationMigration; v++ {
		if name, ok := applied[v]; ok {
			t.Errorf("v%d = %q, after the last replica migration", v, name)
		}
	}
	assertReplicaSchema(t, db)
}

// assertReplicaSchema checks every table and column the replica reads.
func assertReplicaSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, table := range []string{"zfs_replica_servers", "zfs_replica_state", "zfs_replica_runs", "zfs_receive_slots"} {
		if !hasTable(t, db, table) {
			t.Fatalf("%s missing", table)
		}
	}
	for _, col := range []struct{ table, column string }{
		{"zfs_datasets", "replica_target_kind"}, {"zfs_datasets", "replica_peer_pin"}, {"zfs_datasets", "replica_folder"},
		{"zfs_receive_slots", "bytes"}, {"zfs_receive_slots", "last_snapshot"},
	} {
		if !hasColumn(t, db, col.table, col.column) {
			t.Errorf("%s.%s missing", col.table, col.column)
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

// A server and the members stored before the mount setting stay unmounted.
func TestReplicaMountMigrationLeavesExistingServersAndMembersUnmounted(t *testing.T) {
	db := OpenMem(t)
	bootAs(t, db, replicaMountMigration-1)
	if _, err := db.Exec(`INSERT INTO zfs_replica_servers (id, name, host, pool, root, created_at) VALUES ('s1', 'attic', 'attic.lan', 'tank', 'tank/r', 1700000000)`); err != nil {
		t.Fatalf("seed server: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO zfs_replica_state (item_id, dataset, target_path) VALUES ('z1', 'cache/appdata', 'tank/r/bottich/cache/appdata')`); err != nil {
		t.Fatalf("seed state: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	repo := New(db)
	srv, ok, err := repo.GetZFSReplicaServer("s1")
	if err != nil || !ok || srv.MountReadOnly {
		t.Fatalf("server after the upgrade = %+v, %v, %v; want it kept and not mounting", srv, ok, err)
	}
	st, ok, err := repo.GetZFSReplicaState("z1", "cache/appdata")
	if err != nil || !ok || st.Mount != "" {
		t.Fatalf("state after the upgrade = %+v, %v, %v; want it kept without a mount note", st, ok, err)
	}
	if applied := appliedVersions(t, db); applied[replicaMountMigration] != "zfs_replica_mount" {
		t.Fatalf("migration %d = %q, want zfs_replica_mount", replicaMountMigration, applied[replicaMountMigration])
	}
}

func TestReplicaMountMigrationIsSatisfiedWhenAlreadyApplied(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE name = 'zfs_replica_mount'`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate over a database that already has the mount columns: %v", err)
	}
}

func TestTheMountSettingAndNoteSurviveASave(t *testing.T) {
	repo := New(OpenMem(t))
	if err := Migrate(repo.db); err != nil {
		t.Fatal(err)
	}
	srv, err := repo.CreateZFSReplicaServer(ZFSReplicaServer{Name: "attic", Host: "attic.lan", Pool: "tank", Root: "tank/r", Enabled: true, MountReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if got, _, _ := repo.GetZFSReplicaServer(srv.ID); !got.MountReadOnly {
		t.Fatal("a server created with the setting lost it")
	}
	srv.MountReadOnly = false
	if err := repo.UpdateZFSReplicaServer(srv); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := repo.GetZFSReplicaServer(srv.ID); got.MountReadOnly {
		t.Fatal("the edit did not switch the setting off")
	}
	if err := repo.PutZFSReplicaState(ZFSReplicaState{ItemID: "z1", Dataset: "cache/appdata", Mount: "locked"}); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := repo.GetZFSReplicaState("z1", "cache/appdata"); got.Mount != "locked" {
		t.Fatalf("mount note = %q, want locked", got.Mount)
	}
}

// An item stored before the setting keeps only the newest replica snapshot on
// this server.
func TestSourceKeepMigrationLeavesExistingItemsKeepingOnlyTheNewest(t *testing.T) {
	db := OpenMem(t)
	bootAs(t, db, replicaSourceKeepMigration-1)
	if _, err := db.Exec(`INSERT INTO zfs_datasets (id, dataset, created_at) VALUES ('z1', 'cache/appdata', 1700000000)`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	repo := New(db)
	d, err := repo.GetZFSDataset("z1")
	if err != nil || d.Replica.KeepOnSource != 0 {
		t.Fatalf("item after the upgrade = %+v, %v; want 0 kept on the source", d.Replica, err)
	}
	if err := repo.SetZFSReplicaKeepOnSource("z1", 5); err != nil {
		t.Fatal(err)
	}
	if d, _ := repo.GetZFSDataset("z1"); d.Replica.KeepOnSource != 5 {
		t.Fatalf("kept on the source after the save = %d, want 5", d.Replica.KeepOnSource)
	}
	if applied := appliedVersions(t, db); applied[replicaSourceKeepMigration] != "zfs_datasets_replica_keep_source" {
		t.Fatalf("migration %d = %q", replicaSourceKeepMigration, applied[replicaSourceKeepMigration])
	}
}
