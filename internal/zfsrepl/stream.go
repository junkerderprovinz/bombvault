package zfsrepl

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"

	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

func (r *run) stream(ctx context.Context, member string, sendArgs, recvArgs []string, total int64) (int64, error) {
	var report func(int64)
	if r.e.Progress != nil {
		report = func(done int64) { r.e.Progress(member, done, total) }
	}
	if p, ok := r.src.(Piper); ok {
		if n, piped, err := p.Pipe(ctx, sendArgs, r.dst, recvArgs, report); piped {
			// Its progress lags behind the stream, and a short one shows
			// none, so a stream that arrived counts at least its estimate.
			if err == nil {
				n = max(n, total)
			}
			return n, err
		}
	}
	return pipe(ctx, r.src, r.dst, sendArgs, recvArgs, report)
}

// pipe streams one send into one receive and returns the bytes that crossed.
// When the receive gives up first, nothing drains the send any more and it
// would block on its pipe for good, so it is cancelled before its wait.
func pipe(ctx context.Context, from, to End, sendArgs, recvArgs []string, report func(int64)) (int64, error) {
	sendCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	out, wait, err := from.Send(sendCtx, sendArgs)
	if err != nil {
		return 0, err
	}
	counted := newRelay(out, report)
	recvErr := to.Receive(ctx, recvArgs, counted)
	counted.stop()
	if recvErr != nil {
		cancel()
	}
	sendErr := wait()
	_ = out.Close()
	return counted.n.Load(), settle(sendErr, recvErr)
}

// settle is the error of a send and the receive it fed. Both sides have to
// succeed: a receive that ended cleanly on a send that failed took a stream
// that may be short.
func settle(sendErr, recvErr error) error {
	switch {
	case recvErr == nil:
		return sendErr
	// A receive fails too when the send died first, and then the send knows
	// why. A send that was only stopped says nothing specific.
	case sendErr != nil && Code(sendErr) != "zfs-error":
		return sendErr
	}
	return recvErr
}

// estimate asks the source how large a stream will be. It is only for the
// progress bar: an estimate succeeds where the send may still fail, and a
// failed one costs nothing but the total.
func estimate(ctx context.Context, end End, args []string) int64 {
	out, err := end.Run(ctx, args)
	if err != nil {
		return 0
	}
	n, err := zfs.ParseEstimate(out)
	if err != nil {
		return 0
	}
	return n
}

// relayChunk and relayDepth size the buffer between a send and its receive,
// which mbuffer is in a plain zfs send | ssh zfs receive pipeline: a receive
// that stops to write out a transaction group does not stall the send behind
// it, and the stream reaches the receive in large writes instead of the 64 KiB
// a pipe holds.
const (
	relayChunk = 1 << 20
	relayDepth = 32
)

// relay reads a send stream ahead into chunks the receive takes in order, and
// counts what the receive took. The counter is atomic because the reading
// goroutine of a transport may outlive the receive.
type relay struct {
	chunks chan []byte
	free   chan []byte
	done   chan struct{}
	once   sync.Once
	// err is the read error that ended the stream, set before chunks closes.
	err    error
	cur    []byte
	held   []byte
	n      atomic.Int64
	report func(int64)
}

func newRelay(src io.Reader, report func(int64)) *relay {
	r := &relay{
		chunks: make(chan []byte, relayDepth),
		free:   make(chan []byte, relayDepth+1),
		done:   make(chan struct{}),
		report: report,
	}
	go r.fill(src)
	return r
}

func (r *relay) fill(src io.Reader) {
	defer close(r.chunks)
	for {
		var buf []byte
		select {
		case buf = <-r.free:
		default:
			buf = make([]byte, relayChunk)
		}
		n, err := io.ReadFull(src, buf[:cap(buf)])
		if n > 0 {
			select {
			case r.chunks <- buf[:n]:
			case <-r.done:
				return
			}
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return
		}
		if err != nil {
			r.err = err
			return
		}
	}
}

// next hands the chunk the receive is done with back for reuse and takes the
// following one; false at the end of the stream.
func (r *relay) next() bool {
	if r.held != nil {
		select {
		case r.free <- r.held:
		default:
		}
		r.held = nil
	}
	chunk, ok := <-r.chunks
	if !ok {
		return false
	}
	r.cur, r.held = chunk, chunk
	return true
}

func (r *relay) end() error {
	if r.err != nil {
		return r.err
	}
	return io.EOF
}

func (r *relay) took(n int) {
	done := r.n.Add(int64(n))
	if r.report != nil {
		r.report(done)
	}
}

func (r *relay) Read(p []byte) (int, error) {
	if len(r.cur) == 0 && !r.next() {
		return 0, r.end()
	}
	n := copy(p, r.cur)
	r.cur = r.cur[n:]
	r.took(n)
	return n, nil
}

// WriteTo is what io.Copy uses, so the receive gets whole chunks at a time.
func (r *relay) WriteTo(w io.Writer) (int64, error) {
	var total int64
	for len(r.cur) > 0 || r.next() {
		n, err := w.Write(r.cur)
		r.cur = r.cur[n:]
		total += int64(n)
		r.took(n)
		if err != nil {
			return total, err
		}
	}
	if r.err != nil {
		return total, r.err
	}
	return total, nil
}

// stop lets the reading goroutine go once the receive is over.
func (r *relay) stop() { r.once.Do(func() { close(r.done) }) }

// ReadAhead puts the buffer a run keeps between a send and its receive in
// front of a stream that reaches a receive some other way. stop lets the
// reading go once the receive is over.
func ReadAhead(src io.Reader) (ahead io.Reader, stop func()) {
	r := newRelay(src, nil)
	return r, r.stop
}

func listTree(ctx context.Context, end End, root string) ([]zfs.ListEntry, error) {
	args, err := zfs.TreeArgs(root)
	if err != nil {
		return nil, err
	}
	out, err := end.Run(ctx, args)
	if err != nil {
		return nil, err
	}
	return zfs.ParseTree(out, root)
}

func points(ctx context.Context, end End, dataset string) ([]zfs.ReplicaPoint, error) {
	args, err := zfs.ReplicaPointsArgs(dataset)
	if err != nil {
		return nil, err
	}
	out, err := end.Run(ctx, args)
	if err != nil {
		return nil, err
	}
	return zfs.ParseReplicaPoints(out, dataset)
}

func state(ctx context.Context, end End, dataset string) (zfs.DatasetState, error) {
	args, err := zfs.DatasetStateArgs(dataset)
	if err != nil {
		return zfs.DatasetState{}, err
	}
	out, err := end.Run(ctx, args)
	if err != nil {
		return zfs.DatasetState{}, err
	}
	return zfs.ParseDatasetState(out)
}

// release lifts the replica hold from one snapshot. A snapshot without it
// counts as released.
func release(ctx context.Context, end End, dataset, snap string) error {
	args, err := zfs.ReleaseArgs(dataset, snap)
	if err != nil {
		return err
	}
	_, err = end.Run(ctx, args)
	return ignore(err, zfs.IsNotFound)
}
