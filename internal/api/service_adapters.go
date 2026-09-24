package api

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/template"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// resticAdapter wraps a ResticEngine + Mode to satisfy backup.Restic, converting
// the engine's float64 BytesAdded to the orchestrator's int64 Bytes.
type resticAdapter struct {
	engine ResticEngine
	mode   restic.Mode
	// extraTags go on every snapshot the adapter writes, after the
	// orchestrator's own.
	extraTags []string
}

var _ backup.Restic = (*resticAdapter)(nil)

func (a *resticAdapter) Backup(ctx context.Context, repo string, paths, tags []string, excludes ...string) (backup.Summary, error) {
	sum, err := a.engine.Backup(ctx, repo, paths, withTags(tags, a.extraTags), a.mode, excludes...)
	if err != nil {
		return backup.Summary{}, err
	}
	return backup.Summary{SnapshotID: sum.SnapshotID, Bytes: int64(sum.BytesAdded)}, nil
}

func (a *resticAdapter) RestorePaths(ctx context.Context, repo, snapshotID string, paths []string) error {
	for _, p := range paths {
		if err := a.engine.RestorePath(ctx, repo, snapshotID, p, a.mode); err != nil {
			return err
		}
	}
	return nil
}

func (a *resticAdapter) RestoreSubtreeTo(ctx context.Context, repo, snapshotID, subtreePath, target string) error {
	return a.engine.RestoreSubtreeTo(ctx, repo, snapshotID, subtreePath, target, a.mode)
}

// VerifySnapshot lists the repo (which also proves it is reachable and the key
// is right) and confirms snapshotID is present, so a restore aborts before any
// destructive teardown if the snapshot is missing or the repo is unreadable.
func (a *resticAdapter) VerifySnapshot(ctx context.Context, repo, snapshotID string) error {
	snaps, err := a.engine.Snapshots(ctx, repo, a.mode)
	if err != nil {
		return fmt.Errorf("read repo: %w", err)
	}
	prefixMatches := 0
	for _, s := range snaps {
		if s.ID == snapshotID {
			return nil // exact id is unambiguous
		}
		if strings.HasPrefix(s.ID, snapshotID) {
			prefixMatches++
		}
	}
	switch prefixMatches {
	case 0:
		return fmt.Errorf("snapshot %s not found", snapshotID)
	case 1:
		return nil
	default:
		// An ambiguous short id would fail in restic after the destructive
		// teardown, so reject it now, before anything is stopped or destroyed.
		return fmt.Errorf("snapshot id %s is ambiguous (matches %d snapshots)", snapshotID, prefixMatches)
	}
}

// templatesAdapter satisfies backup.Templates over the template package funcs.
type templatesAdapter struct{}

var _ backup.Templates = templatesAdapter{}

func (templatesAdapter) Read(dir, name string) (string, bool, error) { return template.Read(dir, name) }

func (templatesAdapter) Write(dir, name, xml string) error { return template.Write(dir, name, xml) }

// runsAdapter satisfies backup.Runs over *store.Repo (StartRun/FinishRun).
// ctx is captured at construction so Start can read runGroupFromContext
// and stamp a "Backup Everything" pass's parent run id onto the child run
// it just created (see runGroupKey); a ctx without a group changes nothing.
type runsAdapter struct {
	st  *store.Repo
	ctx context.Context
	// svc is read only to ask whether the process is shutting down and whether
	// a user cancelled this particular backup (#200). It is optional: the
	// bookkeeping-only call sites below pass nil, which costs only the
	// shutdown relabel, and those sites are not the run a backup finishes on.
	svc *Service
	// cancelKey is this run's progress key ("files:<id>", "container:<name>",
	// "vm:<name>", "flash"), set only by the call sites that also register a
	// backup cancel func under it. Empty everywhere else, which is what makes
	// the user-cancellation relabel below reach exactly the runs a user can
	// actually cancel and no others.
	cancelKey string
}

var _ backup.Runs = runsAdapter{}

func (r runsAdapter) Start(targetID, kind string) (string, error) {
	id, err := r.st.StartRun(targetID, kind)
	if err != nil {
		return "", err
	}
	if gid := runGroupFromContext(r.ctx); gid != "" {
		// Best-effort, like every other post-Start bookkeeping call in this
		// file: the run already started successfully, so a stamp failure is
		// logged, never returned (see store.SetRunGroup's doc comment).
		if serr := r.st.SetRunGroup(id, gid); serr != nil {
			log.Printf("api: run %s: stamp group %s failed: %v", id, gid, serr) //nolint:gosec // G706: id/gid are internal ids, not user input
		}
	}
	return id, nil
}

// shutdownStatus rewrites a failure that is really a shutdown.
//
// The orchestrator has no idea why its context died; it sees a cancelled
// context and reports "failed", which is right for every other cause. On
// the way out of the process it is wrong where it matters most: a red row
// with a context error looks like a backup that genuinely broke, the
// dashboard counts it, the notification fires for it, and somebody goes
// looking for a fault that was a `docker stop`.
//
// This is the one place with both facts in hand, that the run failed and
// that the process is leaving, so it is the one place that can tell the
// difference. The "cancelled" status already exists and fires no failure
// alert.
//
// It is narrow: it only downgrades a failed run, and only while shutting
// down, so a real failure that lands in the same second keeps its status.
// Errors are not inspected for cancellation: once BeginShutdown has
// cancelled the context, every failure from that run is downstream of it,
// and matching on error text would be a weaker second guess at something
// the flag already knows.
func (s *Service) shutdownStatus(status string) (string, string, bool) {
	if status != "failed" || !s.shuttingDown.Load() {
		return status, "", false
	}
	return "cancelled", store.ReasonShutdown, true
}

func (r runsAdapter) Finish(runID, status, snapshotID string, bytes int64, errMsg string) error {
	if r.svc != nil {
		if newStatus, newMsg, changed := r.svc.shutdownStatus(status); changed {
			status, errMsg = newStatus, newMsg
		} else if status == "failed" && r.svc.backupWasCancelled(r.cancelKey) {
			// A user cancelled this backup (#200). The error in hand is the context
			// cancellation that followed, and recording it as a failure would put a
			// red row in Run History, count it on the dashboard and fire an alert for
			// something somebody asked for.
			//
			// As narrow as the shutdown relabel above, for the same reason: it only
			// downgrades a failed run, and only one whose key was marked. A real
			// failure in a backup nobody cancelled keeps its status, and the error
			// text is not consulted.
			status, errMsg = "cancelled", store.ReasonCancelled
		}
	}
	return r.st.FinishRun(runID, status, snapshotID, bytes, errMsg)
}

// startedRunsAdapter satisfies backup.Runs like runsAdapter, except Start
// returns a run id obtained earlier instead of starting a fresh run.
// BackupVM needs the run id before VMBackupDeps is built, to set RunTag =
// "vmrun:<runID>"; RunTag drives every restic tag the orchestrator builds
// (see VMBackupDeps.RunTag), so it cannot wait for the orchestrator's own
// Runs.Start call. BackupVM calls store.StartRun itself and wraps the
// result here, so the orchestrator's Start is a read rather than a second,
// orphaned run row; Finish delegates to the real store like runsAdapter.
type startedRunsAdapter struct {
	st    *store.Repo
	runID string
	// svc plays the same role as in runsAdapter: a VM backup is a backup, so a
	// shutdown mid-run must read as an abort here too; the two adapters differ
	// only in where the run id comes from.
	svc *Service
	// cancelKey: same role as runsAdapter.cancelKey (#200), and it has to be
	// here for the same reason the line above gives. A VM backup registers
	// "vm:<name>" like every other domain registers its own key, so a user who
	// cancels one must get the same "cancelled" row a cancelled folder backup
	// gets, not a red failure.
	cancelKey string
}

var _ backup.Runs = startedRunsAdapter{}

func (r startedRunsAdapter) Start(string, string) (string, error) { return r.runID, nil }

func (r startedRunsAdapter) Finish(runID, status, snapshotID string, bytes int64, errMsg string) error {
	if r.svc != nil {
		if newStatus, newMsg, changed := r.svc.shutdownStatus(status); changed {
			status, errMsg = newStatus, newMsg
		} else if status == "failed" && r.svc.backupWasCancelled(r.cancelKey) {
			// See runsAdapter.Finish (#200). Repeated rather than shared because the
			// two adapters are separate types, and a helper taking (svc, status, key)
			// would be indirection over three lines of condition.
			status, errMsg = "cancelled", store.ReasonCancelled
		}
	}
	return r.st.FinishRun(runID, status, snapshotID, bytes, errMsg)
}

// sshZFSHost adapts HostSSH's Run/StreamCommand/RunWithStdin surface into
// backup.ZFSHost by running the zfs command lines
// virshcli.ZFSSnapshotArgs/ZFSSnapshotDestroyArgs/ZFSSendArgs/ZFSReceiveArgs
// build over SSH, the way virshcli runs virsh commands over the same
// transport, so the container needs no local ZFS toolchain.
type sshZFSHost struct{ ssh HostSSH }

var _ backup.ZFSHost = sshZFSHost{}

func (h sshZFSHost) SnapshotCreate(ctx context.Context, dataset, snapName string) error {
	_, err := h.ssh.Run(ctx, virshcli.ZFSSnapshotArgs(dataset, snapName)...)
	return err
}

func (h sshZFSHost) SnapshotDestroy(ctx context.Context, dataset, snapName string) error {
	_, err := h.ssh.Run(ctx, virshcli.ZFSSnapshotDestroyArgs(dataset, snapName)...)
	return err
}

func (h sshZFSHost) StreamSend(ctx context.Context, dataset, snapName string) (io.ReadCloser, func() error, error) {
	return h.ssh.StreamCommand(ctx, virshcli.ZFSSendArgs(dataset, snapName)...)
}

func (h sshZFSHost) StreamReceive(ctx context.Context, rd io.Reader, targetDataset string) error {
	return h.ssh.RunWithStdin(ctx, rd, virshcli.ZFSReceiveArgs(targetDataset)...)
}

// resticZvolAdapter wraps a ResticEngine and Mode to satisfy
// backup.ZvolRestic for the zvol stdin backup and restore path, with the
// same float64 BytesAdded to int64 Bytes conversion as resticAdapter. It is
// its own small type because backup.ZvolRestic is separate from
// backup.Restic (file-backed disk backup and restore never need stdin
// streaming).
type resticZvolAdapter struct {
	engine ResticEngine
	mode   restic.Mode
	// extraTags go on every snapshot the adapter writes, after the
	// orchestrator's own.
	extraTags []string
}

var _ backup.ZvolRestic = (*resticZvolAdapter)(nil)

func (a *resticZvolAdapter) BackupStdin(ctx context.Context, repo string, rd io.Reader, path string, tags []string) (backup.Summary, error) {
	sum, err := a.engine.BackupStdin(ctx, repo, rd, path, withTags(tags, a.extraTags), a.mode)
	if err != nil {
		return backup.Summary{}, err
	}
	return backup.Summary{SnapshotID: sum.SnapshotID, Bytes: int64(sum.BytesAdded)}, nil
}

func (a *resticZvolAdapter) DumpTo(ctx context.Context, repo, snapshotID, path string, w io.Writer) error {
	return a.engine.DumpRaw(ctx, repo, snapshotID, path, w, a.mode)
}
