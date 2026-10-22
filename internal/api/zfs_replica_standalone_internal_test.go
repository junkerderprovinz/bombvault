package api

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfsrepl"
)

// replicaOnlyRig is a replica rig whose ZFS section has no place to back up
// to, so its one item can only replicate.
func replicaOnlyRig(t *testing.T) *replicaRig {
	t.Helper()
	r := newReplicaRig(t)
	if err := r.st.SetLocal(zfsDomain, false); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestAReplicaFollowsABackupThatFailed(t *testing.T) {
	r := newReplicaRig(t)
	if _, err := r.s.BackupZFSDataset(context.Background(), r.item.ID); err == nil {
		t.Fatal("the backup worked without a ZFS host")
	}
	r.s.replica.work.Wait()
	if len(r.entries) != 1 {
		t.Fatalf("the replica ran %d times after a failed backup, want once", len(r.entries))
	}
}

func TestACancelledBackupIsNotFollowedByTheReplica(t *testing.T) {
	r := newReplicaRig(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _ = r.s.BackupZFSDataset(ctx, r.item.ID)
	r.s.replica.work.Wait()
	if len(r.entries) != 0 {
		t.Errorf("the replica ran %d times after a cancelled backup", len(r.entries))
	}
}

func TestAnItemWithOnlyAReplicaSendsItInsteadOfBackingUp(t *testing.T) {
	r := replicaOnlyRig(t)
	if _, err := r.s.BackupZFSDataset(context.Background(), r.item.ID); err != nil {
		t.Fatalf("a replica-only backup failed: %v", err)
	}
	if len(r.entries) != 1 {
		t.Fatalf("the replica ran %d times, want once", len(r.entries))
	}
	if runs, err := r.st.RecentRunsOfKind(r.item.ID, "backup", 1); err != nil || len(runs) != 0 {
		t.Errorf("a replica-only item recorded a backup run: %v (err %v)", runs, err)
	}
	if got := r.lastRun().Status; got != "success" {
		t.Errorf("replica run = %s", got)
	}
}

func TestAFailedReplicaFailsTheBackupOfAnItemWithOnlyAReplica(t *testing.T) {
	r := replicaOnlyRig(t)
	r.result = func(e zfsrepl.Entry) (zfsrepl.Result, error) { return zfsrepl.Result{}, errors.New("link down") }
	if _, err := r.s.BackupZFSDataset(context.Background(), r.item.ID); err == nil {
		t.Error("the backup of a replica-only item passed over a failed replica")
	}
}

func TestAnItemWithOnlyAReplicaIsNotRefusedForWantingAPlace(t *testing.T) {
	r := replicaOnlyRig(t)
	if err := r.s.refuseNoPlace(zfsDomain, r.item.ID); err != nil {
		t.Errorf("refused: %v", err)
	}
	if err := r.st.SetZFSReplicaTarget(r.item.ID, store.ZFSReplicaTargetNone, ""); err != nil {
		t.Fatal(err)
	}
	if err := r.s.refuseNoPlace(zfsDomain, r.item.ID); !errors.Is(err, ErrNoPlace) {
		t.Errorf("an item with neither a place nor a replica: %v, want ErrNoPlace", err)
	}
}

func TestAnItemWithItsOwnRepositoryIsNeverReplicaOnly(t *testing.T) {
	r := replicaOnlyRig(t)
	if !r.s.zfsReplicaOnly(r.item) {
		t.Fatal("the rig item is not replica-only")
	}
	r.item.Repo = "repo-1"
	if r.s.zfsReplicaOnly(r.item) {
		t.Error("an item with a repository of its own counts as replica-only")
	}
}

func TestAReplicaOnlyItemIsProtectedOnceItIsReplicated(t *testing.T) {
	r := replicaOnlyRig(t)
	settings, _ := r.st.GetSettings()
	cover := func() CoverageDomain {
		nowhere, err := r.s.placelessSections()
		if err != nil {
			t.Fatal(err)
		}
		return r.s.coverZFSDatasets(settings, declinedBackups{}, nowhere)
	}
	before := cover()
	if before.Total != 1 || before.Waiting != 1 || before.Protected != 0 || len(before.Unprotected) != 0 {
		t.Errorf("before the first replica: %+v, want one item waiting and none unprotected", before)
	}
	if err := r.s.ReplicateZFSDataset(context.Background(), r.item.ID); err != nil {
		t.Fatal(err)
	}
	after := cover()
	if after.Protected != 1 || after.Waiting != 0 || len(after.Unprotected) != 0 {
		t.Errorf("after a replica: %+v, want one item protected", after)
	}
}

func TestAnUnscheduledItemWithAReplicaIsNotCalledNeverBackedUp(t *testing.T) {
	r := replicaOnlyRig(t)
	settings, _ := r.st.GetSettings()
	settings.ZFSSchedule = ""
	unprotected := func() CoverageItem {
		nowhere, err := r.s.placelessSections()
		if err != nil {
			t.Fatal(err)
		}
		got := r.s.coverZFSDatasets(settings, declinedBackups{}, nowhere)
		if len(got.Unprotected) != 1 {
			t.Fatalf("coverage %+v, want the unscheduled item unprotected", got)
		}
		return got.Unprotected[0]
	}
	if !unprotected().NeverBackedUp {
		t.Error("an item with neither a backup nor a replica is not called never backed up")
	}
	if err := r.s.ReplicateZFSDataset(context.Background(), r.item.ID); err != nil {
		t.Fatal(err)
	}
	if unprotected().NeverBackedUp {
		t.Error("an item with a replica is called never backed up")
	}
}

func TestAReplicaOnItsOwnPlanProtectsAnItemWithoutABackupSchedule(t *testing.T) {
	r := replicaOnlyRig(t)
	if err := r.st.SetZFSReplicaAfterBackup(r.item.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := r.st.SetZFSReplicaCadence(r.item.ID, "daily 03:00"); err != nil {
		t.Fatal(err)
	}
	r.item, _ = r.st.GetZFSDataset(r.item.ID)
	settings, _ := r.st.GetSettings()
	settings.ZFSSchedule = ""
	cover := func() CoverageDomain {
		nowhere, err := r.s.placelessSections()
		if err != nil {
			t.Fatal(err)
		}
		return r.s.coverZFSDatasets(settings, declinedBackups{}, nowhere)
	}
	if got := cover(); got.Waiting != 1 || len(got.Unprotected) != 0 {
		t.Errorf("before its first replica: %+v, want the item waiting", got)
	}
	if err := r.s.ReplicateZFSDataset(context.Background(), r.item.ID); err != nil {
		t.Fatal(err)
	}
	if got := cover(); got.Protected != 1 || len(got.Unprotected) != 0 {
		t.Errorf("after a replica: %+v, want the item protected", got)
	}
}

func TestAReplicaIsAllTheDomainHasToBeCurrentBy(t *testing.T) {
	r := replicaOnlyRig(t)
	settings, _ := r.st.GetSettings()
	zfsStatus := func() DomainStatusEntry {
		statuses, err := r.s.domainStatusFrom(settings)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range statuses {
			if e.Domain == zfsDomain {
				return e
			}
		}
		t.Fatal("no zfs status")
		return DomainStatusEntry{}
	}
	if got := zfsStatus(); got.LastSuccess != 0 {
		t.Fatalf("last success before any replica = %d", got.LastSuccess)
	}
	if err := r.s.ReplicateZFSDataset(context.Background(), r.item.ID); err != nil {
		t.Fatal(err)
	}
	if got := zfsStatus(); got.LastSuccess == 0 || got.Status != "ok" {
		t.Errorf("after a replica: last success %d, status %q", got.LastSuccess, got.Status)
	}
}

func TestTheItemListSaysHowTheReplicaStands(t *testing.T) {
	r := newReplicaRig(t)
	item := func() map[string]any {
		_, m, _ := r.call(http.MethodGet, "/api/items/zfs/"+r.item.ID, "")
		it, _ := m["item"].(map[string]any)
		return it
	}
	if got := item(); got["replica"] == nil {
		t.Fatalf("an item with a target has no replica in the list: %v", got)
	} else if rep := got["replica"].(map[string]any); rep["lastAt"] != float64(0) || rep["lastStatus"] != "" {
		t.Errorf("before a run: %v", rep)
	}
	if err := r.s.ReplicateZFSDataset(context.Background(), r.item.ID); err != nil {
		t.Fatal(err)
	}
	rep := item()["replica"].(map[string]any)
	if rep["lastAt"].(float64) == 0 || rep["lastStatus"] != "success" {
		t.Errorf("after a run: %v", rep)
	}
	r.result = func(e zfsrepl.Entry) (zfsrepl.Result, error) { return zfsrepl.Result{}, errors.New("link down") }
	_ = r.s.ReplicateZFSDataset(context.Background(), r.item.ID)
	rep = item()["replica"].(map[string]any)
	if rep["lastAt"].(float64) == 0 || rep["lastStatus"] != "failed" {
		t.Errorf("after a failed run the list says %v, want the earlier success kept and the failure named", rep)
	}
	if err := r.st.SetZFSReplicaTarget(r.item.ID, store.ZFSReplicaTargetNone, ""); err != nil {
		t.Fatal(err)
	}
	if item()["replica"] != nil {
		t.Error("an item that replicates nowhere carries a replica")
	}
}

func TestTheItemListSaysWhenTheReplicaIsAllAnItemGets(t *testing.T) {
	r := newReplicaRig(t)
	h := &Handler{svc: r.s, store: r.st}
	only := func() (entry any, listed bool) {
		_, m, _ := r.call(http.MethodGet, "/api/items/zfs/"+r.item.ID, "")
		it, _ := m["item"].(map[string]any)
		rep, _ := it["replica"].(map[string]any)
		hist, err := h.readItemHistory()
		if err != nil {
			t.Fatal(err)
		}
		return rep["only"], hist.replicas[r.item.ID].Only
	}
	if entry, listed := only(); entry != nil || listed {
		t.Errorf("an item with a place to back up to is called replica-only: entry %v, list %v", entry, listed)
	}
	if err := r.st.SetLocal(zfsDomain, false); err != nil {
		t.Fatal(err)
	}
	if entry, listed := only(); entry != true || !listed {
		t.Errorf("an item with no place to back up to is not called replica-only: entry %v, list %v", entry, listed)
	}
}

func TestAReplicatedDatasetIsMarkedInTheTreeEvenBeforeItsFirstBackup(t *testing.T) {
	r := newReplicaRig(t)
	rows := []store.ZFSMember{{ItemID: r.item.ID, Dataset: "cache/appdata"}, {ItemID: r.item.ID, Dataset: "cache/appdata/plex"}}
	if err := r.st.ReplaceZFSMembers(r.item.ID, rows); err != nil {
		t.Fatal(err)
	}
	views := func() []ZFSMemberView {
		list, err := r.s.ListZFSDatasetViews(context.Background())
		if err != nil || len(list) != 1 {
			t.Fatalf("views: %v (err %v)", list, err)
		}
		return list[0].Members
	}
	for _, m := range views() {
		if m.Replicated {
			t.Errorf("%s is marked replicated before any run", m.Dataset)
		}
	}
	if err := r.s.ReplicateZFSDataset(context.Background(), r.item.ID); err != nil {
		t.Fatal(err)
	}
	for _, m := range views() {
		if !m.Replicated || m.Outcome != "" {
			t.Errorf("%s after a replica: replicated %v, outcome %q", m.Dataset, m.Replicated, m.Outcome)
		}
	}
}

func TestTheReplicaViewSaysWhenTheBackupRunOnlySendsIt(t *testing.T) {
	r := newReplicaRig(t)
	only := func() any {
		_, v, _ := r.call(http.MethodGet, "/api/zfs/datasets/"+r.item.ID+"/replica", "")
		return v["only"]
	}
	if only() != false {
		t.Error("an item with a place to back up to is called replica-only")
	}
	if err := r.st.SetLocal(zfsDomain, false); err != nil {
		t.Fatal(err)
	}
	if only() != true {
		t.Error("an item with no place to back up to is not called replica-only")
	}
}

func TestTheDueGateSeesTheReplicaOfAnItemThatOnlyReplicates(t *testing.T) {
	r := replicaOnlyRig(t)
	gate := schedule.ZFSDueGate(r.st, zfsReplicaGate(r.s))
	if last, err := gate(); err != nil || !last.IsZero() {
		t.Fatalf("before any replica: %v (err %v), want never", last, err)
	}
	if err := r.s.ReplicateZFSDataset(context.Background(), r.item.ID); err != nil {
		t.Fatal(err)
	}
	if last, err := gate(); err != nil || last.IsZero() {
		t.Errorf("after a replica: %v (err %v), want its finish", last, err)
	}
	if err := r.st.SetLocal(zfsDomain, true); err != nil {
		t.Fatal(err)
	}
	if last, err := gate(); err != nil || !last.IsZero() {
		t.Errorf("an item that backs up again answers for the round with its replica: %v (err %v)", last, err)
	}
}

func TestReplicaOnlyLeavesThePlacesAloneAndSwitchingBackBringsThemBack(t *testing.T) {
	r := newReplicaRig(t)
	target, err := r.st.UpsertOffsiteTarget(store.OffsiteTarget{Domain: zfsDomain, Name: "attic", Repo: "rest:http://attic:8000/zfs", Enabled: true, SortOrder: 1})
	if err != nil {
		t.Fatal(err)
	}
	only := func() any {
		_, v, _ := r.call(http.MethodGet, "/api/zfs/datasets/"+r.item.ID+"/replica", "")
		return v["only"]
	}
	if _, m, _ := r.call(http.MethodPut, "/api/zfs/replica/only", `{"on":true}`); m["ok"] != true {
		t.Fatalf("switching on: %v", m)
	}
	if only() != true {
		t.Fatal("the item backs up although the section only replicates")
	}
	if !errors.Is(r.s.refuseNoPlace(zfsDomain, zfsSeedItem(t, r.st, "cache/media").ID), ErrNoPlace) {
		t.Error("an item without a replica has somewhere to back up to")
	}
	if _, m, _ := r.call(http.MethodPut, "/api/zfs/replica/only", `{"on":false}`); m["ok"] != true {
		t.Fatalf("switching off: %v", m)
	}
	if only() != false {
		t.Error("switching back left the item replica-only")
	}
	if got, _, _ := r.st.GetOffsiteTarget(target.ID); !got.Enabled {
		t.Error("the off-site target stayed switched off")
	}
	if off, _ := r.st.LocalOff(); off[zfsDomain] {
		t.Error("the own folder stayed switched off")
	}
}

func TestSwitchingReplicaOnlyOffGivesASectionWithoutPlacesItsFolderBack(t *testing.T) {
	r := replicaOnlyRig(t)
	if err := r.s.SetZFSReplicaOnly(false); err != nil {
		t.Fatal(err)
	}
	if r.s.zfsReplicaOnly(r.item) {
		t.Error("the section still has no place to back up to")
	}
}

func TestBackUpNowOnAReplicaOnlyItemSendsTheReplicaWithoutTheBackupLock(t *testing.T) {
	r := replicaOnlyRig(t)
	release := make(chan struct{})
	started := make(chan struct{})
	r.result = func(zfsrepl.Entry) (zfsrepl.Result, error) {
		close(started)
		<-release
		return zfsrepl.Result{Snapshot: replicaSnap}, nil
	}
	_, m, _ := r.call(http.MethodPost, "/api/zfs/datasets/"+r.item.ID+"/backup", "")
	if m["ok"] != true {
		t.Fatalf("answer = %v, want the start", m)
	}
	<-started
	if r.s.batchActive.Load() {
		t.Error("the replica holds the lock every other backup and restore waits for")
	}
	close(release)
	r.s.replica.work.Wait()
	if got := r.lastRun().Status; got != "success" {
		t.Errorf("replica run = %s", got)
	}
}

func TestBackUpNowOnAReplicaOnlyItemAnswersWhyTheReplicaCannotRun(t *testing.T) {
	r := replicaOnlyRig(t)
	r.srv.Enabled = false
	if err := r.st.UpdateZFSReplicaServer(r.srv); err != nil {
		t.Fatal(err)
	}
	code, m, _ := r.call(http.MethodPost, "/api/zfs/datasets/"+r.item.ID+"/backup", "")
	if code != http.StatusConflict || m["code"] != "server-disabled" {
		t.Errorf("answer = %d %v, want the refusal with its code", code, m)
	}
}
