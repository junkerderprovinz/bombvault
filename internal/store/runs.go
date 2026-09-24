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
	// "Backup Everything", or empty for a run outside one.
	GroupID string `json:"groupId"`
	// StartedVia names what asked for this run when the audit trail has a name
	// for it: "mcp" for the MCP endpoint, empty for the web interface and the
	// scheduler.
	StartedVia string `json:"startedVia"`
	// StartedViaKey is the mcp_keys.id behind an MCP-started run, so the
	// Activity log can name the client even after the key is revoked.
	StartedViaKey string `json:"startedViaKey"`
}

// runCols is the column list of every full run query, in the order scanRun
// reads them.
const runCols = `id, target_id, kind, status, started_at, finished_at, snapshot_id, bytes, error, acknowledged, group_id, started_via, started_via_key`

// RunMeta is what a caller knows about a run beyond its target and kind: the
// pass it belongs to and who asked for it.
type RunMeta struct {
	GroupID       string
	StartedVia    string
	StartedViaKey string
}

// StartRunWith records the beginning of a run with its group and origin and
// returns its ID. Both travel in the INSERT that creates the row, so no run can
// exist without the audit trail that explains it.
func (r *Repo) StartRunWith(targetID, kind string, meta RunMeta) (string, error) {
	id := newID()
	_, err := r.db.Exec(`
		INSERT INTO runs (id, target_id, kind, status, started_at, group_id, started_via, started_via_key)
		VALUES (?, ?, ?, 'running', ?, ?, ?, ?)`,
		id, targetID, kind, time.Now().Unix(), meta.GroupID, meta.StartedVia, meta.StartedViaKey,
	)
	if err != nil {
		return "", fmt.Errorf("StartRunWith: %w", err)
	}
	return id, nil
}

// StartRun records the beginning of a run the web interface or the scheduler
// asked for.
func (r *Repo) StartRun(targetID, kind string) (string, error) {
	return r.StartRunWith(targetID, kind, RunMeta{})
}

// RunMetrics are the source figures restic reports for a run, in the units the
// runs table stores them in.
type RunMetrics struct {
	SourceBytes, SourceFiles, FilesNew, ResticMS int64
	HasParent                                    *bool
}

// RunFinished names what a finish touched. A finish by id sets RunID, and
// FailRunningRun, which has no id to give, sets TargetID.
type RunFinished struct{ RunID, TargetID string }

// SetRunFinishedHook installs fn, called after every finish that changed a row.
// fn must not block and must not call back into the store: it runs on the
// goroutine that finished the run, which holds the database's single connection.
func (r *Repo) SetRunFinishedHook(fn func(RunFinished)) {
	r.runFinishedMu.Lock()
	defer r.runFinishedMu.Unlock()
	r.runFinished = fn
}

func (r *Repo) notifyRunFinished(f RunFinished) {
	r.runFinishedMu.RLock()
	fn := r.runFinished
	r.runFinishedMu.RUnlock()
	if fn != nil {
		fn(f)
	}
}

// FinishRun records a run's final status, snapshot ID, bytes and optional
// error. It is the only writer of runs.completed, which lets LastEverythingPass
// tell a pass that reached its end from one whose process died.
func (r *Repo) FinishRun(id, status, snapshotID string, bytes int64, errMsg string) error {
	return r.finishRun(id, status, snapshotID, bytes, errMsg, nil, "")
}

// FinishRunMeasured is FinishRun plus what restic read and the fingerprint of
// the selection the run covered. Nil metrics and an empty fingerprint leave
// those columns NULL, which is what tells an unmeasured run from one that
// measured a source of nothing.
func (r *Repo) FinishRunMeasured(id, status, snapshotID string, bytes int64, errMsg string, m *RunMetrics, fp string) error {
	return r.finishRun(id, status, snapshotID, bytes, errMsg, m, fp)
}

func (r *Repo) finishRun(id, status, snapshotID string, bytes int64, errMsg string, m *RunMetrics, fp string) error {
	now := time.Now().Unix()
	var snap, errCol any
	if snapshotID != "" {
		snap = snapshotID
	}
	if errMsg != "" {
		errCol = errMsg
	}
	var sourceBytes, sourceFiles, filesNew, resticMS, hasParent, selectionFP any
	if m != nil {
		sourceBytes, sourceFiles = m.SourceBytes, m.SourceFiles
		filesNew, resticMS = m.FilesNew, m.ResticMS
		if m.HasParent != nil {
			hasParent = boolToInt(*m.HasParent)
		}
	}
	if fp != "" {
		selectionFP = fp
	}
	res, err := r.db.Exec(`
		UPDATE runs
		SET status = ?, finished_at = ?, snapshot_id = ?, bytes = ?, error = ?, completed = 1,
		    source_bytes = ?, source_files = ?, files_new = ?, restic_ms = ?, has_parent = ?, selection_fp = ?
		WHERE id = ?`,
		status, now, snap, bytes, errCol,
		sourceBytes, sourceFiles, filesNew, resticMS, hasParent, selectionFP, id,
	)
	if err != nil {
		return fmt.Errorf("FinishRun: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("FinishRun: run %s not found", id)
	}
	r.notifyRunFinished(RunFinished{RunID: id})
	return nil
}

func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
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
	if n > 0 {
		r.notifyRunFinished(RunFinished{TargetID: targetID})
	}
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

// Why a database dump did not produce a snapshot. Each of these may carry a
// detail after a ": " separator, the scrubbed tail of the tool's own output;
// the frontend translates the head by prefix and shows the detail verbatim.
// None of them may begin another one, or a detail could not be told from a
// longer reason (TestRunReasonsAreDistinct).
const (
	ReasonDBDumpAuth          = "database dump failed: the database refused the login"
	ReasonDBDumpPrivileges    = "database dump failed: the database user lacks a privilege the dump needs"
	ReasonDBDumpUnreachable   = "database dump failed: the database server did not accept a connection"
	ReasonDBDumpNoClient      = "database dump failed: no dump tool found in the container"
	ReasonDBDumpSecret        = "database dump failed: a password file could not be read"
	ReasonDBDumpNoCredentials = "database dump failed: no usable credentials in the container settings"
	ReasonDBDumpNotRunning    = "database dump failed: the container is paused or restarting"
	ReasonDBDumpNeedsUpgrade  = "database dump failed: the database's system tables need an upgrade"
	ReasonDBDumpTimeout       = "database dump failed: time limit reached"
	ReasonDBDumpBackupCap     = "database dump failed: the backup's own time limit was reached"
	ReasonDBDumpStalled       = "database dump failed: no progress"
	ReasonDBDumpEmpty         = "database dump failed: the dump was empty"
	ReasonDBDumpIncomplete    = "database dump failed: the dump ended before its completion marker"
	ReasonDBDumpTool          = "database dump failed: the dump tool reported an error"
	ReasonDBDumpDocker        = "database dump failed: Docker refused the command"
	ReasonDBDumpRepository    = "database dump failed: the repository did not accept it"
	ReasonDBDumpHelper        = "database dump failed: the dump helper gave no result"
	ReasonDBDumpMismatch      = "database dump failed: the stored size does not match what was dumped"

	// ReasonDBDumpLeftover is the one dump failure whose run carries a
	// snapshot_id: restic wrote a snapshot that could not be removed again, and
	// the dump list marks that one as damaged. Its detail is the short id.
	ReasonDBDumpLeftover = "database dump failed: a damaged dump snapshot could not be removed"
)

// Notes on a successful run, shown in a warning tone: the work was done, but
// not as completely as the user would expect.
const (
	// NoteDBDumpOneDatabase says the credentials in the container reach one
	// database rather than the whole server.
	NoteDBDumpOneDatabase = "database dump covers one database only"

	// NoteDBDumpNotRecorded sits on the backup run, because it is written
	// exactly when the dump's own run could not be started.
	NoteDBDumpNotRecorded = "database dump skipped: its run could not be recorded"

	NoteDBImportKeptOld = "database imported; the previous data folder was kept"
	NoteDBImportErrors  = "database imported with errors"
)

// What an import appends to its note or reason, after "; " and before ": " and
// the names of the apps it stopped for the import.
const (
	ImportTailAppsDown    = "could not start these apps again"
	ImportTailAppsStopped = "these apps stay stopped until the data folder is sorted out"
)

// Why importing a dump back into a container did not finish. The detail says
// which folder holds the data that is still there.
const (
	ReasonDBImportPrepare  = "database import failed before it started, the old data is back in place"
	ReasonDBImportRollback = "database import failed and the old data could not be put back"
	ReasonDBImportFailed   = "database import failed: the import tool reported an error"
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
		SELECT `+runCols+`
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
		SELECT `+runCols+`
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

// LastSuccessfulZFSBackup is LastSuccessfulContainerBackup for ZFS items.
func (r *Repo) LastSuccessfulZFSBackup() (time.Time, error) {
	row := r.db.QueryRow(`
		SELECT finished_at
		FROM runs
		WHERE kind = 'backup' AND status = 'success' AND finished_at IS NOT NULL
		  AND target_id IN (SELECT id FROM zfs_datasets)`+sanePastStamp+`
		ORDER BY finished_at DESC
		LIMIT 1`, saneStampCutoff())
	return scanLastBackupTime(row, "LastSuccessfulZFSBackup")
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
		SELECT `+runCols+`
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

// RecentRunsOfKind returns up to limit finished runs of one kind for a target,
// newest first. A running row is left out: it has no outcome yet, and the
// callers ask for the last thing that happened. Ties on started_at keep their
// insertion order, so a dump and the backup that triggered it stay in sequence
// however coarse the clock is.
func (r *Repo) RecentRunsOfKind(targetID, kind string, limit int) ([]Run, error) {
	rows, err := r.db.Query(`
		SELECT `+runCols+`
		FROM runs
		WHERE target_id = ? AND kind = ? AND status <> 'running'
		ORDER BY started_at DESC, rowid DESC
		LIMIT ?`, targetID, kind, limit)
	if err != nil {
		return nil, fmt.Errorf("RecentRunsOfKind: %w", err)
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

// LastRunOfKind returns the most recent finished run of one kind for a target,
// or nil when there is none.
func (r *Repo) LastRunOfKind(targetID, kind string) (*Run, error) {
	runs, err := r.RecentRunsOfKind(targetID, kind, 1)
	if err != nil {
		return nil, err
	}
	if len(runs) == 0 {
		return nil, nil
	}
	return &runs[0], nil
}

// LastSuccessOfKind returns when a target last had a successful run of one
// kind, as unix seconds, or 0 when it never did.
func (r *Repo) LastSuccessOfKind(targetID, kind string) (int64, error) {
	row := r.db.QueryRow(`
		SELECT finished_at
		FROM runs
		WHERE target_id = ? AND kind = ? AND status = 'success' AND finished_at IS NOT NULL`+sanePastStamp+`
		ORDER BY finished_at DESC
		LIMIT 1`, targetID, kind, saneStampCutoff())
	var at sql.NullInt64
	err := row.Scan(&at)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("LastSuccessOfKind: %w", err)
	}
	return at.Int64, nil
}

// BackupSnapshotsOfRuns maps each of the given run ids to its snapshot, for the
// successful backup runs among them. It pairs a dump with the files backup it
// was taken for; ids of another kind and of failed runs are absent.
func (r *Repo) BackupSnapshotsOfRuns(ids []string) (map[string]string, error) {
	out := map[string]string{}
	for start := 0; start < len(ids); start += lastBackupAmongChunk {
		end := min(start+lastBackupAmongChunk, len(ids))
		if err := r.backupSnapshotsOfChunk(ids[start:end], out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// backupSnapshotsOfChunk collects one IN (...) clause worth of run ids into
// out, keeping each query under SQLite's parameter limit.
func (r *Repo) backupSnapshotsOfChunk(ids []string, out map[string]string) error {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	//nolint:gosec // G202: `placeholders` is a generated "?,?,…" list sized from
	// len(ids), never user text; every id travels as a bound parameter in args.
	rows, err := r.db.Query(`
		SELECT id, snapshot_id
		FROM runs
		WHERE kind = 'backup' AND status = 'success' AND snapshot_id IS NOT NULL AND snapshot_id <> ''
		  AND id IN (`+placeholders+`)`, args...)
	if err != nil {
		return fmt.Errorf("BackupSnapshotsOfRuns: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	for rows.Next() {
		var id, snap string
		if sErr := rows.Scan(&id, &snap); sErr != nil {
			return fmt.Errorf("BackupSnapshotsOfRuns: %w", sErr)
		}
		out[id] = snap
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("BackupSnapshotsOfRuns: %w", err)
	}
	return nil
}

// FailedDBDumpSnapshots returns the snapshots a failed dump of this target left
// behind: restic wrote them and they could not be removed again, so the dump
// list marks exactly those as damaged.
func (r *Repo) FailedDBDumpSnapshots(targetID string) (map[string]bool, error) {
	rows, err := r.db.Query(`
		SELECT snapshot_id
		FROM runs
		WHERE target_id = ? AND kind = 'dbdump' AND status = 'failed'
		  AND snapshot_id IS NOT NULL AND snapshot_id <> ''`, targetID)
	if err != nil {
		return nil, fmt.Errorf("FailedDBDumpSnapshots: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	out := map[string]bool{}
	for rows.Next() {
		var snap string
		if sErr := rows.Scan(&snap); sErr != nil {
			return nil, fmt.Errorf("FailedDBDumpSnapshots: %w", sErr)
		}
		out[snap] = true
	}
	return out, rows.Err()
}

// RunsSince returns all runs started at or after since (unix seconds), newest
// first, for the dashboard's backup-health heatmap.
func (r *Repo) RunsSince(since int64) ([]Run, error) {
	rows, err := r.db.Query(`
		SELECT `+runCols+`
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
	return r.RunCountsOfKind("backup")
}

// RunCountsOfKind is RunCounts for another kind of run, such as the database
// dumps, which get counters of their own.
func (r *Repo) RunCountsOfKind(kind string) (map[string]map[string]int, error) {
	rows, err := r.db.Query(`
		SELECT
		  CASE
		    WHEN target_id = ?                              THEN 'config'
		    WHEN target_id = ?                              THEN 'flash'
		    WHEN target_id IN (SELECT id FROM vms)          THEN 'vms'
		    WHEN target_id IN (SELECT id FROM file_sets)    THEN 'files'
		    WHEN target_id IN (SELECT id FROM zfs_datasets) THEN 'zfs'
		    WHEN target_id IN (SELECT id FROM targets)      THEN 'containers'
		    ELSE ''
		  END AS domain,
		  status,
		  count(*) AS n
		FROM runs
		WHERE kind = ? AND status IN ('success', 'failed')
		GROUP BY domain, status`, ConfigTargetID, FlashTargetID, kind)
	if err != nil {
		return nil, fmt.Errorf("RunCountsOfKind: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	out := map[string]map[string]int{}
	for rows.Next() {
		var domain, status string
		var n int
		if sErr := rows.Scan(&domain, &status, &n); sErr != nil {
			return nil, fmt.Errorf("RunCountsOfKind: %w", sErr)
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
		&run.StartedVia, &run.StartedViaKey,
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

// SeriesRun is the narrow run row anomaly detection reads. It is separate from
// Run because Run is the shape /api/runs serves, and because a detector needs
// the metric columns to tell a missing value from a measured zero.
type SeriesRun struct {
	ID, Status, SnapshotID, Error string
	// Outcome is a ZFS member's own result within a run, empty for every other
	// series.
	Outcome                                                 string
	StartedAt                                               int64
	FinishedAt                                              int64
	Bytes                                                   int64
	SourceBytes, SourceFiles, FilesNew, ResticMS, HasParent *int64
	SelectionFP                                             *string
}

// RunTargetKind says which series a run belongs to.
type RunTargetKind struct{ TargetID, Kind string }

// UnmeasuredRun is a finished run whose source metrics the backfill can still
// read out of its snapshot.
type UnmeasuredRun struct {
	ID, TargetID, Kind, SnapshotID string
	StartedAt                      int64
}

// BackfillMetrics are the figures one backfilled run gets. Bytes is set only
// for a VM run, whose data added is the sum over its file and zvol snapshots.
type BackfillMetrics struct {
	RunMetrics
	Bytes *int64
}

// eligibleRun is the condition for a run the detectors may learn from: it
// succeeded and left either a snapshot or a measurement behind. A success that
// records a decision instead of a backup, such as a container that was gone,
// carries neither and would otherwise read as a source that vanished.
const eligibleRun = `status = 'success'
	AND ((snapshot_id IS NOT NULL AND snapshot_id <> '') OR source_bytes IS NOT NULL)`

const seriesRunColumns = `id, status, snapshot_id, error, started_at, finished_at, bytes,
		source_bytes, source_files, files_new, restic_ms, has_parent, selection_fp`

// ItemSeries returns the newest limit runs of targetID and kind that reached an
// outcome on or before cutoff, newest first. Ties on started_at are broken by
// rowid of the runs table itself, so a dump and the backup that triggered it
// keep their order however coarse the clock is.
func (r *Repo) ItemSeries(targetID, kind string, cutoff int64, limit int) ([]SeriesRun, error) {
	rows, err := r.db.Query(`
		SELECT `+seriesRunColumns+`
		FROM runs
		WHERE target_id = ? AND kind = ? AND status IN ('success', 'failed') AND started_at <= ?
		ORDER BY started_at DESC, rowid DESC
		LIMIT ?`, targetID, kind, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("ItemSeries: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []SeriesRun
	for rows.Next() {
		run, err := scanSeriesRun(rows)
		if err != nil {
			return nil, fmt.Errorf("ItemSeries: %w", err)
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

// NewDataWindow returns the eligible runs of targetID and kind started within
// [from, cutoff], newest first and at most limit of them. The rows carry what
// the new-data detector reads over a month of runs: the data added, the
// selection fingerprint, and the source figures that tell a fresh upload and an
// unmeasured row apart from a measured one.
func (r *Repo) NewDataWindow(targetID, kind string, from, cutoff int64, limit int) ([]SeriesRun, error) {
	rows, err := r.db.Query(`
		SELECT id, started_at, bytes, source_bytes, source_files, files_new, has_parent, selection_fp
		FROM runs
		WHERE target_id = ? AND kind = ? AND started_at BETWEEN ? AND ?
			AND `+eligibleRun+`
		ORDER BY started_at DESC, rowid DESC
		LIMIT ?`, targetID, kind, from, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("NewDataWindow: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []SeriesRun
	for rows.Next() {
		var run SeriesRun
		var bytes, sourceBytes, sourceFiles, filesNew, hasParent sql.NullInt64
		var fp sql.NullString
		err := rows.Scan(&run.ID, &run.StartedAt, &bytes,
			&sourceBytes, &sourceFiles, &filesNew, &hasParent, &fp)
		if err != nil {
			return nil, fmt.Errorf("NewDataWindow: %w", err)
		}
		run.Bytes = bytes.Int64
		run.SourceBytes = nullableInt(sourceBytes)
		run.SourceFiles = nullableInt(sourceFiles)
		run.FilesNew = nullableInt(filesNew)
		run.HasParent = nullableInt(hasParent)
		if fp.Valid {
			run.SelectionFP = &fp.String
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

// FirstEligibleRunID returns the id of the oldest eligible run of targetID and
// kind, or an empty string when the series has none. It is how a detector tells
// a quiet item from one whose history only starts here.
func (r *Repo) FirstEligibleRunID(targetID, kind string) (string, error) {
	var id string
	err := r.db.QueryRow(`
		SELECT id FROM runs
		WHERE target_id = ? AND kind = ? AND `+eligibleRun+`
		ORDER BY started_at ASC, rowid ASC
		LIMIT 1`, targetID, kind).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("FirstEligibleRunID: %w", err)
	}
	return id, nil
}

// RunTargets resolves run ids to the series they belong to. Ids with no row are
// left out of the result.
func (r *Repo) RunTargets(ids []string) (map[string]RunTargetKind, error) {
	out := make(map[string]RunTargetKind, len(ids))
	for start := 0; start < len(ids); start += lastBackupAmongChunk {
		end := min(start+lastBackupAmongChunk, len(ids))
		if err := r.runTargetsOfChunk(ids[start:end], out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// runTargetsOfChunk resolves one IN (...) clause worth of run ids into out,
// keeping each query under SQLite's parameter limit.
func (r *Repo) runTargetsOfChunk(ids []string, out map[string]RunTargetKind) error {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	//nolint:gosec // G202: `placeholders` is a generated "?,?,…" list sized from
	// len(ids), never user text; every id travels as a bound parameter in args.
	rows, err := r.db.Query(
		`SELECT id, target_id, kind FROM runs WHERE id IN (`+placeholders+`)`, args...)
	if err != nil {
		return fmt.Errorf("RunTargets: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	for rows.Next() {
		var id string
		var tk RunTargetKind
		if sErr := rows.Scan(&id, &tk.TargetID, &tk.Kind); sErr != nil {
			return fmt.Errorf("RunTargets: %w", sErr)
		}
		out[id] = tk
	}
	return rows.Err()
}

// RestorePoint is the backup one run left behind: what to restore, and when it
// was taken.
type RestorePoint struct {
	RunID, SnapshotID string
	At                int64
}

// RestorePoints resolves run ids to the backups they left behind. A run that
// stored no snapshot, and an id no run has, is absent from the result.
func (r *Repo) RestorePoints(ids []string) (map[string]RestorePoint, error) {
	out := make(map[string]RestorePoint, len(ids))
	for start := 0; start < len(ids); start += lastBackupAmongChunk {
		end := min(start+lastBackupAmongChunk, len(ids))
		if err := r.restorePointsOfChunk(ids[start:end], out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *Repo) restorePointsOfChunk(ids []string, out map[string]RestorePoint) error {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	//nolint:gosec // G202: `placeholders` is a generated "?,?,…" list sized from
	// len(ids), never user text; every id travels as a bound parameter in args.
	rows, err := r.db.Query(`
		SELECT id, snapshot_id, COALESCE(finished_at, started_at)
		FROM runs
		WHERE id IN (`+placeholders+`) AND snapshot_id IS NOT NULL AND snapshot_id <> ''`, args...)
	if err != nil {
		return fmt.Errorf("RestorePoints: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	for rows.Next() {
		var p RestorePoint
		if sErr := rows.Scan(&p.RunID, &p.SnapshotID, &p.At); sErr != nil {
			return fmt.Errorf("RestorePoints: %w", sErr)
		}
		out[p.RunID] = p
	}
	return rows.Err()
}

// UnmeasuredSnapshotRuns lists the successful backup and dump runs that left a
// snapshot but no measurement, oldest first. Rows are drained before returning,
// because the backfill writes on the same connection.
func (r *Repo) UnmeasuredSnapshotRuns() ([]UnmeasuredRun, error) {
	rows, err := r.db.Query(`
		SELECT id, target_id, kind, snapshot_id, started_at
		FROM runs
		WHERE status = 'success' AND kind IN ('backup', 'dbdump')
			AND snapshot_id IS NOT NULL AND snapshot_id <> '' AND source_bytes IS NULL
		ORDER BY started_at ASC, rowid ASC`)
	if err != nil {
		return nil, fmt.Errorf("UnmeasuredSnapshotRuns: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []UnmeasuredRun
	for rows.Next() {
		var run UnmeasuredRun
		if err := rows.Scan(&run.ID, &run.TargetID, &run.Kind, &run.SnapshotID, &run.StartedAt); err != nil {
			return nil, fmt.Errorf("UnmeasuredSnapshotRuns: %w", err)
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

// SetRunMetrics fills in the metrics of runs that have none and returns how many
// rows it wrote. The guard on source_bytes keeps a backfill from a snapshot,
// which rounds through restic's summary, from replacing what the run itself
// measured.
func (r *Repo) SetRunMetrics(m map[string]BackfillMetrics) (int, error) {
	if len(m) == 0 {
		return 0, nil
	}
	tx, err := r.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("SetRunMetrics begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	set := 0
	for id, bm := range m {
		var hasParent any
		if bm.HasParent != nil {
			hasParent = boolToInt(*bm.HasParent)
		}
		res, err := tx.Exec(`
			UPDATE runs
			SET source_bytes = ?, source_files = ?, files_new = ?, restic_ms = ?, has_parent = ?,
			    bytes = COALESCE(?, bytes)
			WHERE id = ? AND source_bytes IS NULL`,
			bm.SourceBytes, bm.SourceFiles, bm.FilesNew, bm.ResticMS, hasParent, bm.Bytes, id,
		)
		if err != nil {
			return 0, fmt.Errorf("SetRunMetrics %s: %w", id, err)
		}
		n, _ := res.RowsAffected()
		set += int(n)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("SetRunMetrics commit: %w", err)
	}
	return set, nil
}

func scanSeriesRun(s scanner) (SeriesRun, error) {
	var run SeriesRun
	var snapID, errCol, fp sql.NullString
	var finishedAt, bytes sql.NullInt64
	var sourceBytes, sourceFiles, filesNew, resticMS, hasParent sql.NullInt64
	err := s.Scan(
		&run.ID, &run.Status, &snapID, &errCol, &run.StartedAt, &finishedAt, &bytes,
		&sourceBytes, &sourceFiles, &filesNew, &resticMS, &hasParent, &fp,
	)
	if err != nil {
		return SeriesRun{}, err
	}
	run.SnapshotID = snapID.String
	run.Error = errCol.String
	run.FinishedAt = finishedAt.Int64
	run.Bytes = bytes.Int64
	run.SourceBytes = nullableInt(sourceBytes)
	run.SourceFiles = nullableInt(sourceFiles)
	run.FilesNew = nullableInt(filesNew)
	run.ResticMS = nullableInt(resticMS)
	run.HasParent = nullableInt(hasParent)
	if fp.Valid {
		run.SelectionFP = &fp.String
	}
	return run, nil
}

func nullableInt(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}

// ErrRunNotFound reports a run id that no row carries.
var ErrRunNotFound = errors.New("run not found")

// GetRun returns one run by its id.
func (r *Repo) GetRun(id string) (Run, error) {
	row := r.db.QueryRow(`SELECT `+runCols+` FROM runs WHERE id = ?`, id)
	run, err := scanRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Run{}, ErrRunNotFound
	}
	if err != nil {
		return Run{}, fmt.Errorf("GetRun: %w", err)
	}
	return run, nil
}

// RunFilter narrows ListRunsFiltered. An empty list means "any".
type RunFilter struct {
	TargetIDs []string
	Kinds     []string
	Statuses  []string
	// Since keeps runs started at or after this unix second when it is set.
	Since int64
	Limit int
}

// How far a caller may stretch one filtered query: enough ids for every item of
// an install and enough rows for the longest answer a tool returns, both well
// inside SQLite's parameter limit.
const (
	runFilterMaxTargets = 2000
	runFilterMaxRows    = 500
)

// ListRunsFiltered returns the runs matching f, newest first. Ties on
// started_at keep their insertion order, so a dump and the backup that
// triggered it stay in sequence however coarse the clock is.
func (r *Repo) ListRunsFiltered(f RunFilter) ([]Run, error) {
	var where []string
	var args []any
	in := func(column string, values []string) {
		if len(values) == 0 {
			return
		}
		where = append(where, column+" IN ("+placeholderList(len(values))+")")
		for _, v := range values {
			args = append(args, v)
		}
	}
	in("target_id", f.TargetIDs[:min(len(f.TargetIDs), runFilterMaxTargets)])
	in("kind", f.Kinds)
	in("status", f.Statuses)
	if f.Since > 0 {
		where = append(where, "started_at >= ?")
		args = append(args, f.Since)
	}
	limit := f.Limit
	if limit <= 0 || limit > runFilterMaxRows {
		limit = runFilterMaxRows
	}
	args = append(args, limit)

	q := `SELECT ` + runCols + ` FROM runs`
	if len(where) > 0 {
		//nolint:gosec // G202: the conditions are built above from column names and
		// generated "?,…" lists; every value travels as a bound parameter in args.
		q += ` WHERE ` + strings.Join(where, " AND ")
	}
	q += ` ORDER BY started_at DESC, rowid DESC LIMIT ?`
	rows, err := r.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("ListRunsFiltered: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []Run
	for rows.Next() {
		run, sErr := scanRun(rows)
		if sErr != nil {
			return nil, fmt.Errorf("ListRunsFiltered: %w", sErr)
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

// placeholderList returns the "?,?,…" list for n bound parameters.
func placeholderList(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// LatestMCPStartAt returns the newest started_at among the finished MCP-started
// runs of targetIDs at or after since, or 0 when there is none. It answers the
// cooldown that keeps an assistant from starting the same target again and
// again. A run still in flight is left out: the caller's own guard answers that
// one as busy, which says more than a waiting time.
func (r *Repo) LatestMCPStartAt(targetIDs []string, since int64) (int64, error) {
	var newest int64
	for start := 0; start < len(targetIDs); start += lastBackupAmongChunk {
		end := min(start+lastBackupAmongChunk, len(targetIDs))
		chunk := targetIDs[start:end]

		args := make([]any, 0, len(chunk)+1)
		args = append(args, since)
		for _, id := range chunk {
			args = append(args, id)
		}
		//nolint:gosec // G202: placeholderList generates "?,…" from len(chunk); every id is a bound parameter.
		row := r.db.QueryRow(`
			SELECT max(started_at)
			FROM runs
			WHERE started_via = 'mcp' AND status <> 'running' AND started_at >= ?
			  AND target_id IN (`+placeholderList(len(chunk))+`)`, args...)
		var at sql.NullInt64
		if err := row.Scan(&at); err != nil {
			return 0, fmt.Errorf("LatestMCPStartAt: %w", err)
		}
		if at.Valid && at.Int64 > newest {
			newest = at.Int64
		}
	}
	return newest, nil
}

// MCPBackupsSince counts the backups an MCP key started for targetID at or
// after since and returns when the oldest of them began. It is the daily budget
// a single item has, and oldest says when the next slot frees up. A failed
// attempt counts: the budget is about the downtime a start costs, and a backup
// that failed stopped the container all the same. Only a cancelled run is left
// out, because the caller gave its slot back.
func (r *Repo) MCPBackupsSince(targetID string, since int64) (count int, oldest int64, err error) {
	var first sql.NullInt64
	err = r.db.QueryRow(`
		SELECT count(*), min(started_at)
		FROM runs
		WHERE target_id = ? AND kind = 'backup' AND started_via = 'mcp'
		  AND started_at >= ? AND status <> 'cancelled'`, targetID, since).Scan(&count, &first)
	if err != nil {
		return 0, 0, fmt.Errorf("MCPBackupsSince: %w", err)
	}
	return count, first.Int64, nil
}

// NewestBackupOrigins looks at the newest n successful runs of kind on targetID
// and reports how many there are and how many an MCP key started. Under a
// count-only retention policy that is what says whether one more MCP backup
// would push the last operator-made one out of the repository.
func (r *Repo) NewestBackupOrigins(targetID, kind string, n int) (total, viaMCP int, err error) {
	if n <= 0 {
		return 0, 0, nil
	}
	rows, qErr := r.db.Query(`
		SELECT started_via
		FROM runs
		WHERE target_id = ? AND kind = ? AND status = 'success'
		ORDER BY started_at DESC, rowid DESC
		LIMIT ?`, targetID, kind, n)
	if qErr != nil {
		return 0, 0, fmt.Errorf("NewestBackupOrigins: %w", qErr)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	for rows.Next() {
		var via string
		if sErr := rows.Scan(&via); sErr != nil {
			return 0, 0, fmt.Errorf("NewestBackupOrigins: %w", sErr)
		}
		total++
		if via == "mcp" {
			viaMCP++
		}
	}
	if rErr := rows.Err(); rErr != nil {
		return 0, 0, fmt.Errorf("NewestBackupOrigins: %w", rErr)
	}
	return total, viaMCP, nil
}

// BackupStamp is what a list of items says about a target's backup history
// without a query per item.
type BackupStamp struct {
	LastSuccessAt       int64
	LastDurationSeconds int64
	LastRunAt           int64
	LastRunStatus       string
}

// LatestBackupsByTarget returns every target's newest backup stamps. Both
// queries skip future stamps, which would otherwise win the ordering and hide
// every correctly stamped run behind them.
//
// The bare columns beside max(finished_at) come from the row that matched it,
// which is how each group arrives with its own started_at and status.
func (r *Repo) LatestBackupsByTarget() (map[string]BackupStamp, error) {
	out := map[string]BackupStamp{}
	cutoff := saneStampCutoff()

	success, err := r.db.Query(`
		SELECT target_id, max(finished_at), started_at
		FROM runs
		WHERE kind = 'backup' AND status = 'success' AND finished_at IS NOT NULL`+sanePastStamp+`
		GROUP BY target_id`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("LatestBackupsByTarget: %w", err)
	}
	defer success.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	for success.Next() {
		var target string
		var finishedAt, startedAt int64
		if sErr := success.Scan(&target, &finishedAt, &startedAt); sErr != nil {
			return nil, fmt.Errorf("LatestBackupsByTarget: %w", sErr)
		}
		out[target] = BackupStamp{LastSuccessAt: finishedAt, LastDurationSeconds: finishedAt - startedAt}
	}
	if sErr := success.Err(); sErr != nil {
		return nil, fmt.Errorf("LatestBackupsByTarget: %w", sErr)
	}

	last, err := r.db.Query(`
		SELECT target_id, max(finished_at), started_at, status
		FROM runs
		WHERE kind = 'backup' AND finished_at IS NOT NULL`+sanePastStamp+`
		GROUP BY target_id`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("LatestBackupsByTarget: %w", err)
	}
	defer last.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	for last.Next() {
		var target, status string
		var finishedAt, startedAt int64
		if sErr := last.Scan(&target, &finishedAt, &startedAt, &status); sErr != nil {
			return nil, fmt.Errorf("LatestBackupsByTarget: %w", sErr)
		}
		stamp := out[target]
		stamp.LastRunAt = startedAt
		stamp.LastRunStatus = status
		out[target] = stamp
	}
	return out, last.Err()
}

// LastRunsOfKind returns every target's newest finished run of one kind, so a
// list of items costs one query rather than one per item. A running row is left
// out and ties fall to the row written last, as LastRunOfKind has it.
func (r *Repo) LastRunsOfKind(kind string) (map[string]Run, error) {
	rows, err := r.db.Query(`
		SELECT `+runCols+`
		FROM (
			SELECT `+runCols+`,
			       row_number() OVER (PARTITION BY target_id ORDER BY started_at DESC, rowid DESC) AS rank
			FROM runs
			WHERE kind = ? AND status <> 'running'
		)
		WHERE rank = 1`, kind)
	if err != nil {
		return nil, fmt.Errorf("LastRunsOfKind: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	out := map[string]Run{}
	for rows.Next() {
		run, sErr := scanRun(rows)
		if sErr != nil {
			return nil, fmt.Errorf("LastRunsOfKind: %w", sErr)
		}
		out[run.TargetID] = run
	}
	return out, rows.Err()
}
