package traffic

import (
	"slices"
	"sync"
	"time"
)

// Sample is one reading of a container's cumulative counters.
type Sample struct {
	At       time.Time
	Running  bool
	CPUNanos uint64
	RxBytes  uint64
	TxBytes  uint64
	// HasNet is false for a container on the host network: Docker keeps no
	// counters of its own for it.
	HasNet bool
}

// Rate is the activity between two samples of one container.
type Rate struct {
	At time.Time
	// CPUPct is a percentage of one core, as docker stats shows it.
	CPUPct float64
	RxBps  float64
	TxBps  float64
	HasNet bool
}

// Watch keeps the recent rates of the containers it is fed.
type Watch struct {
	keep time.Duration
	// gap is the longest silence between two samples that still counts as one
	// unbroken series.
	gap time.Duration

	mu     sync.Mutex
	last   map[string]Sample
	since  map[string]time.Time
	rates  map[string][]Rate
	failed map[string]time.Time
}

// NewWatch keeps rates for keep. A pause in sampling longer than gap starts
// the measurement of that container afresh.
func NewWatch(keep, gap time.Duration) *Watch {
	return &Watch{
		keep:   keep,
		gap:    gap,
		last:   map[string]Sample{},
		since:  map[string]time.Time{},
		rates:  map[string][]Rate{},
		failed: map[string]time.Time{},
	}
}

// Record adds a sample. A counter that went backwards means the container
// restarted, so that pair gives no rate.
func (w *Watch) Record(name string, s Sample) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.failed, name)
	prev, ok := w.last[name]
	w.last[name] = s
	if !ok || s.At.Sub(prev.At) > w.gap || !s.Running || !prev.Running {
		w.since[name] = s.At
		w.rates[name] = nil
		return
	}
	secs := s.At.Sub(prev.At).Seconds()
	if secs <= 0 || s.CPUNanos < prev.CPUNanos || s.RxBytes < prev.RxBytes || s.TxBytes < prev.TxBytes {
		return
	}
	r := Rate{
		At:     s.At,
		CPUPct: float64(s.CPUNanos-prev.CPUNanos) / 1e9 / secs * 100,
		RxBps:  float64(s.RxBytes-prev.RxBytes) / secs,
		TxBps:  float64(s.TxBytes-prev.TxBytes) / secs,
		HasNet: s.HasNet && prev.HasNet,
	}
	cut := s.At.Add(-w.keep)
	rates := slices.DeleteFunc(w.rates[name], func(x Rate) bool { return x.At.Before(cut) })
	w.rates[name] = append(rates, r)
}

// RecordFailure notes that a container could not be read.
func (w *Watch) RecordFailure(name string, at time.Time) {
	w.mu.Lock()
	w.failed[name] = at
	w.mu.Unlock()
}

// Retain drops every container not in names.
func (w *Watch) Retain(names []string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	drop := func(n string) {
		if !slices.Contains(names, n) {
			delete(w.last, n)
			delete(w.since, n)
			delete(w.rates, n)
			delete(w.failed, n)
		}
	}
	for n := range w.last {
		drop(n)
	}
	for n := range w.failed {
		drop(n)
	}
}

// StreamRule says what counts as a stream: a media server sending above
// Threshold bytes per second. The stream counts as over only after Hold has
// passed without that.
type StreamRule struct {
	Servers   []string
	Threshold float64
	Hold      time.Duration
}

// Streaming names the media server that sent above the threshold most
// recently within the hold time.
func (w *Watch) Streaming(rule StreamRule, now time.Time) (string, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.streamingLocked(rule, now)
}

func (w *Watch) streamingLocked(rule StreamRule, now time.Time) (string, bool) {
	var who string
	var at time.Time
	for _, name := range rule.Servers {
		for _, r := range w.rates[name] {
			if r.HasNet && r.TxBps >= rule.Threshold && r.At.After(at) {
				who, at = name, r.At
			}
		}
	}
	if who == "" || now.Sub(at) > rule.Hold {
		return "", false
	}
	return who, true
}

// IdleRule says when a container counts as idle. A media server is idle once
// it was measured for the hold of Stream without streaming; any other
// container, and a media server on the host network, when CPU and network
// stayed below the limits for Quiet.
type IdleRule struct {
	CPUPct float64
	NetBps float64
	Quiet  time.Duration
	Stream *StreamRule
}

// Reasons Idle gives for a container that is not idle.
const (
	BusyStreaming = "streaming"
	BusyCPU       = "cpu"
	BusyNetwork   = "network"
	BusyMeasuring = "measuring"
)

// Idle reports whether name is idle under rule, or why not. A container that
// is stopped or could not be read counts as idle, so a broken measurement
// never holds a backup back.
func (w *Watch) Idle(name string, rule IdleRule, now time.Time) (bool, string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	last, ok := w.last[name]
	if f, bad := w.failed[name]; bad && (!ok || f.After(last.At)) {
		return true, ""
	}
	if !ok {
		return false, BusyMeasuring
	}
	if !last.Running {
		return true, ""
	}
	// Without network counters a media server's streams cannot be seen, so
	// it goes by the rule for any other container.
	if rule.Stream != nil && last.HasNet {
		one := *rule.Stream
		one.Servers = []string{name}
		if _, on := w.streamingLocked(one, now); on {
			return false, BusyStreaming
		}
		if now.Sub(w.since[name]) < one.Hold {
			return false, BusyMeasuring
		}
		return true, ""
	}
	if now.Sub(last.At) > w.gap || now.Sub(w.since[name]) < rule.Quiet {
		return false, BusyMeasuring
	}
	from := now.Add(-rule.Quiet)
	reason := ""
	for _, r := range w.rates[name] {
		if r.At.Before(from) {
			continue
		}
		switch {
		case r.CPUPct >= rule.CPUPct:
			reason = BusyCPU
		case r.HasNet && r.RxBps+r.TxBps >= rule.NetBps:
			reason = BusyNetwork
		}
	}
	return reason == "", reason
}
