package api_test

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// mcpStartRig is a router whose containers domain can really run a backup, with
// one key that may start them.
type mcpStartRig struct {
	h     http.Handler
	st    *store.Repo
	svc   *api.Service
	key   string
	keyID string
	dir   string
}

func newMCPStartRig(t *testing.T, d *fakeServiceDocker, eng *fakeResticEngine) *mcpStartRig {
	t.Helper()
	h, st, svc, dir := newTestRouterSvcDir(t, d, eng)
	key, id := createMCPKey(t, h, "Laptop", true)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersEnabled = true
	s.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	establishLocalRepo(t, dir, s.ContainersPath)
	return &mcpStartRig{h: h, st: st, svc: svc, key: key, keyID: id, dir: dir}
}

func (r *mcpStartRig) target(t *testing.T, name string) store.Target {
	t.Helper()
	tg, err := r.st.UpsertTarget(store.Target{ContainerName: name, IncludeInSchedule: true})
	if err != nil {
		t.Fatal(err)
	}
	return tg
}

func TestMCPStartBackupContainer(t *testing.T) {
	docker := &fakeServiceDocker{inspect: model.Inspect{Name: "/plex", Image: "plex:latest", Running: true}}
	rig := newMCPStartRig(t, docker, &fakeResticEngine{})
	tg := rig.target(t, "plex")

	var res mcpToolResult
	out := captureLog(func() {
		res = mcpCallTool(t, rig.h, rig.key, "start_backup", `{"domain":"containers","item":"plex"}`)
	})
	if res.IsError {
		t.Fatalf("start_backup: %v", res.Structured)
	}
	if res.Structured["started"] != true || res.Structured["domain"] != "containers" {
		t.Fatalf("start_backup answered %v", res.Structured)
	}
	items := mcpRows(t, res, "items")
	if len(items) != 1 || items[0]["id"] != tg.ID || items[0]["name"] != "plex" {
		t.Fatalf("items = %v, want the one container that was started", items)
	}
	if _, ok := items[0]["stops"].(map[string]any); !ok {
		t.Fatalf("the item carries no stops block: %v", items[0])
	}
	if _, ok := res.Structured["followUp"].(string); !ok {
		t.Fatalf("no follow-up sentence in %v", res.Structured)
	}

	waitForBackupDone(t, rig.svc)
	run, err := rig.st.LastRunForTarget(tg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if run == nil || run.StartedVia != "mcp" || run.StartedViaKey != rig.keyID {
		t.Fatalf("the run records %+v, want the MCP origin with key %s", run, rig.keyID)
	}
	if !strings.Contains(out, "tool start_backup -> ok") {
		t.Fatalf("the call was not logged:\n%s", out)
	}
}

// MCP never adds configuration: a container running on the host that nobody
// protects is not something an assistant can talk BombVault into backing up.
func TestMCPStartBackupNeverCreatesConfiguration(t *testing.T) {
	docker := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{liveContainer("sonarr")}}
	rig := newMCPStartRig(t, docker, &fakeResticEngine{})

	res := mcpCallTool(t, rig.h, rig.key, "start_backup", `{"domain":"containers","item":"sonarr"}`)
	if code := res.code(t); code != "not_found" {
		t.Fatalf("code = %q, want not_found (result %v)", code, res.Structured)
	}
	if _, err := rig.st.GetTargetByContainer("sonarr"); err == nil {
		t.Fatal("the call created a target row")
	}
	wantNoRuns(t, rig.st)
}

func TestMCPStartBackupRefusesOwnContainer(t *testing.T) {
	docker := &fakeServiceDocker{selfName: "bombvault", listOut: []dockercli.ContainerInfo{liveContainer("bombvault")}}
	rig := newMCPStartRig(t, docker, &fakeResticEngine{})
	rig.target(t, "bombvault")

	res := mcpCallTool(t, rig.h, rig.key, "start_backup", `{"domain":"containers","item":"bombvault"}`)
	if code := res.code(t); code != "invalid_argument" {
		t.Fatalf("code = %q, want invalid_argument (result %v)", code, res.Structured)
	}
	if msg := res.message(t); !strings.Contains(msg, "cannot back itself up") {
		t.Fatalf("message %q does not say why", msg)
	}
	wantNoRuns(t, rig.st)
}

// A domain the operator switched off has no repository, and a start through MCP
// would create one nobody asked for.
func TestMCPStartBackupDomainOff(t *testing.T) {
	rig := newMCPStartRig(t, &fakeServiceDocker{}, &fakeResticEngine{})
	if _, err := rig.st.UpsertVMTarget(store.VMTarget{Name: "win11", IncludeInSchedule: true}); err != nil {
		t.Fatal(err)
	}

	for _, args := range []string{`{"domain":"vms","item":"win11"}`, `{"domain":"vms"}`} {
		res := mcpCallTool(t, rig.h, rig.key, "start_backup", args)
		if code := res.code(t); code != "domain_off" {
			t.Fatalf("start_backup %s: code = %q, want domain_off (result %v)", args, code, res.Structured)
		}
	}
	res := mcpCallTool(t, rig.h, rig.key, "start_domain_backup", `{"domain":"vms"}`)
	if code := res.code(t); code != "domain_off" {
		t.Fatalf("start_domain_backup: code = %q, want domain_off (result %v)", code, res.Structured)
	}
}

func TestMCPStartBackupSingletonsAndFileSets(t *testing.T) {
	rig := newMCPStartRig(t, &fakeServiceDocker{}, &fakeResticEngine{})
	s := mustSettings(t, rig.st)
	s.FlashEnabled = true
	s.FlashPath = "backups/flash"
	s.ConfigEnabled = true
	s.ConfigPath = "backups/config"
	s.FilesEnabled = true
	s.FilesPath = "backups/files"
	if err := rig.st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{s.FlashPath, s.ConfigPath, s.FilesPath} {
		establishLocalRepo(t, rig.dir, rel)
	}
	if _, err := rig.st.CreateFileSet(store.FileSet{Name: "Docs", Path: "appdata", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	media, err := rig.st.CreateFileSet(store.FileSet{Name: "Media", Path: "appdata", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct{ name, args string }{
		{"flash named after its domain", `{"domain":"flash","item":"flash"}`},
		{"config without an item", `{"domain":"config"}`},
		{"a folder set by name", `{"domain":"files","item":"Docs"}`},
		{"a folder set by id", fmt.Sprintf(`{"domain":"files","item":%q}`, media.ID)},
	} {
		res := mcpCallTool(t, rig.h, rig.key, "start_backup", c.args)
		if res.IsError {
			t.Fatalf("%s: %v", c.name, res.Structured)
		}
		waitForBackupDone(t, rig.svc)
	}

	res := mcpCallTool(t, rig.h, rig.key, "start_backup", `{"domain":"flash","item":"x"}`)
	if code := res.code(t); code != "invalid_argument" {
		t.Fatalf("a named flash item gives %q, want invalid_argument (result %v)", code, res.Structured)
	}
}

// "Back up my containers now" means everything the operator protects and has
// not paused, which includes the items the scheduler leaves to their own entry.
func TestMCPStartDomainSelection(t *testing.T) {
	t.Run("containers", func(t *testing.T) {
		rig := newMCPStartRig(t, &fakeServiceDocker{}, &fakeResticEngine{})
		seedSelectionContainers(t, rig, true)

		res := mcpCallTool(t, rig.h, rig.key, "start_domain_backup", `{"domain":"containers"}`)
		if res.IsError {
			t.Fatalf("start_domain_backup: %v", res.Structured)
		}
		if got := mcpStartedNames(t, res); !slices.Equal(got, []string{"immich", "plex"}) {
			t.Fatalf("started %v, want the two containers the operator protects and has not paused", got)
		}
		waitForBackupDone(t, rig.svc)

		targets, err := rig.st.ListTargetsScheduleOrder()
		if err != nil {
			t.Fatal(err)
		}
		var scheduled []string
		for _, tg := range schedule.DomainRunTargets(targets, true) {
			if tg.IncludeInSchedule {
				scheduled = append(scheduled, tg.ContainerName)
			}
		}
		if !slices.Equal(scheduled, []string{"plex"}) {
			t.Fatalf("the scheduler's domain pass takes %v, so the difference the tool makes proves nothing", scheduled)
		}
	})

	t.Run("an override only counts while per-item schedules are on", func(t *testing.T) {
		rig := newMCPStartRig(t, &fakeServiceDocker{}, &fakeResticEngine{})
		seedSelectionContainers(t, rig, false)

		res := mcpCallTool(t, rig.h, rig.key, "start_domain_backup", `{"domain":"containers"}`)
		if res.IsError {
			t.Fatalf("start_domain_backup: %v", res.Structured)
		}
		if got := mcpStartedNames(t, res); !slices.Equal(got, []string{"immich", "plex", "sonarr"}) {
			t.Fatalf("started %v, want the paused container along with the rest", got)
		}
		waitForBackupDone(t, rig.svc)
	})

	t.Run("vms", func(t *testing.T) {
		rig := newMCPStartRig(t, &fakeServiceDocker{}, &fakeResticEngine{})
		s := mustSettings(t, rig.st)
		s.VMsEnabled = true
		s.VMsPath = "backups/vms"
		s.PerItemSchedules = true
		if err := rig.st.UpdateSettings(s); err != nil {
			t.Fatal(err)
		}
		establishLocalRepo(t, rig.dir, s.VMsPath)
		for _, c := range []struct {
			name     string
			included bool
			cadence  string
		}{
			{"win11", true, ""},
			{"debian", true, "daily 04:00"},
			{"old", true, "off"},
			{"spare", false, ""},
		} {
			if _, err := rig.st.UpsertVMTarget(store.VMTarget{Name: c.name, IncludeInSchedule: c.included}); err != nil {
				t.Fatal(err)
			}
			if c.cadence != "" {
				if err := rig.st.SetVMScheduleCadence(c.name, c.cadence); err != nil {
					t.Fatal(err)
				}
			}
		}

		res := mcpCallTool(t, rig.h, rig.key, "start_domain_backup", `{"domain":"vms"}`)
		if res.IsError {
			t.Fatalf("start_domain_backup: %v", res.Structured)
		}
		got := mcpStartedNames(t, res)
		slices.Sort(got)
		if !slices.Equal(got, []string{"debian", "win11"}) {
			t.Fatalf("started %v, want the two VMs the operator protects and has not paused", got)
		}
		waitForBackupDone(t, rig.svc)
	})

	t.Run("a folder set without a folder", func(t *testing.T) {
		rig := newMCPStartRig(t, &fakeServiceDocker{}, &fakeResticEngine{})
		s := mustSettings(t, rig.st)
		s.FilesEnabled = true
		s.FilesPath = "backups/files"
		if err := rig.st.UpdateSettings(s); err != nil {
			t.Fatal(err)
		}
		establishLocalRepo(t, rig.dir, s.FilesPath)
		if _, err := rig.st.CreateFileSet(store.FileSet{Name: "Docs", Path: "appdata", Enabled: true}); err != nil {
			t.Fatal(err)
		}
		empty, err := rig.st.CreateFileSet(store.FileSet{Name: "Unset", Enabled: true})
		if err != nil {
			t.Fatal(err)
		}

		res := mcpCallTool(t, rig.h, rig.key, "start_domain_backup", `{"domain":"files"}`)
		if res.IsError {
			t.Fatalf("start_domain_backup: %v", res.Structured)
		}
		if got := mcpStartedNames(t, res); !slices.Equal(got, []string{"Docs"}) {
			t.Fatalf("started %v, want the set that has a folder", got)
		}
		skipped := mcpRows(t, res, "skipped")
		if len(skipped) != 1 || skipped[0]["id"] != empty.ID || skipped[0]["reason"] != "no_folder" {
			t.Fatalf("skipped = %v, want the folderless set with its reason", skipped)
		}
		waitForBackupDone(t, rig.svc)
	})

	t.Run("nothing left to back up", func(t *testing.T) {
		rig := newMCPStartRig(t, &fakeServiceDocker{}, &fakeResticEngine{})
		if _, err := rig.st.UpsertTarget(store.Target{ContainerName: "plex"}); err != nil {
			t.Fatal(err)
		}

		res := mcpCallTool(t, rig.h, rig.key, "start_domain_backup", `{"domain":"containers"}`)
		if code := res.code(t); code != "nothing_to_back_up" {
			t.Fatalf("code = %q, want nothing_to_back_up (result %v)", code, res.Structured)
		}
	})
}

func TestMCPStartBackupEverything(t *testing.T) {
	eng := &fakeResticEngine{block: make(chan struct{}), backupEntered: make(chan struct{}, 1)}
	rig := newMCPStartRig(t, &fakeServiceDocker{}, eng)
	s := mustSettings(t, rig.st)
	s.FlashEnabled = true
	s.FlashPath = "backups/flash"
	s.ConfigEnabled = true
	s.ConfigPath = "backups/config"
	if err := rig.st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	establishLocalRepo(t, rig.dir, s.FlashPath)
	establishLocalRepo(t, rig.dir, s.ConfigPath)
	rig.target(t, "plex")

	res := mcpCallTool(t, rig.h, rig.key, "start_backup_everything", "")
	if res.IsError {
		t.Fatalf("start_backup_everything: %v", res.Structured)
	}
	if res.Structured["domain"] != "everything" {
		t.Fatalf("domain = %v, want everything", res.Structured["domain"])
	}
	if got := mcpStartedNames(t, res); !slices.Equal(got, []string{"containers", "flash", "config"}) {
		t.Fatalf("items = %v, want the enabled domains in the order the pass takes them", got)
	}
	waitForEverythingInFlight(t, rig.svc)

	second := mcpCallTool(t, rig.h, rig.key, "start_backup_everything", "")
	if code := second.code(t); code != "busy" {
		t.Fatalf("a second pass gives %q, want busy (result %v)", code, second.Structured)
	}
	if msg := second.message(t); !strings.Contains(msg, "Backup Everything") {
		t.Fatalf("message %q does not name the pass that is running", msg)
	}

	close(eng.block)
	waitForEverythingDone(t, rig.svc)

	parent, err := rig.st.LastRunForTarget(store.EverythingTargetID)
	if err != nil {
		t.Fatal(err)
	}
	if parent == nil || parent.StartedVia != "mcp" || parent.StartedViaKey != rig.keyID {
		t.Fatalf("the pass records %+v, want the MCP origin with key %s", parent, rig.keyID)
	}
}

// The tool list is the same for every key, so a key that may only read has to
// be turned away by the handler rather than by a missing tool.
func TestMCPReadOnlyKeyGetsNotPermitted(t *testing.T) {
	rig := newMCPStartRig(t, &fakeServiceDocker{}, &fakeResticEngine{})
	rig.target(t, "plex")
	readOnly, _ := createMCPKey(t, rig.h, "Desktop", false)

	for _, c := range []struct{ tool, args string }{
		{"start_backup", `{"domain":"containers","item":"plex"}`},
		{"start_domain_backup", `{"domain":"containers"}`},
		{"start_backup_everything", ""},
	} {
		res := mcpCallTool(t, rig.h, readOnly, c.tool, c.args)
		if code := res.code(t); code != "not_permitted" {
			t.Fatalf("%s: code = %q, want not_permitted (result %v)", c.tool, code, res.Structured)
		}
		if msg := res.message(t); !strings.Contains(msg, "Settings > System > MCP server") {
			t.Fatalf("%s: message %q does not say where to change it", c.tool, msg)
		}
	}
	wantNoRuns(t, rig.st)
}

// seedSelectionContainers writes the four containers the domain selection has to
// tell apart: protected, protected with a cadence of its own, paused and not
// protected.
func seedSelectionContainers(t *testing.T, rig *mcpStartRig, perItem bool) {
	t.Helper()
	for _, c := range []struct {
		name     string
		included bool
		cadence  string
	}{
		{"plex", true, ""},
		{"immich", true, "daily 03:00"},
		{"sonarr", true, "off"},
		{"radarr", false, ""},
	} {
		if _, err := rig.st.UpsertTarget(store.Target{ContainerName: c.name, IncludeInSchedule: c.included}); err != nil {
			t.Fatal(err)
		}
		if c.cadence != "" {
			if err := rig.st.SetScheduleCadence(c.name, c.cadence); err != nil {
				t.Fatal(err)
			}
		}
	}
	s := mustSettings(t, rig.st)
	s.PerItemSchedules = perItem
	if err := rig.st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
}

// mcpStartedNames is the item names of a start result, in the order the result
// lists them.
func mcpStartedNames(t *testing.T, res mcpToolResult) []string {
	t.Helper()
	rows := mcpRows(t, res, "items")
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		name, _ := row["name"].(string)
		out = append(out, name)
	}
	return out
}

func wantNoRuns(t *testing.T, st *store.Repo) {
	t.Helper()
	runs, err := st.ListRuns(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 0 {
		t.Fatalf("%d runs were recorded, want none", len(runs))
	}
}

// A start returns at once and the backup goes on in BombVault, so the call
// ending must not reach it.
func TestMCPStartSurvivesTheEndOfTheCall(t *testing.T) {
	eng := &fakeResticEngine{block: make(chan struct{}), backupEntered: make(chan struct{}, 1)}
	docker := &fakeServiceDocker{inspect: model.Inspect{Name: "/plex", Image: "plex:latest", Running: true}}
	rig := newMCPStartRig(t, docker, eng)
	tg := rig.target(t, "plex")

	res := mcpCallTool(t, rig.h, rig.key, "start_backup", `{"domain":"containers","item":"plex"}`)
	if res.IsError {
		t.Fatalf("start_backup: %v", res.Structured)
	}
	select {
	case <-eng.backupEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("the backup never reached the engine")
	}

	close(eng.block)
	waitForBackupDone(t, rig.svc)

	run, err := rig.st.LastRunForTarget(tg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if run == nil || run.Status != "success" {
		t.Fatalf("the run ended as %+v, want a backup that ran to the end", run)
	}
}
