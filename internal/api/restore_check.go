package api

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
)

type planOnlyKey struct{}

// planOnly marks a restore that is only being checked. Its preparation
// resolves and validates everything the restore would, and skips what changes
// something: creating the target folder, the ZFS safety snapshot, adopting a
// foreign item and pinning the host key.
func planOnly(ctx context.Context) context.Context {
	return context.WithValue(ctx, planOnlyKey{}, true)
}

func isPlanOnly(ctx context.Context) bool {
	on, _ := ctx.Value(planOnlyKey{}).(bool)
	return on
}

// The kinds of restore a check describes, one per restore endpoint.
const (
	checkContainer      = "container"
	checkContainerFiles = "containerFiles"
	checkContainerTo    = "containerTo"
	checkVM             = "vm"
	checkFileSet        = "fileSet"
	checkFileSetFiles   = "fileSetFiles"
	checkZFS            = "zfs"
	checkForeign        = "foreign"
	checkFlash          = "flash"
	checkConfig         = "config"
	checkDBImport       = "dbImport"
)

// RestoreCheckRequest describes a restore with what its own endpoint takes.
// Name is the container, the VM, the file set id, the ZFS item id or, for a
// foreign restore, the item.
type RestoreCheckRequest struct {
	Kind       string            `json:"kind"`
	Name       string            `json:"name"`
	SnapshotID string            `json:"snapshotId"`
	Source     string            `json:"source"`
	Paths      []string          `json:"paths"`
	TargetPath string            `json:"targetPath"`
	ZFS        ZFSRestoreRequest `json:"zfs"`
	Session    string            `json:"session"`
	Domain     string            `json:"domain"`
	Overwrite  bool              `json:"overwrite"`
	ZvolPool   string            `json:"zvolPool"`
	WholeTree  bool              `json:"wholeTree"`
}

// Check line ids, statuses and the reasons the page turns into sentences.
const (
	lineRepository = "repository"
	lineKey        = "key"
	lineSnapshot   = "snapshot"
	lineSpace      = "space"

	lineOK   = "ok"
	lineFail = "fail"
	lineSkip = "skip"

	reasonUnencrypted = "unencrypted"
	reasonNoRepo      = "no-repository"
	reasonNoSnapshot  = "no-snapshot"
	reasonDownload    = "download"
	reasonNothing     = "nothing-written"
	reasonUnmeasured  = "unmeasured"
	reasonShort       = "short"
	reasonRefused     = "refused"
)

// CheckLine is one line of the pre-flight checklist. Detail carries the
// server's own words for a failure; Need and Free are bytes.
type CheckLine struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
	Detail string `json:"detail,omitempty"`
	Need   int64  `json:"need,omitempty"`
	Free   int64  `json:"free,omitempty"`
}

// RestoreCheck is the answer to a check: the checklist, whether the restore
// may start, and what it would do.
type RestoreCheck struct {
	Ready  bool         `json:"ready"`
	Checks []CheckLine  `json:"checks"`
	Plan   *RestorePlan `json:"plan,omitempty"`
}

// restoreScope is what a prepared restore reads and where it writes, which is
// everything a check needs to know about it.
type restoreScope struct {
	ref        repoRef
	snapshotID string
	steps      []restic.PreviewStep
	inPlace    bool
	// written are the live folders an in-place restore writes into, as
	// container paths.
	written []string
	// owner is the container the restore is for, left out of the
	// shared-folder warning.
	owner     string
	container *model.Inspect
	vmName    string
	vmXML     string
	// fixedNeed is what a restore without a dry run of its own writes at
	// fixedAt, negative when it could not be measured.
	fixedNeed int64
	fixedAt   string
	download  bool
}

// CheckRestore runs the pre-flight checks for a restore and, when they get
// that far, its plan. It changes nothing, so it runs beside a backup and
// never takes the single-flight guard.
func (s *Service) CheckRestore(ctx context.Context, req RestoreCheckRequest) (RestoreCheck, error) {
	switch req.Kind {
	case checkContainer, checkContainerFiles, checkContainerTo, checkVM, checkFileSet, checkFileSetFiles,
		checkZFS, checkForeign, checkFlash, checkConfig, checkDBImport:
	default:
		return RestoreCheck{}, fmt.Errorf("unknown restore kind %q", req.Kind)
	}
	var out RestoreCheck
	ref, err := s.restoreCheckRepo(req)
	if err != nil {
		out.Checks = append(out.Checks,
			CheckLine{ID: lineRepository, Status: lineFail, Detail: scrubError(err)},
			CheckLine{ID: lineKey, Status: lineSkip, Reason: reasonNoRepo},
			CheckLine{ID: lineSnapshot, Status: lineSkip, Reason: reasonNoRepo},
			CheckLine{ID: lineSpace, Status: lineSkip, Reason: reasonNoRepo})
		return out, nil
	}
	repoLine, keyLine := probeLines(s.engine.RepoOpensErr(ctx, ref.repo, ref.mode), ref.mode)
	out.Checks = append(out.Checks, repoLine, keyLine)
	if repoLine.Status != lineOK || keyLine.Status != lineOK {
		out.Checks = append(out.Checks,
			CheckLine{ID: lineSnapshot, Status: lineSkip, Reason: reasonNoRepo},
			CheckLine{ID: lineSpace, Status: lineSkip, Reason: reasonNoRepo})
		return out, nil
	}

	scope, err := s.restoreCheckScope(planOnly(ctx), req)
	switch {
	case ctx.Err() != nil:
		return RestoreCheck{}, ctx.Err()
	case isDestinationRefusal(err):
		out.Checks = append(out.Checks,
			CheckLine{ID: lineSnapshot, Status: lineOK},
			CheckLine{ID: lineSpace, Status: lineFail, Reason: reasonRefused, Detail: scrubError(err)})
		return out, nil
	case err != nil:
		out.Checks = append(out.Checks,
			CheckLine{ID: lineSnapshot, Status: lineFail, Detail: scrubError(err)},
			CheckLine{ID: lineSpace, Status: lineSkip, Reason: reasonNoSnapshot})
		return out, nil
	}
	out.Checks = append(out.Checks, CheckLine{ID: lineSnapshot, Status: lineOK, Detail: shortID(scope.snapshotID)})

	plan, measured, err := s.previewRestore(ctx, scope)
	if err != nil {
		return RestoreCheck{}, err
	}
	out.Checks = append(out.Checks, s.spaceLine(scope, measured, plan.Partial || plan.Error != ""))
	if !scope.download {
		s.definitionChanges(ctx, scope, &plan)
		plan.Shared = s.sharedFolders(ctx, scope)
		out.Plan = &plan
	}
	out.Ready = true
	for _, c := range out.Checks {
		if c.Status == lineFail {
			out.Ready = false
		}
	}
	return out, nil
}

// probeLines reads `restic cat config`: a wrong key still proves the
// repository answered.
func probeLines(err error, mode restic.Mode) (CheckLine, CheckLine) {
	repo := CheckLine{ID: lineRepository, Status: lineOK}
	key := CheckLine{ID: lineKey, Status: lineOK}
	if !mode.Encrypted {
		key.Reason = reasonUnencrypted
	}
	switch {
	case err == nil:
	case strings.Contains(err.Error(), "wrong password or no key found"):
		key.Status, key.Detail = lineFail, scrubError(err)
	default:
		repo.Status, repo.Detail = lineFail, scrubError(err)
		key.Status, key.Reason = lineSkip, reasonNoRepo
	}
	return repo, key
}

// isDestinationRefusal reports the refusals of the destination guards, which
// say the target cannot take the restore rather than that the backup is wrong.
func isDestinationRefusal(err error) bool {
	var dest *restoreDestErr
	if errors.As(err, &dest) {
		return true
	}
	code, ok := zfsRefusalCode(err)
	return ok && (code == "destination-not-mounted" || code == "not-enough-space")
}

// restoreCheckRepo resolves only the repository a restore reads, so a check
// can say whether it answers before the preparation that needs it runs.
func (s *Service) restoreCheckRepo(req RestoreCheckRequest) (repoRef, error) {
	if req.Kind == checkForeign {
		sess, err := s.foreignSession(req.Session)
		if err != nil {
			return repoRef{}, err
		}
		return repoRef{repo: sess.repo, mode: sess.mode}, nil
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return repoRef{}, fmt.Errorf("read settings: %w", err)
	}
	var domain, repo string
	switch req.Kind {
	case checkContainer, checkContainerFiles, checkContainerTo, checkDBImport:
		domain = "containers"
		repo, err = s.containerRepoForName(settings, req.Name, req.Source)
	case checkVM:
		domain = "vms"
		repo, err = s.vmRepoForName(settings, req.Name, req.Source)
	case checkFileSet, checkFileSetFiles:
		domain = "files"
		set, gErr := s.store.GetFileSet(req.Name)
		if gErr != nil {
			return repoRef{}, errFileSetNotFound
		}
		repo, err = s.fileSetRepoFor(settings, set, req.Source)
	case checkZFS:
		domain = zfsDomain
		d, gErr := s.store.GetZFSDataset(req.Name)
		if gErr != nil {
			return repoRef{}, fmt.Errorf("zfs restore: load dataset: %w", gErr)
		}
		repo, err = s.zfsDatasetRepoFor(settings, d, req.Source)
	case checkFlash:
		domain = "flash"
		repo, err = s.repoFor(settings, domain, req.Source)
	case checkConfig:
		domain = "config"
		repo, err = s.repoFor(settings, domain, req.Source)
	}
	if err != nil {
		return repoRef{}, err
	}
	return repoRef{repo: repo, mode: s.repoModeFor(settings, domain, req.Source, repo)}, nil
}

// restoreCheckScope prepares the restore the way its own endpoint does, under
// a planOnly context, and reads the scope off the plan.
func (s *Service) restoreCheckScope(ctx context.Context, req RestoreCheckRequest) (restoreScope, error) {
	switch req.Kind {
	case checkContainer:
		plan, err := s.prepareRestore(ctx, req.Name, req.SnapshotID, true, req.Source)
		if err != nil {
			return restoreScope{}, err
		}
		return containerScope(plan, req.Name), nil
	case checkContainerFiles:
		plan, err := s.prepareRestoreFiles(ctx, req.Name, req.Source, req.SnapshotID, req.Paths, req.TargetPath, true)
		if err != nil {
			return restoreScope{}, err
		}
		sc := restoreScope{ref: repoRef{plan.repo, plan.mode}, snapshotID: plan.snapshotID, owner: req.Name, inPlace: plan.resolved == ""}
		for _, p := range plan.paths {
			sc.steps = append(sc.steps, restic.PreviewStep{SnapshotID: plan.snapshotID, Target: plan.target, Include: escapeGlobLiteral(p)})
			if sc.inPlace {
				sc.written = append(sc.written, p)
			}
		}
		return sc, nil
	case checkContainerTo:
		plan, err := s.prepareRestoreToPath(ctx, req.Name, req.Source, req.SnapshotID, req.TargetPath)
		if err != nil {
			return restoreScope{}, err
		}
		return restoreScope{
			ref: repoRef{plan.repo, plan.mode}, snapshotID: plan.snapshotID, owner: req.Name,
			steps: []restic.PreviewStep{{SnapshotID: plan.snapshotID, Target: plan.target, Include: "/"}},
		}, nil
	case checkVM:
		plan, err := s.prepareRestoreVM(ctx, req.Name, req.SnapshotID, true, req.Source)
		if err != nil {
			return restoreScope{}, err
		}
		return vmScope(plan, req.Name), nil
	case checkFileSet:
		plan, err := s.prepareRestoreFileSet(ctx, req.Name, req.SnapshotID, req.Source, req.TargetPath, true)
		if err != nil {
			return restoreScope{}, err
		}
		return fileSetScope(plan), nil
	case checkFileSetFiles:
		plan, err := s.prepareRestoreFileSetFiles(ctx, req.Name, req.Source, req.SnapshotID, req.Paths, req.TargetPath, true)
		if err != nil {
			return restoreScope{}, err
		}
		return fileSetFilesScope(plan), nil
	case checkZFS:
		zreq := req.ZFS
		// The confirmations are the dialog's own gates, not something a check
		// can fail on.
		zreq.Confirm, zreq.SafetyOffConfirm = true, true
		plan, _, err := s.prepareRestoreZFS(ctx, req.Name, req.Source, zreq)
		if err != nil {
			return restoreScope{}, err
		}
		return zfsScope(plan), nil
	case checkForeign:
		return s.foreignScope(ctx, req)
	case checkFlash, checkConfig:
		return s.selfScope(ctx, req)
	default:
		return s.dbImportScope(ctx, req)
	}
}

// containerScope mirrors backup.RestoreContainer: each appdata path back onto
// itself, or each remapped subtree into its destination.
func containerScope(plan containerRestorePlan, name string) restoreScope {
	in := plan.inspect
	sc := restoreScope{ref: repoRef{plan.repo, plan.mode}, snapshotID: plan.snapshotID, owner: name, inPlace: true, container: &in}
	switch {
	case plan.recreateOnly:
	case len(plan.restoreDirs) > 0:
		for _, d := range plan.restoreDirs {
			sc.steps = append(sc.steps, restic.PreviewStep{SnapshotID: plan.snapshotID, Subtree: d.Subtree, Target: d.Target})
			sc.written = append(sc.written, d.Target)
		}
	default:
		for _, p := range plan.appdataPaths {
			sc.steps = append(sc.steps, restic.PreviewStep{SnapshotID: plan.snapshotID, Subtree: p, Target: p})
			sc.written = append(sc.written, p)
		}
	}
	return sc
}

// vmScope mirrors backup.RestoreVM, which restores the folder of each disk.
// The NVRAM and TPM state travel over SSH and have no dry run.
func vmScope(plan vmRestorePlan, name string) restoreScope {
	sc := restoreScope{ref: repoRef{plan.repo, plan.mode}, snapshotID: plan.snapshotID, inPlace: true, vmName: name, vmXML: plan.domainXML}
	if len(plan.restoreDirs) > 0 {
		for _, d := range plan.restoreDirs {
			sc.steps = append(sc.steps, restic.PreviewStep{SnapshotID: plan.snapshotID, Subtree: d.Subtree, Target: d.Target})
			sc.written = append(sc.written, d.Target)
		}
		return sc
	}
	seen := map[string]bool{}
	for _, p := range plan.diskPaths {
		dir := path.Dir(p)
		if seen[dir] {
			continue
		}
		seen[dir] = true
		sc.steps = append(sc.steps, restic.PreviewStep{SnapshotID: plan.snapshotID, Subtree: dir, Target: dir})
		sc.written = append(sc.written, dir)
	}
	return sc
}

// fileSetScope mirrors runRestoreFileSet.
func fileSetScope(plan fileSetRestorePlan) restoreScope {
	sc := restoreScope{ref: repoRef{plan.repo, plan.mode}, snapshotID: plan.snapshotID}
	switch {
	case plan.inPlace != "":
		sc.inPlace = true
		sc.written = []string{plan.inPlace}
		sc.steps = []restic.PreviewStep{{SnapshotID: plan.snapshotID, Subtree: plan.inPlace, Target: plan.inPlace}}
	case plan.subtree != "":
		sc.steps = []restic.PreviewStep{{SnapshotID: plan.snapshotID, Subtree: plan.subtree, Target: plan.target}}
	default:
		sc.steps = []restic.PreviewStep{{SnapshotID: plan.snapshotID, Target: plan.target, Include: "/"}}
	}
	return sc
}

// fileSetFilesScope mirrors restoreOneFileSetFile.
func fileSetFilesScope(plan fileSetFilesRestorePlan) restoreScope {
	sc := restoreScope{ref: repoRef{plan.repo, plan.mode}, snapshotID: plan.snapshotID, inPlace: plan.target == ""}
	for _, sel := range plan.paths {
		if sc.inPlace {
			sc.steps = append(sc.steps, restic.PreviewStep{SnapshotID: plan.snapshotID, Target: "/", Include: escapeGlobLiteral(sel)})
			sc.written = append(sc.written, sel)
			continue
		}
		parent, base := path.Dir(sel), path.Base(sel)
		if parent == "." || parent == "/" || base == "." || base == "/" || base == "" {
			sc.steps = append(sc.steps, restic.PreviewStep{SnapshotID: plan.snapshotID, Subtree: sel, Target: plan.target})
			continue
		}
		sc.steps = append(sc.steps, restic.PreviewStep{SnapshotID: plan.snapshotID, Subtree: parent, Target: plan.target, Include: escapeGlobLiteral("/" + base)})
	}
	return sc
}

// zfsScope mirrors restoreZFSStep and restoreZFSFile.
func zfsScope(plan zfsRestorePlan) restoreScope {
	sc := restoreScope{ref: repoRef{plan.repo, plan.mode}, snapshotID: plan.snapshotID, inPlace: plan.inPlace}
	excludes := make([]string, len(plan.covered))
	for i, c := range plan.covered {
		excludes[i] = escapeGlobLiteral(c)
	}
	for _, step := range plan.steps {
		if plan.inPlace {
			sc.written = append(sc.written, step.target)
		}
		if len(plan.paths) == 0 {
			sc.steps = append(sc.steps, restic.PreviewStep{SnapshotID: step.snapshotID, Target: step.target, Excludes: excludes})
			continue
		}
		for _, sel := range plan.paths {
			parent, base := path.Dir(sel), path.Base(sel)
			if plan.inPlace || parent == "/" {
				sc.steps = append(sc.steps, restic.PreviewStep{SnapshotID: step.snapshotID, Target: step.target, Include: escapeGlobLiteral(sel)})
				continue
			}
			sc.steps = append(sc.steps, restic.PreviewStep{SnapshotID: step.snapshotID, Subtree: parent, Target: step.target, Include: escapeGlobLiteral("/" + base)})
		}
	}
	return sc
}

// foreignScope prepares a restore out of another instance's repository the
// way prepareForeignRestore does, without adopting its recipe.
func (s *Service) foreignScope(ctx context.Context, req RestoreCheckRequest) (restoreScope, error) {
	if !foreignItemNameOK(req.Domain, req.Name) {
		return restoreScope{}, errors.New("invalid item name")
	}
	sess, err := s.foreignSession(req.Session)
	if err != nil {
		return restoreScope{}, err
	}
	ref := repoRef{repo: sess.repo, mode: sess.mode}
	switch req.Domain {
	case "containers":
		tg, err := s.foreignContainerTarget(sess, req.Name)
		if err != nil {
			return restoreScope{}, err
		}
		destBase, err := s.foreignContainerDestBase(req.TargetPath)
		if err != nil {
			return restoreScope{}, err
		}
		plan, err := s.prepareRestoreForTarget(ctx, ref, req.Name, req.SnapshotID, tg, tagIdentity("container:"+req.Name), destBase, req.Overwrite)
		if err != nil {
			return restoreScope{}, err
		}
		return containerScope(plan, req.Name), nil
	case "vms":
		tg, err := s.foreignVMTarget(sess, req.Name)
		if err != nil {
			return restoreScope{}, err
		}
		destBase, err := s.foreignVMDestBase(req.TargetPath)
		if err != nil {
			return restoreScope{}, err
		}
		plan, err := s.prepareRestoreVMForTarget(ctx, ref, req.Name, req.SnapshotID, tg, tagIdentity("vm:"+req.Name), destBase, req.ZvolPool)
		if err != nil {
			return restoreScope{}, err
		}
		return vmScope(plan, req.Name), nil
	case "files":
		if len(req.Paths) > 0 {
			plan, err := s.prepareForeignFileSetFilesRestore(ctx, sess, req.Name, req.SnapshotID, req.TargetPath, req.Paths)
			if err != nil {
				return restoreScope{}, err
			}
			return fileSetFilesScope(plan), nil
		}
		plan, err := s.prepareForeignFileSetRestore(ctx, sess, req.Name, req.SnapshotID, req.TargetPath)
		if err != nil {
			return restoreScope{}, err
		}
		return fileSetScope(plan), nil
	case zfsDomain:
		plan, err := s.prepareForeignZFSRestore(ctx, sess, req.Name, req.SnapshotID, req.TargetPath, req.Paths, req.WholeTree)
		if err != nil {
			return restoreScope{}, err
		}
		return zfsScope(plan), nil
	default:
		return restoreScope{}, errors.New("unknown domain (must be containers, vms, files or zfs)")
	}
}

// selfScope covers the two restores of the server's own data: the flash
// backup, which only downloads, and the configuration, which is staged in the
// data folder and swapped in on the next start.
func (s *Service) selfScope(ctx context.Context, req RestoreCheckRequest) (restoreScope, error) {
	ref, err := s.restoreCheckRepo(req)
	if err != nil {
		return restoreScope{}, err
	}
	snaps, err := s.engine.Snapshots(ctx, ref.repo, ref.mode)
	if err != nil {
		return restoreScope{}, err
	}
	if req.Kind == checkFlash {
		id, err := resolveFlashSnapshot(snaps, req.SnapshotID)
		return restoreScope{ref: ref, snapshotID: id, download: true}, err
	}
	id, err := resolveConfigSnapshot(snaps, req.SnapshotID)
	if err != nil {
		return restoreScope{}, err
	}
	sc := restoreScope{ref: ref, snapshotID: id, fixedAt: s.cfg.DataDir, fixedNeed: -1}
	if _, bytes, err := s.engine.StatsRestoreSize(ctx, ref.repo, id, ref.mode); err == nil {
		sc.fixedNeed = bytes
	}
	return sc, nil
}

// dbImportScope reads an import plan. The previous data folder is set aside,
// not deleted, so the import needs room for the whole dump next to it.
func (s *Service) dbImportScope(ctx context.Context, req RestoreCheckRequest) (restoreScope, error) {
	plan, err := s.prepareImportDBDump(ctx, req.Name, req.Source, req.SnapshotID)
	if err != nil {
		return restoreScope{}, err
	}
	return restoreScope{
		ref: repoRef{plan.src.repo, plan.src.mode}, snapshotID: plan.dump.ID, owner: req.Name, inPlace: true,
		written: []string{plan.dataDir}, fixedNeed: plan.dump.Bytes, fixedAt: plan.dataDir,
	}, nil
}
