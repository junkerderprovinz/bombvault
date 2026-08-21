// Package progress carries live backup/restore/replicate progress from the
// restic layer (which reports a percentage as it streams) up to an SSE
// endpoint that the SPA subscribes to. Three pieces:
//
//   - A context-carried Sink: restic.run pulls it from ctx and calls it with the
//     current percentage, so no method signatures need a progress argument.
//   - A context-carried CopySink: restic.Copy's counterpart to Sink, for
//     `restic copy`'s different (per-snapshot, not whole-run) progress shape.
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

// CopyProgress is one live update from a `restic copy` run (see restic.Copy):
// SnapshotIndex is the 1-based index of the snapshot currently being copied
// (restarting a fresh pack-copy count for each one — copy has no whole-run
// total across snapshots) and Percent is that ONE snapshot's own 0..100
// pack-copy completion, parsed straight from restic's real stdout. restic
// itself never reports how many snapshots a run will touch in total; a
// consumer that wants "snapshot k of N" phrasing supplies its own best-effort
// N (see api.progBeginCopySink / restic.PendingCopyIDs) — CopyProgress only
// carries what restic actually said.
type CopyProgress struct {
	SnapshotIndex int
	Percent       float64
}

// CopySink receives live restic-copy progress (see CopyProgress). A separate
// type from Sink (not a reuse of its plain float64 shape) because copy's
// progress genuinely has an extra dimension — which snapshot of the batch —
// that a single backup/restore run never does; the SAME ctx-carried-callback
// PATTERN is reused (WithCopySink/CopySinkFrom mirror WithSink/SinkFrom
// exactly), so restic.Copy needs no parallel plumbing to report progress
// without a percent argument threaded through every signature.
type CopySink func(CopyProgress)

// copySinkKey is the unexported context key for the CopySink.
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

// Event is one progress update for a target. Key identifies the target
// ("container:<name>", "vm:<name>", or "flash"); Phase is "backup", "restore",
// "replicate" or "maintenance"; Percent is 0..100; Active is false on the
// terminal event (finished/failed). StartedAt is the Unix-seconds timestamp
// (time.Now().Unix()) the operation began, repeated on every event for the
// same key — including the terminal one — so a client can render a live
// elapsed duration for the whole run. Zero/omitted for older call sites that
// predate this field; a client must treat 0 as "unknown", never as an actual
// epoch second.
//
// SnapshotIndex/SnapshotTotal are set only for off-site replication
// ("offsite:<domain>", Phase "replicate" — see api.copyToOffsiteTarget):
// issue #159 asked for a percentage on off-site upload progress, and — despite
// this feature's first cut concluding otherwise — restic copy DOES print real,
// parseable progress on its stdout (a plain-text line, not --json: copy has no
// JSON mode at all), it just needed RESTIC_PROGRESS_FPS wired up the same way
// backup/restore already get it (see restic.Copy's doc comment for the whole
// story). What restic copy genuinely does NOT report is a whole-run total
// across multiple snapshots — each snapshot's pack-copy percentage restarts at
// 0 — so Percent here is scoped to the CURRENT snapshot (SnapshotIndex, 1-based)
// of an estimated SnapshotTotal (a best-effort candidate count the caller
// computes itself; restic never reports one — see restic.PendingCopyIDs).
// Both are 0/omitted whenever no live per-snapshot signal is available yet
// (e.g. the initial tree-walk before the first pack is copied) or for every
// other Phase, which never set them.
type Event struct {
	Key           string  `json:"key"`
	Phase         string  `json:"phase"`
	Percent       float64 `json:"percent"`
	Active        bool    `json:"active"`
	StartedAt     int64   `json:"startedAt,omitempty"`
	SnapshotIndex int     `json:"snapshotIndex,omitempty"`
	SnapshotTotal int     `json:"snapshotTotal,omitempty"`
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
