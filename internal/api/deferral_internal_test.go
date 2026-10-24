package api

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// retries records the backups the deferral loop starts instead of running
// them.
type retries struct {
	mu   sync.Mutex
	got  []string
	kept []bool
}

func (r *retries) names() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.got...)
}

func newDeferralService(t *testing.T) (*Service, *store.Repo, *webhookRecorder, *retries) {
	t.Helper()
	s, st := newSyncTestService(t)
	s.cfg.AppKey = strings.Repeat("a", 64)
	wh := newWebhookRecorder(t)
	if err := s.SetNotifyConfig(wh.config("failure")); err != nil {
		t.Fatal(err)
	}
	r := &retries{}
	s.deferralState().backup = func(ctx context.Context, name string) error {
		r.mu.Lock()
		r.got = append(r.got, name)
		r.kept = append(r.kept, backsUpRegardless(ctx))
		r.mu.Unlock()
		return nil
	}
	return s, st, wh, r
}

func deferralTarget(t *testing.T, st *store.Repo, name string) store.Target {
	t.Helper()
	tg, err := st.UpsertTarget(store.Target{ContainerName: name, IncludeInSchedule: true})
	if err != nil {
		t.Fatal(err)
	}
	return tg
}

func setPolicy(t *testing.T, st *store.Repo, tg store.Target, change func(*store.DeferPolicy)) {
	t.Helper()
	p := store.DefaultDeferPolicy(tg.ID)
	change(&p)
	if err := st.SetDeferPolicy(p); err != nil {
		t.Fatal(err)
	}
}

func deferredRun(t *testing.T, s *Service, st *store.Repo, tg store.Target) string {
	t.Helper()
	id, err := s.startRun(context.Background(), tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(id, backup.StatusDeferred, "", 0, "Plex has 1 active session"); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestTheFirstDeferralTellsOnceAndLaterOnesOnlyMoveTheNextTry(t *testing.T) {
	s, st, wh, _ := newDeferralService(t)
	tg := deferralTarget(t, st, "plex")
	now := time.Now()
	run := deferredRun(t, s, st, tg)
	s.deferBackup(context.Background(), tg, run, "Plex has 1 active session", now)
	s.deferBackup(context.Background(), tg, "", "Plex has 2 active sessions", now.Add(30*time.Minute))
	s.deferBackup(context.Background(), tg, "", "Plex has 2 active sessions", now.Add(time.Hour))

	msgs := wh.messages()
	if len(msgs) != 1 || !strings.Contains(msgs[0], `"plex" was deferred`) || !strings.Contains(msgs[0], "gives up") {
		t.Fatalf("messages = %q, want one about the deferral", msgs)
	}
	d, ok, err := st.GetDeferral(tg.ID)
	if err != nil || !ok {
		t.Fatalf("no wait stored: %v", err)
	}
	if d.Tries != 3 || d.Since != now.Unix() || d.Reason != "Plex has 2 active sessions" {
		t.Fatalf("wait = %+v", d)
	}
	if d.NextTry != now.Add(90*time.Minute).Unix() {
		t.Fatalf("next try %d, want 30 minutes after the last try", d.NextTry)
	}
	r, err := st.GetRun(run)
	if err != nil || r.RetryAt == nil || *r.RetryAt != now.Add(30*time.Minute).Unix() {
		t.Fatalf("run = %+v, %v; want it to name its next try", r, err)
	}
}

func TestADeferralInASummaryRoundIsNamedInTheSummary(t *testing.T) {
	s, st, wh, _ := newDeferralService(t)
	c, err := s.NotifyConfig()
	if err != nil {
		t.Fatal(err)
	}
	c.ScheduledSummary = true
	if err := s.SetNotifyConfig(c); err != nil {
		t.Fatal(err)
	}
	tg := deferralTarget(t, st, "plex")
	ctx := withRoundTally(notify.WithMessagesSuppressed(WithBulkReplicateSuppressed(context.Background())))
	s.deferBackup(ctx, tg, "", "Plex has 1 active session", time.Now())
	if got := wh.messages(); len(got) != 0 {
		t.Fatalf("a deferral in summary mode sent %q on its own", got)
	}
	s.ScheduledNotifyResult(ctx, "containers", 3, 0, nil)
	got := wh.messages()
	if len(got) != 1 || !strings.Contains(got[0], "1 backup waits for its app") || !strings.Contains(got[0], "plex: Plex has 1 active session") {
		t.Fatalf("summary = %q", got)
	}
}

func TestAWaitSurvivesARestartAndIsTriedWhenDue(t *testing.T) {
	s, st, _, _ := newDeferralService(t)
	tg := deferralTarget(t, st, "plex")
	now := time.Now()
	s.deferBackup(context.Background(), tg, "", "busy", now)

	// A new process on the same database.
	s2 := &Service{store: st, cfg: s.cfg}
	r := &retries{}
	s2.deferralState().backup = func(_ context.Context, name string) error {
		r.mu.Lock()
		r.got = append(r.got, name)
		r.mu.Unlock()
		return nil
	}
	s2.tryDeferrals(context.Background(), now.Add(10*time.Minute))
	s2.deferralState().wg.Wait()
	if len(r.names()) != 0 {
		t.Fatalf("a try came before it was due: %v", r.names())
	}
	s2.tryDeferrals(context.Background(), now.Add(31*time.Minute))
	s2.deferralState().wg.Wait()
	if got := r.names(); len(got) != 1 || got[0] != "plex" {
		t.Fatalf("tries = %v, want plex once", got)
	}
}

func TestATryOutsideTheWindowWaitsForItsStart(t *testing.T) {
	s, st, _, r := newDeferralService(t)
	tg := deferralTarget(t, st, "plex")
	setPolicy(t, st, tg, func(p *store.DeferPolicy) { p.WindowFrom, p.WindowTo, p.MaxHours = "00:00", "08:00", 48 })
	noon := time.Date(2026, time.October, 24, 12, 0, 0, 0, time.Local)
	if err := st.SaveDeferral(store.Deferral{TargetID: tg.ID, Container: "plex", Since: noon.Add(-time.Hour).Unix(), NextTry: noon.Unix(), Tries: 1}); err != nil {
		t.Fatal(err)
	}
	s.tryDeferrals(context.Background(), noon)
	s.deferralState().wg.Wait()
	if len(r.names()) != 0 {
		t.Fatalf("a try ran outside the window: %v", r.names())
	}
	d, _, _ := st.GetDeferral(tg.ID)
	if want := time.Date(2026, time.October, 25, 0, 0, 0, 0, time.Local).Unix(); d.NextTry != want {
		t.Fatalf("next try %v, want midnight", time.Unix(d.NextTry, 0))
	}
}

func TestARetryWaitsWhileAnotherBackupRuns(t *testing.T) {
	s, st, _, r := newDeferralService(t)
	tg := deferralTarget(t, st, "plex")
	now := time.Now()
	s.deferBackup(context.Background(), tg, "", "busy", now.Add(-time.Hour))
	s.batchActive.Store(true)
	s.tryDeferrals(context.Background(), now)
	s.deferralState().wg.Wait()
	if len(r.names()) != 0 {
		t.Fatalf("a retry ran beside a running backup: %v", r.names())
	}
	s.batchActive.Store(false)
	s.tryDeferrals(context.Background(), now)
	s.deferralState().wg.Wait()
	if len(r.names()) != 1 {
		t.Fatalf("the retry did not run once the other backup was done: %v", r.names())
	}
	if s.batchActive.Load() {
		t.Fatal("the retry kept the guard a click on Back up now needs")
	}
}

func TestAtTheDeadlineAWaitThatGivesUpIsAFailureUnderNeedsYou(t *testing.T) {
	s, st, wh, r := newDeferralService(t)
	tg := deferralTarget(t, st, "plex")
	now := time.Now()
	s.deferBackup(context.Background(), tg, "", "Plex has 1 active session", now.Add(-13*time.Hour))
	if !s.deferralHolds("containers", now.Add(-2*time.Hour).Unix()) {
		t.Fatal("a wait within its deadline does not hold the overdue alert")
	}
	s.tryDeferrals(context.Background(), now)
	s.deferralState().wg.Wait()
	if len(r.names()) != 0 {
		t.Fatalf("a backup ran after giving up: %v", r.names())
	}
	if _, ok, _ := st.GetDeferral(tg.ID); ok {
		t.Fatal("the wait is still stored")
	}
	run, err := st.LastRunForTarget(tg.ID)
	if err != nil || run == nil || run.Status != "failed" || run.Error != store.ReasonDeferralGaveUp {
		t.Fatalf("run = %+v, %v", run, err)
	}
	if !s.deferralGaveUp(tg.ID) {
		t.Fatal("coverage does not count the container as unprotected")
	}
	if s.deferralHolds("containers", now.Unix()) {
		t.Fatal("a wait that ended still holds the overdue alert")
	}
	msgs := wh.messages()
	if len(msgs) != 2 || !strings.Contains(msgs[1], "gave up") {
		t.Fatalf("messages = %q, want the deferral and its end", msgs)
	}
}

func TestAtTheDeadlineAWaitSetToBackUpAnywayRunsRegardless(t *testing.T) {
	s, st, _, r := newDeferralService(t)
	tg := deferralTarget(t, st, "plex")
	setPolicy(t, st, tg, func(p *store.DeferPolicy) { p.AtDeadline = store.AtDeadlineBackUp })
	now := time.Now()
	s.deferBackup(context.Background(), tg, "", "busy", now.Add(-13*time.Hour))
	s.tryDeferrals(context.Background(), now)
	s.deferralState().wg.Wait()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.got) != 1 || !r.kept[0] {
		t.Fatalf("tries = %v, regardless = %v; want one that backs up regardless", r.got, r.kept)
	}
}

func TestAWaitEndsWhenItsBackupRuns(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		shutdown bool
		kept     bool
		told     bool
	}{
		{name: "it went through", told: true},
		{name: "it failed for a reason of its own", err: errors.New("disk full")},
		{name: "it was deferred again", err: &backup.DeferredError{Reason: "busy"}, kept: true},
		{name: "BombVault shut down meanwhile", err: context.Canceled, shutdown: true, kept: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, st, wh, _ := newDeferralService(t)
			tg := deferralTarget(t, st, "plex")
			s.deferBackup(context.Background(), tg, "", "busy", time.Now())
			s.shuttingDown.Store(tc.shutdown)
			s.settleDeferral(context.Background(), tg.ID, "plex", backup.Summary{SnapshotID: "deadbeef12345678", Bytes: 1 << 20}, tc.err)
			if _, ok, _ := st.GetDeferral(tg.ID); ok != tc.kept {
				t.Fatalf("wait kept = %v, want %v", ok, tc.kept)
			}
			msgs := wh.messages()
			told := len(msgs) == 2 && strings.Contains(msgs[1], "went through") && strings.Contains(msgs[1], "deadbeef")
			if told != tc.told || (!tc.told && len(msgs) != 1) {
				t.Fatalf("messages = %q", msgs)
			}
		})
	}
}

func TestAWaitThatLostItsScheduleEndsWithARunAndANotice(t *testing.T) {
	s, st, wh, r := newDeferralService(t)
	tg := deferralTarget(t, st, "plex")
	s.deferBackup(WithScheduledBackup(context.Background()), tg, "", "busy", time.Now().Add(-time.Hour))
	if err := st.SetInclude("plex", false); err != nil {
		t.Fatal(err)
	}
	s.tryDeferrals(context.Background(), time.Now())
	s.deferralState().wg.Wait()
	if _, ok, _ := st.GetDeferral(tg.ID); ok || len(r.names()) != 0 {
		t.Fatalf("a wait of a container off the schedule was tried: %v", r.names())
	}
	run, err := st.LastRunForTarget(tg.ID)
	if err != nil || run == nil || run.Status != statusSkipped || run.Error != store.ReasonDeferralUnscheduled {
		t.Fatalf("run = %+v, %v; want the end of the wait recorded", run, err)
	}
	msgs := wh.messages()
	if len(msgs) != 2 || !strings.Contains(msgs[1], "is not tried again") {
		t.Fatalf("messages = %q, want the deferral and its end", msgs)
	}
}

func TestAPolicyIsCheckedAndItsKeyStoredSealed(t *testing.T) {
	s, st, _, _ := newDeferralService(t)
	ok := DeferralPatch{RetryMin: 30, MaxHours: 12, AtDeadline: store.AtDeadlineGiveUp, Preset: "jellyfin", APIKey: " k3y ", FailOpen: true, NetMin: 5}
	for name, bad := range map[string]func(*DeferralPatch){
		"too short an interval": func(p *DeferralPatch) { p.RetryMin = 1 },
		"half a window":         func(p *DeferralPatch) { p.WindowFrom = "00:00" },
		"an empty window":       func(p *DeferralPatch) { p.WindowFrom, p.WindowTo = "03:00", "03:00" },
		"no time of day":        func(p *DeferralPatch) { p.WindowFrom, p.WindowTo = "midnight", "08:00" },
		"too long a wait":       func(p *DeferralPatch) { p.MaxHours = 500 },
		"an unknown outcome":    func(p *DeferralPatch) { p.AtDeadline = "panic" },
		"an unknown preset":     func(p *DeferralPatch) { p.Preset = "kodi" },
		"a traffic window of 0": func(p *DeferralPatch) { p.NetMin = 0 },
	} {
		p := ok
		bad(&p)
		if err := s.SetDeferPolicy(context.Background(), "jf", p); err == nil {
			t.Errorf("%s was taken", name)
		}
	}
	if err := s.SetDeferPolicy(context.Background(), "jf", ok); err != nil {
		t.Fatal(err)
	}
	tg, err := st.GetTargetByContainer("jf")
	if err != nil {
		t.Fatal(err)
	}
	p, err := st.DeferPolicyOf(tg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(p.APIKeyEnc), "k3y") {
		t.Fatal("the API key is stored in the clear")
	}
	if plain, err := secret.Decrypt(s.cfg.AppKey, p.APIKeyEnc); err != nil || string(plain) != "k3y" {
		t.Fatalf("key = %q, %v", plain, err)
	}
	// A save without a key keeps the stored one, and a clear removes it.
	ok.APIKey = ""
	if err := s.SetDeferPolicy(context.Background(), "jf", ok); err != nil {
		t.Fatal(err)
	}
	if p, _ := st.DeferPolicyOf(tg.ID); len(p.APIKeyEnc) == 0 {
		t.Fatal("a save without a key dropped the stored one")
	}
	ok.APIKeyClear = true
	if err := s.SetDeferPolicy(context.Background(), "jf", ok); err != nil {
		t.Fatal(err)
	}
	if p, _ := st.DeferPolicyOf(tg.ID); len(p.APIKeyEnc) != 0 {
		t.Fatal("the key was not removed")
	}
}

func TestANewPolicyMovesTheNextTryOfAWaitingBackup(t *testing.T) {
	s, st, _, _ := newDeferralService(t)
	tg := deferralTarget(t, st, "plex")
	s.deferBackup(context.Background(), tg, "", "busy", time.Now())
	before, _, _ := st.GetDeferral(tg.ID)
	p := DeferralPatch{RetryMin: 5, MaxHours: 12, AtDeadline: store.AtDeadlineGiveUp, FailOpen: true, NetMin: 5}
	if err := s.SetDeferPolicy(context.Background(), "plex", p); err != nil {
		t.Fatal(err)
	}
	after, _, _ := st.GetDeferral(tg.ID)
	if after.NextTry >= before.NextTry || after.NextTry > time.Now().Add(6*time.Minute).Unix() {
		t.Fatalf("next try %v, was %v; want it five minutes out", time.Unix(after.NextTry, 0), time.Unix(before.NextTry, 0))
	}
}
