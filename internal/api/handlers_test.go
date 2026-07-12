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
