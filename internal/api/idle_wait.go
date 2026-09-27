package api

import (
	"context"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/traffic"
)

// IdleWait is a scheduled container backup held back until its app is idle.
type IdleWait struct {
	Domain string `json:"domain"`
	Name   string `json:"name"`
	// Stack is the compose project whose members wait together, "" for a
	// container that waits alone.
	Stack string `json:"stack,omitempty"`
	// Busy names the container whose app holds the wait at the last look. In a
	// stack it can be another member than Name.
	Busy string `json:"busy"`
	// Reason is why Busy counts as busy: one of the traffic.Busy constants.
	Reason   string `json:"reason"`
	Since    int64  `json:"since"`
	Deadline int64  `json:"deadline"`
}

// idleGroup is one held-back backup: a container, or the members of a compose
// stack that were due in the same run. A stack waits as a whole, because its
// members are backed up for one restore point with their project folder.
type idleGroup struct {
	store.IdleWaitGroup
	busy   string
	reason string
	run    func(names []string)
}

type idleWaits struct {
	mu     sync.Mutex
	groups map[string]*idleGroup
}

// idleCheck is how often a waiting backup looks at its app again.
var idleCheck = trafficPoll

// scheduledPassKey marks the context of a scheduled "Backup Everything" pass,
// whose containers wait for an idle app like any other scheduled run.
type scheduledPassKey struct{}

// WithScheduledPass marks ctx as a scheduled "Backup Everything" pass.
func WithScheduledPass(ctx context.Context) context.Context {
	return context.WithValue(ctx, scheduledPassKey{}, true)
}

func scheduledPass(ctx context.Context) bool {
	v, _ := ctx.Value(scheduledPassKey{}).(bool)
	return v
}

// SetHeldContainerRun names what backs up containers once their wait is over,
// for the backups a scheduled "Backup Everything" pass held back and the ones
// resumed after a restart.
func (s *Service) SetHeldContainerRun(fn func(names []string)) {
	s.heldRun = fn
}

func (s *Service) waits() *idleWaits {
	s.waitsOnce.Do(func() { s.idleWaits = &idleWaits{groups: map[string]*idleGroup{}} })
	return s.idleWaits
}

// groupOf is the waiting group a container belongs to, nil when it waits for
// nothing. The caller holds w.mu.
func (w *idleWaits) groupOf(name string) *idleGroup {
	for _, g := range w.groups {
		if slices.Contains(g.Members, name) {
			return g
		}
	}
	return nil
}

// HoldForIdle takes the scheduled containers whose app is busy out of a run and
// returns their names. They back up with run once their apps are idle, or
// anyway at the deadline. A busy member takes the rest of its compose stack
// in the same run along, and a container already waiting is absorbed by its
// wait.
func (s *Service) HoldForIdle(targets []store.Target, trigger string, run func(names []string)) []string {
	if _, ok := s.docker.(statsReader); !ok || len(targets) == 0 {
		return nil
	}
	hours, err := s.store.IdleWaitHours()
	if err != nil {
		log.Printf("api: idle wait: %v", err)
		return nil
	}
	w := s.waits()
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(hours) == 0 && len(w.groups) == 0 {
		return nil
	}
	stacks := s.containerStacks()
	now := time.Now()

	var held []string
	fresh := map[string]*idleGroup{}
	touched := map[string]*idleGroup{}
	join := func(key, stack, name string) *idleGroup {
		g := w.groups[key]
		if g == nil {
			if g = fresh[key]; g == nil {
				g = &idleGroup{IdleWaitGroup: store.IdleWaitGroup{Key: key, Stack: stack, Trigger: trigger, Since: now.Unix()}, run: run}
				fresh[key] = g
			}
		}
		if !slices.Contains(g.Members, name) {
			g.Members = append(g.Members, name)
			touched[key] = g
		}
		held = append(held, name)
		return g
	}

	for _, t := range targets {
		name := t.ContainerName
		if g := w.groupOf(name); g != nil {
			held = append(held, name)
			continue
		}
		h := hours[t.ID]
		if h <= 0 {
			continue
		}
		idle, reason := s.appIdle(name, now)
		if idle {
			continue
		}
		key := name
		if stack := stacks[name]; stack != "" {
			key = "stack:" + stack
		}
		g := join(key, stacks[name], name)
		// A stack waits no longer than the shortest wait any busy member
		// allows.
		deadline := now.Add(time.Duration(h) * time.Hour).Unix()
		if fresh[key] == g && (g.Deadline == 0 || deadline < g.Deadline) {
			g.Deadline, g.busy, g.reason = deadline, name, reason
		}
	}
	// The members of a waiting stack that were not busy themselves go along.
	for _, t := range targets {
		name := t.ContainerName
		stack := stacks[name]
		if stack == "" || slices.Contains(held, name) {
			continue
		}
		key := "stack:" + stack
		if w.groups[key] != nil || fresh[key] != nil {
			join(key, stack, name)
		}
	}

	for key, g := range touched {
		w.groups[key] = g
		if err := s.store.SaveIdleWaitGroup(g.IdleWaitGroup); err != nil {
			log.Printf("api: idle wait: remember %s: %v", key, err)
		}
	}
	for _, g := range fresh {
		log.Printf("api: scheduled backup of %s waits for %s to be idle (%s), at the latest until %s", //nolint:gosec // G706: container names from Docker
			strings.Join(g.Members, ", "), g.busy, g.reason, time.Unix(g.Deadline, 0).Format(time.RFC3339))
		go s.waitForIdle(g)
	}
	return held
}

// ResumeIdleWaits picks up the waits a restart interrupted. A wait whose
// deadline passed meanwhile backs up at once, and a container that was
// removed or taken off the schedule is dropped from its wait.
func (s *Service) ResumeIdleWaits() {
	groups, err := s.store.ListIdleWaitGroups()
	if err != nil {
		log.Printf("api: idle wait: resume: %v", err)
		return
	}
	if len(groups) == 0 || s.heldRun == nil {
		return
	}
	targets, err := s.store.ListTargets()
	if err != nil {
		log.Printf("api: idle wait: resume: %v", err)
		return
	}
	scheduled := map[string]bool{}
	for _, t := range targets {
		scheduled[t.ContainerName] = t.IncludeInSchedule
	}
	now := time.Now().Unix()
	w := s.waits()
	for _, g := range groups {
		members := slices.DeleteFunc(slices.Clone(g.Members), func(n string) bool { return !scheduled[n] })
		if len(members) < len(g.Members) {
			log.Printf("api: idle wait: %s left the schedule while BombVault was down, dropped from its wait", //nolint:gosec // G706: container names from Docker
				strings.Join(slices.DeleteFunc(slices.Clone(g.Members), func(n string) bool { return scheduled[n] }), ", "))
		}
		if len(members) == 0 {
			s.forgetIdleGroup(g.Key)
			continue
		}
		g.Members = members
		if now >= g.Deadline {
			s.forgetIdleGroup(g.Key)
			log.Printf("api: starting the held backup of %s: its wait ended while BombVault was down", strings.Join(members, ", ")) //nolint:gosec // G706: container names from Docker
			go s.heldRun(members)
			continue
		}
		ig := &idleGroup{IdleWaitGroup: g, busy: members[0], reason: traffic.BusyMeasuring, run: s.heldRun}
		w.mu.Lock()
		w.groups[g.Key] = ig
		w.mu.Unlock()
		if err := s.store.SaveIdleWaitGroup(g); err != nil {
			log.Printf("api: idle wait: remember %s: %v", g.Key, err)
		}
		log.Printf("api: scheduled backup of %s waits again for an idle app, at the latest until %s", //nolint:gosec // G706: container names from Docker
			strings.Join(members, ", "), time.Unix(g.Deadline, 0).Format(time.RFC3339))
		go s.waitForIdle(ig)
	}
}

func (s *Service) forgetIdleGroup(key string) {
	w := s.waits()
	w.mu.Lock()
	delete(w.groups, key)
	w.mu.Unlock()
	if err := s.store.DeleteIdleWaitGroup(key); err != nil {
		log.Printf("api: idle wait: forget %s: %v", key, err)
	}
}

func (s *Service) waitForIdle(g *idleGroup) {
	t := time.NewTicker(idleCheck)
	defer t.Stop()
	for now := range t.C {
		why := ""
		busy, reason, waiting := s.groupBusy(g, now)
		switch {
		case !waiting:
			why = "the wait was switched off"
		case busy == "":
			why = "the apps are idle"
		case now.Unix() >= g.Deadline:
			why = "the wait is over"
		default:
			w := s.waits()
			w.mu.Lock()
			g.busy, g.reason = busy, reason
			w.mu.Unlock()
			continue
		}
		w := s.waits()
		w.mu.Lock()
		members := slices.Clone(g.Members)
		w.mu.Unlock()
		s.forgetIdleGroup(g.Key)
		log.Printf("api: starting the held backup of %s: %s", strings.Join(members, ", "), why) //nolint:gosec // G706: container names from Docker
		g.run(members)
		return
	}
}

// groupBusy names the first member with a wait whose app is still busy.
// waiting is false once no member waits for its app any more.
func (s *Service) groupBusy(g *idleGroup, now time.Time) (busy, reason string, waiting bool) {
	hours, err := s.store.IdleWaitHours()
	if err != nil {
		return g.busy, g.reason, true
	}
	targets, err := s.store.ListTargets()
	if err != nil {
		return g.busy, g.reason, true
	}
	w := s.waits()
	w.mu.Lock()
	members := slices.Clone(g.Members)
	w.mu.Unlock()
	for _, t := range targets {
		if !slices.Contains(members, t.ContainerName) || hours[t.ID] <= 0 {
			continue
		}
		waiting = true
		if idle, why := s.appIdle(t.ContainerName, now); !idle && busy == "" {
			busy, reason = t.ContainerName, why
		}
	}
	return busy, reason, waiting
}

// containerStacks maps each container to its compose project.
func (s *Service) containerStacks() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	infos, err := s.docker.List(ctx)
	if err != nil {
		log.Printf("api: idle wait: list containers: %v", err)
		return nil
	}
	out := map[string]string{}
	for _, c := range infos {
		if c.Stack != "" {
			out[c.Name] = c.Stack
		}
	}
	return out
}

// IdleWaits lists the container backups waiting for an idle app, the longest
// waiting first.
func (s *Service) IdleWaits() []IdleWait {
	w := s.waits()
	w.mu.Lock()
	var out []IdleWait
	for _, g := range w.groups {
		for _, name := range g.Members {
			out = append(out, IdleWait{
				Domain: "containers", Name: name, Stack: g.Stack, Busy: g.busy, Reason: g.reason,
				Since: g.Since, Deadline: g.Deadline,
			})
		}
	}
	w.mu.Unlock()
	if out == nil {
		out = []IdleWait{}
	}
	slices.SortFunc(out, func(a, b IdleWait) int {
		if a.Since != b.Since {
			return int(a.Since - b.Since)
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out
}

// appIdle asks the traffic watch whether a container's app is idle. A media
// server is idle while it does not stream, any other app once its CPU and
// traffic stayed low for the quiet time.
func (s *Service) appIdle(name string, now time.Time) (bool, string) {
	cfg, err := s.store.TrafficSettings()
	if err != nil {
		return true, ""
	}
	rule := traffic.IdleRule{
		CPUPct: float64(cfg.IdleCPUPct),
		NetBps: float64(cfg.IdleNetMbit) * 1e6 / 8,
		Quiet:  time.Duration(cfg.IdleQuietMin) * time.Minute,
	}
	servers := s.mediaServers(context.Background(), cfg, now)
	if slices.Contains(servers, name) {
		stream := streamRule(cfg, servers)
		rule.Stream = &stream
	}
	return s.trafficState().watch.Idle(name, rule, now)
}

// idleWaitContainers names the containers whose scheduled backup may wait for
// their app, so the traffic watch measures them ahead of the fire.
func (s *Service) idleWaitContainers() []string {
	hours, err := s.store.IdleWaitHours()
	if err != nil || len(hours) == 0 {
		return nil
	}
	targets, err := s.store.ListTargets()
	if err != nil {
		return nil
	}
	var names []string
	for _, t := range targets {
		if hours[t.ID] > 0 {
			names = append(names, t.ContainerName)
		}
	}
	return names
}

// holdEverythingContainers takes the containers of a scheduled "Backup
// Everything" pass whose app is busy out of it, the way the scheduler does for
// a domain run.
func (s *Service) holdEverythingContainers(ctx context.Context, targets []store.Target) []store.Target {
	if !scheduledPass(ctx) || s.heldRun == nil {
		return targets
	}
	included := make([]store.Target, 0, len(targets))
	for _, t := range targets {
		if t.IncludeInSchedule {
			included = append(included, t)
		}
	}
	held := s.HoldForIdle(included, "everything", s.heldRun)
	out := make([]store.Target, 0, len(targets))
	for _, t := range targets {
		if !slices.Contains(held, t.ContainerName) {
			out = append(out, t)
		}
	}
	return out
}
