package zfs

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// exitStatus stands in for the *exec.ExitError an ssh call returns. ssh reports
// the remote command's exit status as its own, except for its own failures,
// which are always 255.
type exitStatus int

func (e exitStatus) ExitCode() int { return int(e) }
func (e exitStatus) Error() string { return fmt.Sprintf("exit status %d", int(e)) }

func TestClassify(t *testing.T) {
	cases := []struct {
		name   string
		stderr string
		err    error
		want   string
	}{
		{"missing dataset", "cannot open 'cache/nope': dataset does not exist", exitStatus(1), "not-found"},
		{"nothing to destroy", "could not find any snapshots to destroy; check snapshot names.", exitStatus(1), "not-found"},
		{"busy", "cannot destroy 'cache/appdata@bombvault-20260917031500': dataset is busy", exitStatus(1), "busy"},
		{"pool busy", "cannot destroy 'cache/appdata': pool or dataset is busy", exitStatus(1), "busy"},
		{"already there", "cannot create snapshot 'cache/appdata@bombvault-20260917031500': dataset already exists", exitStatus(1), "exists"},
		{"no zfs on PATH", "bash: line 1: zfs: command not found", exitStatus(127), "zfs-not-found"},
		{"no zfs, no message", "", exitStatus(127), "zfs-not-found"},
		{"unprivileged user", "cannot create snapshots in 'cache/appdata': permission denied", exitStatus(1), "zfs-permission"},
		{"key rejected", "root@nas.lan: Permission denied (publickey,password).", exitStatus(255), "ssh-auth"},
		{"port closed", "ssh: connect to host nas.lan port 22: Connection refused", exitStatus(255), "ssh-unreachable"},
		{"no route", "ssh: connect to host 192.168.1.9 port 22: No route to host", exitStatus(255), "ssh-unreachable"},
		{"unknown name", "ssh: Could not resolve hostname nas.lan: Name or service not known", exitStatus(255), "ssh-unreachable"},
		{"anything else", "cannot hold snapshot: out of space", exitStatus(1), "zfs-error"},
		{"no stderr at all", "", errors.New("context deadline exceeded"), "zfs-error"},
	}
	for _, c := range cases {
		if got := Classify(c.stderr, c.err); got != c.want {
			t.Errorf("%s: Classify = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestIsBusyAndIsNotFound(t *testing.T) {
	busy := &CmdError{Code: Classify("cannot destroy: dataset is busy", exitStatus(1))}
	gone := &CmdError{Code: Classify("cannot open 'x': dataset does not exist", exitStatus(1))}
	swept := &CmdError{Code: Classify("could not find any snapshots to destroy; check snapshot names.", exitStatus(1))}
	other := &CmdError{Code: Classify("out of space", exitStatus(1))}

	if !IsBusy(busy) || IsBusy(gone) || IsBusy(other) || IsBusy(nil) {
		t.Error("IsBusy is true for something other than a busy dataset")
	}
	if !IsNotFound(gone) || !IsNotFound(swept) {
		t.Error("IsNotFound missed a dataset or a snapshot that is already gone")
	}
	if IsNotFound(busy) || IsNotFound(other) || IsNotFound(nil) {
		t.Error("IsNotFound is true for a failure that left something behind")
	}
}

func TestCmdErrorKeepsStderrShortAndUnwraps(t *testing.T) {
	inner := errors.New("exit status 1")
	e := &CmdError{
		Args:   []string{"zfs", "destroy", "-r", "cache/appdata@bombvault-20260917031500"},
		Stderr: "cannot destroy 'cache/appdata@bombvault-20260917031500': dataset is busy",
		Code:   "busy",
		Err:    inner,
	}
	if !errors.Is(e, inner) {
		t.Error("CmdError does not unwrap to the transport error")
	}
	if got := e.Error(); len(got) > 600 || got == "" {
		t.Errorf("Error() = %q", got)
	}

	long := &CmdError{Code: "zfs-error", Stderr: strings.Repeat("x", 4096)}
	if len(long.Error()) > 600 {
		t.Errorf("Error() of a flood is %d bytes", len(long.Error()))
	}
}
