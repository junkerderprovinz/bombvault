package api

// The receiver watch is the overdue-backup watchdog for a box that receives
// off-site copies. On its daily tick (schedule.ReceiverCadence, gated on
// Settings.ReceiverEnabled) it walks every enabled received repo:
//
//   - Dead-man's switch: it finds the newest snapshot per source (host and item
//     tag) and alerts once per stale episode for a source older than the repo's
//     dead_man_hours. received_alert_state remembers which snapshot the alert
//     was based on, and a newer one ends the episode.
//   - Integrity: when the repo's CheckCadence is due it runs an independent
//     restic check on the receiving hardware, persists the verdict and alerts
//     only on the transition into failure.
//
// Verdicts are persisted whatever the notify policy says; only the alerts
// follow it.

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// nullCheckOK wraps a definite check verdict as a valid sql.NullBool (a check that
// actually ran is never the NULL "never checked" state).
func nullCheckOK(ok bool) sql.NullBool { return sql.NullBool{Bool: ok, Valid: true} }

// receiverDeadManSource is one source as the dead-man's switch sees it: its
// label and the time of the newest snapshot received from it.
type receiverDeadManSource struct {
	source string // human label, e.g. "container:web @ tower"
	key    string // stable episode key, unique per (host, item)
	newest int64  // newest snapshot time (unix); always > 0 (a source has >=1 snapshot)
}

// receiverDeadManDecision is the per-source verdict runReceiverChecksAt acts
// on, kept pure so the once-per-episode rule can be tested without a store,
// clock or transport. As in watchdogDecision, a disabled switch or a fresh
// source clears any episode, and a stale source alerts unless its episode is
// based on the same newest snapshot.
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

// receiverIntegrityShouldAlert reports whether a failed check is a new breach,
// that is, the previous verdict was "never checked" or OK. A repo that stays
// broken alerts once, and an OK verdict re-arms the alert.
func receiverIntegrityShouldAlert(prev sql.NullBool, newOK bool) bool {
	if newOK {
		return false
	}
	return !prev.Valid || prev.Bool
}

// receiverDeadManSources opens a received repo read-only, lists its snapshots
// once and returns the newest snapshot time per source (host and item tag). It
// reads no stats, so the daily scan stays cheap.
func (s *Service) receiverDeadManSources(ctx context.Context, rr store.ReceivedRepo) ([]receiverDeadManSource, error) {
	repo, mode, err := s.receiverOpen(ctx, rr)
	if err != nil {
		return nil, err
	}
	snaps, err := s.listSnapshots(ctx, repo, mode)
	if err != nil {
		return nil, err
	}
	return receiverDeadManSourcesOf(snaps), nil
}

// receiverDeadManSourcesOf groups snapshots into the sources the sweep watches:
// one per host and item tag, with its newest snapshot time.
//
// A database dump is not one of them. Its series stops whenever the toggle, the
// global switch or a stopped database says so, and the container's own item
// already answers the question the sweep asks: has this box stopped sending?
func receiverDeadManSourcesOf(snaps []restic.Snapshot) []receiverDeadManSource {
	type agg struct {
		host, item string
		newest     time.Time
		haveNewest bool
	}
	groups := map[string]*agg{}
	for _, snap := range snaps {
		item := receiverItemTag(snap)
		if strings.HasPrefix(item, dbDumpIdentityPrefix) {
			continue
		}
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
	return out
}

// RunReceiverChecks runs the daily receiver watch across every enabled
// received repo.
func (s *Service) RunReceiverChecks(ctx context.Context) error {
	return s.runReceiverChecksAt(ctx, time.Now().Unix())
}

// runReceiverChecksAt is RunReceiverChecks with an injectable now. A muted
// notify policy still runs and persists the checks but sends no alerts.
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

// receiverDeadManSweep evaluates the dead-man's switch for one repo's sources
// and alerts once per stale episode. A repo that cannot be opened is skipped:
// the scheduled check reports that as an integrity failure, while a dead-man
// alert means a source stopped sending.
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
		// Recorded after the send, so a failed write costs one duplicate alert
		// rather than a lost episode, and under a muted policy too, so enabling
		// notifications later does not replay a stale source as new.
		if uErr := s.store.UpsertReceivedAlertState(store.ReceivedAlertState{
			ReceivedRepoID: rr.ID, Source: src.key, NotifiedAt: now, BasedOn: src.newest,
		}); uErr != nil {
			log.Printf("api: receiver: alert-state record for %q failed: %v", rr.Name, uErr) //nolint:gosec // G706: rr.Name is %q-quoted
		}
	}
}

// receiverScheduledCheck runs the independent restic check for one repo when
// its CheckCadence is due, persists the verdict and alerts only on the
// transition into failure. A cadence of "off", or one that does not parse,
// disables the scheduled check; the dead-man's switch still runs.
//
// The due gate is schedule.PeriodDue, which compares calendar days.
// LastCheckAt is stamped when the previous check finished, minutes after its
// sweep fired, so in elapsed seconds the next sweep falls short of the period
// and the check would slip by a whole day.
func (s *Service) receiverScheduledCheck(ctx context.Context, c notify.Config, alertsOn bool, rr store.ReceivedRepo, now int64) {
	period := cadencePeriodSeconds(rr.CheckCadence)
	if period <= 0 {
		return
	}
	var lastCheck time.Time
	if rr.LastCheckAt != 0 {
		lastCheck = time.Unix(rr.LastCheckAt, 0)
	}
	if !schedule.PeriodDue(lastCheck, time.Unix(now, 0), period) {
		return
	}
	prev := rr.LastCheckOK
	// The gate above cannot see a manual check in flight, since LastCheckAt is
	// stamped at the end. Skipping leaves the timestamp alone, so the next sweep
	// tries again if that check never lands.
	res, ran := s.receiverCheckExclusive(ctx, rr, rr.ReadDataPercent > 0)
	if !ran {
		log.Printf("api: receiver: %q is already being checked, skipping the scheduled one", rr.Name) //nolint:gosec // G706: rr.Name is %q-quoted
		return
	}
	if err := s.store.UpdateReceivedRepoCheckResult(rr.ID, res.At, nullCheckOK(res.OK), res.Error, res.RanReadData); err != nil {
		log.Printf("api: receiver: persist check result for %q failed: %v", rr.Name, err) //nolint:gosec // G706: rr.Name is %q-quoted
	}
	if alertsOn && receiverIntegrityShouldAlert(prev, res.OK) {
		s.notifyReceiverIntegrity(ctx, c, rr.Name, res.Error)
	}
}

// notifyReceiverDeadMan sends the dead-man's switch alert through the notify
// channels and the Unraid mirror, like notifyBackupOverdue. The Healthchecks
// ping is suppressed: this is an alert about a missing backup, not a run event.
func (s *Service) notifyReceiverDeadMan(ctx context.Context, c notify.Config, repoName, source string, hours int) {
	repo := repoLabel(repoName)
	msg := fmt.Sprintf("No backup received from %s on %s in %dh", source, repo, hours)
	notify.Send(notify.WithHealthchecksSuppressed(ctx), c, "receiver",
		notify.Event{Title: "BombVault", Message: msg, OK: false})
	if s.unraidGate(c.Unraid) {
		if e := s.sendUnraidNotify(ctx, "BombVault: no backup received", msg, "warning"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// notifyReceiverIntegrity sends the integrity alert when the independent check
// fails, the same way as the dead-man alert.
func (s *Service) notifyReceiverIntegrity(ctx context.Context, c notify.Config, repoName, detail string) {
	repo := repoLabel(repoName)
	msg := fmt.Sprintf("Integrity check FAILED on %s: %s", repo, detail)
	notify.Send(notify.WithHealthchecksSuppressed(ctx), c, "receiver",
		notify.Event{Title: "BombVault", Message: msg, OK: false})
	if s.unraidGate(c.Unraid) {
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
