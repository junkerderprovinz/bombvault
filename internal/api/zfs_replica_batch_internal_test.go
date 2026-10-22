package api

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfsrepl"
)

// replicating adds an item that replicates to the rig's server.
func (r *replicaRig) replicating(root string) store.ZFSDataset {
	r.t.Helper()
	d := zfsSeedItem(r.t, r.st, root)
	if err := r.st.SetZFSReplicaTarget(d.ID, store.ZFSReplicaTargetServer, r.srv.ID); err != nil {
		r.t.Fatal(err)
	}
	return d
}

func (r *replicaRig) replicateAll(ids ...string) map[string]any {
	r.t.Helper()
	_, m, _ := r.call(http.MethodPost, "/api/zfs/replica/run-all", `{"ids":["`+strings.Join(ids, `","`)+`"]}`)
	return m
}

func (r *replicaRig) roots() []string {
	var out []string
	for _, e := range r.entries {
		out = append(out, e.Root)
	}
	return out
}

func TestReplicateAllRunsEveryItemOneAfterTheOther(t *testing.T) {
	r := newReplicaRig(t)
	r.s.progress = progress.NewStore()
	events, stop := r.s.progress.Subscribe()
	defer stop()
	b, c := r.replicating("cache/media"), r.replicating("cache/vms")

	m := r.replicateAll(r.item.ID, b.ID, c.ID)
	if m["ok"] != true || m["started"] != float64(3) {
		t.Fatalf("answer = %v, want three started", m)
	}
	r.s.replica.work.Wait()

	if got, want := r.roots(), []string{"cache/appdata", "cache/media", "cache/vms"}; !slices.Equal(got, want) {
		t.Fatalf("replicated %v, want %v", got, want)
	}
	for e := range events {
		if e.Key == zfsReplicaBatchKey && !e.Active {
			if e.Done != 3 || e.Total != 3 || e.Failed != 0 {
				t.Errorf("the batch ended at %d of %d with %d failed", e.Done, e.Total, e.Failed)
			}
			break
		}
	}
}

func TestReplicateAllGoesOnPastAnItemThatIsRefusedOrFails(t *testing.T) {
	r := newReplicaRig(t)
	off, err := r.st.CreateZFSReplicaServer(store.ZFSReplicaServer{
		Name: "off", Host: "off.lan", User: "root", Port: 22, Pool: "tank", Root: "tank/off", Enabled: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	parked := zfsSeedItem(t, r.st, "cache/parked")
	if err := r.st.SetZFSReplicaTarget(parked.ID, store.ZFSReplicaTargetServer, off.ID); err != nil {
		t.Fatal(err)
	}
	held, last := r.replicating("cache/held"), r.replicating("cache/last")
	unlock, _ := r.s.lockZFSReplica(held.ID)
	defer unlock()
	r.result = func(e zfsrepl.Entry) (zfsrepl.Result, error) {
		if e.Root == r.item.Dataset {
			return zfsrepl.Result{}, errors.New("target unreachable")
		}
		return zfsrepl.Result{Snapshot: replicaSnap}, nil
	}

	m := r.replicateAll(r.item.ID, parked.ID, held.ID, last.ID)
	r.s.replica.work.Wait()

	refused, _ := m["refused"].([]any)
	if m["ok"] != true || m["started"] != float64(3) || len(refused) != 1 {
		t.Fatalf("answer = %v, want the parked item refused and three started", m)
	}
	if first, _ := refused[0].(map[string]any); first["id"] != parked.ID || first["code"] != "server-disabled" {
		t.Errorf("refused = %v, want the parked item with server-disabled", first)
	}
	if got, want := r.roots(), []string{"cache/appdata", "cache/last"}; !slices.Equal(got, want) {
		t.Fatalf("replicated %v, want %v", got, want)
	}
}

func TestASecondReplicateAllWhileOneRunsIsRefused(t *testing.T) {
	r := newReplicaRig(t)
	release := make(chan struct{})
	started := make(chan struct{})
	r.result = func(zfsrepl.Entry) (zfsrepl.Result, error) {
		close(started)
		<-release
		return zfsrepl.Result{Snapshot: replicaSnap}, nil
	}
	if m := r.replicateAll(r.item.ID); m["ok"] != true {
		t.Fatalf("first answer = %v", m)
	}
	<-started

	m := r.replicateAll(r.item.ID)
	close(release)
	r.s.replica.work.Wait()
	if m["ok"] != false || m["busy"] != true {
		t.Fatalf("second answer = %v, want busy", m)
	}
	if len(r.entries) != 1 {
		t.Errorf("the item replicated %d times, want once", len(r.entries))
	}

	r.result = nil
	if m := r.replicateAll(r.item.ID); m["ok"] != true {
		t.Errorf("answer once the batch ended = %v", m)
	}
	r.s.replica.work.Wait()
}
