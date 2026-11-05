package api

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// pullSweepFixture has one source on cadence, saved long before the sweep.
func pullSweepFixture(t *testing.T, cadence string, saved time.Time) (*Service, *store.Repo, *pullRecorder, store.PullSource) {
	t.Helper()
	eng := &pullRecorder{snaps: map[string][]restic.Snapshot{pullSrcLoc: {{ID: "aaa"}}}}
	svc, st, ps := pullFixture(t, eng)
	if _, err := st.MutateSettings(func(s *store.Settings) error {
		s.PullEnabled = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.DeletePullSource(ps.ID); err != nil {
		t.Fatal(err)
	}
	ps.Cadence, ps.CreatedAt = cadence, saved.Unix()
	ps, err := st.CreatePullSource(ps)
	if err != nil {
		t.Fatal(err)
	}
	return svc, st, eng, ps
}

func pullCopied(eng *pullRecorder) bool {
	for _, c := range eng.calls {
		if strings.HasPrefix(c, "Copy dest=") {
			return true
		}
	}
	return false
}

func TestAWeeklySourceWaitsForItsDayAndTime(t *testing.T) {
	thursday := time.Date(2026, time.November, 5, 9, 30, 0, 0, time.Local)
	sunday := time.Date(2026, time.November, 8, 2, 0, 0, 0, time.Local)
	svc, _, eng, _ := pullSweepFixture(t, "weekly Sun 02:00", thursday.Add(-time.Hour))

	if err := svc.runPullsAt(context.Background(), thursday); err != nil {
		t.Fatal(err)
	}
	if pullCopied(eng) {
		t.Fatalf("a weekly Sunday source pulled on a Thursday: %v", eng.calls)
	}
	if err := svc.runPullsAt(context.Background(), sunday.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if pullCopied(eng) {
		t.Fatalf("pulled a minute before its time: %v", eng.calls)
	}
	if err := svc.runPullsAt(context.Background(), sunday); err != nil {
		t.Fatal(err)
	}
	if !pullCopied(eng) {
		t.Fatalf("no pull at its time: %v", eng.calls)
	}
}

func TestANewSourceWaitsForItsFirstFire(t *testing.T) {
	now := time.Date(2026, time.November, 5, 9, 30, 0, 0, time.Local)
	svc, _, eng, _ := pullSweepFixture(t, "daily 04:00", now.Add(-time.Hour))
	if err := svc.runPullsAt(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if pullCopied(eng) {
		t.Fatal("a source saved after today's 04:00 pulled at once")
	}
}

func TestAWeeklySourceThatFailedRetriesTheNextDayAtItsTime(t *testing.T) {
	sunday := time.Date(2026, time.November, 8, 2, 0, 20, 0, time.Local)
	svc, st, eng, ps := pullSweepFixture(t, "weekly Sun 02:00", sunday.AddDate(0, -1, 0))
	if err := st.UpdatePullSourceResult(ps.ID, sunday.Unix(), sql.NullBool{Bool: false, Valid: true}, "the source did not answer", 0); err != nil {
		t.Fatal(err)
	}

	if err := svc.runPullsAt(context.Background(), sunday.Add(3*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if pullCopied(eng) {
		t.Fatal("retried the same night, outside its time")
	}
	if err := svc.runPullsAt(context.Background(), sunday.AddDate(0, 0, 1)); err != nil {
		t.Fatal(err)
	}
	if !pullCopied(eng) {
		t.Fatal("a failed weekly source did not retry the next day")
	}
}

func TestASourceThatWorkedDoesNotRunAgainAtTheNextTick(t *testing.T) {
	sunday := time.Date(2026, time.November, 8, 2, 0, 0, 0, time.Local)
	svc, st, eng, ps := pullSweepFixture(t, "weekly Sun 02:00", sunday.AddDate(0, -1, 0))
	if err := st.UpdatePullSourceResult(ps.ID, sunday.Add(10*time.Second).Unix(), sql.NullBool{Bool: true, Valid: true}, "", 1); err != nil {
		t.Fatal(err)
	}
	if err := svc.runPullsAt(context.Background(), sunday.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if pullCopied(eng) {
		t.Fatal("pulled again one tick after a good pull")
	}
}

func TestAnOffSourceOnlyRunsFromTheButton(t *testing.T) {
	now := time.Date(2026, time.November, 5, 9, 30, 0, 0, time.Local)
	svc, _, eng, _ := pullSweepFixture(t, "off", now.AddDate(0, -1, 0))
	if err := svc.runPullsAt(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if pullCopied(eng) {
		t.Fatal("an off source was pulled")
	}
	if _, ok := svc.NextPull(now); ok {
		t.Fatal("an off source has a next pull")
	}
}

func TestTheNextPullIsTheSoonestSourcesOwnFire(t *testing.T) {
	now := time.Date(2026, time.November, 5, 9, 30, 0, 0, time.Local)
	svc, _, _, _ := pullSweepFixture(t, "weekly Sun 02:00", now.AddDate(0, -1, 0))
	next, ok := svc.NextPull(now)
	if want := time.Date(2026, time.November, 8, 2, 0, 0, 0, time.Local); !ok || !next.Equal(want) {
		t.Fatalf("next = %v, %v, want %v", next, ok, want)
	}
}
