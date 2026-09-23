package api

// The received-repo integrity check has to run on its configured cadence. The
// receiver watch is a fixed daily sweep, and last_check_at is stamped when the
// previous check finished, minutes after its sweep fired. Measured in elapsed
// seconds, the next sweep then falls just short of a day and the check slips
// to the sweep after.

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// dueGateService is a Service over a real in-memory store with notifications
// muted, since these tests are about whether the check runs.
func dueGateService(t *testing.T, appKey string) (*Service, *store.Repo) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	// The real engine, pointed at a path with no repo. The check fails without
	// needing a restic binary, and a failure is still a recorded verdict.
	return &Service{cfg: config.Config{AppKey: appKey}, store: st, engine: restic.Restic{Bin: "restic"}}, st
}

// seedDueGateRepo registers one enabled received repo on cadence whose last
// check finished at lastCheck with no verdict recorded yet, so a check that
// runs is visible as last_check_ok turning non-NULL. The dead-man's switch is
// off, leaving the integrity gate as the only thing that can act.
func seedDueGateRepo(t *testing.T, st *store.Repo, appKey, cadence string, lastCheck time.Time) store.ReceivedRepo {
	t.Helper()
	rr := makeReceivedRepo(t, appKey, strings.Repeat("cd", 32), t.TempDir()+"/no-such-repo", 0)
	rr.Name = "Off-site A"
	rr.DeadManHours = 0
	rr.CheckCadence = cadence
	created, err := st.CreateReceivedRepo(rr)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateReceivedRepoCheckResult(created.ID, lastCheck.Unix(), sql.NullBool{}, "", false); err != nil {
		t.Fatal(err)
	}
	return created
}

// checked reports whether a verdict has been recorded for the repo since the
// seed (last_check_ok is no longer NULL).
func checked(t *testing.T, st *store.Repo, id string) bool {
	t.Helper()
	got, ok, err := st.GetReceivedRepo(id)
	if err != nil || !ok {
		t.Fatalf("GetReceivedRepo(%s): ok=%v err=%v", id, ok, err)
	}
	return got.LastCheckOK.Valid
}

// A daily repo whose previous check finished ten minutes after yesterday's
// sweep has to be checked on today's.
func TestReceiverDailyCheckRunsTheNextDayNotTheDayAfter(t *testing.T) {
	appKey := strings.Repeat("ab", 32)
	svc, st := dueGateService(t, appKey)

	sweepDay0 := time.Date(2026, time.March, 10, 9, 15, 0, 0, time.Local)
	sweepDay1 := time.Date(2026, time.March, 11, 9, 15, 0, 0, time.Local)
	finishedDay0 := sweepDay0.Add(10 * time.Minute) // the check's own runtime

	if elapsed := sweepDay1.Sub(finishedDay0); elapsed >= 24*time.Hour {
		t.Fatalf("test premise broken: the gap is %v, not short of a day", elapsed)
	}
	rr := seedDueGateRepo(t, st, appKey, "daily 04:00", finishedDay0)

	if err := svc.runReceiverChecksAt(context.Background(), sweepDay1.Unix()); err != nil {
		t.Fatalf("runReceiverChecksAt: %v", err)
	}
	if !checked(t, st, rr.ID) {
		t.Fatal("a repo on a daily check cadence was not checked on the next day's sweep; " +
			"it is being checked every 48h, at half the configured frequency, and the skipped day logs nothing")
	}
}

// At the weekly cadence the same slip would mean a check every 14 days instead
// of every 7.
func TestReceiverWeeklyCheckRunsOnTheSeventhDay(t *testing.T) {
	appKey := strings.Repeat("ab", 32)
	svc, st := dueGateService(t, appKey)

	finished := time.Date(2026, time.March, 10, 9, 15, 0, 0, time.Local).Add(37 * time.Minute)
	sweepDay7 := time.Date(2026, time.March, 17, 9, 15, 0, 0, time.Local)
	rr := seedDueGateRepo(t, st, appKey, "weekly Tue 05:00", finished)

	if err := svc.runReceiverChecksAt(context.Background(), sweepDay7.Unix()); err != nil {
		t.Fatalf("runReceiverChecksAt: %v", err)
	}
	if !checked(t, st, rr.ID) {
		t.Fatal("a repo on a WEEKLY check cadence was not checked on the seventh day's sweep")
	}
}

// Inside the cadence the gate stays closed, or every daily sweep would run a
// full check on every received repo.
func TestReceiverCheckStillHeldInsideTheCadence(t *testing.T) {
	appKey := strings.Repeat("ab", 32)
	svc, st := dueGateService(t, appKey)

	sweep := time.Date(2026, time.March, 11, 9, 15, 0, 0, time.Local)
	rr := seedDueGateRepo(t, st, appKey, "weekly Tue 05:00", sweep.AddDate(0, 0, -3))

	if err := svc.runReceiverChecksAt(context.Background(), sweep.Unix()); err != nil {
		t.Fatalf("runReceiverChecksAt: %v", err)
	}
	if checked(t, st, rr.ID) {
		t.Fatal("a weekly repo checked three days ago must not be checked again on today's sweep")
	}
}

// A cadence of "off" must not turn into a daily check.
func TestReceiverCheckOffCadenceNeverRuns(t *testing.T) {
	appKey := strings.Repeat("ab", 32)
	svc, st := dueGateService(t, appKey)

	sweep := time.Date(2026, time.March, 11, 9, 15, 0, 0, time.Local)
	rr := seedDueGateRepo(t, st, appKey, "off", time.Time{})

	if err := svc.runReceiverChecksAt(context.Background(), sweep.Unix()); err != nil {
		t.Fatalf("runReceiverChecksAt: %v", err)
	}
	if checked(t, st, rr.ID) {
		t.Fatal("a repo with its check cadence set to \"off\" must never be checked")
	}
}
