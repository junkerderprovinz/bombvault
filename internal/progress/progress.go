// Package progress carries live backup/restore progress from the restic layer
// (which reports a percentage as it streams) up to an SSE endpoint that the SPA
// subscribes to. Two pieces:
//
//   - A context-carried Sink: restic.run pulls it from ctx and calls it with the
//     current percentage, so no method signatures need a progress argument.
//   - A Store: a tiny in-process pub/sub the API service publishes to and the
//     SSE handler subscribes to, keyed per backup target.
package progress

import (
	"context"
	"sync"
)

// Sink receives a 0..100 completion percentage for the in-flight restic command.
type Sink func(percent float64)

// ctxKey is the unexported context key for the Sink.
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

// Event is one progress update for a target. Key identifies the target
// ("container:<name>", "vm:<name>", or "flash"); Phase is "backup" or "restore";
// Percent is 0..100; Active is false on the terminal event (finished/failed).
type Event struct {
	Key     string  `json:"key"`
	Phase   string  `json:"phase"`
	Percent float64 `json:"percent"`
	Active  bool    `json:"active"`
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
			// Slow subscriber — drop this frequent percent update rather than
			// block the backup goroutine.
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
