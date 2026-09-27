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

func fastIdleCheck(t *testing.T) {
	t.Helper()
	old := idleCheck
	idleCheck = 20 * time.Millisecond
	t.Cleanup(func() { idleCheck = old })
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
	if s.HoldForIdle(tg, func() {}) {
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
	idle, why := s.appIdle("db", time.Now())
	if idle || why != "measuring" {
		t.Fatalf("appIdle = %v, %q; want still measuring after 30s", idle, why)
	}
}

func TestABusyAppIsHeldAndItsWaitIsListed(t *testing.T) {
	s, st, d := newTrafficService(t)
	tg := idleTarget(t, st, "db", 2)
	load(s, d, "db", 20, 9e9, 0)
	before := time.Now().Unix()
	if !s.HoldForIdle(tg, func() {}) {
		t.Fatal("a busy app was not held")
	}
	waits := s.IdleWaits()
	if len(waits) != 1 || waits[0].Name != "db" || waits[0].Reason != "cpu" || waits[0].Domain != "containers" {
		t.Fatalf("waits = %+v", waits)
	}
	if got := waits[0].Deadline - waits[0].Since; got != 2*3600 || waits[0].Since < before {
		t.Fatalf("wait %+v does not end two hours after it began", waits[0])
	}
	if !s.HoldForIdle(tg, func() { t.Error("a second fire ran its own backup") }) {
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
	if s.HoldForIdle(tg, func() {}) {
		t.Fatalf("an idle app was held: %+v", s.IdleWaits())
	}
}

func TestAStreamingMediaServerWaitsUntilTheStreamCountsAsOver(t *testing.T) {
	fastIdleCheck(t)
	s, st, d := newTrafficService(t)
	tg := idleTarget(t, st, "plex", 2)
	// The stream ended two minutes ago, inside the five-minute hold.
	loadUntil(s, d, "plex", 4, 0, 5_000_000, time.Now().Add(-2*time.Minute))
	var ran atomic.Bool
	if !s.HoldForIdle(tg, func() { ran.Store(true) }) {
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
	fastIdleCheck(t)
	s, st, d := newTrafficService(t)
	tg := idleTarget(t, st, "db", 1)
	load(s, d, "db", 20, 9e9, 0)
	var ran atomic.Bool
	item := &IdleWait{Domain: "containers", Name: "db", Since: time.Now().Unix(), Deadline: time.Now().Unix() - 1}
	s.waits().items["db"] = item
	go s.waitForIdle(tg.ID, item, func() { ran.Store(true) })
	waitFor(t, "the backup at the deadline", ran.Load)
}

func TestSwitchingTheWaitOffStartsTheHeldBackup(t *testing.T) {
	fastIdleCheck(t)
	s, st, d := newTrafficService(t)
	tg := idleTarget(t, st, "db", 3)
	load(s, d, "db", 20, 9e9, 0)
	var ran atomic.Bool
	if !s.HoldForIdle(tg, func() { ran.Store(true) }) {
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
	s.SetHeldContainerRun(func(name string) { held = append(held, name) })

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
	s.HoldForIdle(tg, func() {})
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
