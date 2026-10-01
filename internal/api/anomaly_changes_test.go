package api_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestFindingExplainsWhichFoldersChanged(t *testing.T) {
	eng := &fakeResticEngine{
		snaps: plexSnapshots(),
		noLockEntries: []restic.FileEntry{
			{Path: "/host/user/appdata/plex/Library/Cache", Type: "dir"},
			{Path: "/host/user/appdata/plex/Library/Cache/old", Type: "file", Size: 900},
			{Path: "/host/user/appdata/plex/Library/Cache/new", Type: "file", Size: 40},
			{Path: "/host/user/appdata/plex/Preferences.xml", Type: "file", Size: 5},
		},
		diffChanges: []restic.DiffChange{
			{Path: "/host/user/appdata/plex/Library/", Modifier: "U"},
			{Path: "/host/user/appdata/plex/Library/Cache/old", Modifier: "-"},
			{Path: "/host/user/appdata/plex/Library/Cache/new", Modifier: "+"},
			{Path: "/host/user/appdata/plex/Preferences.xml", Modifier: "U"},
		},
	}
	h, st := breakdownRouter(t, eng)
	plex, err := st.GetTargetByContainer("plex")
	if err != nil {
		t.Fatal(err)
	}
	good := seedRun(t, st, plex.ID, "backup", "running", store.RunMeta{})
	if err := st.FinishRun(good, "success", "aaaa1111aaaa1111", 0, ""); err != nil {
		t.Fatal(err)
	}
	bad := seedRun(t, st, plex.ID, "backup", "running", store.RunMeta{})
	if err := st.FinishRun(bad, "success", "bbbb2222bbbb2222", 0, ""); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	if _, err := st.ApplyAnomalyChanges(store.AnomalyChanges{Now: now, Insert: []store.Anomaly{{
		Fingerprint: "source|item|" + plex.ID + "|source_bytes_shrink",
		Detector:    "source", Metric: "source_bytes_shrink", Severity: "critical",
		ScopeKind: "item", ScopeID: plex.ID, TargetID: plex.ID, Domain: "container",
		RunID: bad, LastRunID: bad, LastGoodRunID: good,
		FirstSeenAt: now, LastSeenAt: now, LastRunAt: now,
	}}}); err != nil {
		t.Fatal(err)
	}
	rows, _, err := st.ListAnomalies(store.AnomalyFilter{})
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows = %v, %v", rows, err)
	}

	deadline := time.Now().Add(5 * time.Second)
	var changes map[string]any
	for {
		w, m := doJSON(t, h, http.MethodGet, "/api/anomalies/"+rows[0].ID+"/changes", "")
		if w.Code != http.StatusOK || m["ok"] != true {
			t.Fatalf("changes: %d %v", w.Code, m)
		}
		changes = m["changes"].(map[string]any)
		if changes["state"] != "running" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the comparison never finished")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if changes["state"] != "ready" {
		t.Fatalf("changes = %v", changes)
	}
	summary := changes["summary"].(map[string]any)
	if summary["focus"] != "Library/Cache" || summary["regenerable"] != true {
		t.Fatalf("summary = %v", summary)
	}
	total := summary["total"].(map[string]any)
	if total["removedBytes"] != float64(900) || total["addedBytes"] != float64(40) || total["changedFiles"] != float64(0) {
		t.Fatalf("total = %v", total)
	}

	w, m := doJSON(t, h, http.MethodGet, "/api/anomalies/"+rows[0].ID, "")
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("anomaly: %d %v", w.Code, m)
	}
	if at := m["anomaly"].(map[string]any)["firstRunAt"]; at == nil || at.(float64) < float64(now-60) {
		t.Fatalf("firstRunAt = %v", at)
	}
}
