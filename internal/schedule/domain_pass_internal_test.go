package schedule

import (
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// A scheduled containers pass is recorded for the everyN gate once it backed
// something up, and not when every container failed.
func TestDomainPassIsRecordedOnlyWhenItBackedSomethingUp(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"backed up", nil, 1},
		{"all failed", errors.New("repository locked"), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := New(
				func(string) error { return tc.err },
				func() ([]store.Target, error) {
					return []store.Target{{ContainerName: "plex", IncludeInSchedule: true}}, nil
				},
			)
			jr := newFakeJobRuns()
			sc.SetJobRunStore(jr)
			settings := store.Settings{ContainersEnabled: true, ContainersSchedule: "daily 03:00"}
			if err := sc.ReloadWithDueChecks(settings, nil, nil, nil, nil, nil, nil); err != nil {
				t.Fatal(err)
			}
			fired := 0
			for _, e := range sc.entries {
				if e.job == "backup" && e.domain == "containers" {
					fired++
					sc.c.Entry(e.id).WrappedJob.Run()
				}
			}
			if fired != 1 {
				t.Fatalf("fired %d containers entries, want 1", fired)
			}
			if got := jr.recordCount(store.ScheduleJobDomainPass("containers")); got != tc.want {
				t.Fatalf("recorded %d passes, want %d", got, tc.want)
			}
		})
	}
}
