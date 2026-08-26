package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Run represents a single backup or restore operation.
type Run struct {
	ID         string `json:"id"`
	TargetID   string `json:"targetId"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	StartedAt  int64  `json:"startedAt"`
	FinishedAt *int64 `json:"finishedAt"`
	SnapshotID string `json:"snapshotId"`
	Bytes      int64  `json:"bytes"`
	Error      string `json:"error"`
	// Acknowledged is set once the user dismisses this failed run from the
	// dashboard error panel (#126); an acknowledged run no longer counts toward
	// the dashboard's failure badge.
	Acknowledged bool `json:"acknowledged"`
	// GroupID ties this run to the parent run of a multi-domain pass (e.g.
	// "Backup Everything"): every child run a pass produces carries
	// group_id = the parent run's id, so it can be traced back to the pass that
	// triggered it. '' (the default) means this run is not part of any group —
	// true for every run outside such a pass. Owned by SetRunGroup.
	GroupID string `json:"groupId"`
}

// StartRun records the beginning of a run and returns its ID.
func (r *Repo) StartRun(targetID, kind string) (string, error) {
	id := newID()
	_, err := r.db.Exec(`
		INSERT INTO runs (id, target_id, kind, status, started_at)
		VALUES (?, ?, ?, 'running', ?)`,
		id, targetID, kind, time.Now().Unix(),
	)
	if err != nil {
		return "", fmt.Errorf("StartRun: %w", err)
	}
	return id, nil
}

// FinishRun updates a run with its final status, snapshot ID, bytes, and optional error.
//
// It is also the ONLY writer of runs.completed, which is the whole point of that
// column: reaching here means the operation ran to its own conclusion and
// recorded the verdict, whatever the verdict was. The two writers that stamp
// finished_at WITHOUT that being true (ReapInterruptedRuns, FailRunningRun)
// leave completed at its 0 default, so a query that must distinguish "the pass
// ran" from "the row was closed out for a process that died mid-pass" can ask
// structurally instead of pattern-matching an error string. See
// LastEverythingPass, the query that needs it.
func (r *Repo) FinishRun(id, status, snapshotID string, bytes int64, errMsg string) error {
	now := time.Now().Unix()
	var snap, errCol any
	if snapshotID != "" {
		snap = snapshotID
	}
	if errMsg != "" {
		errCol = errMsg
	}
	res, err := r.db.Exec(`
		UPDATE runs SET status = ?, finished_at = ?, snapshot_id = ?, bytes = ?, error = ?, completed = 1
		WHERE id = ?`,
		status, now, snap, bytes, errCol, id,
	)
	if err != nil {
		return fmt.Errorf("FinishRun: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("FinishRun: run %s not found", id)
	}
	return nil
}

// FailRunningRun marks targetID's still-'running' run (if any) as failed, with
// errMsg as its recorded error. It is the single-target counterpart of
// ReapInterruptedRuns: that one only ever runs once at process startup (a
// 'running' row can only be an orphan of a PREVIOUS lifetime, since BombVault
// is a single process). This one lets a caller close out a run immediately,
// without waiting for a restart, the moment it knows a specific target's
// operation ended abnormally — namely api.Service's panic recovery, where the
// very goroutine that would have finished the run is the one that panicked,
// so nothing else will ever call FinishRun for it. Scoped to targetID (never
// global, unlike ReapInterruptedRuns) so it can never disturb a genuinely
// in-flight run for a DIFFERENT target. Returns the number of rows updated —
// 0 is a normal, harmless outcome (nothing was running for this target, e.g.
// the panic happened before StartRun was even reached), not an error.
//
// This assumes at most one operation is ever "running" against a given
// targetID at a time — true everywhere batchActive or a domain lock guards
// whatever called StartRun, which is every caller today except one known,
// accepted exception: api.Service.DownloadFlashZip holds a "restore" run open
// on store.FlashTargetID for the whole streamed-download duration WITHOUT
// taking either guard, so a concurrent StartBackupFlash panic's failStuckRun
// could in theory mark that still-in-flight download's run row failed instead
// of the backup's own. It self-heals — the download's own FinishRun overwrites
// the row by id once it completes — so the only real damage is a transiently
// wrong Activity Log entry, never lost data.
//
// It deliberately leaves runs.completed at 0: the run it closes out did NOT
// reach its own conclusion, and a due-gate that measures "when did this last
// run" must not read the panic instant as a completed pass.
func (r *Repo) FailRunningRun(targetID, errMsg string) (int64, error) {
	res, err := r.db.Exec(`
		UPDATE runs SET status = 'failed', finished_at = ?, error = ?
		WHERE target_id = ? AND status = 'running'`,
		time.Now().Unix(), errMsg, targetID,
	)
	if err != nil {
		return 0, fmt.Errorf("FailRunningRun: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// SetRunGroup stamps runID with groupID, tying it to the parent run of a
// multi-domain pass (e.g. "Backup Everything") so every child run it produces
// can later be traced back to it via a group_id-scoped query. Called through
// the same backup.Runs.Start choke point every domain orchestrator already
// uses (runsAdapter/startedRunsAdapter) when the pass's context carries a
// group id — every other caller today carries none, so this is never invoked
// outside a grouped pass. Best-effort bookkeeping: a runID that matches no row
// (e.g. already deleted) is NOT an error, since the caller treats this as
// fire-and-forget and must never fail a backup over it.
func (r *Repo) SetRunGroup(runID, groupID string) error {
	_, err := r.db.Exec(`UPDATE runs SET group_id = ? WHERE id = ?`, groupID, runID)
	if err != nil {
		return fmt.Errorf("SetRunGroup: %w", err)
	}
	return nil
}

// ReapInterruptedRuns marks any run still in 'running' as failed. It is meant to
// be called once at startup: BombVault is a single process, so a run left in
// 'running' is necessarily an orphan from a previous lifetime (the process
// crashed or was updated mid-backup) and can never still be in progress. Without
// this, such a run keeps a NULL bytes/finished_at and shows a perpetual "running"
// chip on the dashboard. Returns how many runs were reaped.
//
// Like FailRunningRun it leaves runs.completed at 0. The finished_at it writes is
// the RESTART instant, not the end of the work: an "everything" pass runs for
// hours, so a reboot mid-pass is exactly the case where a due-gate that trusted
// finished_at alone would call the abandoned pass "the pass that ran" and skip a
// whole interval of whole-server backups. See LastEverythingPass.
func (r *Repo) ReapInterruptedRuns() (int64, error) {
	res, err := r.db.Exec(`
		UPDATE runs
		SET status = 'failed', finished_at = ?, error = 'interrupted (BombVault restarted mid-run)'
		WHERE status = 'running'`, time.Now().Unix())
	if err != nil {
		return 0, fmt.Errorf("ReapInterruptedRuns: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// LastSuccessfulBackup returns the most recent successful backup run for targetID, or nil.
func (r *Repo) LastSuccessfulBackup(targetID string) (*Run, error) {
	row := r.db.QueryRow(`
		SELECT id, target_id, kind, status, started_at, finished_at, snapshot_id, bytes, error, acknowledged, group_id
		FROM runs
		WHERE target_id = ? AND kind = 'backup' AND status = 'success'
		ORDER BY started_at DESC
		LIMIT 1`, targetID)
	run, err := scanRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("LastSuccessfulBackup: %w", err)
	}
	return &run, nil
}

// LastRunForTarget returns the most recent backup run for targetID regardless of
// status (success, failed, skipped, running), or nil when there is none. Used to
// debounce repeated "container missing" skip warnings: warn on the first miss,
// stay quiet while the target keeps being skipped.
func (r *Repo) LastRunForTarget(targetID string) (*Run, error) {
	row := r.db.QueryRow(`
		SELECT id, target_id, kind, status, started_at, finished_at, snapshot_id, bytes, error, acknowledged, group_id
		FROM runs
		WHERE target_id = ? AND kind = 'backup'
		ORDER BY started_at DESC
		LIMIT 1`, targetID)
	run, err := scanRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("LastRunForTarget: %w", err)
	}
	return &run, nil
}

// lastBackupAmongChunk bounds how many target ids go into one IN (...) clause.
// SQLite's compiled-in parameter limit is the constraint; a box with hundreds
// of containers is unusual but a file-set list is user-defined, so the query is
// chunked rather than trusting the list to stay small.
const lastBackupAmongChunk = 400

// LastSuccessfulBackupAmong returns the most recent successful backup among the
// GIVEN target ids, or a zero time when none of them has ever been backed up.
//
// It exists because a DOMAIN's everyN due-gate must measure the items that
// domain's scheduled pass actually covers, and that set is not "every row in
// the table": per-item schedule overrides (#121) move an item onto its own cron
// entry and REMOVE it from the domain run (schedule.DomainRunTargets), and an
// item that is not included in the schedule was never part of it either. The
// caller decides the set; this only answers for it. See schedule's
// ContainersDueGate/VMsDueGate/FilesDueGate for the composition.
//
// An EMPTY id list is a definite "none of the items this pass covers has ever
// been backed up" and reports a zero time, which the gate reads as due — the
// same answer a fresh install gets. The alternative (treating it as "not due")
// would silently freeze a domain forever the moment its last item was excluded.
func (r *Repo) LastSuccessfulBackupAmong(ids []string) (time.Time, error) {
	var newest time.Time
	for start := 0; start < len(ids); start += lastBackupAmongChunk {
		end := min(start+lastBackupAmongChunk, len(ids))
		chunk := ids[start:end]

		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(chunk)), ",")
		args := make([]any, 0, len(chunk)+1)
		for _, id := range chunk {
			args = append(args, id)
		}
		args = append(args, saneStampCutoff())
		row := r.db.QueryRow(`
			SELECT finished_at
			FROM runs
			WHERE kind = 'backup' AND status = 'success' AND finished_at IS NOT NULL
			  AND target_id IN (`+placeholders+`)`+sanePastStamp+`
			ORDER BY finished_at DESC
			LIMIT 1`, args...) //nolint:gosec // G202: placeholders is a generated "?,?,…" list, never user text; every id is a bound parameter
		ts, err := scanLastBackupTime(row, "LastSuccessfulBackupAmong")
		if err != nil {
			return time.Time{}, err
		}
		if ts.After(newest) {
			newest = ts
		}
	}
	return newest, nil
}

// LastSuccessfulContainerBackup returns the time of the most recent successful
// backup run across ALL container targets, or a zero time when there has been
// none. It is scoped to container targets (target_id in the `targets` table) so
// a VM backup never satisfies it.
//
// This is the domain's PROTECTION currency (the dashboard's RPO chip, the
// overdue watchdog): "when was anything in this domain last backed up". It is
// deliberately NOT the everyN due-gate any more — that asks a different
// question ("has the pass's interval elapsed?") and must not be answered by an
// item the pass does not even run; see LastSuccessfulBackupAmong.
func (r *Repo) LastSuccessfulContainerBackup() (time.Time, error) {
	row := r.db.QueryRow(`
		SELECT finished_at
		FROM runs
		WHERE kind = 'backup' AND status = 'success' AND finished_at IS NOT NULL
		  AND target_id IN (SELECT id FROM targets)`+sanePastStamp+`
		ORDER BY finished_at DESC
		LIMIT 1`, saneStampCutoff())
	return scanLastBackupTime(row, "LastSuccessfulContainerBackup")
}

// LastSuccessfulVMBackup is the VM-domain counterpart of
// LastSuccessfulContainerBackup, scoped to VM targets (target_id in the `vms`
// table). Drives the VMs domain everyN due-gate.
func (r *Repo) LastSuccessfulVMBackup() (time.Time, error) {
	row := r.db.QueryRow(`
		SELECT finished_at
		FROM runs
		WHERE kind = 'backup' AND status = 'success' AND finished_at IS NOT NULL
		  AND target_id IN (SELECT id FROM vms)`+sanePastStamp+`
		ORDER BY finished_at DESC
		LIMIT 1`, saneStampCutoff())
	return scanLastBackupTime(row, "LastSuccessfulVMBackup")
}

// LastSuccessfulFilesBackup is the files-domain counterpart of
// LastSuccessfulContainerBackup, scoped to file-set targets (target_id in the
// `file_sets` table). Drives the files domain everyN due-gate.
func (r *Repo) LastSuccessfulFilesBackup() (time.Time, error) {
	row := r.db.QueryRow(`
		SELECT finished_at
		FROM runs
		WHERE kind = 'backup' AND status = 'success' AND finished_at IS NOT NULL
		  AND target_id IN (SELECT id FROM file_sets)`+sanePastStamp+`
		ORDER BY finished_at DESC
		LIMIT 1`, saneStampCutoff())
	return scanLastBackupTime(row, "LastSuccessfulFilesBackup")
}

// FlashTargetID is the reserved runs.target_id for the singleton flash domain
// (the Unraid USB). Flash has no per-item table, so its runs are tagged with
// this fixed id — distinct from the hex/UUID ids of container and VM targets,
// so it never collides with or pollutes the other domains' gates.
const FlashTargetID = "flash"

// LastSuccessfulFlashBackup drives the flash domain everyN due-gate, scoped to
// the reserved flash target id.
func (r *Repo) LastSuccessfulFlashBackup() (time.Time, error) {
	row := r.db.QueryRow(`
		SELECT finished_at
		FROM runs
		WHERE kind = 'backup' AND status = 'success' AND finished_at IS NOT NULL AND target_id = ?`+sanePastStamp+`
		ORDER BY finished_at DESC
		LIMIT 1`, FlashTargetID, saneStampCutoff())
	return scanLastBackupTime(row, "LastSuccessfulFlashBackup")
}

// ConfigTargetID is the reserved runs.target_id for the singleton config self-
// backup domain (BombVault's own /config). Like FlashTargetID it is a fixed
// literal, distinct from the hex/UUID ids of container and VM targets.
const ConfigTargetID = "config"

// LastSuccessfulConfigBackup drives the config domain everyN due-gate, scoped to
// the reserved config target id.
func (r *Repo) LastSuccessfulConfigBackup() (time.Time, error) {
	row := r.db.QueryRow(`
		SELECT finished_at
		FROM runs
		WHERE kind = 'backup' AND status = 'success' AND finished_at IS NOT NULL AND target_id = ?`+sanePastStamp+`
		ORDER BY finished_at DESC
		LIMIT 1`, ConfigTargetID, saneStampCutoff())
	return scanLastBackupTime(row, "LastSuccessfulConfigBackup")
}

// EverythingTargetID is the reserved runs.target_id for the singleton
// "Backup Everything" pass (a 6th, independent pseudo-domain that runs
// containers/vms/flash/files/config in sequence). Like FlashTargetID and
// ConfigTargetID it is a fixed literal, distinct from the hex/UUID ids of
// container and VM targets, so it never collides with or pollutes the other
// domains' gates.
const EverythingTargetID = "everything"

// LastEverythingPass drives the "Backup Everything" pass's everyN due-gate: when
// the pass last COMPLETED, whatever it concluded. A run that is still running
// does not count; a run that reached its own end does, success or not.
//
// "Reached its own end" is runs.completed, not finished_at, and the distinction
// is the whole gate. Two writers stamp finished_at on a pass that never
// completed: ReapInterruptedRuns closes out every 'running' row at EVERY
// startup, and FailRunningRun does the same on the panic path. The parent run
// here is held open across containers → vms → flash → files → config plus the
// batched prune and off-site replication — hours — so a reboot or a container
// update mid-pass is an ordinary event, and it writes a finished_at at the
// RESTART instant. Read as "the pass ran", that stamp shuts the everyN gate for
// the next N days AND the anacron catch-up along with it (the stamp lies after
// the missed fire, so nothing looks missed): a whole interval of whole-server
// backups skipped, silently, because the box rebooted. completed is set by
// FinishRun alone, so only a pass that recorded its own verdict answers here.
//
// The status is deliberately not part of it. The parent run is all-or-nothing —
// "success" iff EVERY domain step had zero item failures (internal/api/
// everything.go) — so one item that fails every night (a container whose volume
// is gone, or the flash step on a host that has no /boot mount at all, which is
// a supported deployment) means the pass NEVER records a success. Gated on
// success, the everyN due-gate then reads "never ran" on every daily trigger
// and runs the whole containers+VMs+flash+files+config pass, plus the batched
// prune and off-site replication for three domains, EVERY NIGHT instead of
// every N days: maximum expense at exactly the moment the system is already
// unhealthy, and silently, since a gate that opens logs nothing.
//
// This is the same rule the drill and tamper passes already apply for the same
// reason (internal/store/schedule_job_runs.go: "an expensive multi-task pass
// records even when some tasks failed"), and it matches how a daily/weekly
// cadence behaves — a failed pass is not retried before its next fire either.
// The failure itself is not lost: it is the parent run's own status, its
// breakdown, and the per-item notifications.
func (r *Repo) LastEverythingPass() (time.Time, error) {
	row := r.db.QueryRow(`
		SELECT finished_at
		FROM runs
		WHERE kind = 'backup' AND completed = 1 AND finished_at IS NOT NULL AND target_id = ?`+sanePastStamp+`
		ORDER BY finished_at DESC
		LIMIT 1`, EverythingTargetID, saneStampCutoff())
	return scanLastBackupTime(row, "LastEverythingPass")
}

// scanLastBackupTime reads the single nullable finished_at column from a
// last-successful-backup query, mapping no-rows / NULL to a zero time.
//
// It is also where a stamp from the FUTURE is refused. Every last-run query in
// this file funnels through here, so this is the one place that rule has to be
// right (LastScheduleJobRun applies the same rule for the three job-run
// schedules, which do not come through here). See SanitizeRecordedTime.
func scanLastBackupTime(row *sql.Row, label string) (time.Time, error) {
	var ts sql.NullInt64
	if err := row.Scan(&ts); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("%s: %w", label, err)
	}
	if !ts.Valid {
		return time.Time{}, nil
	}
	return SanitizeRecordedTime(time.Unix(ts.Int64, 0), time.Now()), nil
}

// futureStampTolerance is how far ahead of the reading clock a recorded
// timestamp may sit and still count as a real measurement. It absorbs ordinary
// skew — a stamp taken moments ago, a second-resolution column rounding up, an
// NTP slew mid-run — without absorbing a wrong-clock stamp, which is off by days
// or years, never by minutes.
const futureStampTolerance = 5 * time.Minute

// sanePastStamp is the SQL half of the same rule, and it is the half that
// matters. Every currency query in this file is an `ORDER BY finished_at DESC
// LIMIT 1`, so refusing the value AFTER the read is not enough: the poisoned row
// still WINS the ordering, which means a correctly-stamped run that lands
// afterwards is never seen and the schedule never heals — it just runs on every
// trigger forever instead of on its interval. Excluding the row in the query
// makes the answer the newest SANE success, so one run restores the cadence.
//
// It is a fragment rather than seven copies for the reason this project keeps
// relearning: a guard spelled out once per call site is a guard missing at the
// call site added next. Every use pairs it with saneStampCutoff() as the LAST
// bound parameter.
const sanePastStamp = ` AND finished_at <= ?`

// saneStampCutoff is sanePastStamp's bound parameter: the newest instant a
// recorded stamp may carry and still be a measurement of the past.
func saneStampCutoff() int64 { return time.Now().Add(futureStampTolerance).Unix() }

// SanitizeRecordedTime reports a recorded "when did this last happen" instant,
// reading one that lies in the FUTURE as the zero time ("never happened").
//
// A stamp later than now cannot be a measurement of the past. It is what a box
// with a wrong clock wrote — a dead CMOS battery, or the early-boot window
// before NTP steps the clock — after which the clock was corrected back. Every
// consumer of these timestamps does the same arithmetic, now − last, and every
// one of them fails in the SILENT direction on a negative result:
//
//   - the everyN due-gate skips every fire from then on, forever, because a
//     negative elapsed time is below any interval (schedule.EveryNDue);
//   - the anacron catch-up mirrors that arithmetic and never flags the miss;
//   - the overdue-backup watchdog reads the domain as freshly current and never
//     raises the alert it exists to raise;
//   - the dashboard's RPO chip reports the domain green for as long as the bogus
//     stamp stays in the future.
//
// Worse, a poisoned row keeps WINNING these queries: they are all
// `ORDER BY finished_at DESC LIMIT 1`, so a later, correctly-stamped run never
// displaces it and the corruption never ages out. Refusing it here means the
// gate measures the newest SANE success instead, so a single run heals the
// schedule and the cadence is correct from that run on.
//
// The row itself is not hidden. ListRuns and the run history still report it
// exactly as recorded; only the currency arithmetic declines to measure an
// elapsed interval against an instant that has not happened yet.
func SanitizeRecordedTime(at, now time.Time) time.Time {
	if at.IsZero() || at.After(now.Add(futureStampTolerance)) {
		return time.Time{}
	}
	return at
}

// ListRuns returns up to limit recent runs across all targets, newest first.
func (r *Repo) ListRuns(limit int) ([]Run, error) {
	rows, err := r.db.Query(`
		SELECT id, target_id, kind, status, started_at, finished_at, snapshot_id, bytes, error, acknowledged, group_id
		FROM runs
		ORDER BY started_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("ListRuns: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []Run
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

// RunsSince returns all runs with started_at >= since (unix seconds), newest
// first. Used by the dashboard's backup-health heatmap to bucket a window of
// runs by day and domain.
func (r *Repo) RunsSince(since int64) ([]Run, error) {
	rows, err := r.db.Query(`
		SELECT id, target_id, kind, status, started_at, finished_at, snapshot_id, bytes, error, acknowledged, group_id
		FROM runs
		WHERE started_at >= ?
		ORDER BY started_at DESC`, since)
	if err != nil {
		return nil, fmt.Errorf("RunsSince: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []Run
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

// AcknowledgeRuns marks the given run ids as acknowledged so the dashboard's
// error panel can dismiss them from the failure count without editing SQLite by
// hand (#126). It is a no-op (0 rows, no query) on an empty id list. Returns the
// number of rows affected.
func (r *Repo) AcknowledgeRuns(ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	//nolint:gosec // G202: only "?" placeholders are concatenated; the ids are passed as parameterized args below, never interpolated.
	q := "UPDATE runs SET acknowledged = 1 WHERE id IN (" + strings.Join(placeholders, ", ") + ")"
	res, err := r.db.Exec(q, args...)
	if err != nil {
		return 0, fmt.Errorf("AcknowledgeRuns: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// AcknowledgeAllFailed marks every currently-unacknowledged failed run as
// acknowledged, clearing the whole dashboard error badge in one call (#126).
// Returns the number of rows affected.
func (r *Repo) AcknowledgeAllFailed() (int64, error) {
	res, err := r.db.Exec("UPDATE runs SET acknowledged = 1 WHERE status = 'failed' AND acknowledged = 0")
	if err != nil {
		return 0, fmt.Errorf("AcknowledgeAllFailed: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// RunCounts returns the total number of backup runs per domain ("containers" |
// "vms" | "flash" | "config" | "files") and status ("success" | "failed"), keyed
// [domain][status]. Domain is attributed the same way as the last-successful
// helpers: container targets live in `targets`, VM targets in `vms`, file-set
// targets in `file_sets`, and the singleton flash and config domains use the
// reserved FlashTargetID and ConfigTargetID. Only finished backup runs
// (success/failed) are counted; "running" runs are skipped. A domain/status
// with no runs is absent from the map (the caller defaults it to 0). Drives
// the Prometheus `bombvault_runs_total` counter.
func (r *Repo) RunCounts() (map[string]map[string]int, error) {
	rows, err := r.db.Query(`
		SELECT
		  CASE
		    WHEN target_id = ?                              THEN 'config'
		    WHEN target_id = ?                              THEN 'flash'
		    WHEN target_id IN (SELECT id FROM vms)          THEN 'vms'
		    WHEN target_id IN (SELECT id FROM file_sets)    THEN 'files'
		    WHEN target_id IN (SELECT id FROM targets)      THEN 'containers'
		    ELSE ''
		  END AS domain,
		  status,
		  count(*) AS n
		FROM runs
		WHERE kind = 'backup' AND status IN ('success', 'failed')
		GROUP BY domain, status`, ConfigTargetID, FlashTargetID)
	if err != nil {
		return nil, fmt.Errorf("RunCounts: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	out := map[string]map[string]int{}
	for rows.Next() {
		var domain, status string
		var n int
		if sErr := rows.Scan(&domain, &status, &n); sErr != nil {
			return nil, fmt.Errorf("RunCounts: %w", sErr)
		}
		if domain == "" {
			continue // run for a deleted/unknown target — not attributable to a domain
		}
		if out[domain] == nil {
			out[domain] = map[string]int{}
		}
		out[domain][status] = n
	}
	return out, rows.Err()
}

func scanRun(s scanner) (Run, error) {
	var run Run
	var finishedAt, bytes sql.NullInt64
	var snapID, errCol sql.NullString
	err := s.Scan(
		&run.ID, &run.TargetID, &run.Kind, &run.Status,
		&run.StartedAt, &finishedAt, &snapID, &bytes, &errCol, &run.Acknowledged, &run.GroupID,
	)
	if err != nil {
		return Run{}, err
	}
	if finishedAt.Valid {
		run.FinishedAt = &finishedAt.Int64
	}
	if bytes.Valid {
		run.Bytes = bytes.Int64
	}
	if snapID.Valid {
		run.SnapshotID = snapID.String
	}
	if errCol.Valid {
		run.Error = errCol.String
	}
	return run, nil
}
