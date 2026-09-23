package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Scheduled jobs that record their own last run for the scheduler's everyN
// due-gate. The drill and tamper result tables cannot serve, because the manual
// single-domain buttons write them too, and the digest leaves nothing behind.
const (
	// ScheduleJobDrills is the restore drill pass over every drill task.
	ScheduleJobDrills = "drills"
	// ScheduleJobTamper is the tamper-test sweep over every immutable off-site
	// repo.
	ScheduleJobTamper = "tamper"
	// ScheduleJobDigest is the app-wide digest notification.
	ScheduleJobDigest = "digest"
)

// RecordScheduleJobRun stores at as job's last run. The scheduler decides what
// counts as done: the expensive drill and tamper passes record even when some
// tasks failed, so a broken repo is not re-drilled every night, while the
// digest records only on success, so a failed send is retried the next day.
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

// LastScheduleJobRun returns when job last ran, or the zero time when it never
// has, which the everyN due-gate treats as due. A query failure is returned as
// an error, never as the zero time, because the gate skips on an error and
// would run on zero. A future stamp reads as zero, as in SanitizeRecordedTime.
func (r *Repo) LastScheduleJobRun(job string) (time.Time, error) {
	row := r.db.QueryRow(`SELECT at FROM schedule_job_runs WHERE job = ?`, job)
	var at sql.NullInt64
	if err := row.Scan(&at); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("LastScheduleJobRun(%s): %w", job, err)
	}
	if !at.Valid || at.Int64 <= 0 {
		return time.Time{}, nil
	}
	return SanitizeRecordedTime(time.Unix(at.Int64, 0), time.Now()), nil
}
