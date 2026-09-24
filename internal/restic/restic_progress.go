package restic

import (
	"context"
	"encoding/json"
)

// Progress is one restic `--json` status line, read as the set of counters that
// say whether the run is alive.
//
// statusPercent (restic.go) reads the same lines for the progress BAR and wants
// exactly one number. This is the other question about the same line: not "how
// far along", but "did anything happen since last time". A percentage cannot
// answer that, because percent_done sits at 0 for the whole scan phase.
type Progress struct {
	// Percent is 0..100, clamped, for the display.
	Percent float64
	// TotalFiles and TotalBytes are what restic currently believes the job is.
	// They GROW during the scan phase, which is the only sign of life there is
	// before the first pack is written.
	TotalFiles uint64
	TotalBytes uint64
	// FilesDone and BytesDone are the work actually completed.
	FilesDone uint64
	BytesDone uint64
	// SecondsElapsed is restic's own clock, and no sign of life: it
	// advances whether or not anything is happening, so a guard that accepted it
	// could never fire. Kept because it is useful in a log line.
	SecondsElapsed uint64
}

// MovedSince reports whether anything advanced between two status lines.
//
// Any counter changing counts, and that matters. The failure this
// prevents is killing a healthy backup during the SCAN phase: before restic
// writes its first pack it walks the tree, and on a large appdata tree
// bytes_done stays at 0 for minutes while total_bytes climbs. A guard watching
// bytes_done alone would call that a stall and cancel exactly the run that
// needs the most time.
//
// A total that moves DOWN counts too: restic revises its estimate when the scan
// over-guessed, which is the scanner working, not a wedge.
func (p Progress) MovedSince(prev Progress) bool {
	return p.BytesDone != prev.BytesDone ||
		p.FilesDone != prev.FilesDone ||
		p.TotalBytes != prev.TotalBytes ||
		p.TotalFiles != prev.TotalFiles
}

// ParseProgress reads a restic `--json` status line. It reports ok=false for
// every other line (summary, error, non-JSON), so a caller can feed it the raw
// stream.
func ParseProgress(line []byte) (Progress, bool) {
	var s struct {
		MessageType    string  `json:"message_type"`
		PercentDone    float64 `json:"percent_done"`
		TotalFiles     uint64  `json:"total_files"`
		FilesDone      uint64  `json:"files_done"`
		TotalBytes     uint64  `json:"total_bytes"`
		BytesDone      uint64  `json:"bytes_done"`
		SecondsElapsed uint64  `json:"seconds_elapsed"`
	}
	if json.Unmarshal(line, &s) != nil || s.MessageType != "status" {
		return Progress{}, false
	}
	pct := s.PercentDone * 100
	if pct < 0 {
		pct = 0
	} else if pct > 100 {
		// restic briefly reports over 100% when its scan under-estimated the
		// total; the bar must not run off the end.
		pct = 100
	}
	return Progress{
		Percent:        pct,
		TotalFiles:     s.TotalFiles,
		FilesDone:      s.FilesDone,
		TotalBytes:     s.TotalBytes,
		BytesDone:      s.BytesDone,
		SecondsElapsed: s.SecondsElapsed,
	}, true
}

// ProgressWatcher receives every status line's full counter set.
//
// Separate from progress.Sink, which carries a single percentage to the UI and
// is wired into dozens of call sites. The stall guard needs the counters, not
// the percentage: percent_done sits at 0 for the entire scan phase, so a
// watcher built on it would see a healthy scan as a dead run. Adding a second,
// optional channel leaves the existing one untouched.
type ProgressWatcher func(Progress)

type watcherKey struct{}

// WithWatcher returns a context carrying w, so a restic call made with it
// reports its counters. A nil watcher returns ctx unchanged, which is what
// keeps every existing call site behaving exactly as before.
//
// Opt-in per call, because a watcher must reach backup runs alone. A
// restore is not cancellable on purpose (an interrupted restore has already
// removed the container and half-written its appdata), and maintenance
// commands emit no byte counters at all, so silence there means nothing.
func WithWatcher(ctx context.Context, w ProgressWatcher) context.Context {
	if w == nil {
		return ctx
	}
	return context.WithValue(ctx, watcherKey{}, w)
}

// WithAddedWatcher chains w after the watcher already on ctx instead of
// replacing it. A database dump reports to the stall guard and to the byte
// publisher on the same restic call. A nil w returns ctx unchanged.
func WithAddedWatcher(ctx context.Context, w ProgressWatcher) context.Context {
	prev := WatcherFrom(ctx)
	if w == nil || prev == nil {
		return WithWatcher(ctx, w)
	}
	return WithWatcher(ctx, func(p Progress) {
		prev(p)
		w(p)
	})
}

// WatcherFrom returns the watcher on ctx, or nil.
func WatcherFrom(ctx context.Context) ProgressWatcher {
	w, _ := ctx.Value(watcherKey{}).(ProgressWatcher)
	return w
}
