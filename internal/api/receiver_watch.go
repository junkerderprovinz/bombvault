package api

// The scheduled receiver watch: the counterpart to the overdue-backup watchdog for
// a box that RECEIVES immutable off-site copies. On its fixed daily tick
// (schedule.ReceiverCadence, gated on Settings.ReceiverEnabled) it walks every
// enabled received repo and, per repo:
//
//   - DEAD-MANS-SWITCH: opens the repo READ-ONLY, finds the newest snapshot per
//     SOURCE (host + BombVault item tag), and raises an alert for any source whose
//     newest snapshot is older than the repo's dead_man_hours. Each stale episode
//     alerts ONCE (received_alert_state remembers the newest-snapshot time the
//     verdict was based on); a newer received snapshot ends the episode and re-arms.
//   - INTEGRITY: when the repo's own CheckCadence is due, runs the INDEPENDENT
//     restic check on the receiving hardware, persists the verdict, and raises an
//     alert only on the transition into failure (first breach / ok->fail), debounced
//     off the persisted last_check_ok so it never re-fires every tick while failing.
//
// Check results are persisted regardless of the notify policy (they are dashboard
// state); only the ALERTS are gated on the policy, exactly like the watchdog.

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/platform"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// nullCheckOK wraps a definite check verdict as a valid sql.NullBool (a check that
// actually ran is never the NULL "never checked" state).
func nullCheckOK(ok bool) sql.NullBool { return sql.NullBool{Bool: ok, Valid: true} }

// receiverDeadManSource is one source's newest-snapshot fact the dead-mans-switch
// evaluates: the human label plus the newest snapshot time (unix) received from it.
type receiverDeadManSource struct {
	source string // human label, e.g. "container:web @ tower"
	key    string // stable episode key, unique per (host, item)
	newest int64  // newest snapshot time (unix); always > 0 (a source has >=1 snapshot)
}

// receiverDeadManDecision is the pure per-source verdict runReceiverChecksAt acts
// on, so the once-per-episode dedupe is unit-testable without a store, clock or
// notify transport. It mirrors watchdogDecision:
//
//   - dead-man disabled (deadManSeconds <= 0): never alert; clear any episode.
//   - not stale (newest within the window): never alert; clear any episode (the
//     source recovered, re-arming the switch).
//   - stale with an episode based on the SAME newest snapshot: already alerted for
//     this episode - stay quiet.
//   - stale otherwise (no episode, or a newer snapshot since the recorded one, or
//     an episode that is now newer/older): alert.
func receiverDeadManDecision(now, newest, deadManSeconds int64, state store.ReceivedAlertState, haveState bool) (alert, clearState bool) {
	if deadManSeconds <= 0 || newest <= 0 {
		return false, haveState
	}
	stale := now-newest > deadManSeconds
	if !stale {
		return false, haveState
	}
	if haveState && state.BasedOn == newest {
		return false, false
	}
	return true, false
}

// receiverIntegrityShouldAlert reports whether a check verdict is a fresh breach:
// the check failed AND the previous verdict was either "never checked" (NULL) or
// OK. A failure that follows an already-recorded failure does not re-alert, so a
// persistently broken repo alerts once, not every tick. A recovery (OK) never
// alerts and, by leaving an OK verdict behind, re-arms the alert for the next
// failure.
func receiverIntegrityShouldAlert(prev sql.NullBool, newOK bool) bool {
	if newOK {
		return false
	}
	return !prev.Valid || prev.Bool
}

// receiverDeadManSources opens a received repo READ-ONLY, lists its snapshots
// ONCE, and returns the newest snapshot time per SOURCE (host + BombVault item
// tag). No stats are read - the dead-mans-switch only needs snapshot times, so the
// daily scan stays cheap. Read-only; nothing is written to the received repo.
func (s *Service) receiverDeadManSources(ctx context.Context, rr store.ReceivedRepo) ([]receiverDeadManSource, error) {
	repo, mode, err := s.receiverOpen(ctx, rr)
	if err != nil {
		return nil, err
	}
	snaps, err := s.listSnapshots(ctx, repo, mode)
	if err != nil {
		return nil, err
	}
	type agg struct {
		host, item string
		newest     time.Time
		haveNewest bool
	}
	groups := map[string]*agg{}
	for _, snap := range snaps {
		item := receiverItemTag(snap)
		key := snap.Hostname + "\x00" + item
		g := groups[key]
		if g == nil {
			g = &agg{host: snap.Hostname, item: item}
			groups[key] = g
		}
		when := parseSnapshotTime(snap.Time)
		if !g.haveNewest || when.After(g.newest) {
			g.newest = when
			g.haveNewest = true
		}
	}
	out := make([]receiverDeadManSource, 0, len(groups))
	for key, g := range groups {
		out = append(out, receiverDeadManSource{
			source: g.item + " @ " + g.host,
			key:    key,
			newest: g.newest.Unix(),
		})
	}
	return out, nil
}

// RunReceiverChecks performs the daily receiver watch across every enabled
// received repo. It is the scheduler's receiver job (SetReceiverJob in
// cmd/bombvault/main.go).
func (s *Service) RunReceiverChecks(ctx context.Context) error {
	return s.runReceiverChecksAt(ctx, time.Now().Unix())
}

// runReceiverChecksAt is the testable core of RunReceiverChecks with an injectable
// "now". The notify config is read once; a muted policy (On empty/"never") still
// runs + persists the checks (dashboard state) but sends no alerts.
func (s *Service) runReceiverChecksAt(ctx context.Context, now int64) error {
	c, cErr := s.NotifyConfig()
	if cErr != nil {
		log.Printf("api: receiver: read notify config: %v (alerts muted this run)", cErr)
		c = notify.Config{}
	}
	alertsOn := c.On == "always" || c.On == "failure"

	repos, err := s.store.ListReceivedRepos()
	if err != nil {
		return fmt.Errorf("list received repos: %w", err)
	}
	for _, rr := range repos {
		if !rr.Enabled {
			continue
		}
		s.receiverDeadManSweep(ctx, c, alertsOn, rr, now)
		s.receiverScheduledCheck(ctx, c, alertsOn, rr, now)
	}
	return nil
}

// receiverDeadManSweep evaluates the dead-mans-switch for one repo's sources and
// alerts (once per stale episode) on any source that has gone quiet. A repo that
// cannot be opened is skipped here - that failure surfaces as an INTEGRITY alert
// from the scheduled check, not as a dead-man alert (which is about a source that
// stopped sending, not a receiver that cannot read).
func (s *Service) receiverDeadManSweep(ctx context.Context, c notify.Config, alertsOn bool, rr store.ReceivedRepo, now int64) {
	deadManSeconds := int64(rr.DeadManHours) * 3600
	if deadManSeconds <= 0 {
		return
	}
	sources, err := s.receiverDeadManSources(ctx, rr)
	if err != nil {
		log.Printf("api: receiver: dead-man scan for %q skipped (cannot open): %v", rr.Name, err) //nolint:gosec // G706: rr.Name is %q-quoted
		return
	}
	for _, src := range sources {
		state, haveState, sErr := s.store.GetReceivedAlertState(rr.ID, src.key)
		if sErr != nil {
			log.Printf("api: receiver: alert-state read for %q failed: %v", rr.Name, sErr) //nolint:gosec // G706: rr.Name is %q-quoted
			continue
		}
		alert, clearState := receiverDeadManDecision(now, src.newest, deadManSeconds, state, haveState)
		if clearState {
			if dErr := s.store.DeleteReceivedAlertState(rr.ID, src.key); dErr != nil {
				log.Printf("api: receiver: alert-state clear for %q failed: %v", rr.Name, dErr) //nolint:gosec // G706: rr.Name is %q-quoted
			}
		}
		if !alert {
			continue
		}
		if alertsOn {
			s.notifyReceiverDeadMan(ctx, c, rr.Name, src.source, rr.DeadManHours)
		}
		// Record the episode AFTER the send: a failed record means at worst one
		// duplicate alert next tick, better than a silently lost episode. Recorded
		// even under a muted policy so enabling notifications later does not replay
		// an already-stale source as brand new.
		if uErr := s.store.UpsertReceivedAlertState(store.ReceivedAlertState{
			ReceivedRepoID: rr.ID, Source: src.key, NotifiedAt: now, BasedOn: src.newest,
		}); uErr != nil {
			log.Printf("api: receiver: alert-state record for %q failed: %v", rr.Name, uErr) //nolint:gosec // G706: rr.Name is %q-quoted
		}
	}
}

// receiverScheduledCheck runs the independent restic check for one repo when its
// CheckCadence is due, persists the verdict, and fires an integrity alert only on
// the transition into failure. A cadence of "off" (or an unparseable one) means no
// scheduled check for that repo - the dead-mans-switch still runs.
func (s *Service) receiverScheduledCheck(ctx context.Context, c notify.Config, alertsOn bool, rr store.ReceivedRepo, now int64) {
	period := cadencePeriodSeconds(rr.CheckCadence)
	if period <= 0 {
		return // "off"/invalid: no scheduled integrity check for this repo
	}
	if rr.LastCheckAt != 0 && now-rr.LastCheckAt < period {
		return // not due yet
	}
	prev := rr.LastCheckOK // the verdict BEFORE this check, for the transition test
	res := s.receiverCheck(ctx, rr, rr.ReadDataPercent > 0)
	if err := s.store.UpdateReceivedRepoCheckResult(rr.ID, res.At, nullCheckOK(res.OK), res.Error, res.RanReadData); err != nil {
		log.Printf("api: receiver: persist check result for %q failed: %v", rr.Name, err) //nolint:gosec // G706: rr.Name is %q-quoted
	}
	if alertsOn && receiverIntegrityShouldAlert(prev, res.OK) {
		s.notifyReceiverIntegrity(ctx, c, rr.Name, res.Error)
	}
}

// notifyReceiverDeadMan sends the dead-mans-switch alert through the established
// notify fan-out (message channels + the Unraid mirror), the same shape
// notifyReplicationFailed / notifyBackupOverdue use. The Healthchecks ping is
// suppressed: this is a human alert about a MISSING backup, not a run lifecycle
// event.
func (s *Service) notifyReceiverDeadMan(ctx context.Context, c notify.Config, repoName, source string, hours int) {
	repo := repoLabel(repoName)
	msg := fmt.Sprintf("No backup received from %s on %s in %dh", source, repo, hours)
	notify.Send(notify.WithHealthchecksSuppressed(ctx), c, "receiver",
		notify.Event{Title: "BombVault", Message: msg, OK: false})
	if c.Unraid && s.ssh != nil && s.platformFn().Kind() == platform.KindUnraid {
		if e := s.sendUnraidNotify(ctx, "BombVault: no backup received", msg, "warning"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// notifyReceiverIntegrity sends the integrity alert when the independent check on
// the receiving hardware fails. Same fan-out + Healthchecks-suppress discipline as
// the dead-man alert.
func (s *Service) notifyReceiverIntegrity(ctx context.Context, c notify.Config, repoName, detail string) {
	repo := repoLabel(repoName)
	msg := fmt.Sprintf("Integrity check FAILED on %s: %s", repo, detail)
	notify.Send(notify.WithHealthchecksSuppressed(ctx), c, "receiver",
		notify.Event{Title: "BombVault", Message: msg, OK: false})
	if c.Unraid && s.ssh != nil && s.platformFn().Kind() == platform.KindUnraid {
		if e := s.sendUnraidNotify(ctx, "BombVault: integrity check failed", msg, "warning"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// repoLabel renders a received repo's display name for an alert, falling back to a
// generic label when the repo was registered without a name.
func repoLabel(name string) string {
	if name == "" {
		return "the received repository"
	}
	return name
}
