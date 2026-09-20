package dbdump

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"time"
)

// Execer runs a command inside a container; the dump helper adapts a Docker
// client to it.
type Execer interface {
	// ExecStream copies the command's stdout and stderr out unbounded and
	// returns the real exit code of the process, read once the daemon reports
	// it is no longer running.
	ExecStream(ctx context.Context, container string, cmd []string, stdout, stderr io.Writer) (int, error)
	// ExecQuick runs a short command whose output does not matter.
	ExecQuick(ctx context.Context, container string, cmd []string) error
}

// ErrNotRunning is what an Execer wraps when the container is paused,
// restarting or stopped, which the daemon answers with a 409.
var ErrNotRunning = errors.New("container is paused, restarting or stopped")

// How much of each stream Stream holds on to: enough of stdout to find the
// completion marker at its end, enough of stderr to explain a failure.
const (
	markerTailBytes = 4 << 10
	detailTailBytes = 2 << 10
)

// StreamOptions describes one dump.
type StreamOptions struct {
	Container string
	Engine    Engine
	// Max is the time limit the dump tool gets inside the container.
	Max time.Duration
	// State is where Stream reports the progress of its stdout writer. A
	// caller with a write watchdog passes one in.
	State *StreamState
}

// StreamState carries what a watcher may read while a dump is running.
type StreamState struct {
	mu        sync.Mutex
	lastWrite time.Time
}

// LastWrite reports when the dump last completed a write to stdout. A dump
// whose writer blocks leaves this standing still while the process is healthy.
func (s *StreamState) LastWrite() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastWrite
}

func (s *StreamState) wrote() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastWrite = time.Now()
}

// Stream takes one dump and always returns a Result: never a success without
// the completion marker, never a failure without a reason. It leaves an orphan
// dump alone, because the process running Stream may be killed at any moment
// and would lose the race; the BombVault process stops it instead.
func Stream(ctx context.Context, ex Execer, opts StreamOptions, stdout, stderr io.Writer) Result {
	argv, err := ExecArgv(opts.Engine, int(opts.Max/time.Second))
	if err != nil {
		return Result{V: 1, Reason: ReasonUsage, Detail: ScrubDetail(err.Error())}
	}
	state := opts.State
	if state == nil {
		state = &StreamState{}
	}
	dump := &dumpWriter{out: stdout, state: state, tail: tailKeeper{max: markerTailBytes}}
	errs := &logWriter{out: stderr, detail: tailKeeper{max: detailTailBytes}}

	exit, execErr := ex.ExecStream(ctx, opts.Container, argv, dump, errs)
	errs.flush()

	obs := Observation{
		Cancelled:   errors.Is(ctx.Err(), context.Canceled),
		TimedOut:    errors.Is(ctx.Err(), context.DeadlineExceeded),
		WriteFailed: dump.err != nil,
		NotRunning:  errors.Is(execErr, ErrNotRunning),
		Exit:        exit,
		StderrTail:  errs.detail.String(),
		Bytes:       dump.bytes,
		MarkerSeen:  strings.Contains(dump.tail.String(), opts.Engine.CompletionMarker()),
	}
	obs.DockerErr = execErr != nil && !obs.Cancelled && !obs.TimedOut && !obs.NotRunning && !obs.WriteFailed

	reason := Classify(obs)
	res := Result{V: 1, OK: reason == "", Reason: reason, Exit: exit, Bytes: obs.Bytes, Scope: errs.scope}
	if !res.OK {
		detail := obs.StderrTail
		if detail == "" && execErr != nil {
			detail = execErr.Error()
		}
		res.Detail = ScrubDetail(detail)
	}
	return res
}

// dumpWriter passes the dump through, counts it and keeps its end, where the
// completion marker stands.
type dumpWriter struct {
	out   io.Writer
	state *StreamState
	tail  tailKeeper
	bytes int64
	err   error
}

func (w *dumpWriter) Write(p []byte) (int, error) {
	n, err := w.out.Write(p)
	w.bytes += int64(n)
	w.tail.add(p[:n])
	if err != nil {
		w.err = err
		return n, err
	}
	w.state.wrote()
	return n, nil
}

// logWriter forwards the container's stderr line by line, so the pid line
// reaches restic while the dump is still running, and keeps the tail of
// everything that is not a protocol line as the failure detail.
type logWriter struct {
	out     io.Writer
	detail  tailKeeper
	pending []byte
	scope   string
}

func (w *logWriter) Write(p []byte) (int, error) {
	w.pending = append(w.pending, p...)
	for {
		at := bytes.IndexByte(w.pending, '\n')
		if at < 0 {
			return len(p), nil
		}
		w.emit(w.pending[:at+1])
		w.pending = w.pending[at+1:]
	}
}

func (w *logWriter) flush() {
	if len(w.pending) > 0 {
		w.emit(w.pending)
		w.pending = nil
	}
}

func (w *logWriter) emit(line []byte) {
	_, _ = w.out.Write(line)
	text := strings.TrimRight(string(line), "\r\n")
	if scope := ParseScope([]string{text}); scope != "" {
		w.scope = scope
		return
	}
	if strings.Contains(text, PIDLinePrefix) {
		return
	}
	w.detail.add(line)
}

// tailKeeper remembers the last max bytes written through it.
type tailKeeper struct {
	max int
	buf []byte
}

func (t *tailKeeper) add(p []byte) {
	if len(p) > t.max {
		p = p[len(p)-t.max:]
	}
	t.buf = append(t.buf, p...)
	if extra := len(t.buf) - t.max; extra > 0 {
		t.buf = t.buf[:copy(t.buf, t.buf[extra:])]
	}
}

func (t *tailKeeper) String() string { return string(t.buf) }
