package api

// HostShell runs a shell command in BombVault's OWN container — the seam
// behind the "Backup Everything" global pre/post hooks (design spec:
// docs/superpowers/specs/2026-08-20-backup-everything-design.md, decision 6).
// Unlike the existing per-container PreHook/PostHook, which exec INSIDE the
// target container via the Docker API (internal/backup/orchestrator.go's
// `Docker.Exec(ctx, ref, []string{"sh", "-c", d.PreHook})`), a whole-pass hook
// has no single target container to exec into, so it runs as a local shell
// command here instead.

import (
	"context"
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
	c.WaitDelay = hostShellWaitDelay
	out, err := c.CombinedOutput()
	if err != nil {
		log.Printf("api: host shell hook failed (best-effort, backup continues): %v; output: %s", err, out)
	}
	return err
}
