package api_test

import (
	"net/http"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const slowLoad = `{"samples":40,"disks":[{"name":"sdf","label":"disk1","role":"target","busy":0.98}],"slow":true,"cause":{"kind":"disk","name":"disk1","role":"target","share":0.98}}`

func TestMCPListRunsWordsTheBottleneck(t *testing.T) {
	docker := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{runningContainer("plex")}}
	h, st, _, key := newMCPToolRouter(t, docker, &fakeResticEngine{})
	plex := seedTarget(t, st, "plex")
	slow := seedRun(t, st, plex.ID, "backup", "success", store.RunMeta{})
	plain := seedRun(t, st, plex.ID, "backup", "success", store.RunMeta{})
	if err := st.SetRunLoad(slow, slowLoad); err != nil {
		t.Fatal(err)
	}
	rows := mcpRunRows(t, mcpCallTool(t, h, key, "list_runs", ""))
	byID := map[string]map[string]any{}
	for _, r := range rows {
		byID[r["id"].(string)] = r
	}
	if got := byID[slow]["bottleneck"]; got != "The target disk disk1 was 98% busy." {
		t.Fatalf("bottleneck = %v", got)
	}
	if _, ok := byID[plain]["bottleneck"]; ok {
		t.Fatalf("a run without a cause carries one: %v", byID[plain])
	}
}

func TestRunsCarryTheBottleneckForTheInterface(t *testing.T) {
	h, st := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
	plex := seedTarget(t, st, "plex")
	id := seedRun(t, st, plex.ID, "backup", "success", store.RunMeta{})
	if err := st.SetRunLoad(id, slowLoad); err != nil {
		t.Fatal(err)
	}
	w, m := doJSON(t, h, http.MethodGet, "/api/runs", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	run := m["runs"].([]any)[0].(map[string]any)
	b, _ := run["bottleneck"].(map[string]any)
	if b["kind"] != "disk" || b["name"] != "disk1" || b["role"] != "target" {
		t.Fatalf("bottleneck = %v", run["bottleneck"])
	}
}
