package schedule

import (
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestContainersJobBatchedOffsiteRunsAfterAllBackups pins the #95 replication
// rewrite: a scheduled containers run backs up every included container FIRST and
// then runs the batched off-site replication ONCE, after the whole loop — never
// per-container inline. White-box (package schedule) so it can fire the registered
// cron entry synchronously and observe call ordering.
func TestContainersJobBatchedOffsiteRunsAfterAllBackups(t *testing.T) {
	var mu sync.Mutex
	var events []string

	backupFn := func(name string) error {
		mu.Lock()
		events = append(events, "backup:"+name)
		mu.Unlock()
		return nil
	}
	listFn := func() ([]store.Target, error) {
		return []store.Target{
			{ContainerName: "plex", IncludeInSchedule: true},
			{ContainerName: "radarr", IncludeInSchedule: true},
		}, nil
	}

	sc := New(backupFn, listFn)
	sc.SetOffsiteAfterBulkJob(func(domain string) {
		mu.Lock()
		events = append(events, "offsite:"+domain)
		mu.Unlock()
	})

	s := store.Settings{ContainersSchedule: "daily 03:00"}
	if err := sc.ReloadWithDueChecks(s, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks: %v", err)
	}

	// Fire the containers entry synchronously through its wrapped job (the same path
	// cron would run), so we observe the real fn including the post-loop batched call.
	fired := false
	for _, e := range sc.entries {
		if e.domain == "containers" {
			sc.c.Entry(e.id).WrappedJob.Run()
			fired = true
		}
	}
	if !fired {
		t.Fatal("no containers entry registered")
	}

	mu.Lock()
	defer mu.Unlock()
	want := []string{"backup:plex", "backup:radarr", "offsite:containers"}
	if len(events) != len(want) {
		t.Fatalf("events = %v, want %v (backups then ONE batched off-site copy)", events, want)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("events = %v, want %v", events, want)
		}
	}
}

// TestContainersJobNoOffsiteAfterBulkWhenUnwired ensures the batched off-site hook
// is optional: with no SetOffsiteAfterBulkJob wired, the run still backs up every
// container and simply performs no batched replication (nil-guarded).
func TestContainersJobNoOffsiteAfterBulkWhenUnwired(t *testing.T) {
	var mu sync.Mutex
	var backups int

	sc := New(
		func(string) error { mu.Lock(); backups++; mu.Unlock(); return nil },
		func() ([]store.Target, error) {
			return []store.Target{{ContainerName: "plex", IncludeInSchedule: true}}, nil
		},
	)
	// Deliberately NOT calling SetOffsiteAfterBulkJob.

	if err := sc.ReloadWithDueChecks(store.Settings{ContainersSchedule: "daily 03:00"}, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("ReloadWithDueChecks: %v", err)
	}
	for _, e := range sc.entries {
		if e.domain == "containers" {
			sc.c.Entry(e.id).WrappedJob.Run()
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if backups != 1 {
		t.Fatalf("expected 1 backup, got %d", backups)
	}
}
