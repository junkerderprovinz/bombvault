package zfsrepl

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/sshconn"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

type exitRunner struct{ calls [][]string }

// RunCapture answers like a host whose zfs only lives under /usr/sbin.
func (f *exitRunner) RunCapture(_ context.Context, args ...string) (string, string, error) {
	f.calls = append(f.calls, args)
	if args[0] == "zfs" {
		return "", "bash: zfs: command not found", exitStatus(127)
	}
	return "zfs-2.4.3-1", "", nil
}

type stubStreamer struct {
	sent, received []string
	sendErr        error
}

func (s *stubStreamer) StreamCommand(_ context.Context, args ...string) (io.ReadCloser, func() error, error) {
	s.sent = args
	return io.NopCloser(strings.NewReader("")), func() error { return s.sendErr }, nil
}

func (s *stubStreamer) RunWithStdin(_ context.Context, _ io.Reader, args ...string) error {
	s.received = args
	return &sshconn.RemoteError{Cmd: args[0], Stderr: "cannot receive incremental stream: destination t has been modified\nsince most recent snapshot", Err: exitStatus(1)}
}

func TestSSHEndStreamsWithTheBinaryTheHostAnsweredTo(t *testing.T) {
	host := zfs.NewSSHHost(&exitRunner{})
	if _, err := host.Run(context.Background(), zfs.VersionArgs()); err != nil {
		t.Fatal(err)
	}
	conn := &stubStreamer{sendErr: &sshconn.RemoteError{Cmd: "/usr/sbin/zfs", Stderr: "cannot resume send: 'a@b' used in the initial send no longer exists", Err: exitStatus(255)}}
	end := NewSSHEnd(host, conn)

	_, wait, err := end.Send(context.Background(), []string{"zfs", "send", "-t", "1-ab-12-cd"})
	if err != nil {
		t.Fatal(err)
	}
	if err := wait(); Code(err) != "resume-token-stale" {
		t.Errorf("a stale token = %v, want resume-token-stale", err)
	}
	if conn.sent[0] != "/usr/sbin/zfs" {
		t.Errorf("send ran %q", conn.sent)
	}

	err = end.Receive(context.Background(), []string{"zfs", "receive", "-s", "-u", "t"}, strings.NewReader(""))
	var ce *zfs.CmdError
	if !errors.As(err, &ce) || ce.Code != "target-changed" || !strings.Contains(ce.Stderr, "has been modified") {
		t.Errorf("a refused receive = %v, want target-changed with the host's words", err)
	}
	if conn.received[0] != "/usr/sbin/zfs" {
		t.Errorf("receive ran %q", conn.received)
	}
}
