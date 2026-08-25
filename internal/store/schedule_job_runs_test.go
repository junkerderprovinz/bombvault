package store_test

// #166 — the durable last-run record behind an "every N days" cadence on the
// drills / tamper-test / digest schedules. These exercise the real SQLite path
// (migration v89 included), not a fake, because the safety property the
// scheduler leans on lives here: a MISSING row must be reported as a definite
// "never ran" (zero time, nil error) while a real failure must stay an error.

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

// TestScheduleJobRunNeverRanIsZeroAndNil is the case a fresh install and a
// just-upgraded install both hit: no row for the job. It must come back as a
// zero time with a NIL error — the scheduler reads that as "has never run" and
// lets the first fire after enabling proceed, so flattening a real error into it
// would silently authorise a run.
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

// TestScheduleJobRunRoundTrip records and reads back each job's stamp, and pins
// that the three are stored INDEPENDENTLY — a drill pass must not satisfy the
// tamper or digest gate.
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

// TestScheduleJobRunOverwrites pins the upsert: a job keeps ONE row, and each
// recorded pass replaces the previous timestamp rather than appending. A second
// row would make the table grow without bound and make "the last run" ambiguous.
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

// TestScheduleJobRunQueryFailureIsAnError is the safety property stated the
// other way round: when the table is genuinely unreadable the read must ERROR,
// never quietly return the zero time that means "never ran". The scheduler skips
// on the error and would RUN on the zero time, so the two must not be confused.
// Driven by dropping the table out from under the query.
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

// TestScheduleJobRunsTableCreatedByMigration pins that v89 actually ran, so an
// UPGRADE (not just a fresh install) has the table the gate depends on.
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
	// And a freshly migrated database is EMPTY — every job reads as never-ran,
	// which is exactly the upgrade case: the table exists, nothing has run yet.
	var rows int
	if err := db.QueryRow(`SELECT count(*) FROM schedule_job_runs`).Scan(&rows); err != nil {
		t.Fatalf("count: %v", err)
	}
	if rows != 0 {
		t.Fatalf("a fresh schedule_job_runs must be empty, got %d row(s)", rows)
	}
}
