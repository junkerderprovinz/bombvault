package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// createAPIToken makes a token through the settings route and returns the
// secret, shown this once, and its id.
func createAPIToken(t *testing.T, h http.Handler, label string, canStart bool) (token, id string) {
	t.Helper()
	body := `{"label":"` + label + `","canStartBackups":false}`
	if canStart {
		body = `{"label":"` + label + `","canStartBackups":true}`
	}
	w, m := doMCPKey(t, h, http.MethodPost, "/api/tokens", body)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("create token %q: status=%d body=%v", label, w.Code, m)
	}
	token, _ = m["key"].(string)
	item, _ := m["item"].(map[string]any)
	id, _ = item["id"].(string)
	if !strings.HasPrefix(token, secret.APITokenPrefix) || id == "" {
		t.Fatalf("create token %q: got %v", label, m)
	}
	return token, id
}

func apiV1Call(t *testing.T, h http.Handler, method, path, token, body string) (int, map[string]any) {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	r.Header.Set("X-API-Key", token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var m map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	return w.Code, m
}

func TestAPITokenStartsABackupRecordedUnderItsName(t *testing.T) {
	docker := &fakeServiceDocker{inspect: model.Inspect{Name: "/plex", Image: "plex:latest", Running: true}}
	h, st, svc, dir := newTestRouterSvcDir(t, docker, &fakeResticEngine{})
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersEnabled = true
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	establishLocalRepo(t, dir, s.ContainersPath)
	tg, err := st.UpsertTarget(store.Target{ContainerName: "plex", IncludeInSchedule: true})
	if err != nil {
		t.Fatal(err)
	}
	token, id := createAPIToken(t, h, "Home Assistant", true)

	code, body := apiV1Call(t, h, http.MethodPost, "/api/v1/backups", token, `{"domain":"containers","item":"plex"}`)
	if code != http.StatusOK || body["started"] != true {
		t.Fatalf("start answered %d %v", code, body)
	}
	if _, ok := body["followUp"]; ok {
		t.Fatalf("the answer carries the assistant's follow-up sentence: %v", body)
	}
	waitForBackupDone(t, svc)

	run, err := st.LastRunForTarget(tg.ID)
	if err != nil || run == nil {
		t.Fatalf("no run recorded: %v", err)
	}
	if run.StartedVia != "api" || run.StartedViaKey != id {
		t.Fatalf("the run records %q/%q, want api/%s", run.StartedVia, run.StartedViaKey, id)
	}

	code, body = apiV1Call(t, h, http.MethodGet, "/api/v1/runs?domain=containers&item=plex", token, "")
	if code != http.StatusOK {
		t.Fatalf("runs answered %d %v", code, body)
	}
	runs, _ := body["runs"].([]any)
	if len(runs) != 1 {
		t.Fatalf("runs = %v, want the one backup", runs)
	}
	row, _ := runs[0].(map[string]any)
	if row["startedVia"] != "api" || row["startedViaLabel"] != "Home Assistant" {
		t.Fatalf("the run row names %v/%v, want api and the token's name", row["startedVia"], row["startedViaLabel"])
	}

	// The same item again at once is held back for everyone outside the web
	// interface, not only for this token.
	code, body = apiV1Call(t, h, http.MethodPost, "/api/v1/backups", token, `{"domain":"containers","item":"plex"}`)
	if code != http.StatusTooManyRequests {
		t.Fatalf("a second start answered %d %v, want 429", code, body)
	}
	mcpKey, _ := createMCPKey(t, h, "Laptop", true)
	if res := mcpCallTool(t, h, mcpKey, "start_backup", `{"domain":"containers","item":"plex"}`); !res.IsError {
		t.Fatalf("an MCP start right after an API start went through: %v", res.Structured)
	}
}

func TestReplacingAnAPITokenKeepsItsPrefix(t *testing.T) {
	h, _, _, _ := newTestRouterSvcDir(t, &fakeServiceDocker{}, &fakeResticEngine{})
	old, id := createAPIToken(t, h, "Dashboard", false)
	w, m := doMCPKey(t, h, http.MethodPost, "/api/tokens/"+id+"/rotate", "")
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("rotate: status=%d body=%v", w.Code, m)
	}
	fresh, _ := m["key"].(string)
	if !strings.HasPrefix(fresh, secret.APITokenPrefix) || fresh == old {
		t.Fatalf("the replacement is %q, want a new token with the %q prefix", fresh, secret.APITokenPrefix)
	}
	if code, _ := apiV1Call(t, h, http.MethodGet, "/api/v1/health", old, ""); code != http.StatusUnauthorized {
		t.Fatalf("the replaced token still answers %d", code)
	}
	if code, _ := apiV1Call(t, h, http.MethodGet, "/api/v1/health", fresh, ""); code != http.StatusOK {
		t.Fatalf("the new token answers %d", code)
	}
}
