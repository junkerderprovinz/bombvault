package schedule

import (
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

type runRecorder struct {
	mu     sync.Mutex
	events []string
}

func (r *runRecorder) record(e string) {
	r.mu.Lock()
	r.events = append(r.events, e)
	r.mu.Unlock()
}

func (r *runRecorder) got() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.events)
}

func newBracketScheduler(t *testing.T, r *runRecorder, targets []store.Target) *Scheduler {
	t.Helper()
	sc := New(func(name string) error { r.record("backup:" + name); return nil },
		func() ([]store.Target, error) { return targets, nil })
	sc.SetRunBracket(func(domain string) func() {
		r.record("open:" + domain)
		return func() { r.record("close:" + domain) }
	})
	sc.SetStacksAfterBulkJob(func(names []string) { r.record("stacks:" + strings.Join(names, ",")) })
	sc.SetPruneAfterBulkJob(func(domain string) { r.record("prune:" + domain) })
	sc.SetOffsiteAfterBulkJob(func(domain string) { r.record("offsite:" + domain) })
	return sc
}

func fireDomainEntry(t *testing.T, sc *Scheduler, domain string) {
	t.Helper()
	for _, e := range sc.entries {
		if e.domain == domain {
			sc.c.Entry(e.id).WrappedJob.Run()
			return
		}
	}
	t.Fatalf("no %s entry", domain)
}

// A scheduled run stays open from its first item through the prune and the
// off-site copy after the last one, so nothing else takes the domain in a gap.
func TestAScheduledRunIsOpenFromItsFirstItemUntilItsCopy(t *testing.T) {
	r := &runRecorder{}
	sc := newBracketScheduler(t, r, []store.Target{
		{ContainerName: "radarr", IncludeInSchedule: true},
		{ContainerName: "sonarr", IncludeInSchedule: true},
	})
	if err := sc.ReloadWithDueChecks(store.Settings{ContainersEnabled: true, ContainersSchedule: "daily 03:00"}, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	fireDomainEntry(t, sc, "containers")
	want := []string{"open:containers", "backup:radarr", "backup:sonarr", "stacks:radarr,sonarr", "prune:containers", "offsite:containers", "close:containers"}
	if got := r.got(); !slices.Equal(got, want) {
		t.Fatalf("domain run = %v, want %v", got, want)
	}
}

func TestAScheduledFolderRunIsOpenUntilItsCopy(t *testing.T) {
	r := &runRecorder{}
	sc := newBracketScheduler(t, r, nil)
	sc.SetFilesJob(func(id string) error { r.record("backup:" + id); return nil }, func() ([]store.FileSet, error) {
		return []store.FileSet{{ID: "docs", Enabled: true}}, nil
	})
	if err := sc.ReloadWithDueChecks(store.Settings{FilesEnabled: true, FilesSchedule: "daily 04:00"}, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	fireDomainEntry(t, sc, "files")
	want := []string{"open:files", "backup:docs", "prune:files", "offsite:files", "close:files"}
	if got := r.got(); !slices.Equal(got, want) {
		t.Fatalf("folder run = %v, want %v", got, want)
	}
}
