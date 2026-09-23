package schedule_test

// Two concurrent reloads must leave one set of entries. Reload drops mu between
// clearing the old entries and registering the new ones, because cron must not
// be called while holding it, so two reloads could each register a full set.
// SkipIfStillRunning only stops an entry from overlapping itself, so the
// duplicates would each run the nightly backup over the same repo.

import (
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestConcurrentReloadsDoNotDoubleRegister(t *testing.T) {
	sc := schedule.New(func(string) error { return nil }, func() ([]store.Target, error) { return nil, nil })
	settings := store.Settings{
		ContainersEnabled:  true,
		ContainersSchedule: "daily 03:00",
		FlashEnabled:       true,
		FlashSchedule:      "daily 04:00",
	}

	// The window is a few instructions wide, so a single pair would rarely hit
	// it.
	const workers = 8
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			if err := sc.Reload(settings); err != nil {
				t.Errorf("Reload: %v", err)
			}
		}()
	}
	wg.Wait()

	sc.Start()
	defer sc.Stop()

	counts := map[string]int{}
	for _, r := range sc.NextRuns() {
		counts[r.Job+":"+r.Domain]++
	}
	for key, n := range counts {
		if n != 1 {
			t.Fatalf("%s registered %d times after %d concurrent reloads, want 1 (all: %v)", key, n, workers, counts)
		}
	}
	if len(counts) == 0 {
		t.Fatal("no entries registered at all")
	}
}
