package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/spike"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// newZFSTestRouter serves the ZFS routes over a seeded repository and no host,
// which is the state a user is in before the SSH connection works.
func newZFSTestRouter(t *testing.T, eng *fakeResticEngine) (http.Handler, *store.Repo) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ZFSEnabled = true
	s.ZFSPath = "backups/zfs"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(dir, "backups", "zfs")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	d := &fakeServiceDocker{}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)
	sched := schedule.New(func(string) error { return nil }, st.ListTargets)
	return api.NewHandler(cfg, st, d, svc, sched, spike.DefaultProbes()).Router(), st
}

func zfsRowsOf(t *testing.T, h http.Handler) []map[string]any {
	t.Helper()
	w, m := doJSON(t, h, http.MethodGet, "/api/zfs", "")
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("list failed: %d %v", w.Code, m)
	}
	raw, _ := m["datasets"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, r := range raw {
		row, _ := r.(map[string]any)
		out = append(out, row)
	}
	return out
}

func TestZFSRoutesRoundTrip(t *testing.T) {
	h, _ := newZFSTestRouter(t, &fakeResticEngine{})

	w, m := doJSON(t, h, http.MethodPost, "/api/zfs/datasets",
		`{"items":[{"dataset":"cache/appdata","excludedChildren":["cache/appdata/cachey"],"excludes":[],"stopContainers":[],"repo":"","enabled":true}]}`)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("create failed: %d %v", w.Code, m)
	}
	results, _ := m["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("create returned %v, want one verdict", m)
	}
	first, _ := results[0].(map[string]any)
	if first["code"] != "ok" {
		t.Fatalf("create verdict = %v, want ok", first)
	}
	id, _ := first["id"].(string)
	if id == "" {
		t.Fatalf("create returned no id: %v", first)
	}

	rows := zfsRowsOf(t, h)
	if len(rows) != 1 || rows[0]["dataset"] != "cache/appdata" || rows[0]["enabled"] != true {
		t.Fatalf("list = %v", rows)
	}

	w, m = doJSON(t, h, http.MethodPatch, "/api/zfs/datasets/"+id, `{"enabled":false,"excludes":["*.tmp"]}`)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("patch failed: %d %v", w.Code, m)
	}
	rows = zfsRowsOf(t, h)
	if rows[0]["enabled"] != false {
		t.Fatalf("patch did not switch the item off: %v", rows[0])
	}
	excludes, _ := rows[0]["excludes"].([]any)
	if len(excludes) != 1 || excludes[0] != "*.tmp" {
		t.Fatalf("excludes did not round-trip: %v", rows[0]["excludes"])
	}

	w, m = doJSON(t, h, http.MethodGet, "/api/zfs/connection", "")
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("connection failed: %d %v", w.Code, m)
	}
	if m["code"] != "ssh-missing" {
		t.Fatalf("connection without a host = %v, want ssh-missing", m["code"])
	}

	w, m = doJSON(t, h, http.MethodGet, "/api/zfs/host", "")
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("host listing failed: %d %v", w.Code, m)
	}
	if m["available"] != false {
		t.Fatalf("host listing without a host = %v, want unavailable", m)
	}

	w, m = doJSON(t, h, http.MethodGet, "/api/zfs/datasets/"+id+"/restore-points", "")
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("restore points failed: %d %v", w.Code, m)
	}
	if _, ok := m["points"]; !ok {
		t.Fatalf("restore points response has no points: %v", m)
	}

	w, m = doJSON(t, h, http.MethodDelete, "/api/zfs/datasets/"+id, "")
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("delete failed: %d %v", w.Code, m)
	}
	if left := zfsRowsOf(t, h); len(left) != 0 {
		t.Fatalf("expected no items after the delete, got %v", left)
	}
}

func TestZFSRoutesRefuseAnInvalidID(t *testing.T) {
	h, _ := newZFSTestRouter(t, &fakeResticEngine{})
	for _, path := range []string{
		"/api/zfs/datasets/../../etc",
		"/api/zfs/datasets/not%20an%20id/safety-snapshots",
	} {
		w, _ := doJSON(t, h, http.MethodGet, path, "")
		if w.Code == http.StatusOK {
			t.Fatalf("%s answered 200, want a refusal", path)
		}
	}
}

func TestSettingsRoundTripZFSFields(t *testing.T) {
	h, _ := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})

	body := `{
		"containersPath": "backups/c",
		"vmsPath": "backups/v",
		"flashPath": "backups/f",
		"containersSchedule": "off",
		"vmsSchedule": "off",
		"flashSchedule": "off",
		"zfsEnabled": true,
		"zfsPath": "backups/zfs",
		"zfsSchedule": "daily 03:00",
		"zfsOffsite": "rest:https://box:8000/zfs",
		"zfsOffsiteSchedule": "weekly SUN 02:00",
		"zfsOffsiteImmutable": true
	}`
	w, m := doJSON(t, h, http.MethodPut, "/api/settings", body)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("put status=%d body=%s", w.Code, w.Body.String())
	}
	w, m = doJSON(t, h, http.MethodGet, "/api/settings", "")
	if w.Code != http.StatusOK {
		t.Fatalf("get status=%d", w.Code)
	}
	settings, _ := m["settings"].(map[string]any)
	for key, want := range map[string]any{
		"zfsEnabled":          true,
		"zfsPath":             "backups/zfs",
		"zfsSchedule":         "daily 03:00",
		"zfsOffsite":          "rest:https://box:8000/zfs",
		"zfsOffsiteSchedule":  "weekly SUN 02:00",
		"zfsOffsiteImmutable": true,
	} {
		if settings[key] != want {
			t.Errorf("%s = %v, want %v", key, settings[key], want)
		}
	}
}

func TestRunsAttributesZFSDatasetRuns(t *testing.T) {
	h, st := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
	item, err := st.CreateZFSDataset(store.ZFSDataset{Dataset: "cache/appdata", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	runID, err := st.StartRun(item.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(runID, "success", "deadbeef12345678", 1024, ""); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/runs", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Runs []struct {
			Target string `json:"target"`
			Domain string `json:"domain"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	if len(resp.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(resp.Runs))
	}
	if resp.Runs[0].Target != "cache/appdata" || resp.Runs[0].Domain != "zfs" {
		t.Fatalf("zfs run not attributed: target=%q domain=%q", resp.Runs[0].Target, resp.Runs[0].Domain)
	}
}

// Every domain-scoped route answers for zfs. Each one is asked whether the
// domain exists, not whether the operation succeeds without a repository.
func TestDomainRoutesAcceptZFS(t *testing.T) {
	h, _ := newZFSTestRouter(t, &fakeResticEngine{})

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/check/zfs", ""},
		{http.MethodPost, "/api/verify/zfs?source=local&kind=subset", ""},
		{http.MethodGet, "/api/verify?domain=zfs", ""},
		{http.MethodPost, "/api/unlock/zfs", ""},
		{http.MethodPost, "/api/prune/zfs", ""},
		{http.MethodGet, "/api/retention/preview/zfs", ""},
		{http.MethodDelete, "/api/snapshots/zfs/deadbeef12345678", ""},
		{http.MethodPost, "/api/offsite/zfs", ""},
		{http.MethodPost, "/api/offsite/zfs/test", ""},
		{http.MethodGet, "/api/offsite/zfs/deploy-snippet", ""},
		{http.MethodPost, "/api/offsite/zfs/tamper-test", ""},
		{http.MethodGet, "/api/stats?domain=zfs", ""},
	}
	for _, c := range cases {
		w, m := doJSON(t, h, c.method, c.path, c.body)
		if w.Code == http.StatusBadRequest {
			t.Errorf("%s %s = 400 %v, want the domain to be known", c.method, c.path, m)
			continue
		}
		if err, _ := m["error"].(string); strings.Contains(err, "unknown domain") {
			t.Errorf("%s %s = %q", c.method, c.path, err)
		}
	}
}
