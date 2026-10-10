package api_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/platform"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/spike"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// newItemsRouter is newTestRouterSvc with a libvirt the test chooses.
func newItemsRouter(t *testing.T, d *fakeServiceDocker, v virshcli.Virsh) (http.Handler, *store.Repo, *api.Service) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	svc := api.NewService(cfg, st, d, v, &fakeResticEngine{})
	sched := schedule.New(func(string) error { return nil }, st.ListTargets)
	return api.NewHandler(cfg, st, d, svc, sched, spike.DefaultProbes()).Router(), st, svc
}

// enableEveryKind switches all six kinds on and lets change adjust the rest.
func enableEveryKind(t *testing.T, st *store.Repo, change func(*store.Settings)) {
	t.Helper()
	s := mustSettings(t, st)
	s.ContainersEnabled, s.VMsEnabled, s.FilesEnabled = true, true, true
	s.ZFSEnabled, s.FlashEnabled, s.ConfigEnabled = true, true, true
	if change != nil {
		change(&s)
	}
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
}

// listItems reads GET /api/items and indexes its rows by "kind/key".
func listItems(t *testing.T, h http.Handler) (rows map[string]map[string]any, body map[string]any) {
	t.Helper()
	w, body := doJSON(t, h, http.MethodGet, "/api/items", "")
	if w.Code != http.StatusOK || body["ok"] != true {
		t.Fatalf("GET /api/items: %d %v", w.Code, body)
	}
	items, ok := body["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %v", body["items"])
	}
	rows = make(map[string]map[string]any, len(items))
	for _, raw := range items {
		row, _ := raw.(map[string]any)
		id := fmt.Sprintf("%v/%v", row["kind"], row["key"])
		if _, twice := rows[id]; twice {
			t.Fatalf("%s is listed twice", id)
		}
		rows[id] = row
	}
	return rows, body
}

func TestListItemsCarriesEveryKindUnderItsKey(t *testing.T) {
	docker := &fakeServiceDocker{
		selfName: "bombvault",
		listOut:  []dockercli.ContainerInfo{runningContainer("plex"), runningContainer("bombvault"), liveContainer("fresh")},
	}
	virsh := listVMsVirsh{vms: []virshcli.VMInfo{
		{Name: "Windows11", State: "running", FriendlyName: "Windows11"},
		{Name: "1_debian", State: "shut off", FriendlyName: "debian"},
	}}
	h, st, svc := newItemsRouter(t, docker, virsh)
	enableEveryKind(t, st, nil)

	seedTarget(t, st, "plex")
	seedTarget(t, st, "archive")
	for _, name := range []string{"Windows11", "old-vm"} {
		if _, err := st.UpsertVMTarget(store.VMTarget{Name: name, Method: "graceful", IncludeInSchedule: true}); err != nil {
			t.Fatal(err)
		}
	}
	docs, err := st.CreateFileSet(store.FileSet{Name: "Documents", Path: "documents", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	appdata, err := st.CreateZFSDataset(store.ZFSDataset{Dataset: "cache/appdata", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	rows, body := listItems(t, h)
	names := map[string]string{
		"container/plex":      "plex",
		"container/bombvault": "bombvault",
		"container/fresh":     "fresh",
		"container/archive":   "archive",
		"vm/Windows11":        "Windows11",
		"vm/1_debian":         "1_debian",
		"vm/old-vm":           "old-vm",
		"files/" + docs.ID:    "Documents",
		"zfs/" + appdata.ID:   "cache/appdata",
		"flash/flash":         "flash",
		"config/config":       "config",
	}
	if len(rows) != len(names) {
		t.Fatalf("listed %d items, want %d: %v", len(rows), len(names), rows)
	}
	for id, name := range names {
		row, ok := rows[id]
		if !ok {
			t.Fatalf("%s is missing from %v", id, rows)
		}
		if row["name"] != name {
			t.Fatalf("%s is named %v, want %q", id, row["name"], name)
		}
		if _, disabled := row["kindDisabled"]; disabled {
			t.Fatalf("%s reports a disabled kind with every kind on", id)
		}
	}
	if unlisted, _ := body["unlisted"].([]any); len(unlisted) != 0 {
		t.Fatalf("unlisted = %v with Docker and libvirt answering", body["unlisted"])
	}

	host := []struct {
		id        string
		installed any
		state     any
	}{
		{"container/plex", true, "running"},
		{"container/archive", false, nil},
		{"vm/Windows11", true, "running"},
		{"vm/1_debian", true, "shut off"},
		{"vm/old-vm", false, nil},
		{"files/" + docs.ID, nil, nil},
		{"flash/flash", nil, nil},
	}
	for _, c := range host {
		if got := rows[c.id]["installed"]; got != c.installed {
			t.Fatalf("%s reports installed = %v, want %v", c.id, got, c.installed)
		}
		if got := rows[c.id]["state"]; got != c.state {
			t.Fatalf("%s reports state = %v, want %v", c.id, got, c.state)
		}
	}
	if rows["container/bombvault"]["self"] != true {
		t.Fatalf("BombVault's own container is not marked: %v", rows["container/bombvault"])
	}
	if _, marked := rows["container/plex"]["self"]; marked {
		t.Fatalf("plex is marked as BombVault's own container: %v", rows["container/plex"])
	}
	if fresh := rows["container/fresh"]; fresh["included"] != false || fresh["lastBackup"] != float64(0) {
		t.Fatalf("a container without an entry reads as %v", fresh)
	}

	svc.SetPlatform(platform.TrueNAS{})
	t.Cleanup(func() { svc.SetPlatform(nil) })
	rows, _ = listItems(t, h)
	if got := rows["vm/1_debian"]["name"]; got != "debian" {
		t.Fatalf("on TrueNAS the VM 1_debian is named %v, want debian", got)
	}
}

func TestListItemsScheduledAndPausedStates(t *testing.T) {
	docker := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{runningContainer("plex")}}
	h, st, _ := newItemsRouter(t, docker, fakeVirsh{})
	enableEveryKind(t, st, func(s *store.Settings) {
		s.PerItemSchedules = true
		s.ContainersSchedule = "daily 02:30"
		s.FilesSchedule = "weekly mon 04:00"
		s.ConfigSchedule = "daily 06:00"
		s.EverythingSchedule = "daily 05:00"
	})

	seedTarget(t, st, "plex")
	seedTarget(t, st, "archive")
	if err := st.SetScheduleCadence("archive", "off"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertTarget(store.Target{ContainerName: "excluded"}); err != nil {
		t.Fatal(err)
	}
	docs, err := st.CreateFileSet(store.FileSet{Name: "Documents", Path: "documents", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetFileSetScheduleCadence(docs.ID, "weekly sun 06:00"); err != nil {
		t.Fatal(err)
	}

	rows, _ := listItems(t, h)
	cases := []struct {
		id               string
		included, paused bool
		schedule         map[string]any
	}{
		{"container/plex", true, false, map[string]any{"kind": "both", "spec": "daily 02:30", "alsoSpec": "daily 05:00"}},
		{"container/archive", true, true, map[string]any{"kind": "none", "spec": "", "alsoSpec": "", "reason": "override-off"}},
		{"container/excluded", false, false, map[string]any{"kind": "none", "spec": "", "alsoSpec": "", "reason": "excluded"}},
		{"files/" + docs.ID, true, false, map[string]any{"kind": "own", "spec": "weekly sun 06:00", "alsoSpec": ""}},
		{"flash/flash", true, false, map[string]any{"kind": "everything", "spec": "daily 05:00", "alsoSpec": ""}},
		{"config/config", true, false, map[string]any{"kind": "both", "spec": "daily 06:00", "alsoSpec": "daily 05:00"}},
	}
	for _, c := range cases {
		row := rows[c.id]
		if row["included"] != c.included || row["paused"] != c.paused {
			t.Fatalf("%s reports included = %v and paused = %v, want %v and %v", c.id, row["included"], row["paused"], c.included, c.paused)
		}
		if !reflect.DeepEqual(row["effectiveSchedule"], c.schedule) {
			t.Fatalf("%s reports the schedule %v, want %v", c.id, row["effectiveSchedule"], c.schedule)
		}
	}
}

func TestListItemsCarriesTheNewestFourteenRuns(t *testing.T) {
	h, st, _ := newItemsRouter(t, &fakeServiceDocker{}, fakeVirsh{})
	enableEveryKind(t, st, nil)

	plex := seedTarget(t, st, "plex")
	broken := seedTarget(t, st, "broken")
	seedTarget(t, st, "idle")

	measured, err := st.StartRun(plex.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRunMeasured(measured, "success", "snap", 10, "", &store.RunMetrics{SourceBytes: 4096}, ""); err != nil {
		t.Fatal(err)
	}
	type run struct{ id, kind, status string }
	seeded := []run{{measured, "backup", "success"}}
	for i := range 16 {
		r := run{kind: "backup", status: "success"}
		switch i % 4 {
		case 1:
			r.status = "failed"
		case 3:
			r.kind = "dbdump"
		}
		r.id = seedRun(t, st, plex.ID, r.kind, r.status, store.RunMeta{})
		seeded = append(seeded, r)
	}
	seedRun(t, st, broken.ID, "backup", "failed", store.RunMeta{})

	rows, _ := listItems(t, h)
	runs, _ := rows["container/plex"]["runs"].([]any)
	if len(runs) != 14 {
		t.Fatalf("plex carries %d runs, want 14 of its %d", len(runs), len(seeded))
	}
	for i, raw := range runs {
		got, _ := raw.(map[string]any)
		want := seeded[len(seeded)-1-i]
		if got["id"] != want.id || got["kind"] != want.kind || got["status"] != want.status {
			t.Fatalf("run %d is %v, want %+v", i, got, want)
		}
		if at, _ := got["startedAt"].(float64); at <= 0 {
			t.Fatalf("run %d carries no start time: %v", i, got)
		}
	}
	if got := rows["container/plex"]["sourceBytes"]; got != float64(4096) {
		t.Fatalf("plex reports sourceBytes = %v, want the 4096 of its last measured backup", got)
	}
	if at, _ := rows["container/plex"]["lastBackup"].(float64); at <= 0 {
		t.Fatalf("plex reports lastBackup = %v after successful backups", rows["container/plex"]["lastBackup"])
	}

	if got := rows["container/broken"]; got["lastRunStatus"] != "failed" || got["lastBackup"] != float64(0) {
		t.Fatalf("an item whose only backup failed reads as %v", got)
	}

	idle := rows["container/idle"]
	if runs, ok := idle["runs"].([]any); !ok || len(runs) != 0 {
		t.Fatalf("an item without runs carries runs = %v, want an empty list", idle["runs"])
	}
	if size, present := idle["sourceBytes"]; !present || size != nil {
		t.Fatalf("an item without runs reports sourceBytes = %v, want null", size)
	}
	if idle["lastRunStatus"] != "" || idle["lastBackup"] != float64(0) {
		t.Fatalf("an item without runs reads as %v", idle)
	}
}

func TestListItemsKeepsAKindThatIsOffExceptFlashAndConfig(t *testing.T) {
	virsh := &countingVirsh{listErr: errors.New("ssh: could not resolve hostname")}
	h, st, _ := newItemsRouter(t, &fakeServiceDocker{listOut: []dockercli.ContainerInfo{runningContainer("plex")}}, virsh)
	s := mustSettings(t, st)
	s.ContainersEnabled, s.VMsEnabled, s.FilesEnabled = false, false, false
	s.ZFSEnabled, s.FlashEnabled, s.ConfigEnabled = false, false, false
	s.EverythingSchedule = "daily 05:00"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	seedTarget(t, st, "plex")
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "Windows11", Method: "graceful", IncludeInSchedule: true}); err != nil {
		t.Fatal(err)
	}
	docs, err := st.CreateFileSet(store.FileSet{Name: "Documents", Path: "documents", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	appdata, err := st.CreateZFSDataset(store.ZFSDataset{Dataset: "cache/appdata", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	rows, body := listItems(t, h)
	want := []string{"container/plex", "vm/Windows11", "files/" + docs.ID, "zfs/" + appdata.ID}
	if len(rows) != len(want) {
		t.Fatalf("listed %v, want only %v", rows, want)
	}
	for _, id := range want {
		row, ok := rows[id]
		if !ok {
			t.Fatalf("%s is missing from %v", id, rows)
		}
		if row["kindDisabled"] != true {
			t.Fatalf("%s does not say its kind is off: %v", id, row)
		}
		sched, _ := row["effectiveSchedule"].(map[string]any)
		if sched["kind"] != "none" || sched["reason"] != "domain-off" {
			t.Fatalf("%s reports the schedule %v with its kind off", id, sched)
		}
	}

	if virsh.listCalls != 0 {
		t.Fatalf("libvirt was asked %d times with the VMs kind off", virsh.listCalls)
	}
	if _, known := rows["vm/Windows11"]["installed"]; known {
		t.Fatalf("a VM libvirt was not asked about reports installed: %v", rows["vm/Windows11"])
	}
	if unlisted, _ := body["unlisted"].([]any); len(unlisted) != 0 {
		t.Fatalf("unlisted = %v, though no host failed to answer", unlisted)
	}
}

func TestListItemsFallsBackToTheStoredEntriesWithoutTheHosts(t *testing.T) {
	docker := &fakeServiceDocker{listErr: errors.New("docker: socket unreachable")}
	virsh := &countingVirsh{listErr: errors.New("ssh: could not resolve hostname")}
	h, st, _ := newItemsRouter(t, docker, virsh)
	enableEveryKind(t, st, nil)

	seedTarget(t, st, "plex")
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "Windows11", Method: "graceful", IncludeInSchedule: true}); err != nil {
		t.Fatal(err)
	}

	rows, body := listItems(t, h)
	for _, id := range []string{"container/plex", "vm/Windows11"} {
		row, ok := rows[id]
		if !ok {
			t.Fatalf("%s is missing from %v", id, rows)
		}
		if _, known := row["installed"]; known {
			t.Fatalf("%s reports installed = %v though its host did not answer", id, row["installed"])
		}
	}
	if got, want := body["unlisted"], []any{"container", "vm"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unlisted = %v, want %v", got, want)
	}
}

// runIDs reads the ids of a GET /api/runs answer in order.
func runIDs(t *testing.T, h http.Handler, query string) []string {
	t.Helper()
	w, body := doJSON(t, h, http.MethodGet, "/api/runs"+query, "")
	if w.Code != http.StatusOK || body["ok"] != true {
		t.Fatalf("GET /api/runs%s: %d %v", query, w.Code, body)
	}
	runs, ok := body["runs"].([]any)
	if !ok {
		t.Fatalf("GET /api/runs%s: runs is not an array: %v", query, body["runs"])
	}
	ids := make([]string, len(runs))
	for i, raw := range runs {
		row, _ := raw.(map[string]any)
		ids[i], _ = row["id"].(string)
	}
	return ids
}

func TestRunsOfOneItem(t *testing.T) {
	h, st, _ := newItemsRouter(t, &fakeServiceDocker{}, fakeVirsh{})

	plex := seedTarget(t, st, "plex")
	sonarr := seedTarget(t, st, "sonarr")
	vm, err := st.UpsertVMTarget(store.VMTarget{Name: "Windows 11", Method: "graceful"})
	if err != nil {
		t.Fatal(err)
	}
	docs, err := st.CreateFileSet(store.FileSet{Name: "Documents", Path: "documents", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	appdata, err := st.CreateZFSDataset(store.ZFSDataset{Dataset: "cache/appdata", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	plexBackup := seedRun(t, st, plex.ID, "backup", "success", store.RunMeta{})
	plexDump := seedRun(t, st, plex.ID, "dbdump", "failed", store.RunMeta{})
	plexRestore := seedRun(t, st, plex.ID, "restore", "success", store.RunMeta{})
	sonarrBackup := seedRun(t, st, sonarr.ID, "backup", "success", store.RunMeta{})
	vmBackup := seedRun(t, st, vm.ID, "backup", "success", store.RunMeta{})
	docsBackup := seedRun(t, st, docs.ID, "backup", "success", store.RunMeta{})
	zfsBackup := seedRun(t, st, appdata.ID, "backup", "success", store.RunMeta{})
	flashBackup := seedRun(t, st, store.FlashTargetID, "backup", "success", store.RunMeta{})
	configBackup := seedRun(t, st, store.ConfigTargetID, "backup", "success", store.RunMeta{})

	item := func(kind, key string) string {
		return "?" + url.Values{"itemKind": {kind}, "itemKey": {key}}.Encode()
	}
	cases := []struct {
		name, query string
		want        []string
	}{
		{"a container, newest first, every kind of run", item("container", "plex"), []string{plexRestore, plexDump, plexBackup}},
		{"a limit", item("container", "plex") + "&limit=2", []string{plexRestore, plexDump}},
		{"another container", item("container", "sonarr"), []string{sonarrBackup}},
		{"a VM by its libvirt name", item("vm", "Windows 11"), []string{vmBackup}},
		{"a folder set by its id", item("files", docs.ID), []string{docsBackup}},
		{"a ZFS item by its id", item("zfs", appdata.ID), []string{zfsBackup}},
		{"flash", item("flash", "flash"), []string{flashBackup}},
		{"config", item("config", "config"), []string{configBackup}},
		{"a container without an entry", item("container", "fresh"), []string{}},
		{"a folder set asked for by its name", item("files", "Documents"), []string{}},
		{"flash under another key", item("flash", "config"), []string{}},
	}
	for _, c := range cases {
		if got := runIDs(t, h, c.query); !reflect.DeepEqual(got, c.want) {
			t.Fatalf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	if got := runIDs(t, h, ""); len(got) != 9 {
		t.Fatalf("without a filter the list holds %d runs, want all 9", len(got))
	}
	if got := runIDs(t, h, "?limit=4"); len(got) != 4 {
		t.Fatalf("a limit without a filter returned %d runs, want 4", len(got))
	}

	_, body := doJSON(t, h, http.MethodGet, "/api/runs"+item("container", "plex")+"&limit=1", "")
	row, _ := body["runs"].([]any)[0].(map[string]any)
	if row["target"] != "plex" || row["domain"] != "container" {
		t.Fatalf("a filtered run reads as %v, want the target plex in the container domain", row)
	}

	for _, query := range []string{item("nas", "plex"), "?itemKey=plex"} {
		w, body := doJSON(t, h, http.MethodGet, "/api/runs"+query, "")
		if w.Code != http.StatusBadRequest || body["ok"] != false {
			t.Fatalf("GET /api/runs%s: %d %v, want a 400", query, w.Code, body)
		}
	}
}

func TestRunViewCarriesSourceBytesAndGroup(t *testing.T) {
	h, st, _ := newItemsRouter(t, &fakeServiceDocker{}, fakeVirsh{})
	plex := seedTarget(t, st, "plex")

	pass := seedRun(t, st, store.EverythingTargetID, "backup", "success", store.RunMeta{})
	child, err := st.StartRunWith(plex.ID, "backup", store.RunMeta{GroupID: pass})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRunMeasured(child, "success", "snap", 10, "", &store.RunMetrics{SourceBytes: 4096}, ""); err != nil {
		t.Fatal(err)
	}

	_, body := doJSON(t, h, http.MethodGet, "/api/runs", "")
	runs, _ := body["runs"].([]any)
	byID := map[string]map[string]any{}
	for _, raw := range runs {
		row, _ := raw.(map[string]any)
		id, _ := row["id"].(string)
		byID[id] = row
	}
	if got := byID[child]; got["sourceBytes"] != float64(4096) || got["groupId"] != pass {
		t.Fatalf("the child run reads as %v, want sourceBytes 4096 and groupId %q", got, pass)
	}
	parent := byID[pass]
	if size, present := parent["sourceBytes"]; !present || size != nil {
		t.Fatalf("an unmeasured run reports sourceBytes = %v, want null", size)
	}
	if parent["groupId"] != "" {
		t.Fatalf("a run outside a pass reports groupId = %v", parent["groupId"])
	}
}

func TestItemRoutesRequireSession(t *testing.T) {
	h, _, _ := newItemsRouter(t, &fakeServiceDocker{}, fakeVirsh{})
	cookie := loginCookie(t, h, "correct horse battery staple")

	for _, path := range []string{"/api/items", "/api/runs?itemKind=container&itemKey=plex"} {
		if w, _ := doJSON(t, h, http.MethodGet, path, ""); w.Code != http.StatusUnauthorized {
			t.Fatalf("GET %s: status %d, want 401 without a session", path, w.Code)
		}
		r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s: status %d with a session, want 200", path, w.Code)
		}
	}
}
