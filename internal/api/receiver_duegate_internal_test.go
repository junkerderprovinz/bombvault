package api

// ---------------------------------------------------------------------------
// The received-repo integrity check runs on its configured cadence, not double
// it.
//
// The receiver watch is a FIXED daily sweep (schedule.ReceiverCadence, "daily
// 09:15") that walks every enabled received repo and asks each one whether its
// own CheckCadence is due. `now` is the sweep's fire time; last_check_at is
// stamped when the previous check FINISHED, and an independent restic check on a
// received repo takes minutes. So the day a "daily 04:00" repo comes due,
// elapsed seconds measured a few minutes short of 86400, the gate closed, and
// the next chance was the following day's sweep: every received repo was checked
// every 48h, an "everyN 7" one every eight days, and the skipped day logged
// nothing at all.
//
// These drive the real sweep (runReceiverChecksAt) over a real store, so what is
// pinned is the whole tick → gate → check → persist chain. No restic binary is
// needed: the check fails (there is no repo at the path), which is still a
// definite verdict the sweep persists, and "was a verdict written?" is exactly
// the observation these need.
// ---------------------------------------------------------------------------

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
// muted — these tests are about whether the CHECK runs, not about alerts.
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
	// The real engine, pointed at a path with no repo: the check FAILS, which is
	// still a definite verdict the sweep persists, and no restic binary is needed
	// for that. Whether a verdict was written is exactly the observation here.
	return &Service{cfg: config.Config{AppKey: appKey}, store: st, engine: restic.Restic{Bin: "restic"}}, st
}

// seedDueGateRepo registers one enabled received repo on `cadence` whose last
// check FINISHED at `lastCheck` with no verdict recorded yet, so a check that
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

// TestReceiverDailyCheckRunsTheNextDayNotTheDayAfter is the regression. A daily
// repo whose previous check finished ten minutes after yesterday's sweep must be
// checked on TODAY's sweep.
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
		t.Fatal("a repo on a DAILY check cadence was not checked on the next day's sweep — " +
			"it is being checked every 48h, at half the configured frequency, and the skipped day logs nothing")
	}
}

// TestReceiverWeeklyCheckRunsOnTheSeventhDay is the same slip at the weekly
// cadence, where it costs a check every 14 days instead of every 7.
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

// TestReceiverCheckStillHeldInsideTheCadence pins the other half: the gate must
// still close, or every daily sweep would run a full restic check on every
// received repo regardless of what the user configured.
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

// TestReceiverCheckOffCadenceNeverRuns pins that "off" still means off — the new
// gate must not turn a disabled cadence into a daily check.
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
