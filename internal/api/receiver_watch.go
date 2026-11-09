package api

// The receiver watch is the overdue-backup watchdog for a box that receives
// off-site copies, gated on Settings.ReceiverEnabled:
//
//   - Dead-man's switch: on its daily tick (schedule.ReceiverCadence) it finds the
//     newest snapshot per source (host and item tag) of every enabled received
//     repo and alerts once per stale episode for a source older than the repo's
//     dead_man_hours. received_alert_state remembers which snapshot the alert
//     was based on, and a newer one ends the episode.
//   - Integrity: at a fire of a repo's CheckCadence (schedule.SweepCadence asks)
//     it runs an independent restic check on the receiving hardware, persists the
//     verdict and alerts only on the transition into failure.
//
// Verdicts are persisted whatever the notify policy says; only the alerts
// follow it.

import (
	"context"
	"database/sql"
	"errors"
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

// The episodes of a repository that says nothing about its sources. The alert
// state holds them beside the sources, whose keys always name an item tag.
const (
	receiverUnreadEpisode = "\x00unread"
	receiverNeverEpisode  = "\x00never"
)

// receiverSnapshots opens a received repo read-only and lists its snapshots
// once, with the legacy host folded into the sender's (see foldLegacyHost). It
// reads no stats, so the daily scan stays cheap.
func (s *Service) receiverSnapshots(ctx context.Context, rr store.ReceivedRepo) ([]restic.Snapshot, error) {
	repo, mode, err := s.receiverOpen(ctx, rr)
	if err != nil {
		return nil, err
	}
	snaps, err := s.listSnapshots(ctx, repo, mode)
	return foldLegacyHost(snaps), err
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

// RunReceiverWatch runs the daily dead-man's switch across every enabled
// received repo.
func (s *Service) RunReceiverWatch(ctx context.Context) error {
	return s.runReceiverWatchAt(ctx, time.Now().Unix())
}

// RunReceiverChecks runs the integrity checks that have a fire due.
func (s *Service) RunReceiverChecks(ctx context.Context) error {
	return s.runReceiverChecksAt(ctx, time.Now().Unix())
}

// receiverCheckUnread is when a scheduled check last could not open repo id. It
// lives in memory only: after a restart the next tick asks once more.
func (s *Service) receiverCheckUnread(id string) (time.Time, bool) {
	s.receiverCheckMu.Lock()
	defer s.receiverCheckMu.Unlock()
	at, ok := s.receiverUnread[id]
	return at, ok
}

func (s *Service) noteReceiverCheckUnread(id string, unread bool, at time.Time) {
	s.receiverCheckMu.Lock()
	defer s.receiverCheckMu.Unlock()
	if !unread {
		delete(s.receiverUnread, id)
		return
	}
	if s.receiverUnread == nil {
		s.receiverUnread = map[string]time.Time{}
	}
	s.receiverUnread[id] = at
}

func (s *Service) runReceiverWatchAt(ctx context.Context, now int64) error {
	return s.eachReceivedRepo(func(c notify.Config, alertsOn bool, rr store.ReceivedRepo) {
		s.receiverDeadManSweep(ctx, c, alertsOn, rr, now)
	})
}

func (s *Service) runReceiverChecksAt(ctx context.Context, now int64) error {
	return s.eachReceivedRepo(func(c notify.Config, alertsOn bool, rr store.ReceivedRepo) {
		s.receiverScheduledCheck(ctx, c, alertsOn, rr, now)
	})
}

// eachReceivedRepo walks the enabled received repos. A muted notify policy still
// runs and persists the work but sends no alerts.
func (s *Service) eachReceivedRepo(do func(c notify.Config, alertsOn bool, rr store.ReceivedRepo)) error {
	repos, err := s.store.ListReceivedRepos()
	if err != nil {
		return fmt.Errorf("list received repos: %w", err)
	}
	if len(repos) == 0 {
		return nil
	}
	c, cErr := s.NotifyConfig()
	if cErr != nil {
		log.Printf("api: receiver: read notify config: %v (alerts muted this run)", cErr)
		c = notify.Config{}
	}
	alertsOn := c.Active()
	for _, rr := range repos {
		if rr.Enabled {
			do(c, alertsOn, rr)
		}
	}
	return nil
}

// NextReceiverCheck is when the soonest enabled repo's check comes up next.
func (s *Service) NextReceiverCheck(now time.Time) (time.Time, bool) {
	repos, err := s.store.ListReceivedRepos()
	if err != nil {
		return time.Time{}, false
	}
	var soonest time.Time
	for _, rr := range repos {
		cad, err := schedule.ParseCadence(rr.CheckCadence)
		if err != nil || !rr.Enabled {
			continue
		}
		if next, ok := cad.NextDue(time.Unix(max(rr.LastCheckAt, rr.CreatedAt), 0), now); ok && (soonest.IsZero() || next.Before(soonest)) {
			soonest = next
		}
	}
	return soonest, !soonest.IsZero()
}

// receiverDeadManSweep evaluates the dead-man's switch for one repo and alerts
// once per episode: for a source gone stale, for a repo that cannot be read, and
// for one that never received anything within the window.
func (s *Service) receiverDeadManSweep(ctx context.Context, c notify.Config, alertsOn bool, rr store.ReceivedRepo, now int64) {
	deadManSeconds := int64(rr.DeadManHours) * 3600
	if deadManSeconds <= 0 {
		return
	}
	snaps, err := s.receiverSnapshots(ctx, rr)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		log.Printf("api: receiver: dead-man scan for %q cannot read the repository: %v", rr.Name, err) //nolint:gosec // G706: rr.Name is %q-quoted
		everRead := rr.LastReadAt > 0 || (rr.LastCheckOK.Valid && rr.LastCheckOK.Bool)
		absent := errors.As(err, new(notThereYetError))
		if absent && !everRead {
			s.receiverNothingYet(ctx, c, alertsOn, rr, now, deadManSeconds)
			return
		}
		detail := scrubError(err)
		if absent {
			detail = "it is no longer at its location"
		}
		s.receiverEpisodeOnce(ctx, alertsOn, rr, receiverUnreadEpisode, now, func() {
			s.notifyReceiver(ctx, c, "BombVault: received repository unreadable", fmt.Sprintf("Received repository %s cannot be read: %s", repoLabel(rr.Name), detail))
		})
		return
	}
	s.endReceiverEpisode(rr, receiverUnreadEpisode)
	if err := s.store.SetReceivedRepoRead(rr.ID, now); err != nil {
		log.Printf("api: receiver: note the read of %q: %v", rr.Name, err) //nolint:gosec // G706: rr.Name is %q-quoted
	}
	if len(snaps) == 0 {
		s.receiverNothingYet(ctx, c, alertsOn, rr, now, deadManSeconds)
		return
	}
	s.endReceiverEpisode(rr, receiverNeverEpisode)
	for _, src := range receiverDeadManSourcesOf(snaps) {
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

// receiverNothingYet is the switch for a repo whose sender has not made a first
// copy. A window after saving it, that is news: a repo saved ahead of the first
// copy is the normal setup order until then.
func (s *Service) receiverNothingYet(ctx context.Context, c notify.Config, alertsOn bool, rr store.ReceivedRepo, now, deadManSeconds int64) {
	if now-rr.CreatedAt <= deadManSeconds {
		return
	}
	s.receiverEpisodeOnce(ctx, alertsOn, rr, receiverNeverEpisode, now, func() {
		s.notifyReceiver(ctx, c, "BombVault: no backup received", fmt.Sprintf("No backup has ever arrived on %s (window %dh)", repoLabel(rr.Name), rr.DeadManHours))
	})
}

// receiverEpisodeOnce sends an alert for an episode that has no snapshot to
// base it on, once until endReceiverEpisode. Under a muted policy the episode is
// recorded too, as for a stale source.
func (s *Service) receiverEpisodeOnce(ctx context.Context, alertsOn bool, rr store.ReceivedRepo, episode string, now int64, send func()) {
	_, have, err := s.store.GetReceivedAlertState(rr.ID, episode)
	if err != nil || have || ctx.Err() != nil {
		return
	}
	if alertsOn {
		send()
	}
	if err := s.store.UpsertReceivedAlertState(store.ReceivedAlertState{ReceivedRepoID: rr.ID, Source: episode, NotifiedAt: now, BasedOn: now}); err != nil {
		log.Printf("api: receiver: alert-state record for %q failed: %v", rr.Name, err) //nolint:gosec // G706: rr.Name is %q-quoted
	}
}

func (s *Service) endReceiverEpisode(rr store.ReceivedRepo, episode string) {
	if err := s.store.DeleteReceivedAlertState(rr.ID, episode); err != nil {
		log.Printf("api: receiver: alert-state clear for %q failed: %v", rr.Name, err) //nolint:gosec // G706: rr.Name is %q-quoted
	}
}

// receiverScheduledCheck runs the independent restic check for one repo at a
// fire of its CheckCadence, persists the verdict and alerts only on the
// transition into failure. A cadence of "off", or one that does not parse,
// disables the scheduled check; the dead-man's switch still runs. A new repo
// waits for its first fire after it was saved. A check that could not open the
// repo knows nothing about its integrity: it leaves no verdict, the dead-man's
// switch tells about the repo, and it tries again the next day at the same time.
func (s *Service) receiverScheduledCheck(ctx context.Context, c notify.Config, alertsOn bool, rr store.ReceivedRepo, now int64) {
	cad, err := schedule.ParseCadence(rr.CheckCadence)
	if err != nil {
		return
	}
	at, since := time.Unix(now, 0), time.Unix(max(rr.LastCheckAt, rr.CreatedAt), 0)
	due := cad.FireDue(since, at)
	if tried, ok := s.receiverCheckUnread(rr.ID); ok && tried.After(since) {
		due = cad.RetryDue(tried, at)
	}
	if !due {
		return
	}
	prev := rr.LastCheckOK
	// The gate above cannot see a manual check in flight, since LastCheckAt is
	// stamped at the end. Skipping leaves the timestamp alone, so the next sweep
	// tries again if that check never lands.
	res, err := s.receiverCheckExclusive(ctx, rr, rr.ReadDataPercent > 0)
	if errors.Is(err, errReceiverCheckBusy) {
		log.Printf("api: receiver: %q is already being checked, skipping the scheduled one", rr.Name) //nolint:gosec // G706: rr.Name is %q-quoted
		return
	}
	if err != nil {
		return
	}
	s.noteReceiverCheckUnread(rr.ID, res.Unread, at)
	if res.Unread {
		log.Printf("api: receiver: %q cannot be read, so its check waits: %s", rr.Name, res.Error) //nolint:gosec // G706: rr.Name is %q-quoted, the error is scrubbed
		return
	}
	if err := s.store.UpdateReceivedRepoCheckResult(rr.ID, res.At, nullCheckOK(res.OK), res.Error, res.RanReadData); err != nil {
		log.Printf("api: receiver: persist check result for %q failed: %v", rr.Name, err) //nolint:gosec // G706: rr.Name is %q-quoted
	}
	if alertsOn && receiverIntegrityShouldAlert(prev, res.OK) {
		s.notifyReceiverIntegrity(ctx, c, rr.Name, res.Error)
	}
}

// notifyReceiver sends a receiver alert, like notifyBackupOverdue. The Healthchecks ping is suppressed:
// this is an alert about a repository, not a run event.
func (s *Service) notifyReceiver(ctx context.Context, c notify.Config, subject, msg string) {
	notify.Send(notify.WithHealthchecksSuppressed(ctx), c, "receiver",
		notify.Event{Title: subject, Message: msg})
}

func (s *Service) notifyReceiverDeadMan(ctx context.Context, c notify.Config, repoName, source string, hours int) {
	s.notifyReceiver(ctx, c, "BombVault: no backup received", fmt.Sprintf("No backup received from %s on %s in %dh", source, repoLabel(repoName), hours))
}

func (s *Service) notifyReceiverIntegrity(ctx context.Context, c notify.Config, repoName, detail string) {
	s.notifyReceiver(ctx, c, "BombVault: integrity check failed", fmt.Sprintf("Integrity check FAILED on %s: %s", repoLabel(repoName), detail))
}

// repoLabel renders a received repo's display name for an alert, falling back to a
// generic label when the repo was registered without a name.
func repoLabel(name string) string {
	if name == "" {
		return "the received repository"
	}
	return name
}
