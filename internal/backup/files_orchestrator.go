package backup

import (
	"context"
	"fmt"
)

// FilesRestic is the restic surface the files domain needs for backup. A file
// set is a plain directory backup of one named host folder (#62) — like flash,
// there is no lifecycle to manage (no stop/start) and no recreate definition
// (no defs dir): the fileset:<Name> tag is the only link between a snapshot
// and its set. Restore is handled in the service layer (original path or a
// chosen folder), never here.
type FilesRestic interface {
	Backup(ctx context.Context, repo string, paths, tags []string, excludes ...string) (Summary, error)
}

// FileSetBackupDeps bundles everything BackupFileSetDir needs.
type FileSetBackupDeps struct {
	// SourceDir is the container-visible resolved path of the set's folder
	// (paths.Resolve(HostMountRoot, set.Path), e.g. /host/user/data/docs).
	SourceDir string
	// SourcePaths is the COMPILED positional source list (Phase 4 file-sets
	// parity): the maximal-root includes of the set's stored tree selection,
	// re-anchored against SourceDir by the single compile site in
	// service.BackupFileSet. Additive beside SourceDir because CONTEXT D-01's
	// "orchestrator unchanged" is true of the FilesRestic interface (Backup
	// already takes paths []string) and false of this file: the old
	// []string{d.SourceDir} wrap capped positionals at one, so multi-root
	// selections were unreachable without this field (RESEARCH A1/Pitfall 1).
	// nil (or empty — a zero-include list must never reach restic as zero
	// positionals) is the legacy switch: the wrap falls back to
	// []string{SourceDir} byte-identically, so a set never edited via the tree
	// backs up exactly as before. Set together with SourceDir by the compile;
	// SourceDir stays the anchor either way.
	SourcePaths []string
	Repo        string
	// TargetID is the set's stable file_sets.id — runs are attributed to it so
	// renames never orphan run history.
	TargetID string
	// SetName is the user-visible set name; the snapshot is tagged
	// "fileset:<SetName>" (mirrors "container:<name>" / "vm:<name>").
	SetName string
	// Excludes are the set's restic --exclude patterns, passed through verbatim.
	Excludes []string
	Restic   FilesRestic
	Runs     Runs
}

// BackupFileSetDir backs up one file set's directory via restic. Like
// BackupFlash it is a thin record-around-restic: no stop/start lifecycle, no
// defs — just the snapshot (tagged fileset:<Name>) bracketed by a run.
func BackupFileSetDir(ctx context.Context, d FileSetBackupDeps) (Summary, error) {
	runID, err := d.Runs.Start(d.TargetID, kindBackup)
	if err != nil {
		return Summary{}, fmt.Errorf("files backup: start run: %w", err)
	}
	// Compiled multi-root positionals when the set carries a tree selection;
	// the nil/empty fallback keeps the legacy single-positional argv
	// byte-identical for a set never touched by the tree (D-03). Positionals
	// travel after -- in restic.Backup's typed builder — never as flags.
	paths := []string{d.SourceDir}
	if len(d.SourcePaths) > 0 {
		paths = d.SourcePaths
	}
	summary, err := d.Restic.Backup(ctx, d.Repo, paths, []string{"fileset:" + d.SetName}, d.Excludes...)
	if err != nil {
		_ = d.Runs.Finish(runID, statusFailed, "", 0, truncateErr(err))
		return Summary{}, err
	}
	if err := d.Runs.Finish(runID, statusSuccess, summary.SnapshotID, summary.Bytes, ""); err != nil {
		return summary, fmt.Errorf("files backup: record run: %w", err)
	}
	return summary, nil
}
