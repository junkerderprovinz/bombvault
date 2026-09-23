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
	// dashboard error panel, which takes it out of the failure badge.
	Acknowledged bool `json:"acknowledged"`
	// GroupID is the id of the parent run of a multi-domain pass such as
	// "Backup Everything", or empty for a run outside one. Set by SetRunGroup.
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

// FinishRun records a run's final status, snapshot ID, bytes and optional
// error. It is the only writer of runs.completed, which lets LastEverythingPass
// tell a pass that reached its end from one whose process died.
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

// FailRunningRun marks targetID's running run, if any, as failed and returns
// how many rows it changed. api.Service's panic recovery needs it because the
// goroutine that would have called FinishRun is the one that panicked.
//
// It assumes one running operation per target, which batchActive or a domain
// lock guarantees except for api.Service.DownloadFlashZip: a panicking flash
// backup could mark the download's "restore" run failed until the download's
// own FinishRun overwrites it.
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

// SetRunGroup ties runID to the parent run of a multi-domain pass such as
// "Backup Everything". An unknown runID is not an error, because this
// bookkeeping must not fail a backup.
func (r *Repo) SetRunGroup(runID, groupID string) error {
	_, err := r.db.Exec(`UPDATE runs SET group_id = ? WHERE id = ?`, groupID, runID)
	if err != nil {
		return fmt.Errorf("SetRunGroup: %w", err)
	}
	return nil
}

// Reasons BombVault itself writes into runs.error; the rest of that column
// comes from restic, rclone or Docker. They are English sentences rather than
// codes because people read the column in `docker logs` and support pastes,
// and web/src/lib/runReason.ts translates them by exact match, which
// TestRunReasonsMatchTheFrontend keeps in step.
const (
	// ReasonInterrupted is written by the startup sweep for runs of a previous
	// process, which can no longer say why it stopped.
	ReasonInterrupted = "interrupted (BombVault restarted mid-run)"

	// ReasonShutdown is written by a process that is shutting down, which tells
	// a stopped container apart from a crash.
	ReasonShutdown = "aborted: BombVault was shut down"

	// ReasonContainerGone marks a definition whose container is no longer on the
	// host. Not a failure: BombVault chose not to run.
	ReasonContainerGone = "container no longer exists on the host"

	// ReasonCancelled is written when the user cancels a running backup, so the
	// row does not read like a failure.
	ReasonCancelled = "cancelled by the user"
)

// ReapInterruptedRuns marks every run still in 'running' as failed and returns
// how many it changed. It runs once at startup: BombVault is a single process,
// so such a run was orphaned by a crash or an update. The finished_at it
// writes is the restart time, not the end of the work.
func (r *Repo) ReapInterruptedRuns() (int64, error) {
	res, err := r.db.Exec(`
		UPDATE runs
		SET status = 'failed', finished_at = ?, error = ?
		WHERE status = 'running'`, time.Now().Unix(), ReasonInterrupted)
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

// LastRunForTarget returns the most recent backup run for targetID whatever its
// status, or nil. It lets a "container missing" skip warn only on the first
// miss.
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

// lastBackupAmongChunk bounds how many target ids go into one IN (...) clause,
// keeping each query under SQLite's parameter limit however long the list is.
const lastBackupAmongChunk = 400

// LastSuccessfulBackupAmong returns the most recent successful backup among
// ids, or the zero time when none of them has one. A domain's everyN due-gate
// passes only the items its scheduled pass covers (see schedule's
// ContainersDueGate), since overrides and exclusions move items out of it.
//
// An empty list reports the zero time, which the gate reads as due; reading it
// as not due would freeze a domain once its last item was excluded.
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
		//nolint:gosec // G202: `placeholders` is a generated "?,?,…" list sized from
		// len(chunk), never user text; every id travels as a bound parameter in args.
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
// backup of any container target, or the zero time. It feeds the dashboard's
// RPO chip and the overdue watchdog; the everyN due-gate uses
// LastSuccessfulBackupAmong, which counts only the items the pass runs.
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

// LastSuccessfulVMBackup is LastSuccessfulContainerBackup for VM targets.
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

// LastSuccessfulFilesBackup is LastSuccessfulContainerBackup for file sets.
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
// (the Unraid USB). Flash has no per-item table, so its runs carry this fixed
// id, which cannot collide with the hex ids of other targets.
const FlashTargetID = "flash"

// LastSuccessfulFlashBackup returns the time of the last successful flash
// backup, for the flash domain's everyN due-gate.
func (r *Repo) LastSuccessfulFlashBackup() (time.Time, error) {
	row := r.db.QueryRow(`
		SELECT finished_at
		FROM runs
		WHERE kind = 'backup' AND status = 'success' AND finished_at IS NOT NULL AND target_id = ?`+sanePastStamp+`
		ORDER BY finished_at DESC
		LIMIT 1`, FlashTargetID, saneStampCutoff())
	return scanLastBackupTime(row, "LastSuccessfulFlashBackup")
}

// ConfigTargetID is the reserved runs.target_id for the singleton config
// domain, the backup of BombVault's own /config.
const ConfigTargetID = "config"

// LastSuccessfulConfigBackup returns the time of the last successful config
// backup, for the config domain's everyN due-gate.
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
// "Backup Everything" pass, which runs every domain in sequence.
const EverythingTargetID = "everything"

// LastEverythingPass returns when the "Backup Everything" pass last reached its
// own end, success or not, for its everyN due-gate.
//
// It reads runs.completed rather than finished_at, because ReapInterruptedRuns
// and FailRunningRun also stamp finished_at and a reboot during an hours-long
// pass is ordinary. Counting that stamp would close the gate, and the anacron
// catch-up with it, for a whole interval.
//
// The status is ignored because the parent run succeeds only when no item
// failed. One item that fails every night, such as a container whose volume is
// gone, would otherwise repeat the whole pass, prune and off-site replication
// included, every night instead of every N days. RecordScheduleJobRun follows
// the same rule for the drill and tamper passes.
func (r *Repo) LastEverythingPass() (time.Time, error) {
	row := r.db.QueryRow(`
		SELECT finished_at
		FROM runs
		WHERE kind = 'backup' AND completed = 1 AND finished_at IS NOT NULL AND target_id = ?`+sanePastStamp+`
		ORDER BY finished_at DESC
		LIMIT 1`, EverythingTargetID, saneStampCutoff())
	return scanLastBackupTime(row, "LastEverythingPass")
}

// scanLastBackupTime reads the nullable finished_at of a last-backup query.
// No rows, NULL and a future stamp (SanitizeRecordedTime) all give the zero
// time.
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
// timestamp may be and still count. It absorbs ordinary skew (second rounding,
// an NTP slew mid-run) but not a wrong clock, which is off by days or years.
const futureStampTolerance = 5 * time.Minute

// sanePastStamp filters future stamps inside the query. The currency queries
// are `ORDER BY finished_at DESC LIMIT 1`, so a future row would win the
// ordering and hide every correctly stamped run after it. Pair it with
// saneStampCutoff() as the last bound parameter.
const sanePastStamp = ` AND finished_at <= ?`

// saneStampCutoff is sanePastStamp's bound parameter: the newest instant a
// recorded stamp may carry and still be a measurement of the past.
func saneStampCutoff() int64 { return time.Now().Add(futureStampTolerance).Unix() }

// SanitizeRecordedTime returns at, or the zero time when at lies in the future.
//
// A future stamp comes from a box whose clock was wrong and later corrected (a
// dead CMOS battery, early boot before NTP). Every consumer computes now minus
// last, and a negative result fails silently: the everyN gate and the anacron
// catch-up never fire again, the overdue watchdog never alerts and the RPO chip
// stays green.
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

// RunsSince returns all runs started at or after since (unix seconds), newest
// first, for the dashboard's backup-health heatmap.
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

// AcknowledgeRuns marks the given runs as acknowledged, removing them from the
// dashboard's failure count, and returns the number of rows affected. An empty
// list runs no query.
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

// AcknowledgeAllFailed acknowledges every failed run not yet acknowledged,
// clearing the dashboard's error badge, and returns how many it changed.
func (r *Repo) AcknowledgeAllFailed() (int64, error) {
	res, err := r.db.Exec("UPDATE runs SET acknowledged = 1 WHERE status = 'failed' AND acknowledged = 0")
	if err != nil {
		return 0, fmt.Errorf("AcknowledgeAllFailed: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// RunCounts returns the number of finished backup runs keyed by domain and
// then by status ("success" or "failed"), for the Prometheus
// `bombvault_runs_total` counter. A missing entry means 0.
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
			continue // deleted or unknown target
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
