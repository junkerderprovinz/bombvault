package api

import (
	"context"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/schedule"
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
	// every is how often a waiting backup looks at its app again.
	every time.Duration
	// wg counts the running waits, so a test can wait for them to end.
	wg sync.WaitGroup
}

// idleDockerTimeout bounds what one look at the apps may spend on Docker.
const idleDockerTimeout = 10 * time.Second

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
	s.waitsOnce.Do(func() { s.idleWaits = &idleWaits{groups: map[string]*idleGroup{}, every: trafficPoll} })
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
	waiting := len(w.groups) > 0
	w.mu.Unlock()
	if len(hours) == 0 && !waiting {
		return nil
	}
	// Docker is asked before the waits are locked, so a slow daemon holds up
	// this run only and not everybody who reads the waits.
	ctx, cancel := context.WithTimeout(context.Background(), idleDockerTimeout)
	defer cancel()
	now := time.Now()
	stacks := s.containerStacks(ctx)
	type look struct {
		idle   bool
		reason string
	}
	looks := map[string]look{}
	for _, t := range targets {
		if hours[t.ID] > 0 {
			idle, reason := s.appIdle(ctx, t.ContainerName, now)
			looks[t.ContainerName] = look{idle, reason}
		}
	}

	w.mu.Lock()
	defer w.mu.Unlock()
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
		if h <= 0 || looks[name].idle {
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
			g.Deadline, g.busy, g.reason = deadline, name, looks[name].reason
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
		s.startWait(g)
	}
	return held
}

// waitDropped says why a held backup has no run to go back to any more, or ""
// when it still has one. A wait from a container's own cadence or from a
// Backup Everything pass does not depend on the domain schedule.
func waitDropped(settings store.Settings, trigger string) string {
	if !settings.ContainersEnabled {
		return "the containers were switched off"
	}
	if trigger == "domain" {
		if c, err := schedule.ParseCadence(settings.ContainersSchedule); err == nil && !c.Enabled {
			return "the containers schedule was switched off"
		}
	}
	return ""
}

// ResumeIdleWaits picks up the waits a restart interrupted. A wait whose
// deadline passed meanwhile backs up at once, a container that was removed
// or taken off the schedule is dropped from its wait, and a wait whose run
// was switched off is dropped as a whole.
func (s *Service) ResumeIdleWaits() {
	groups, err := s.store.ListIdleWaitGroups()
	if err != nil {
		log.Printf("api: idle wait: resume: %v", err)
		return
	}
	if len(groups) == 0 || s.heldRun == nil {
		return
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		log.Printf("api: idle wait: resume: %v", err)
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
		if why := waitDropped(settings, g.Trigger); why != "" {
			s.forgetIdleGroup(g.Key)
			log.Printf("api: dropped the held backup of %s: %s", strings.Join(g.Members, ", "), why) //nolint:gosec // G706: container names from Docker
			continue
		}
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
		s.startWait(ig)
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

// endWait takes a wait off the list and out of the store in one step, so a
// container that comes along meanwhile starts a wait of its own instead of
// joining one nobody runs any more. It reports false when the wait is no
// longer listed.
func (s *Service) endWait(g *idleGroup) ([]string, bool) {
	w := s.waits()
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.groups[g.Key] != g {
		return nil, false
	}
	delete(w.groups, g.Key)
	if err := s.store.DeleteIdleWaitGroup(g.Key); err != nil {
		log.Printf("api: idle wait: forget %s: %v", g.Key, err)
	}
	return slices.Clone(g.Members), true
}

// startWait looks at a waiting group's apps until the wait ends or the
// service stops. A stop leaves the stored wait for the next start.
func (s *Service) startWait(g *idleGroup) {
	w := s.waits()
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		s.waitForIdle(s.StopContext(), g, w.every)
	}()
}

func (s *Service) waitForIdle(ctx context.Context, g *idleGroup, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		var now time.Time
		select {
		case <-ctx.Done():
			return
		case now = <-t.C:
		}
		if settings, err := s.store.GetSettings(); err == nil {
			if why := waitDropped(settings, g.Trigger); why != "" {
				if members, ok := s.endWait(g); ok {
					log.Printf("api: dropped the held backup of %s: %s", strings.Join(members, ", "), why) //nolint:gosec // G706: container names from Docker
				}
				return
			}
		}
		lctx, cancel := context.WithTimeout(ctx, idleDockerTimeout)
		busy, reason, limit, waiting := s.groupBusy(lctx, g, now)
		cancel()
		s.shortenWait(g, limit)
		why := ""
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
		members, ok := s.endWait(g)
		if !ok {
			return
		}
		log.Printf("api: starting the held backup of %s: %s", strings.Join(members, ", "), why) //nolint:gosec // G706: container names from Docker
		g.run(members)
		return
	}
}

// shortenWait brings a wait's deadline forward to limit, the end the busy
// members' waits allow as they are set now, so lowering the hours takes
// effect on a running wait. It never moves the deadline back.
func (s *Service) shortenWait(g *idleGroup, limit int64) {
	if limit == 0 || limit >= g.Deadline {
		return
	}
	w := s.waits()
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.groups[g.Key] != g {
		return
	}
	g.Deadline = limit
	if err := s.store.SaveIdleWaitGroup(g.IdleWaitGroup); err != nil {
		log.Printf("api: idle wait: remember %s: %v", g.Key, err)
	}
}

// groupBusy names the first member with a wait whose app is still busy, and
// the earliest end the waits of the busy members allow. waiting is false once
// no member waits for its app any more.
func (s *Service) groupBusy(ctx context.Context, g *idleGroup, now time.Time) (busy, reason string, limit int64, waiting bool) {
	hours, err := s.store.IdleWaitHours()
	if err != nil {
		return g.busy, g.reason, 0, true
	}
	targets, err := s.store.ListTargets()
	if err != nil {
		return g.busy, g.reason, 0, true
	}
	w := s.waits()
	w.mu.Lock()
	members := slices.Clone(g.Members)
	w.mu.Unlock()
	for _, t := range targets {
		h := hours[t.ID]
		if !slices.Contains(members, t.ContainerName) || h <= 0 {
			continue
		}
		waiting = true
		idle, why := s.appIdle(ctx, t.ContainerName, now)
		if idle {
			continue
		}
		if busy == "" {
			busy, reason = t.ContainerName, why
		}
		if end := g.Since + int64(h)*3600; limit == 0 || end < limit {
			limit = end
		}
	}
	return busy, reason, limit, waiting
}

// containerStacks maps each container to its compose project.
func (s *Service) containerStacks(ctx context.Context) map[string]string {
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
func (s *Service) appIdle(ctx context.Context, name string, now time.Time) (bool, string) {
	cfg, err := s.store.TrafficSettings()
	if err != nil {
		return true, ""
	}
	rule := traffic.IdleRule{
		CPUPct: float64(cfg.IdleCPUPct),
		NetBps: float64(cfg.IdleNetMbit) * 1e6 / 8,
		Quiet:  time.Duration(cfg.IdleQuietMin) * time.Minute,
	}
	servers := s.mediaServers(ctx, cfg, now)
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
