package api_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/spike"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The retry the restore panel offers after Docker refused the GPU recreates
// the container without it, and its run says so.
func TestRestoreWithoutRuntimeLeavesOutTheGPU(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		AppKey:            strings.Repeat("a", 64),
		DataDir:           dir,
		HostMountRoot:     "/host/user",
		FlashTemplatesDir: dir + "/flash",
	}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "rest:http://127.0.0.1/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	def, err := marshalDefinition(model.Inspect{
		Name:   "/plex",
		Config: model.Config{Image: "plex:latest"},
		HostConfig: model.HostConfig{
			Runtime:        "nvidia",
			DeviceRequests: []model.DeviceRequest{{Count: -1, Capabilities: [][]string{{"gpu"}}}},
			Memory:         4 << 30,
		},
	}, "<xml/>")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertTarget(store.Target{
		ContainerName: "plex",
		AppdataPaths:  []string{"/host/user/user/appdata/plex"},
		Definition:    string(def),
	}); err != nil {
		t.Fatal(err)
	}
	eng := &fakeResticEngine{snaps: []restic.Snapshot{{
		ID: "aaaa1111", Tags: []string{"container:plex", "p1"}, Paths: []string{"/host/user/user/appdata/plex"},
	}}}
	d := &fakeServiceDocker{}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)
	h := api.NewHandler(cfg, st, d, svc, schedule.New(func(string) error { return nil }, st.ListTargets), spike.DefaultProbes()).Router()

	w, m := doJSON(t, h, http.MethodPost, "/api/containers/plex/restore",
		`{"snapshotId":"aaaa1111","confirm":true,"withoutRuntime":true}`)
	if w.Code != http.StatusOK || m["ok"] != true {
		t.Fatalf("restore: %d %v", w.Code, m)
	}
	waitForBackupDone(t, svc)

	hc := d.createdIn.HostConfig
	if hc.Runtime != "" || hc.DeviceRequests != nil || hc.Memory != 4<<30 {
		t.Fatalf("recreated with runtime %q, devices %v, memory %d", hc.Runtime, hc.DeviceRequests, hc.Memory)
	}
	runs, err := st.ListRuns(5)
	if err != nil || len(runs) == 0 {
		t.Fatalf("runs = %v, %v", runs, err)
	}
	if runs[0].Kind != "restore" || runs[0].Status != "success" || runs[0].Error != store.NoteRestoredWithoutRuntime {
		t.Fatalf("run = %+v, want a success that names what was left out", runs[0])
	}
}
