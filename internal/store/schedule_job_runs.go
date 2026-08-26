package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// The scheduled jobs that record their OWN last-run fact (#166), so the
// scheduler's everyN due-gate has something to measure against. The five backup
// domains do not appear here: their gate reads the last successful backup out of
// `runs` (LastSuccessfulContainerBackup and friends), which is a better answer
// for them — a backup that ran is a backup that landed a snapshot.
//
// These three have no such natural signal. A drill pass, a tamper sweep and a
// digest send leave per-RESULT rows (restore_drills, tamper_tests) or nothing at
// all, and those result tables are also written by the MANUAL single-domain
// buttons, which must not satisfy a schedule that covers every domain.
const (
	// ScheduleJobDrills is the restore-verification drill pass (one scheduled
	// fire runs every (domain, source, kind) task in drillTasks).
	ScheduleJobDrills = "drills"
	// ScheduleJobTamper is the off-site tamper-test sweep (one scheduled fire
	// probes every domain whose off-site repo is flagged immutable).
	ScheduleJobTamper = "tamper"
	// ScheduleJobDigest is the digest notification (one scheduled fire sends one
	// app-wide summary message).
	ScheduleJobDigest = "digest"
)

// RecordScheduleJobRun stamps `job`'s last-run time, replacing any previous
// value. Call it only once the pass has actually done its work — the scheduler
// decides per job what "done" means (see the domain specs in
// internal/schedule/schedule.go), because the right answer differs: an expensive
// multi-task pass records even when some tasks failed, so a permanently broken
// repo is not re-drilled every single night, while the cheap idempotent digest
// records only on success, so a transient notify failure is retried tomorrow
// instead of being lost for the whole interval.
func (r *Repo) RecordScheduleJobRun(job string, at time.Time) error {
	_, err := r.db.Exec(`
		INSERT INTO schedule_job_runs (job, at) VALUES (?, ?)
		ON CONFLICT(job) DO UPDATE SET at = excluded.at`,
		job, at.Unix(),
	)
	if err != nil {
		return fmt.Errorf("RecordScheduleJobRun(%s): %w", job, err)
	}
	return nil
}

// LastScheduleJobRun returns when `job` last ran, or a ZERO time when it never
// has. The zero time is a definite answer, not an unknown: it means the row is
// absent (a fresh install, or an upgrade that has just created this table), and
// the scheduler's everyN due-gate deliberately lets the first trigger after
// enabling proceed on it — exactly as it already does for a domain that has
// never been backed up.
//
// A real failure is returned as an error and NEVER flattened into that zero
// time. That distinction is the safety property: the due-gate skips the job when
// this returns an error, so a broken database can never be mistaken for "never
// ran" and turned into a run.
//
// A stamp in the FUTURE reads as the zero time too, for the reason
// SanitizeRecordedTime gives — the same rule the last-successful-backup queries
// in runs.go apply, so all eight schedules answer a wrong-clock stamp alike.
func (r *Repo) LastScheduleJobRun(job string) (time.Time, error) {
	row := r.db.QueryRow(`SELECT at FROM schedule_job_runs WHERE job = ?`, job)
	var at sql.NullInt64
	if err := row.Scan(&at); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, nil // never ran — a definite answer
		}
		return time.Time{}, fmt.Errorf("LastScheduleJobRun(%s): %w", job, err)
	}
	if !at.Valid || at.Int64 <= 0 {
		return time.Time{}, nil
	}
	return SanitizeRecordedTime(time.Unix(at.Int64, 0), time.Now()), nil
}
