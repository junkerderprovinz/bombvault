package dockercli

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// blockingReader never delivers a byte until it is closed, which is how a
// hijacked exec attach behaves while the hook inside the container hangs: the
// SDK's raw connection does not look at the context at all.
type blockingReader struct {
	closed chan struct{}
	closes atomic.Int32
}

func newBlockingReader() *blockingReader { return &blockingReader{closed: make(chan struct{})} }

func (b *blockingReader) Read([]byte) (int, error) {
	<-b.closed
	return 0, io.ErrClosedPipe
}

func (b *blockingReader) Close() error {
	if b.closes.Add(1) == 1 {
		close(b.closed)
	}
	return nil
}

// A hung hook used to hold the backup forever: cancel, the BACKUP_MAX_HOURS cap
// and shutdown all end in a cancelled context, and none of them reached the
// attach read. The read has to give up once the context does.
func TestDrainExecOutputStopsWhenContextEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := newBlockingReader()

	done := make(chan error, 1)
	go func() {
		_, _, err := drainExecOutput(ctx, r, func() { _ = r.Close() })
		done <- err
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("drainExecOutput kept reading after the context was cancelled")
	}
	if r.closes.Load() == 0 {
		t.Fatal("the attach was never closed, so the goroutine reading it would leak")
	}
}

// The ordinary case must not change: a hook that finishes is read to the end
// and its output kept for the failure reason.
func TestDrainExecOutputReadsAFinishedHook(t *testing.T) {
	// stdcopy frame: stream 1 (stdout), 4-byte big-endian length, payload.
	const payload = "dump ok\n"
	frame := append([]byte{1, 0, 0, 0, 0, 0, 0, 8}, payload...)
	closer := &countingCloser{}

	stdout, stderr, err := drainExecOutput(context.Background(), strings.NewReader(string(frame)), func() { _ = closer.Close() })
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got := stdout.String(); got != payload {
		t.Errorf("stdout = %q, want %q", got, payload)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

type countingCloser struct{ n atomic.Int32 }

func (c *countingCloser) Close() error { c.n.Add(1); return nil }

// The exit code is set by the daemon's event handler, separately from the end
// of the output stream, so reading it once right after the stream ends can see
// a zero that only means "not finished yet". A failing hook would then pass.
func TestWaitExecExitWaitsUntilNotRunning(t *testing.T) {
	calls := 0
	inspect := func(context.Context) (bool, int, error) {
		calls++
		if calls < 3 {
			return true, 0, nil
		}
		return false, 7, nil
	}

	code, err := waitExecExit(context.Background(), inspect, time.Millisecond, time.Second)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if code != 7 {
		t.Fatalf("exit code = %d, want 7 (read before the process had finished)", code)
	}
}

// A process that never reports finished must not turn into success either.
func TestWaitExecExitGivesUpWithAnError(t *testing.T) {
	inspect := func(context.Context) (bool, int, error) { return true, 0, nil }

	_, err := waitExecExit(context.Background(), inspect, time.Millisecond, 20*time.Millisecond)
	if err == nil {
		t.Fatal("a hook still running after the wait was treated as finished")
	}
}

func TestWaitExecExitHonoursCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	inspect := func(context.Context) (bool, int, error) { return true, 0, nil }

	_, err := waitExecExit(ctx, inspect, time.Millisecond, time.Minute)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}
