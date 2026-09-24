package zfs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/sshconn"
)

// The connection the domain runs on has to fit the seam the fake stands in for.
var _ Runner = (*sshconn.Conn)(nil)

type fakeRunner struct {
	calls [][]string
	reply func(call int, args []string) (stdout, stderr string, err error)
}

func (f *fakeRunner) RunCapture(_ context.Context, args ...string) (string, string, error) {
	f.calls = append(f.calls, append([]string(nil), args...))
	if f.reply == nil {
		return "", "", nil
	}
	return f.reply(len(f.calls)-1, args)
}

func TestSSHHostKeepsStderrInCmdError(t *testing.T) {
	stderr := "cannot open 'cache/nope': dataset does not exist"
	f := &fakeRunner{reply: func(int, []string) (string, string, error) {
		return "", stderr, exitStatus(1)
	}}
	h := NewSSHHost(f)

	_, err := h.Tree(context.Background(), "cache/nope")
	var ce *CmdError
	if !errors.As(err, &ce) {
		t.Fatalf("Tree error = %v, want a CmdError", err)
	}
	if ce.Stderr != stderr {
		t.Errorf("stderr = %q, want %q", ce.Stderr, stderr)
	}
	if ce.Code != "not-found" || !IsNotFound(err) {
		t.Errorf("code = %q", ce.Code)
	}
	if len(ce.Args) == 0 || ce.Args[0] != "zfs" {
		t.Errorf("the argv is not on the error: %q", ce.Args)
	}
	if !strings.Contains(err.Error(), "dataset does not exist") {
		t.Errorf("Error() = %q, want the host's own message", err.Error())
	}
}

func TestSSHHostRefusesACutListing(t *testing.T) {
	// The cut falls on a line boundary, so what arrives parses cleanly and only
	// the transport knows that datasets are missing.
	f := &fakeRunner{reply: func(int, []string) (string, string, error) {
		return treeFixture, "", fmt.Errorf("sshconn: run %q: %w", "zfs", sshconn.ErrStdoutCut)
	}}
	h := NewSSHHost(f)

	for name, list := range map[string]func() error{
		"tree": func() error { _, err := h.Tree(context.Background(), "cache/appdata"); return err },
		"list": func() error { _, err := h.List(context.Background()); return err },
		"snapshots": func() error {
			_, err := h.Snapshots(context.Background(), "cache/appdata")
			return err
		},
	} {
		err := list()
		var ce *CmdError
		if !errors.As(err, &ce) || ce.Code != "zfs-error" || !strings.Contains(err.Error(), "16 MiB") {
			t.Errorf("%s of a cut listing = %v, want a zfs-error naming the limit", name, err)
		}
	}
}

func TestSSHHostDestroyRefusesForeignSnapshotWithoutRunning(t *testing.T) {
	f := &fakeRunner{}
	h := NewSSHHost(f)
	ctx := context.Background()

	for _, snap := range []string{"autosnap_2026-09-17_03:00:00_hourly", "bombvault-prerestore-20260917031500", ""} {
		if err := h.DestroyRecursive(ctx, "cache/appdata", snap); err == nil {
			t.Errorf("DestroyRecursive took %q", snap)
		}
	}
	if err := h.DestroySafety(ctx, "cache/appdata", "bombvault-20260917031500"); err == nil {
		t.Error("DestroySafety took a backup snapshot name")
	}
	if len(f.calls) != 0 {
		t.Fatalf("a refused name still reached the host: %q", f.calls)
	}
}

func TestSSHHostRetriesWithUsrSbinZfsOn127(t *testing.T) {
	f := &fakeRunner{reply: func(call int, args []string) (string, string, error) {
		if args[0] == "zfs" {
			return "", "bash: line 1: zfs: command not found", exitStatus(127)
		}
		return "zfs-2.3.4-1", "", nil
	}}
	h := NewSSHHost(f)
	ctx := context.Background()

	got, err := h.Version(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != "zfs-2.3.4-1" {
		t.Errorf("Version = %q", got)
	}
	if len(f.calls) != 2 {
		t.Fatalf("got %d calls, want the first one and its retry", len(f.calls))
	}
	if f.calls[0][0] != "zfs" || f.calls[1][0] != "/usr/sbin/zfs" {
		t.Fatalf("retry argv = %q", f.calls)
	}
	if !equalArgs(f.calls[0][1:], f.calls[1][1:]) {
		t.Errorf("the retry changed more than the binary: %q", f.calls)
	}

	if _, err := h.Version(ctx); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 3 || f.calls[2][0] != "/usr/sbin/zfs" {
		t.Errorf("the working binary was not remembered: %q", f.calls)
	}
}

func TestSSHHostParsesWhatEachCommandReturns(t *testing.T) {
	f := &fakeRunner{reply: func(call int, args []string) (string, string, error) {
		switch {
		case args[1] == "list" && contains(args, "snapshot"):
			return "cache/appdata@bombvault-20260917031500\t1789629300\t0\n", "", nil
		case args[1] == "list":
			return treeFixture, "", nil
		}
		return "", "", nil
	}}
	h := NewSSHHost(f)
	ctx := context.Background()

	tree, err := h.Tree(ctx, "cache/appdata")
	if err != nil || len(tree) != 9 {
		t.Fatalf("Tree = %d entries, %v", len(tree), err)
	}
	snaps, err := h.Snapshots(ctx, "cache/appdata")
	if err != nil || len(snaps) != 1 || snaps[0].Name != "bombvault-20260917031500" {
		t.Fatalf("Snapshots = %+v, %v", snaps, err)
	}
	if err := h.SnapshotRecursive(ctx, "cache/appdata", "bombvault-20260917031500"); err != nil {
		t.Fatal(err)
	}
	if err := h.Prime(ctx, "/mnt/cache/appdata", "bombvault-20260917031500"); err != nil {
		t.Fatal(err)
	}

	last := f.calls[len(f.calls)-1]
	if last[0] != "stat" {
		t.Errorf("Prime argv = %q, want a stat of the snapshot directory", last)
	}
}
