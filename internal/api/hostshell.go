package api

// HostShell runs a shell command in BombVault's OWN container — the seam
// behind the "Backup Everything" global pre/post hooks (design spec:
// the design notes, decision 6).
// Unlike the existing per-container PreHook/PostHook, which exec INSIDE the
// target container via the Docker API (internal/backup/orchestrator.go's
// `Docker.Exec(ctx, ref, []string{"sh", "-c", d.PreHook})`), a whole-pass hook
// has no single target container to exec into, so it runs as a local shell
// command here instead.

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"time"
)

// hostShellTimeout bounds how long a single hook command may run before it is
// killed: generous for a healthcheck ping, short enough that a hung command
// can't wedge a "Backup Everything" pass indefinitely.
const hostShellTimeout = 5 * time.Minute

// hostShellWaitDelay bounds how long CombinedOutput's underlying cmd.Wait may
// block AFTER hostShellTimeout kills the direct "sh" child, before Go force-
// closes the stdout/stderr pipes and returns anyway. Without this, a hook
// command that backgrounds a grandchild (e.g. `some-daemon &`) which inherits
// the pipe would keep Wait() blocked past hostShellTimeout even though "sh"
// itself was already killed — silently defeating the "can't wedge a pass
// indefinitely" contract above. Mirrors internal/restic/proc_unix.go and
// proc_windows.go's identical resticWaitDelay guard, which exists for the
// same reason (see also the Dockerfile's tini comment: this container has
// already hit orphaned-grandchild-process issues with restic's rclone child).
const hostShellWaitDelay = 10 * time.Second

// HostShell is the DI seam a global hook command runs through. Run is
// BEST-EFFORT by contract: unlike the per-container pre-hook (which aborts
// the backup on failure, protecting snapshot consistency), a global hook has
// no snapshot-consistency contract to protect, so callers must never let a
// failing or hanging command block or fail a backup — they may log the
// returned error but must not propagate it as a backup failure.
type HostShell interface {
	Run(ctx context.Context, cmd string) error
}

// execHostShell is the real HostShell, shelling cmd out via `sh -c`.
type execHostShell struct{}

var _ HostShell = execHostShell{}

// Run executes cmd via `sh -c` under a fixed hostShellTimeout, capturing
// combined stdout+stderr and logging them on failure. See the HostShell
// interface doc: the returned error is for the caller's own logging/
// bookkeeping only — it must never be treated as fatal to a backup.
func (execHostShell) Run(ctx context.Context, cmd string) error {
	ctx, cancel := context.WithTimeout(ctx, hostShellTimeout)
	defer cancel()

	// G204 is real and accepted, not absent. cmd is a shell command the operator
	// stores in Settings (EverythingPreHook/EverythingPostHook) precisely so it
	// can run arbitrary commands — that IS the feature, the same trust model as
	// the per-container hook's Docker.Exec(ctx, ref, []string{"sh", "-c",
	// d.PreHook}) in internal/backup/orchestrator.go and restic.go's own argv
	// construction. What makes it acceptable is not that the string never came
	// from a request (it reaches Settings through PUT /api/settings, so it did —
	// an earlier version of this comment claimed otherwise and was simply wrong),
	// but that the write path is the one an operator uses to configure the
	// instance: same-site only (csrfGate), and behind the login when one is set.
	// The settings IMPORT path deliberately strips these two fields instead
	// (settings_portable.go), because an imported file is not an operator sitting
	// at the UI. Anything else that ever comes to write them needs the same
	// question asked of it.
	c := exec.CommandContext(ctx, "sh", "-c", cmd) //nolint:gosec // G204: see the comment above — an operator-configured hook command is the feature.
	// Kills the whole process group, not just `sh` — a hook is routinely a
	// pipeline or an `&&` chain, whose children would otherwise be orphaned
	// inside this container when the command overruns. Also sets WaitDelay.
	configureHookProcGroup(c)

	// Output is CAPPED rather than collected wholesale. CombinedOutput() would
	// buffer stdout+stderr unbounded for up to five minutes; the sibling
	// primitive for the per-container hook (dockercli.go) caps at the same 64
	// KiB with the comment "a hook flooding stdout cannot balloon memory", and
	// this file names that path as its model. Only a short tail is ever logged,
	// so the rest was never wanted in the first place.
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

// hostShellOutputCap bounds how much hook output is kept in memory, matching
// dockercli.go's cap for the per-container hook exactly.
const hostShellOutputCap = 64 << 10

// cappedBuffer accepts writes until limit bytes have been kept, then discards
// the rest while still REPORTING them as written — a short write would make
// exec close the pipe and hand the hook an EPIPE, turning a chatty command into
// a failed one. Discarding keeps the command's own fate untouched.
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
