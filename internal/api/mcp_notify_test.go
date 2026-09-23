package api_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/notify"
)

// A backup the operator did not start is the one that needs explaining, so the
// notification about it and the failed run on the dashboard both name the key
// behind it.
func TestMCPOriginInNotificationsAndErrorPanelData(t *testing.T) {
	var mu sync.Mutex
	var messages []string
	hook := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var ev struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &ev)
		mu.Lock()
		messages = append(messages, ev.Message)
		mu.Unlock()
	}))
	defer hook.Close()
	lastMessage := func() string {
		mu.Lock()
		defer mu.Unlock()
		if len(messages) == 0 {
			t.Fatal("no notification was sent")
		}
		return messages[len(messages)-1]
	}

	eng := &fakeResticEngine{backupErr: errors.New("the repository did not answer")}
	docker := &fakeServiceDocker{inspect: model.Inspect{Name: "/plex", Image: "plex:latest", Running: true}}
	rig := newMCPStartRig(t, docker, eng)
	tg := rig.target(t, "plex")
	if err := rig.svc.SetNotifyConfig(notify.Config{
		On: "always", WebhookEnabled: true, WebhookURL: hook.URL, WebhookFormat: "generic",
	}); err != nil {
		t.Fatal(err)
	}

	if res := mcpCallTool(t, rig.h, rig.key, "start_backup", `{"domain":"containers","item":"plex"}`); res.IsError {
		t.Fatalf("start_backup: %v", res.Structured)
	}
	waitForBackupDone(t, rig.svc)

	const sentence = `Started through MCP with the key "Laptop".`
	if msg := lastMessage(); !strings.HasSuffix(msg, sentence) {
		t.Fatalf("the notification reads %q, want it to end in %q", msg, sentence)
	}

	w, body := doJSON(t, rig.h, http.MethodPost, "/api/containers/plex/backup", "")
	if w.Code != http.StatusOK || body["ok"] != true {
		t.Fatalf("start a backup in the web interface: status=%d body=%v", w.Code, body)
	}
	waitForBackupDone(t, rig.svc)

	if msg := lastMessage(); strings.Contains(msg, "MCP") {
		t.Fatalf("a backup started in the web interface reports an origin: %q", msg)
	}

	runs := runViewsOfTarget(t, rig, tg.ID)
	if len(runs) != 2 {
		t.Fatalf("%d runs of the container, want the two that failed", len(runs))
	}
	for _, run := range runs {
		want := ""
		if run["startedVia"] == "mcp" {
			want = "Laptop"
		}
		if label := run["startedViaLabel"]; label != want {
			t.Fatalf("a run started via %v reads startedViaLabel %v, want %q", run["startedVia"], label, want)
		}
	}
}

// runViewsOfTarget is what the dashboard reads for its error panel.
func runViewsOfTarget(t *testing.T, rig *mcpStartRig, targetID string) []map[string]any {
	t.Helper()
	w, body := doJSON(t, rig.h, http.MethodGet, "/api/runs", "")
	if w.Code != http.StatusOK || body["ok"] != true {
		t.Fatalf("list runs: status=%d body=%v", w.Code, body)
	}
	rows, _ := body["runs"].([]any)
	out := make([]map[string]any, 0, len(rows))
	for _, raw := range rows {
		row, _ := raw.(map[string]any)
		if row["targetId"] == targetID {
			out = append(out, row)
		}
	}
	return out
}
