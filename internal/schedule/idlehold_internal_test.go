package schedule

import (
	"slices"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// idleProbe records backups and holds the containers named in busy.
type idleProbe struct {
	mu      sync.Mutex
	events  []string
	busy    map[string]bool
	pending map[string]func()
}

func (p *idleProbe) backup(name string) error {
	p.mu.Lock()
	p.events = append(p.events, "backup:"+name)
	p.mu.Unlock()
	return nil
}

func (p *idleProbe) hold(t store.Target, run func()) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.busy[t.ContainerName] {
		return false
	}
	p.pending[t.ContainerName] = run
	return true
}

func (p *idleProbe) got() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.events)
}

func newIdleScheduler(t *testing.T, p *idleProbe, settings store.Settings, targets []store.Target) *Scheduler {
	t.Helper()
	sc := New(p.backup, func() ([]store.Target, error) { return targets, nil })
	sc.SetIdleHold(p.hold)
	sc.SetOffsiteAfterBulkJob(func(domain string) {
		p.mu.Lock()
		p.events = append(p.events, "offsite:"+domain)
		p.mu.Unlock()
	})
	if err := sc.ReloadWithDueChecks(settings, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	return sc
}

func fireDomain(t *testing.T, sc *Scheduler, domain string) {
	t.Helper()
	for _, e := range sc.entries {
		if e.domain == domain {
			sc.c.Entry(e.id).WrappedJob.Run()
			return
		}
	}
	t.Fatalf("no %s entry", domain)
}

func TestABusyContainerLeavesTheDomainRunAndRunsLaterOnItsOwn(t *testing.T) {
	p := &idleProbe{busy: map[string]bool{"plex": true}, pending: map[string]func(){}}
	sc := newIdleScheduler(t, p, store.Settings{ContainersEnabled: true, ContainersSchedule: "daily 03:00"}, []store.Target{
		{ContainerName: "plex", IncludeInSchedule: true},
		{ContainerName: "radarr", IncludeInSchedule: true},
	})

	fireDomain(t, sc, "containers")
	if got := p.got(); !slices.Equal(got, []string{"backup:radarr", "offsite:containers"}) {
		t.Fatalf("domain run = %v, want radarr without plex", got)
	}
	run := p.pending["plex"]
	if run == nil {
		t.Fatal("plex was not handed to the wait")
	}
	run()
	if got := p.got(); !slices.Equal(got[2:], []string{"backup:plex", "offsite:containers"}) {
		t.Fatalf("after the wait = %v, want plex backed up and copied off-site", got)
	}
}

func TestAnIdleContainerStaysInTheDomainRun(t *testing.T) {
	p := &idleProbe{busy: map[string]bool{}, pending: map[string]func(){}}
	sc := newIdleScheduler(t, p, store.Settings{ContainersEnabled: true, ContainersSchedule: "daily 03:00"}, []store.Target{
		{ContainerName: "plex", IncludeInSchedule: true},
	})
	fireDomain(t, sc, "containers")
	if got := p.got(); !slices.Equal(got, []string{"backup:plex", "offsite:containers"}) {
		t.Fatalf("domain run = %v", got)
	}
}

func TestAnExcludedContainerIsNeverOfferedToTheWait(t *testing.T) {
	p := &idleProbe{busy: map[string]bool{"plex": true}, pending: map[string]func(){}}
	sc := newIdleScheduler(t, p, store.Settings{ContainersEnabled: true, ContainersSchedule: "daily 03:00"}, []store.Target{
		{ContainerName: "plex", IncludeInSchedule: false},
		{ContainerName: "radarr", IncludeInSchedule: true},
	})
	fireDomain(t, sc, "containers")
	if len(p.pending) != 0 {
		t.Fatalf("an excluded container waits: %v", p.pending)
	}
}

func TestAPerItemFireOfABusyContainerWaitsToo(t *testing.T) {
	p := &idleProbe{busy: map[string]bool{"plex": true}, pending: map[string]func(){}}
	sc := newIdleScheduler(t, p, store.Settings{ContainersEnabled: true, ContainersSchedule: "off", PerItemSchedules: true}, []store.Target{
		{ContainerName: "plex", IncludeInSchedule: true, ScheduleCadence: "daily 04:00"},
	})
	fireDomain(t, sc, "containers")
	if got := p.got(); len(got) != 0 {
		t.Fatalf("the per-item fire backed up at once: %v", got)
	}
	p.pending["plex"]()
	if got := p.got(); !slices.Equal(got, []string{"backup:plex", "offsite:containers"}) {
		t.Fatalf("after the wait = %v", got)
	}
}

func TestRunContainerNowSkipsAContainerExcludedMeanwhile(t *testing.T) {
	p := &idleProbe{busy: map[string]bool{}, pending: map[string]func(){}}
	targets := []store.Target{{ContainerName: "plex", IncludeInSchedule: false}}
	sc := New(p.backup, func() ([]store.Target, error) { return targets, nil })
	sc.RunContainerNow("plex")
	sc.RunContainerNow("gone")
	if got := p.got(); len(got) != 0 {
		t.Fatalf("backed up %v", got)
	}
}
