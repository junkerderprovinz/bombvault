package api

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

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
	// selectionFP fingerprints what this item is configured to back up, empty
	// for the restore and maintenance constructions, which cover no selection.
	selectionFP string
}

var (
	_ backup.Restic    = (*resticAdapter)(nil)
	_ backup.ZFSRestic = (*resticAdapter)(nil)
)

// snapshotParentTimeout bounds the extra restic call a run pays for when every
// file it read was new. It reads one snapshot, so a repository that needs
// longer than this has stopped answering.
const snapshotParentTimeout = 30 * time.Second

func (a *resticAdapter) Backup(ctx context.Context, repo string, paths, tags []string, excludes ...string) (backup.Summary, error) {
	sum, err := a.engine.Backup(ctx, repo, paths, withTags(tags, a.extraTags), a.mode, excludes...)
	if err != nil {
		return backup.Summary{}, err
	}
	out := backupSummaryFrom(sum)
	out.SelectionFP = a.selectionFP
	out.HasParent = a.parentFlag(ctx, repo, out)
	return out, nil
}

// backupSummaryFrom converts what restic reported about a finished backup. A
// line without total_duration leaves the run unmeasured: restic prints the
// totals together, so a missing duration means the figures beside it are
// missing too, and recording them as zeros would read as a source that emptied
// itself.
func backupSummaryFrom(sum restic.Summary) backup.Summary {
	out := backup.Summary{SnapshotID: sum.SnapshotID, Bytes: int64(sum.BytesAdded)}
	if sum.TotalDuration == nil {
		return out
	}
	out.Measured = true
	out.SourceBytes = int64(sum.TotalBytesProcessed) //nolint:gosec // G115: restic cannot have read more than 8 EiB
	out.SourceFiles = int64(sum.TotalFilesProcessed) //nolint:gosec // G115: nor more than 2^63 files
	out.FilesNew = int64(sum.FilesNew)
	// The span the backfill reads from backup_start and backup_end, so a
	// series holds one measure on both sides of an upgrade.
	out.ResticMS = sum.Elapsed.Milliseconds()
	return out
}

// parentFlag answers whether restic had an earlier snapshot to compare
// against, and asks only when every file it read was new. A first backup into a
// fresh repository and a rewrite of every file read the same way, because
// restic counts a file as new when its path is missing from the parent tree,
// and appending a suffix to every file is what ransomware leaves behind. Only
// the parent separates the two, so an unreadable one stays unknown.
func (a *resticAdapter) parentFlag(ctx context.Context, repo string, sum backup.Summary) *bool {
	if !sum.Measured || sum.FilesNew == 0 || sum.FilesNew != sum.SourceFiles {
		return nil
	}
	pctx, cancel := context.WithTimeout(ctx, snapshotParentTimeout)
	defer cancel()
	parent, err := a.engine.SnapshotParent(pctx, repo, sum.SnapshotID, a.mode)
	if err != nil {
		log.Printf("api: backup: could not read whether snapshot %s was based on an earlier one: %v", shortID(sum.SnapshotID), err)
		return nil
	}
	has := parent != ""
	return &has
}

// metricsOf returns what a run recorded of restic's own figures, nil when
// restic did not report them.
func metricsOf(sum backup.Summary) *store.RunMetrics {
	if !sum.Measured {
		return nil
	}
	return &store.RunMetrics{
		SourceBytes: sum.SourceBytes,
		SourceFiles: sum.SourceFiles,
		FilesNew:    sum.FilesNew,
		ResticMS:    sum.ResticMS,
		HasParent:   sum.HasParent,
	}
}

// BackupDir carries restic's per-file counters through, which a ZFS run
// records per member and the domain's baselines read back, together with the
// totals anomaly detection watches every dataset by.
func (a *resticAdapter) BackupDir(ctx context.Context, repo, dir string, tags []string, excludes ...string) (backup.ZFSBackupSummary, error) {
	sum, err := a.engine.BackupDir(ctx, repo, dir, withTags(tags, a.extraTags), a.mode, excludes...)
	if err != nil {
		return backup.ZFSBackupSummary{}, err
	}
	measured := backupSummaryFrom(sum)
	return backup.ZFSBackupSummary{
		SnapshotID:      sum.SnapshotID,
		BytesAdded:      int64(sum.BytesAdded),
		FilesNew:        int64(sum.FilesNew),
		FilesChanged:    int64(sum.FilesChanged),
		FilesUnmodified: int64(sum.FilesUnmodified),
		Measured:        measured.Measured,
		SourceBytes:     measured.SourceBytes,
		SourceFiles:     measured.SourceFiles,
		ResticMS:        measured.ResticMS,
		HasParent:       a.parentFlag(ctx, repo, measured),
	}, nil
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
// ctx is captured at construction so Start can read the "Backup Everything"
// pass's parent run id (see runGroupKey) and the run origin off it. A ctx
// carrying neither records a plain row.
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
	// The package-level form, because the bookkeeping-only call sites build
	// this adapter without a Service.
	runID, err := startRunWith(r.ctx, r.st, targetID, kind)
	if err == nil && r.svc != nil && r.cancelKey != "" && kind == "backup" {
		r.svc.bindBackupRun(r.cancelKey, runID)
	}
	return runID, err
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

// stalledReason is the reason a backup the stall guard cancelled is recorded
// and reported with, and whether ctx was cancelled by it. A reason that already
// names the stall, as a ZFS run's does with its dataset, is kept.
func stalledReason(ctx context.Context, errMsg string) (string, bool) {
	stall := backup.StalledBy(ctx)
	switch {
	case stall == nil:
		return errMsg, false
	case strings.HasPrefix(errMsg, store.ReasonStalled):
		return errMsg, true
	}
	return stall.Error(), true
}

func (r runsAdapter) Finish(runID, status string, sum backup.Summary, errMsg string) error {
	if r.svc != nil {
		stalled, isStall := stalledReason(r.ctx, errMsg)
		if newStatus, newMsg, changed := r.svc.shutdownStatus(status); changed {
			status, errMsg = newStatus, newMsg
		} else if status == "failed" && isStall {
			// The guard cancelled first; a cancel pressed while the run unwinds
			// came too late to be the reason.
			errMsg = stalled
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
	return r.st.FinishRunMeasured(runID, status, sum.SnapshotID, sum.Bytes, errMsg, metricsOf(sum), sum.SelectionFP)
}

// startedRunsAdapter satisfies backup.Runs like runsAdapter, except Start
// returns a run id obtained earlier instead of starting a fresh run.
// BackupVM needs the run id before VMBackupDeps is built, to set RunTag =
// "vmrun:<runID>"; RunTag drives every restic tag the orchestrator builds
// (see VMBackupDeps.RunTag), so it cannot wait for the orchestrator's own
// Runs.Start call. BackupVM calls startRun itself and wraps the
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
	// ctx is the backup's context, whose cause tells a stall apart.
	ctx context.Context
}

var _ backup.Runs = startedRunsAdapter{}

func (r startedRunsAdapter) Start(string, string) (string, error) { return r.runID, nil }

func (r startedRunsAdapter) Finish(runID, status string, sum backup.Summary, errMsg string) error {
	if r.svc != nil {
		stalled, isStall := stalledReason(r.ctx, errMsg)
		if newStatus, newMsg, changed := r.svc.shutdownStatus(status); changed {
			status, errMsg = newStatus, newMsg
		} else if status == "failed" && isStall {
			errMsg = stalled
		} else if status == "failed" && r.svc.backupWasCancelled(r.cancelKey) {
			// See runsAdapter.Finish (#200). Repeated rather than shared because the
			// two adapters are separate types, and a helper taking (svc, status, key)
			// would be indirection over three lines of condition.
			status, errMsg = "cancelled", store.ReasonCancelled
		}
	}
	return r.st.FinishRunMeasured(runID, status, sum.SnapshotID, sum.Bytes, errMsg, metricsOf(sum), sum.SelectionFP)
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
	return backupSummaryFrom(sum), nil
}

func (a *resticZvolAdapter) DumpTo(ctx context.Context, repo, snapshotID, path string, w io.Writer) error {
	return a.engine.DumpRaw(ctx, repo, snapshotID, path, w, a.mode)
}
