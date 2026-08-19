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
		UPDATE runs SET status = ?, finished_at = ?, snapshot_id = ?, bytes = ?, error = ?
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

// LastSuccessfulContainerBackup returns the time of the most recent successful
// backup run across ALL container targets, or a zero time when there has been
// none. This is used by the scheduler's everyN due-gate to decide whether the
// containers domain is due for a run. It is scoped to container targets
// (target_id in the `targets` table) so a VM backup never satisfies the gate.
func (r *Repo) LastSuccessfulContainerBackup() (time.Time, error) {
	row := r.db.QueryRow(`
		SELECT finished_at
		FROM runs
		WHERE kind = 'backup' AND status = 'success' AND finished_at IS NOT NULL
		  AND target_id IN (SELECT id FROM targets)
		ORDER BY finished_at DESC
		LIMIT 1`)
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
		  AND target_id IN (SELECT id FROM vms)
		ORDER BY finished_at DESC
		LIMIT 1`)
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
		  AND target_id IN (SELECT id FROM file_sets)
		ORDER BY finished_at DESC
		LIMIT 1`)
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
		WHERE kind = 'backup' AND status = 'success' AND finished_at IS NOT NULL AND target_id = ?
		ORDER BY finished_at DESC
		LIMIT 1`, FlashTargetID)
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
		WHERE kind = 'backup' AND status = 'success' AND finished_at IS NOT NULL AND target_id = ?
		ORDER BY finished_at DESC
		LIMIT 1`, ConfigTargetID)
	return scanLastBackupTime(row, "LastSuccessfulConfigBackup")
}

// EverythingTargetID is the reserved runs.target_id for the singleton
// "Backup Everything" pass (a 6th, independent pseudo-domain that runs
// containers/vms/flash/files/config in sequence). Like FlashTargetID and
// ConfigTargetID it is a fixed literal, distinct from the hex/UUID ids of
// container and VM targets, so it never collides with or pollutes the other
// domains' gates.
const EverythingTargetID = "everything"

// LastSuccessfulEverythingBackup drives the "Backup Everything" pass's everyN
// due-gate, scoped to the reserved everything target id.
func (r *Repo) LastSuccessfulEverythingBackup() (time.Time, error) {
	row := r.db.QueryRow(`
		SELECT finished_at
		FROM runs
		WHERE kind = 'backup' AND status = 'success' AND finished_at IS NOT NULL AND target_id = ?
		ORDER BY finished_at DESC
		LIMIT 1`, EverythingTargetID)
	return scanLastBackupTime(row, "LastSuccessfulEverythingBackup")
}

// scanLastBackupTime reads the single nullable finished_at column from a
// last-successful-backup query, mapping no-rows / NULL to a zero time.
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
	return time.Unix(ts.Int64, 0), nil
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
