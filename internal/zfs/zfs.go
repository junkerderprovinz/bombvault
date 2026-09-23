// Package zfs builds and validates the argv for the ZFS tools, parses their
// output and classifies their failures. SSHHost is the only part of it that
// talks to a machine.
package zfs

import (
	"context"
	"errors"
	"strings"
)

// Host is everything the ZFS domain asks of the machine that owns the pools.
// It is separate from backup.ZFSHost, which serves the VM domain's zvol
// streams and keeps its own argv.
type Host interface {
	Version(ctx context.Context) (string, error)
	Tree(ctx context.Context, root string) ([]ListEntry, error)
	List(ctx context.Context) ([]ListEntry, error)
	SnapshotRecursive(ctx context.Context, root, snap string) error
	DestroyRecursive(ctx context.Context, root, snap string) error
	SnapshotSafety(ctx context.Context, dataset, snap string) error
	DestroySafety(ctx context.Context, dataset, snap string) error
	Snapshots(ctx context.Context, root string) ([]SnapshotEntry, error)
	Prime(ctx context.Context, hostMountpoint, snap string) error
}

// ListEntry is one dataset or volume as zfs list reports it. Used is 0 in a
// tree listing, which does not ask for it.
type ListEntry struct {
	Name          string
	Type          string
	Mountpoint    string
	Canmount      string
	Encryption    string
	Keystatus     string
	Snapdir       string
	Mounted       bool
	Used          int64
	Referenced    int64
	UsedByDataset int64
}

// SnapshotEntry is one snapshot, split at the single '@' of its full name.
type SnapshotEntry struct {
	Dataset  string
	Name     string
	Creation int64
	Used     int64
}

// MemberCode decides whether one tree entry can be read, from its properties
// and the item's exclusions. "" means readable; anything else is a member code
// the page turns into a sentence. Mount visibility is checked separately,
// against the container's own mount table.
func MemberCode(e ListEntry, excluded []string) string {
	// A descendant whose snapshot name would not fit makes the whole
	// recursive snapshot fail, so it counts even when the user excluded it.
	if !SnapshotNameFits(e.Name) {
		return "name-too-long"
	}
	for _, ex := range excluded {
		if e.Name == ex || DescendantOf(e.Name, ex) {
			return "excluded"
		}
	}
	if e.Type == "volume" {
		return "zvol"
	}
	if err := validateNameChars(e.Name); err != nil {
		return "invalid-name"
	}
	switch {
	case e.Mountpoint == "legacy":
		return "legacy-mount"
	case e.Mountpoint == "none":
		return "no-mountpoint"
	case e.Canmount == "off":
		return "canmount-off"
	case e.Keystatus == "unavailable":
		return "key-not-loaded"
	case !e.Mounted:
		return "not-mounted"
	case e.Snapdir == "disabled":
		return "snapdir-disabled"
	}
	return ""
}

// DescendantOf reports whether name lies strictly below root. The separator is
// part of the comparison, so "cache/app" is not below "cache/ap".
func DescendantOf(name, root string) bool {
	if root == "" || name == "" {
		return false
	}
	return strings.HasPrefix(name, root+"/")
}

// CmdError carries what a failed zfs call said. Stderr is cut to stderrLimit
// and still needs the caller's path scrubber before it reaches a run row.
type CmdError struct {
	Args   []string
	Stderr string
	Code   string
	Err    error
}

// stderrLimit keeps a remote command that floods stderr out of a run row.
const stderrLimit = 500

func (e *CmdError) Error() string {
	msg := strings.TrimSpace(e.Stderr)
	if len(msg) > stderrLimit {
		msg = msg[:stderrLimit]
	}
	if msg == "" {
		if e.Err != nil {
			return "zfs: " + e.Code + ": " + e.Err.Error()
		}
		return "zfs: " + e.Code
	}
	return "zfs: " + e.Code + ": " + msg
}

func (e *CmdError) Unwrap() error { return e.Err }

// codeBusy and codeExists stay inside the backend: busy drives the destroy
// retry, and exists on a snapshot becomes snapshot-failed with the stderr as
// its detail.
const (
	codeBusy     = "busy"
	codeExists   = "exists"
	codeNotFound = "not-found"
)

// exitCoder is what os/exec's ExitError offers. SSH reports the remote exit
// status as its own, so 127 means the remote shell did not find zfs and 255 is
// ssh itself failing.
type exitCoder interface{ ExitCode() int }

// Classify maps a failed zfs call to a reason code, from the exit status and
// what the tools wrote to stderr.
func Classify(stderr string, err error) string {
	low := strings.ToLower(stderr)
	exit := -1
	var ec exitCoder
	if errors.As(err, &ec) {
		exit = ec.ExitCode()
	}

	if exit == 255 {
		switch {
		case strings.Contains(low, "permission denied (publickey"):
			return "ssh-auth"
		case strings.Contains(low, "connection refused"),
			strings.Contains(low, "connection timed out"),
			strings.Contains(low, "operation timed out"),
			strings.Contains(low, "no route to host"),
			strings.Contains(low, "could not resolve"):
			return "ssh-unreachable"
		}
	}
	switch {
	case exit == 127, strings.Contains(low, "command not found"):
		return "zfs-not-found"
	case strings.Contains(low, "dataset is busy"), strings.Contains(low, "pool or dataset is busy"):
		return codeBusy
	case strings.Contains(low, "dataset does not exist"),
		strings.Contains(low, "could not find any snapshots to destroy"):
		return codeNotFound
	case strings.Contains(low, "dataset already exists"):
		return codeExists
	case strings.Contains(low, "permission denied"):
		return "zfs-permission"
	}
	return "zfs-error"
}

// IsBusy reports whether a destroy failed because something still holds the
// snapshot, which is worth retrying within the budget.
func IsBusy(err error) bool { return hasCode(err, codeBusy) }

// IsNotFound reports whether the dataset or the snapshot was already gone,
// which a destroy treats as done.
func IsNotFound(err error) bool { return hasCode(err, codeNotFound) }

func hasCode(err error, code string) bool {
	var ce *CmdError
	return errors.As(err, &ce) && ce.Code == code
}
