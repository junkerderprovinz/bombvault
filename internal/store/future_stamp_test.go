package store_test

// A box that boots with a wrong clock (dead CMOS battery, or before NTP steps
// it) stamps its runs years ahead. Every currency read takes the newest
// finished_at, so such a row would keep winning after the clock is fixed and
// the elapsed time would stay negative: the everyN gate would skip every fire
// and the watchdog would never alert.

import (
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestLastSuccessfulBackupIgnoresAFutureStamp(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	tg, err := r.UpsertTarget(store.Target{ContainerName: "sonarr", AppdataPaths: []string{"/data"}})
	if err != nil {
		t.Fatalf("UpsertTarget: %v", err)
	}

	// A run stamped by a clock nine years ahead, as the row looks after NTP
	// steps the clock back.
	poisoned, err := r.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.FinishRun(poisoned, "success", "deadbeef", 1, ""); err != nil {
		t.Fatal(err)
	}
	future := time.Now().AddDate(9, 0, 0)
	if _, err := db.Exec(`UPDATE runs SET finished_at = ? WHERE id = ?`, future.Unix(), poisoned); err != nil {
		t.Fatal(err)
	}

	last, err := r.LastSuccessfulContainerBackup()
	if err != nil {
		t.Fatalf("LastSuccessfulContainerBackup: %v", err)
	}
	if last.After(time.Now()) {
		t.Fatalf("a finished_at from the future must not be reported as a last-success measurement, got %v", last)
	}
	if !last.IsZero() {
		t.Fatalf("with only a poisoned row the answer is \"never\", got %v", last)
	}

	// A correctly stamped run must win over the future row, or the schedule
	// never recovers.
	good, err := r.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.FinishRun(good, "success", "cafebabe", 2, ""); err != nil {
		t.Fatal(err)
	}
	last, err = r.LastSuccessfulContainerBackup()
	if err != nil {
		t.Fatal(err)
	}
	if last.IsZero() || last.After(time.Now().Add(time.Minute)) {
		t.Fatalf("after a correctly-stamped run the last success must be that run, got %v", last)
	}
}

func TestLastScheduleJobRunIgnoresAFutureStamp(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := store.New(db)

	future := time.Now().AddDate(9, 0, 0)
	if err := r.RecordScheduleJobRun(store.ScheduleJobDrills, future); err != nil {
		t.Fatalf("RecordScheduleJobRun: %v", err)
	}

	last, err := r.LastScheduleJobRun(store.ScheduleJobDrills)
	if err != nil {
		t.Fatalf("LastScheduleJobRun: %v", err)
	}
	if !last.IsZero() {
		t.Fatalf("a job-run stamp from the future must read as \"never ran\" so the pass can run once and re-stamp it, got %v", last)
	}

	// A sane stamp is still reported unchanged.
	sane := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	if err := r.RecordScheduleJobRun(store.ScheduleJobDrills, sane); err != nil {
		t.Fatal(err)
	}
	last, err = r.LastScheduleJobRun(store.ScheduleJobDrills)
	if err != nil {
		t.Fatal(err)
	}
	if !last.Equal(sane) {
		t.Fatalf("a normal stamp must round-trip: got %v, want %v", last, sane)
	}
}

// TestSanitizeRecordedTimeTolerance keeps ordinary skew (a stamp taken moments
// ago, a second-resolution column rounding up) and drops a wrong-clock stamp.
func TestSanitizeRecordedTimeTolerance(t *testing.T) {
	now := time.Date(2026, time.March, 11, 9, 15, 0, 0, time.UTC)
	cases := []struct {
		name string
		at   time.Time
		kept bool
	}{
		{"an hour ago", now.Add(-time.Hour), true},
		{"exactly now", now, true},
		{"a minute ahead (clock skew)", now.Add(time.Minute), true},
		{"an hour ahead", now.Add(time.Hour), false},
		{"nine years ahead", now.AddDate(9, 0, 0), false},
		{"zero stays zero", time.Time{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := store.SanitizeRecordedTime(c.at, now)
			if c.kept && !got.Equal(c.at) {
				t.Fatalf("want %v kept, got %v", c.at, got)
			}
			if !c.kept && !got.IsZero() {
				t.Fatalf("want %v refused, got %v", c.at, got)
			}
		})
	}
}
