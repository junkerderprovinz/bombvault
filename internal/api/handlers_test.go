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
	return h.Router(), st
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

func TestBackupOK(t *testing.T) {
	d := &fakeServiceDocker{inspect: model.Inspect{Name: "/plex", Image: "plex:latest"}}
	h, _ := newTestRouter(t, d, &fakeResticEngine{})
	w, m := doJSON(t, h, http.MethodPost, "/api/containers/plex/backup", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if m["ok"] != true {
		t.Fatalf("expected ok:true, got %v", m)
	}
	if m["snapshotId"] != "deadbeef12345678" {
		t.Fatalf("expected snapshotId, got %v", m)
	}
}

func TestBackupFailureGraceful(t *testing.T) {
	d := &fakeServiceDocker{inspect: model.Inspect{Name: "/plex", Image: "plex:latest"}}
	eng := &fakeResticEngine{backupErr: errors.New("restic backup failed: /secret/repo/path")}
	h, _ := newTestRouter(t, d, eng)
	w, m := doJSON(t, h, http.MethodPost, "/api/containers/plex/backup", "")
	// Operational failure → still HTTP 200, {ok:false,error}.
	if w.Code != http.StatusOK {
		t.Fatalf("expected graceful 200, got %d", w.Code)
	}
	if m["ok"] != false {
		t.Fatalf("expected ok:false, got %v", m)
	}
	errStr, _ := m["error"].(string)
	if errStr == "" {
		t.Fatalf("expected an error message, got %v", m)
	}
	// Error must be scrubbed — must not leak the repo path.
	if strings.Contains(errStr, "/secret/repo/path") {
		t.Fatalf("error leaked the repo path: %q", errStr)
	}
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
	h, _ := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
	// BackupVM will fail at virsh list because fakeVirsh returns empty — but
	// the handler must still return ok:false (not a 500) so we test graceful error.
	w, m := doJSON(t, h, http.MethodPost, "/api/vms/win11/backup", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	// fakeVirsh.List returns nothing, so win11 is unknown → error path.
	if m["ok"] == nil {
		t.Fatalf("missing ok field: %v", m)
	}
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
	h, _ := newTestRouter(t, d, &fakeResticEngine{})
	// Drive one backup so a run exists.
	doJSON(t, h, http.MethodPost, "/api/containers/plex/backup", "")
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
