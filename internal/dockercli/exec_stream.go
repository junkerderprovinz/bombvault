package dockercli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/junkerderprovinz/bombvault/internal/dbdump"
)

var _ dbdump.Execer = (*Client)(nil)

// ExecStream runs cmd inside a running container, copies its output out as it
// arrives and returns the process's real exit code. Nothing is buffered: the
// stdout of a dump is the backup stream itself.
func (c *Client) ExecStream(ctx context.Context, name string, cmd []string, stdout, stderr io.Writer) (int, error) {
	id, att, err := c.startExec(ctx, name, container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return 0, err
	}
	defer att.Close()

	if err := drainExecStreams(ctx, att.Reader, att.Close, stdout, stderr); err != nil {
		return 0, fmt.Errorf("dockercli: exec stream: %w", err)
	}
	return c.execExit(ctx, id)
}

// ExecQuick runs a short command whose output does not matter. It is the name
// dbdump.Execer asks for; the orphan stop is its only caller.
func (c *Client) ExecQuick(ctx context.Context, name string, cmd []string) error {
	return c.Exec(ctx, name, cmd)
}

// ExecOutput runs a short command and returns the first max bytes of its
// stdout together with the real exit code. The rest of the output is read and
// discarded so the command can finish.
func (c *Client) ExecOutput(ctx context.Context, name string, cmd []string, max int) (string, int, error) {
	id, att, err := c.startExec(ctx, name, container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return "", 0, err
	}
	defer att.Close()

	stdout, _, err := capturedOutput(ctx, att.Reader, att.Close, max)
	if err != nil {
		return "", 0, fmt.Errorf("dockercli: exec output: %w", err)
	}
	code, err := c.execExit(ctx, id)
	return stdout, code, err
}

// ExecStdin runs cmd with stdin attached, keeps the last tailMax bytes of its
// stderr for the caller's error report and returns the real exit code.
func (c *Client) ExecStdin(ctx context.Context, name string, cmd []string, stdin io.Reader, tailMax int) (string, int, error) {
	id, att, err := c.startExec(ctx, name, container.ExecOptions{
		Cmd:          cmd,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return "", 0, err
	}

	tail := &tailBuffer{max: tailMax}
	if err := feedExecStdin(ctx, hijackedAttach{resp: &att}, stdin, tail); err != nil {
		return "", 0, fmt.Errorf("dockercli: exec stdin: %w", err)
	}
	code, err := c.execExit(ctx, id)
	return tail.String(), code, err
}

func (c *Client) startExec(ctx context.Context, name string, opts container.ExecOptions) (string, types.HijackedResponse, error) {
	created, err := c.api.ContainerExecCreate(ctx, name, opts)
	if err != nil {
		return "", types.HijackedResponse{}, fmt.Errorf("dockercli: exec create: %w", mapExecCreateErr(err))
	}
	att, err := c.api.ContainerExecAttach(ctx, created.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", types.HijackedResponse{}, fmt.Errorf("dockercli: exec attach: %w", err)
	}
	return created.ID, att, nil
}

func (c *Client) execExit(ctx context.Context, id string) (int, error) {
	code, err := waitExecExit(ctx, func(ctx context.Context) (bool, int, error) {
		insp, err := c.api.ContainerExecInspect(ctx, id)
		return insp.Running, insp.ExitCode, err
	}, execExitPoll, execExitWait)
	if err != nil {
		return 0, fmt.Errorf("dockercli: exec inspect: %w", err)
	}
	return code, nil
}

// mapExecCreateErr names the 409 the daemon answers with when the container is
// paused, restarting or stopped, so a dump can report that instead of a
// nondescript docker error.
func mapExecCreateErr(err error) error {
	if cerrdefs.IsConflict(err) {
		return fmt.Errorf("%w: %w", dbdump.ErrNotRunning, err)
	}
	return err
}

// drainExecStreams demultiplexes an exec attach into stdout and stderr without
// a cap and reports a failed write, because for a dump the write is the backup.
// The attach is a hijacked connection whose Read ignores the context, so
// closing it when the context ends is what unblocks the read.
func drainExecStreams(ctx context.Context, r io.Reader, closeAttach func(), stdout, stderr io.Writer) error {
	stop := context.AfterFunc(ctx, closeAttach)
	defer stop()

	_, err := stdcopy.StdCopy(stdout, stderr, r)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return err
}

// capturedOutput reads an exec attach to the end, keeping at most max bytes of
// each stream.
func capturedOutput(ctx context.Context, r io.Reader, closeAttach func(), max int) (string, string, error) {
	stdout, stderr := &cappedBuffer{max: max}, &cappedBuffer{max: max}
	if err := drainExecStreams(ctx, r, closeAttach, stdout, stderr); err != nil {
		return "", "", err
	}
	return stdout.String(), stderr.String(), nil
}

// feedExecStdin copies stdin into the attach, half-closes the write side at the
// end of the input and keeps the command's stderr. The attach is closed before
// the copy is waited for: a command that exits early leaves the write blocked,
// and only the close releases it. An input that fails never ends, so the attach
// is closed at once and the input's error is the one reported.
func feedExecStdin(ctx context.Context, conn execConn, stdin io.Reader, stderr io.Writer) error {
	src := &sourceReader{r: stdin}
	fed := make(chan struct{})
	go func() {
		defer close(fed)
		if _, err := io.Copy(conn, src); err != nil {
			conn.Close()
			return
		}
		_ = conn.CloseWrite()
	}()

	err := drainExecStreams(ctx, conn, conn.Close, io.Discard, stderr)
	conn.Close()
	<-fed
	if src.err != nil {
		return fmt.Errorf("read the input: %w", src.err)
	}
	return err
}

// sourceReader keeps the input's own read error apart from a failed write
// into an attach that was closed on purpose.
type sourceReader struct {
	r   io.Reader
	err error
}

func (s *sourceReader) Read(p []byte) (int, error) {
	n, err := s.r.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		s.err = err
	}
	return n, err
}

// execConn is an exec attach as the stdin feed uses it: output to read, stdin
// to write, a half-close for the end of the input and a close for both.
type execConn interface {
	io.Reader
	io.Writer
	CloseWrite() error
	Close()
}

type hijackedAttach struct{ resp *types.HijackedResponse }

func (h hijackedAttach) Read(p []byte) (int, error)  { return h.resp.Reader.Read(p) }
func (h hijackedAttach) Write(p []byte) (int, error) { return h.resp.Conn.Write(p) }
func (h hijackedAttach) CloseWrite() error           { return h.resp.CloseWrite() }
func (h hijackedAttach) Close()                      { h.resp.Close() }

// cappedBuffer keeps the first max bytes written to it and swallows the rest.
type cappedBuffer struct {
	max int
	buf bytes.Buffer
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	if room := c.max - c.buf.Len(); room > 0 {
		c.buf.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}

func (c *cappedBuffer) String() string { return c.buf.String() }

// tailBuffer keeps the last max bytes written to it, which is where a failing
// command says what went wrong.
type tailBuffer struct {
	max int
	buf []byte
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if len(p) > t.max {
		p = p[len(p)-t.max:]
	}
	t.buf = append(t.buf, p...)
	if extra := len(t.buf) - t.max; extra > 0 {
		t.buf = t.buf[:copy(t.buf, t.buf[extra:])]
	}
	return n, nil
}

func (t *tailBuffer) String() string { return string(t.buf) }
