// Package logring keeps the most recent log output in memory, bounded, so a
// support bundle can carry it.
//
// BombVault logs to stdout and nowhere else: `docker logs` is the only place
// the output exists. That is fine while someone can reach the host, and useless
// in the case a support bundle is for — a user who can open the web UI, cannot
// or will not run docker commands, and is asked to attach "the log".
//
// A ring rather than a file, deliberately. A log file would need a path, a
// rotation policy, disk budget and a decision about which volume it lives on,
// for output that is already being written to stdout. The ring costs a fixed
// number of bytes and adds no new place for data to pile up.
//
// The limit it does have, stated plainly because it decides when this helps:
// the ring holds THIS PROCESS's output. A container that crashed and restarted
// starts an empty one, so the bundle from after a restart shows the run since
// the restart, not the one that failed. For that case `docker logs` remains the
// answer, and the bundle's manifest says so.
package logring

import (
	"io"
	"sync"
)

// Ring is a fixed-size, thread-safe tail buffer of recent output. The zero
// value is not usable; call New.
type Ring struct {
	mu   sync.Mutex
	buf  []byte
	size int
	// full marks that buf has wrapped at least once, so the read order is
	// [pos:] followed by [:pos] rather than just [:pos].
	pos  int
	full bool
}

// New returns a ring holding at most size bytes. A size of zero or less yields
// a ring that records nothing but is still safe to write to and read from, so a
// caller never has to guard the disabled case.
func New(size int) *Ring {
	if size < 0 {
		size = 0
	}
	return &Ring{buf: make([]byte, size), size: size}
}

// Write records p, dropping the oldest bytes when the ring is full. It never
// fails and never blocks on anything but its own mutex: this sits on the path
// of every log line in the process, so it must not be a place output can get
// stuck or lost through an error nobody checks.
func (r *Ring) Write(p []byte) (int, error) {
	n := len(p)
	if r.size == 0 {
		return n, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	// A write larger than the ring can only leave its TAIL, which is the half
	// worth keeping: the end of a stack trace names what finally failed.
	if len(p) >= r.size {
		copy(r.buf, p[len(p)-r.size:])
		r.pos = 0
		r.full = true
		return n, nil
	}

	written := copy(r.buf[r.pos:], p)
	if written < len(p) {
		copy(r.buf, p[written:])
		r.full = true
	}
	r.pos = (r.pos + len(p)) % r.size
	if r.pos == 0 {
		r.full = true
	}
	return n, nil
}

// String returns the recorded tail, oldest byte first.
func (r *Ring) String() string {
	if r.size == 0 {
		return ""
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.full {
		return string(r.buf[:r.pos])
	}
	out := make([]byte, 0, r.size)
	out = append(out, r.buf[r.pos:]...)
	out = append(out, r.buf[:r.pos]...)
	return string(out)
}

// Tee returns a writer that records into the ring AND forwards to w.
//
// A tee, never a replacement: stdout is what `docker logs` shows, and taking
// that away to gain an in-memory copy would trade the log everyone already
// knows how to reach for one only this app can read.
func (r *Ring) Tee(w io.Writer) io.Writer {
	return io.MultiWriter(w, r)
}

// defaultSize is what the process-wide ring holds: enough to carry the startup
// banner, the host-integration probe results and a normal run's worth of
// activity, small enough that it is never worth thinking about. 256 KiB of text
// is on the order of a few thousand log lines.
const defaultSize = 256 << 10

// Default is the process-wide ring. main tees the standard logger through it at
// startup and the diagnostics bundle reads it.
//
// A package-level singleton rather than an injected dependency, because the
// thing it mirrors is one too: `log` is process-global, and threading a second
// handle to the same stream through every constructor would buy nothing but
// ceremony. Tests that need isolation use New instead.
var Default = New(defaultSize)
