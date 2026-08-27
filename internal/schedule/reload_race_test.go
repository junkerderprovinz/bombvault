package schedule_test

// Two reloads arriving together must leave ONE set of entries, not two.
//
// mu is deliberately dropped between clearing the old entries and registering
// the new ones, because cron must never be called while holding it. That gap is
// what let two concurrent reloads interleave: both snapshot the same old set,
// both remove it, and both register a full set — leaving every domain in cron
// twice. SkipIfStillRunning does not help, since it only stops an entry
// overlapping ITSELF and these are two distinct entries, so a nightly backup
// ran twice over the same repo.

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

	// Hammer it from several goroutines at once: the window is a few
	// instructions wide, so one pair would be a coin flip.
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
