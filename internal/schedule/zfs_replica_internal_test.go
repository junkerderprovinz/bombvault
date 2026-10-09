package schedule

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func replicaItem(id string, afterBackup bool, cadence string) store.ZFSDataset {
	rep := store.DefaultZFSReplica()
	rep.TargetKind, rep.TargetID = store.ZFSReplicaTargetServer, "srv"
	rep.AfterBackup, rep.Cadence = afterBackup, cadence
	return store.ZFSDataset{ID: id, Dataset: "tank/" + id, Enabled: true, Replica: rep}
}

func TestAReplicaWithACadenceOfItsOwnGetsAnEntry(t *testing.T) {
	off := replicaItem("off", false, "daily 04:00")
	off.Replica.TargetKind, off.Replica.TargetID = store.ZFSReplicaTargetNone, ""
	disabled := replicaItem("disabled", false, "daily 04:00")
	disabled.Enabled = false
	items := []store.ZFSDataset{
		replicaItem("own", false, "daily 04:00"),
		replicaItem("after", true, "daily 04:00"),
		replicaItem("manual", false, ""),
		replicaItem("paused", false, "off"),
		off,
		disabled,
	}
	sc, _ := newZFSScheduler(items)
	replica, rec := recordingBackup()
	sc.SetZFSReplicaJob(replica)

	if err := sc.ReloadWithGates(store.Settings{ZFSEnabled: true}, DueGates{}); err != nil {
		t.Fatalf("reload: %v", err)
	}
	got := entriesFor(sc, "replica", "zfs")
	if len(got) != 1 {
		t.Fatalf("want one replica entry, got %d", len(got))
	}
	sc.c.Entry(got[0].id).Job.Run()
	if len(*rec) != 1 || (*rec)[0] != "own" {
		t.Fatalf("the entry replicated %v, want only the item with its own cadence", *rec)
	}
}

func TestReplicaEntriesNeedNoPerItemSchedulesButTheDomain(t *testing.T) {
	sc, _ := newZFSScheduler([]store.ZFSDataset{replicaItem("own", false, "daily 04:00")})
	replica, _ := recordingBackup()
	sc.SetZFSReplicaJob(replica)

	if err := sc.ReloadWithGates(store.Settings{ZFSEnabled: true, PerItemSchedules: false}, DueGates{}); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := entriesFor(sc, "replica", "zfs"); len(got) != 1 {
		t.Fatalf("per-item schedules off: %d replica entries, want 1", len(got))
	}
	if err := sc.ReloadWithGates(store.Settings{ZFSEnabled: false}, DueGates{}); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := entriesFor(sc, "replica", "zfs"); len(got) != 0 {
		t.Fatalf("domain off: %d replica entries, want none", len(got))
	}
}
