package store_test

import (
	"database/sql"
	"errors"
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func aReplicaServer(t *testing.T, r *store.Repo, name string) store.ZFSReplicaServer {
	t.Helper()
	s, err := r.CreateZFSReplicaServer(store.ZFSReplicaServer{
		Name: name, Host: name + ".lan", User: "root", Port: 22, Pool: "tank", Root: "tank/bombvault-replica", Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateZFSReplicaServer(%s): %v", name, err)
	}
	return s
}

func aReplicaState(t *testing.T, r *store.Repo, itemID, dataset string) {
	t.Helper()
	if err := r.PutZFSReplicaState(store.ZFSReplicaState{
		ItemID: itemID, Dataset: dataset, TargetPath: "tank/bombvault-replica/tower/" + dataset,
		SourceBase: "@bombvault-replica-20261009030000", SourceGUID: "1",
		TargetBase: "@bombvault-replica-20261009030000", TargetGUID: "1",
	}); err != nil {
		t.Fatalf("PutZFSReplicaState: %v", err)
	}
}

func replicaStates(t *testing.T, r *store.Repo, itemID string) int {
	t.Helper()
	states, err := r.ListZFSReplicaStates(itemID)
	if err != nil {
		t.Fatalf("ListZFSReplicaStates: %v", err)
	}
	return len(states)
}

func TestANewZFSItemDoesNotReplicateAndWouldKeepAboutAMonth(t *testing.T) {
	_, r := zfsStore(t)
	made := aZFSDataset(t, r, "cache/appdata")
	got, err := r.GetZFSDataset(made.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := store.ZFSReplica{TargetKind: store.ZFSReplicaTargetNone, AfterBackup: true,
		Keep: store.ZFSReplicaKeep{Preset: "own", Own: [5]int{0, 7, 3, 0, 0}}}
	if !reflect.DeepEqual(got.Replica, want) {
		t.Fatalf("stored replica = %+v, want %+v", got.Replica, want)
	}
	if !reflect.DeepEqual(made.Replica, want) {
		t.Fatalf("CreateZFSDataset returned replica %+v, want what it stored", made.Replica)
	}
	counts, ok := got.Replica.Keep.Counts()
	if !ok || counts != (store.RetentionKeep{KeepDaily: 7, KeepWeekly: 3}) {
		t.Fatalf("default keep counts = %+v, %v", counts, ok)
	}
}

func TestZFSReplicaKeepPresetsResolveToTheirCounts(t *testing.T) {
	for preset, want := range map[string]store.RetentionKeep{
		"short":    {KeepDaily: 7, KeepWeekly: 4, KeepMonthly: 3},
		"balanced": {KeepDaily: 7, KeepWeekly: 4, KeepMonthly: 6, KeepYearly: 1},
		"long":     {KeepDaily: 14, KeepWeekly: 8, KeepMonthly: 12, KeepYearly: 3},
	} {
		got, ok := store.ZFSReplicaKeep{Preset: preset, Own: [5]int{9, 9, 9, 9, 9}}.Counts()
		if !ok || got != want {
			t.Fatalf("%s = %+v, %v; want %+v, the own counts ignored", preset, got, ok, want)
		}
	}
	if _, ok := (store.ZFSReplicaKeep{Preset: "forever"}).Counts(); ok {
		t.Fatal("an unknown preset resolved to counts")
	}
}

func TestZFSReplicaSettersWriteOnlyTheirOwnColumn(t *testing.T) {
	_, r := zfsStore(t)
	d := aZFSDataset(t, r, "cache/appdata")
	if err := r.SetZFSDatasetRepo(d.ID, "repo-1"); err != nil {
		t.Fatal(err)
	}
	server := aReplicaServer(t, r, "attic")

	keep := store.ZFSReplicaKeep{Preset: "balanced", Own: [5]int{1, 2, 3, 4, 5}}
	for _, set := range []func() error{
		func() error { return r.SetZFSReplicaTarget(d.ID, store.ZFSReplicaTargetServer, server.ID) },
		func() error { return r.SetZFSReplicaAfterBackup(d.ID, false) },
		func() error { return r.SetZFSReplicaCadence(d.ID, "daily 05:00") },
		func() error { return r.SetZFSReplicaKeep(d.ID, keep) },
	} {
		if err := set(); err != nil {
			t.Fatal(err)
		}
	}
	got, err := r.GetZFSDataset(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := store.ZFSReplica{TargetKind: store.ZFSReplicaTargetServer, TargetID: server.ID, Cadence: "daily 05:00", Keep: keep}
	if !reflect.DeepEqual(got.Replica, want) {
		t.Fatalf("replica = %+v, want %+v", got.Replica, want)
	}
	if got.Repo != "repo-1" || !got.Enabled {
		t.Fatalf("the replica setters changed the backup: repo %q, enabled %v", got.Repo, got.Enabled)
	}

	d.Enabled = false
	if err := r.UpdateZFSDataset(d); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.GetZFSDataset(d.ID); !reflect.DeepEqual(got.Replica, want) {
		t.Fatalf("UpdateZFSDataset reset the replica to %+v", got.Replica)
	}

	if err := r.SetZFSReplicaCadence("missing", "daily 05:00"); err == nil {
		t.Fatal("a setter on a missing item reported success")
	}
	if err := r.SetZFSReplicaTarget("missing", store.ZFSReplicaTargetNone, ""); err == nil {
		t.Fatal("SetZFSReplicaTarget on a missing item reported success")
	}
}

func TestAnotherReplicaTargetStartsFromScratch(t *testing.T) {
	_, r := zfsStore(t)
	d := aZFSDataset(t, r, "cache/appdata")
	a, b := aReplicaServer(t, r, "attic"), aReplicaServer(t, r, "barn")
	if err := r.SetZFSReplicaTarget(d.ID, store.ZFSReplicaTargetServer, a.ID); err != nil {
		t.Fatal(err)
	}
	aReplicaState(t, r, d.ID, "cache/appdata")

	if err := r.SetZFSReplicaTarget(d.ID, store.ZFSReplicaTargetServer, a.ID); err != nil {
		t.Fatal(err)
	}
	if n := replicaStates(t, r, d.ID); n != 1 {
		t.Fatalf("setting the same target again left %d member states, want the one kept", n)
	}
	if err := r.SetZFSReplicaTarget(d.ID, store.ZFSReplicaTargetServer, b.ID); err != nil {
		t.Fatal(err)
	}
	if n := replicaStates(t, r, d.ID); n != 0 {
		t.Fatalf("a new target kept %d member states of the old one", n)
	}
}

func TestZFSReplicaServerRoundTripKeepsItsKeyDirectory(t *testing.T) {
	_, r := zfsStore(t)
	made, err := r.CreateZFSReplicaServer(store.ZFSReplicaServer{
		Name: "barn", Host: "10.0.0.5", User: "replica", Port: 2222, Pool: "tank", Root: "tank/bombvault-replica",
		Enabled: true, KeyDir: "/config/ssh-replica/x",
	})
	if err != nil {
		t.Fatal(err)
	}
	if made.ID == "" || made.CreatedAt == 0 {
		t.Fatalf("create returned %+v, want an id and a creation time", made)
	}
	aReplicaServer(t, r, "attic")

	got, ok, err := r.GetZFSReplicaServer(made.ID)
	if err != nil || !ok || got != made {
		t.Fatalf("GetZFSReplicaServer = %+v, %v, %v; want %+v", got, ok, err, made)
	}

	edit := made
	edit.Name, edit.Host, edit.User, edit.Port, edit.Root, edit.Enabled = "barn2", "10.0.0.6", "root", 22, "tank/replicas", false
	edit.KeyDir = "/elsewhere"
	if err := r.UpdateZFSReplicaServer(edit); err != nil {
		t.Fatal(err)
	}
	got, _, _ = r.GetZFSReplicaServer(made.ID)
	edit.KeyDir = made.KeyDir
	if got != edit {
		t.Fatalf("after the update = %+v, want %+v with the key directory untouched", got, edit)
	}
	if err := r.SetZFSReplicaServerKeyDir(made.ID, "/config/ssh-replica/y"); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := r.GetZFSReplicaServer(made.ID); got.KeyDir != "/config/ssh-replica/y" {
		t.Fatalf("key dir = %q", got.KeyDir)
	}

	list, err := r.ListZFSReplicaServers()
	if err != nil || len(list) != 2 || list[0].Name != "attic" || list[1].Name != "barn2" {
		t.Fatalf("ListZFSReplicaServers = %+v, %v; want attic then barn2", list, err)
	}
	if err := r.UpdateZFSReplicaServer(store.ZFSReplicaServer{ID: "missing"}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("updating a missing server = %v, want sql.ErrNoRows", err)
	}
	if _, ok, err := r.GetZFSReplicaServer("missing"); ok || err != nil {
		t.Fatalf("GetZFSReplicaServer(missing) = %v, %v", ok, err)
	}
}

func TestDeletingAReplicaServerDetachesTheItemsOnIt(t *testing.T) {
	_, r := zfsStore(t)
	a, b := aReplicaServer(t, r, "attic"), aReplicaServer(t, r, "barn")
	onA1, onA2, onB := aZFSDataset(t, r, "cache/a1"), aZFSDataset(t, r, "cache/a2"), aZFSDataset(t, r, "cache/b")
	for _, pin := range []struct{ item, server string }{{onA1.ID, a.ID}, {onA2.ID, a.ID}, {onB.ID, b.ID}} {
		if err := r.SetZFSReplicaTarget(pin.item, store.ZFSReplicaTargetServer, pin.server); err != nil {
			t.Fatal(err)
		}
		aReplicaState(t, r, pin.item, "cache/x")
	}

	users, err := r.ZFSReplicaServerUsers()
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string][]string{a.ID: {onA1.ID, onA2.ID}, b.ID: {onB.ID}}; !reflect.DeepEqual(users, want) {
		t.Fatalf("users = %v, want %v", users, want)
	}

	if err := r.DeleteZFSReplicaServer(a.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := r.GetZFSReplicaServer(a.ID); ok {
		t.Fatal("the deleted server is still there")
	}
	for _, id := range []string{onA1.ID, onA2.ID} {
		got, _ := r.GetZFSDataset(id)
		if got.Replica.TargetKind != store.ZFSReplicaTargetNone || got.Replica.TargetID != "" {
			t.Fatalf("an item on the deleted server still points at %s %q", got.Replica.TargetKind, got.Replica.TargetID)
		}
		if n := replicaStates(t, r, id); n != 0 {
			t.Fatalf("an item on the deleted server kept %d member states", n)
		}
	}
	if got, _ := r.GetZFSDataset(onB.ID); got.Replica.TargetID != b.ID || replicaStates(t, r, onB.ID) != 1 {
		t.Fatalf("the item on the other server lost its target or state: %+v", got.Replica)
	}
	if err := r.DeleteZFSReplicaServer("missing"); err != nil {
		t.Fatalf("deleting a missing server: %v", err)
	}
}

func TestZFSReplicaStateRoundTripKeepsGUIDsAboveInt64(t *testing.T) {
	_, r := zfsStore(t)
	want := store.ZFSReplicaState{
		ItemID: "item", Dataset: "cache/vm-disk", TargetPath: "tank/bombvault-replica/tower/cache/vm-disk", Volume: true,
		SourceBase: "#bombvault-replica-20261009030000", SourceGUID: "18446744073709551615",
		TargetBase: "@bombvault-replica-20261009030000", TargetGUID: "18446744073709551615",
		CreatedParent: true, UpdatedAt: 1760000000,
	}
	if err := r.PutZFSReplicaState(want); err != nil {
		t.Fatal(err)
	}
	got, ok, err := r.GetZFSReplicaState("item", "cache/vm-disk")
	if err != nil || !ok || got != want {
		t.Fatalf("GetZFSReplicaState = %+v, %v, %v; want %+v", got, ok, err, want)
	}

	next := want
	next.SourceBase, next.SourceGUID, next.CreatedParent, next.UpdatedAt = "@bombvault-replica-20261010030000", "42", false, 0
	if err := r.PutZFSReplicaState(next); err != nil {
		t.Fatal(err)
	}
	got, _, _ = r.GetZFSReplicaState("item", "cache/vm-disk")
	if got.SourceGUID != "42" || got.CreatedParent || got.UpdatedAt == 0 {
		t.Fatalf("a second put = %+v, want it replaced and stamped", got)
	}
	aReplicaState(t, r, "item", "cache/appdata")
	if n := replicaStates(t, r, "item"); n != 2 {
		t.Fatalf("%d member states, want 2", n)
	}

	if err := r.DeleteZFSReplicaState("item", "cache/vm-disk"); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := r.GetZFSReplicaState("item", "cache/vm-disk"); ok || err != nil {
		t.Fatalf("a deleted state reads as %v, %v", ok, err)
	}
}

func TestLatestZFSReplicaRunIsTheNewestReplicaRunEvenWhileItRuns(t *testing.T) {
	_, r := zfsStore(t)
	d := aZFSDataset(t, r, "cache/appdata")
	if _, ok, err := r.LatestZFSReplicaRun(d.ID); ok || err != nil {
		t.Fatalf("an item that never replicated has a latest run: %v, %v", ok, err)
	}

	first, err := r.StartRun(d.ID, store.ZFSReplicaRunKind)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.FinishRun(first, "success", "", 0, ""); err != nil {
		t.Fatal(err)
	}
	second, err := r.StartRun(d.ID, store.ZFSReplicaRunKind)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.StartRun(d.ID, "backup"); err != nil {
		t.Fatal(err)
	}
	members := []store.ZFSReplicaRunMember{
		{RunID: second, ItemID: d.ID, Dataset: "cache/appdata/b", Snapshot: "bombvault-replica-20261009030000",
			Bytes: 4096, Seconds: 3, Resumed: true, FinishedAt: 1760000003},
		{RunID: second, ItemID: d.ID, Dataset: "cache/appdata/a", Base: "#bombvault-replica-20261008030000",
			Snapshot: "bombvault-replica-20261009030000", Code: "no-space"},
	}
	for _, m := range members {
		if err := r.AddZFSReplicaRunMember(m); err != nil {
			t.Fatal(err)
		}
	}
	members[1].Bytes = 10
	if err := r.AddZFSReplicaRunMember(members[1]); err != nil {
		t.Fatal(err)
	}

	got, ok, err := r.LatestZFSReplicaRun(d.ID)
	if err != nil || !ok {
		t.Fatalf("LatestZFSReplicaRun = %v, %v", ok, err)
	}
	if got.ID != second || got.Status != "running" || got.Kind != store.ZFSReplicaRunKind {
		t.Fatalf("latest run = %s %s %s, want the running replica run %s", got.ID, got.Kind, got.Status, second)
	}
	want := []store.ZFSReplicaRunMember{members[1], members[0]}
	if !reflect.DeepEqual(got.Members, want) {
		t.Fatalf("members = %+v, want %+v ordered by dataset with the rewrite kept", got.Members, want)
	}
}

func TestDeleteZFSDatasetRemovesItsReplicaRows(t *testing.T) {
	db, r := zfsStore(t)
	doomed, kept := aZFSDataset(t, r, "cache/appdata"), aZFSDataset(t, r, "cache/system")
	for _, d := range []store.ZFSDataset{doomed, kept} {
		aReplicaState(t, r, d.ID, d.Dataset)
		if err := r.AddZFSReplicaRunMember(store.ZFSReplicaRunMember{RunID: "run-" + d.ID, ItemID: d.ID, Dataset: d.Dataset}); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.DeleteZFSDataset(doomed.ID); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"zfs_replica_state", "zfs_replica_runs"} {
		var gone, left int
		if err := db.QueryRow(`SELECT count(*) FROM `+table+` WHERE item_id = ?`, doomed.ID).Scan(&gone); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(`SELECT count(*) FROM `+table+` WHERE item_id = ?`, kept.ID).Scan(&left); err != nil {
			t.Fatal(err)
		}
		if gone != 0 || left != 1 {
			t.Fatalf("%s: %d rows of the deleted item left, %d of the other; want 0 and 1", table, gone, left)
		}
	}
}

func TestImportZFSReplicaReplacesServersAndAppliesItemsByDataset(t *testing.T) {
	_, r := zfsStore(t)
	kept, err := r.CreateZFSReplicaServer(store.ZFSReplicaServer{Name: "attic", Host: "old", Pool: "tank", Root: "tank/r", KeyDir: "/config/ssh-replica/k"})
	if err != nil {
		t.Fatal(err)
	}
	dropped := aReplicaServer(t, r, "barn")
	onDropped := aZFSDataset(t, r, "cache/system")
	if err := r.SetZFSReplicaTarget(onDropped.ID, store.ZFSReplicaTargetServer, dropped.ID); err != nil {
		t.Fatal(err)
	}
	item := aZFSDataset(t, r, "cache/appdata")

	keep := store.ZFSReplicaKeep{Preset: "long"}
	err = r.ImportZFSReplica(store.ZFSReplicaImport{
		Servers: []store.ZFSReplicaServer{
			{ID: kept.ID, Name: "attic", Host: "10.0.0.7", User: "root", Port: 22, Pool: "tank", Root: "tank/r", Enabled: true},
			{ID: "new-server", Name: "cellar", Host: "10.0.0.8", User: "root", Port: 22, Pool: "pool", Root: "pool/r", Enabled: true},
		},
		Items: map[string]store.ZFSReplica{
			"cache/appdata": {TargetKind: store.ZFSReplicaTargetServer, TargetID: "new-server", Cadence: "daily 01:00", Keep: keep},
			"cache/gone":    {TargetKind: store.ZFSReplicaTargetServer, TargetID: kept.ID},
		},
	})
	if err != nil {
		t.Fatalf("ImportZFSReplica: %v", err)
	}

	got, _, _ := r.GetZFSReplicaServer(kept.ID)
	if got.Host != "10.0.0.7" || !got.Enabled || got.KeyDir != "/config/ssh-replica/k" || got.CreatedAt != kept.CreatedAt {
		t.Fatalf("the kept server = %+v, want the file's fields over this instance's key dir and creation time", got)
	}
	if fresh, ok, _ := r.GetZFSReplicaServer("new-server"); !ok || fresh.KeyDir != "" || fresh.CreatedAt == 0 {
		t.Fatalf("the new server = %+v, %v", fresh, ok)
	}
	if _, ok, _ := r.GetZFSReplicaServer(dropped.ID); ok {
		t.Fatal("a server the file does not carry survived the import")
	}
	if d, _ := r.GetZFSDataset(onDropped.ID); d.Replica.TargetKind != store.ZFSReplicaTargetNone {
		t.Fatalf("the item on the dropped server still replicates to %s", d.Replica.TargetID)
	}
	d, _ := r.GetZFSDataset(item.ID)
	want := store.ZFSReplica{TargetKind: store.ZFSReplicaTargetServer, TargetID: "new-server", Cadence: "daily 01:00", Keep: keep}
	if !reflect.DeepEqual(d.Replica, want) {
		t.Fatalf("imported item = %+v, want %+v", d.Replica, want)
	}
	if _, err := r.GetZFSDatasetByName("cache/gone"); err == nil {
		t.Fatal("the import created an item for a dataset this instance does not have")
	}
}
