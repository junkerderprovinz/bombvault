package api

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// endEngine answers a received repo the way the far end stands: open is what
// opening it says, snaps what it holds.
type endEngine struct {
	ResticEngine
	open   error
	snaps  []restic.Snapshot
	checks int
}

func (e *endEngine) RepoOpensErr(context.Context, string, restic.Mode) error { return e.open }

func (e *endEngine) Snapshots(context.Context, string, restic.Mode) ([]restic.Snapshot, error) {
	return e.snaps, nil
}

func (e *endEngine) Check(context.Context, string, restic.Mode) error {
	e.checks++
	return nil
}

var (
	errHostDown = errors.New("dial tcp 192.168.1.9:8000: connect: connection refused")
	errNoRepo   = errors.New("repository does not exist: unable to open config file")
)

const deadEndNow = int64(2_000_000_000)

func deadEndFixture(t *testing.T, eng *endEngine, rr store.ReceivedRepo) (*Service, *store.Repo, func() []string, store.ReceivedRepo) {
	t.Helper()
	svc, st := dueGateService(t, strings.Repeat("ab", 32))
	svc.engine = eng
	alerts := catchAlerts(t, svc)
	seed := makeReceivedRepo(t, strings.Repeat("ab", 32), strings.Repeat("cd", 32), "rest:http://vault:8000/tower/files", 0)
	seed.Name, seed.DeadManHours, seed.CheckCadence = "tower", rr.DeadManHours, rr.CheckCadence
	seed.CreatedAt, seed.LastCheckAt, seed.LastCheckOK, seed.LastReadAt = rr.CreatedAt, rr.LastCheckAt, rr.LastCheckOK, rr.LastReadAt
	stored, err := st.CreateReceivedRepo(seed)
	if err != nil {
		t.Fatal(err)
	}
	return svc, st, alerts, stored
}

func TestADeadReceivingEndAlertsWhateverTheCheckSchedule(t *testing.T) {
	for _, cadence := range []string{"off", "weekly Sun 02:00", "daily 03:00"} {
		t.Run(cadence, func(t *testing.T) {
			eng := &endEngine{open: errHostDown}
			svc, _, alerts, _ := deadEndFixture(t, eng, store.ReceivedRepo{DeadManHours: 26, CheckCadence: cadence, CreatedAt: deadEndNow - 90*86400})
			if err := svc.runReceiverWatchAt(context.Background(), deadEndNow); err != nil {
				t.Fatal(err)
			}
			got := alerts()
			if len(got) != 1 || !strings.Contains(got[0], "cannot be read") || strings.Contains(got[0], "Integrity") {
				t.Fatalf("alerts = %q, want one saying the repository cannot be read", got)
			}
		})
	}
}

func TestADeadReceivingEndAlertsOnceUntilItIsBack(t *testing.T) {
	eng := &endEngine{open: errHostDown}
	svc, _, alerts, _ := deadEndFixture(t, eng, store.ReceivedRepo{DeadManHours: 26, CheckCadence: "off", CreatedAt: deadEndNow - 90*86400})
	day := int64(86400)
	for i := range int64(3) {
		if err := svc.runReceiverWatchAt(context.Background(), deadEndNow+i*day); err != nil {
			t.Fatal(err)
		}
	}
	if got := alerts(); len(got) != 1 {
		t.Fatalf("alerts over three days = %q, want one", got)
	}
	eng.open = nil
	eng.snaps = []restic.Snapshot{{ID: "aaa", Hostname: "tower", Tags: []string{"flash"}, Time: time.Unix(deadEndNow+3*day, 0).Format(time.RFC3339)}}
	if err := svc.runReceiverWatchAt(context.Background(), deadEndNow+3*day); err != nil {
		t.Fatal(err)
	}
	eng.open = errHostDown
	if err := svc.runReceiverWatchAt(context.Background(), deadEndNow+4*day); err != nil {
		t.Fatal(err)
	}
	if got := alerts(); len(got) != 2 {
		t.Fatalf("alerts after it came back and died again = %q, want a second one", got)
	}
}

func TestASenderThatNeverSentAlertsAfterTheWindow(t *testing.T) {
	cases := []struct {
		name string
		open error
	}{
		{"the repository is not there", errNoRepo},
		{"the repository is empty", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			eng := &endEngine{open: tc.open}
			created := deadEndNow - 3600
			svc, _, alerts, _ := deadEndFixture(t, eng, store.ReceivedRepo{DeadManHours: 26, CheckCadence: "off", CreatedAt: created})

			if err := svc.runReceiverWatchAt(context.Background(), deadEndNow); err != nil {
				t.Fatal(err)
			}
			if got := alerts(); len(got) != 0 {
				t.Fatalf("alerts an hour after adding it = %q", got)
			}
			if err := svc.runReceiverWatchAt(context.Background(), created+27*3600); err != nil {
				t.Fatal(err)
			}
			if err := svc.runReceiverWatchAt(context.Background(), created+51*3600); err != nil {
				t.Fatal(err)
			}
			got := alerts()
			if len(got) != 1 || !strings.Contains(got[0], "No backup has ever arrived on tower") {
				t.Fatalf("alerts past the window = %q, want one", got)
			}
		})
	}
}

func TestARepositoryThatWasReadAndWentIsNotTakenForANewOne(t *testing.T) {
	cases := []struct {
		name string
		rr   store.ReceivedRepo
	}{
		{"the switch read it", store.ReceivedRepo{LastReadAt: deadEndNow - 86400}},
		{"a check passed on it", store.ReceivedRepo{LastCheckOK: sql.NullBool{Bool: true, Valid: true}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			eng := &endEngine{open: errNoRepo}
			rr := tc.rr
			rr.DeadManHours, rr.CheckCadence, rr.CreatedAt = 26, "off", deadEndNow-3600
			svc, _, alerts, _ := deadEndFixture(t, eng, rr)
			if err := svc.runReceiverWatchAt(context.Background(), deadEndNow); err != nil {
				t.Fatal(err)
			}
			if got := alerts(); len(got) != 1 || !strings.Contains(got[0], "cannot be read") {
				t.Fatalf("alerts = %q, want one at once", got)
			}
		})
	}
}

func TestAReadRepositoryIsRemembered(t *testing.T) {
	eng := &endEngine{}
	svc, st, _, rr := deadEndFixture(t, eng, store.ReceivedRepo{DeadManHours: 26, CheckCadence: "off", CreatedAt: deadEndNow - 3600})
	if err := svc.runReceiverWatchAt(context.Background(), deadEndNow); err != nil {
		t.Fatal(err)
	}
	got, _, _ := st.GetReceivedRepo(rr.ID)
	if got.LastReadAt != deadEndNow {
		t.Fatalf("lastReadAt = %d", got.LastReadAt)
	}
}

func TestACheckThatCannotOpenTheRepositoryIsNoIntegrityVerdict(t *testing.T) {
	for _, tc := range []struct {
		name string
		open error
	}{
		{"saved before the first copy", errNoRepo},
		{"the host is merely down", errHostDown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			eng := &endEngine{open: tc.open}
			fire := time.Date(2033, time.May, 18, 3, 33, 20, 0, time.Local)
			svc, st, alerts, rr := deadEndFixture(t, eng, store.ReceivedRepo{DeadManHours: 26, CheckCadence: "daily 03:00", CreatedAt: fire.AddDate(0, 0, -1).Unix()})
			if err := svc.runReceiverChecksAt(context.Background(), fire.Unix()); err != nil {
				t.Fatal(err)
			}
			if got := alerts(); len(got) != 0 {
				t.Fatalf("alerts = %q; nothing is known about integrity", got)
			}
			got, _, _ := st.GetReceivedRepo(rr.ID)
			if got.LastCheckOK.Valid || got.LastCheckAt != 0 {
				t.Fatalf("verdict = %+v at %d, want none", got.LastCheckOK, got.LastCheckAt)
			}
		})
	}
}

func TestAnUnreadCheckWaitsForTheNextDaySlotAndAWorkingOneRuns(t *testing.T) {
	eng := &endEngine{open: errHostDown}
	fire := time.Date(2033, time.May, 15, 2, 0, 0, 0, time.Local)
	svc, st, _, rr := deadEndFixture(t, eng, store.ReceivedRepo{DeadManHours: 26, CheckCadence: "weekly Sun 02:00", CreatedAt: fire.AddDate(0, 0, -1).Unix()})
	if err := svc.runReceiverChecksAt(context.Background(), fire.Unix()); err != nil {
		t.Fatal(err)
	}
	eng.open = nil
	if err := svc.runReceiverChecksAt(context.Background(), fire.Add(time.Minute).Unix()); err != nil {
		t.Fatal(err)
	}
	if eng.checks != 0 {
		t.Fatal("tried again at the next tick")
	}
	if err := svc.runReceiverChecksAt(context.Background(), fire.AddDate(0, 0, 1).Unix()); err != nil {
		t.Fatal(err)
	}
	if eng.checks != 1 {
		t.Fatalf("checks = %d, want one the next day", eng.checks)
	}
	if got, _, _ := st.GetReceivedRepo(rr.ID); !got.LastCheckOK.Valid || !got.LastCheckOK.Bool {
		t.Fatalf("verdict = %+v", got.LastCheckOK)
	}
}

func TestARealFailureStillAlertsAfterAnUnreadCheck(t *testing.T) {
	eng := &endEngine{open: errNoRepo}
	fire := time.Date(2033, time.May, 15, 2, 0, 0, 0, time.Local)
	svc, _, alerts, _ := deadEndFixture(t, eng, store.ReceivedRepo{DeadManHours: 26, CheckCadence: "daily 02:00", CreatedAt: fire.AddDate(0, 0, -1).Unix()})
	if err := svc.runReceiverChecksAt(context.Background(), fire.Unix()); err != nil {
		t.Fatal(err)
	}
	eng.open = nil
	failing := &failingCheck{endEngine: eng}
	svc.engine = failing
	if err := svc.runReceiverChecksAt(context.Background(), fire.AddDate(0, 0, 1).Unix()); err != nil {
		t.Fatal(err)
	}
	if got := alerts(); len(got) != 1 || !strings.Contains(got[0], "Integrity check FAILED") {
		t.Fatalf("alerts = %q, want the real failure", got)
	}
}

type failingCheck struct{ *endEngine }

func (e *failingCheck) Check(context.Context, string, restic.Mode) error {
	return errors.New("Fatal: repository contains errors")
}
