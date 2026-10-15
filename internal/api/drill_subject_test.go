package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

func TestADRDrillNamesTheContainerAndSnapshotItRestored(t *testing.T) {
	eng := &fakeResticEngine{
		snaps: []restic.Snapshot{
			{ID: "aaaa1111bbbb2222", Time: "2026-06-01T00:00:00Z", Tags: []string{"container:plex"}},
			{ID: "cccc3333dddd4444", Time: "2026-07-01T00:00:00Z", Tags: []string{"container:plex"}},
		},
		lsEntries: []restic.FileEntry{{Path: "/appdata/plex/a.conf", Type: "file", Size: 100}},
	}
	h, st, svc := newTestRouterSvc(t, &fakeServiceDocker{}, eng)
	settings := mustSettings(t, st)
	settings.EncryptionEnabled = false
	settings.ContainersOffsite = "rest:http://192.168.20.9:8000/containers"
	settings.DRDrillTarget = "plex"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	drill, err := svc.RunRestoreDrill(context.Background(), "containers", "offsite", "dr", false)
	if err != nil {
		t.Fatalf("RunRestoreDrill: %v", err)
	}
	if drill.Item != "plex" || drill.SnapshotID != "cccc3333" {
		t.Fatalf("drill = %+v, want item plex and snapshot cccc3333", drill)
	}
	latest, found, err := svc.LatestDrill("containers", "offsite")
	if err != nil || !found || latest.Item != "plex" || latest.SnapshotID != "cccc3333" {
		t.Fatalf("stored drill = %+v, %v, %v; want item and snapshot kept", latest, found, err)
	}

	statuses, err := svc.DomainStatus()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range statuses {
		if d.Domain == "containers" && (d.LastDRDrillItem != "plex" || d.LastDRDrillSnapshot != "cccc3333") {
			t.Fatalf("status = %q, %q; want plex and cccc3333", d.LastDRDrillItem, d.LastDRDrillSnapshot)
		}
	}

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/runs", nil))
	var resp struct {
		Runs []struct {
			Kind       string `json:"kind"`
			Target     string `json:"target"`
			SnapshotID string `json:"snapshotId"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	if len(resp.Runs) != 1 || resp.Runs[0].Kind != "drdrill" || resp.Runs[0].Target != "plex" || resp.Runs[0].SnapshotID != "cccc3333" {
		t.Fatalf("runs = %+v, want one drdrill run on plex at cccc3333", resp.Runs)
	}
}
