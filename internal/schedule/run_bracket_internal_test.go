package schedule

import (
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func bracketEvents(p *idleProbe, sc *Scheduler) {
	record := func(e string) {
		p.mu.Lock()
		p.events = append(p.events, e)
		p.mu.Unlock()
	}
	sc.SetRunBracket(func(domain string) func() {
		record("open:" + domain)
		return func() { record("close:" + domain) }
	})
	sc.SetPruneAfterBulkJob(func(domain string) { record("prune:" + domain) })
}

// A scheduled run stays open from its first item through the prune and the
// off-site copy after the last one, so nothing else takes the domain in a gap.
func TestAScheduledRunIsOpenFromItsFirstItemUntilItsCopy(t *testing.T) {
	p := &idleProbe{busy: map[string]bool{"plex": true}, pending: map[string]func(){}}
	sc := newIdleScheduler(t, p, store.Settings{ContainersEnabled: true, ContainersSchedule: "daily 03:00", FilesEnabled: true, FilesSchedule: "daily 04:00"}, []store.Target{
		{ContainerName: "plex", IncludeInSchedule: true},
		{ContainerName: "radarr", IncludeInSchedule: true},
		{ContainerName: "sonarr", IncludeInSchedule: true},
	})
	bracketEvents(p, sc)

	fireDomain(t, sc, "containers")
	want := []string{"open:containers", "backup:radarr", "backup:sonarr", "stacks:radarr,sonarr", "prune:containers", "offsite:containers", "close:containers"}
	if got := p.got(); !slices.Equal(got, want) {
		t.Fatalf("domain run = %v, want %v", got, want)
	}

	p.pending["plex"]()
	want = []string{"open:containers", "backup:plex", "stacks:plex", "prune:containers", "offsite:containers", "close:containers"}
	if got := p.got()[7:]; !slices.Equal(got, want) {
		t.Fatalf("held run = %v, want %v", got, want)
	}
}

func TestAScheduledFolderRunIsOpenUntilItsCopy(t *testing.T) {
	p := &idleProbe{busy: map[string]bool{}, pending: map[string]func(){}}
	sc := newIdleScheduler(t, p, store.Settings{FilesEnabled: true, FilesSchedule: "daily 04:00"}, nil)
	bracketEvents(p, sc)
	sc.SetFilesJob(func(id string) error { return p.backup(id) }, func() ([]store.FileSet, error) {
		return []store.FileSet{{ID: "docs", Enabled: true}}, nil
	})
	if err := sc.ReloadWithDueChecks(store.Settings{FilesEnabled: true, FilesSchedule: "daily 04:00"}, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	fireDomain(t, sc, "files")
	want := []string{"open:files", "backup:docs", "prune:files", "offsite:files", "close:files"}
	if got := p.got(); !slices.Equal(got, want) {
		t.Fatalf("folder run = %v, want %v", got, want)
	}
}
