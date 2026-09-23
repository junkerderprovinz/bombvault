package store_test

// These run against real SQLite because the property the scheduler relies on
// lives here: a missing row reads as "never ran" (zero time, nil error), while
// a real failure stays an error.

import (
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func newJobRunRepo(t *testing.T) *store.Repo {
	t.Helper()
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return store.New(db)
}

// TestScheduleJobRunNeverRanIsZeroAndNil covers a job without a row, as after a
// fresh install or an upgrade: the result is the zero time and a nil error.
func TestScheduleJobRunNeverRanIsZeroAndNil(t *testing.T) {
	r := newJobRunRepo(t)
	for _, job := range []string{store.ScheduleJobDrills, store.ScheduleJobTamper, store.ScheduleJobDigest} {
		at, err := r.LastScheduleJobRun(job)
		if err != nil {
			t.Fatalf("%s: fresh install must not error, got %v", job, err)
		}
		if !at.IsZero() {
			t.Fatalf("%s: fresh install must read as never-ran, got %v", job, at)
		}
	}
}

// TestScheduleJobRunRoundTrip expects each job to keep its own stamp, so a
// drill pass does not satisfy the tamper or digest gate.
func TestScheduleJobRunRoundTrip(t *testing.T) {
	r := newJobRunRepo(t)
	want := time.Now().Add(-36 * time.Hour).Truncate(time.Second)

	if err := r.RecordScheduleJobRun(store.ScheduleJobDrills, want); err != nil {
		t.Fatalf("record: %v", err)
	}
	got, err := r.LastScheduleJobRun(store.ScheduleJobDrills)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !got.Equal(want) {
		t.Fatalf("drills last-run = %v, want %v", got, want)
	}
	for _, other := range []string{store.ScheduleJobTamper, store.ScheduleJobDigest} {
		at, oErr := r.LastScheduleJobRun(other)
		if oErr != nil {
			t.Fatalf("%s: %v", other, oErr)
		}
		if !at.IsZero() {
			t.Fatalf("%s must stay never-ran after a drills run, got %v", other, at)
		}
	}
}

// TestScheduleJobRunOverwrites expects one row per job, with each recorded run
// replacing the previous stamp.
func TestScheduleJobRunOverwrites(t *testing.T) {
	r := newJobRunRepo(t)
	first := time.Now().Add(-10 * 24 * time.Hour).Truncate(time.Second)
	second := time.Now().Truncate(time.Second)

	if err := r.RecordScheduleJobRun(store.ScheduleJobTamper, first); err != nil {
		t.Fatalf("record first: %v", err)
	}
	if err := r.RecordScheduleJobRun(store.ScheduleJobTamper, second); err != nil {
		t.Fatalf("record second: %v", err)
	}
	got, err := r.LastScheduleJobRun(store.ScheduleJobTamper)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !got.Equal(second) {
		t.Fatalf("last-run = %v, want the newer %v", got, second)
	}
}

// TestScheduleJobRunQueryFailureIsAnError drops the table and expects an error,
// not the zero time: the scheduler skips on an error but runs on zero.
func TestScheduleJobRunQueryFailureIsAnError(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	r := store.New(db)
	if _, err := db.Exec(`DROP TABLE schedule_job_runs`); err != nil {
		t.Fatalf("drop: %v", err)
	}
	at, err := r.LastScheduleJobRun(store.ScheduleJobDigest)
	if err == nil {
		t.Fatal("a broken table must return an error, not a zero time that reads as never-ran")
	}
	if !at.IsZero() {
		t.Fatalf("the error path must return a zero time too, got %v", at)
	}
	if !strings.Contains(err.Error(), store.ScheduleJobDigest) {
		t.Fatalf("the error should name the job, got %v", err)
	}
}

// TestScheduleJobRunsTableCreatedByMigration expects migration 89 to create the
// table and record itself.
func TestScheduleJobRunsTableCreatedByMigration(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	var n int
	row := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='schedule_job_runs'`)
	if err := row.Scan(&n); err != nil || n != 1 {
		t.Fatalf("schedule_job_runs table missing after migrate (n=%d, err=%v)", n, err)
	}
	var applied int
	row = db.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version = 89 AND name = 'schedule_job_runs'`)
	if err := row.Scan(&applied); err != nil || applied != 1 {
		t.Fatalf("migration v89 not recorded (applied=%d, err=%v)", applied, err)
	}
	// A fresh table is empty, so every job reads as never ran.
	var rows int
	if err := db.QueryRow(`SELECT count(*) FROM schedule_job_runs`).Scan(&rows); err != nil {
		t.Fatalf("count: %v", err)
	}
	if rows != 0 {
		t.Fatalf("a fresh schedule_job_runs must be empty, got %d row(s)", rows)
	}
}
