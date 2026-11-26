package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// pairedCall is call with the session of a paired phone.
func (r *replicaRig) pairedCall(method, path string) (int, string) {
	r.t.Helper()
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	r.h.ServeHTTP(w, withPaired(req))
	return w.Code, w.Body.String()
}

func TestAPairedSessionReadsTheReplicaServersWithoutTheirAddress(t *testing.T) {
	r := newReplicaRig(t)
	r.server.pools = "tank\t1000\t3000\n"

	code, body := r.pairedCall(http.MethodGet, "/api/zfs/replica/servers")
	if code != http.StatusOK {
		t.Fatalf("a paired session listing the servers = %d %s", code, body)
	}
	for _, leak := range []string{"backup.lan", `"user":"root"`, `"port":22`} {
		if strings.Contains(body, leak) {
			t.Errorf("a paired session read %s in %s", leak, body)
		}
	}
	var list []ZFSReplicaServerView
	if err := json.Unmarshal([]byte(body), &list); err != nil || len(list) != 1 {
		t.Fatalf("the server list %s holds no one server (%v)", body, err)
	}
	got := list[0]
	if got.Name != "backup" || got.Root != "tank/bombvault-replica" || got.FreeBytes != 3000 || len(got.UsedBy) != 1 || !got.Reachable {
		t.Errorf("a paired session lost what the cards show: %+v", got)
	}

	_, _, own := r.call(http.MethodGet, "/api/zfs/replica/servers", "")
	if srv, _ := own[0].(map[string]any); srv["host"] != "backup.lan" || srv["user"] != "root" {
		t.Errorf("the person's own session lost the address: %v", own)
	}
}

func TestAPairedSessionReadsAnItemsReplicaAndItsSnapshots(t *testing.T) {
	r := newReplicaRig(t)
	if err := r.s.ReplicateZFSDataset(context.Background(), r.item.ID); err != nil {
		t.Fatal(err)
	}
	replica := "tank/bombvault-replica/bottich/cache/appdata"
	r.server.points[replica] = replica + "@" + replicaSnap + "\t7\t10\n"

	code, body := r.pairedCall(http.MethodGet, "/api/zfs/datasets/"+r.item.ID+"/replica")
	if code != http.StatusOK {
		t.Fatalf("a paired session reading the replica = %d %s", code, body)
	}
	if strings.Contains(body, "backup.lan") {
		t.Errorf("a paired session read the server's address in %s", body)
	}
	var v ZFSReplicaView
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Fatal(err)
	}
	if v.Target.Kind != "server" || v.Target.ID != r.srv.ID {
		t.Errorf("the replica reads its target as %+v, want the server", v.Target)
	}
	if v.State != zfsReplicaOK || len(v.Snapshots) != 1 || len(v.Members) == 0 {
		t.Errorf("a paired session reads the replica as %s with %d snapshots and %d members, want ok, the one snapshot and its members", v.State, len(v.Snapshots), len(v.Members))
	}
}
