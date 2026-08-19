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

	c := exec.CommandContext(ctx, "sh", "-c", cmd) //nolint:gosec // G204: cmd is operator-configured via Settings (EverythingPreHook/EverythingPostHook), not request/user input — same trust model as the existing per-container hook's Docker.Exec(ctx, ref, []string{"sh", "-c", d.PreHook}) in internal/backup/orchestrator.go; see also internal/restic/restic.go:823's identical justification for restic's own argv construction.
	out, err := c.CombinedOutput()
	if err != nil {
		log.Printf("api: host shell hook failed (best-effort, backup continues): %v; output: %s", err, out)
	}
	return err
}
