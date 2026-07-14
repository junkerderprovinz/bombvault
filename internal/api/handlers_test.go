package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/spike"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// newTestRouter wires a handler over fakes and returns the http.Handler plus the
// underlying fakes for assertions.
func newTestRouter(t *testing.T, d *fakeServiceDocker, eng *fakeResticEngine) (http.Handler, *store.Repo) {
	t.Helper()
	h, st, _ := newTestRouterSvc(t, d, eng)
	return h, st
}

// newTestRouterSvc is newTestRouter that also returns the service, so backup
// tests can wait for the now-async backup goroutine to fully finish (the work is
// detached, so without waiting it can outlive the test and touch a closed store).
func newTestRouterSvc(t *testing.T, d *fakeServiceDocker, eng *fakeResticEngine) (http.Handler, *store.Repo, *api.Service) {
	t.Helper()
	dir := t.TempDir()
	// The conventional appdata dir for the "plex" container the backup tests use,
	// so the (now existence-filtered) backup actually has a source to snapshot.
	if err := os.MkdirAll(filepath.Join(dir, "appdata", "plex"), 0o750); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)
	sched := schedule.New(
		func(string) error { return nil },
		st.ListTargets,
	)
	h := api.NewHandler(cfg, st, d, svc, sched, spike.DefaultProbes())
	return h.Router(), st, svc
}

// waitForBackupDone blocks until the detached single-backup/batch goroutine has
// released the shared guard, i.e. all of its work (run record + best-effort
// retention/stats) is done. This keeps async-backup tests from racing cleanup.
func waitForBackupDone(t *testing.T, svc *api.Service) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !svc.BackupInProgress() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the async backup goroutine to finish")
}

func doJSON(t *testing.T, h http.Handler, method, path, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var m map[string]any
	if w.Body.Len() > 0 {
		_ = json.Unmarshal(w.Body.Bytes(), &m)
	}
	return w, m
}

func TestHealth(t *testing.T) {
	h, _ := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
	w, m := doJSON(t, h, http.MethodGet, "/api/health", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if m["ok"] != true {
		t.Fatalf("health body = %v", m)
	}
}

func TestListContainers(t *testing.T) {
	d := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{
		{ID: "abc", Name: "plex", Image: "plex:latest", State: "running", Status: "Up 2h"},
	}}
	h, st := newTestRouter(t, d, &fakeResticEngine{})
	// Mark plex as included to exercise the include flag.
	if _, err := st.UpsertTarget(store.Target{ContainerName: "plex", AppdataPaths: []string{"/x"}}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetInclude("plex", true); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/containers", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Containers []struct {
			Name    string `json:"name"`
			Image   string `json:"image"`
			Status  string `json:"status"`
			Include bool   `json:"includeInSchedule"`
		} `json:"containers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	if len(resp.Containers) != 1 || resp.Containers[0].Name != "plex" {
		t.Fatalf("containers = %+v", resp.Containers)
	}
	if !resp.Containers[0].Include {
		t.Fatalf("expected include flag true: %+v", resp.Containers[0])
	}
}

// waitForBackupRun polls the runs store until a backup run reaches a terminal
// (success/failed) state, then returns it. Single backups are now ASYNC: the
// handler returns immediately and the work runs in a detached goroutine, so a
// test must wait for the recorded run before reading the outcome — and before
// the in-memory store closes on cleanup.
func waitForBackupRun(t *testing.T, st *store.Repo) store.Run {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		runs, err := st.ListRuns(10)
		if err != nil {
			t.Fatalf("ListRuns: %v", err)
		}
		for _, r := range runs {
			if r.Kind == "backup" && (r.Status == "success" || r.Status == "failed") {
				return r
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the async backup run to finish")
	return store.Run{}
}

func TestBackupOK(t *testing.T) {
	d := &fakeServiceDocker{inspect: model.Inspect{Name: "/plex", Image: "plex:latest"}}
	h, st, svc := newTestRouterSvc(t, d, &fakeResticEngine{})
	// The single backup is now ASYNC: the handler only acknowledges acceptance.
	w, m := doJSON(t, h, http.MethodPost, "/api/containers/plex/backup", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if m["ok"] != true || m["started"] != true {
		t.Fatalf("expected ok:true, started:true, got %v", m)
	}
	// The outcome is recorded on the run, not returned synchronously.
	run := waitForBackupRun(t, st)
	if run.Status != "success" {
		t.Fatalf("expected a successful run, got %q (%s)", run.Status, run.Error)
	}
	if run.SnapshotID != "deadbeef12345678" {
		t.Fatalf("expected snapshot id on the run, got %q", run.SnapshotID)
	}
	waitForBackupDone(t, svc)
}

func TestBackupFailureGraceful(t *testing.T) {
	d := &fakeServiceDocker{inspect: model.Inspect{Name: "/plex", Image: "plex:latest"}}
	eng := &fakeResticEngine{backupErr: errors.New("restic backup failed: /secret/repo/path")}
	h, st, svc := newTestRouterSvc(t, d, eng)
	// Async: the handler accepts the job (200, started:true) and the failure is
	// recorded on the run rather than returned in the response.
	w, m := doJSON(t, h, http.MethodPost, "/api/containers/plex/backup", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected graceful 200, got %d", w.Code)
	}
	if m["ok"] != true || m["started"] != true {
		t.Fatalf("expected the job to be accepted (ok:true, started:true), got %v", m)
	}
	run := waitForBackupRun(t, st)
	if run.Status != "failed" {
		t.Fatalf("expected a failed run, got %q", run.Status)
	}
	if run.Error == "" {
		t.Fatalf("expected an error message on the run, got %+v", run)
	}
	waitForBackupDone(t, svc)
}

func TestSnapshots(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ContainersPath = "backups/c"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	// Initialise the repo marker so Snapshots calls the engine.
	repo := filepath.Join(dir, "backups", "c")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	d := &fakeServiceDocker{}
	// Two snapshots in the shared repo; only the plex-tagged one must come back.
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "deadbeef12345678", Time: "2026-06-09T00:00:00Z", Tags: []string{"container:plex", "p1"}},
		{ID: "cafebabe87654321", Time: "2026-06-09T00:00:00Z", Tags: []string{"container:sonarr", "p1"}},
	}}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)
	sched := schedule.New(func(string) error { return nil }, st.ListTargets)
	h := api.NewHandler(cfg, st, d, svc, sched, spike.DefaultProbes()).Router()

	w, m := doJSON(t, h, http.MethodGet, "/api/containers/plex/snapshots", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if m["ok"] != true {
		t.Fatalf("expected ok, got %v", m)
	}
	snaps, _ := m["snapshots"].([]any)
	if len(snaps) != 1 {
		t.Fatalf("expected 1 plex snapshot (filtered by tag), got %v", m)
	}
}

func TestRestoreNotConfirmed(t *testing.T) {
	d := &fakeServiceDocker{liveName: "/plex"}
	h, _ := newTestRouter(t, d, &fakeResticEngine{})
	w, m := doJSON(t, h, http.MethodPost, "/api/containers/plex/restore",
		`{"snapshotId":"deadbeef12345678","confirm":false}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected graceful 200, got %d", w.Code)
	}
	if m["ok"] != false {
		t.Fatalf("expected ok:false, got %v", m)
	}
	errStr, _ := m["error"].(string)
	if !strings.Contains(strings.ToLower(errStr), "confirm") {
		t.Fatalf("expected a confirm message, got %q", errStr)
	}
}

// TestRestoreCancelUnknownKey pins the cancel endpoint's idempotent no-op:
// cancelling a key with no in-flight restore is a graceful success reporting
// cancelled:false (never an error), so the button is safe to click any time.
func TestRestoreCancelUnknownKey(t *testing.T) {
	h, _ := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
	w, m := doJSON(t, h, http.MethodPost, "/api/restore/cancel", `{"key":"container:plex"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if m["ok"] != true {
		t.Fatalf("expected ok:true, got %v", m)
	}
	if m["cancelled"] != false {
		t.Fatalf("cancelling an unknown key must report cancelled:false, got %v", m["cancelled"])
	}
}

func TestPatchInclude(t *testing.T) {
	d := &fakeServiceDocker{}
	h, st := newTestRouter(t, d, &fakeResticEngine{})
	if _, err := st.UpsertTarget(store.Target{ContainerName: "plex", AppdataPaths: []string{"/x"}}); err != nil {
		t.Fatal(err)
	}
	w, m := doJSON(t, h, http.MethodPatch, "/api/containers/plex",
		`{"includeInSchedule":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if m["ok"] != true {
		t.Fatalf("expected ok, got %v", m)
	}
	tg, err := st.GetTargetByContainer("plex")
	if err != nil {
		t.Fatal(err)
	}
	if !tg.IncludeInSchedule {
		t.Fatal("include flag not persisted")
	}
}

// TestPatchIncludeBeforeFirstBackup exercises the find-or-create path: the
// container toggle must succeed even when no target row exists yet.
func TestPatchIncludeBeforeFirstBackup(t *testing.T) {
	d := &fakeServiceDocker{inspect: model.Inspect{
		Name:  "/sonarr",
		Image: "sonarr:latest",
		Mounts: []model.Mount{
			{Type: "bind", Source: "/host/user/appdata/sonarr", Destination: "/config"},
		},
	}}
	h, st := newTestRouter(t, d, &fakeResticEngine{})

	// No target row exists yet — the PATCH must still succeed.
	w, m := doJSON(t, h, http.MethodPatch, "/api/containers/sonarr",
		`{"includeInSchedule":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if m["ok"] != true {
		t.Fatalf("expected ok (find-or-create path), got %v", m)
	}
	tg, err := st.GetTargetByContainer("sonarr")
	if err != nil {
		t.Fatalf("target must have been created: %v", err)
	}
	if !tg.IncludeInSchedule {
		t.Fatal("include flag not set on new target")
	}
}

// TestPatchExcludesAndPreview pins Task 4's API surface: a PATCH carrying
// "excludes" persists to the target and surfaces in the container list view, and
// the preview endpoint resolves a candidate list into one entry per non-empty
// line.
func TestPatchExcludesAndPreview(t *testing.T) {
	d := &fakeServiceDocker{
		listOut: []dockercli.ContainerInfo{
			{ID: "abc", Name: "plex", Image: "plex:latest", State: "running", Status: "Up 2h"},
		},
		inspect: model.Inspect{
			Name: "/plex",
			Mounts: []model.Mount{
				{Type: "bind", Source: "/mnt/user/appdata/plex", Destination: "/config"},
			},
		},
	}
	h, st := newTestRouter(t, d, &fakeResticEngine{})
	if _, err := st.UpsertTarget(store.Target{ContainerName: "plex", AppdataPaths: []string{"/x"}}); err != nil {
		t.Fatal(err)
	}

	// PATCH excludes → persisted on the target (trimmed, order preserved).
	w, m := doJSON(t, h, http.MethodPatch, "/api/containers/plex",
		`{"excludes":["/config/Cache",".git"]}`)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("patch status=%d body=%s", w.Code, w.Body.String())
	}
	tg, err := st.GetTargetByContainer("plex")
	if err != nil {
		t.Fatal(err)
	}
	if len(tg.Excludes) != 2 || tg.Excludes[0] != "/config/Cache" || tg.Excludes[1] != ".git" {
		t.Fatalf("excludes not persisted: %+v", tg.Excludes)
	}

	// The list view exposes the persisted excludes.
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/containers", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Containers []struct {
			Name     string   `json:"name"`
			Excludes []string `json:"excludes"`
		} `json:"containers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	if len(resp.Containers) != 1 || len(resp.Containers[0].Excludes) != 2 {
		t.Fatalf("list view excludes = %+v", resp.Containers)
	}

	// The preview endpoint returns one entry per non-empty candidate line.
	w, m = doJSON(t, h, http.MethodPost, "/api/containers/plex/excludes/preview",
		`{"patterns":["/config/Cache",".git","   "]}`)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("preview status=%d body=%s", w.Code, w.Body.String())
	}
	preview, ok := m["preview"].([]any)
	if !ok || len(preview) != 2 {
		t.Fatalf("expected 2 preview entries (blank dropped), got %v", m["preview"])
	}
	first, _ := preview[0].(map[string]any)
	if first["raw"] != "/config/Cache" || first["status"] == nil {
		t.Fatalf("unexpected first preview entry: %v", preview[0])
	}
}

func TestSettingsGetPut(t *testing.T) {
	d := &fakeServiceDocker{}
	h, _ := newTestRouter(t, d, &fakeResticEngine{})

	// GET defaults — settings are nested under "settings" (symmetric with PUT).
	w, m := doJSON(t, h, http.MethodGet, "/api/settings", "")
	if w.Code != http.StatusOK {
		t.Fatalf("get status = %d", w.Code)
	}
	settings, ok := m["settings"].(map[string]any)
	if !ok {
		t.Fatalf("settings missing or not nested: %v", m)
	}
	if settings["containersPath"] == nil {
		t.Fatalf("settings missing containersPath: %v", settings)
	}

	// PUT a valid update.
	body := `{
		"encryptionEnabled": false,
		"containersEnabled": true,
		"containersPath": "backups/c",
		"vmsPath": "backups/v",
		"flashPath": "backups/f",
		"containersSchedule": "daily 02:30",
		"vmsSchedule": "off",
		"flashSchedule": "off"
	}`
	w, m = doJSON(t, h, http.MethodPut, "/api/settings", body)
	if w.Code != http.StatusOK {
		t.Fatalf("put status = %d body=%s", w.Code, w.Body.String())
	}
	if m["ok"] != true {
		t.Fatalf("expected ok, got %v", m)
	}
}

// TestSettingsPutImmutableRetentionWarning pins the warnings extension of the
// PUT /api/settings envelope: saving an immutable off-site flag together with
// an off-site retention policy succeeds (ok:true, backward compatible) but
// carries a "warnings" array — BombVault will not prune an append-only repo,
// so the policy is inert until enforced far-side. Without the conflict the
// response has no warnings.
func TestSettingsPutImmutableRetentionWarning(t *testing.T) {
	d := &fakeServiceDocker{}
	h, _ := newTestRouter(t, d, &fakeResticEngine{})

	// Immutable flag + off-site retention set → ok with a warning.
	body := `{
		"containersPath": "backups/c",
		"vmsPath": "backups/v",
		"flashPath": "backups/f",
		"containersSchedule": "off",
		"vmsSchedule": "off",
		"flashSchedule": "off",
		"containersOffsite": "rest:http://192.168.1.2:8000/containers",
		"containersOffsiteImmutable": true,
		"offsiteRetentionKeepDaily": 14
	}`
	w, m := doJSON(t, h, http.MethodPut, "/api/settings", body)
	if w.Code != http.StatusOK {
		t.Fatalf("put status = %d body=%s", w.Code, w.Body.String())
	}
	if m["ok"] != true {
		t.Fatalf("immutable + retention must still save (warn, not fail), got %v", m)
	}
	warnings, ok := m["warnings"].([]any)
	if !ok || len(warnings) == 0 {
		t.Fatalf("expected a non-empty warnings array, got %v", m)
	}
	if s, _ := warnings[0].(string); !strings.Contains(s, "append-only") {
		t.Fatalf("warning must explain the append-only conflict, got %q", warnings[0])
	}

	// Immutable without off-site retention → plain ok, no warnings key.
	body = `{
		"containersPath": "backups/c",
		"vmsPath": "backups/v",
		"flashPath": "backups/f",
		"containersSchedule": "off",
		"vmsSchedule": "off",
		"flashSchedule": "off",
		"containersOffsite": "rest:http://192.168.1.2:8000/containers",
		"containersOffsiteImmutable": true
	}`
	w, m = doJSON(t, h, http.MethodPut, "/api/settings", body)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("put status = %d body=%s", w.Code, w.Body.String())
	}
	if _, present := m["warnings"]; present {
		t.Fatalf("no warnings expected without an off-site retention policy, got %v", m)
	}
}

func TestSettingsPutRejectsBadCadence(t *testing.T) {
	d := &fakeServiceDocker{}
	h, _ := newTestRouter(t, d, &fakeResticEngine{})
	body := `{"containersPath":"backups/c","vmsPath":"backups/v","flashPath":"backups/f",
		"containersSchedule":"daily 99:99","vmsSchedule":"off","flashSchedule":"off"}`
	w, m := doJSON(t, h, http.MethodPut, "/api/settings", body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected graceful 200, got %d", w.Code)
	}
	if m["ok"] != false {
		t.Fatalf("expected ok:false for bad cadence, got %v", m)
	}
}

func TestSettingsPutRejectsTraversalPath(t *testing.T) {
	d := &fakeServiceDocker{}
	h, _ := newTestRouter(t, d, &fakeResticEngine{})
	body := `{"containersPath":"../../etc","vmsPath":"backups/v","flashPath":"backups/f",
		"containersSchedule":"off","vmsSchedule":"off","flashSchedule":"off"}`
	w, m := doJSON(t, h, http.MethodPut, "/api/settings", body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected graceful 200, got %d", w.Code)
	}
	if m["ok"] != false {
		t.Fatalf("expected ok:false for traversal path, got %v", m)
	}
}

// TestSettingsConfigFieldsRoundTrip pins the settings DTO's config self-backup
// fields: a PUT carrying every config* field is persisted and comes back verbatim
// on the following GET (the JSON DTO round-trips them both directions, like flash).
func TestSettingsConfigFieldsRoundTrip(t *testing.T) {
	d := &fakeServiceDocker{}
	h, _ := newTestRouter(t, d, &fakeResticEngine{})

	body := `{
		"containersPath": "backups/c",
		"vmsPath": "backups/v",
		"flashPath": "backups/f",
		"containersSchedule": "off",
		"vmsSchedule": "off",
		"flashSchedule": "off",
		"configEnabled": true,
		"configPath": "backups/config",
		"configSchedule": "daily 03:30",
		"configOffsite": "rest:http://192.168.1.2:8000/config",
		"configOffsiteSchedule": "weekly Sun 04:00",
		"configOffsiteImmutable": true
	}`
	w, m := doJSON(t, h, http.MethodPut, "/api/settings", body)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("put status=%d body=%s", w.Code, w.Body.String())
	}

	w, m = doJSON(t, h, http.MethodGet, "/api/settings", "")
	if w.Code != http.StatusOK {
		t.Fatalf("get status=%d", w.Code)
	}
	settings, ok := m["settings"].(map[string]any)
	if !ok {
		t.Fatalf("settings missing or not nested: %v", m)
	}
	for k, want := range map[string]any{
		"configEnabled":          true,
		"configPath":             "backups/config",
		"configSchedule":         "daily 03:30",
		"configOffsite":          "rest:http://192.168.1.2:8000/config",
		"configOffsiteSchedule":  "weekly Sun 04:00",
		"configOffsiteImmutable": true,
	} {
		if settings[k] != want {
			t.Fatalf("%s not round-tripped: got %v, want %v", k, settings[k], want)
		}
	}
}

// TestSettingsFilesFieldsRoundTrip pins the settings DTO's files-domain fields:
// a PUT carrying every files* field is persisted and comes back verbatim on the
// following GET (the JSON DTO round-trips them both directions, like config).
func TestSettingsFilesFieldsRoundTrip(t *testing.T) {
	d := &fakeServiceDocker{}
	h, _ := newTestRouter(t, d, &fakeResticEngine{})

	body := `{
		"containersPath": "backups/c",
		"vmsPath": "backups/v",
		"flashPath": "backups/f",
		"containersSchedule": "off",
		"vmsSchedule": "off",
		"flashSchedule": "off",
		"filesEnabled": true,
		"filesPath": "backups/files",
		"filesSchedule": "daily 03:00",
		"filesOffsite": "rest:http://192.168.1.2:8000/files",
		"filesOffsiteSchedule": "weekly Sun 05:00",
		"filesOffsiteImmutable": true
	}`
	w, m := doJSON(t, h, http.MethodPut, "/api/settings", body)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("put status=%d body=%s", w.Code, w.Body.String())
	}

	w, m = doJSON(t, h, http.MethodGet, "/api/settings", "")
	if w.Code != http.StatusOK {
		t.Fatalf("get status=%d", w.Code)
	}
	settings, ok := m["settings"].(map[string]any)
	if !ok {
		t.Fatalf("settings missing or not nested: %v", m)
	}
	for k, want := range map[string]any{
		"filesEnabled":          true,
		"filesPath":             "backups/files",
		"filesSchedule":         "daily 03:00",
		"filesOffsite":          "rest:http://192.168.1.2:8000/files",
		"filesOffsiteSchedule":  "weekly Sun 05:00",
		"filesOffsiteImmutable": true,
	} {
		if settings[k] != want {
			t.Fatalf("%s not round-tripped: got %v, want %v", k, settings[k], want)
		}
	}
}

// TestSettingsFilesImmutableRetentionWarning pins that the immutable-vs-offsite-
// retention warning also fires when ONLY the files domain is flagged append-only
// (the warning condition must include FilesOffsiteImmutable).
func TestSettingsFilesImmutableRetentionWarning(t *testing.T) {
	d := &fakeServiceDocker{}
	h, _ := newTestRouter(t, d, &fakeResticEngine{})

	body := `{
		"containersPath": "backups/c",
		"vmsPath": "backups/v",
		"flashPath": "backups/f",
		"containersSchedule": "off",
		"vmsSchedule": "off",
		"flashSchedule": "off",
		"filesOffsite": "rest:http://192.168.1.2:8000/files",
		"filesOffsiteImmutable": true,
		"offsiteRetentionKeepDaily": 14
	}`
	w, m := doJSON(t, h, http.MethodPut, "/api/settings", body)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("put status=%d body=%s", w.Code, w.Body.String())
	}
	warnings, ok := m["warnings"].([]any)
	if !ok || len(warnings) == 0 {
		t.Fatalf("expected the append-only warning for a files-only immutable flag, got %v", m)
	}
}

// TestCheckFilesDomainAccepted pins that POST /api/check/files reaches the
// service (the domain switch no longer 400s "unknown domain"): with no files
// repo initialised yet it reports the friendly not-yet error instead.
func TestCheckFilesDomainAccepted(t *testing.T) {
	h, _ := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
	w, m := doJSON(t, h, http.MethodPost, "/api/check/files", "")
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/check/files must not 400 anymore, got %d body=%s", w.Code, w.Body.String())
	}
	errMsg, _ := m["error"].(string)
	if strings.Contains(errMsg, "unknown domain") {
		t.Fatalf("files must be an accepted check domain, got error %q", errMsg)
	}
	if m["ok"] != false || !strings.Contains(errMsg, "no backups to verify yet") {
		t.Fatalf("expected the friendly no-repo-yet error, got %v", m)
	}
}

// TestRunsAttributesFileSetRuns pins /api/runs' target_id resolution for the
// files domain: a run recorded against a file set's id comes back with the
// set's name and domain "files" (the dashboard run history shows WHICH set).
func TestRunsAttributesFileSetRuns(t *testing.T) {
	h, st := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
	fs, err := st.CreateFileSet(store.FileSet{Name: "docs", Path: "data/docs"})
	if err != nil {
		t.Fatal(err)
	}
	runID, err := st.StartRun(fs.ID, "backup")
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
	if resp.Runs[0].Target != "docs" || resp.Runs[0].Domain != "files" {
		t.Fatalf("file-set run not attributed: target=%q domain=%q, want docs/files",
			resp.Runs[0].Target, resp.Runs[0].Domain)
	}
}

// TestSettingsPruneImageAfterUpdateRoundTrip guards the /api/settings wire boundary
// for the #56 image-prune toggle: the strict decoder rejects unknown fields, so the
// field must be in settingsView (PUT) and mapped by toView (GET). Regression test for
// the DTO omission that made the Save button always error and the toggle unreachable.
func TestSettingsPruneImageAfterUpdateRoundTrip(t *testing.T) {
	d := &fakeServiceDocker{}
	h, _ := newTestRouter(t, d, &fakeResticEngine{})

	body := `{
		"containersPath": "backups/c",
		"vmsPath": "backups/v",
		"flashPath": "backups/f",
		"containersSchedule": "off",
		"vmsSchedule": "off",
		"flashSchedule": "off",
		"pruneImageAfterUpdate": true
	}`
	w, m := doJSON(t, h, http.MethodPut, "/api/settings", body)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("put status=%d body=%s", w.Code, w.Body.String())
	}
	w, m = doJSON(t, h, http.MethodGet, "/api/settings", "")
	if w.Code != http.StatusOK {
		t.Fatalf("get status=%d", w.Code)
	}
	settings, ok := m["settings"].(map[string]any)
	if !ok {
		t.Fatalf("settings missing or not nested: %v", m)
	}
	if settings["pruneImageAfterUpdate"] != true {
		t.Fatalf("pruneImageAfterUpdate not round-tripped: got %v", settings["pruneImageAfterUpdate"])
	}
}

// TestRestoreConfigHandlerStagesAndAutoRestarts drives POST /api/config/restore
// end-to-end over the real service + fakes: the restore is staged (staged:true)
// and, because the self container name resolves, the response reports an
// auto-restart was scheduled (autoRestart:true — the SPA then waits for the app
// to come back rather than telling the user to restart manually).
func TestRestoreConfigHandlerStagesAndAutoRestarts(t *testing.T) {
	t.Setenv("BOMBVAULT_SELF_CONTAINER", "") // ignore any ambient override; resolve via docker.Self
	d := &fakeServiceDocker{selfName: "BombVault"}
	eng := &fakeResticEngine{snaps: []restic.Snapshot{{ID: "aaaa1111bbbb2222"}}}
	h, st, _ := newTestRouterSvc(t, d, eng)

	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ConfigEnabled = true
	s.ConfigPath = "backups/config"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	w, m := doJSON(t, h, http.MethodPost, "/api/config/restore", `{"source":"local","snapshot":"latest"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if m["ok"] != true || m["staged"] != true {
		t.Fatalf("expected ok:true, staged:true, got %v", m)
	}
	if m["autoRestart"] != true {
		t.Fatalf("expected autoRestart:true when the self container is known, got %v", m["autoRestart"])
	}
}

// TestRestoreConfigHandlerManualRestartWhenSelfUnknown pins the fallback: when the
// own-container name can't be resolved (Docker unreachable / not in a container),
// the restore is still staged but autoRestart:false — the SPA then instructs the
// user to restart the BombVault container manually to apply the restore.
func TestRestoreConfigHandlerManualRestartWhenSelfUnknown(t *testing.T) {
	t.Setenv("BOMBVAULT_SELF_CONTAINER", "") // no ambient override; docker.Self returns "" below
	d := &fakeServiceDocker{selfName: ""}
	eng := &fakeResticEngine{snaps: []restic.Snapshot{{ID: "aaaa1111bbbb2222"}}}
	h, st, _ := newTestRouterSvc(t, d, eng)

	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ConfigPath = "backups/config"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	w, m := doJSON(t, h, http.MethodPost, "/api/config/restore", `{"source":"local","snapshot":"latest"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if m["ok"] != true || m["staged"] != true {
		t.Fatalf("expected ok:true, staged:true, got %v", m)
	}
	if m["autoRestart"] != false {
		t.Fatalf("expected autoRestart:false when the self container is unknown, got %v", m["autoRestart"])
	}
}

func TestSpike(t *testing.T) {
	d := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{{Name: "plex"}}}
	// Inject a single stub probe so the test does not depend on a real restic.
	h := newSpikeRouter(t, d)
	w, m := doJSON(t, h, http.MethodPost, "/api/spike", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	checks, _ := m["checks"].([]any)
	if len(checks) == 0 {
		t.Fatalf("expected spike checks, got %v", m)
	}
}

// ---------------------------------------------------------------------------
// VM handler tests
// ---------------------------------------------------------------------------

func TestListVMsEmpty(t *testing.T) {
	h, _ := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
	w, m := doJSON(t, h, http.MethodGet, "/api/vms", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if m["ok"] != true {
		t.Fatalf("expected ok, got %v", m)
	}
	vms, ok := m["vms"].([]any)
	if !ok {
		t.Fatalf("vms field missing or wrong type: %v", m)
	}
	if len(vms) != 0 {
		t.Fatalf("expected empty vms slice, got %v", vms)
	}
}

func TestListVMsWithEntry(t *testing.T) {
	h, st := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
	_, err := st.UpsertVMTarget(store.VMTarget{
		Name:   "win11",
		Method: "graceful",
	})
	if err != nil {
		t.Fatal(err)
	}

	w, m := doJSON(t, h, http.MethodGet, "/api/vms", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	vms, ok := m["vms"].([]any)
	if !ok || len(vms) != 1 {
		t.Fatalf("expected 1 vm, got %v", m)
	}
	vm := vms[0].(map[string]any)
	if vm["name"] != "win11" {
		t.Fatalf("unexpected vm name: %v", vm)
	}
}

func TestBackupVMHandlerReturnsOK(t *testing.T) {
	h, _, svc := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})
	// The VM backup is now ASYNC: the handler accepts the job and returns
	// immediately; the actual backup (which fails in the background here, since
	// the fake VM has no disks) runs detached.
	w, m := doJSON(t, h, http.MethodPost, "/api/vms/win11/backup", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if m["ok"] != true || m["started"] != true {
		t.Fatalf("expected the job to be accepted (ok:true, started:true), got %v", m)
	}
	// Wait for the detached goroutine so it can't outlive the test and touch a
	// closed store.
	waitForBackupDone(t, svc)
}

func TestSnapshotsVMHandlerEmpty(t *testing.T) {
	h, _ := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
	// vmsPath not configured → expect ok:false error, not a panic.
	w, m := doJSON(t, h, http.MethodGet, "/api/vms/win11/snapshots", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if m["ok"] == nil {
		t.Fatalf("missing ok field: %v", m)
	}
}

func TestRestoreVMHandlerRequiresConfirm(t *testing.T) {
	h, _ := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
	w, m := doJSON(t, h, http.MethodPost, "/api/vms/win11/restore",
		`{"snapshotId":"abc123def456","confirm":false}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if m["ok"] != false {
		t.Fatalf("expected ok:false (not confirmed), got %v", m)
	}
}

func TestPatchVMHandlerMethod(t *testing.T) {
	h, st := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
	_, err := st.UpsertVMTarget(store.VMTarget{Name: "win11", Method: "graceful"})
	if err != nil {
		t.Fatal(err)
	}
	w, m := doJSON(t, h, http.MethodPatch, "/api/vms/win11",
		`{"method":"graceful"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if m["ok"] != true {
		t.Fatalf("expected ok, got %v", m)
	}
}

func TestPatchVMHandlerInclude(t *testing.T) {
	h, st := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
	_, err := st.UpsertVMTarget(store.VMTarget{Name: "ubuntu", Method: "graceful"})
	if err != nil {
		t.Fatal(err)
	}
	w, m := doJSON(t, h, http.MethodPatch, "/api/vms/ubuntu",
		`{"includeInSchedule":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if m["ok"] != true {
		t.Fatalf("expected ok, got %v", m)
	}
	tg, err := st.GetVMTargetByName("ubuntu")
	if err != nil {
		t.Fatal(err)
	}
	if !tg.IncludeInSchedule {
		t.Fatal("include flag not persisted")
	}
}

func newSpikeRouter(t *testing.T, d *fakeServiceDocker) http.Handler {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	svc := api.NewService(cfg, st, d, fakeVirsh{}, &fakeResticEngine{})
	sched := schedule.New(func(string) error { return nil }, st.ListTargets)
	probes := []spike.Probe{
		{Name: "stub", Fn: func(spike.Deps) (string, error) { return "ok", nil }},
	}
	h := api.NewHandler(cfg, st, d, svc, sched, probes)
	return h.Router()
}

func TestRuns(t *testing.T) {
	d := &fakeServiceDocker{inspect: model.Inspect{Name: "/plex", Image: "plex:latest"}}
	h, st, svc := newTestRouterSvc(t, d, &fakeResticEngine{})
	// Drive one backup so a run exists. It's async now, so wait for it to record.
	doJSON(t, h, http.MethodPost, "/api/containers/plex/backup", "")
	waitForBackupRun(t, st)
	waitForBackupDone(t, svc)
	w, m := doJSON(t, h, http.MethodGet, "/api/runs", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	runs, _ := m["runs"].([]any)
	if len(runs) == 0 {
		t.Fatalf("expected at least one run, got %v", m)
	}
}

// ---------------------------------------------------------------------------
// Browse handler tests
// ---------------------------------------------------------------------------

// newBrowseRouter builds a minimal router whose HostMountRoot points to
// the supplied temp dir, making it easy to test the browse handler in isolation.
func newBrowseRouter(t *testing.T, mountRoot string) http.Handler {
	t.Helper()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: t.TempDir(), HostMountRoot: mountRoot}
	st := newMemStore(t)
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, &fakeResticEngine{})
	sched := schedule.New(func(string) error { return nil }, st.ListTargets)
	h := api.NewHandler(cfg, st, &fakeServiceDocker{}, svc, sched, spike.DefaultProbes())
	return h.Router()
}

func TestBrowseListsMountRoot(t *testing.T) {
	root := t.TempDir()
	// Create some subdirectories in the mount root.
	for _, d := range []string{"appdata", "downloads", "media"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	// Also create a file (must NOT appear in results) and a hidden dir.
	if err := os.WriteFile(filepath.Join(root, "readme.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".hidden"), 0o700); err != nil {
		t.Fatal(err)
	}

	h := newBrowseRouter(t, root)
	w, m := doJSON(t, h, http.MethodGet, "/api/browse", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if m["ok"] != true {
		t.Fatalf("expected ok:true, got %v", m)
	}
	dirsRaw, _ := m["dirs"].([]any)
	names := make([]string, 0, len(dirsRaw))
	for _, d := range dirsRaw {
		dm, _ := d.(map[string]any)
		names = append(names, dm["name"].(string))
	}
	// Should contain the three dirs but not the file or the hidden dir.
	if len(names) != 3 {
		t.Fatalf("expected 3 dirs, got %v", names)
	}
	for i, want := range []string{"appdata", "downloads", "media"} {
		if names[i] != want {
			t.Fatalf("dir[%d]: expected %q, got %q", i, want, names[i])
		}
	}
}

func TestBrowseListsSubpath(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "appdata", "plex"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "appdata", "sonarr"), 0o700); err != nil {
		t.Fatal(err)
	}

	h := newBrowseRouter(t, root)
	w, m := doJSON(t, h, http.MethodGet, "/api/browse?path=appdata", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if m["ok"] != true {
		t.Fatalf("expected ok:true, got %v", m)
	}
	dirsRaw, _ := m["dirs"].([]any)
	if len(dirsRaw) != 2 {
		t.Fatalf("expected 2 dirs under appdata, got %v", dirsRaw)
	}
	// paths must be relative to mount root.
	first, _ := dirsRaw[0].(map[string]any)
	if first["path"] != "appdata/plex" {
		t.Fatalf("expected path 'appdata/plex', got %v", first["path"])
	}
}

func TestBrowseRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	h := newBrowseRouter(t, root)

	for _, bad := range []string{"../etc", "../../etc"} {
		w, m := doJSON(t, h, http.MethodGet, "/api/browse?path="+bad, "")
		if w.Code != http.StatusOK {
			t.Fatalf("expected graceful 200 for %q, got %d", bad, w.Code)
		}
		if m["ok"] != false {
			t.Fatalf("expected ok:false for traversal %q, got %v", bad, m)
		}
	}
}

func TestBrowseRejectsAbsolutePath(t *testing.T) {
	root := t.TempDir()
	h := newBrowseRouter(t, root)

	w, m := doJSON(t, h, http.MethodGet, "/api/browse?path=%2Fetc", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected graceful 200, got %d", w.Code)
	}
	if m["ok"] != false {
		t.Fatalf("expected ok:false for absolute path, got %v", m)
	}
}

// ensure context import is used (Router handlers run under request context).
var _ = context.Background

// ---------------------------------------------------------------------------
// Files endpoints (the files domain — named host folders backed up as file sets)
// ---------------------------------------------------------------------------

// newFilesTestRouter builds a router over a temp HostMountRoot with the files
// repo marker initialised (so snapshot listings reach the engine), a
// "data/docs" source folder to point sets at, and the given fake engine.
// Returns the router, the store, the service (for waitForBackupDone), and the
// mount root dir.
func newFilesTestRouter(t *testing.T, eng *fakeResticEngine) (http.Handler, *store.Repo, *api.Service, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.FilesPath = "backups/files"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(dir, "backups", "files")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "data", "docs"), 0o750); err != nil {
		t.Fatal(err)
	}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)
	sched := schedule.New(func(string) error { return nil }, st.ListTargets)
	h := api.NewHandler(cfg, st, &fakeServiceDocker{}, svc, sched, spike.DefaultProbes())
	return h.Router(), st, svc, dir
}

// fileSetRow mirrors the FileSetView JSON shape for list assertions.
type fileSetRow struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	Excludes   []string `json:"excludes"`
	Enabled    bool     `json:"enabled"`
	LastBackup int64    `json:"lastBackup"`
	PathExists bool     `json:"pathExists"`
}

// fileSetsOf decodes the GET /api/files list response.
func fileSetsOf(t *testing.T, h http.Handler) []fileSetRow {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/files", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/files status = %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		OK       bool         `json:"ok"`
		FileSets []fileSetRow `json:"fileSets"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode list: %v (%s)", err, w.Body.String())
	}
	if !resp.OK {
		t.Fatalf("GET /api/files not ok: %s", w.Body.String())
	}
	return resp.FileSets
}

// TestFileSetCRUDRoundTrip pins the manage surface: create returns the new id;
// the list carries the full FileSetView shape (enabled-by-default, excludes
// round-trip, pathExists for a real folder); PATCH merges partial fields; and
// DELETE removes the set.
func TestFileSetCRUDRoundTrip(t *testing.T) {
	h, _, _, _ := newFilesTestRouter(t, &fakeResticEngine{})

	// Create.
	w, m := doJSON(t, h, http.MethodPost, "/api/files/sets", `{"name":"docs","path":"data/docs","excludes":["*.tmp"]}`)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("create failed: %d %v", w.Code, m)
	}
	id, _ := m["id"].(string)
	if id == "" {
		t.Fatalf("create must return the new set id, got %v", m)
	}

	// List: the full view row.
	sets := fileSetsOf(t, h)
	if len(sets) != 1 {
		t.Fatalf("expected 1 set, got %+v", sets)
	}
	got := sets[0]
	if got.ID != id || got.Name != "docs" || got.Path != "data/docs" {
		t.Fatalf("row = %+v", got)
	}
	if !got.Enabled {
		t.Fatal("a created set must be enabled by default")
	}
	if len(got.Excludes) != 1 || got.Excludes[0] != "*.tmp" {
		t.Fatalf("excludes did not round-trip: %+v", got.Excludes)
	}
	if !got.PathExists {
		t.Fatal("pathExists must be true for an existing source folder")
	}
	if got.LastBackup != 0 {
		t.Fatalf("lastBackup must be 0 before any backup, got %d", got.LastBackup)
	}

	// Patch: partial update must not reset the untouched fields.
	w, m = doJSON(t, h, http.MethodPatch, "/api/files/sets/"+id, `{"enabled":false,"excludes":["*.log","cache"]}`)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("patch failed: %d %v", w.Code, m)
	}
	got = fileSetsOf(t, h)[0]
	if got.Enabled {
		t.Fatal("patch must disable the set")
	}
	if len(got.Excludes) != 2 || got.Excludes[0] != "*.log" {
		t.Fatalf("patched excludes = %+v", got.Excludes)
	}
	if got.Name != "docs" || got.Path != "data/docs" {
		t.Fatalf("patch must not clobber name/path: %+v", got)
	}

	// Delete.
	w, m = doJSON(t, h, http.MethodDelete, "/api/files/sets/"+id, "")
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("delete failed: %d %v", w.Code, m)
	}
	if left := fileSetsOf(t, h); len(left) != 0 {
		t.Fatalf("expected no sets after delete, got %+v", left)
	}
}

// TestCreateFileSetRejectsBadPaths pins the save-time path guard: a traversal
// path and a non-existent path are both refused gracefully and nothing is
// stored (the path is validated BEFORE the row is written).
func TestCreateFileSetRejectsBadPaths(t *testing.T) {
	h, _, _, _ := newFilesTestRouter(t, &fakeResticEngine{})

	for _, c := range []struct{ name, body string }{
		{"traversal", `{"name":"evil","path":"../etc"}`},
		{"absolute", `{"name":"evil","path":"/etc"}`},
		{"non-existent", `{"name":"ghost","path":"data/missing"}`},
		{"empty path", `{"name":"empty","path":""}`},
	} {
		w, m := doJSON(t, h, http.MethodPost, "/api/files/sets", c.body)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: expected graceful 200, got %d", c.name, w.Code)
		}
		if m["ok"] != false {
			t.Fatalf("%s: expected ok:false, got %v", c.name, m)
		}
	}
	if sets := fileSetsOf(t, h); len(sets) != 0 {
		t.Fatalf("no set may be stored from a rejected create, got %+v", sets)
	}
}

// TestRestoreFileSetUnconfirmedInPlace pins the never-silent rule: an in-place
// restore (no targetPath) without confirm:true fails synchronously with the
// familiar not-confirmed sentinel and starts no restic work.
func TestRestoreFileSetUnconfirmedInPlace(t *testing.T) {
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "deadbeef12345678", Time: "2026-07-14T00:00:00Z", Tags: []string{"fileset:docs"}},
	}}
	h, _, _, _ := newFilesTestRouter(t, eng)
	_, m := doJSON(t, h, http.MethodPost, "/api/files/sets", `{"name":"docs","path":"data/docs"}`)
	id, _ := m["id"].(string)

	w, m := doJSON(t, h, http.MethodPost, "/api/files/sets/"+id+"/restore", `{"snapshotId":"deadbeef12345678","confirm":false}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected graceful 200, got %d", w.Code)
	}
	if m["ok"] != false || !strings.Contains(m["error"].(string), "not confirmed") {
		t.Fatalf("expected the not-confirmed error, got %v", m)
	}
	if len(eng.restored) != 0 {
		t.Fatalf("an unconfirmed restore must start no restic work, got %v", eng.restored)
	}
}

// TestRestoreFileSetInPlaceConfirmed pins the destructive path: with
// confirm:true and no targetPath the snapshot is restored back over the set's
// resolved source folder via RestorePath, and the outcome is recorded as a
// kind "restore" run against the set's stable id.
func TestRestoreFileSetInPlaceConfirmed(t *testing.T) {
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "deadbeef12345678", Time: "2026-07-14T00:00:00Z", Tags: []string{"fileset:docs"}},
	}}
	h, st, svc, dir := newFilesTestRouter(t, eng)
	_, m := doJSON(t, h, http.MethodPost, "/api/files/sets", `{"name":"docs","path":"data/docs"}`)
	id, _ := m["id"].(string)

	w, m := doJSON(t, h, http.MethodPost, "/api/files/sets/"+id+"/restore", `{"snapshotId":"deadbeef12345678","confirm":true}`)
	if w.Code != http.StatusOK || m["ok"] != true || m["started"] != true {
		t.Fatalf("expected ok/started, got %d %v", w.Code, m)
	}
	if m["target"] != "" {
		t.Fatalf("an in-place restore acks no alternate target, got %v", m["target"])
	}
	waitForBackupDone(t, svc)

	// The service resolves both the repo and the source with slash joins
	// (paths.Resolve), so the expectation is built the same way.
	repo := dir + "/backups/files"
	wantSuffix := repo + ":deadbeef12345678:" + dir + "/data/docs"
	if len(eng.restored) != 1 || eng.restored[0] != wantSuffix {
		t.Fatalf("restored = %v, want [%s]", eng.restored, wantSuffix)
	}
	runs, err := st.ListRuns(10)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range runs {
		if r.Kind == "restore" && r.Status == "success" && r.TargetID == id {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a successful restore run against the set id, got %+v", runs)
	}
}

// TestRestoreFileSetToFolder pins the non-destructive path: a relative
// targetPath is resolved + created under the mount root, the ack returns the
// resolved folder, and the whole snapshot tree is extracted into it via
// RestoreInclude("/").
func TestRestoreFileSetToFolder(t *testing.T) {
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "deadbeef12345678", Time: "2026-07-14T00:00:00Z", Tags: []string{"fileset:docs"}},
	}}
	h, _, svc, dir := newFilesTestRouter(t, eng)
	_, m := doJSON(t, h, http.MethodPost, "/api/files/sets", `{"name":"docs","path":"data/docs"}`)
	id, _ := m["id"].(string)

	w, m := doJSON(t, h, http.MethodPost, "/api/files/sets/"+id+"/restore", `{"snapshotId":"deadbeef12345678","targetPath":"restore-here/docs"}`)
	if w.Code != http.StatusOK || m["ok"] != true || m["started"] != true {
		t.Fatalf("expected ok/started, got %d %v", w.Code, m)
	}
	wantTarget := dir + "/restore-here/docs"
	if m["target"] != wantTarget {
		t.Fatalf("target = %v, want %s", m["target"], wantTarget)
	}
	if _, err := os.Stat(filepath.Join(dir, "restore-here", "docs")); err != nil {
		t.Fatalf("the target folder must be created under the mount root: %v", err)
	}
	waitForBackupDone(t, svc)

	// Slash-joined like the service's paths.Resolve output (see the in-place test).
	repo := dir + "/backups/files"
	want := repo + ":deadbeef12345678:/->" + wantTarget
	if len(eng.restored) != 1 || eng.restored[0] != want {
		t.Fatalf("restored = %v, want [%s]", eng.restored, want)
	}
}

// TestRestoreFileSetRefusesTraversalTarget pins the containment guard on the
// alternate folder: a "../" target is refused synchronously and no restic work
// starts.
func TestRestoreFileSetRefusesTraversalTarget(t *testing.T) {
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "deadbeef12345678", Time: "2026-07-14T00:00:00Z", Tags: []string{"fileset:docs"}},
	}}
	h, _, _, _ := newFilesTestRouter(t, eng)
	_, m := doJSON(t, h, http.MethodPost, "/api/files/sets", `{"name":"docs","path":"data/docs"}`)
	id, _ := m["id"].(string)

	w, m := doJSON(t, h, http.MethodPost, "/api/files/sets/"+id+"/restore", `{"snapshotId":"deadbeef12345678","targetPath":"../escape"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected graceful 200, got %d", w.Code)
	}
	if m["ok"] != false || !strings.Contains(m["error"].(string), "invalid target folder") {
		t.Fatalf("expected the invalid-target-folder error, got %v", m)
	}
	if len(eng.restored) != 0 {
		t.Fatalf("a refused restore must start no restic work, got %v", eng.restored)
	}
}

// TestSnapshotsFileSetFilteredByTag pins the tag scoping: only THIS set's
// fileset:<Name> snapshots come back — another set's and other domains'
// snapshots in the shared repo never leak through this route.
func TestSnapshotsFileSetFilteredByTag(t *testing.T) {
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "deadbeef12345678", Time: "2026-07-14T00:00:00Z", Tags: []string{"fileset:docs"}},
		{ID: "cafebabe87654321", Time: "2026-07-14T00:00:00Z", Tags: []string{"fileset:other"}},
		{ID: "abcdef0123456789", Time: "2026-07-14T00:00:00Z", Tags: []string{"container:plex"}},
	}}
	h, _, _, _ := newFilesTestRouter(t, eng)
	_, m := doJSON(t, h, http.MethodPost, "/api/files/sets", `{"name":"docs","path":"data/docs"}`)
	id, _ := m["id"].(string)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/files/sets/"+id+"/snapshots", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var resp struct {
		OK        bool              `json:"ok"`
		Snapshots []restic.Snapshot `json:"snapshots"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	if !resp.OK || len(resp.Snapshots) != 1 || resp.Snapshots[0].ID != "deadbeef12345678" {
		t.Fatalf("expected exactly the docs-tagged snapshot, got %s", w.Body.String())
	}
}

// TestDiscoverFileSets pins the defs-less rebuild: names come from fileset:
// tags alone; ?probe=true counts without writing (the Recovery readability
// check must never resurrect entries); the real run creates a DISABLED,
// PATH-LESS set per unknown name and never clobbers an existing set.
func TestDiscoverFileSets(t *testing.T) {
	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "deadbeef12345678", Time: "2026-07-14T00:00:00Z", Tags: []string{"fileset:docs"}},
		{ID: "abcdef0123456789", Time: "2026-07-14T00:00:00Z", Tags: []string{"container:plex"}},
	}}
	h, _, _, _ := newFilesTestRouter(t, eng)

	// Probe: count only, write nothing.
	w, m := doJSON(t, h, http.MethodPost, "/api/files/discover?probe=true", "")
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("probe failed: %d %v", w.Code, m)
	}
	if m["discovered"] != float64(1) {
		t.Fatalf("probe discovered = %v, want 1", m["discovered"])
	}
	if sets := fileSetsOf(t, h); len(sets) != 0 {
		t.Fatalf("a probe must create no sets, got %+v", sets)
	}

	// Real run: one disabled, path-less set.
	w, m = doJSON(t, h, http.MethodPost, "/api/files/discover", "")
	if w.Code != http.StatusOK || m["ok"] != true || m["discovered"] != float64(1) {
		t.Fatalf("discover failed: %d %v", w.Code, m)
	}
	sets := fileSetsOf(t, h)
	if len(sets) != 1 {
		t.Fatalf("expected 1 discovered set, got %+v", sets)
	}
	if sets[0].Name != "docs" || sets[0].Path != "" || sets[0].Enabled || sets[0].PathExists {
		t.Fatalf("discovered set must be disabled and path-less: %+v", sets[0])
	}

	// Idempotent: a second run counts the (now known) set but never duplicates
	// or re-enables it.
	_, m = doJSON(t, h, http.MethodPost, "/api/files/discover", "")
	if m["discovered"] != float64(1) {
		t.Fatalf("re-discover = %v, want 1", m["discovered"])
	}
	sets = fileSetsOf(t, h)
	if len(sets) != 1 || sets[0].Enabled {
		t.Fatalf("re-discover must not duplicate or enable: %+v", sets)
	}
}
