package api

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// StartBackupFileSet launches a single file-set backup in a background
// goroutine and returns immediately, mirroring StartBackupVM for the files
// domain. id is the set's stable store id; progress is published under
// "files:<name>". Shares batchActive (no overlap with any other backup);
// returns (false, nil) if one is already running, or (false, err) if the files
// domain is already busy with another op.
func (s *Service) StartBackupFileSet(ctx context.Context, id string) (bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	if op, busy := s.domainBusy("files"); busy {
		s.batchActive.Store(false)
		return false, fmt.Errorf("%s is running on files", op)
	}
	bctx := context.WithoutCancel(ctx)
	go func() {
		defer s.recoverOperation("backup file set: "+id, nil, func(msg string) {
			s.failStuckRun(id, msg) // id is the runs.target_id for a file set, no lookup needed
		})
		defer s.batchActive.Store(false)
		if _, err := s.BackupFileSet(bctx, id); err != nil {
			log.Printf("api: backup file set: %q failed: %v", id, err) //nolint:gosec // G706: id is %q-quoted
		}
	}()
	return true, nil
}

// StartBackupFilesAll launches sequential backups for the given file-set ids
// in one background batch and returns immediately, mirroring StartBackupAll
// for the files domain. There is no self-container to skip: every non-empty
// id is attempted, and failures are logged and counted without aborting the
// rest. Overall progress is published under "batch:files" while each set
// still publishes its own "files:<name>" bar. Shares batchActive; returns
// (false, nil) if a backup or batch is already running, or (false, err) if
// the files domain is already busy with another op.
func (s *Service) StartBackupFilesAll(ctx context.Context, ids []string) (bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	if op, busy := s.domainBusy("files"); busy {
		s.batchActive.Store(false)
		return false, fmt.Errorf("%s is running on files", op)
	}
	// Detach immediately so the batch is independent of the request that
	// started it (canceled the moment the handler returns). Each per-set
	// BackupFileSet applies its own hard timeout, so the batch needs no
	// deadline of its own. #95: each set's inline off-site replication is
	// suppressed and the whole batch is replicated once after the loop
	// (ReplicateOffsiteAfterBulk below), like the containers batch.
	bctx := WithBulkReplicateSuppressed(context.WithoutCancel(ctx))
	go func() {
		// See StartBackupAll's pair of defers: this one contains a panic outside
		// the per-item loop, while each item gets its own more precise recovery
		// (backupFileSetOneForBatch) so one bad set can't abort the rest.
		defer s.recoverOperation("backup-files-all", nil, nil)
		defer s.batchActive.Store(false)

		queue := make([]string, 0, len(ids))
		for _, id := range ids {
			if id != "" {
				queue = append(queue, id)
			}
		}
		total := len(queue)
		const key = "batch:files"
		s.publishBatch(key, 0, true)
		ok, fail := 0, 0
		for i, id := range queue {
			if err := s.backupFileSetOneForBatch(bctx, id); err != nil {
				fail++
				log.Printf("api: backup-files-all: %q failed (continuing): %v", id, err) //nolint:gosec // G706: id is %q-quoted
			} else {
				ok++
			}
			s.publishBatch(key, float64(i+1)/float64(total)*100, true)
		}
		s.publishBatch(key, 100, false)
		// Retention first: one local prune for the whole batch (each set's forget
		// ran without --prune under the bulk flag), before the batched off-site
		// replication, so fewer snapshots are left to copy.
		s.PruneAfterBulk(bctx, "files")
		// #95: one batched off-site replication after the whole manual batch (no-op
		// unless files replicate on a blank/coupled schedule with an off-site
		// repo). The per-set inline copy was suppressed via the bulk flag on bctx.
		s.ReplicateOffsiteAfterBulk(bctx, "files")
		// One sample for the whole round, exactly as the container batch does.
		s.maybeCollectStats(bctx, "files")
		log.Printf("api: backup-files-all done: %d ok, %d failed (of %d requested %d)", ok, fail, total, len(ids))
	}()
	return true, nil
}

// backupFileSetOneForBatch backs up a single queued file set on behalf of
// StartBackupFilesAll; see backupOneForBatch for why each item gets its own
// recovery. id is already the runs.target_id, so no lookup is needed.
func (s *Service) backupFileSetOneForBatch(ctx context.Context, id string) (err error) {
	// See backupOneForBatch for why this must be a direct defer, not wrapped.
	defer s.recoverOperation("backup-files-all: "+id, &err, func(msg string) {
		s.failStuckRun(id, msg)
	})
	_, err = s.BackupFileSet(ctx, id)
	return err
}

// concludeFileSetRestore ends a file-set restore (settings-driven or
// foreign): it drives the terminal progress event and the run record, and
// downgrades a metadata-only restic failure (all data present, only
// ownership or metadata could not be set on a /mnt/user FUSE share,
// restic.ErrRestoreMetadataOnly) to a success with a warning. Genuine
// failures (missing snapshot, no space, unreachable repo) and user cancels
// are recorded as they are. It returns the effective error (nil for the
// metadata-only case), so the caller logs or propagates only a real
// failure. startedAt must be the value the matching progBegin returned
// (see progEnd).
func (s *Service) concludeFileSetRestore(runID, rkey, snapshotID string, rerr error, startedAt int64) error {
	if errors.Is(rerr, restic.ErrRestoreMetadataOnly) {
		s.progEnd(rkey, "restore", true, startedAt)
		s.finishRestoreRunWarn(runID, snapshotID, restic.RestoreMetadataWarning)
		return nil
	}
	s.progEnd(rkey, "restore", rerr == nil, startedAt)
	s.finishRestoreRun(runID, snapshotID, rerr)
	return rerr
}

// resticAdapter also satisfies the files domain's backup surface.
var _ backup.FilesRestic = (*resticAdapter)(nil)

// BackupFileSet backs up one file set (a named host folder under the /mnt
// mount, resolved like settings.ContainersPath) to the files repo via
// restic, tagged fileset:<Name>. It mirrors BackupFlash: no lifecycle, no
// defs, with retention, off-site replication and stats as in the other
// domains. A source folder that does not exist under the host mount fails
// with a clear error before any restic call, recording a failed run against
// the set's id so a scheduled backup of a vanished folder surfaces in Run
// History.
func (s *Service) BackupFileSet(ctx context.Context, id string) (backup.Summary, error) {
	// Survive the client that triggered it disconnecting (see Backup): detach from
	// the request's cancellation with a generous hard cap.
	ctx, cancel := backupHoldCtx(ctx)
	defer cancel()
	defer s.lockDomain("files")() // serialise per repo; blocks maintenance ops meanwhile
	settings, err := s.store.GetSettings()
	if err != nil {
		return backup.Summary{}, fmt.Errorf("read settings: %w", err)
	}
	set, err := s.store.GetFileSet(id)
	if err != nil {
		return backup.Summary{}, fmt.Errorf("files backup: load file set: %w", err)
	}
	// Registered under the set's name, not its id, and here rather than at the
	// top of the function for that reason (#200). It is the key the progress
	// stream publishes under ("files:"+set.Name, a few lines below), the key
	// containers and VMs use for both purposes. The interface cancels with the
	// one key it has, the one from the progress stream, and a mismatch would
	// make the Cancel button answer "cancelled: false" and do nothing, with no
	// error anywhere to explain it. What runs before this is a settings read
	// and a row lookup, neither of which can hang or is worth cancelling.
	s.registerBackupCancel("files:"+set.Name, cancel)
	defer s.unregisterBackupCancel("files:" + set.Name)
	// A set without a path cannot be backed up (Discover creates path-less,
	// disabled sets from fileset: tags alone); say so instead of letting
	// paths.Resolve report a misleading traversal error for "".
	if strings.TrimSpace(set.Path) == "" {
		return backup.Summary{}, fmt.Errorf("files backup: file set %q has no source path configured. Set a path before backing up", set.Name)
	}
	src, err := paths.Resolve(s.cfg.HostMountRoot, set.Path)
	if err != nil {
		return backup.Summary{}, fmt.Errorf("files backup: resolve source path for %q: %w", set.Name, err)
	}
	if _, statErr := os.Stat(src); errors.Is(statErr, fs.ErrNotExist) {
		// Record the miss as a failed run so a scheduled backup of a renamed or
		// deleted folder shows up in Run History instead of failing invisibly.
		err := fmt.Errorf("files backup: source path not found for %q (%s does not exist under the host mount)", set.Name, src)
		if runID, sErr := s.store.StartRun(set.ID, "backup"); sErr != nil {
			log.Printf("api: files backup: %q: record missing-path run: %v", set.Name, sErr) //nolint:gosec // G706: name is %q-quoted
			// truncateRunErr, like every other FinishRun of the service: the message
			// embeds the resolved host path (scrub), and a file set's name and path
			// are never length-validated at creation, so an arbitrarily long string
			// could otherwise reach runs.error uncapped and travel on into the weekly
			// digest.
		} else if fErr := s.store.FinishRun(runID, "failed", "", 0, truncateRunErr(err)); fErr != nil {
			log.Printf("api: files backup: %q: finish missing-path run: %v", set.Name, fErr) //nolint:gosec // G706: name is %q-quoted
		}
		return backup.Summary{}, err
	}
	// The set's own repository if it has one (#204), otherwise the domain's.
	// Everything below takes `repo` as a parameter (EnsureRepo, the run
	// record, applyRetention, makeRepoReadable, checkPrimaryRemoteBudget), so a
	// per-set repository carries through the whole backup without a second
	// decision point. replicateOffsite is the exception and stays per domain:
	// an off-site copy is configured for the Folders domain, and a set's own
	// primary says nothing about where its replica should live.
	item := store.ItemRef{Domain: "files", Key: set.ID}
	step, err := s.prepareHome(ctx, settings, item)
	if err != nil {
		return backup.Summary{}, err
	}
	if step, err = s.recordHome(ctx, settings, item, step); err != nil {
		return backup.Summary{}, err
	}
	repo, mode := step.repo, step.mode
	// An empty or unreadable source still makes a successful restic snapshot,
	// and keep-N retention groups by tag alone, so each such snapshot would
	// age a real one of a set with history out. The walk stops at the first
	// file, so a populated folder never pays for the listing.
	if empty, eErr := fileSetSourceEmpty(src); eErr != nil || empty {
		hasHistory := true
		if snaps, hErr := s.snapshotsForTag(ctx, repo, mode, "fileset:"+set.Name); hErr == nil {
			hasHistory = len(snaps) > 0
		} else {
			log.Printf("api: files backup: %q: could not check existing history: %v", set.Name, hErr) //nolint:gosec // G706: name is %q-quoted
		}
		if hasHistory {
			reason := fmt.Sprintf("source folder for %q has no files (%s); check whether it moved", set.Name, src)
			if eErr != nil {
				reason = fmt.Sprintf("source folder for %q could not be read (%s): %v", set.Name, src, eErr)
			}
			err := fmt.Errorf("files backup: %s", reason)
			if runID, sErr := s.store.StartRun(set.ID, "backup"); sErr != nil {
				log.Printf("api: files backup: %q: record failed run: %v", set.Name, sErr) //nolint:gosec // G706: name is %q-quoted
			} else if fErr := s.store.FinishRun(runID, "failed", "", 0, truncateRunErr(err)); fErr != nil {
				log.Printf("api: files backup: %q: finish failed run: %v", set.Name, fErr) //nolint:gosec // G706: name is %q-quoted
			}
			return backup.Summary{}, err
		}
	}
	// Healthchecks /start ping: deferred to here, past the source-exists + EnsureRepo
	// guards, so the paired done/fail notifyBackup below always follows (no dangling /start).
	s.notifyBackupStart(ctx, "files")
	key := "files:" + set.Name
	fctx, startedAt := s.progBegin(ctx, key, "backup")
	// The single compile site for the files domain. The set was read fresh
	// above (a selection saved between two backups affects exactly the later
	// one, and a concurrent edit can never tear a mid-run argv), and
	// fileSetPositionals re-anchors the stored entries against this run's
	// resolved root, so a Path edit can never hand restic a positional outside
	// the set's current scope. set.SelectedPaths == nil (the NULL column)
	// compiles to []string{src}. User-owned exclude patterns stay first; the
	// selection-derived tail enforces the stored exclusion branches on the
	// argv, like the container compile line in the BackupDeps literal
	// (excludedBranches(nil) is empty, so the append is always safe). Patterns
	// travel as typed builder arguments (excludes before --, positionals
	// after), never through a shell.
	positionals := fileSetPositionals(set.SelectedPaths, src)
	// The exclusion tail is derived against the same anchored view, never the
	// raw stored list. With the exclusions read straight off storage, a Path
	// edit could make the includes fall out of scope and the positional fall
	// back to src, and an exclusion above the new root would then be emitted
	// as a strict descendant of the old one, asking restic to filter its own
	// source root. selection.go never emits that pair and relies on the tree
	// UI being unable to construct it; the fallback would construct it around
	// the UI. Pairing the positionals with the stored exclusions closes that:
	// an exclusion equal to a positional is not a strict descendant, so it is
	// not emitted.
	var anchored []string
	anchored = append(anchored, positionals...)
	for _, e := range set.SelectedPaths {
		if bare, excluded := SplitExclusion(e); excluded && bare != "" {
			anchored = append(anchored, e)
		}
	}
	sum, err := backup.BackupFileSetDir(fctx, backup.FileSetBackupDeps{
		SourceDir:   src,
		SourcePaths: positionals,
		Repo:        repo,
		TargetID:    set.ID,
		SetName:     set.Name,
		Excludes: append(append([]string{}, set.Excludes...),
			excludedBranches(anchored)...),
		Restic: &resticAdapter{engine: s.engine, mode: mode, extraTags: s.directTags(settings, "files", repo)},
		Runs:   runsAdapter{st: s.store, ctx: ctx, svc: s, cancelKey: "files:" + set.Name},
	})
	s.progEnd(key, "backup", err == nil, startedAt)
	s.notifyBackup(ctx, "files", set.Name, err == nil, sum, err)
	if err != nil {
		return backup.Summary{}, err
	}
	s.applyRetention(ctx, repo, settings, mode, tagIdentity("fileset:"+set.Name), "files")
	makeRepoReadable(repo, s.cfg.DataDir) // keep the local repo copyable off-box by a non-root user
	s.replicateOffsite(ctx, "files", settings, repo, "fileset:"+set.Name)
	s.collectStatsAfterItem(ctx, "files")
	s.checkPrimaryRemoteBudget(ctx, "files", repo, settings)
	return sum, nil
}

// walkFileSetSource is fileSetSourceEmpty's directory walk, replaceable in
// tests so a read failure can be forced.
var walkFileSetSource = filepath.WalkDir

// fileSetSourceEmpty reports whether src holds nothing but directories.
// Excludes play no part: a fully excluded folder is still the real folder.
func fileSetSourceEmpty(src string) (bool, error) {
	empty := true
	err := walkFileSetSource(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == src || d.IsDir() {
			return nil
		}
		empty = false
		return filepath.SkipAll
	})
	if err != nil {
		return false, err
	}
	return empty, nil
}

// errFileSetNotFound is the user-safe error for an unknown file-set id (the
// raw store error would leak SQL wording through the API surface).
var errFileSetNotFound = errors.New("file set not found")

// errFileSetEmptySelection is the file-set boundary's refusal; the PATCH
// handler maps it (errors.Is) to the machine-routable "empty-selection"
// envelope code, as the containers' boundary does for errEmptySelection. A
// file set cannot mean "back up nothing": a set with zero included folders
// is either a mistake or a set the user no longer wants, so the message
// points to removing the set rather than "select something".
var errFileSetEmptySelection = errors.New("a file set needs at least one folder selected; use Remove set if you no longer want this set")

// FileSetView is the per-set row returned by ListFileSetViews, the files
// domain's counterpart of VMView. LastBackup is the unix time of the last
// successful backup run (0 = never; runs-based, so listing never spawns a
// restic process per row). PathExists reports whether the set's resolved
// source folder exists under the host mount; it is false for a vanished
// folder and for the path-less sets DiscoverFileSets creates, so the UI
// can flag "set path before backup".
type FileSetView struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	Excludes   []string `json:"excludes"`
	Enabled    bool     `json:"enabled"`
	LastBackup int64    `json:"lastBackup"`
	PathExists bool     `json:"pathExists"`
	// SelectedPaths is the set's tree selection, served back for the editor in
	// the flat encoding the containers' backupPaths uses (bare entries are
	// included roots, "!"-prefixed ones deselected branches), in mount-root
	// absolute space. omitempty keeps the NULL column ("never touched by the
	// tree") distinguishable on the wire from a written selection: a
	// never-edited set must not render a tree state it never had.
	SelectedPaths []string `json:"selectedPaths,omitempty"`
	// ScheduleCadence is the set's per-item schedule override (#199); empty means
	// it follows the Folders domain schedule. Always sent, so the interface can
	// show the cadence without a second request, and only acted on while the
	// per-item-schedules toggle is on.
	ScheduleCadence string `json:"scheduleCadence"`
	// Repo is the set's own repository (#204); empty means it follows the
	// Folders domain repository. Always sent, so the card can show where a set
	// backs up without a second request, and a set with an override says so
	// instead of looking like every other set while its snapshots live
	// somewhere else.
	Repo string `json:"repo"`
	// RepoEffective is where this set's backups actually land, already resolved:
	// the override if there is one, otherwise the domain path. Computed here so
	// the sentence on the card comes from the same resolution the backup runs
	// through, not from a second copy in the interface that can drift.
	RepoEffective string `json:"repoEffective"`
	// EffectiveSchedule is what actually happens to this set: which schedule
	// backs it up, whether two of them do, or whether none does. Computed here
	// rather than in the interface so the sentence on the Folders card comes from
	// the same rules the scheduler runs, not from a second copy that can drift
	// (#199: three controls that all read like scheduling, and the combination
	// that looks most sensible silently protects nothing).
	EffectiveSchedule schedule.EffectiveSchedule `json:"effectiveSchedule"`
	Placement         placementView              `json:"placement"`
}

// ListFileSetViews returns all configured file sets with their last-backup
// time and source-path existence.
func (s *Service) ListFileSetViews(ctx context.Context) ([]FileSetView, error) {
	sets, err := s.store.ListFileSets()
	if err != nil {
		return nil, fmt.Errorf("list file sets: %w", err)
	}
	// Read once for the whole list: the effective schedule depends on four
	// settings fields, and asking per set would issue N identical reads.
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	// Dated from the backups a set owns, as on the container and VM lists.
	var snapTimes map[string]int64
	snapTimesFailed := false
	if len(sets) > 0 {
		if m, sErr := s.LatestFileSetBackupTimes(ctx); sErr != nil {
			log.Printf("api: list file sets: latest backup times: %v", sErr)
			snapTimesFailed = true
		} else {
			snapTimes = m
		}
	}
	views := make([]FileSetView, 0, len(sets))
	for _, set := range sets {
		v := FileSetView{
			ID:                set.ID,
			Name:              set.Name,
			Path:              set.Path,
			Excludes:          set.Excludes,
			Enabled:           set.Enabled,
			ScheduleCadence:   set.ScheduleCadence,
			Repo:              set.Repo,
			EffectiveSchedule: schedule.EffectiveFileSetSchedule(set, settings),
		}
		// Resolved through the same helper the backup uses, so the card and the
		// run can never disagree about where this set goes. An unresolvable
		// override is shown as the raw stored value rather than swallowed: a
		// location that cannot resolve is exactly what the user has to see.
		if eff, rErr := s.fileSetRepoPath(settings, set); rErr == nil {
			v.RepoEffective = eff
		} else {
			v.RepoEffective = set.Repo
		}
		if v.Excludes == nil {
			v.Excludes = []string{}
		}
		// The stored form is served back verbatim, never re-normalized here: the
		// compile (fileSetPositionals) re-runs the shared NormalizeSelection, and
		// the view mirrors the column. nil stays nil so omitempty drops the key
		// (the NULL column).
		v.SelectedPaths = set.SelectedPaths
		run, _ := s.store.LastSuccessfulBackup(set.ID)
		if finished, _ := lastBackupDate(set.Name, run, snapTimes, snapTimesFailed); finished != nil {
			v.LastBackup = *finished
		}
		if resolved, rErr := paths.Resolve(s.cfg.HostMountRoot, set.Path); rErr == nil {
			if _, statErr := os.Stat(resolved); statErr == nil { //nolint:gosec // G703: resolved is containment-validated under the host mount root
				v.PathExists = true
			}
		}
		views = append(views, v)
	}
	return views, nil
}

// validateFileSet guards everything a file set feeds into: the name becomes a
// restic tag and a progress key, so it passes the same strict charset as
// container names; the path must be a relative subpath under the host mount,
// and, when checkPathExists is true, must also exist on disk. A patch checks
// the path only when it changes the path or enables the set, so a dead path
// does not block the rest of the set's settings. A path-less set is valid
// while it stays disabled, the shape DiscoverFileSets creates from fileset:
// tags alone.
func (s *Service) validateFileSet(fs store.FileSet, checkPathExists bool) error {
	if !validResourceName(fs.Name) {
		return errors.New("invalid file set name (letters, digits, . _ - only; must start with a letter or digit)")
	}
	if strings.TrimSpace(fs.Path) == "" {
		if fs.Enabled {
			return errors.New("file set has no source path: set a path before enabling it")
		}
		return nil
	}
	resolved, err := paths.Resolve(s.cfg.HostMountRoot, fs.Path)
	if err != nil {
		return errors.New("invalid path: must be a relative subpath under the host mount")
	}
	if checkPathExists {
		if _, statErr := os.Stat(resolved); statErr != nil { //nolint:gosec // G703: resolved is containment-validated under the host mount root
			return errors.New("source path not found under the host mount")
		}
	}
	return nil
}

// maxFileSetSelectedPaths caps one file-set selection: a tree save
// overwrites the whole column, so the cap bounds both the JSON blob and the
// normalize and compile work per save. It matches the ceiling
// SetExcludeCaches puts on the per-root exclusion map, so no editor surface
// can differ.
const maxFileSetSelectedPaths = 64

// SetFileSetSelectedPaths validates and stores a file set's tree
// selection. It is the file-set twin of SetBackupPaths (the containers'
// flat-set setter) and the only writer the tree editor reaches (the PATCH
// handler's selectedPaths field lands here). Unlike SetBackupPaths, the
// anchor is the set's own resolved root (its Path under the host mount),
// not a mount translation, and entries live in mount-root absolute space;
// they are never container-translated.
//
// Per-entry validation runs before any store write, so the whole save is
// rejected atomically: a list whose 40th entry escapes the root must leave
// the prior selection untouched, not half-apply. Each entry is trimmed,
// split into its class (SplitExclusion; the "!" prefix marks an excluded
// branch), cleaned, and must equal the set's resolved root or lie strictly
// below it (isStrictDescendant, segment-aligned, so /data/doc can never
// pass for a root /data/docs; fileSetPositionals re-anchors with the same
// primitive at compile time as a second layer). The raw entry text goes
// back into the error so the log identifies the offender; the envelope
// scrubs it to [path] on the way out.
//
// The validated list is then normalized through the shared
// NormalizeSelection (dedupe, per-class maximal-root prune, canonical
// order; never a second pruning site), and only a list that still has at
// least one include survives: zero includes is refused with
// errFileSetEmptySelection before any write, so a refused deselect leaves
// the prior selection untouched. The read-side stale-root re-anchor in
// fileSetPositionals remains the defense for anything already stored.
func (s *Service) SetFileSetSelectedPaths(_ context.Context, id string, entries []string) error {
	if len(entries) > maxFileSetSelectedPaths {
		return fmt.Errorf("too many selected paths (%d, max %d)", len(entries), maxFileSetSelectedPaths)
	}
	set, err := s.store.GetFileSet(id)
	if err != nil {
		return errFileSetNotFound
	}
	root, err := paths.Resolve(s.cfg.HostMountRoot, set.Path)
	if err != nil {
		return fmt.Errorf("file set %q has no valid source path to select folders under", set.Name)
	}
	cleaned := make([]string, 0, len(entries))
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" {
			return errors.New("empty selected path")
		}
		bare, excluded := SplitExclusion(e)
		if bare == "" {
			return fmt.Errorf("empty excluded path %q", e)
		}
		bare = path.Clean(bare)
		if bare != root && !isStrictDescendant(bare, root) {
			return fmt.Errorf("selected path %q is not under the set's source folder", e)
		}
		if excluded {
			bare = ExclusionPrefix + bare
		}
		cleaned = append(cleaned, bare)
	}
	normalized := NormalizeSelection(cleaned)
	if len(includesOnly(normalized)) == 0 {
		return errFileSetEmptySelection
	}
	if err := s.store.SetFileSetSelectedPaths(id, normalized); err != nil {
		return fmt.Errorf("store file set selection: %w", err)
	}
	return nil
}

// errFileSetRepoUnreachable means fileSetHasBackups could not ask the set's
// repository: it is established but not mounted, or its established marker
// could not be read. The set then counts as having backups.
var errFileSetRepoUnreachable = errors.New("its repository could not be checked right now (not reachable)")

// fileSetHasBackups reports whether the file set id already has at least one
// recorded successful backup run, i.e. fileset:<Name>-tagged snapshots exist
// in the repo. handlePatchFileSet refuses a name change when this is true,
// since those snapshots stay tagged with the old name and are never
// re-tagged.
func (s *Service) fileSetHasBackups(ctx context.Context, id string) (bool, error) {
	run, err := s.store.LastSuccessfulBackup(id)
	if err != nil {
		return false, err
	}
	if run != nil {
		return true, nil
	}
	// The listing below reads a missing local repo directory as never
	// established, even when the marker could not be read, so that case is
	// caught here first.
	set, sErr := s.store.GetFileSet(id)
	if sErr != nil {
		return false, errFileSetNotFound
	}
	settings, gErr := s.store.GetSettings()
	if gErr != nil {
		return false, fmt.Errorf("read settings: %w", gErr)
	}
	if repo, rErr := s.fileSetRepoPath(settings, set); rErr == nil {
		if s.repoEstablishmentOf(repo) == repoEstablishmentUnknown {
			return true, errFileSetRepoUnreachable
		}
	}
	// A Discover-rebuilt set has real fileset:<Name> snapshots in the repo
	// but a fresh id with no run rows, so the runs table alone would miss it.
	snaps, err := s.SnapshotsFileSet(ctx, id, "local")
	if err != nil {
		if errors.Is(err, errFileSetNotFound) {
			return false, err
		}
		if errors.Is(err, ErrBackupPathNotMounted) {
			return true, errFileSetRepoUnreachable
		}
		return true, fmt.Errorf("%w: %v", errFileSetRepoUnreachable, err)
	}
	return len(snaps) > 0, nil
}

// fileSetNameAdoptable decides whether a create, or a repository or
// pre-backup name change, may land a file set on name: it refuses when
// fileset:<name> snapshots already exist in the repository this set will
// use, unless every one of them recorded a path at or below the resolved
// source, the same folder coming back. A listing failure refuses too, as in
// fileSetHasBackups.
func (s *Service) fileSetNameAdoptable(ctx context.Context, name, repoOverride, path string) error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.fileSetRepoPath(settings, store.FileSet{Name: name, Repo: repoOverride})
	if err != nil {
		return err
	}
	mode := s.repoModeFor(settings, "files", "local", repo)
	snaps, err := s.snapshotsForTag(ctx, repo, mode, "fileset:"+name)
	if err != nil {
		return fmt.Errorf("%q: leftover backups could not be checked: %w", name, err)
	}
	if len(snaps) == 0 {
		return nil
	}
	resolved, rErr := paths.Resolve(s.cfg.HostMountRoot, path)
	if rErr != nil {
		return fmt.Errorf("%q already has backups; set a valid path so its source can be compared, or bring the old set back with Discover, or delete its old snapshots first", name)
	}
	for _, snap := range snaps {
		if !snapshotPathsMatchRoot(snap.Paths, resolved) {
			return fmt.Errorf("%q already has backups from a different source folder; bring the old set back with Discover, or delete its old snapshots first", name)
		}
	}
	return nil
}

// snapshotPathsMatchRoot reports whether every one of a snapshot's recorded
// paths is the resolved root itself or lies strictly below it. A set with a
// tree selection records the selected subpaths, never the root, so this is
// what tells "the same folder coming back" from a recorded parent or a
// disjoint folder wearing the same name.
func snapshotPathsMatchRoot(recorded []string, resolved string) bool {
	if len(recorded) == 0 {
		return false
	}
	for _, p := range recorded {
		if p != resolved && !isStrictDescendant(p, resolved) {
			return false
		}
	}
	return true
}

// containerHasBackups / vmHasBackups are fileSetHasBackups for the other two
// domains. An item's snapshots stay in the repository they were written to,
// so re-pointing an item that already has some splits its history, and the
// old half stays invisible, never pruned and reachable only through restic by
// hand.
//
// The runs table alone is not enough: an item rebuilt by Discover after a
// /config loss has real snapshots but a fresh id with no run rows. The
// interface's lastBackup lock misses that case, so the server refuses.
func (s *Service) containerHasBackups(ctx context.Context, name string) (bool, error) {
	tg, err := s.store.GetTargetByContainer(name)
	if err == nil {
		if run, rErr := s.store.LastSuccessfulBackup(tg.ID); rErr == nil && run != nil {
			return true, nil
		}
	}
	id := s.containerIdentity(name)
	if id.readErr != nil {
		return false, fmt.Errorf("its backups could not be checked: %w", id.readErr)
	}
	snaps, err := s.containerSnapshotsOf(ctx, name, "local", id)
	if err != nil {
		// Unreadable is not "empty": refusing conservatively is the safe way
		// round, because the cost of being wrong the other way is a split
		// history nobody can see.
		return true, nil //nolint:nilerr // unknown counts as "has backups"
	}
	return len(snaps) > 0, nil
}

func (s *Service) vmHasBackups(ctx context.Context, name string) (bool, error) {
	vm, err := s.store.GetVMTargetByName(name)
	if err == nil {
		if run, rErr := s.store.LastSuccessfulBackup(vm.ID); rErr == nil && run != nil {
			return true, nil
		}
	}
	id := s.vmIdentity(name)
	if id.readErr != nil {
		return false, fmt.Errorf("its backups could not be checked: %w", id.readErr)
	}
	snaps, err := s.vmSnapshotsOf(ctx, name, "local", id)
	if err != nil {
		return true, nil //nolint:nilerr // see containerHasBackups
	}
	return len(snaps) > 0, nil
}

// SnapshotsFileSet lists restic snapshots for a single file set, filtered
// by the "fileset:<Name>" tag its backups write; it is the files
// counterpart of SnapshotsVM. id is the set's stable store id; source
// selects the local or off-site repo.
func (s *Service) SnapshotsFileSet(ctx context.Context, id, source string) ([]restic.Snapshot, error) {
	set, err := s.store.GetFileSet(id)
	if err != nil {
		return nil, errFileSetNotFound
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.fileSetRepoFor(settings, set, source)
	if err != nil {
		return nil, err
	}
	mode := s.repoModeFor(settings, "files", source, repo)
	// A listing before any backup has run is "no snapshots yet", not an error.
	if localRepoMissing(repo) {
		// #55 vs #120: only surface "not mounted" when the backing store is truly
		// absent. If the destination is mounted, this is a fresh or phantom repo
		// on a healthy disk, so report an empty list (EnsureRepo re-establishes on
		// write).
		if s.repoEstablished(repo) && !s.destinationMounted(repo) {
			return nil, ErrBackupPathNotMounted // #55: backing store not mounted
		}
		return nil, nil
	}
	all, err := s.listSnapshots(ctx, repo, mode)
	if err != nil {
		return nil, err
	}
	tag := "fileset:" + set.Name
	out := make([]restic.Snapshot, 0, len(all))
	for _, snap := range all {
		for _, t := range snap.Tags {
			if t == tag {
				out = append(out, snap)
				break
			}
		}
	}
	return out, nil
}

// ListSnapshotFilesFileSet lists the files in a file-set snapshot for the
// selective (pick-some-files) restore, the files counterpart of
// ListSnapshotFiles. snapshotID must be valid hex and must belong to this
// set (tag-scoped via SnapshotsFileSet and snapshotBelongs), so one set's
// file tree can't be listed through another's route.
func (s *Service) ListSnapshotFilesFileSet(ctx context.Context, id, snapshotID, source string) ([]restic.FileEntry, error) {
	if !backup.ValidSnapshotID(snapshotID) {
		return nil, backup.ErrInvalidSnapshotID
	}
	snaps, err := s.SnapshotsFileSet(ctx, id, source)
	if err != nil {
		return nil, err
	}
	if !snapshotBelongs(snaps, snapshotID) {
		return nil, notInListing{snapshotID, "file set"}
	}
	// Loaded for its repository override (#204): a set with its own repo is
	// listed from that repo, and reading the domain's would report "snapshot
	// not found" for a snapshot that exists.
	set, err := s.store.GetFileSet(id)
	if err != nil {
		return nil, errFileSetNotFound
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.fileSetRepoFor(settings, set, source)
	if err != nil {
		return nil, err
	}
	return s.lsSelfHeal(ctx, repo, snapshotID, s.repoModeFor(settings, "files", source, repo))
}

// fileSetRestorePlan carries everything prepareRestoreFileSet validated and
// resolved so the restic work can run detached from the request that asked for
// it (StartRestoreFileSet), mirroring toPathRestorePlan.
type fileSetRestorePlan struct {
	repo       string
	mode       restic.Mode
	snapshotID string
	setID      string // runs.target_id the detached run is recorded against
	setName    string // progress key suffix ("files:<name>")
	inPlace    string // in-place: the set's resolved source path (engine.RestorePath); "" = to-folder
	target     string // to-folder: the resolved alternate folder under the host mount ("" = in-place)
	subtree    string // to-folder: the snapshot's own tree node covering all its recorded paths, the <id>:<subtree> restore root ("" = path-less snapshot or roots with no shared ancestor -> whole-tree fallback)
}

// prepareRestoreFileSet performs all of a file-set restore's validation and
// resolution synchronously, so a bad request fails immediately with a
// clear error, and creates the alternate target folder once containment
// passes.
//
// Security: the snapshot id passes the strict hex guard
// (backup.ValidSnapshotID) and must belong to this set (tag-scoped via
// SnapshotsFileSet and snapshotBelongs, like prepareRestoreToPath), so one
// set's data can't be extracted through another's route. An empty
// targetSubPath restores in place over the set's source folder, which is
// destructive and therefore confirm-gated; a non-empty targetSubPath is
// resolved with paths.Resolve under the host mount (rejecting absolute and
// `..` escapes) and created only after containment passes, which is
// non-destructive and needs no confirm (as in RestoreContainerToPath).
func (s *Service) prepareRestoreFileSet(ctx context.Context, id, snapshotID, source, targetSubPath string, confirm bool) (fileSetRestorePlan, error) {
	set, err := s.store.GetFileSet(id)
	if err != nil {
		return fileSetRestorePlan{}, errFileSetNotFound
	}
	if source != "local" && !isOffsiteSource(source) {
		return fileSetRestorePlan{}, errors.New("invalid source (must be local or offsite)")
	}
	if !backup.ValidSnapshotID(snapshotID) {
		return fileSetRestorePlan{}, backup.ErrInvalidSnapshotID
	}

	plan := fileSetRestorePlan{snapshotID: snapshotID, setID: set.ID, setName: set.Name}
	if sub := strings.TrimSpace(targetSubPath); sub != "" {
		// Alternate folder: shared containment helper (path.Cleans the input and
		// rejects an absolute path or any "../" that would escape the mount).
		t, rErr := paths.Resolve(s.cfg.HostMountRoot, sub)
		if rErr != nil {
			return fileSetRestorePlan{}, errors.New("invalid target folder: must be a relative subpath under the host mount")
		}
		plan.target = t
	} else {
		// In place writes over the set's source folder, so require the explicit
		// confirmation (the same sentinel discipline as prepareRestore).
		if !confirm {
			return fileSetRestorePlan{}, backup.ErrNotConfirmed
		}
		// A discovered, path-less set has no original location to restore to; say
		// so instead of letting paths.Resolve report a misleading traversal error
		// for "" (restore to a folder works without a path).
		if strings.TrimSpace(set.Path) == "" {
			return fileSetRestorePlan{}, fmt.Errorf("file set %q has no source path configured: restore to a folder instead, or set a path first", set.Name)
		}
		src, rErr := paths.Resolve(s.cfg.HostMountRoot, set.Path)
		if rErr != nil {
			return fileSetRestorePlan{}, errors.New("invalid file set path: must be a relative subpath under the host mount")
		}
		plan.inPlace = src
	}

	// Scope to this set: the snapshot must be one of its snapshots.
	snaps, err := s.SnapshotsFileSet(ctx, id, source)
	if err != nil {
		return fileSetRestorePlan{}, err
	}
	if !snapshotBelongs(snaps, snapshotID) {
		return fileSetRestorePlan{}, notInListing{snapshotID, "file set"}
	}
	// Take the to-folder restore subtree from the snapshot itself, not a
	// recompute of set.Path: HostMountRoot may have changed since the backup,
	// and a recomputed <id>:<path> selector would then miss and fail. It is
	// the node covering every path the snapshot recorded; a set whose
	// selection is two sub-folders records two roots, and taking only the
	// first would restore half the set while reporting success. Empty (a
	// path-less snapshot) falls back to a whole-tree restore in
	// runRestoreFileSet; it is unused for an in-place restore.
	plan.subtree = snapshotRestoreRoot(snaps, snapshotID)

	// An in-place restore writes back over the set's source folder, so before
	// anything destructive the set's compiled selection (the
	// fileSetPositionals the backup ran, so guard and backup never disagree
	// about what the set selects) is mapped against the chosen snapshot's
	// recorded Paths; selectors come from the snapshot, never replayed from
	// storage. An empty intersection is refused here, synchronously: the
	// failure mode the container route's mapRestorePaths guard prevents, a
	// restore that would miss mid-loop after the folder was torn down.
	//
	// Only the in-place route does this: a to-folder restore is
	// non-destructive and its subtree comes from the snapshot, so a selection
	// that no longer matches it must not abort (TestRestoreFileSetToFolder's
	// cross-root snapshot keeps restoring). Snapshots with no recorded Paths
	// have nothing to map against and keep restoring whole.
	if plan.inPlace != "" {
		if chosen := chosenSnapshot(snaps, snapshotID); chosen != nil && len(chosen.Paths) > 0 {
			compiled := fileSetPositionals(set.SelectedPaths, plan.inPlace)
			// Neither skipped nor the third return is used: this guard only answers
			// "is the intersection empty", and the restore it guards hands restic the
			// set's own root (plan.inPlace), never one of these mapped paths, so no
			// unchecked selector can reach the engine and there are no per-path skips
			// to report.
			mapped, _, _ := mapRestorePaths(compiled, chosen.Paths)
			if len(mapped) == 0 {
				return fileSetRestorePlan{}, errors.New("nothing to restore for this set from this snapshot")
			}
		}
	}

	settings, err := s.store.GetSettings()
	if err != nil {
		return fileSetRestorePlan{}, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.fileSetRepoFor(settings, set, source)
	if err != nil {
		return fileSetRestorePlan{}, err
	}
	plan.repo = repo
	plan.mode = s.repoModeFor(settings, "files", source, repo)

	// Create the alternate target dir only after every validation passed,
	// with the readable (0o755) variant: the restore target lives on a
	// user-visible or synced share, so the operator's non-root SMB user must be
	// able to read what root restored there (see EnsureDirReadable).
	if plan.target != "" {
		if err := paths.EnsureDirReadable(plan.target); err != nil {
			return fileSetRestorePlan{}, fmt.Errorf("create target folder: %w", err)
		}
	}
	return plan, nil
}

// runRestoreFileSet restores an already-validated file-set plan: in place
// (restic restores the set's source path back to its own location) or the
// whole snapshot tree into the alternate target folder.
func (s *Service) runRestoreFileSet(ctx context.Context, plan fileSetRestorePlan) error {
	// Hold the domain repo lock for the restic work: scheduled backups bypass
	// batchActive and the domain lock is the layer they respect (see
	// executeRestore).
	unlock := s.lockDomainFor("files", "restore")
	defer unlock()
	if plan.inPlace != "" {
		return s.engine.RestorePath(ctx, plan.repo, plan.snapshotID, plan.inPlace, plan.mode)
	}
	if plan.subtree != "" {
		// Restore the snapshot's own subtree directly into the chosen folder, so
		// its contents land at <target>/… and not <target>/host/user/… (#62's
		// nested restore, which a bare RestoreInclude("/") produces).
		return s.engine.RestoreSubtreeTo(ctx, plan.repo, plan.snapshotID, plan.subtree, plan.target, plan.mode)
	}
	// Degenerate fallback: a snapshot with no recorded path. Restore the whole
	// tree rather than emit an invalid "<id>:" selector.
	return s.engine.RestoreInclude(ctx, plan.repo, plan.snapshotID, "/", plan.target, plan.mode)
}

// StartRestoreFileSet launches a file-set restore in a background
// goroutine and returns immediately (see StartRestoreToPath for why). All
// validation runs synchronously (a bad request fails right away, no
// goroutine); the resolved alternate target folder ("" for an in-place
// restore) is returned in the ack so the UI can show it. The detached run
// publishes "files:<name>" progress (phase "restore"), registers a cancel
// key, and records a run (kind "restore") against the set's stable id, so
// the outcome, including the real restic error text, lands in the run
// history.
//
// Shares batchActive with backups and the other restores; returns
// ("", false, nil) when one is already running.
func (s *Service) StartRestoreFileSet(ctx context.Context, id, snapshotID, source, targetSubPath string, confirm bool) (string, bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return "", false, nil
	}
	plan, err := s.prepareRestoreFileSet(ctx, id, snapshotID, source, targetSubPath, confirm)
	if err != nil {
		s.batchActive.Store(false)
		return "", false, err
	}
	bctx := context.WithoutCancel(ctx)
	rkey := "files:" + plan.setName // the exact progBegin key this restore publishes under
	go func() {
		var runID string // see StartRestoreFiles's identical goroutine for why this is declared here
		defer s.recoverOperation("restore file set: "+id, nil, func(msg string) {
			// A panic is a genuine failure, never restic.ErrRestoreMetadataOnly, so
			// finishRestoreRun directly (bypassing concludeFileSetRestore's
			// metadata-only downgrade) is the correct, simpler call here.
			s.finishRestoreRun(runID, "", errors.New(msg))
		})
		defer s.batchActive.Store(false)
		tctx, tcancel := context.WithTimeout(bctx, restoreTimeout)
		defer tcancel()
		rctx, cancel := context.WithCancel(tctx)
		defer cancel()
		s.registerCancel(rkey, cancel)
		defer s.unregisterCancel(rkey)
		runID = s.beginRestoreRunForTarget(plan.setID)
		pctx, startedAt := s.progBegin(rctx, rkey, "restore")
		rerr := s.runRestoreFileSet(pctx, plan)
		if err := s.concludeFileSetRestore(runID, rkey, plan.snapshotID, rerr, startedAt); err != nil {
			log.Printf("api: restore file set: %q failed: %v", plan.setName, err) //nolint:gosec // G706: name is %q-quoted
		}
	}()
	return plan.target, true, nil
}

// fileSetFilesRestorePlan carries everything prepareRestoreFileSetFiles
// validated and resolved so the restic work can run detached from the
// request that asked for it (StartRestoreFileSetFiles); it is the
// selective (pick-some-files) counterpart of fileSetRestorePlan.
type fileSetFilesRestorePlan struct {
	repo       string
	mode       restic.Mode
	snapshotID string
	setID      string   // runs.target_id the detached run is recorded against
	setName    string   // progress key suffix ("files:<name>")
	paths      []string // cleaned selection, containment-validated
	subtree    string   // the snapshot's own backed-up root (snapshotRestoreRoot); "" = path-less snapshot
	target     string   // to-folder: the resolved alternate folder under the host mount ("" = in-place)
}

// prepareRestoreFileSetFiles performs all of a selective file-set
// restore's validation and resolution synchronously, so a bad request
// fails immediately with a clear error, and creates the alternate target
// folder once containment passes. It is the files-domain counterpart of
// prepareRestoreFiles (containers).
//
// Security: confirm-gated like the container file-level restore; the
// snapshot must belong to this set (tag-scoped via SnapshotsFileSet), and
// buildFileSetFilesPlan applies the hex guard and the containment rules.
func (s *Service) prepareRestoreFileSetFiles(ctx context.Context, id, source, snapshotID string, filePaths []string, targetSubPath string, confirm bool) (fileSetFilesRestorePlan, error) {
	if !confirm {
		return fileSetFilesRestorePlan{}, backup.ErrNotConfirmed
	}
	set, err := s.store.GetFileSet(id)
	if err != nil {
		return fileSetFilesRestorePlan{}, errFileSetNotFound
	}
	if source != "local" && !isOffsiteSource(source) {
		return fileSetFilesRestorePlan{}, errors.New("invalid source (must be local or offsite)")
	}
	// Cheap guards before the snapshot listing: a malformed id or an empty
	// selection must fail fast without a (possibly remote, slow) restic
	// snapshots call. buildFileSetFilesPlan re-checks both, so the shared path
	// stays safe on its own.
	if !backup.ValidSnapshotID(snapshotID) {
		return fileSetFilesRestorePlan{}, backup.ErrInvalidSnapshotID
	}
	if len(filePaths) == 0 {
		return fileSetFilesRestorePlan{}, errors.New("no files selected")
	}
	// Scope to this set: the snapshot must be one of its snapshots.
	snaps, err := s.SnapshotsFileSet(ctx, id, source)
	if err != nil {
		return fileSetFilesRestorePlan{}, err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return fileSetFilesRestorePlan{}, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.fileSetRepoFor(settings, set, source)
	if err != nil {
		return fileSetFilesRestorePlan{}, err
	}
	return s.buildFileSetFilesPlan(snaps, snapshotID, set.ID, set.Name, repo, s.repoModeFor(settings, "files", source, repo), filePaths, targetSubPath)
}

// buildFileSetFilesPlan builds a validated selective plan from already
// resolved snaps, repo, mode and set identity. It is the shared core of
// the local (settings-driven) prepareRestoreFileSetFiles and the foreign
// (session-driven) prepareForeignFileSetFilesRestore, so the containment
// and target guards are written once.
//
// Security: the snapshot id passes the strict hex guard
// (backup.ValidSnapshotID) and must belong to the passed snaps (tag-scoped
// by the caller via SnapshotsFileSet or snapshotsForTag), so one set's
// data can't be extracted through another's route. An empty targetSubPath
// restores in place (each selected path back to its absolute location,
// restic target "/"), so every path is re-validated inside the host mount
// (paths.Within). A non-empty targetSubPath is resolved with paths.Resolve
// under the host mount (rejecting absolute and `..` escapes) and, when the
// snapshot has a backed-up root, every selected path must sit within that
// subtree, since the selection feeds --include patterns. The target dir is
// created (EnsureDirReadable, 0o755) only after all containment passes.
func (s *Service) buildFileSetFilesPlan(snaps []restic.Snapshot, snapshotID, setID, setName, repo string, mode restic.Mode, filePaths []string, targetSubPath string) (fileSetFilesRestorePlan, error) {
	if !backup.ValidSnapshotID(snapshotID) {
		return fileSetFilesRestorePlan{}, backup.ErrInvalidSnapshotID
	}
	if len(filePaths) == 0 {
		return fileSetFilesRestorePlan{}, errors.New("no files selected")
	}

	// Clean each selected path once, so the validated path is the path that runs.
	cleaned := make([]string, 0, len(filePaths))
	for _, p := range filePaths {
		cleaned = append(cleaned, path.Clean(p))
	}

	if !snapshotBelongs(snaps, snapshotID) {
		return fileSetFilesRestorePlan{}, notInListing{snapshotID, "file set"}
	}
	// The subtree comes from the snapshot itself (the node covering all of its
	// recorded paths, so a file under a second recorded root still passes this
	// guard), not a recompute of set.Path, since HostMountRoot may have changed
	// since the backup. A "." or "/" clean means a path-less snapshot (a
	// degenerate discovered set).
	subtree := path.Clean(snapshotRestoreRoot(snaps, snapshotID))
	if subtree == "." || subtree == "/" {
		subtree = ""
	}

	plan := fileSetFilesRestorePlan{
		repo:       repo,
		mode:       mode,
		snapshotID: snapshotID,
		setID:      setID,
		setName:    setName,
		paths:      cleaned,
		subtree:    subtree,
	}

	if sub := strings.TrimSpace(targetSubPath); sub != "" {
		t, rErr := paths.Resolve(s.cfg.HostMountRoot, sub)
		if rErr != nil {
			return fileSetFilesRestorePlan{}, errors.New("invalid target folder: must be a relative subpath under the host mount")
		}
		// When the snapshot has a backed-up root, every selection must sit within
		// it: the selection becomes an --include relative to that subtree, so a
		// path outside it would be a client trying to reach beyond the set. A
		// path-less snapshot has no root to scope against; the whole-path include
		// fallback in runRestoreFileSetFiles is contained by --target alone.
		if subtree != "" {
			for _, c := range cleaned {
				if c != subtree && !strings.HasPrefix(c, subtree+"/") {
					return fileSetFilesRestorePlan{}, errors.New("restore file: selected path is outside the file set snapshot")
				}
			}
		}
		plan.target = t
	} else {
		// In place writes each path back to its absolute location, so every path must
		// sit within the host mount (defense-in-depth), exactly like the container
		// in-place file restore.
		for _, c := range cleaned {
			if !paths.Within(s.cfg.HostMountRoot, c) {
				return fileSetFilesRestorePlan{}, errors.New("restore file: path is outside the backup mount")
			}
		}
	}

	// Create the alternate target dir only after every validation passed, with
	// the readable (0o755) variant, so the operator's non-root SMB user can
	// read what root restored to the synced share (see EnsureDirReadable).
	if plan.target != "" {
		if err := paths.EnsureDirReadable(plan.target); err != nil {
			return fileSetFilesRestorePlan{}, fmt.Errorf("create target folder: %w", err)
		}
	}
	return plan, nil
}

// runRestoreFileSetFiles restores each selected path of an
// already-validated selective plan. Like runRestoreFiles (containers) it is
// not atomic, since restic writes per path, so a mid-batch failure says how
// many already went through and which path stopped it.
func (s *Service) runRestoreFileSetFiles(ctx context.Context, plan fileSetFilesRestorePlan) error {
	// Hold the domain repo lock for the restic work (see runRestoreFileSet).
	unlock := s.lockDomainFor("files", "restore")
	defer unlock()
	for i, c := range plan.paths {
		if err := s.restoreOneFileSetFile(ctx, plan, c); err != nil {
			if len(plan.paths) > 1 {
				return fmt.Errorf("restored %d of %d files, then failed on %q: %w", i, len(plan.paths), c, err)
			}
			return err
		}
	}
	return nil
}

// restoreOneFileSetFile restores a single selected path of an
// already-validated plan. In place (empty target) writes it back to its
// absolute location (RestoreInclude to "/"). To a folder, the restore is
// rooted at the selection's parent and includes only its basename, so it
// lands at <target>/<name> and not <target>/host/user/… (#62's nesting). A
// selection without a usable parent restores its contents straight into
// the target.
func (s *Service) restoreOneFileSetFile(ctx context.Context, plan fileSetFilesRestorePlan, sel string) error {
	if plan.target == "" {
		// In place: restore each selected path back to its own absolute location.
		// Same glob rule as the to-folder branch above: sel is a picked path.
		return s.engine.RestoreInclude(ctx, plan.repo, plan.snapshotID, escapeGlobLiteral(sel), "/", plan.mode)
	}
	// To a folder: root the restore at the selection's immediate parent and
	// include only its basename, so the picked file or folder lands directly
	// as <target>/<name> with no intermediate tree. Rooted at the file set's
	// backed-up root, the whole path between that root and a deeply nested
	// selection would be recreated under the target (selecting
	// dockhand/stacks/DXP480T/xo would give <target>/stacks/DXP480T/xo instead
	// of <target>/xo, #123). restic includes ancestor path nodes, so rooting
	// at the parent is valid for files and folders at any depth.
	parent := path.Dir(sel)
	base := path.Base(sel)
	if parent == "." || parent == "/" || base == "." || base == "/" || base == "" {
		// Degenerate (no usable parent or name): drop the selection's contents in.
		return s.engine.RestoreSubtreeTo(ctx, plan.repo, plan.snapshotID, sel, plan.target, plan.mode)
	}
	// The subtree root travels as a selector (not a pattern) and stays raw;
	// the include is a glob and is escaped.
	return s.engine.RestoreSubtreeInclude(ctx, plan.repo, plan.snapshotID, parent, escapeGlobLiteral("/"+base), plan.target, plan.mode)
}

// StartRestoreFileSetFiles launches a selective file-set restore in a
// background goroutine and returns immediately (see StartRestoreFileSet).
// All validation runs synchronously (a bad request fails right away, no
// goroutine); the resolved alternate target folder ("" for an in-place
// restore) is returned in the ack so the UI can show it. The detached run
// publishes "files:<name>" progress (phase "restore"), registers a cancel
// key, and records a run (kind "restore") against the set's stable id,
// with the same metadata-error tolerance as the whole-set restore
// (concludeFileSetRestore).
//
// Shares batchActive with backups and the other restores; returns
// ("", false, nil) when one is already running.
func (s *Service) StartRestoreFileSetFiles(ctx context.Context, id, source, snapshotID string, filePaths []string, targetSubPath string, confirm bool) (string, bool, error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return "", false, nil
	}
	plan, err := s.prepareRestoreFileSetFiles(ctx, id, source, snapshotID, filePaths, targetSubPath, confirm)
	if err != nil {
		s.batchActive.Store(false)
		return "", false, err
	}
	bctx := context.WithoutCancel(ctx)
	rkey := "files:" + plan.setName // the exact progBegin key this restore publishes under
	go func() {
		var runID string // see StartRestoreFiles's identical goroutine for why this is declared here
		defer s.recoverOperation("restore file set files: "+id, nil, func(msg string) {
			s.finishRestoreRun(runID, "", errors.New(msg)) // see StartRestoreFileSet for why not concludeFileSetRestore
		})
		defer s.batchActive.Store(false)
		tctx, tcancel := context.WithTimeout(bctx, restoreTimeout)
		defer tcancel()
		rctx, cancel := context.WithCancel(tctx)
		defer cancel()
		s.registerCancel(rkey, cancel)
		defer s.unregisterCancel(rkey)
		runID = s.beginRestoreRunForTarget(plan.setID)
		pctx, startedAt := s.progBegin(rctx, rkey, "restore")
		rerr := s.runRestoreFileSetFiles(pctx, plan)
		if err := s.concludeFileSetRestore(runID, rkey, plan.snapshotID, rerr, startedAt); err != nil {
			log.Printf("api: restore file set files: %q failed: %v", plan.setName, err) //nolint:gosec // G706: name is %q-quoted
		}
	}()
	return plan.target, true, nil
}

// DeleteBackupsFileSet removes every backup of a file set. From the local source
// it also forgets the set; from an off-site source it deletes at that target only
// and the set stays.
func (s *Service) DeleteBackupsFileSet(ctx context.Context, id, source string) error {
	// Loaded first for its repository override (#204): this deletes the
	// snapshots of one set, and they live wherever that set backs up. Reading
	// the domain repository would report "no backups to delete yet" for a set
	// whose snapshots sit in its own repo, a refusal that looks like success.
	set, err := s.store.GetFileSet(id)
	if err != nil {
		return errFileSetNotFound
	}
	if isOffsiteSource(source) {
		_, err := s.forgetAtTarget(ctx, "files", "fileset:"+set.Name, source, taggedForItem, nil)
		return err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.fileSetRepoPath(settings, set)
	if err != nil {
		return err
	}
	// Refused when this repo is a remote primary flagged append-only in its
	// safety settings (#152), the same gate pruneDomain and DeleteSnapshot use:
	// this path runs Forget with prune, which reclaims space irreversibly.
	if f := s.primaryAppendOnly("files", repo); f != appendOnlyNone {
		return appendOnlyRefusal(f)
	}
	if err := s.requireExistingRepo(repo, "no backups to delete yet"); err != nil {
		return err
	}
	unlock, ok := s.tryLockDomainFor("files", "delete")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	mode := s.repoModeFor(settings, "files", "local", repo)
	s.unlockStale(ctx, repo, mode)

	// Collect this set's snapshot IDs (tag-filtered fileset:<Name>) and
	// forget+prune them in one restic call (Forget with prune=true).
	snaps, err := s.SnapshotsFileSet(ctx, id, "local")
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(snaps))
	for _, snap := range snaps {
		ids = append(ids, snap.ID)
	}
	if len(ids) > 0 {
		if err := s.engine.Forget(ctx, repo, ids, true, mode); err != nil {
			return fmt.Errorf("forget snapshots: %w", err)
		}
	}

	// Drop the set row + its run history so the set disappears from the list
	// once its backups are gone.
	if err := s.store.DeleteFileSet(id); err != nil {
		return fmt.Errorf("delete file set: %w", err)
	}
	return nil
}
