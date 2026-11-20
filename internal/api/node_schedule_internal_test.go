package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/nodelink"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestHomeKeepsWhenANodeBacksUpEachSection(t *testing.T) {
	home := homeWithRelay(t)
	compose := addNode(t, home, "vps")
	id, _ := nodelink.NodeID(compose["APP_KEY"])
	before := time.Now().Unix()

	code, out := home.do(t, http.MethodPut, "/api/nodes/"+id+"/schedule", map[string]any{"containers": "daily 02:00"})
	if code != http.StatusOK || out["ok"] != true {
		t.Fatalf("schedule = %d %v", code, out)
	}
	code, _ = home.do(t, http.MethodPut, "/api/nodes/"+id+"/schedule", map[string]any{"files": "weekly sun 04:00"})
	if code != http.StatusOK {
		t.Fatalf("the second section = %d", code)
	}
	n, err := home.st.GetNode(id)
	if err != nil || n.Cadence("containers") != "daily 02:00" || n.Cadence("files") != "weekly sun 04:00" {
		t.Fatalf("node = %+v %v", n, err)
	}
	// A new cadence counts from when it was set, so no fire from before it
	// starts a backup.
	if n.StartedAt("containers") < before || n.StartedAt("files") < before {
		t.Errorf("started = %d and %d, want both from now on", n.StartedAt("containers"), n.StartedAt("files"))
	}

	if code, out := home.do(t, http.MethodPut, "/api/nodes/"+id+"/schedule", map[string]any{"files": "fortnightly"}); code != http.StatusBadRequest {
		t.Errorf("a cadence that does not read = %d %v", code, out)
	}
	if code, _ := home.do(t, http.MethodPut, "/api/nodes/nobody/schedule", map[string]any{"files": "off"}); code != http.StatusNotFound {
		t.Errorf("a node that is not there = %d", code)
	}
}

func TestHomeListsHowEachNodesBackupsGo(t *testing.T) {
	home := homeWithRelay(t)
	compose := addNode(t, home, "vps")
	id, _ := nodelink.NodeID(compose["APP_KEY"])
	if err := home.st.SaveNodeRuns(id, []store.NodeRun{
		{ID: "r1", Domain: "containers", Target: "web", Status: "success", StartedAt: 100, FinishedAt: 160},
		{ID: "r2", Domain: "files", Target: "photos", Status: "failed", StartedAt: 200, FinishedAt: 260},
		{ID: "r3", Domain: "containers", Target: "web", Status: "running", StartedAt: 300},
	}); err != nil {
		t.Fatal(err)
	}
	for _, ep := range []string{nodeFailedEpisode("files", "photos"), nodeAwayEpisode} {
		if err := home.st.OpenNodeEpisode(id, ep, 270); err != nil {
			t.Fatal(err)
		}
	}
	rr, err := home.st.CreateReceivedRepo(store.ReceivedRepo{ID: nodeRepoID(id, "files"), NodeID: id, Name: "vps files", Repo: "backups/vps/files", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := home.st.UpsertReceivedAlertState(store.ReceivedAlertState{ReceivedRepoID: rr.ID, Source: receiverUnreadEpisode, NotifiedAt: 280, BasedOn: 280}); err != nil {
		t.Fatal(err)
	}
	home.svc.nodes.mu.Lock()
	home.svc.nodes.open[id] = 4
	home.svc.nodes.mu.Unlock()

	code, out := home.do(t, http.MethodGet, "/api/nodes", nil)
	raw, _ := json.Marshal(out["nodes"])
	var nodes []NodeView
	if err := json.Unmarshal(raw, &nodes); err != nil || code != http.StatusOK || len(nodes) != 1 {
		t.Fatalf("list = %d %s", code, raw)
	}
	if got := nodes[0]; got.LastBackup != 160 || got.Failures != 2 || got.Anomalies != 4 {
		t.Errorf("node = %+v, want the last backup at 160, the failing folder set and the unread repository, and four anomalies", got)
	}
}
