package api

import (
	"context"
	"fmt"
	"log"
	"slices"
	"sort"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// stackSnapshotTag is the snapshot tag of a stack's project directory. It has
// the same shape as container:<ref>, so restic's per-tag retention applies
// unchanged and a stack restore finds the snapshot without a second index.
func stackSnapshotTag(project string) string { return "stack:" + project }

// stackDirFor returns the compose project directory of a container, translated
// into this process's view and checked for containment, plus the project name.
// ok is false when the container is not part of a compose project, carries no
// working-dir label, or the directory lies outside the host mount.
func (s *Service) stackDirFor(in model.Inspect) (project, dir string, ok bool) {
	project = composeProject(in.Config.Labels)
	if project == "" {
		return "", "", false
	}
	host, has := composeProjectDataDir(in.Config.Labels)
	if !has {
		return "", "", false
	}
	cand, inMount := s.toContainerPath(host)
	if !inMount {
		return "", "", false
	}
	return project, cand, true
}

// stackDirsFor maps each compose project among the named containers to its
// directory, so a backup round visits each project once.
func (s *Service) stackDirsFor(ctx context.Context, names []string) map[string]stackFolder {
	dirs := map[string]stackFolder{}
	for _, name := range names {
		in, err := s.docker.Inspect(ctx, name)
		if err != nil {
			// A container that vanished mid-round is not a reason to skip the
			// rest of the stacks; the member backup reports its own failure.
			log.Printf("api: stack backup: inspect %s: %v", name, err)
			continue
		}
		project, dir, ok := s.stackDirFor(in)
		if !ok {
			continue
		}
		if _, known := dirs[project]; !known {
			dirs[project] = stackFolder{dir: dir, member: name}
		}
	}
	return dirs
}

// stackFolder is a project's directory and the member of the round whose run
// speaks for it, since the folder has no run of its own.
type stackFolder struct {
	dir, member string
}

// backupStackDir snapshots one project directory into the containers repo under
// the stack tag. It stops nothing: the directory holds the compose file and the
// stack's shared files, and each member has already been stopped and started
// around its own data.
func (s *Service) backupStackDir(ctx context.Context, project string, folder stackFolder) error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("stack %s: read settings: %w", project, err)
	}
	repo, err := s.containersRepoPath(settings)
	if err != nil {
		return fmt.Errorf("stack %s: %w", project, err)
	}
	mode := s.primaryModeFor(settings, "containers", repo)
	if err := s.EnsureRepo(ctx, repo, mode); err != nil {
		return fmt.Errorf("stack %s: %w", project, err)
	}
	if err := s.refuseLinkOutOfMount([]string{folder.dir}); err != nil {
		return fmt.Errorf("stack %s: %w", project, err)
	}
	tag := stackSnapshotTag(project)
	if s.stackFolderEmptied(ctx, repo, mode, tag, folder.dir) {
		log.Printf("api: stack backup: %s: the project folder holds no files, while earlier backups held some; no backup made", project)
		s.noteOnStackMember(folder.member, store.NoteStackFolderEmpty+": "+s.toHostPath(folder.dir))
		s.notifyBackup(notify.WithHealthchecksSuppressed(ctx), "stack", project, "", true, backup.Summary{Emptied: true}, nil)
		return nil
	}
	sum, err := s.engine.Backup(ctx, repo, []string{folder.dir}, []string{tag, "p1"}, mode)
	if err != nil {
		return fmt.Errorf("stack %s: %w", project, err)
	}
	if sum.Unreadable != "" {
		s.noteOnStackMember(folder.member, store.NoteFilesUnreadable+": "+s.hostPathsIn(sum.Unreadable))
		s.notifyBackup(notify.WithHealthchecksSuppressed(ctx), "stack", project, "", true, backupSummaryFrom(sum), nil)
	} else {
		s.applyRetention(ctx, repo, settings, mode, tagIdentity(tag), "containers", anomalyScope{})
	}
	// A round backs the stacks up after its last container, and without a
	// keep-policy no batched prune follows to relax what this wrote.
	makeLocalRepoReadable(repo, s.cfg.DataDir)
	return nil
}

// stackFolderEmptied reports whether a project folder has no file left while an
// earlier backup of it held some, as when the disk under it did not mount. No
// snapshot is written then, so no pass has a reason to age the good ones.
func (s *Service) stackFolderEmptied(ctx context.Context, repo string, mode restic.Mode, tag, dir string) bool {
	if empty, err := fileSetSourceEmpty(dir); err != nil || !empty {
		return false
	}
	snaps, err := s.snapshotsForTag(ctx, repo, mode, tag)
	if err != nil {
		log.Printf("api: stack backup: could not read the earlier backups of %s: %v", tag, err)
		return true
	}
	return slices.ContainsFunc(snaps, heldData)
}

// noteOnStackMember puts a warning on the newest backup run of a stack's member.
func (s *Service) noteOnStackMember(member, note string) {
	tg, err := s.store.GetTargetByContainer(member)
	if err != nil {
		log.Printf("api: stack backup: no target for %q to carry the note: %v", member, err) //nolint:gosec // G706: name is %q-quoted
		return
	}
	run, err := s.store.LastSuccessfulBackup(tg.ID)
	if err != nil || run == nil {
		log.Printf("api: stack backup: no run of %q to carry the note: %v", member, err) //nolint:gosec // G706: name is %q-quoted
		return
	}
	if err := s.store.AddRunNote(run.ID, note); err != nil {
		log.Printf("api: stack backup: %v", err)
	}
}

// BackupStacks backs up the project directory of every compose stack in names,
// once per stack. Inside each member's snapshot, every service would walk and
// hash the whole directory again on every run. A stack that fails does not stop
// the others.
func (s *Service) BackupStacks(ctx context.Context, names []string) error {
	dirs := s.stackDirsFor(ctx, names)
	if len(dirs) == 0 {
		return nil
	}
	projects := make([]string, 0, len(dirs))
	for p := range dirs {
		projects = append(projects, p)
	}
	sort.Strings(projects)

	var failed []string
	for _, p := range projects {
		if err := s.backupStackDir(ctx, p, dirs[p]); err != nil {
			log.Printf("api: stack backup: %v", err)
			failed = append(failed, p)
			continue
		}
		log.Printf("api: stack backup: %s (%s) done", p, dirs[p].dir)
	}
	if len(failed) > 0 {
		return fmt.Errorf("stack backup failed for: %s", strings.Join(failed, ", "))
	}
	return nil
}

// BackupStacksAfterBulk backs up the stacks of a scheduled container round. Their
// retention forgets without --prune, because the round prunes once afterwards.
func (s *Service) BackupStacksAfterBulk(ctx context.Context, names []string) {
	if err := s.BackupStacks(WithBulkReplicateSuppressed(ctx), names); err != nil {
		log.Printf("api: %v", err)
	}
}

// stackDir is the newest snapshot of a project's folder at one source, and what
// it takes to restore it from there.
type stackDir struct {
	repo  string
	mode  restic.Mode
	snap  restic.Snapshot
	found bool
}

// latestStackDir looks for the project's folder at the containers repository
// the source names. found is false for a project never backed up as a stack.
func (s *Service) latestStackDir(ctx context.Context, project, source string) (stackDir, error) {
	settings, repo, err := s.domainRepoSource("containers", source)
	if err != nil {
		return stackDir{}, err
	}
	d := stackDir{repo: repo, mode: s.repoModeFor(settings, "containers", source, repo)}
	snaps, err := s.listRepo(ctx, repo, d.mode)
	if err != nil {
		return stackDir{}, err
	}
	want := stackSnapshotTag(project)
	for _, sn := range snaps {
		// Times are RFC3339 from restic, so a string compare orders them; taking
		// the max avoids depending on the listing order.
		if slices.Contains(sn.Tags, want) && (!d.found || sn.Time > d.snap.Time) {
			d.snap, d.found = sn, true
		}
	}
	return d, nil
}

// guardStackDir is what the restore of a project's folder refuses on. The
// recorded path comes from the repository, which is less to be trusted than what
// is stored here, as for the mapped paths of a container restore.
func (s *Service) guardStackDir(project string, d stackDir) error {
	if !paths.Within(s.cfg.HostMountRoot, d.snap.Paths[0]) {
		return fmt.Errorf("restore stack %s: the backup names a path outside the host mount, so refusing to restore", project)
	}
	if err := s.refuseLinkOutOfMount(d.snap.Paths); err != nil {
		return fmt.Errorf("restore stack %s: %w", project, err)
	}
	if err := s.guardInPlaceRestore(d.snap.Paths[:1]); err != nil {
		return fmt.Errorf("restore stack %s: %w", project, err)
	}
	return nil
}

// RestoreStackDir restores a compose project's working directory in place from
// its stack snapshot. It returns false and no error when there is none: older
// backups keep the folder inside each member's snapshot, so it comes back with
// the members. Whatever runs out of the folder has to be stopped first.
func (s *Service) RestoreStackDir(ctx context.Context, project, source string) (ok bool, err error) {
	d, err := s.latestStackDir(ctx, project, source)
	if err != nil {
		return false, err
	}
	if !d.found || len(d.snap.Paths) == 0 {
		return false, nil
	}
	if err := s.guardStackDir(project, d); err != nil {
		return false, err
	}
	unlock := s.lockDomainFor("containers", "restore")
	defer unlock()
	if err := s.engine.RestorePath(ctx, d.repo, d.snap.ID, d.snap.Paths[0], d.mode); err != nil {
		return false, fmt.Errorf("restore stack %s: %w", project, err)
	}
	log.Printf("api: stack restore: %s from %s", project, d.snap.ID[:8])
	return true, nil
}
