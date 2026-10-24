package api

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/activity"
	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// DeferredBackup is a container backup that waits while its app is busy and
// is tried again on its own.
type DeferredBackup struct {
	Domain string `json:"domain"`
	Name   string `json:"name"`
	// Reason is why the last try was deferred.
	Reason   string `json:"reason"`
	Since    int64  `json:"since"`
	NextTry  int64  `json:"nextTry"`
	Deadline int64  `json:"deadline"`
	// AtDeadline is what happens at the deadline: store.AtDeadlineGiveUp or
	// store.AtDeadlineBackUp.
	AtDeadline string `json:"atDeadline"`
	Tries      int    `json:"tries"`
}

// deferralTick is how often the waiting backups are looked at.
const deferralTick = 30 * time.Second

type deferrals struct {
	mu sync.Mutex
	// trying holds the targets whose retry runs now.
	trying map[string]bool
	wake   chan struct{}
	pauses activity.Pauses
	// backup backs up the container a retry is for.
	backup func(ctx context.Context, name string) error
	// wg counts the loop and the retries, so a test can wait for them.
	wg sync.WaitGroup
}

func (s *Service) deferralState() *deferrals {
	s.deferralOnce.Do(func() {
		s.deferralSt = &deferrals{
			trying: map[string]bool{},
			wake:   make(chan struct{}, 1),
			backup: func(ctx context.Context, name string) error {
				_, err := s.Backup(ctx, name)
				return err
			},
		}
	})
	return s.deferralSt
}

// regardlessKey marks a backup that runs although its app is busy.
type regardlessKey struct{}

func backsUpRegardless(ctx context.Context) bool {
	v, _ := ctx.Value(regardlessKey{}).(bool)
	return v
}

// scheduledBackupKey marks a container backup that a schedule started, the
// only kind that waits for a busy app. A backup somebody asked for runs at
// once.
type scheduledBackupKey struct{}

// WithScheduledBackup marks ctx as a container backup a schedule started.
func WithScheduledBackup(ctx context.Context) context.Context {
	return context.WithValue(ctx, scheduledBackupKey{}, true)
}

// waitsForApp reports whether a backup under ctx waits while its app is
// busy: one a schedule started, a scheduled Backup Everything pass included,
// and the retry of one that waits already.
func waitsForApp(ctx context.Context) bool {
	v, _ := ctx.Value(scheduledBackupKey{}).(bool)
	return v || scheduledPass(ctx)
}

// policyOf is the target's policy. A failed read falls back to the default,
// so a deferral still gets tried again.
func (s *Service) policyOf(targetID string) store.DeferPolicy {
	p, err := s.store.DeferPolicyOf(targetID)
	if err != nil {
		log.Printf("api: deferral: read the policy of %s: %v", targetID, err)
		return store.DefaultDeferPolicy(targetID)
	}
	return p
}

// deferBackup records a backup that was deferred. The first deferral of a
// container starts its wait and tells; a later one only moves the next try.
func (s *Service) deferBackup(ctx context.Context, tg store.Target, runID, reason string, now time.Time) {
	reason = scrubSecrets(reason)
	p := s.policyOf(tg.ID)
	d, waiting, err := s.store.GetDeferral(tg.ID)
	if err != nil {
		log.Printf("api: deferral: %v", err)
	}
	if !waiting {
		d = store.Deferral{TargetID: tg.ID, Since: now.Unix()}
	}
	d.Container, d.Reason = tg.ContainerName, reason
	d.Tries++
	next, _ := schedule.NextRetry(p, time.Unix(d.Since, 0), now)
	d.NextTry = next.Unix()
	if err := s.store.SaveDeferral(d); err != nil {
		log.Printf("api: deferral: remember %s: %v", tg.ContainerName, err)
	}
	if runID != "" {
		if err := s.store.SetRunRetryAt(runID, d.NextTry); err != nil {
			log.Printf("api: deferral: %v", err)
		}
	}
	log.Printf("api: backup of %s deferred (%s), next try %s", tg.ContainerName, reason, next.Format(time.RFC3339)) //nolint:gosec // G706: container names from Docker
	if waiting {
		if c, cErr := s.NotifyConfig(); cErr == nil {
			notify.Ping(ctx, c, "container", true)
		}
	} else {
		s.notifyDeferred(ctx, tg.ContainerName, reason, next, schedule.RetryDeadline(p, now), p.AtDeadline)
	}
	s.wakeDeferrals()
}

// settleDeferral ends the wait of a container whose backup ran: it went
// through, or failed for a reason of its own, which puts it under Needs you
// like any other failure. A backup cut off by a shutdown keeps its wait for
// the next start.
func (s *Service) settleDeferral(ctx context.Context, targetID, name string, sum backup.Summary, err error) {
	if targetID == "" {
		return
	}
	if _, deferred := backup.IsDeferred(err); deferred || (err != nil && s.IsShuttingDown()) {
		return
	}
	d, waiting, gErr := s.store.GetDeferral(targetID)
	if gErr != nil || !waiting {
		return
	}
	if dErr := s.store.DeleteDeferral(targetID); dErr != nil {
		log.Printf("api: deferral: forget %s: %v", name, dErr)
	}
	if err != nil {
		log.Printf("api: the deferred backup of %s failed, which ends its wait: %v", name, err) //nolint:gosec // G706: container names from Docker
		return
	}
	log.Printf("api: the deferred backup of %s went through", name) //nolint:gosec // G706: container names from Docker
	s.notifyDeferralDone(ctx, name, d, sum)
}

// hasDeferral reports whether a container's backup waits for its app.
func (s *Service) hasDeferral(targetID string) bool {
	_, waiting, err := s.store.GetDeferral(targetID)
	return err == nil && waiting
}

func (s *Service) wakeDeferrals() {
	select {
	case s.deferralState().wake <- struct{}{}:
	default:
	}
}

// StartDeferrals tries the waiting backups again whenever one is due, until
// BombVault stops. The waits are in the store, so a restart carries on with
// them; one whose deadline passed meanwhile is settled at the first look.
func (s *Service) StartDeferrals() {
	st := s.deferralState()
	ctx := s.StopContext()
	st.wg.Add(1)
	go func() {
		defer st.wg.Done()
		t := time.NewTicker(deferralTick)
		defer t.Stop()
		for {
			s.tryDeferrals(ctx, time.Now())
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			case <-st.wake:
			}
		}
	}()
}

// deferralDropped is the reason a waiting backup is not tried again, or "".
func deferralDropped(settings store.Settings, tg store.Target) string {
	if !settings.ContainersEnabled {
		return store.ReasonDeferralSectionOff
	}
	if !tg.IncludeInSchedule {
		return store.ReasonDeferralUnscheduled
	}
	return ""
}

// dropDeferral ends a wait that has no schedule left to belong to. The end
// is recorded as a skipped run and told once, like any other end of a wait.
func (s *Service) dropDeferral(tg store.Target, d store.Deferral, why string, now time.Time) {
	if err := s.store.DeleteDeferral(tg.ID); err != nil {
		log.Printf("api: deferral: forget %s: %v", tg.ContainerName, err)
	}
	log.Printf("api: the deferred backup of %s is not tried again: %s", tg.ContainerName, why) //nolint:gosec // G706: container names from Docker
	ctx := context.Background()
	if runID, err := s.startRun(ctx, tg.ID, "backup"); err != nil {
		log.Printf("api: deferral: record the end of the wait of %s: %v", tg.ContainerName, err)
	} else if err := s.store.FinishRun(runID, statusSkipped, "", 0, why); err != nil {
		log.Printf("api: deferral: record the end of the wait of %s: %v", tg.ContainerName, err)
	}
	c, err := s.NotifyConfig()
	if err != nil || !c.Active() {
		return
	}
	msg := fmt.Sprintf("Backup of container %q is not tried again: %s. It had waited since %s.", tg.ContainerName, why, clockAt(time.Unix(d.Since, 0), now))
	s.sendAlert(notify.WithHealthchecksSuppressed(ctx), c, "container", notify.Event{Title: "BombVault", Message: msg, OK: true, Warning: true},
		"BombVault: deferred backup ended", msg, "normal")
}

// containersBusy reports whether a backup of the containers runs or is about
// to, which a retry waits for rather than taking the next turn from it.
func (s *Service) containersBusy() bool {
	if s.batchActive.Load() || s.everythingActive.Load() || s.scheduledRunOpen("containers") {
		return true
	}
	_, busy := s.domainBusy("containers")
	return busy
}

// tryDeferrals looks at every waiting backup that is due at now.
func (s *Service) tryDeferrals(ctx context.Context, now time.Time) {
	list, err := s.store.ListDeferrals()
	if err != nil || len(list) == 0 {
		if err != nil {
			log.Printf("api: deferral: %v", err)
		}
		return
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		log.Printf("api: deferral: %v", err)
		return
	}
	targets, err := s.store.ListTargets()
	if err != nil {
		log.Printf("api: deferral: %v", err)
		return
	}
	byID := make(map[string]store.Target, len(targets))
	for _, t := range targets {
		byID[t.ID] = t
	}
	st := s.deferralState()
	for _, d := range list {
		if d.NextTry > now.Unix() {
			continue
		}
		st.mu.Lock()
		trying := st.trying[d.TargetID]
		st.mu.Unlock()
		if trying {
			continue
		}
		tg, ok := byID[d.TargetID]
		if !ok {
			// The entry was removed with its runs, so there is nothing to
			// record the end on.
			if dErr := s.store.DeleteDeferral(d.TargetID); dErr != nil {
				log.Printf("api: deferral: forget %s: %v", d.Container, dErr)
			}
			log.Printf("api: dropped the deferred backup of %s: its entry is gone", d.Container) //nolint:gosec // G706: container names from Docker
			continue
		}
		if why := deferralDropped(settings, tg); why != "" {
			s.dropDeferral(tg, d, why, now)
			continue
		}
		p := s.policyOf(tg.ID)
		deadline := schedule.RetryDeadline(p, time.Unix(d.Since, 0))
		switch {
		case !now.Before(deadline) && p.AtDeadline != store.AtDeadlineBackUp:
			s.giveUpDeferral(tg, d, now)
		case !now.Before(deadline):
			s.retryDeferred(tg, true)
		case !schedule.InWindow(p, now):
			next := schedule.IntoWindow(p, now)
			if !next.Before(deadline) {
				next = deadline
			}
			d.NextTry = next.Unix()
			if sErr := s.store.SaveDeferral(d); sErr != nil {
				log.Printf("api: deferral: %v", sErr)
			}
		case s.containersBusy():
			// Asked again at the next look, once the other backup is done.
		default:
			s.retryDeferred(tg, false)
		}
	}
}

// retryDeferred backs up a waiting container on its own, as one backup the
// way Back up now runs it, with its stack folder, retention and off-site copy.
// It takes the guard Back up now takes, so a click meanwhile hears that a
// backup runs, and it leaves the retry for the next look when that guard is
// taken.
func (s *Service) retryDeferred(tg store.Target, regardless bool) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return
	}
	st := s.deferralState()
	st.mu.Lock()
	st.trying[tg.ID] = true
	st.mu.Unlock()
	st.wg.Add(1)
	go func() {
		defer st.wg.Done()
		defer s.recoverOperation("deferred backup: "+tg.ContainerName, nil, func(msg string) { s.failStuckRun(tg.ID, msg) })
		defer func() {
			st.mu.Lock()
			delete(st.trying, tg.ID)
			st.mu.Unlock()
		}()
		defer s.batchActive.Store(false)
		// The scheduled run this backup belonged to has pinged its check already.
		ctx := WithScheduledBackup(notify.WithHealthchecksSuppressed(context.Background()))
		if regardless {
			ctx = context.WithValue(ctx, regardlessKey{}, true)
		}
		if err := st.backup(ctx, tg.ContainerName); err != nil {
			if _, deferred := backup.IsDeferred(err); !deferred {
				log.Printf("api: deferred backup of %q %s: %v", tg.ContainerName, backupEnding(err), err) //nolint:gosec // G706: name is %q-quoted
			}
		}
	}()
}

// giveUpDeferral ends a wait whose deadline passed with the app still busy.
// The backup is recorded as failed, which puts it under Needs you with its
// Retry, and the alert says so once.
func (s *Service) giveUpDeferral(tg store.Target, d store.Deferral, now time.Time) {
	ctx := context.Background()
	if runID, err := s.startRun(ctx, tg.ID, "backup"); err != nil {
		log.Printf("api: deferral: record the end of the wait of %s: %v", tg.ContainerName, err)
	} else if err := s.store.FinishRun(runID, "failed", "", 0, store.ReasonDeferralGaveUp); err != nil {
		log.Printf("api: deferral: record the end of the wait of %s: %v", tg.ContainerName, err)
	}
	if err := s.store.DeleteDeferral(tg.ID); err != nil {
		log.Printf("api: deferral: forget %s: %v", tg.ContainerName, err)
	}
	log.Printf("api: gave up the deferred backup of %s: the app was still busy at the deadline", tg.ContainerName) //nolint:gosec // G706: container names from Docker
	c, err := s.NotifyConfig()
	if err != nil || !c.Active() {
		return
	}
	msg := fmt.Sprintf("Backup of container %q gave up: the app was still busy when its wait ended (deferred since %s, %d tries, last: %s). It is under Needs you, where Retry backs it up.",
		tg.ContainerName, clockAt(time.Unix(d.Since, 0), now), d.Tries, d.Reason)
	s.sendAlert(ctx, c, "container", notify.Event{Title: "BombVault", Message: msg, OK: false},
		"BombVault: deferred backup gave up", msg, "warning")
}

// notifyDeferred tells once that a backup waits for its app. In summary mode
// the round's summary names it instead.
func (s *Service) notifyDeferred(ctx context.Context, name, reason string, next, deadline time.Time, atDeadline string) {
	c, err := s.NotifyConfig()
	if err != nil || !c.Active() {
		return
	}
	now := time.Now()
	then := "gives up and puts it under Needs you"
	if atDeadline == store.AtDeadlineBackUp {
		then = "backs it up anyway"
	}
	msg := fmt.Sprintf("Backup of container %q was deferred: %s. BombVault tries again at %s, and at %s it %s.",
		name, reason, clockAt(next, now), clockAt(deadline, now), then)
	summary := notify.MessagesSuppressed(ctx) && c.ScheduledSummary
	if tally := roundTallyFrom(ctx); tally != nil && summary {
		tally.addDeferred(fmt.Sprintf("- %s: %s, next try %s", name, reason, clockAt(next, now)))
	}
	notify.Send(ctx, c, "container", notify.Event{Title: "BombVault", Message: msg, OK: true, ItemResult: true, Warning: true})
	if s.unraidGate(c.Unraid) && !summary {
		if e := s.sendUnraidNotify(ctx, "BombVault: backup deferred", msg, "normal"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// notifyDeferralDone tells that a deferred backup went through, in place of
// the message of a plain success, so whoever heard of the deferral hears how
// it ended.
func (s *Service) notifyDeferralDone(ctx context.Context, name string, d store.Deferral, sum backup.Summary) {
	c, err := s.NotifyConfig()
	if err != nil || !c.Active() {
		return
	}
	how := "once the app was free"
	switch {
	case backsUpRegardless(ctx):
		how = "at the end of its wait, with the app still busy"
	case !waitsForApp(ctx):
		how = "when it was started by hand"
	}
	msg := fmt.Sprintf("Backup of container %q went through %s (snapshot %s, %s). It had waited since %s.",
		name, how, shortID(sum.SnapshotID), humanBytes(sum.Bytes), clockAt(time.Unix(d.Since, 0), time.Now()))
	s.sendAlert(ctx, c, "container", notify.Event{Title: "BombVault", Message: msg, OK: true, Warning: true},
		"BombVault: deferred backup done", msg, "normal")
}

// clockAt is t as the time of day, with the day in front when it is not the
// day of now.
func clockAt(t, now time.Time) string {
	t, now = t.Local(), now.Local()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("15:04")
	}
	return t.Format("Mon Jan 2 15:04")
}

// DeferredBackups lists the container backups that wait for their app, the
// next to be tried first.
func (s *Service) DeferredBackups() []DeferredBackup {
	out := []DeferredBackup{}
	list, err := s.store.ListDeferrals()
	if err != nil {
		log.Printf("api: deferral: %v", err)
		return out
	}
	for _, d := range list {
		p := s.policyOf(d.TargetID)
		out = append(out, DeferredBackup{
			Domain: "containers", Name: d.Container, Reason: d.Reason,
			Since: d.Since, NextTry: d.NextTry, Deadline: schedule.RetryDeadline(p, time.Unix(d.Since, 0)).Unix(),
			AtDeadline: p.AtDeadline, Tries: d.Tries,
		})
	}
	return out
}

// deferralHolds reports whether a backup of domain waits for its app within
// its deadline at now. The overdue alert and the protection status wait for
// it as well; once the deadline has passed the wait has ended and they count
// as before.
func (s *Service) deferralHolds(domain string, now int64) bool {
	if domain != "containers" {
		return false
	}
	for _, d := range s.DeferredBackups() {
		if d.Deadline > now {
			return true
		}
	}
	return false
}
