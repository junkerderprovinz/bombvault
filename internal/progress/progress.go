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

// Event is one progress update for a target. Key is "container:<name>",
// "vm:<name>", "flash" or "offsite:<domain>"; Phase is "backup", "restore",
// "replicate" or "maintenance". Active is false on the final event. StartedAt
// is the Unix time the run began, repeated on every event including the final
// one so a client can show the elapsed time; 0 means unknown.
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
	Stage         string `json:"stage,omitempty"`
	Bytes         int64  `json:"bytes,omitempty"`
	StartedAt     int64  `json:"startedAt,omitempty"`
	SnapshotIndex int    `json:"snapshotIndex,omitempty"`
	SnapshotTotal int    `json:"snapshotTotal,omitempty"`
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
// subscriber's buffer is full, Publish drops the update (the next one, or the
// terminal Active:false event, catches it up).
func (s *Store) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 32)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	s.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			s.mu.Lock()
			delete(s.subs, ch)
			close(ch)
			s.mu.Unlock()
		})
	}
	return ch, cancel
}

// Publish fans an Event out to all subscribers and updates the per-key latest
// state (cleared when the Event is terminal, Active=false).
func (s *Store) Publish(e Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.Active {
		s.last[e.Key] = e
	} else {
		delete(s.last, e.Key)
	}
	for ch := range s.subs {
		select {
		case ch <- e:
		default:
			// Drop the update rather than block the backup on a slow
			// subscriber.
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
