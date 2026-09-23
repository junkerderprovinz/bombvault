package api_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
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

// runningContainer is a live container the host reports as up, which is what
// decides whether a backup has to stop it.
func runningContainer(name string) dockercli.ContainerInfo {
	c := liveContainer(name)
	c.State = "running"
	return c
}

// imageContainer is a running container on a named image, so the database
// recognition has something to read.
func imageContainer(name, image string) dockercli.ContainerInfo {
	c := runningContainer(name)
	c.Image = image
	return c
}

// mcpItemDomains indexes a list_items answer by domain name.
func mcpItemDomains(t *testing.T, res mcpToolResult) map[string]map[string]any {
	t.Helper()
	if res.IsError {
		t.Fatalf("list_items: %v", res.Structured)
	}
	rows, _ := res.Structured["domains"].([]any)
	if len(rows) == 0 {
		t.Fatalf("list_items returned no domains: %v", res.Structured)
	}
	out := make(map[string]map[string]any, len(rows))
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("a domain row is not an object: %v", raw)
		}
		name, _ := row["domain"].(string)
		out[name] = row
	}
	return out
}

// mcpItemsByName indexes one domain row's items by their name.
func mcpItemsByName(t *testing.T, domain map[string]any) map[string]map[string]any {
	t.Helper()
	items, ok := domain["items"].([]any)
	if !ok {
		t.Fatalf("domain row %v has no items array", domain)
	}
	out := make(map[string]map[string]any, len(items))
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("an item is not an object: %v", raw)
		}
		name, _ := item["name"].(string)
		out[name] = item
	}
	return out
}

// seedTarget adds a container target the way a first backup would.
func seedTarget(t *testing.T, st *store.Repo, name string) store.Target {
	t.Helper()
	tg, err := st.UpsertTarget(store.Target{ContainerName: name, IncludeInSchedule: true})
	if err != nil {
		t.Fatalf("seed target %q: %v", name, err)
	}
	return tg
}

// seedRun writes one run and returns its id. A status of "running" leaves it
// open.
func seedRun(t *testing.T, st *store.Repo, targetID, kind, status string, meta store.RunMeta) string {
	t.Helper()
	id, err := st.StartRunWith(targetID, kind, meta)
	if err != nil {
		t.Fatalf("start %s run on %s: %v", kind, targetID, err)
	}
	if status == "running" {
		return id
	}
	if err := st.FinishRun(id, status, "", 0, ""); err != nil {
		t.Fatalf("finish %s run on %s: %v", kind, targetID, err)
	}
	return id
}

func TestMCPListItemsIdsNamesAndStamps(t *testing.T) {
	docker := &fakeServiceDocker{
		selfName: "bombvault",
		listOut:  []dockercli.ContainerInfo{runningContainer("plex"), runningContainer("bombvault")},
	}
	h, st, _, key := newMCPToolRouter(t, docker, &fakeResticEngine{})

	plex := seedTarget(t, st, "plex")
	seedTarget(t, st, "archive")
	seedTarget(t, st, "bombvault")
	if err := st.SetScheduleCadence("archive", "off"); err != nil {
		t.Fatal(err)
	}
	docs, err := st.CreateFileSet(store.FileSet{Name: "Documents", Path: "documents", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetFileSetScheduleCadence(docs.ID, "weekly sun 06:00"); err != nil {
		t.Fatal(err)
	}

	settings := mustSettings(t, st)
	settings.ContainersEnabled = true
	settings.ContainersSchedule = "daily 02:30"
	settings.FilesEnabled = true
	settings.FilesSchedule = "weekly mon 04:00"
	settings.FlashEnabled = true
	settings.ConfigEnabled = true
	settings.ConfigSchedule = "daily 06:00"
	settings.EverythingSchedule = "daily 05:00"
	settings.PerItemSchedules = true
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	runID := seedRun(t, st, plex.ID, "backup", "success", store.RunMeta{})
	run, err := st.GetRun(runID)
	if err != nil {
		t.Fatal(err)
	}

	domains := mcpItemDomains(t, mcpCallTool(t, h, key, "list_items", ""))
	containers := mcpItemsByName(t, domains["containers"])

	if _, ok := containers["bombvault"]; ok {
		t.Fatalf("BombVault's own container is offered as a backup item: %v", containers["bombvault"])
	}
	if got := containers["plex"]["id"]; got != plex.ID {
		t.Fatalf("plex id = %v, want %q", got, plex.ID)
	}
	if got := containers["plex"]["installed"]; got != true {
		t.Fatalf("a listed container reports installed = %v", got)
	}
	if got := containers["archive"]["installed"]; got != false {
		t.Fatalf("a container the host does not run reports installed = %v", got)
	}
	if got := domains["containers"]["installedKnown"]; got != true {
		t.Fatalf("installedKnown = %v with Docker answering", got)
	}
	if got := containers["plex"]["schedule"]; got != "both" {
		t.Fatalf("plex schedule = %v, want both (the domain cadence and Backup Everything)", got)
	}
	if got := containers["archive"]["paused"]; got != true {
		t.Fatalf("a container switched off by its own override reports paused = %v", got)
	}
	if got := containers["archive"]["schedule"]; got != "none" {
		t.Fatalf("a paused container reports schedule = %v, want none", got)
	}

	if got, want := containers["plex"]["lastSuccessAt"], float64(*run.FinishedAt); got != want {
		t.Fatalf("lastSuccessAt = %v, want %v", got, want)
	}
	if got, want := containers["plex"]["lastRunAt"], float64(run.StartedAt); got != want {
		t.Fatalf("lastRunAt = %v, want %v", got, want)
	}
	if got := containers["plex"]["lastRunStatus"]; got != "success" {
		t.Fatalf("lastRunStatus = %v", got)
	}
	if got, want := containers["plex"]["lastDurationSeconds"], float64(*run.FinishedAt-run.StartedAt); got != want {
		t.Fatalf("lastDurationSeconds = %v, want %v", got, want)
	}

	files := mcpItemsByName(t, domains["files"])
	if got := files["Documents"]["id"]; got != docs.ID {
		t.Fatalf("a set without a single run is listed as %v, want id %q", files["Documents"], docs.ID)
	}
	if got := files["Documents"]["schedule"]; got != "own" {
		t.Fatalf("Documents schedule = %v, want own", got)
	}
	if got := files["Documents"]["lastRunStatus"]; got != "" {
		t.Fatalf("a set without runs reports lastRunStatus = %v", got)
	}

	flash := mcpItemsByName(t, domains["flash"])
	if got := flash["flash"]["schedule"]; got != "everything" {
		t.Fatalf("flash without its own cadence reports schedule = %v, want everything", got)
	}
	config := mcpItemsByName(t, domains["config"])
	if got := config["config"]["schedule"]; got != "domain" {
		t.Fatalf("config with its own cadence reports schedule = %v, want domain", got)
	}

	if got := domains["vms"]["enabled"]; got != false {
		t.Fatalf("the VMs domain reports enabled = %v", got)
	}
	if items, _ := domains["vms"]["items"].([]any); len(items) != 0 {
		t.Fatalf("a switched-off domain lists %v", items)
	}

	only := mcpItemDomains(t, mcpCallTool(t, h, key, "list_items", `{"domain":"files"}`))
	if len(only) != 1 || only["files"] == nil {
		t.Fatalf("a domain argument returned %v", only)
	}
	if code := mcpCallTool(t, h, key, "list_items", `{"domain":"nas"}`).code(t); code != "invalid_argument" {
		t.Fatalf("an unknown domain gives %q, want invalid_argument", code)
	}
}

func TestMCPListItemsReportsWhatABackupStops(t *testing.T) {
	docker := &fakeServiceDocker{
		listOut: []dockercli.ContainerInfo{
			runningContainer("immich"),
			runningContainer("immich_postgres"),
			liveContainer("redis"),
			liveContainer("archive"),
		},
	}
	h, st, _, key := newMCPToolRouter(t, docker, &fakeResticEngine{})

	seedTarget(t, st, "immich")
	seedTarget(t, st, "archive")
	if err := st.SetStopContainers("immich", []string{"immich_postgres", "redis"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "Windows11", Method: "graceful", IncludeInSchedule: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "Ubuntu", Method: "live", IncludeInSchedule: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateFileSet(store.FileSet{Name: "Documents", Path: "documents", Enabled: true}); err != nil {
		t.Fatal(err)
	}

	settings := mustSettings(t, st)
	settings.ContainersEnabled = true
	settings.VMsEnabled = true
	settings.FilesEnabled = true
	settings.FlashEnabled = true
	settings.ConfigEnabled = true
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	domains := mcpItemDomains(t, mcpCallTool(t, h, key, "list_items", ""))
	containers := mcpItemsByName(t, domains["containers"])

	stops, _ := containers["immich"]["stops"].(map[string]any)
	if stops["self"] != true || stops["known"] != true {
		t.Fatalf("a running container reports stops = %v", stops)
	}
	also, _ := stops["containers"].([]any)
	if len(also) != 1 || also[0] != "immich_postgres" {
		t.Fatalf("stops.containers = %v, want only the one that is up", also)
	}

	stopped, _ := containers["archive"]["stops"].(map[string]any)
	if stopped["self"] != false {
		t.Fatalf("a container that is already down reports stops = %v", stopped)
	}

	vms := mcpItemsByName(t, domains["vms"])
	if s, _ := vms["Windows11"]["stops"].(map[string]any); s["self"] != true {
		t.Fatalf("a gracefully backed-up VM reports stops = %v", s)
	}
	if s, _ := vms["Ubuntu"]["stops"].(map[string]any); s["self"] != false {
		t.Fatalf("a live-backed-up VM reports stops = %v", s)
	}

	for _, row := range []struct{ domain, item string }{
		{"files", "Documents"}, {"flash", "flash"}, {"config", "config"},
	} {
		items := mcpItemsByName(t, domains[row.domain])
		s, _ := items[row.item]["stops"].(map[string]any)
		if s["self"] != false {
			t.Fatalf("%s/%s reports stops = %v, want nothing stopped", row.domain, row.item, s)
		}
		if c, _ := s["containers"].([]any); len(c) != 0 {
			t.Fatalf("%s/%s stops the containers %v", row.domain, row.item, c)
		}
	}
}

func TestMCPListItemsDescribesDatabaseContainers(t *testing.T) {
	docker := &fakeServiceDocker{
		listOut: []dockercli.ContainerInfo{
			imageContainer("immich_postgres", "postgres:16"),
			imageContainer("paperless_db", "mariadb:11"),
			runningContainer("plex"),
		},
		inspects: map[string]model.Inspect{
			"immich_postgres": {Running: true, Config: model.Config{Image: "postgres:16", Env: []string{"POSTGRES_PASSWORD=x"}}},
			"paperless_db":    {Running: true, Config: model.Config{Image: "mariadb:11", Env: []string{"MARIADB_ROOT_PASSWORD=x"}}},
		},
	}
	h, st, _, key := newMCPToolRouter(t, docker, &fakeResticEngine{})

	pg := seedTarget(t, st, "immich_postgres")
	seedTarget(t, st, "paperless_db")
	seedTarget(t, st, "plex")
	if err := st.SetDBDumpOff("paperless_db", true); err != nil {
		t.Fatal(err)
	}

	settings := mustSettings(t, st)
	settings.ContainersEnabled = true
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	dumpID := seedRun(t, st, pg.ID, "dbdump", "success", store.RunMeta{})
	dump, err := st.GetRun(dumpID)
	if err != nil {
		t.Fatal(err)
	}

	containers := mcpItemsByName(t, mcpItemDomains(t, mcpCallTool(t, h, key, "list_items", ""))["containers"])

	db, _ := containers["immich_postgres"]["database"].(map[string]any)
	if db["engine"] != "postgres" || db["engineKnown"] != true || db["dumpOff"] != false {
		t.Fatalf("the recognised postgres container reports database = %v", db)
	}
	last, _ := db["lastDump"].(map[string]any)
	if last["at"] != float64(*dump.FinishedAt) || last["status"] != "success" {
		t.Fatalf("lastDump = %v, want the seeded dump run", last)
	}
	if off, _ := containers["paperless_db"]["database"].(map[string]any); off["dumpOff"] != true {
		t.Fatalf("a container whose dumps are switched off reports database = %v", off)
	}
	if _, ok := containers["plex"]["database"]; ok {
		t.Fatalf("a plain container carries a database block: %v", containers["plex"])
	}

	inspected := 0
	for _, call := range docker.calls {
		if strings.HasPrefix(call, "inspect:") {
			inspected++
		}
	}
	if inspected != 2 {
		t.Fatalf("the listing inspected %d containers (%v), want only the two database candidates", inspected, docker.calls)
	}

	down := &fakeServiceDocker{listErr: errors.New("docker: no such host")}
	h2, st2, _, key2 := newMCPToolRouter(t, down, &fakeResticEngine{})
	seedTarget(t, st2, "paperless_db")
	if err := st2.SetDBDumpOff("paperless_db", true); err != nil {
		t.Fatal(err)
	}
	blind := mustSettings(t, st2)
	blind.ContainersEnabled = true
	if err := st2.UpdateSettings(blind); err != nil {
		t.Fatal(err)
	}
	rows := mcpItemsByName(t, mcpItemDomains(t, mcpCallTool(t, h2, key2, "list_items", ""))["containers"])
	unknown, _ := rows["paperless_db"]["database"].(map[string]any)
	if unknown["engineKnown"] != false || unknown["engine"] != "" || unknown["dumpOff"] != true {
		t.Fatalf("with Docker unreachable the database block reads %v", unknown)
	}
}

func TestMCPListItemsDockerDown(t *testing.T) {
	docker := &fakeServiceDocker{listErr: errors.New("docker: no such host")}
	h, st, _, key := newMCPToolRouter(t, docker, &fakeResticEngine{})
	seedTarget(t, st, "plex")
	settings := mustSettings(t, st)
	settings.ContainersEnabled = true
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	res := mcpCallTool(t, h, key, "list_items", "")
	if res.IsError {
		t.Fatalf("an unreachable Docker made the whole listing fail: %v", res.Structured)
	}
	domains := mcpItemDomains(t, res)
	if got := domains["containers"]["installedKnown"]; got != false {
		t.Fatalf("installedKnown = %v with Docker unreachable", got)
	}
	containers := mcpItemsByName(t, domains["containers"])
	if len(containers) != 1 {
		t.Fatalf("the stored rows are gone from the listing: %v", containers)
	}
	if _, ok := containers["plex"]["installed"]; ok {
		t.Fatalf("plex claims an installed state nobody could read: %v", containers["plex"])
	}
	stops, _ := containers["plex"]["stops"].(map[string]any)
	if stops["known"] != false || stops["self"] != false {
		t.Fatalf("stops = %v, want an honest unknown", stops)
	}
}

// mcpRunRows is the runs array of a list_runs answer.
func mcpRunRows(t *testing.T, res mcpToolResult) []map[string]any {
	t.Helper()
	if res.IsError {
		t.Fatalf("list_runs: %v", res.Structured)
	}
	raw, ok := res.Structured["runs"].([]any)
	if !ok {
		t.Fatalf("list_runs answered without a runs array: %v", res.Structured)
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		row, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("a run row is not an object: %v", item)
		}
		out = append(out, row)
	}
	return out
}

// mcpRunIDs is the id list of a list_runs answer, in the order it came back.
func mcpRunIDs(t *testing.T, res mcpToolResult) []string {
	t.Helper()
	rows := mcpRunRows(t, res)
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		id, _ := row["id"].(string)
		out = append(out, id)
	}
	return out
}

func TestMCPListRunsLimitFilterAndEnrichment(t *testing.T) {
	docker := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{runningContainer("plex")}}
	h, st, _, key := newMCPToolRouter(t, docker, &fakeResticEngine{})

	plex := seedTarget(t, st, "plex")
	vm, err := st.UpsertVMTarget(store.VMTarget{Name: "Windows11", IncludeInSchedule: true})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 25; i++ {
		seedRun(t, st, plex.ID, "backup", "success", store.RunMeta{})
	}
	failed := seedRun(t, st, plex.ID, "backup", "failed", store.RunMeta{})
	vmRun := seedRun(t, st, vm.ID, "backup", "success", store.RunMeta{})

	res := mcpCallTool(t, h, key, "list_runs", "")
	rows := mcpRunRows(t, res)
	if len(rows) != 20 {
		t.Fatalf("list_runs returned %d rows, want the default 20", len(rows))
	}
	if res.Structured["truncated"] != true {
		t.Fatalf("truncated = %v with more runs than the limit", res.Structured["truncated"])
	}

	for _, args := range []string{`{"limit":0}`, `{"limit":101}`} {
		if code := mcpCallTool(t, h, key, "list_runs", args).code(t); code != "invalid_argument" {
			t.Fatalf("%s: code = %q, want invalid_argument", args, code)
		}
	}

	byDomain := mcpRunIDs(t, mcpCallTool(t, h, key, "list_runs", `{"domain":"vms"}`))
	if len(byDomain) != 1 || byDomain[0] != vmRun {
		t.Fatalf("the vms domain returned %v, want only %q", byDomain, vmRun)
	}
	byItem := mcpRunIDs(t, mcpCallTool(t, h, key, "list_runs", `{"domain":"vms","item":"Windows11"}`))
	if len(byItem) != 1 || byItem[0] != vmRun {
		t.Fatalf("the item filter returned %v, want only %q", byItem, vmRun)
	}
	if code := mcpCallTool(t, h, key, "list_runs", `{"item":"Windows11"}`).code(t); code != "invalid_argument" {
		t.Fatalf("an item without a domain gives %q, want invalid_argument", code)
	}
	if code := mcpCallTool(t, h, key, "list_runs", `{"domain":"vms","item":"Gone"}`).code(t); code != "not_found" {
		t.Fatalf("an unknown item gives %q, want not_found", code)
	}

	byStatus := mcpRunIDs(t, mcpCallTool(t, h, key, "list_runs", `{"status":"failed"}`))
	if len(byStatus) != 1 || byStatus[0] != failed {
		t.Fatalf("the failed filter returned %v, want only %q", byStatus, failed)
	}
	if code := mcpCallTool(t, h, key, "list_runs", `{"status":"broken"}`).code(t); code != "invalid_argument" {
		t.Fatalf("an unknown status gives %q, want invalid_argument", code)
	}

	prune := seedRun(t, st, "containers", "prune", "success", store.RunMeta{})
	byKind := mcpRunIDs(t, mcpCallTool(t, h, key, "list_runs", `{"kind":"prune"}`))
	if len(byKind) != 1 || byKind[0] != prune {
		t.Fatalf("the prune filter returned %v, want only %q", byKind, prune)
	}
	if code := mcpCallTool(t, h, key, "list_runs", `{"kind":"reboot"}`).code(t); code != "invalid_argument" {
		t.Fatalf("an unknown kind gives %q, want invalid_argument", code)
	}

	pruneRun, err := st.GetRun(prune)
	if err != nil {
		t.Fatal(err)
	}
	since := mcpRunIDs(t, mcpCallTool(t, h, key, "list_runs", fmt.Sprintf(`{"since":%d}`, pruneRun.StartedAt)))
	if len(since) == 0 || since[0] != prune {
		t.Fatalf("the since filter returned %v, want the prune run first", since)
	}

	ack, err := st.AcknowledgeRuns([]string{failed})
	if err != nil || ack != 1 {
		t.Fatalf("acknowledge the failed run: n=%d err=%v", ack, err)
	}
	acked := mcpRunRows(t, mcpCallTool(t, h, key, "list_runs", `{"status":"failed"}`))
	if acked[0]["acknowledged"] != true {
		t.Fatalf("an acknowledged failure comes back as %v", acked[0])
	}

	_, body := doJSON(t, h, http.MethodGet, "/api/runs", "")
	httpRuns, _ := body["runs"].([]any)
	if len(httpRuns) == 0 {
		t.Fatalf("GET /api/runs is empty, so the comparison below proves nothing")
	}
	names := map[string]string{}
	for _, raw := range httpRuns {
		row, _ := raw.(map[string]any)
		id, _ := row["id"].(string)
		names[id], _ = row["target"].(string)
	}
	for _, row := range mcpRunRows(t, mcpCallTool(t, h, key, "list_runs", `{"limit":100}`)) {
		id, _ := row["id"].(string)
		if row["itemName"] != names[id] {
			t.Fatalf("run %s is named %v through MCP and %q over HTTP", id, row["itemName"], names[id])
		}
	}
}

func TestMCPListRunsDomainLevelRowsAndVocabulary(t *testing.T) {
	docker := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{runningContainer("plex")}}
	h, st, _, key := newMCPToolRouter(t, docker, &fakeResticEngine{})

	plex := seedTarget(t, st, "plex")
	vm, err := st.UpsertVMTarget(store.VMTarget{Name: "Windows11", IncludeInSchedule: true})
	if err != nil {
		t.Fatal(err)
	}
	set, err := st.CreateFileSet(store.FileSet{Name: "Documents", Path: "documents", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	seedRun(t, st, plex.ID, "backup", "success", store.RunMeta{})
	seedRun(t, st, vm.ID, "backup", "success", store.RunMeta{})
	seedRun(t, st, set.ID, "backup", "success", store.RunMeta{})
	prune := seedRun(t, st, "containers", "prune", "success", store.RunMeta{})
	verify := seedRun(t, st, "vms", "verify", "success", store.RunMeta{})
	seedRun(t, st, "files", "offsite", "success", store.RunMeta{})
	seedRun(t, st, store.EverythingTargetID, "backup", "success", store.RunMeta{})

	rows := mcpRunRows(t, mcpCallTool(t, h, key, "list_runs", `{"domain":"containers","kind":"prune"}`))
	if len(rows) != 1 || rows[0]["id"] != prune {
		t.Fatalf("a containers prune is invisible under its own domain: %v", rows)
	}
	if rows[0]["domain"] != "containers" || rows[0]["itemId"] != "containers" {
		t.Fatalf("the prune row reads %v, want the containers domain", rows[0])
	}
	rows = mcpRunRows(t, mcpCallTool(t, h, key, "list_runs", `{"domain":"vms","kind":"verify"}`))
	if len(rows) != 1 || rows[0]["id"] != verify {
		t.Fatalf("a vms verify is invisible under its own domain: %v", rows)
	}

	seen := map[string]bool{}
	for _, row := range mcpRunRows(t, mcpCallTool(t, h, key, "list_runs", `{"limit":100}`)) {
		domain, _ := row["domain"].(string)
		if domain == "" {
			t.Fatalf("a run came back without a domain: %v", row)
		}
		seen[domain] = true
	}
	for _, domain := range []string{"containers", "vms", "files", "everything"} {
		if !seen[domain] {
			t.Fatalf("no run came back with domain %q: %v", domain, seen)
		}
	}
	for domain := range seen {
		back := mcpCallTool(t, h, key, "list_runs", fmt.Sprintf(`{"domain":%q}`, domain))
		if back.IsError {
			t.Fatalf("list_runs refuses its own domain value %q: %v", domain, back.Structured)
		}
		if domain == "everything" {
			continue
		}
		items := mcpCallTool(t, h, key, "list_items", fmt.Sprintf(`{"domain":%q}`, domain))
		if items.IsError {
			t.Fatalf("list_items refuses the domain value %q that list_runs returned: %v", domain, items.Structured)
		}
	}
}

func TestMCPListRunsShowsOriginLabelEvenWhenRevoked(t *testing.T) {
	docker := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{runningContainer("plex")}}
	h, st, _, key := newMCPToolRouter(t, docker, &fakeResticEngine{})
	_, desktopID := createMCPKey(t, h, "Desktop", true)

	plex := seedTarget(t, st, "plex")
	runID := seedRun(t, st, plex.ID, "backup", "success", store.RunMeta{StartedVia: "mcp", StartedViaKey: desktopID})

	w, _ := doMCPKey(t, h, http.MethodPost, "/api/mcp/keys/"+desktopID+"/revoke", "")
	if w.Code != http.StatusOK {
		t.Fatalf("revoke: status = %d", w.Code)
	}

	rows := mcpRunRows(t, mcpCallTool(t, h, key, "list_runs", `{"domain":"containers","item":"plex"}`))
	if len(rows) != 1 || rows[0]["id"] != runID {
		t.Fatalf("list_runs returned %v", rows)
	}
	if rows[0]["startedVia"] != "mcp" || rows[0]["startedViaLabel"] != "Desktop" {
		t.Fatalf("the run of a revoked key reads %v, want it named Desktop", rows[0])
	}

	_, body := doJSON(t, h, http.MethodGet, "/api/runs", "")
	httpRuns, _ := body["runs"].([]any)
	found := false
	for _, raw := range httpRuns {
		row, _ := raw.(map[string]any)
		if row["id"] != runID {
			continue
		}
		found = true
		if row["startedViaLabel"] != "Desktop" || row["startedViaRevoked"] != true {
			t.Fatalf("GET /api/runs reads %v, want the revoked Desktop key named", row)
		}
	}
	if !found {
		t.Fatalf("GET /api/runs does not carry the run at all: %v", httpRuns)
	}
}
