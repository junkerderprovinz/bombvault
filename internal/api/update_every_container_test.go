package api_test

import (
	"net/http"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
)

// The switch for every container starts off, a body that does not know it
// leaves it as it is, and one that names it sets it.
func TestTheSwitchForEveryContainerIsKeptByABodyThatDoesNotNameIt(t *testing.T) {
	h, st := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})

	_, m := doJSON(t, h, http.MethodGet, "/api/settings", "")
	settings, _ := m["settings"].(map[string]any)
	if settings["updateContainersAfterBackup"] != false {
		t.Fatalf("updateContainersAfterBackup = %v, want false by default", settings["updateContainersAfterBackup"])
	}

	const on = `{"containersPath": "backups/c", "containersSchedule": "off", "vmsSchedule": "off", "flashSchedule": "off", "updateContainersAfterBackup": true}`
	if w, env := doJSON(t, h, http.MethodPut, "/api/settings", on); w.Code != http.StatusOK || env["ok"] != true {
		t.Fatalf("put status = %d body = %v", w.Code, env)
	}
	const oldShape = `{"containersPath": "backups/c", "containersSchedule": "off", "vmsSchedule": "off", "flashSchedule": "off"}`
	if w, env := doJSON(t, h, http.MethodPut, "/api/settings", oldShape); w.Code != http.StatusOK || env["ok"] != true {
		t.Fatalf("put status = %d body = %v", w.Code, env)
	}
	stored, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !stored.UpdateContainersAfterBackup {
		t.Fatal("a body that does not know the switch must leave it on")
	}
}

// A container's row says whether it updates after its backup and whether that
// is its own choice, and the page can give it one or take it back.
func TestAContainerUpdatesWithTheSwitchUntilItChoosesForItself(t *testing.T) {
	d := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{{Name: "plex", Image: "plexinc/pms-docker:latest"}}}
	h, st := newTestRouter(t, d, &fakeResticEngine{})
	row := func() map[string]any {
		_, m := doJSON(t, h, http.MethodGet, "/api/containers", "")
		return containerRows(t, m)["plex"]
	}

	if r := row(); r["updateAfterBackup"] != false || r["updateAfterBackupOwn"] != false {
		t.Fatalf("a fresh container follows the switch, which is off; got %v / %v", r["updateAfterBackup"], r["updateAfterBackupOwn"])
	}
	s, _ := st.GetSettings()
	s.UpdateContainersAfterBackup = true
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	if r := row(); r["updateAfterBackup"] != true || r["updateAfterBackupOwn"] != false {
		t.Fatalf("with the switch on the container updates without a choice of its own; got %v / %v", r["updateAfterBackup"], r["updateAfterBackupOwn"])
	}

	if w, env := doJSON(t, h, http.MethodPatch, "/api/containers/plex", `{"updateAfterBackup": false}`); w.Code != http.StatusOK || env["ok"] != true {
		t.Fatalf("patch status = %d body = %v", w.Code, env)
	}
	if r := row(); r["updateAfterBackup"] != false || r["updateAfterBackupOwn"] != true {
		t.Fatalf("its own choice of off wins over the switch; got %v / %v", r["updateAfterBackup"], r["updateAfterBackupOwn"])
	}

	if w, env := doJSON(t, h, http.MethodPatch, "/api/containers/plex", `{"updateAfterBackupDefault": true}`); w.Code != http.StatusOK || env["ok"] != true {
		t.Fatalf("patch status = %d body = %v", w.Code, env)
	}
	if r := row(); r["updateAfterBackup"] != true || r["updateAfterBackupOwn"] != false {
		t.Fatalf("back on the switch it updates again; got %v / %v", r["updateAfterBackup"], r["updateAfterBackupOwn"])
	}
}
