package api_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/model"
)

// mcpKeyActivity reads GET /api/mcp/keys/{id}/activity.
func mcpKeyActivity(t *testing.T, h http.Handler, id string) (map[string]any, string) {
	t.Helper()
	w, m := doMCPKey(t, h, http.MethodGet, "/api/mcp/keys/"+id+"/activity", "")
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("key activity: status=%d body=%v", w.Code, m)
	}
	return m, w.Body.String()
}

// eventPairs is "tool -> outcome" for every event of an activity answer,
// newest first.
func eventPairs(t *testing.T, activity map[string]any) []string {
	t.Helper()
	var out []string
	for _, row := range mcpKeyRows(t, activity, "events") {
		out = append(out, fmt.Sprintf("%v -> %v", row["tool"], row["outcome"]))
	}
	return out
}

func TestMCPKeyActivityRecordsCallsAndRefusals(t *testing.T) {
	h, _, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})
	key, id := createMCPKey(t, h, "Desktop", false)
	other, _ := createMCPKey(t, h, "Laptop", true)

	if res := mcpCallTool(t, h, key, "get_health", ""); res.IsError {
		t.Fatalf("get_health: %v", res.Structured)
	}
	res := mcpCallTool(t, h, key, "start_backup", `{"domain":"containers","item":"secret-media-box"}`)
	if code := res.code(t); code != "not_permitted" {
		t.Fatalf("start_backup with a read-only key: %q", code)
	}
	mcpCallTool(t, h, other, "get_status", "")

	activity, body := mcpKeyActivity(t, h, id)
	got := eventPairs(t, activity)
	want := []string{"start_backup -> not_permitted", "get_health -> ok"}
	if strings.Join(got, "; ") != strings.Join(want, "; ") {
		t.Fatalf("events = %v, want %v", got, want)
	}
	for _, trace := range []string{key, "secret-media-box", "digest"} {
		if strings.Contains(body, trace) {
			t.Fatalf("the activity answer carries %q: %s", trace, body)
		}
	}
}

func TestMCPKeyActivityLinksTheRunsAKeyStartedAndCancelled(t *testing.T) {
	eng := &fakeResticEngine{block: make(chan struct{}), backupEntered: make(chan struct{}, 1)}
	docker := &fakeServiceDocker{inspect: model.Inspect{Name: "/plex", Image: "plex:latest", Running: true}}
	rig := newMCPStartRig(t, docker, eng)
	rig.target(t, "plex")

	if res := mcpCallTool(t, rig.h, rig.key, "start_backup", `{"domain":"containers","item":"plex"}`); res.IsError {
		t.Fatalf("start_backup: %v", res.Structured)
	}
	select {
	case <-eng.backupEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("the backup never reached the engine")
	}
	runID := mcpRunningBackupID(t, rig, rig.key)
	if res := mcpCallTool(t, rig.h, rig.key, "cancel_backup", fmt.Sprintf(`{"runId":%q}`, runID)); res.IsError {
		t.Fatalf("cancel_backup: %v", res.Structured)
	}
	waitForBackupDone(t, rig.svc)
	if _, err := rig.st.StartRun("elsewhere", "backup"); err != nil {
		t.Fatal(err)
	}

	activity, _ := mcpKeyActivity(t, rig.h, rig.keyID)
	events := mcpKeyRows(t, activity, "events")
	if events[0]["tool"] != "cancel_backup" || events[0]["outcome"] != "ok" || events[0]["runId"] != runID {
		t.Fatalf("newest event = %v, want the cancel naming run %s", events[0], runID)
	}
	runs := mcpKeyRows(t, activity, "runs")
	if len(runs) != 1 || runs[0]["id"] != runID || runs[0]["status"] != "cancelled" || runs[0]["target"] != "plex" {
		t.Fatalf("runs = %v, want the one cancelled backup of plex", runs)
	}
}

func TestMCPKeyListCountsTodaysCalls(t *testing.T) {
	h, _, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})
	key, _ := createMCPKey(t, h, "Laptop", true)
	createMCPKey(t, h, "Desktop", true)
	for range 3 {
		mcpCallTool(t, h, key, "get_health", "")
	}

	since := time.Now().Add(-time.Hour).Unix()
	w, m := doMCPKey(t, h, http.MethodGet, fmt.Sprintf("/api/mcp/keys?since=%d", since), "")
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("list keys: status=%d body=%v", w.Code, m)
	}
	calls := map[string]any{}
	for _, row := range mcpKeyRows(t, m, "keys") {
		calls[row["label"].(string)] = row["callsToday"]
	}
	if calls["Laptop"] != float64(3) || calls["Desktop"] != float64(0) {
		t.Fatalf("callsToday = %v, want 3 for the key that called and 0 for the other", calls)
	}

	future := time.Now().Add(time.Hour).Unix()
	_, m = doMCPKey(t, h, http.MethodGet, fmt.Sprintf("/api/mcp/keys?since=%d", future), "")
	for _, row := range mcpKeyRows(t, m, "keys") {
		if row["label"] == "Laptop" && row["callsToday"] != float64(3) {
			t.Fatalf("a start in the future counted %v calls, want today's 3", row["callsToday"])
		}
	}
}

func TestMCPKeyActivityRoute(t *testing.T) {
	h, st, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})
	_, id := createMCPKey(t, h, "Laptop", true)

	w, m := doMCPKey(t, h, http.MethodGet, "/api/mcp/keys/0123456789abcdef0123456789abcdef/activity", "")
	if w.Code != http.StatusOK || m["code"] != "mcp-key-not-found" {
		t.Fatalf("unknown key: status=%d body=%v", w.Code, m)
	}
	if w, _ := doMCPKey(t, h, http.MethodGet, "/api/mcp/keys/not-an-id/activity", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("malformed id: status = %d, want 400", w.Code)
	}

	activity, _ := mcpKeyActivity(t, h, id)
	if events := mcpKeyRows(t, activity, "events"); len(events) != 0 {
		t.Fatalf("a key that never called has events: %v", events)
	}
	if runs := mcpKeyRows(t, activity, "runs"); len(runs) != 0 {
		t.Fatalf("a key that never started a backup has runs: %v", runs)
	}

	enableLogin(t, st, strings.Repeat("a", 64))
	if w, _ := doMCPKey(t, h, http.MethodGet, "/api/mcp/keys/"+id+"/activity", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("without a session: status = %d, want 401", w.Code)
	}
}
