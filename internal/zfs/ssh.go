package zfs

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/junkerderprovinz/bombvault/internal/sshconn"
)

// Runner is the transport SSHHost speaks over, satisfied by sshconn.Conn.
type Runner interface {
	RunCapture(ctx context.Context, args ...string) (stdout, stderr string, err error)
}

// usrSbinZFS is where zfs lives on Debian-based hosts whose non-interactive
// PATH for a non-root user leaves /usr/sbin out.
const usrSbinZFS = "/usr/sbin/zfs"

// SSHHost runs the ZFS tools on the pool owner over SSH.
type SSHHost struct {
	r       Runner
	usrSbin atomic.Bool
}

var _ Host = (*SSHHost)(nil)

// NewSSHHost returns a Host that talks over r.
func NewSSHHost(r Runner) *SSHHost { return &SSHHost{r: r} }

// Binary is the zfs command that worked on this host.
func (h *SSHHost) Binary() string {
	if h.usrSbin.Load() {
		return usrSbinZFS
	}
	return zfsBinary
}

// Version returns what the host's zfs reports, the cheapest proof that the
// whole path works.
func (h *SSHHost) Version(ctx context.Context) (string, error) {
	return h.run(ctx, VersionArgs())
}

// Tree lists one item's tree, root first.
func (h *SSHHost) Tree(ctx context.Context, root string) ([]ListEntry, error) {
	args, err := TreeArgs(root)
	if err != nil {
		return nil, err
	}
	out, err := h.runCapped(ctx, args)
	if err != nil {
		return nil, err
	}
	entries, err := ParseTree(out, root)
	if err != nil {
		return nil, &CmdError{Args: args, Code: "zfs-error", Err: err}
	}
	return entries, nil
}

// List reports every dataset and volume of every pool, for discovery.
func (h *SSHHost) List(ctx context.Context) ([]ListEntry, error) {
	args := ListArgs()
	out, err := h.runCapped(ctx, args)
	if err != nil {
		return nil, err
	}
	entries, err := ParseList(out)
	if err != nil {
		return nil, &CmdError{Args: args, Code: "zfs-error", Err: err}
	}
	return entries, nil
}

// Snapshots lists the snapshots of one item's tree, for the leftover sweep.
func (h *SSHHost) Snapshots(ctx context.Context, root string) ([]SnapshotEntry, error) {
	args, err := SnapshotsArgs(root)
	if err != nil {
		return nil, err
	}
	out, err := h.runCapped(ctx, args)
	if err != nil {
		return nil, err
	}
	snaps, err := ParseSnapshots(out)
	if err != nil {
		return nil, &CmdError{Args: args, Code: "zfs-error", Err: err}
	}
	return snaps, nil
}

// SnapshotRecursive takes the one atomic snapshot a backup run reads.
func (h *SSHHost) SnapshotRecursive(ctx context.Context, root, snap string) error {
	args, err := SnapshotRecursiveArgs(root, snap)
	if err != nil {
		return err
	}
	_, err = h.run(ctx, args)
	return err
}

// DestroyRecursive removes a run's snapshot from the whole tree.
func (h *SSHHost) DestroyRecursive(ctx context.Context, root, snap string) error {
	args, err := DestroyRecursiveArgs(root, snap)
	if err != nil {
		return err
	}
	_, err = h.run(ctx, args)
	return err
}

// SnapshotSafety takes the snapshot that lets a user undo an in-place restore.
func (h *SSHHost) SnapshotSafety(ctx context.Context, dataset, snap string) error {
	args, err := SnapshotSafetyArgs(dataset, snap)
	if err != nil {
		return err
	}
	_, err = h.run(ctx, args)
	return err
}

// DestroySafety removes one safety snapshot.
func (h *SSHHost) DestroySafety(ctx context.Context, dataset, snap string) error {
	args, err := DestroyPreRestoreArgs(dataset, snap)
	if err != nil {
		return err
	}
	_, err = h.run(ctx, args)
	return err
}

// Prime reads the snapshot directory on the host so the kernel mounts it
// there, where the path it stored for the dataset is the one that applies.
func (h *SSHHost) Prime(ctx context.Context, hostMountpoint, snap string) error {
	args, err := PrimeArgs(hostMountpoint, snap)
	if err != nil {
		return err
	}
	_, stderr, err := h.r.RunCapture(ctx, args...)
	if err != nil {
		return &CmdError{Args: args, Stderr: stderr, Code: Classify(stderr, err), Err: err}
	}
	return nil
}

// runCapped runs a listing. A listing the transport cut would parse into a
// tree with datasets silently missing, so it is refused whole.
func (h *SSHHost) runCapped(ctx context.Context, args []string) (string, error) {
	out, err := h.run(ctx, args)
	if errors.Is(err, sshconn.ErrStdoutCut) {
		return "", &CmdError{Args: args, Code: "zfs-error", Stderr: "dataset listing is larger than the 16 MiB limit", Err: err}
	}
	return out, err
}

// run sends a zfs argv to the host. A host that answers "command not found"
// gets one retry under /usr/sbin, and a binary that worked is kept for the
// rest of the process.
func (h *SSHHost) run(ctx context.Context, args []string) (string, error) {
	argv := withBinary(args, h.Binary())
	stdout, stderr, err := h.r.RunCapture(ctx, argv...)
	if err == nil {
		return stdout, nil
	}
	if !h.usrSbin.Load() && Classify(stderr, err) == "zfs-not-found" {
		argv = withBinary(args, usrSbinZFS)
		stdout, stderr, err = h.r.RunCapture(ctx, argv...)
		if err == nil {
			h.usrSbin.Store(true)
			return stdout, nil
		}
	}
	return "", &CmdError{Args: argv, Stderr: stderr, Code: Classify(stderr, err), Err: err}
}

// withBinary replaces the argv[0] the builders emit with the one that works on
// this host.
func withBinary(args []string, bin string) []string {
	out := make([]string, len(args))
	copy(out, args)
	out[0] = bin
	return out
}
