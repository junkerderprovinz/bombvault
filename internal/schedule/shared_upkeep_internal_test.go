package schedule

import (
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestTheUpkeepOfSharedRepositoriesTicksAndShowsItsNextRun(t *testing.T) {
	noTargets := func() ([]store.Target, error) { return nil, nil }
	sc := New(func(string) error { return nil }, noTargets)
	fired := 0
	due := time.Now().Add(3 * time.Hour)
	sc.SetSharedUpkeepJob(func() error { fired++; return nil }, func(time.Time) (time.Time, bool) { return due, true })

	if err := sc.ReloadWithDueChecks(store.Settings{}, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks: %v", err)
	}
	found := false
	for _, e := range sc.entries {
		if e.job == "receiver" && e.next != nil {
			if got, ok := e.next(time.Now()); ok && got.Equal(due) {
				found = true
				sc.c.Entry(e.id).WrappedJob.Run()
			}
		}
	}
	if !found || fired != 1 {
		t.Fatalf("found %v, fired %d times; want the upkeep registered and run once", found, fired)
	}
}
