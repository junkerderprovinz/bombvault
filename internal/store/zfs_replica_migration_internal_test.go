package store

import (
	"encoding/json"
	"reflect"
	"testing"
)

// An instance updated from the release before the replica keeps its ZFS items
// and pull sources: the items do not replicate yet, and the pull sources stay
// restic ones.
func TestReplicaMigrationsKeepItemsAndPullSources(t *testing.T) {
	db := OpenMem(t)
	bootAs(t, db, zfsReplicaMigration-1)
	for _, q := range []string{
		`INSERT INTO zfs_datasets (id, dataset, repo, created_at) VALUES ('z1', 'cache/appdata', 'repo-1', 1700000000)`,
		`INSERT INTO pull_sources (id, name, repo, domain, created_at) VALUES ('ps1', 'attic', 'rest:http://192.168.1.9:8000/c', 'containers', 1700000000)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	r := New(db)

	d, err := r.GetZFSDataset("z1")
	if err != nil {
		t.Fatal(err)
	}
	if d.Repo != "repo-1" || !reflect.DeepEqual(d.Replica, DefaultZFSReplica()) {
		t.Fatalf("item after the upgrade: repo %q, replica %+v; want its repo and the default replica", d.Repo, d.Replica)
	}
	ps, ok, err := r.GetPullSource("ps1")
	if err != nil || !ok {
		t.Fatalf("pull source lost: %v", err)
	}
	if ps.Kind != PullSourceRestic || ps.Repo != "rest:http://192.168.1.9:8000/c" || len(ps.ZFSDatasets) != 0 ||
		ps.ZFSKeep != DefaultZFSReplicaKeep || ps.GrantState != "" {
		t.Fatalf("pull source after the upgrade = %+v", ps)
	}

	want, err := json.Marshal(DefaultZFSReplicaKeep)
	if err != nil {
		t.Fatal(err)
	}
	for _, col := range []struct{ table, column string }{{"zfs_datasets", "replica_keep"}, {"pull_sources", "zfs_keep"}} {
		var def string
		if err := db.QueryRow(`SELECT dflt_value FROM pragma_table_info(?) WHERE name = ?`, col.table, col.column).Scan(&def); err != nil {
			t.Fatal(err)
		}
		if def != "'"+string(want)+"'" {
			t.Fatalf("%s.%s defaults to %s, want DefaultZFSReplicaKeep %s", col.table, col.column, def, want)
		}
	}
	for _, table := range []string{"zfs_replica_servers", "zfs_replica_state", "zfs_replica_runs", "zfs_replica_grants"} {
		if !hasTable(t, db, table) {
			t.Fatalf("%s missing after the upgrade", table)
		}
	}
}

func TestReplicaColumnMigrationsAreSatisfiedWhenAlreadyApplied(t *testing.T) {
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO zfs_datasets (id, dataset, replica_cadence) VALUES ('z1', 'cache/appdata', 'daily 05:00')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE name IN ('zfs_datasets_replica', 'pull_sources_zfs')`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate over a database that already has the replica columns: %v", err)
	}
	applied := appliedVersions(t, db)
	if applied[zfsReplicaMigration+1] != "zfs_datasets_replica" || applied[zfsReplicaMigration+5] != "pull_sources_zfs" {
		t.Fatalf("the guarded migrations were not recorded again: %v", applied)
	}
	d, err := New(db).GetZFSDataset("z1")
	if err != nil || d.Replica.Cadence != "daily 05:00" {
		t.Fatalf("item after the second run = %+v, %v", d.Replica, err)
	}
}
