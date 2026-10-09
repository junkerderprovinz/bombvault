package zfsrepl

import (
	"context"
	"io"
	"sync/atomic"

	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

func (r *run) stream(ctx context.Context, member string, sendArgs, recvArgs, estArgs []string) (int64, error) {
	total := estimate(ctx, r.src, estArgs)
	var report func(int64)
	if r.e.Progress != nil {
		report = func(done int64) { r.e.Progress(member, done, total) }
	}
	return pipe(ctx, r.src, r.dst, sendArgs, recvArgs, report)
}

// pipe streams one send into one receive and returns the bytes that crossed.
// When the receive gives up first, nothing drains the send any more and it
// would block on its pipe for good, so it is cancelled before its wait. Both
// sides have to succeed: a receive that ended cleanly on a send that failed
// took a stream that may be short.
func pipe(ctx context.Context, from, to End, sendArgs, recvArgs []string, report func(int64)) (int64, error) {
	sendCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	out, wait, err := from.Send(sendCtx, sendArgs)
	if err != nil {
		return 0, err
	}
	counted := &countingReader{r: out, report: report}
	recvErr := to.Receive(ctx, recvArgs, counted)
	if recvErr != nil {
		cancel()
	}
	sendErr := wait()
	_ = out.Close()
	n := counted.n.Load()

	switch {
	case recvErr == nil:
		return n, sendErr
	// A receive fails too when the send died first, and then the send knows
	// why. A send that was only cancelled here says nothing specific.
	case sendErr != nil && Code(sendErr) != "zfs-error":
		return n, sendErr
	}
	return n, recvErr
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

// countingReader counts what the receive read. The counter is atomic because
// the reading goroutine of a transport may outlive the receive.
type countingReader struct {
	r      io.Reader
	n      atomic.Int64
	report func(int64)
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 {
		done := c.n.Add(int64(n))
		if c.report != nil {
			c.report(done)
		}
	}
	return n, err
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
