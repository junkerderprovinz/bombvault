package api_test

import (
	"context"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// An assistant that is told something different from what the dashboard shows
// is worse than one that is told nothing, so the tool answers out of the same
// service calls the HTTP handler makes.
func TestMCPGetStatusMatchesAPIStatus(t *testing.T) {
	h, st, _, key := newMCPToolRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
	settings := mustSettings(t, st)
	settings.ContainersEnabled = true
	settings.ContainersSchedule = "daily 02:30"
	settings.VMsEnabled = true
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	res := mcpCallTool(t, h, key, "get_status", "")
	if res.IsError {
		t.Fatalf("get_status: %v", res.Structured)
	}

	_, body := doJSON(t, h, http.MethodGet, "/api/status", "")
	if domains, _ := body["domains"].([]any); len(domains) != 5 {
		t.Fatalf("GET /api/status reports %v, so the comparison below proves nothing", body["domains"])
	}
	if !reflect.DeepEqual(res.Structured["domains"], body["domains"]) {
		t.Fatalf("domains differ from GET /api/status:\n tool = %v\n http = %v", res.Structured["domains"], body["domains"])
	}

	_, next := doJSON(t, h, http.MethodGet, "/api/schedule/next", "")
	if !reflect.DeepEqual(res.Structured["nextRuns"], next["runs"]) {
		t.Fatalf("nextRuns differ from GET /api/schedule/next:\n tool = %v\n http = %v", res.Structured["nextRuns"], next["runs"])
	}
	if res.Structured["backupRunning"] != false || res.Structured["everythingRunning"] != false {
		t.Fatalf("an idle instance reports %v", res.Structured)
	}
}

func TestMCPGetCoverageMatchesAPICoverage(t *testing.T) {
	docker := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{liveContainer("plex"), liveContainer("immich")}}
	h, st, _, key := newMCPToolRouter(t, docker, &fakeResticEngine{})
	settings := mustSettings(t, st)
	settings.ContainersEnabled = true
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	res := mcpCallTool(t, h, key, "get_coverage", "")
	if res.IsError {
		t.Fatalf("get_coverage: %v", res.Structured)
	}

	_, body := doJSON(t, h, http.MethodGet, "/api/coverage", "")
	report, _ := body["coverage"].(map[string]any)
	if report["total"] != float64(2) {
		t.Fatalf("GET /api/coverage counts %v containers, so the comparison below proves nothing", report["total"])
	}
	if !reflect.DeepEqual(res.Structured, body["coverage"]) {
		t.Fatalf("coverage differs from GET /api/coverage:\n tool = %v\n http = %v", res.Structured, body["coverage"])
	}
}

func TestMCPGetActivity(t *testing.T) {
	eng := &fakeResticEngine{block: make(chan struct{}), backupEntered: make(chan struct{}, 1)}
	docker := &fakeServiceDocker{inspect: model.Inspect{Name: "/plex", Image: "plex:latest", Running: true}}
	h, st, svc, key := newMCPToolRouter(t, docker, eng)
	svc.SetProgress(progress.NewStore())
	settings := mustSettings(t, st)
	settings.EncryptionEnabled = false
	settings.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	idle := mcpCallTool(t, h, key, "get_activity", "")
	if idle.IsError {
		t.Fatalf("get_activity while idle: %v", idle.Structured)
	}
	if running, _ := idle.Structured["running"].([]any); len(running) != 0 {
		t.Fatalf("an idle instance reports %v as running", running)
	}
	if busy, _ := idle.Structured["domainsBusy"].(map[string]any); len(busy) != 0 {
		t.Fatalf("an idle instance reports %v as busy", busy)
	}

	if started, err := svc.StartBackup(context.Background(), "plex"); err != nil || !started {
		t.Fatalf("backup should start: started=%v err=%v", started, err)
	}
	select {
	case <-eng.backupEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("backup never reached the engine")
	}

	res := mcpCallTool(t, h, key, "get_activity", "")
	if res.IsError {
		t.Fatalf("get_activity mid-run: %v", res.Structured)
	}
	running, _ := res.Structured["running"].([]any)
	if len(running) != 1 {
		t.Fatalf("running = %v, want the one backup in flight", running)
	}
	row, _ := running[0].(map[string]any)
	if row["domain"] != "containers" || row["item"] != "plex" || row["phase"] != "backup" {
		t.Fatalf("running entry = %v", row)
	}
	tg, err := st.GetTargetByContainer("plex")
	if err != nil {
		t.Fatal(err)
	}
	run, err := st.LastRunForTarget(tg.ID)
	if err != nil || run == nil {
		t.Fatalf("the backup opened no run row: %v", err)
	}
	if row["runId"] != run.ID {
		t.Fatalf("runId = %v, want the id of the run row in flight, %q", row["runId"], run.ID)
	}
	if busy, _ := res.Structured["domainsBusy"].(map[string]any); busy["containers"] != "backup" {
		t.Fatalf("domainsBusy = %v, want containers backup", busy)
	}
	if res.Structured["backupRunning"] != true {
		t.Fatalf("backupRunning = %v, want true", res.Structured["backupRunning"])
	}

	close(eng.block)
	waitForBackupDone(t, svc)
}

func TestMCPGetStorageStats(t *testing.T) {
	h, st, _, key := newMCPToolRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})

	now := time.Now().Unix()
	const week = int64(7 * 86400)
	for i, sample := range []store.RepoStat{
		{At: now - 2*week, RawSize: 1_000_000, RestoreSize: 3_000_000, Snapshots: 10},
		{At: now - week, RawSize: 1_500_000, RestoreSize: 3_500_000, Snapshots: 15},
		{At: now, RawSize: 3_000_000, RestoreSize: 5_000_000, Snapshots: 20},
	} {
		sample.Domain, sample.Source = "containers", "local"
		if err := st.AddRepoStat(sample); err != nil {
			t.Fatalf("seed sample %d: %v", i, err)
		}
	}

	res := mcpCallTool(t, h, key, "get_storage_stats", `{"domain":"containers"}`)
	if res.IsError {
		t.Fatalf("get_storage_stats: %v", res.Structured)
	}
	samples, _ := res.Structured["samples"].([]any)
	if len(samples) != 3 {
		t.Fatalf("samples = %v, want the three seeded ones", samples)
	}
	newest, _ := samples[0].(map[string]any)
	if newest["at"] != float64(now) || newest["rawSize"] != float64(3_000_000) {
		t.Fatalf("the first sample is not the newest: %v", newest)
	}
	if got, want := newest["restoreSize"], float64(5_000_000); got != want {
		t.Fatalf("restoreSize = %v, want %v", got, want)
	}
	if got, want := newest["snapshots"], float64(20); got != want {
		t.Fatalf("snapshots = %v, want %v", got, want)
	}
	// Two weeks between the oldest and the newest sample, two million bytes on
	// top, so the rate is a million a week.
	if got, want := res.Structured["growthBytesPerWeek"], float64(1_000_000); got != want {
		t.Fatalf("growthBytesPerWeek = %v, want %v", got, want)
	}

	res = mcpCallTool(t, h, key, "get_storage_stats", `{"domain":"containers","limit":1}`)
	if samples, _ := res.Structured["samples"].([]any); len(samples) != 1 {
		t.Fatalf("limit 1 returned %v", samples)
	}
	for _, args := range []string{`{"domain":"containers","limit":0}`, `{"domain":"containers","limit":91}`} {
		res = mcpCallTool(t, h, key, "get_storage_stats", args)
		if code := res.code(t); code != "invalid_argument" {
			t.Fatalf("%s: code = %q, want invalid_argument", args, code)
		}
	}
	if code := mcpCallTool(t, h, key, "get_storage_stats", `{"domain":"nas"}`).code(t); code != "invalid_argument" {
		t.Fatalf("an unknown domain gives %q, want invalid_argument", code)
	}
	if code := mcpCallTool(t, h, key, "get_storage_stats", `{}`).code(t); code != "invalid_argument" {
		t.Fatalf("a missing domain gives %q, want invalid_argument", code)
	}

	// The configuration domain records no size samples, so the tool answers
	// with an empty list rather than the 400 the HTTP endpoint gives.
	res = mcpCallTool(t, h, key, "get_storage_stats", `{"domain":"config"}`)
	if res.IsError {
		t.Fatalf("get_storage_stats for config: %v", res.Structured)
	}
	if samples, ok := res.Structured["samples"].([]any); !ok || len(samples) != 0 {
		t.Fatalf("config samples = %v, want an empty list", res.Structured["samples"])
	}
	if res.Structured["growthBytesPerWeek"] != nil {
		t.Fatalf("growthBytesPerWeek = %v, want null without samples", res.Structured["growthBytesPerWeek"])
	}
}
