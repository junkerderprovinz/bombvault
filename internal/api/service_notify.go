package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/platform"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// NotifyConfig returns the decrypted notification config (an empty Config when
// none is set).
func (s *Service) NotifyConfig() (notify.Config, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return notify.Config{}, err
	}
	var c notify.Config
	if strings.TrimSpace(settings.NotifyConf) == "" {
		return c, nil
	}
	enc, err := base64.StdEncoding.DecodeString(settings.NotifyConf)
	if err != nil {
		return c, err
	}
	plain, err := secret.Decrypt(s.cfg.AppKey, enc)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(plain, &c); err != nil {
		return c, err
	}
	return c, nil
}

// SetNotifyConfig encrypts + stores the notification config. A config with no
// channel and no policy clears it.
func (s *Service) SetNotifyConfig(c notify.Config) error {
	stored := ""
	if c.Configured() || (c.On != "" && c.On != "never") {
		blob, mErr := json.Marshal(c)
		if mErr != nil {
			return fmt.Errorf("marshal notify conf: %w", mErr)
		}
		enc, eErr := secret.Encrypt(s.cfg.AppKey, blob)
		if eErr != nil {
			return fmt.Errorf("encrypt notify conf: %w", eErr)
		}
		stored = base64.StdEncoding.EncodeToString(enc)
	}
	_, err := s.store.MutateSettings(func(settings *store.Settings) error {
		settings.NotifyConf = stored
		return nil
	})
	return err
}

// notifyBackup sends a best-effort notification for a completed backup. It reads
// the stored config each call (cheap; backups are infrequent) and is a no-op when
// notifications are off.
func (s *Service) notifyBackup(ctx context.Context, domain, name string, ok bool, sum backup.Summary, backupErr error) {
	c, err := s.NotifyConfig()
	if err != nil || c.On == "" || c.On == "never" {
		return
	}
	// Singleton domains have no per-item name, so a "%s %q" label would render an
	// empty quote (e.g. `config ""`). Give each a clean human label.
	var target string
	switch domain {
	case "flash":
		target = "Unraid flash"
	case "config":
		target = "BombVault configuration"
	default:
		target = fmt.Sprintf("%s %q", domain, name)
	}
	var msg string
	if ok {
		msg = fmt.Sprintf("Backup of %s succeeded (snapshot %s, %s).", target, shortID(sum.SnapshotID), humanBytes(sum.Bytes))
	} else {
		msg = fmt.Sprintf("Backup of %s FAILED: %s", target, scrubError(backupErr))
	}
	notify.Send(ctx, c, domain, notify.Event{Title: "BombVault", Message: msg, OK: ok})

	// Unraid native notification (delivered over SSH; notify.Send is
	// HTTP-only). Honour the same policy: notifyBackup already returned for
	// "never", so send on "always" or on any failure. In scheduled summary mode
	// drop the per-item Unraid push too; ScheduledNotifyResult sends the one
	// aggregate (#56).
	if s.unraidGate(c.Unraid) && (c.On == "always" || !ok) &&
		(!notify.MessagesSuppressed(ctx) || !c.ScheduledSummary) {
		level := "normal"
		subject := "BombVault: backup OK"
		if !ok {
			level = "warning"
			subject = "BombVault: backup FAILED"
		}
		if e := s.sendUnraidNotify(ctx, subject, msg, level); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// statusSkipped marks a run BombVault intentionally did not perform because the
// target's container no longer exists on the host (#57). runs.status is free-text,
// so this needs no schema migration (cf. the orchestrator's "cancelled" status).
const statusSkipped = "skipped"

// recordAndNotifyContainerSkip handles a scheduled target whose container
// is gone (#57): it records a "skipped" run so the dashboard reflects it,
// agreeing with the green aggregate Healthchecks ping instead of showing
// nothing, and warns the user, debounced to the first miss so a
// permanently removed container doesn't send a notification every night.
// The warning never pings Healthchecks (a skip is not a backup failure), so
// it can never turn a green monitor red.
func (s *Service) recordAndNotifyContainerSkip(ctx context.Context, name string) {
	tg, err := s.store.GetTargetByContainer(name)
	if err != nil {
		log.Printf("api: Backup: skip %q: load target: %v", name, err) //nolint:gosec // G706: name is %q-quoted
		return
	}
	// Debounce: warn only when the previous run for this target wasn't already a skip.
	firstMiss := true
	if last, lErr := s.store.LastRunForTarget(tg.ID); lErr == nil && last != nil && last.Status == statusSkipped {
		firstMiss = false
	}
	// Always record the skip so Run History shows it every run (a cheap audit trail)
	// rather than the removed target silently vanishing from the dashboard.
	if runID, sErr := s.store.StartRun(tg.ID, "backup"); sErr != nil {
		log.Printf("api: Backup: skip %q: start skipped run: %v", name, sErr) //nolint:gosec // G706: name is %q-quoted
	} else if fErr := s.store.FinishRun(runID, statusSkipped, "", 0, store.ReasonContainerGone); fErr != nil {
		log.Printf("api: Backup: skip %q: finish skipped run: %v", name, fErr) //nolint:gosec // G706: name is %q-quoted
	}
	if !firstMiss {
		return
	}
	c, err := s.NotifyConfig()
	// Honour the notify policy on both channels (message and Unraid): when
	// muted ("never" or unset) the skipped run row and the dashboard chip
	// already surface it, so no push should fire; otherwise a benign skip
	// would be noisier than a real backup failure, which notifyBackup keeps
	// quiet about under the same policy.
	if err != nil || (c.On != "always" && c.On != "failure") {
		return
	}
	// #111: say plainly that nothing is backed up any more, that the existing
	// backups stay restorable, and how to stop the reminder.
	msg := fmt.Sprintf("Container %q was removed from this host. BombVault is not backing it up anymore; the scheduled backup now skips it. Its existing backups are kept and remain restorable. To stop this reminder, exclude the container from the backup schedule or delete its backups in BombVault.", name)
	// Suppress the per-call Healthchecks ping unconditionally: a skip must
	// never flip the monitor to fail, and the scheduled run's aggregate ping
	// already speaks for the domain. The ctx is based on Background, not the
	// scheduled ctx, so the message is not swept up by scheduled-summary
	// suppression: a "target no longer exists" warning must reach the user
	// even in summary mode (like the update notice).
	notify.Send(notify.WithHealthchecksSuppressed(context.Background()), c, "containers",
		notify.Event{Title: "BombVault: backup target skipped", Message: msg, OK: false})
	if s.unraidGate(c.Unraid) {
		if e := s.sendUnraidNotify(ctx, "BombVault: backup target skipped", msg, "warning"); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// recordPreflightFailure records a failed backup run for an entry that failed
// before the orchestrator could take over run bookkeeping, at one of the
// pre-flight early returns (settings, repo path, EnsureRepo, inspect, upsert).
// Without it a domain-wide fault that trips those for every entry leaves no red
// anywhere (#64), and the card that started a single backup waits for a run that
// never comes (#251). Best-effort: a bookkeeping error is logged, never returned,
// since the caller is already returning the real one. An entry with no target row
// yet has nothing to key a run to, so there the reason is only logged, and the
// scheduled summary still names it from the returned error.
func (s *Service) recordPreflightFailure(kind, name, targetID string, cause error) {
	if targetID == "" {
		log.Printf("api: %s: %q failed before a run could be recorded (no target row yet): %v", kind, name, cause) //nolint:gosec // G706: kind is a fixed literal, name is %q-quoted
		return
	}
	runID, err := s.store.StartRun(targetID, "backup")
	if err != nil {
		log.Printf("api: %s: %q: start failed run: %v", kind, name, err) //nolint:gosec // G706: see above
		return
	}
	if err := s.store.FinishRun(runID, "failed", "", 0, truncateRunErr(cause)); err != nil {
		log.Printf("api: %s: %q: finish failed run: %v", kind, name, err) //nolint:gosec // G706: see above
	}
}

// notifyBackupStart pings the Healthchecks /start endpoint at the beginning of a
// backup (best-effort; never affects the backup). The message channels have no
// "start" concept, so this is Healthchecks-only.
func (s *Service) notifyBackupStart(ctx context.Context, domain string) {
	c, err := s.NotifyConfig()
	if err != nil {
		return
	}
	notify.SendStart(ctx, c, domain)
}

// ScheduledHealthchecksStart pings the domain's Healthchecks check /start once at the
// beginning of a scheduled per-domain run (containers/VMs). The scheduler runs each
// item with its own per-item /start suppressed (see main.go), so this single ping
// represents the whole domain job instead of one ping per container/VM (#49). It is
// best-effort and a no-op when the domain has no check configured or notifications are
// off. domain is the scheduler's spelling ("containers"|"vms"); notify normalises it.
func (s *Service) ScheduledHealthchecksStart(ctx context.Context, domain string) {
	c, err := s.NotifyConfig()
	if err != nil {
		return
	}
	notify.PingDomainStart(ctx, c, domain)
}

// ScheduledHealthchecksResult pings the domain's Healthchecks check once at the end of
// a scheduled per-domain run: success when every item succeeded (failed == 0), else
// /fail with a short aggregate summary ("N of M items failed"). It is the aggregate
// counterpart to the per-item success/fail ping, which the run suppresses, so the check
// reflects the whole domain job (#49). Best-effort; a no-op when the domain has no
// check configured or notifications are off.
func (s *Service) ScheduledHealthchecksResult(ctx context.Context, domain string, attempted, failed int) {
	c, err := s.NotifyConfig()
	if err != nil {
		return
	}
	ok := failed == 0
	var summary string
	if ok {
		summary = fmt.Sprintf("%d of %d items succeeded", attempted, attempted)
	} else {
		summary = fmt.Sprintf("%d of %d items failed", failed, attempted)
	}
	notify.PingDomainResult(ctx, c, domain, ok, summary)
}

// ScheduledNotifyResult sends one summary message per scheduled per-domain
// run on the message channels (webhook, Matrix, SMTP and Unraid) instead of
// one per item, the message-channel counterpart to
// ScheduledHealthchecksResult (#56). It is a no-op unless
// Config.ScheduledSummary is on (in which case the per-item messages were
// suppressed); the On policy still governs whether an all-success run
// notifies at all. domain is the scheduler spelling ("containers"|"vms").
// failures names the items that failed (name and reason), so a failing run
// lists which items broke and why instead of a bare count (#64).
func (s *Service) ScheduledNotifyResult(ctx context.Context, domain string, attempted, failed int, failures []schedule.ItemFailure) {
	c, err := s.NotifyConfig()
	// Honour the notify policy on all channels (message and Unraid): muted
	// ("never" or unset) sends nothing, so the Unraid push below can't leak
	// past a muted policy.
	if err != nil || !c.ScheduledSummary || attempted == 0 ||
		(c.On != "always" && c.On != "failure") {
		return
	}
	ok := failed == 0
	// "no failures" rather than "all succeeded": attempted counts every
	// scheduled item, including any skipped because their container is gone
	// (#57). A skip is not a failure, but it isn't a success either, so don't
	// overstate it. The skipped target still gets its own per-item warning
	// (recordAndNotifyContainerSkip).
	var summary string
	if ok {
		summary = fmt.Sprintf("Scheduled %s backup: %d items, no failures.", domain, attempted)
	} else {
		summary = fmt.Sprintf("Scheduled %s backup: %d of %d items failed.\n%s",
			domain, failed, attempted, formatItemFailures(failures))
	}
	// Reuse Send for the message channels with the Healthchecks ping suppressed
	// (ScheduledHealthchecksResult already sent the one aggregate HC ping). The summary
	// ctx carries no message-suppress flag, so Send delivers it (shouldSend still gates
	// an all-success summary out under On="failure").
	notify.Send(notify.WithHealthchecksSuppressed(ctx), c, domain,
		notify.Event{Title: "BombVault", Message: summary, OK: ok})
	if s.unraidGate(c.Unraid) && (c.On == "always" || !ok) {
		level := "normal"
		if !ok {
			level = "warning"
		}
		if e := s.sendUnraidNotify(ctx, "BombVault: scheduled "+domain+" backup", summary, level); e != nil {
			log.Printf("notify: unraid: %v", e)
		}
	}
}

// maxListedFailures caps how many failed items the scheduled summary lists
// individually before collapsing the rest into a "+N more" tail, so a
// night where 35 of 45 containers failed stays readable in a chat or email.
const maxListedFailures = 10

// formatItemFailures renders a scheduled run's per-item failures as "- name:
// reason" lines for the summary notification, capping the list at
// maxListedFailures with a "+N more" tail (#64). Each reason is scrubbed
// of absolute host paths, as the per-item notifyBackup does with its error
// text, so the aggregated summary leaks nothing the suppressed per-item
// messages would not have.
func formatItemFailures(failures []schedule.ItemFailure) string {
	lines := make([]string, 0, len(failures))
	for i, f := range failures {
		if i == maxListedFailures {
			lines = append(lines, fmt.Sprintf("+%d more", len(failures)-maxListedFailures))
			break
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", f.Name, scrubError(errors.New(f.Reason))))
	}
	return strings.Join(lines, "\n")
}

// sendUnraidNotify triggers Unraid's native notification system by running the
// host's notify script over SSH. level is "normal" | "warning" | "alert".
func (s *Service) sendUnraidNotify(ctx context.Context, subject, desc, level string) error {
	if s.ssh == nil {
		return errors.New("no SSH connection for Unraid notifications (set it up in Settings → VM Backup over SSH)")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_, err := s.ssh.Run(ctx, "/usr/local/emhttp/webGui/scripts/notify",
		"-e", "BombVault", "-s", subject, "-d", desc, "-i", level)
	return err
}

// TestNotify sends a test to every channel the (unsaved) config enables: the HTTP
// channels via notify.SendTest, plus the Unraid channel over SSH. It errors when
// nothing is configured or a configured channel fails, so the UI's Test button
// reflects the real result.
func (s *Service) TestNotify(ctx context.Context, c notify.Config) error {
	if !c.Configured() && !c.Unraid {
		return errors.New("no notification channel configured")
	}
	if c.Configured() {
		if err := notify.SendTest(ctx, c); err != nil {
			return err
		}
	}
	if c.Unraid {
		// TestNotify is a request-scoped, user-initiated action (the Settings
		// "Test" button), not a background best-effort job: silently reporting
		// success without attempting anything would be dishonest, so a
		// non-Unraid platform gets a clear, immediate refusal instead of an SSH
		// attempt that could only fail on the far end.
		if s.platformFn().Kind() != platform.KindUnraid {
			return fmt.Errorf("unraid: %w", s.unraidPlatformMismatchError("the Unraid notification channel"))
		}
		if err := s.sendUnraidNotify(ctx, "BombVault test notification",
			"If you see this in Unraid, BombVault notifications are working.", "normal"); err != nil {
			return fmt.Errorf("unraid: %w", err)
		}
	}
	return nil
}
