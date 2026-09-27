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

// IdleWait is a scheduled backup held back until its app is idle.
type IdleWait struct {
	Domain string `json:"domain"`
	Name   string `json:"name"`
	// Reason is why the app counts as busy at the last look: one of the
	// traffic.Busy constants.
	Reason   string `json:"reason"`
	Since    int64  `json:"since"`
	Deadline int64  `json:"deadline"`
}

type idleWaits struct {
	mu    sync.Mutex
	items map[string]*IdleWait
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

// SetHeldContainerRun names what backs up a container a scheduled "Backup
// Everything" pass held back for its app, once the wait is over.
func (s *Service) SetHeldContainerRun(fn func(name string)) {
	s.heldRun = fn
}

// HoldForIdle takes a scheduled container whose app is busy out of its run and
// backs it up with run once the app is idle, or anyway when its wait is over.
// It reports whether it took the container. A second fire for a container
// that is already waiting is absorbed by that wait.
func (s *Service) HoldForIdle(t store.Target, run func()) bool {
	if _, ok := s.docker.(statsReader); !ok {
		return false
	}
	hours, err := s.store.IdleWaitHours()
	if err != nil {
		log.Printf("api: idle wait: %v", err)
		return false
	}
	h := hours[t.ID]
	if h <= 0 {
		return false
	}
	w := s.waits()
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, waiting := w.items[t.ContainerName]; waiting {
		return true
	}
	now := time.Now()
	idle, reason := s.appIdle(t.ContainerName, now)
	if idle {
		return false
	}
	item := &IdleWait{
		Domain: "containers", Name: t.ContainerName, Reason: reason,
		Since: now.Unix(), Deadline: now.Add(time.Duration(h) * time.Hour).Unix(),
	}
	w.items[t.ContainerName] = item
	log.Printf("api: scheduled backup of %s waits for its app to be idle (%s), at the latest until %s", //nolint:gosec // G706: a container name from Docker
		t.ContainerName, reason, time.Unix(item.Deadline, 0).Format(time.RFC3339))
	go s.waitForIdle(t.ID, item, run)
	return true
}

func (s *Service) waitForIdle(targetID string, item *IdleWait, run func()) {
	t := time.NewTicker(idleCheck)
	defer t.Stop()
	for now := range t.C {
		why := ""
		switch idle, reason := s.appIdle(item.Name, now); {
		case idle:
			why = "the app is idle"
		case now.Unix() >= item.Deadline:
			why = "the wait is over"
		case !s.stillWaits(targetID):
			why = "the wait was switched off"
		default:
			s.waits().setReason(item.Name, reason)
			continue
		}
		s.waits().drop(item.Name)
		log.Printf("api: starting the held backup of %s: %s", item.Name, why) //nolint:gosec // G706: a container name from Docker
		run()
		return
	}
}

func (s *Service) stillWaits(targetID string) bool {
	hours, err := s.store.IdleWaitHours()
	return err != nil || hours[targetID] > 0
}

func (s *Service) waits() *idleWaits {
	s.waitsOnce.Do(func() { s.idleWaits = &idleWaits{items: map[string]*IdleWait{}} })
	return s.idleWaits
}

func (w *idleWaits) setReason(name, reason string) {
	w.mu.Lock()
	if it, ok := w.items[name]; ok {
		it.Reason = reason
	}
	w.mu.Unlock()
}

func (w *idleWaits) drop(name string) {
	w.mu.Lock()
	delete(w.items, name)
	w.mu.Unlock()
}

// IdleWaits lists the backups waiting for an idle app, the longest waiting
// first.
func (s *Service) IdleWaits() []IdleWait {
	w := s.waits()
	w.mu.Lock()
	out := make([]IdleWait, 0, len(w.items))
	for _, it := range w.items {
		out = append(out, *it)
	}
	w.mu.Unlock()
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
	run := s.heldRun
	return slices.DeleteFunc(targets, func(t store.Target) bool {
		name := t.ContainerName
		return t.IncludeInSchedule && s.HoldForIdle(t, func() { run(name) })
	})
}
