package api

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"time"
)

// hostShellTimeout bounds a single hook command, so a hung command cannot
// stall a "Backup Everything" pass.
const hostShellTimeout = 5 * time.Minute

// hostShellWaitDelay is how long Wait may block after the timeout has killed
// sh before Go closes the pipes. Without it, a background child that inherited
// a pipe would keep Wait blocked past hostShellTimeout.
const hostShellWaitDelay = 10 * time.Second

// HostShell runs the global pre and post hooks of a "Backup Everything" pass.
// They run in BombVault's own container, because a whole pass has no single
// container to exec into. Unlike a per-container pre-hook, a global hook
// protects no snapshot, so callers log its error and carry on with the backup.
type HostShell interface {
	Run(ctx context.Context, cmd string) error
}

// execHostShell runs hooks with `sh -c`.
type execHostShell struct{}

var _ HostShell = execHostShell{}

// Run executes cmd with `sh -c` under hostShellTimeout and logs its output if
// it fails.
func (execHostShell) Run(ctx context.Context, cmd string) error {
	ctx, cancel := context.WithTimeout(ctx, hostShellTimeout)
	defer cancel()

	// cmd is a hook the operator stored in Settings, and running arbitrary
	// commands is the feature. It can only be written through PUT
	// /api/settings, which is same-site (csrfGate) and behind the login when
	// one is set; the settings import strips both hook fields. Any new way of
	// writing them needs the same protection.
	c := exec.CommandContext(ctx, "sh", "-c", cmd) //nolint:gosec // G204: an operator-configured hook command is the feature, see above
	configureHookProcGroup(c)

	// Capped so a hook flooding stdout cannot grow memory without bound.
	var out cappedBuffer
	out.limit = hostShellOutputCap
	c.Stdout = &out
	c.Stderr = &out
	err := c.Run()
	if err != nil {
		log.Printf("api: host shell hook failed (best-effort, backup continues): %v; output: %s", err, out.String())
	}
	return err
}

// hostShellOutputCap is how much hook output is kept, the same cap
// dockercli.go uses for per-container hooks.
const hostShellOutputCap = 64 << 10

// cappedBuffer keeps the first limit bytes and discards the rest. Discarded
// writes still report success, because a short write would close the pipe and
// fail a chatty hook with EPIPE.
type cappedBuffer struct {
	buf     []byte
	limit   int
	dropped int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if room := b.limit - len(b.buf); room > 0 {
		if len(p) <= room {
			b.buf = append(b.buf, p...)
		} else {
			b.buf = append(b.buf, p[:room]...)
			b.dropped += len(p) - room
		}
	} else {
		b.dropped += len(p)
	}
	return len(p), nil
}

func (b *cappedBuffer) String() string {
	if b.dropped == 0 {
		return string(b.buf)
	}
	return fmt.Sprintf("%s… (%d more bytes dropped)", b.buf, b.dropped)
}
