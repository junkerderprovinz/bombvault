// Package hostsnap reads and changes the snapshots the server itself keeps:
// ZFS snapshots, btrfs snapshots and the snapshots libvirt records for a VM.
// They are not backups; a backup is a restic snapshot in a repository, and
// that word stays with the restic side of BombVault.
//
// ZFS and btrfs are reached through a shell on the host, VMs through virsh.
// A host with neither tool, or without the SSH connection, is a normal state
// the survey reports per kind.
package hostsnap

import (
	"context"
	"errors"
	"sort"
)

// Kind is the system that keeps a snapshot.
type Kind string

const (
	KindZFS   Kind = "zfs"
	KindBtrfs Kind = "btrfs"
	KindVM    Kind = "vm"
)

// Shell runs a command on the host and returns its two streams apart. It is
// satisfied by sshconn.Conn.
type Shell interface {
	RunCapture(ctx context.Context, args ...string) (stdout, stderr string, err error)
}

// VMs is the part of virsh the snapshots of a VM need.
type VMs interface {
	DomainNames(ctx context.Context) ([]string, error)
	SnapshotTable(ctx context.Context, domain string) (string, error)
	SnapshotCreate(ctx context.Context, domain, name string) error
	SnapshotDelete(ctx context.Context, domain, name string) error
	DumpXML(ctx context.Context, domain string) (string, error)
}

// Ref names one snapshot. Source is the dataset, the subvolume path below the
// filesystem's top level, or the VM; FS is the mountpoint of a btrfs
// filesystem's top level and empty for the other kinds.
type Ref struct {
	Kind   Kind   `json:"kind"`
	FS     string `json:"fs,omitempty"`
	Source string `json:"source"`
	Name   string `json:"name"`
}

// Caps is what can be done to one snapshot from here.
type Caps struct {
	Rename   bool `json:"rename"`
	Hold     bool `json:"hold"`
	Delete   bool `json:"delete"`
	Estimate bool `json:"estimate"`
}

// Snapshot is one snapshot as the page lists it. A size of -1 means the
// system does not report it: btrfs without quotas, and every VM snapshot.
type Snapshot struct {
	Ref
	// Label is the name to show: Name itself, except for btrfs, whose Name
	// is the snapshot's path.
	Label      string   `json:"label"`
	CreatedAt  int64    `json:"createdAt"`
	Used       int64    `json:"usedBytes"`
	Referenced int64    `json:"referencedBytes"`
	NewSince   int64    `json:"newSinceBytes"`
	Holds      []string `json:"holds"`
	Clones     []string `json:"clones"`
	Owner      string   `json:"owner"`
	PlanID     string   `json:"planId,omitempty"`
	ReadOnly   bool     `json:"readOnly"`
	// ManagedBy names the tool that keeps the snapshot when it is not one
	// this page may change, such as Unraid's own VM snapshots.
	ManagedBy string `json:"managedBy,omitempty"`
	Caps      Caps   `json:"caps"`
}

// Source is something a snapshot can be taken of.
type Source struct {
	Kind  Kind   `json:"kind"`
	FS    string `json:"fs,omitempty"`
	Name  string `json:"source"`
	Group string `json:"group"`
	// Used is what the source holds, -1 where it is not known.
	Used int64 `json:"usedBytes"`
	// NewSinceLatest is what was written since its newest snapshot.
	NewSinceLatest int64 `json:"newSinceLatestBytes"`
	Recursive      bool  `json:"canRecursive"`
	Create         bool  `json:"canCreate"`
	// Reason says why Create is off, such as CodeNeedsQcow2.
	Reason string `json:"reason,omitempty"`
}

// Pool is a ZFS pool or a btrfs filesystem with the power state of its disks.
type Pool struct {
	Kind  Kind   `json:"kind"`
	Name  string `json:"name"`
	State string `json:"state"`
	// Read says whether its snapshots were listed. A pool whose disks sleep
	// is only read when the person asks for it.
	Read bool `json:"read"`
	// Woke is set after such a read when a disk that slept spun up.
	Woke bool `json:"woke"`
}

const (
	StateAwake   = "awake"
	StateAsleep  = "asleep"
	StateUnknown = "unknown"
)

// KindState is whether one kind could be read at all.
type KindState struct {
	// Code is "ok", "missing" when the host lacks the tool, or the reason a
	// read failed.
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}

// Survey is everything the page lists.
type Survey struct {
	Kinds     map[Kind]KindState `json:"kinds"`
	Pools     []Pool             `json:"pools"`
	Sources   []Source           `json:"sources"`
	Snapshots []Snapshot         `json:"snapshots"`
	// HiddenSystem counts the snapshots of Docker's storage layers, which are
	// not listed.
	HiddenSystem int `json:"hiddenSystem"`
}

// Host is the snapshots of one server.
type Host struct {
	sh  Shell
	vms VMs
}

// New returns a Host. A nil vms leaves VMs out of the survey.
func New(sh Shell, vms VMs) *Host { return &Host{sh: sh, vms: vms} }

// Error carries a reason code the page turns into a sentence, and what the
// host said.
type Error struct {
	Code   string
	Detail string
}

func (e *Error) Error() string {
	if e.Detail == "" {
		return e.Code
	}
	return e.Code + ": " + e.Detail
}

// CodeOf returns the reason code of err, "error" when it carries none.
func CodeOf(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	if errors.Is(err, ErrInvalidName) {
		return "invalid-name"
	}
	return "error"
}

func sortSnapshots(snaps []Snapshot) {
	sort.SliceStable(snaps, func(i, j int) bool {
		a, b := snaps[i], snaps[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.FS != b.FS {
			return a.FS < b.FS
		}
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.CreatedAt != b.CreatedAt {
			return a.CreatedAt > b.CreatedAt
		}
		return a.Name < b.Name
	})
}
