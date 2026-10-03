package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func idleTarget(t *testing.T, st *store.Repo, name string, hours int) store.Target {
	t.Helper()
	tg, err := st.UpsertTarget(store.Target{ContainerName: name, IncludeInSchedule: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetIdleWaitHours(tg.ID, hours); err != nil {
		t.Fatal(err)
	}
	return tg
}

// load feeds polls ending at the current time, adding cpu CPU-nanoseconds and
// tx bytes to name before each one.
func load(s *Service, d *trafficDocker, name string, polls int, cpu, tx uint64) {
	loadUntil(s, d, name, polls, cpu, tx, time.Now())
}

func loadUntil(s *Service, d *trafficDocker, name string, polls int, cpu, tx uint64, end time.Time) {
	start := end.Add(-time.Duration(polls-1) * trafficPoll)
	for i := range polls {
		d.cpu[name] += cpu
		d.tx[name] += tx
		s.pollTraffic(context.Background(), d, start.Add(time.Duration(i)*trafficPoll))
	}
}

// hold offers one container to HoldForIdle as a domain run would and reports
// whether it was taken.
func hold(s *Service, tg store.Target, run func()) bool {
	return len(s.HoldForIdle([]store.Target{tg}, "domain", func([]string) { run() })) > 0
}

func fastIdleCheck(s *Service) {
	s.waits().every = 20 * time.Millisecond
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestAContainerWithoutAWaitIsNeverHeld(t *testing.T) {
	s, st, d := newTrafficService(t)
	tg := idleTarget(t, st, "db", 0)
	load(s, d, "db", 20, 9e9, 0)
	if hold(s, tg, func() {}) {
		t.Fatal("a container without a wait was held")
	}
}

func TestTheWatchMeasuresWaitingContainersWithTheThrottleOff(t *testing.T) {
	s, st, d := newTrafficService(t)
	idleTarget(t, st, "db", 2)
	load(s, d, "db", 3, 1e9, 0)
	if d.reads["db"] != 3 || d.reads["plex"] != 0 {
		t.Fatalf("reads = %v, want db on every poll and plex never", d.reads)
	}
	idle, why := s.appIdle(context.Background(), "db", time.Now())
	if idle || why != "measuring" {
		t.Fatalf("appIdle = %v, %q; want still measuring after 30s", idle, why)
	}
}

func TestABusyAppIsHeldAndItsWaitIsListed(t *testing.T) {
	s, st, d := newTrafficService(t)
	tg := idleTarget(t, st, "db", 2)
	load(s, d, "db", 20, 9e9, 0)
	before := time.Now().Unix()
	if !hold(s, tg, func() {}) {
		t.Fatal("a busy app was not held")
	}
	waits := s.IdleWaits()
	if len(waits) != 1 || waits[0].Name != "db" || waits[0].Reason != "cpu" || waits[0].Domain != "containers" {
		t.Fatalf("waits = %+v", waits)
	}
	if got := waits[0].Deadline - waits[0].Since; got != 2*3600 || waits[0].Since < before {
		t.Fatalf("wait %+v does not end two hours after it began", waits[0])
	}
	if !hold(s, tg, func() { t.Error("a second fire ran its own backup") }) {
		t.Fatal("a second fire for a waiting container was not absorbed")
	}
	if len(s.IdleWaits()) != 1 {
		t.Fatal("a second fire added a second wait")
	}
}

func TestAnIdleAppIsNotHeld(t *testing.T) {
	s, st, d := newTrafficService(t)
	tg := idleTarget(t, st, "db", 2)
	load(s, d, "db", 20, 1e8, 1000)
	if hold(s, tg, func() {}) {
		t.Fatalf("an idle app was held: %+v", s.IdleWaits())
	}
}

func TestAStreamingMediaServerWaitsUntilTheStreamCountsAsOver(t *testing.T) {
	s, st, d := newTrafficService(t)
	fastIdleCheck(s)
	tg := idleTarget(t, st, "plex", 2)
	// The stream ended two minutes ago, inside the five-minute hold.
	loadUntil(s, d, "plex", 4, 0, 5_000_000, time.Now().Add(-2*time.Minute))
	var ran atomic.Bool
	if !hold(s, tg, func() { ran.Store(true) }) {
		t.Fatal("a streaming media server was not held")
	}
	if w := s.IdleWaits(); len(w) != 1 || w[0].Reason != "streaming" {
		t.Fatalf("waits = %+v", w)
	}
	time.Sleep(100 * time.Millisecond)
	if ran.Load() {
		t.Fatal("the backup started while the stream still counted")
	}
	// A one-minute hold puts the stream behind it.
	cfg, _ := st.TrafficSettings()
	cfg.StreamHoldMin = 1
	if err := st.SetTrafficSettings(cfg); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the held backup", ran.Load)
	if len(s.IdleWaits()) != 0 {
		t.Fatal("the wait is still listed after its backup started")
	}
}

func TestABackupStillBusyAtItsDeadlineStartsAnyway(t *testing.T) {
	s, st, d := newTrafficService(t)
	fastIdleCheck(s)
	tg := idleTarget(t, st, "db", 1)
	load(s, d, "db", 20, 9e9, 0)
	var ran atomic.Bool
	g := &idleGroup{IdleWaitGroup: store.IdleWaitGroup{Key: "db", Members: []string{tg.ContainerName}, Since: time.Now().Unix(), Deadline: time.Now().Unix() - 1},
		run: func([]string) { ran.Store(true) }}
	s.waits().groups["db"] = g
	s.startWait(g)
	waitFor(t, "the backup at the deadline", ran.Load)
}

func TestSwitchingTheWaitOffStartsTheHeldBackup(t *testing.T) {
	s, st, d := newTrafficService(t)
	fastIdleCheck(s)
	tg := idleTarget(t, st, "db", 3)
	load(s, d, "db", 20, 9e9, 0)
	var ran atomic.Bool
	if !hold(s, tg, func() { ran.Store(true) }) {
		t.Fatal("not held")
	}
	if err := st.SetIdleWaitHours(tg.ID, 0); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the backup after the switch", ran.Load)
}

func TestOnlyAScheduledEverythingPassHoldsBusyApps(t *testing.T) {
	s, st, d := newTrafficService(t)
	tg := idleTarget(t, st, "db", 2)
	load(s, d, "db", 20, 9e9, 0)
	var held []string
	s.SetHeldContainerRun(func(names []string) { held = append(held, names...) })

	manual := s.holdEverythingContainers(context.Background(), []store.Target{tg})
	if len(manual) != 1 {
		t.Fatal("a manual pass held a container back")
	}
	scheduled := s.holdEverythingContainers(WithScheduledPass(context.Background()), []store.Target{tg})
	if len(scheduled) != 0 {
		t.Fatal("a scheduled pass did not hold the busy container")
	}
}

func TestContainerPatchSetsTheIdleWait(t *testing.T) {
	s, st, _ := newTrafficService(t)
	h := &Handler{store: st, svc: s}
	patch := func(hours int) map[string]any {
		body, _ := json.Marshal(map[string]any{"idleWaitHours": hours})
		req := jsonReq(http.MethodPatch, "/api/containers/db", bytes.NewReader(body))
		req.SetPathValue("name", "db")
		rec := httptest.NewRecorder()
		h.handlePatchContainer(rec, req)
		return decodeEnvelope(t, rec)
	}
	if env := patch(6); env["ok"] != true {
		t.Fatalf("patch failed: %v", env)
	}
	tg, err := st.GetTargetByContainer("db")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := st.IdleWaitHours(); got[tg.ID] != 6 {
		t.Fatalf("hours = %v", got)
	}
	if env := patch(25); env["ok"] != false {
		t.Fatalf("25 hours were accepted: %v", env)
	}
}

func TestIdleSettingsHandlersRoundTripAndRefuseNonsense(t *testing.T) {
	s, st, _ := newTrafficService(t)
	h := &Handler{store: st, svc: s}
	put := func(v idleView) map[string]any {
		body, _ := json.Marshal(v)
		rec := httptest.NewRecorder()
		h.handleSetIdle(rec, jsonReq(http.MethodPut, "/api/settings/idle", bytes.NewReader(body)))
		return decodeEnvelope(t, rec)
	}
	if env := put(idleView{CPUPct: 20, NetMbit: 3, QuietMin: 5}); env["ok"] != true {
		t.Fatalf("save failed: %v", env)
	}
	cfg, _ := st.TrafficSettings()
	if cfg.IdleCPUPct != 20 || cfg.IdleNetMbit != 3 || cfg.IdleQuietMin != 5 || cfg.StreamMbit != 2 {
		t.Fatalf("stored %+v", cfg)
	}
	for _, bad := range []idleView{{CPUPct: 0, NetMbit: 1, QuietMin: 1}, {CPUPct: 1, NetMbit: 0, QuietMin: 1}, {CPUPct: 1, NetMbit: 1, QuietMin: 61}} {
		if env := put(bad); env["ok"] != false {
			t.Fatalf("%+v accepted", bad)
		}
	}
}

func TestScheduleWaitingListsTheWaits(t *testing.T) {
	s, st, d := newTrafficService(t)
	tg := idleTarget(t, st, "db", 2)
	load(s, d, "db", 20, 9e9, 0)
	hold(s, tg, func() {})
	h := &Handler{store: st, svc: s}
	rec := httptest.NewRecorder()
	h.handleScheduleWaiting(rec, httptest.NewRequest(http.MethodGet, "/api/schedule/waiting", nil))
	env := decodeEnvelope(t, rec)
	waits := env["waiting"].([]any)
	if len(waits) != 1 || waits[0].(map[string]any)["name"] != "db" {
		t.Fatalf("waiting = %v", env["waiting"])
	}
	if !slices.Contains([]string{"cpu"}, waits[0].(map[string]any)["reason"].(string)) {
		t.Fatalf("reason = %v", waits[0])
	}
}

// stack puts containers into one compose project as Docker lists them.
func stack(d *trafficDocker, project string, names ...string) {
	for _, n := range names {
		d.infos = append(d.infos, dockercli.ContainerInfo{Name: n, Image: "alpine", Stack: project})
	}
}

func TestABusyMemberHoldsItsWholeStackUnderOneDeadline(t *testing.T) {
	s, st, d := newTrafficService(t)
	stack(d, "immich", "immich-server", "immich-db")
	server := idleTarget(t, st, "immich-server", 3)
	db := idleTarget(t, st, "immich-db", 0)
	lone := idleTarget(t, st, "radarr", 0)
	load(s, d, "immich-server", 20, 9e9, 0)

	held := s.HoldForIdle([]store.Target{server, db, lone}, "domain", func([]string) {})
	if !slices.Equal(held, []string{"immich-server", "immich-db"}) {
		t.Fatalf("held %v, want both stack members and not radarr", held)
	}
	waits := s.IdleWaits()
	if len(waits) != 2 {
		t.Fatalf("waits = %+v", waits)
	}
	for _, w := range waits {
		if w.Stack != "immich" || w.Busy != "immich-server" || w.Reason != "cpu" || w.Deadline-w.Since != 3*3600 {
			t.Fatalf("wait %+v", w)
		}
	}
}

func TestALaterFireOfAStackMemberJoinsTheWaitingStack(t *testing.T) {
	s, st, d := newTrafficService(t)
	stack(d, "immich", "immich-server", "immich-ml")
	server := idleTarget(t, st, "immich-server", 3)
	ml := idleTarget(t, st, "immich-ml", 0)
	load(s, d, "immich-server", 20, 9e9, 0)
	var ran [][]string
	s.HoldForIdle([]store.Target{server}, "item", func(names []string) { ran = append(ran, names) })
	if held := s.HoldForIdle([]store.Target{ml}, "item", func([]string) { t.Error("the second fire runs on its own") }); !slices.Equal(held, []string{"immich-ml"}) {
		t.Fatalf("held %v", held)
	}
	groups, _ := st.ListIdleWaitGroups()
	if len(groups) != 1 || !slices.Equal(groups[0].Members, []string{"immich-server", "immich-ml"}) {
		t.Fatalf("stored %+v", groups)
	}
}

func TestAWaitIsStoredWhileItLastsAndClearedWhenItRuns(t *testing.T) {
	s, st, d := newTrafficService(t)
	fastIdleCheck(s)
	tg := idleTarget(t, st, "db", 2)
	load(s, d, "db", 20, 9e9, 0)
	var ran atomic.Bool
	if !hold(s, tg, func() { ran.Store(true) }) {
		t.Fatal("not held")
	}
	groups, _ := st.ListIdleWaitGroups()
	if len(groups) != 1 || groups[0].Trigger != "domain" || !slices.Equal(groups[0].Members, []string{"db"}) {
		t.Fatalf("stored %+v", groups)
	}
	// Raising the CPU limit makes the busy readings count as idle.
	cfg, _ := st.TrafficSettings()
	cfg.IdleCPUPct = 1000
	if err := st.SetTrafficSettings(cfg); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the held backup", ran.Load)
	if groups, _ := st.ListIdleWaitGroups(); len(groups) != 0 {
		t.Fatalf("still stored after it ran: %+v", groups)
	}
}

func TestAWaitResumesAfterARestartWithItsDeadline(t *testing.T) {
	s, st, d := newTrafficService(t)
	fastIdleCheck(s)
	idleTarget(t, st, "db", 2)
	// Two hours from half an hour before the restart.
	since := time.Now().Add(-30 * time.Minute).Unix()
	deadline := since + 2*3600
	if err := st.SaveIdleWaitGroup(store.IdleWaitGroup{Key: "db", Members: []string{"db"}, Trigger: "domain", Since: since, Deadline: deadline}); err != nil {
		t.Fatal(err)
	}
	load(s, d, "db", 20, 9e9, 0)
	ran := make(chan []string, 1)
	s.SetHeldContainerRun(func(names []string) { ran <- names })
	s.ResumeIdleWaits()
	w := s.IdleWaits()
	if len(w) != 1 || w[0].Deadline != deadline || w[0].Since != since {
		t.Fatalf("resumed waits = %+v", w)
	}
	s.StartIdleWaits()
	time.Sleep(100 * time.Millisecond)
	select {
	case <-ran:
		t.Fatal("the resumed backup ran while the app was busy")
	default:
	}
	cfg, _ := st.TrafficSettings()
	cfg.IdleCPUPct = 1000
	if err := st.SetTrafficSettings(cfg); err != nil {
		t.Fatal(err)
	}
	select {
	case names := <-ran:
		if !slices.Equal(names, []string{"db"}) {
			t.Fatalf("ran %v", names)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the resumed backup never ran")
	}
}

func TestAWaitThatEndedDuringTheRestartRunsOnceTheStartIsDone(t *testing.T) {
	s, st, _ := newTrafficService(t)
	fastIdleCheck(s)
	idleTarget(t, st, "db", 2)
	if err := st.SaveIdleWaitGroup(store.IdleWaitGroup{Key: "db", Members: []string{"db"}, Since: 1, Deadline: time.Now().Unix() - 60}); err != nil {
		t.Fatal(err)
	}
	var ran atomic.Bool
	s.SetHeldContainerRun(func([]string) { ran.Store(true) })
	s.ResumeIdleWaits()
	time.Sleep(100 * time.Millisecond)
	if ran.Load() {
		t.Fatal("the overdue backup ran before the start was done")
	}
	if w := s.IdleWaits(); len(w) != 1 || w[0].Name != "db" {
		t.Fatalf("waits = %+v, want db listed until it starts", w)
	}
	s.StartIdleWaits()
	waitFor(t, "the overdue backup", ran.Load)
	if groups, _ := st.ListIdleWaitGroups(); len(groups) != 0 {
		t.Fatalf("still stored: %+v", groups)
	}
}

func TestAResumedWaitLooksAtItsAppOnlyOnceTheStartIsDone(t *testing.T) {
	s, st, d := newTrafficService(t)
	fastIdleCheck(s)
	idleTarget(t, st, "db", 2)
	if err := st.SaveIdleWaitGroup(store.IdleWaitGroup{Key: "db", Members: []string{"db"}, Since: 1, Deadline: time.Now().Unix() + 3600}); err != nil {
		t.Fatal(err)
	}
	// A container that cannot be read counts as idle.
	d.down["db"] = true
	s.pollTraffic(context.Background(), d, time.Now())
	var ran atomic.Bool
	s.SetHeldContainerRun(func([]string) { ran.Store(true) })
	s.ResumeIdleWaits()
	time.Sleep(100 * time.Millisecond)
	if ran.Load() {
		t.Fatal("the resumed backup ran before the start was done")
	}
	s.StartIdleWaits()
	waitFor(t, "the resumed backup", ran.Load)
}

func TestAContainerTakenOffTheScheduleIsDroppedFromItsResumedWait(t *testing.T) {
	s, st, _ := newTrafficService(t)
	idleTarget(t, st, "db", 2)
	idleTarget(t, st, "app", 2)
	if err := st.SetInclude("app", false); err != nil {
		t.Fatal(err)
	}
	for _, g := range []store.IdleWaitGroup{
		{Key: "stack:x", Stack: "x", Members: []string{"db", "app"}, Since: 1, Deadline: time.Now().Unix() + 3600},
		{Key: "gone", Members: []string{"gone"}, Since: 1, Deadline: time.Now().Unix() + 3600},
	} {
		if err := st.SaveIdleWaitGroup(g); err != nil {
			t.Fatal(err)
		}
	}
	s.SetHeldContainerRun(func(names []string) { t.Errorf("ran %v", names) })
	s.ResumeIdleWaits()
	if w := s.IdleWaits(); len(w) != 1 || w[0].Name != "db" {
		t.Fatalf("waits = %+v, want db alone", w)
	}
	groups, _ := st.ListIdleWaitGroups()
	if len(groups) != 1 || !slices.Equal(groups[0].Members, []string{"db"}) {
		t.Fatalf("stored %+v", groups)
	}
}

func TestMCPStatusListsTheBackupsWaitingForAnIdleApp(t *testing.T) {
	h, _, _, _ := newMCPGateHandler(t)
	h.svc.waits().groups["stack:immich"] = &idleGroup{
		IdleWaitGroup: store.IdleWaitGroup{Key: "stack:immich", Stack: "immich", Members: []string{"immich-db", "immich-server"}, Since: 10, Deadline: 3610},
		busy:          "immich-server", reason: "streaming",
	}
	ctx := withMCPCaller(context.Background(), mcpCaller{KeyID: "0b7e", Hint: "x9Qa"})
	res, _ := h.toolGetStatus(ctx, &mcp.CallToolRequest{})
	if res.IsError {
		t.Fatalf("get_status: %v", res.StructuredContent)
	}
	body, _ := json.Marshal(res.StructuredContent)
	var got struct {
		Waiting []map[string]any `json:"waitingForIdle"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Waiting) != 2 {
		t.Fatalf("waitingForIdle = %v", got.Waiting)
	}
	w := got.Waiting[0]
	if w["item"] != "immich-db" || w["stack"] != "immich" || w["busyItem"] != "immich-server" || w["reason"] != "streaming" || w["deadline"] != float64(3610) || w["domain"] != "containers" {
		t.Fatalf("first wait = %v", w)
	}
}

// A wait ends with the process and stays stored, so the next start resumes it
// instead of backing up during the shutdown.
func TestAWaitEndsWithTheServiceAndStaysStored(t *testing.T) {
	s, st, d := newTrafficService(t)
	fastIdleCheck(s)
	tg := idleTarget(t, st, "db", 2)
	load(s, d, "db", 20, 9e9, 0)
	if !hold(s, tg, func() { t.Error("the held backup ran at shutdown") }) {
		t.Fatal("not held")
	}
	s.EndDetachedWork()
	ended := make(chan struct{})
	go func() {
		s.waits().wg.Wait()
		close(ended)
	}()
	select {
	case <-ended:
	case <-time.After(2 * time.Second):
		t.Fatal("the wait outlived the service")
	}
	if groups, _ := st.ListIdleWaitGroups(); len(groups) != 1 {
		t.Fatalf("stored %+v, want the wait kept for the next start", groups)
	}
}

// A wait that ends while a newer one took its key leaves the newer one alone,
// in memory and in the store.
func TestAnEndedWaitLeavesANewerWaitUnderItsKeyAlone(t *testing.T) {
	s, st, _ := newTrafficService(t)
	old := &idleGroup{IdleWaitGroup: store.IdleWaitGroup{Key: "db", Members: []string{"db"}, Since: 1, Deadline: 2}}
	newer := &idleGroup{IdleWaitGroup: store.IdleWaitGroup{Key: "db", Members: []string{"db"}, Since: 5, Deadline: 9}}
	s.waits().groups["db"] = newer
	if err := st.SaveIdleWaitGroup(newer.IdleWaitGroup); err != nil {
		t.Fatal(err)
	}
	if _, ended := s.endWait(old); ended {
		t.Fatal("the old wait ended the newer one")
	}
	if s.waits().groups["db"] != newer {
		t.Fatal("the newer wait left the list")
	}
	if groups, _ := st.ListIdleWaitGroups(); len(groups) != 1 || groups[0].Since != 5 {
		t.Fatalf("stored %+v, want the newer wait", groups)
	}
	members, ended := s.endWait(newer)
	if !ended || !slices.Equal(members, []string{"db"}) {
		t.Fatalf("endWait = %v, %v", members, ended)
	}
	if groups, _ := st.ListIdleWaitGroups(); len(groups) != 0 {
		t.Fatalf("still stored: %+v", groups)
	}
}

func TestSwitchingTheContainersOffDropsTheWaits(t *testing.T) {
	s, st, d := newTrafficService(t)
	fastIdleCheck(s)
	tg := idleTarget(t, st, "db", 2)
	load(s, d, "db", 20, 9e9, 0)
	if !hold(s, tg, func() { t.Error("a switched-off domain backed up") }) {
		t.Fatal("not held")
	}
	settings, _ := st.GetSettings()
	settings.ContainersEnabled = false
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the wait to go", func() bool { return len(s.IdleWaits()) == 0 })
	if groups, _ := st.ListIdleWaitGroups(); len(groups) != 0 {
		t.Fatalf("still stored: %+v", groups)
	}
}

// Switching the domain schedule off drops what its runs held back. A container
// with its own cadence keeps waiting.
func TestSwitchingTheScheduleOffDropsTheWaitsOfItsRuns(t *testing.T) {
	s, st, d := newTrafficService(t)
	fastIdleCheck(s)
	db := idleTarget(t, st, "db", 2)
	app := idleTarget(t, st, "app", 2)
	load(s, d, "db", 20, 9e9, 0)
	load(s, d, "app", 20, 9e9, 0)
	never := func([]string) { t.Error("a dropped wait backed up") }
	s.HoldForIdle([]store.Target{db}, "domain", never)
	s.HoldForIdle([]store.Target{app}, "item", never)
	settings, _ := st.GetSettings()
	settings.ContainersSchedule = "off"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the domain run's wait to go", func() bool { return len(s.IdleWaits()) == 1 })
	if w := s.IdleWaits(); w[0].Name != "app" {
		t.Fatalf("waits = %+v, want app", w)
	}
}

func TestAWaitOfASwitchedOffDomainIsNotResumed(t *testing.T) {
	s, st, _ := newTrafficService(t)
	idleTarget(t, st, "db", 2)
	if err := st.SaveIdleWaitGroup(store.IdleWaitGroup{Key: "db", Members: []string{"db"}, Trigger: "domain", Since: 1, Deadline: time.Now().Unix() - 60}); err != nil {
		t.Fatal(err)
	}
	settings, _ := st.GetSettings()
	settings.ContainersSchedule = "off"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	s.SetHeldContainerRun(func(names []string) { t.Errorf("ran %v", names) })
	s.ResumeIdleWaits()
	if w := s.IdleWaits(); len(w) != 0 {
		t.Fatalf("waits = %+v", w)
	}
	if groups, _ := st.ListIdleWaitGroups(); len(groups) != 0 {
		t.Fatalf("still stored: %+v", groups)
	}
}

func TestLoweringTheWaitShortensTheDeadline(t *testing.T) {
	s, st, d := newTrafficService(t)
	fastIdleCheck(s)
	tg := idleTarget(t, st, "db", 3)
	load(s, d, "db", 20, 9e9, 0)
	if !hold(s, tg, func() {}) {
		t.Fatal("not held")
	}
	since := s.IdleWaits()[0].Since
	if err := st.SetIdleWaitHours(tg.ID, 1); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the shorter deadline", func() bool {
		w := s.IdleWaits()
		return len(w) == 1 && w[0].Deadline == since+3600
	})
	if groups, _ := st.ListIdleWaitGroups(); len(groups) != 1 || groups[0].Deadline != since+3600 {
		t.Fatalf("stored %+v", groups)
	}
}

// Docker is asked before the waits are locked, so a slow daemon holds up the
// run that asks and not everybody reading the list.
func TestASlowDockerDoesNotHoldTheWaitListUp(t *testing.T) {
	s, st, d := newTrafficService(t)
	tg := idleTarget(t, st, "db", 2)
	load(s, d, "db", 20, 9e9, 0)
	d.listGate = make(chan struct{})
	held := make(chan bool, 1)
	go func() { held <- hold(s, tg, func() {}) }()
	time.Sleep(50 * time.Millisecond)
	listed := make(chan struct{})
	go func() {
		s.IdleWaits()
		close(listed)
	}()
	select {
	case <-listed:
	case <-time.After(time.Second):
		t.Error("listing the waits waited for Docker")
	}
	close(d.listGate)
	if !<-held {
		t.Fatal("not held")
	}
}
