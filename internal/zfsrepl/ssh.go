package zfsrepl

import (
	"context"
	"errors"
	"io"

	"github.com/junkerderprovinz/bombvault/internal/sshconn"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// Streamer is the part of an sshconn.Conn a zfs stream travels over.
type Streamer interface {
	StreamCommand(ctx context.Context, args ...string) (io.ReadCloser, func() error, error)
	RunWithStdin(ctx context.Context, rd io.Reader, args ...string) error
}

var _ Streamer = (*sshconn.Conn)(nil)

// SSHEnd is an End on a host reached over SSH: the host's own connection for
// one side, an sshconn.NewIsolated one for the other. host runs the short
// commands and has learned which zfs binary answers there, which the streams
// then use too.
type SSHEnd struct {
	host *zfs.SSHHost
	conn Streamer
}

var _ End = (*SSHEnd)(nil)

// NewSSHEnd returns an End whose commands go through host and whose streams go
// through conn, both on the same machine.
func NewSSHEnd(host *zfs.SSHHost, conn Streamer) *SSHEnd {
	return &SSHEnd{host: host, conn: conn}
}

func (e *SSHEnd) Run(ctx context.Context, args []string) (string, error) {
	return e.host.Run(ctx, args)
}

func (e *SSHEnd) Send(ctx context.Context, args []string) (io.ReadCloser, func() error, error) {
	argv := e.host.Argv(args)
	out, wait, err := e.conn.StreamCommand(ctx, argv...)
	if err != nil {
		return nil, nil, &zfs.CmdError{Args: argv, Code: "zfs-error", Err: err}
	}
	return out, func() error { return remoteError(argv, wait()) }, nil
}

func (e *SSHEnd) Receive(ctx context.Context, args []string, stream io.Reader) error {
	argv := e.host.Argv(args)
	return remoteError(argv, e.conn.RunWithStdin(ctx, stream, argv...))
}

// remoteError gives a failed stream the reason code a failed Run would carry.
func remoteError(argv []string, err error) error {
	if err == nil {
		return nil
	}
	var re *sshconn.RemoteError
	stderr := ""
	if errors.As(err, &re) {
		stderr = re.Stderr
	}
	return &zfs.CmdError{Args: argv, Stderr: stderr, Code: zfs.Classify(stderr, err), Err: err}
}
