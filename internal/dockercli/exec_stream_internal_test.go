package dockercli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/junkerderprovinz/bombvault/internal/dbdump"
)

// execFrames builds the multiplexed output the daemon sends on an exec attach,
// one frame per chunk.
func execFrames(stream stdcopy.StdType, payload []byte, chunk int) []byte {
	var out bytes.Buffer
	w := stdcopy.NewStdWriter(&out, stream)
	for at := 0; at < len(payload); at += chunk {
		_, _ = w.Write(payload[at:min(at+chunk, len(payload))])
	}
	return out.Bytes()
}

// A dump is megabytes, not the kilobytes a hook's failure reason needs, so this
// drain has no cap at all.
func TestDrainExecStreamsCopiesEverything(t *testing.T) {
	payload := bytes.Repeat([]byte("dump"), 256<<10)
	frames := execFrames(stdcopy.Stdout, payload, 32<<10)

	var stdout, stderr bytes.Buffer
	err := drainExecStreams(context.Background(), bytes.NewReader(frames), func() {}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if stdout.Len() != len(payload) {
		t.Errorf("stdout = %d bytes, want %d", stdout.Len(), len(payload))
	}
	if !bytes.Equal(stdout.Bytes(), payload) {
		t.Error("stdout differs from the payload")
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

// restic's pipe can close under the dump; discarding that error the way the
// hook drain does would turn a truncated dump into a success.
func TestDrainExecStreamsReturnsStdoutWriteError(t *testing.T) {
	frames := execFrames(stdcopy.Stdout, []byte("half a dump"), 4)
	want := errors.New("pipe closed")

	err := drainExecStreams(context.Background(), bytes.NewReader(frames), func() {}, failingWriter{err: want}, io.Discard)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

type failingWriter struct{ err error }

func (f failingWriter) Write([]byte) (int, error) { return 0, f.err }

func TestDrainExecStreamsStopsWhenContextEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := newBlockingReader()

	done := make(chan error, 1)
	go func() {
		done <- drainExecStreams(ctx, r, func() { _ = r.Close() }, io.Discard, io.Discard)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("drainExecStreams kept reading after the context was cancelled")
	}
	if r.closes.Load() == 0 {
		t.Fatal("the attach was never closed, so the goroutine reading it would leak")
	}
}

// A paused or stopped container answers exec create with a 409, which the dump
// reports as its own reason instead of a docker error.
func TestExecCreateConflictMapsToNotRunning(t *testing.T) {
	conflict := fmt.Errorf("Error response from daemon: container pg is paused: %w", cerrdefs.ErrConflict)
	if err := mapExecCreateErr(conflict); !errors.Is(err, dbdump.ErrNotRunning) {
		t.Errorf("err = %v, want it to carry dbdump.ErrNotRunning", err)
	}

	other := errors.New("dial unix /var/run/docker.sock: no such file")
	if err := mapExecCreateErr(other); !errors.Is(err, other) || errors.Is(err, dbdump.ErrNotRunning) {
		t.Errorf("err = %v, want the original error unchanged", err)
	}
}

func TestCapturedOutputKeepsCapAndDrains(t *testing.T) {
	payload := bytes.Repeat([]byte("v"), 1<<20)
	r := &countingReader{Reader: bytes.NewReader(execFrames(stdcopy.Stdout, payload, 64<<10))}

	stdout, _, err := capturedOutput(context.Background(), r, func() {}, 4<<10)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if stdout != strings.Repeat("v", 4<<10) {
		t.Errorf("stdout = %d bytes, want the first %d", len(stdout), 4<<10)
	}
	if !r.atEOF.Load() {
		t.Error("the rest of the output was left unread, so the process could not finish")
	}
}

type countingReader struct {
	io.Reader
	atEOF atomic.Bool
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.Reader.Read(p)
	if errors.Is(err, io.EOF) {
		c.atEOF.Store(true)
	}
	return n, err
}

// An importer that exits while BombVault is still writing the dump leaves the
// write blocked; closing the attach first is what ends it.
func TestStdinFeedDoesNotOutliveEarlyExit(t *testing.T) {
	conn := &blockingAttach{out: bytes.NewReader(execFrames(stdcopy.Stderr, []byte("ERROR:  relation exists\n"), 8))}
	conn.blocked = make(chan struct{})

	var tail bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- feedExecStdin(context.Background(), conn, strings.NewReader(strings.Repeat("INSERT;\n", 1<<16)), &tail)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("err = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the stdin feed waited for a writer that only the attach close can release")
	}
	if conn.closes.Load() == 0 {
		t.Fatal("the attach was never closed")
	}
	if !strings.Contains(tail.String(), "ERROR:  relation exists") {
		t.Errorf("stderr tail = %q, want the importer's message", tail.String())
	}
}

// blockingAttach is an exec attach whose output ends right away while every
// write to it blocks until the attach is closed.
type blockingAttach struct {
	out     io.Reader
	blocked chan struct{}
	closes  atomic.Int32
}

func (b *blockingAttach) Read(p []byte) (int, error) { return b.out.Read(p) }

func (b *blockingAttach) Write([]byte) (int, error) {
	<-b.blocked
	return 0, io.ErrClosedPipe
}

func (b *blockingAttach) CloseWrite() error { return nil }

func (b *blockingAttach) Close() {
	if b.closes.Add(1) == 1 {
		close(b.blocked)
	}
}
