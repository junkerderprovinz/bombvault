// Package zfsrepl keeps the datasets of a ZFS item current on a second ZFS
// host with incremental zfs send and receive. It drives two Ends and never
// assumes which of them is the local one, so a push and a pull run the same
// code with the roles swapped.
package zfsrepl

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// End is one host of a replica, the source the snapshots leave or the target
// that keeps them. A command that fails on the host comes back as a
// *zfs.CmdError carrying its reason code.
type End interface {
	Run(ctx context.Context, args []string) (string, error)
	// Send starts a zfs send and streams its output. wait is called exactly
	// once, after reading stops.
	Send(ctx context.Context, args []string) (stream io.ReadCloser, wait func() error, err error)
	Receive(ctx context.Context, args []string, stream io.Reader) error
}

// SelfPruning is an End that applies its own retention after each receive. A
// run never prunes such a target, whatever Entry.Keep says.
type SelfPruning interface {
	PrunesItself()
}

// Entry is one ZFS item to replicate: its root, every dataset and volume below
// it that is not excluded, and where they land on the target.
type Entry struct {
	Root     string
	Excluded []string
	// TargetBase is <target root>/<server name>. A member lands at
	// TargetBase/<member>, so the source pool is part of its path.
	TargetBase string
	// Placeholders are the empty parents earlier runs created on the target.
	// They are the only datasets a first full stream may overwrite.
	Placeholders []string
	// Owner is the instance id every parent this run creates is marked with,
	// as zfs.SourceProperty. Empty marks nothing.
	Owner string
	// Keep is the target's retention over the replica snapshots. All zero
	// keeps every one.
	Keep store.RetentionKeep
	Now  func() time.Time
	// Progress hears how many bytes of a member's stream crossed so far, and
	// the estimate of its size, which is 0 when the estimate failed. It is
	// called from the goroutine that feeds the receive.
	Progress func(member string, done, total int64)
	// Finished hears each member's result as soon as it has one, so a run cut
	// short still leaves what it did.
	Finished func(MemberResult)
}

// Result is what one run did. Created lists the placeholders it made on the
// target, which the caller keeps for later runs as Entry.Placeholders.
type Result struct {
	Snapshot string
	Members  []MemberResult
	Created  []string
}

// MemberResult is how one dataset or volume fared. Code is "" when the member
// holds Snapshot on both sides now, and a reason code otherwise.
type MemberResult struct {
	Dataset string
	Target  string
	Volume  bool
	Raw     bool
	// Base is the snapshot or bookmark the increment started from, "" for a
	// full stream.
	Base         string
	FromBookmark bool
	Snapshot     string
	GUID         uint64
	Bytes        int64
	// Resumed says that an earlier interrupted stream was finished first.
	Resumed bool
	Pruned  []string
	Code    string
	Err     error
}

// Refusal is a failure the engine decided on itself. Err is the host's own
// failure behind it, if there was one.
type Refusal struct {
	Code   string
	Detail string
	Err    error
}

func (e *Refusal) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Detail + ": " + e.Err.Error()
	}
	return e.Code + ": " + e.Detail
}

func (e *Refusal) Unwrap() error { return e.Err }

// Code is the reason code of an error the engine or an End returned.
func Code(err error) string {
	var rf *Refusal
	var ce *zfs.CmdError
	var ne *zfs.NameError
	switch {
	case errors.As(err, &rf):
		return rf.Code
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "not-reached"
	case errors.As(err, &ce):
		return ce.Code
	case errors.As(err, &ne):
		return ne.Code
	}
	return "zfs-error"
}
