// Package progress carries live backup, restore and replication progress from
// the restic layer to the SSE endpoint the web UI subscribes to. A Sink or
// CopySink travels in the context, so restic calls need no progress argument,
// and a Store fans the events out to subscribers.
package progress

import (
	"context"
	"sync"
)

// Sink receives a 0..100 completion percentage for the in-flight restic command.
type Sink func(percent float64)

type ctxKey struct{}

// WithSink returns a context carrying fn so a downstream restic call can report
// progress. A nil fn returns ctx unchanged.
func WithSink(ctx context.Context, fn Sink) context.Context {
	if fn == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, fn)
}

// SinkFrom returns the Sink carried by ctx, or nil when none is set.
func SinkFrom(ctx context.Context) Sink {
	if fn, ok := ctx.Value(ctxKey{}).(Sink); ok {
		return fn
	}
	return nil
}

// CopyProgress is one update from `restic copy`. SnapshotIndex is the 1-based
// snapshot being copied and Percent that snapshot's own completion. restic
// reports no total across snapshots, so a caller that wants "k of N" has to
// estimate N itself (restic.PendingCopyIDs).
type CopyProgress struct {
	SnapshotIndex int
	Percent       float64
}

// CopySink receives restic copy progress. It is separate from Sink because
// copy progress also says which snapshot it is on.
type CopySink func(CopyProgress)

type copySinkKey struct{}

// WithCopySink returns a context carrying fn so restic.Copy can report live
// per-snapshot progress. A nil fn returns ctx unchanged.
func WithCopySink(ctx context.Context, fn CopySink) context.Context {
	if fn == nil {
		return ctx
	}
	return context.WithValue(ctx, copySinkKey{}, fn)
}

// CopySinkFrom returns the CopySink carried by ctx, or nil when none is set.
func CopySinkFrom(ctx context.Context) CopySink {
	if fn, ok := ctx.Value(copySinkKey{}).(CopySink); ok {
		return fn
	}
	return nil
}

// CountProgress is one counter line of a restic command that reports no
// percentage of its own, such as check or prune: Done of Total units, where
// Unit is "packs", "snapshots", "indexes", "files" or "items".
type CountProgress struct {
	Done  int64
	Total int64
	Unit  string
}

// CountSink receives restic's counter lines.
type CountSink func(CountProgress)

type countSinkKey struct{}

// WithCountSink returns a context carrying fn so restic check and prune report
// their counters. A nil fn returns ctx unchanged.
func WithCountSink(ctx context.Context, fn CountSink) context.Context {
	if fn == nil {
		return ctx
	}
	return context.WithValue(ctx, countSinkKey{}, fn)
}

// CountSinkFrom returns the CountSink carried by ctx, or nil when none is set.
func CountSinkFrom(ctx context.Context) CountSink {
	if fn, ok := ctx.Value(countSinkKey{}).(CountSink); ok {
		return fn
	}
	return nil
}

// The steps of a backup that stops what it backs up, in the order it takes them.
const (
	StepStop  = "stop"
	StepRead  = "read"
	StepStart = "start"
)

// EverythingKey is the key of a "Backup Everything" pass. Its events count the
// items of the pass in Done and Total.
const EverythingKey = "everything"

// Event is one progress update for a target. Key is "container:<name>",
// "vm:<name>", "flash", "offsite:<domain>" or EverythingKey; Phase is "backup",
// "restore", "replicate" or "maintenance". Active is false on the final event.
// StartedAt is the Unix time the run began, repeated on every event including
// the final one so a client can show the elapsed time; 0 means unknown.
//
// SnapshotIndex and SnapshotTotal are set only for off-site replication.
// restic copy restarts its percentage for every snapshot, so Percent then
// covers snapshot SnapshotIndex of SnapshotTotal, an estimate the caller
// computes because restic reports no total. Both stay 0 until the first pack
// is copied.
type Event struct {
	Key     string  `json:"key"`
	Phase   string  `json:"phase"`
	Percent float64 `json:"percent"`
	Active  bool    `json:"active"`
	// Stage names a step inside the phase that reports bytes instead of a
	// percentage, such as a database dump inside a container backup. Empty for
	// the phase itself.
	Stage string `json:"stage,omitempty"`
	// Step is where a backup that stops its container or VM stands: StepStop,
	// StepRead or StepStart. A backup that stops nothing has none. The copy that
	// follows runs under its own "offsite:<domain>" key.
	Step          string `json:"step,omitempty"`
	Bytes         int64  `json:"bytes,omitempty"`
	StartedAt     int64  `json:"startedAt,omitempty"`
	SnapshotIndex int    `json:"snapshotIndex,omitempty"`
	SnapshotTotal int    `json:"snapshotTotal,omitempty"`
	// Committed marks a backup whose restore point is written and that only
	// starts its containers again, so it can no longer be cancelled.
	Committed bool `json:"committed,omitempty"`
	// Done, Total and Unit are set for a check or prune while restic counts
	// through a step, and for a "Backup Everything" pass, which counts its
	// items. Remaining is the estimated seconds left in a step of a check or
	// prune, 0 while there is too little to go by.
	Done      int64  `json:"done,omitempty"`
	Total     int64  `json:"total,omitempty"`
	Unit      string `json:"unit,omitempty"`
	Remaining int64  `json:"remaining,omitempty"`
	// Throttle marks an off-site copy slowed for a media server's stream:
	// "now" while it runs at the streaming limit, "next" while a stream runs
	// that the copy can only meet from its next step.
	Throttle string `json:"throttle,omitempty"`
	// Failed counts the items of a batch that did not work, such as the
	// archives of an import, on the event that ends it.
	Failed int `json:"failed,omitempty"`
}

// Store is an in-process fan-out of progress Events. It keeps the latest active
// Event per key so a newly-connected subscriber can render an in-flight bar
// immediately (Snapshot).
type Store struct {
	mu   sync.Mutex
	subs map[chan Event]struct{}
	last map[string]Event
}

// NewStore returns an empty Store.
func NewStore() *Store {
	return &Store{
		subs: make(map[chan Event]struct{}),
		last: make(map[string]Event),
	}
}

// Subscribe registers a new subscriber and returns its event channel plus a
// cancel func that unregisters and closes it. The channel is buffered; if a slow
// subscriber's buffer is full, Publish drops an update, which the next one
// catches up. One that misses the event ending a run is cut off and its
// channel closed, so its reader starts again from the Snapshot instead of
// keeping a bar that never ends.
func (s *Store) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 32)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	s.mu.Unlock()

	cancel := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.drop(ch)
	}
	return ch, cancel
}

func (s *Store) drop(ch chan Event) {
	if _, ok := s.subs[ch]; ok {
		delete(s.subs, ch)
		close(ch)
	}
}

// Publish fans an Event out to all subscribers and updates the per-key latest
// state (cleared when the Event is terminal, Active=false). An active event
// that names no step takes the one its key is in, so a percentage update does
// not have to know it.
func (s *Store) Publish(e Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.publish(e)
}

// SetStep moves the run under key to step and publishes its latest event with
// it. A stage belongs to the step it ran in and does not carry over.
func (s *Store) SetStep(key, step string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.last[key]
	if !ok {
		return
	}
	e.Step, e.Stage, e.Bytes = step, "", 0
	s.publish(e)
}

func (s *Store) publish(e Event) {
	if e.Active {
		if e.Step == "" {
			e.Step = s.last[e.Key].Step
		}
		s.last[e.Key] = e
	} else {
		delete(s.last, e.Key)
	}
	for ch := range s.subs {
		select {
		case ch <- e:
		default:
			// A backup never waits for a slow subscriber.
			if !e.Active {
				s.drop(ch)
			}
		}
	}
}

// Snapshot returns the current active Events (one per in-flight target) so a new
// subscriber can render bars that are already running.
func (s *Store) Snapshot() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Event, 0, len(s.last))
	for _, e := range s.last {
		out = append(out, e)
	}
	return out
}
