package dbdump_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/dbdump"
)

// fakeExecer plays the container side of a dump: run writes what the dump tool
// would write and returns its exit code.
type fakeExecer struct {
	run        func(ctx context.Context, stdout, stderr io.Writer) (int, error)
	container  string
	cmd        []string
	quickCalls int
}

func (f *fakeExecer) ExecStream(ctx context.Context, container string, cmd []string, stdout, stderr io.Writer) (int, error) {
	f.container, f.cmd = container, cmd
	return f.run(ctx, stdout, stderr)
}

func (f *fakeExecer) ExecQuick(context.Context, string, []string) error {
	f.quickCalls++
	return nil
}

// syncBuffer collects what Stream forwards while the dump is still running.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func postgresDump(size int) string {
	marker := dbdump.EnginePostgres.CompletionMarker() + "\n"
	return strings.Repeat("x", size-len(marker)) + marker
}

func streamOpts() dbdump.StreamOptions {
	return dbdump.StreamOptions{Container: "pg", Engine: dbdump.EnginePostgres, Max: time.Hour}
}

func TestStreamSuccess(t *testing.T) {
	const size = 100 << 10
	dump := postgresDump(size)
	ex := &fakeExecer{run: func(_ context.Context, stdout, stderr io.Writer) (int, error) {
		_, _ = io.WriteString(stderr, dbdump.ScopeLinePrefix+"all\n")
		_, _ = io.WriteString(stderr, dbdump.PIDLinePrefix+"4711\n")
		for at := 0; at < len(dump); at += 8 << 10 {
			end := min(at+8<<10, len(dump))
			if _, err := io.WriteString(stdout, dump[at:end]); err != nil {
				return 1, err
			}
		}
		return 0, nil
	}}

	var stdout, stderr bytes.Buffer
	res := dbdump.Stream(context.Background(), ex, streamOpts(), &stdout, &stderr)

	if !res.OK || res.Reason != "" {
		t.Fatalf("result = %+v, want ok", res)
	}
	if res.V != 1 {
		t.Errorf("v = %d, want 1", res.V)
	}
	if res.Bytes != size {
		t.Errorf("bytes = %d, want %d", res.Bytes, size)
	}
	if res.Scope != "all" {
		t.Errorf("scope = %q, want all", res.Scope)
	}
	if stdout.String() != dump {
		t.Errorf("stdout has %d bytes, want the %d bytes of the dump", stdout.Len(), len(dump))
	}
	if ex.container != "pg" {
		t.Errorf("container = %q, want pg", ex.container)
	}
	want, err := dbdump.ExecArgv(dbdump.EnginePostgres, 3600)
	if err != nil {
		t.Fatalf("ExecArgv: %v", err)
	}
	if strings.Join(ex.cmd, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("cmd = %q, want %q", ex.cmd, want)
	}
}

// A dump tool can be killed mid-write and still leave exit 0 behind, so the
// completion marker decides, not the exit code.
func TestStreamTruncatedDumpWithExitZeroFails(t *testing.T) {
	ex := &fakeExecer{run: func(_ context.Context, stdout, _ io.Writer) (int, error) {
		_, err := io.WriteString(stdout, strings.Repeat("x", 4096))
		return 0, err
	}}

	var stdout, stderr bytes.Buffer
	res := dbdump.Stream(context.Background(), ex, streamOpts(), &stdout, &stderr)

	if res.OK {
		t.Fatal("a dump without its completion marker was reported as a success")
	}
	if res.Reason != dbdump.ReasonIncomplete {
		t.Errorf("reason = %q, want %q", res.Reason, dbdump.ReasonIncomplete)
	}
	if res.Bytes != 4096 {
		t.Errorf("bytes = %d, want 4096", res.Bytes)
	}
}

func TestStreamForwardsProtocolLines(t *testing.T) {
	forwarded := &syncBuffer{}
	dump := postgresDump(16 << 10)
	ex := &fakeExecer{run: func(_ context.Context, stdout, stderr io.Writer) (int, error) {
		_, _ = io.WriteString(stderr, dbdump.PIDLinePrefix+"42\n")
		_, _ = io.WriteString(stderr, dbdump.ScopeLinePrefix+"database\n")
		if seen := forwarded.String(); !strings.Contains(seen, dbdump.PIDLinePrefix+"42") {
			t.Errorf("stderr = %q, want the pid line while the dump is still running", seen)
		}
		_, err := io.WriteString(stdout, dump)
		return 0, err
	}}

	var stdout bytes.Buffer
	res := dbdump.Stream(context.Background(), ex, streamOpts(), &stdout, forwarded)

	out := forwarded.String()
	pid := strings.Index(out, dbdump.PIDLinePrefix+"42")
	scope := strings.Index(out, dbdump.ScopeLinePrefix+"database")
	if pid < 0 || scope < 0 {
		t.Fatalf("stderr = %q, want both protocol lines unchanged", out)
	}
	if pid > scope {
		t.Errorf("stderr = %q, want the pid line before the scope line", out)
	}
	if res.Scope != "database" {
		t.Errorf("scope = %q, want database", res.Scope)
	}
}

func TestStreamWriteFailure(t *testing.T) {
	dump := postgresDump(8 << 10)
	ex := &fakeExecer{run: func(_ context.Context, stdout, _ io.Writer) (int, error) {
		if _, err := io.WriteString(stdout, dump); err != nil {
			return 2, err
		}
		return 0, nil
	}}

	var stderr bytes.Buffer
	res := dbdump.Stream(context.Background(), ex, streamOpts(), brokenWriter{}, &stderr)

	if res.Reason != dbdump.ReasonWrite {
		t.Errorf("reason = %q, want %q", res.Reason, dbdump.ReasonWrite)
	}
	if res.OK {
		t.Error("a failed write was reported as a success")
	}
	if ex.quickCalls != 0 {
		t.Errorf("ExecQuick called %d times, want 0: stopping an orphan is not the stream's job", ex.quickCalls)
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, syscall.EPIPE }

// The helper can be killed at any moment, so an orphan stop from inside the
// stream would lose that race; the BombVault process runs it instead.
func TestStreamCancelNeverStopsOrphansItself(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ex := &fakeExecer{run: func(ctx context.Context, _, stderr io.Writer) (int, error) {
		_, _ = io.WriteString(stderr, dbdump.PIDLinePrefix+"42\n")
		<-ctx.Done()
		return 0, ctx.Err()
	}}

	var stdout, stderr bytes.Buffer
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	res := dbdump.Stream(ctx, ex, streamOpts(), &stdout, &stderr)

	if res.Reason != dbdump.ReasonCancelled {
		t.Errorf("reason = %q, want %q", res.Reason, dbdump.ReasonCancelled)
	}
	if ex.quickCalls != 0 {
		t.Errorf("ExecQuick called %d times, want 0", ex.quickCalls)
	}
}

func TestStreamNotRunning(t *testing.T) {
	ex := &fakeExecer{run: func(context.Context, io.Writer, io.Writer) (int, error) {
		return 0, fmt.Errorf("exec create: %w", dbdump.ErrNotRunning)
	}}

	var stdout, stderr bytes.Buffer
	res := dbdump.Stream(context.Background(), ex, streamOpts(), &stdout, &stderr)

	if res.Reason != dbdump.ReasonNotRunning {
		t.Errorf("reason = %q, want %q", res.Reason, dbdump.ReasonNotRunning)
	}
	if res.Detail == "" {
		t.Error("detail is empty, want the docker error to reach the run")
	}
}
