package api_test

import (
	"net/http"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestThePrimaryOffsiteCopyCanUseADestination(t *testing.T) {
	h, st := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
	d, err := st.SaveDestination(store.OffsiteTarget{Name: "Box", Repo: "rest:http://box:8000/bv", Immutable: true})
	if err != nil {
		t.Fatal(err)
	}
	const loc = "rest:http://box:8000/bv/containers"

	_, m := doJSON(t, h, http.MethodPost, "/api/offsite/destinations/"+d.ID+"/primary/containers", "")
	if m["ok"] != true || m["location"] != loc || m["immutable"] != true {
		t.Fatalf("answer = %v", m)
	}
	if target := m["target"].(map[string]any); target["destinationId"] != d.ID || target["sortOrder"] != float64(0) {
		t.Fatalf("target = %v, want the primary following Box", target)
	}

	// A page loaded before the switch still has append-only off.
	if _, m := doJSON(t, h, http.MethodPut, "/api/settings", `{"containersOffsite":"`+loc+`","containersOffsiteImmutable":false}`); m["ok"] != true {
		t.Fatalf("settings save = %v", m)
	}
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.ContainersOffsite != loc || !s.ContainersOffsiteImmutable {
		t.Fatalf("containers off-site = %q, append-only %v, want %q and on", s.ContainersOffsite, s.ContainersOffsiteImmutable, loc)
	}
	primary, _, err := st.FieldOffsiteTarget("containers")
	if err != nil || primary.DestinationID != d.ID || primary.Name != "Box" {
		t.Fatalf("primary after the save = %+v, %v, want it still following Box", primary, err)
	}

	if _, m := doJSON(t, h, http.MethodPut, "/api/offsite/destinations/"+d.ID, `{"name":"Box","immutable":false}`); m["ok"] != true {
		t.Fatalf("destination edit = %v", m)
	}
	if s, _ := st.GetSettings(); s.ContainersOffsiteImmutable {
		t.Fatal("switching off append-only on the destination left it on for the primary that follows it")
	}
}
