// Package backup orchestrates container backup and restore. It is the
// security-critical core and is dependency-injected: it imports ONLY the
// interfaces defined here (Docker, Restic, Templates, Runs) and never the
// concrete dockercli/restic packages, so it is fully unit-testable with fakes.
//
// Ported safeguards (SEC parity with the TypeScript implementation):
//   - Backup ALWAYS restarts the container, even if the backup fails.
//   - Restore is gated on an explicit confirmation flag.
//   - Restore validates the snapshot id against a strict hex pattern
//     (arg-injection guard; restic also gets a `--` end-of-options guard).
//   - Restore re-inspects the LIVE container by name and aborts on a name
//     mismatch before the destructive stop/remove (wrong-target guard).
//   - Restore targets "/" so backed-up absolute appdata paths land at origin.
//   - The recreated container preserves the original's security-relevant fields
//     (delegated to Docker.CreateAndStart, which maps them).
package backup

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// DI types & interfaces (the seam — no concrete adapters imported)
// ---------------------------------------------------------------------------

// Inspect is the captured container definition used to recreate a container on
// restore. It mirrors dockercli.ContainerInspect structurally so the real
// adapter can convert between them without this package importing dockercli.
type Inspect struct {
	Name  string // may carry a leading slash (e.g. "/plex")
	Image string
}

// Summary is the result of a successful backup.
type Summary struct {
	SnapshotID string
	Bytes      int64
}

// Docker is the subset of host control the orchestrator needs.
type Docker interface {
	Stop(ctx context.Context, name string, timeout time.Duration) error
	Start(ctx context.Context, name string) error
	Remove(ctx context.Context, name string) error
	Pull(ctx context.Context, image string) error
	CreateAndStart(ctx context.Context, in Inspect) error
	// InspectName returns the live container's name (the adapter normalizes it),
	// or "" when no such container exists.
	InspectName(ctx context.Context, name string) (string, error)
}

// Restic is the subset of the backup engine the orchestrator needs.
type Restic interface {
	Backup(ctx context.Context, repo string, paths, tags []string) (Summary, error)
	Restore(ctx context.Context, repo, snapshotID, target string) error
}

// Templates reads and writes Unraid container templates.
type Templates interface {
	Read(dir, name string) (string, bool)
	Write(dir, name, xml string) error
}

// Runs records the lifecycle of a backup/restore run.
type Runs interface {
	Start(targetID, kind string) (runID string, err error)
	Finish(runID, status, snapshotID string, bytes int64, errMsg string) error
}

// ---------------------------------------------------------------------------
// BackupDeps / RestoreDeps
// ---------------------------------------------------------------------------

// BackupDeps bundles everything BackupContainer needs.
type BackupDeps struct {
	// ContainerRef is the name/id used for stop/start and the `container:<ref>` tag.
	ContainerRef string
	// ContainerName is the display name used for the template filename.
	ContainerName string
	// RepoPath is the local restic repository path.
	RepoPath string
	// AppdataPaths are the paths included in the backup.
	AppdataPaths []string
	// StopTimeout is how long to wait for a graceful stop before SIGKILL.
	StopTimeout time.Duration
	// TargetID is the run-recording target id.
	TargetID string
	// SnapshotTemplatesDir is where per-snapshot template copies are written.
	SnapshotTemplatesDir string
	// FlashTemplatesDir is where the live Unraid templates are read from.
	FlashTemplatesDir string

	Docker    Docker
	Restic    Restic
	Templates Templates
	Runs      Runs
}

// RestoreDeps bundles everything RestoreContainer needs.
type RestoreDeps struct {
	// Confirmed MUST be true — guard against an accidental destructive restore.
	Confirmed bool
	// ContainerRef is the name/id to stop/remove and re-inspect.
	ContainerRef string
	// ContainerName is the expected live name (wrong-target guard) and the
	// template filename.
	ContainerName string
	// RepoPath is the local restic repository path.
	RepoPath string
	// SnapshotID is the snapshot to restore (validated hex).
	SnapshotID string
	// RestoreTargetDir is the restic --target; MUST be "/" so absolute appdata
	// paths land back at origin (SEC: never double-nest under an appdata root).
	RestoreTargetDir string
	// TemplateXML is the captured template flashed back on restore.
	TemplateXML string
	// FlashTemplatesDir is where the live Unraid templates live.
	FlashTemplatesDir string
	// Inspect is the captured definition used to recreate the container.
	Inspect Inspect
	// TargetID is the run-recording target id.
	TargetID string

	Docker    Docker
	Restic    Restic
	Templates Templates
	Runs      Runs
}

// ---------------------------------------------------------------------------
// constants & validation
// ---------------------------------------------------------------------------

const (
	kindBackup  = "backup"
	kindRestore = "restore"

	statusSuccess = "success"
	statusFailed  = "failed"

	defaultStopTimeout = 30 * time.Second
)

// snapshotIDRe matches a restic short or full snapshot id (8–64 lowercase hex).
var snapshotIDRe = regexp.MustCompile(`^[0-9a-f]{8,64}$`)

// normalizeName strips a single leading slash from a docker container name.
func normalizeName(n string) string {
	return strings.TrimPrefix(n, "/")
}

// ---------------------------------------------------------------------------
// BackupContainer
// ---------------------------------------------------------------------------

// BackupContainer orchestrates a container backup:
//
//	recordRunStart
//	→ stop → restic backup → capture+persist template
//	→ FINALLY always start (even on error)
//	→ recordRunFinish(success|failed)
//	→ re-throw on failure
//
// The container is GUARANTEED to restart even if the backup throws.
func BackupContainer(ctx context.Context, d BackupDeps) (Summary, error) {
	runID, err := d.Runs.Start(d.TargetID, kindBackup)
	if err != nil {
		return Summary{}, fmt.Errorf("backup: record run start: %w", err)
	}

	stopTimeout := d.StopTimeout
	if stopTimeout <= 0 {
		stopTimeout = defaultStopTimeout
	}
	tags := []string{"container:" + d.ContainerRef, "p1"}

	var (
		summary     Summary
		backupErr   error
		summarySeen bool
	)

	func() {
		// Always restart, even if anything below throws.
		defer func() {
			if startErr := d.Docker.Start(ctx, d.ContainerRef); startErr != nil && backupErr == nil {
				backupErr = fmt.Errorf("backup: restart container: %w", startErr)
			}
		}()

		if backupErr = d.Docker.Stop(ctx, d.ContainerRef, stopTimeout); backupErr != nil {
			backupErr = fmt.Errorf("backup: stop container: %w", backupErr)
			return
		}

		summary, backupErr = d.Restic.Backup(ctx, d.RepoPath, d.AppdataPaths, tags)
		if backupErr != nil {
			return
		}
		summarySeen = true

		// Capture and persist the live Unraid template alongside the snapshot.
		// Templates.Write prepends "my-", so the snapshot copy lands at
		// "my-<snapshotID>-<ContainerName>.xml" (a single, snapshot-scoped name).
		if xml, ok := d.Templates.Read(d.FlashTemplatesDir, d.ContainerName); ok {
			snapName := summary.SnapshotID + "-" + d.ContainerName
			if backupErr = d.Templates.Write(d.SnapshotTemplatesDir, snapName, xml); backupErr != nil {
				backupErr = fmt.Errorf("backup: persist template: %w", backupErr)
				return
			}
		}
	}()

	if backupErr != nil {
		_ = d.Runs.Finish(runID, statusFailed, "", 0, scrub(backupErr))
		return Summary{}, backupErr
	}

	snap := ""
	if summarySeen {
		snap = summary.SnapshotID
	}
	if err := d.Runs.Finish(runID, statusSuccess, snap, summary.Bytes, ""); err != nil {
		return summary, fmt.Errorf("backup: record run finish: %w", err)
	}
	return summary, nil
}

// ---------------------------------------------------------------------------
// RestoreContainer
// ---------------------------------------------------------------------------

// RestoreContainer orchestrates a container restore:
//
//	guard Confirmed==true → validate snapshot id
//	→ recordRunStart
//	→ InspectName live re-check (abort on name mismatch, wrong-target guard)
//	→ pull → stop (ignore absent) → remove (ignore absent)
//	→ restic restore --target "/" → write template → CreateAndStart
//	→ recordRunFinish(success|failed)
//
// Returns an error WITHOUT recording a run when not confirmed or the snapshot
// id is invalid (nothing destructive has happened yet).
func RestoreContainer(ctx context.Context, d RestoreDeps) error {
	if !d.Confirmed {
		return errors.New("restore: not confirmed (confirm must be true)")
	}
	if !snapshotIDRe.MatchString(d.SnapshotID) {
		return errors.New("restore: invalid snapshot id (must be 8–64 lowercase hex)")
	}

	runID, err := d.Runs.Start(d.TargetID, kindRestore)
	if err != nil {
		return fmt.Errorf("restore: record run start: %w", err)
	}

	restoreErr := runRestore(ctx, d)
	if restoreErr != nil {
		_ = d.Runs.Finish(runID, statusFailed, "", 0, scrub(restoreErr))
		return restoreErr
	}

	if err := d.Runs.Finish(runID, statusSuccess, d.SnapshotID, 0, ""); err != nil {
		return fmt.Errorf("restore: record run finish: %w", err)
	}
	return nil
}

// runRestore performs the destructive restore sequence after the guards pass.
func runRestore(ctx context.Context, d RestoreDeps) error {
	// Wrong-target guard: re-verify the LIVE container matches the target before
	// the destructive stop/remove. A missing container ("") is fine — a fresh
	// restore recreates it. Any other mismatch aborts.
	liveName, err := d.Docker.InspectName(ctx, d.ContainerRef)
	if err != nil {
		return fmt.Errorf("restore: inspect live container: %w", err)
	}
	if liveName != "" && normalizeName(liveName) != normalizeName(d.ContainerName) {
		return fmt.Errorf(
			"restore aborted: live container %q does not match target %q",
			normalizeName(liveName), normalizeName(d.ContainerName),
		)
	}

	// Pull the image before touching the running container.
	if err := d.Docker.Pull(ctx, d.Inspect.Image); err != nil {
		return fmt.Errorf("restore: pull image: %w", err)
	}

	// Stop & remove the existing container — absent/already-stopped is acceptable.
	if err := d.Docker.Stop(ctx, d.ContainerRef, defaultStopTimeout); err != nil {
		// ignore: container may be absent or already stopped
		_ = err
	}
	if err := d.Docker.Remove(ctx, d.ContainerRef); err != nil {
		// ignore: container may be absent
		_ = err
	}

	// Restore appdata from the snapshot to "/" (SEC: absolute paths land at origin).
	if err := d.Restic.Restore(ctx, d.RepoPath, d.SnapshotID, d.RestoreTargetDir); err != nil {
		return fmt.Errorf("restore: restic restore: %w", err)
	}

	// Flash the captured template back, then recreate+start the container.
	if err := d.Templates.Write(d.FlashTemplatesDir, d.ContainerName, d.TemplateXML); err != nil {
		return fmt.Errorf("restore: write template: %w", err)
	}
	if err := d.Docker.CreateAndStart(ctx, d.Inspect); err != nil {
		return fmt.Errorf("restore: recreate container: %w", err)
	}
	return nil
}

// scrub returns an error message safe to persist/return to clients. The
// adapters (restic/dockercli) already scrub secrets/paths from their own
// errors; this is a final guard that trims the message length.
func scrub(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	const max = 500
	if len(msg) > max {
		return msg[:max]
	}
	return msg
}
