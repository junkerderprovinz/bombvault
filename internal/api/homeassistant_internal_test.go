package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func putHomeAssistant(t *testing.T, router http.Handler, body string) map[string]any {
	t.Helper()
	r := httptest.NewRequest(http.MethodPut, "/api/homeassistant", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("PUT answered %d %s", w.Code, w.Body)
	}
	return out
}

func TestHomeAssistantSettingsKeepThePasswordToThemselves(t *testing.T) {
	_, router, repo, _ := newMCPGateHandler(t)

	out := putHomeAssistant(t, router, `{"enabled":false,"host":"10.0.0.2","port":1883,"username":"bv","password":"s3cret","tls":false,"prefix":"bombvault","buttons":true}`)
	if out["ok"] != true {
		t.Fatalf("save refused: %v", out)
	}
	view, _ := out["settings"].(map[string]any)
	if view["passwordSet"] != true || view["password"] != nil || view["nodeId"] == "" {
		t.Fatalf("view = %v, want the password set, not shown, and a node id", view)
	}
	stored, err := repo.GetMQTTSettings()
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.PasswordEnc) == 0 || strings.Contains(string(stored.PasswordEnc), "s3cret") {
		t.Fatalf("the password is stored as %q", stored.PasswordEnc)
	}
	node := stored.NodeID

	putHomeAssistant(t, router, `{"enabled":false,"host":"10.0.0.2","port":1883,"username":"bv","password":"","tls":false,"prefix":"bombvault","buttons":true}`)
	kept, _ := repo.GetMQTTSettings()
	if string(kept.PasswordEnc) != string(stored.PasswordEnc) || kept.NodeID != node {
		t.Fatal("a save without a password lost the stored one or the node id")
	}

	putHomeAssistant(t, router, `{"enabled":false,"host":"10.0.0.2","port":1883,"username":"","password":"","tls":false,"prefix":"bombvault","buttons":true}`)
	cleared, _ := repo.GetMQTTSettings()
	if len(cleared.PasswordEnc) != 0 {
		t.Fatal("clearing the user name kept the password")
	}
}

func TestHomeAssistantSettingsRefuseWhatNoBrokerTakes(t *testing.T) {
	_, router, _, _ := newMCPGateHandler(t)
	for body, code := range map[string]string{
		`{"enabled":true,"host":"","port":1883,"prefix":"bombvault","buttons":true}`:          "mqtt-host-invalid",
		`{"enabled":true,"host":"a b","port":1883,"prefix":"bombvault","buttons":true}`:       "mqtt-host-invalid",
		`{"enabled":true,"host":"broker","port":0,"prefix":"bombvault","buttons":true}`:       "mqtt-port-invalid",
		`{"enabled":true,"host":"broker","port":1883,"prefix":"bomb/#","buttons":true}`:       "mqtt-prefix-invalid",
		`{"enabled":true,"host":"broker","port":1883,"prefix":"/bombvault","buttons":true}`:   "mqtt-prefix-invalid",
		`{"enabled":true,"host":"broker","port":1883,"prefix":"bombvault/","buttons":true}`:   "mqtt-prefix-invalid",
		`{"enabled":true,"host":"broker","port":70000,"prefix":"bombvault","buttons":true}`:   "mqtt-port-invalid",
		`{"enabled":false,"host":"broker!","port":1883,"prefix":"bombvault","buttons":false}`: "mqtt-host-invalid",
	} {
		out := putHomeAssistant(t, router, body)
		if out["ok"] != false || out["code"] != code {
			t.Errorf("%s answered %v, want %s", body, out, code)
		}
	}
}

func TestAHomeAssistantButtonStartsUnderTheSharedLimits(t *testing.T) {
	h, repo, sets := newMCPStartHandler(t, "photos")
	if err := repo.SaveMQTTSettings(store.MQTTSettings{Port: 1883, Prefix: "bombvault", Buttons: false, NodeID: "a1b2c3d4"}); err != nil {
		t.Fatal(err)
	}
	if got := h.haStart(context.Background(), "files"); got != "not_permitted" {
		t.Fatalf("a press with buttons switched off answered %q", got)
	}

	if err := repo.SaveMQTTSettings(store.MQTTSettings{Port: 1883, Prefix: "bombvault", Buttons: true, NodeID: "a1b2c3d4"}); err != nil {
		t.Fatal(err)
	}
	if got := h.haStart(context.Background(), "files"); got != "started" {
		t.Fatalf("a press answered %q, want started", got)
	}
	var run *store.Run
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var err error
		run, err = repo.LastRunForTarget(sets["photos"].ID)
		if err != nil {
			t.Fatal(err)
		}
		if run != nil && run.Status != "running" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if run == nil || run.StartedVia != "mqtt" || run.StartedViaKey != "" {
		t.Fatalf("the run records %+v, want the mqtt origin without a key", run)
	}
	if got := h.haStart(context.Background(), "files"); got != "cooldown" {
		t.Fatalf("a second press right after answered %q, want cooldown", got)
	}

	st, err := h.haState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	files, ok := st.Domains["files"]
	if !ok || files.LastResult != run.Status {
		t.Fatalf("the files sensors read %+v, want the last result %q", files, run.Status)
	}
	if _, ok := st.Domains["vms"]; ok {
		t.Fatal("a switched-off domain is in the state")
	}
	if run.Status == "failed" && st.Status != "failed" {
		t.Fatalf("overall status %q after a failed backup", st.Status)
	}
}
