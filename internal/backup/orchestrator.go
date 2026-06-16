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
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/model"
)

// ---------------------------------------------------------------------------
// DI types & interfaces (the seam — no concrete adapters imported)
// ---------------------------------------------------------------------------

// Sentinel errors returned by the restore guards so the API layer can branch on
// them with errors.Is (e.g. to map to a 4xx) without string matching.
var (
	// ErrNotConfirmed is returned when a restore is attempted without the
	// explicit confirmation flag set.
	ErrNotConfirmed = errors.New("restore: not confirmed (confirm must be true)")
	// ErrInvalidSnapshotID is returned when the snapshot id fails the strict
	// hex validation (arg-injection guard).
	ErrInvalidSnapshotID = errors.New("restore: invalid snapshot id (must be 8–64 lowercase hex)")
	// ErrRestoreConflict is returned by the pre-flight check when the container's
	// static IP or a published host port is already held by another container.
	// It wraps a human-readable list of the conflicts; nothing destructive has
	// run yet, so the user can free the resources and retry.
	ErrRestoreConflict = errors.New("restore: ip/port conflict")
)

// Summary is the result of a successful backup.
type Summary struct {
	SnapshotID string
	Bytes      int64
}

// Docker is the subset of host control the orchestrator needs. The rich
// container profile travels as model.Inspect so all security-relevant fields
// reach the real adapter on recreate (SEC §8). The orchestrator imports only
// the model types — never the concrete dockercli adapter (the DI seam).
type Docker interface {
	Stop(ctx context.Context, name string, timeout time.Duration) error
	Start(ctx context.Context, name string) error
	Remove(ctx context.Context, name string) error
	Pull(ctx context.Context, image string) error
	CreateAndStart(ctx context.Context, in model.Inspect) error
	// InspectName returns the live container's name (the adapter normalizes it),
	// or "" when no such container exists.
	InspectName(ctx context.Context, name string) (string, error)
	// Allocations reports the static IP / published host ports every container
	// currently holds, for the restore pre-flight conflict check.
	Allocations(ctx context.Context) ([]model.Allocation, error)
}

// Restic is the subset of the backup engine the orchestrator needs.
type Restic interface {
	Backup(ctx context.Context, repo string, paths, tags []string) (Summary, error)
	// RestorePaths restores each backed-up path back to its own location as a
	// subtree, so restic never reconciles shared parent dirs (SEC §8 / DR).
	RestorePaths(ctx context.Context, repo, snapshotID string, paths []string) error
}

// Templates reads and writes Unraid container templates. Read returns
// (xml, found, err): a genuine not-exist is (\"\", false, nil); a real I/O
// error (e.g. permission) is surfaced so the caller never silently treats it as
// "no template".
type Templates interface {
	Read(dir, name string) (string, bool, error)
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
	// AppdataPaths are the backed-up absolute paths; each is restored as its own
	// subtree back to origin (restic restore <id>:<path> --target <path>), so
	// restic never tries to reconcile (and fail on) the shared appdata parent.
	AppdataPaths []string
	// TemplateXML is the captured template flashed back on restore.
	TemplateXML string
	// FlashTemplatesDir is where the live Unraid templates live.
	FlashTemplatesDir string
	// Inspect is the captured definition used to recreate the container. The
	// full profile (incl. security fields) flows through CreateAndStart.
	Inspect model.Inspect
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

// ValidSnapshotID reports whether id is a well-formed restic snapshot id
// (8–64 lowercase hex). Exported so the service layer reuses the same guard for
// file-level restore before shelling out to restic.
func ValidSnapshotID(id string) bool { return snapshotIDRe.MatchString(id) }

// normalizeName strips a single leading slash from a docker container name.
func normalizeName(n string) string {
	return strings.TrimPrefix(n, "/")
}

// pullRef returns the registry reference to pull when recreating a container.
// The inspect's top-level Image is the image ID (sha256:…) which cannot be
// pulled from a registry; the human reference lives in Config.Image. The
// top-level Image is used only as a fallback when Config.Image is empty.
func pullRef(in model.Inspect) string {
	if in.Config.Image != "" {
		return in.Config.Image
	}
	return in.Image
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
		//
		// A genuine "no template" (ok==false, err==nil) is fine — not every
		// container has an Unraid template. A real I/O error must NOT silently
		// pass: the snapshot itself is valid, so the backup still succeeds, but
		// we surface the template-capture failure in the log rather than
		// pretending a template was never there.
		xml, ok, readErr := d.Templates.Read(d.FlashTemplatesDir, d.ContainerName)
		switch {
		case readErr != nil:
			log.Printf("backup: capture template for %q failed (snapshot still valid): %v",
				d.ContainerName, readErr)
		case ok:
			snapName := summary.SnapshotID + "-" + d.ContainerName
			if backupErr = d.Templates.Write(d.SnapshotTemplatesDir, snapName, xml); backupErr != nil {
				backupErr = fmt.Errorf("backup: persist template: %w", backupErr)
				return
			}
		}
	}()

	if backupErr != nil {
		_ = d.Runs.Finish(runID, statusFailed, "", 0, truncateErr(backupErr))
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
		return ErrNotConfirmed
	}
	if !snapshotIDRe.MatchString(d.SnapshotID) {
		return ErrInvalidSnapshotID
	}

	runID, err := d.Runs.Start(d.TargetID, kindRestore)
	if err != nil {
		return fmt.Errorf("restore: record run start: %w", err)
	}

	restoreErr := runRestore(ctx, d)
	if restoreErr != nil {
		_ = d.Runs.Finish(runID, statusFailed, "", 0, truncateErr(restoreErr))
		return restoreErr
	}

	if err := d.Runs.Finish(runID, statusSuccess, d.SnapshotID, 0, ""); err != nil {
		return fmt.Errorf("restore: record run finish: %w", err)
	}
	return nil
}

// runRestore performs the destructive restore sequence after the guards pass.
func runRestore(ctx context.Context, d RestoreDeps) error {
	// SEC: every appdata path must be absolute and traversal-free. Each is
	// restored as its own subtree back to origin (restore <id>:<path> --target
	// <path>), so restic never reconciles the shared appdata parent directory
	// (which fails on a populated dir: "failed to remove stale item ... is a
	// directory"). This replaces the old single "--target /" restore (SEC-102).
	if len(d.AppdataPaths) == 0 {
		return errors.New("restore: no appdata paths to restore")
	}
	for _, p := range d.AppdataPaths {
		if !strings.HasPrefix(p, "/") || strings.Contains(p, "..") {
			return fmt.Errorf("restore: unsafe appdata path %q (SEC)", p)
		}
	}

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

	// Pre-flight: refuse to start a destructive restore that we already know will
	// fail because the container's static IP or a published host port is held by
	// ANOTHER running container. Reported BEFORE pull/stop/remove so nothing is
	// changed — the user frees the resource and retries.
	if err := checkRestoreConflicts(ctx, d); err != nil {
		return err
	}

	// Pull the image before touching the running container. Pull the human
	// registry REFERENCE (Config.Image), never the inspect's top-level Image —
	// that is the image ID (sha256:…), which is not pullable from a registry
	// ("pull access denied for sha256").
	if err := d.Docker.Pull(ctx, pullRef(d.Inspect)); err != nil {
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

	// Restore each appdata path back to its origin as its own subtree.
	if err := d.Restic.RestorePaths(ctx, d.RepoPath, d.SnapshotID, d.AppdataPaths); err != nil {
		return fmt.Errorf("restore: restic restore: %w", err)
	}

	// Flash the captured template back, then recreate+start the container. Only
	// write when a template was actually captured: writing an empty placeholder
	// would make Unraid treat the app as broken/third-party (e.g. for backups
	// taken before the flash templates dir was mounted).
	if d.TemplateXML != "" {
		if err := d.Templates.Write(d.FlashTemplatesDir, d.ContainerName, d.TemplateXML); err != nil {
			return fmt.Errorf("restore: write template: %w", err)
		}
	}
	if err := d.Docker.CreateAndStart(ctx, d.Inspect); err != nil {
		return fmt.Errorf("restore: recreate container: %w", err)
	}
	return nil
}

// checkRestoreConflicts runs the restore pre-flight: it refuses to proceed when
// the container's requested static IP or a published host port is already held
// by ANOTHER container (the container being restored is excluded — its own
// resources free up when it is removed). It returns ErrRestoreConflict wrapping
// a human-readable, actionable list. A container with no static IP and no
// published ports (DHCP / host networking / no ports) has nothing to conflict
// on and skips the Docker call entirely.
func checkRestoreConflicts(ctx context.Context, d RestoreDeps) error {
	targetIP := d.Inspect.Network.IPv4Address
	targetPorts := publishedHostPorts(d.Inspect)
	if targetIP == "" && len(targetPorts) == 0 {
		return nil
	}

	allocs, err := d.Docker.Allocations(ctx)
	if err != nil {
		return fmt.Errorf("restore: check ip/port conflicts: %w", err)
	}

	self := normalizeName(d.ContainerName)
	var conflicts []string
	for _, a := range allocs {
		if normalizeName(a.Name) == self {
			continue // the container being restored — its resources free on remove
		}
		if targetIP != "" && a.IPv4 == targetIP {
			conflicts = append(conflicts, fmt.Sprintf("IP %s is already used by container %q", targetIP, normalizeName(a.Name)))
		}
		for _, hp := range a.HostPorts {
			if targetPorts[hp] {
				conflicts = append(conflicts, fmt.Sprintf("host port %s is already used by container %q", hp, normalizeName(a.Name)))
			}
		}
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("%w — free these and retry: %s", ErrRestoreConflict, strings.Join(conflicts, "; "))
	}
	return nil
}

// publishedHostPorts returns the set of host ports the inspect publishes, keyed
// as "<hostPort>/<proto>" (e.g. "8080/tcp"). Bindings without a host port
// (container-internal only) are ignored — they never collide on the host.
func publishedHostPorts(in model.Inspect) map[string]bool {
	out := map[string]bool{}
	for portProto, binds := range in.HostConfig.PortBindings {
		proto := "tcp"
		if i := strings.LastIndex(portProto, "/"); i >= 0 && i+1 < len(portProto) {
			proto = portProto[i+1:]
		}
		for _, b := range binds {
			if b.HostPort != "" {
				out[b.HostPort+"/"+proto] = true
			}
		}
	}
	return out
}

// truncateErr returns an error message bounded to the DB column length. It is a
// length-only guard: the adapters (restic/dockercli) already scrub
// secrets/paths from their own errors, so this only trims the message so it fits
// the runs.error_message column.
func truncateErr(err error) string {
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
