package api_test

import (
	"net/http"
	"testing"
)

func TestSnapshotPlansRoundTripWithoutTheHost(t *testing.T) {
	h, _ := newZFSTestRouter(t, &fakeResticEngine{})

	w, m := doJSON(t, h, http.MethodGet, "/api/host-snapshots", "")
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("list: %d %v", w.Code, m)
	}
	kinds, _ := m["kinds"].(map[string]any)
	zfsKind, _ := kinds["zfs"].(map[string]any)
	if zfsKind["code"] != "ssh-missing" {
		t.Errorf("zfs = %v, want ssh-missing while no host is wired", kinds["zfs"])
	}
	if snaps, _ := m["snapshots"].([]any); snaps == nil {
		t.Errorf("snapshots = %v, want an empty list", m["snapshots"])
	}

	_, m = doJSON(t, h, http.MethodPost, "/api/host-snapshots/plans", `{"kind":"zfs","source":"tank/apps","cadence":"off","keep":3,"enabled":true}`)
	if m["ok"] != false || m["code"] != "invalid-plan" {
		t.Errorf("a plan that never runs = %v, want invalid-plan", m)
	}
	_, m = doJSON(t, h, http.MethodPost, "/api/host-snapshots/plans", `{"kind":"zfs","source":"tank/apps@x","cadence":"daily 03:00","keep":3,"enabled":true}`)
	if m["code"] != "invalid-plan" {
		t.Errorf("a snapshot as the source = %v, want invalid-plan", m)
	}
	_, m = doJSON(t, h, http.MethodPost, "/api/host-snapshots/plans", `{"kind":"zfs","source":"tank/apps","cadence":"daily 03:00","keep":3,"recursive":true,"skipAsleep":true,"enabled":true}`)
	plan, _ := m["plan"].(map[string]any)
	id, _ := plan["id"].(string)
	if m["ok"] != true || id == "" {
		t.Fatalf("add: %v", m)
	}

	_, m = doJSON(t, h, http.MethodPut, "/api/host-snapshots/plans/"+id, `{"kind":"vm","source":"win11","cadence":"weekly Sun 04:00","keep":5,"recursive":true,"skipAsleep":false,"enabled":false}`)
	plan, _ = m["plan"].(map[string]any)
	if m["ok"] != true || plan["source"] != "tank/apps" || plan["kind"] != "zfs" || plan["keep"] != float64(5) || plan["enabled"] != false {
		t.Errorf("update = %v, want keep and switches changed and the source kept", m)
	}

	_, m = doJSON(t, h, http.MethodPost, "/api/host-snapshots/plans/"+id+"/run", "")
	if m["ok"] != false || m["code"] != "ssh-missing" {
		t.Errorf("run without the host = %v, want ssh-missing", m)
	}

	_, m = doJSON(t, h, http.MethodGet, "/api/host-snapshots", "")
	if plans, _ := m["plans"].([]any); len(plans) != 1 {
		t.Errorf("plans = %v, want the one", m["plans"])
	}
	w, m = doJSON(t, h, http.MethodDelete, "/api/host-snapshots/plans/"+id, "")
	if m["ok"] != true {
		t.Fatalf("delete: %d %v", w.Code, m)
	}
	w, _ = doJSON(t, h, http.MethodDelete, "/api/host-snapshots/plans/"+id, "")
	if w.Code != http.StatusNotFound {
		t.Errorf("second delete = %d, want 404", w.Code)
	}
}

func TestADeleteNeedsASelection(t *testing.T) {
	h, _ := newZFSTestRouter(t, &fakeResticEngine{})
	_, m := doJSON(t, h, http.MethodPost, "/api/host-snapshots/delete", `{"refs":[]}`)
	if m["ok"] != false || m["code"] != "invalid-selection" {
		t.Errorf("empty delete = %v", m)
	}
}
