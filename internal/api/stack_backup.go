package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"sort"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// A Docker Compose project's working directory is backed up once per project,
// as its own snapshot tagged stack:<project>. Inside each member's snapshot
// restic would store it once, but every member would walk and hash the whole
// folder. The tag gives the folder a retention of its own, applied after each
// stack backup and by every per-identity pass, and lets a stack restore find it
// without a second index.

// stackSnapshotTag is the snapshot tag of a stack's project directory, shaped
// like the container tag: one tag, one owner.
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
		// Same rule every other path obeys: a directory BombVault cannot see
		// from inside its container is skipped rather than guessed at.
		return "", "", false
	}
	return project, cand, true
}

// stackDirsFor collects the distinct project directories across a set of
// containers, so a backup round can visit each project once regardless of how
// many of its services took part. Sorted by project name, because a round's log
// should read the same way twice.
func (s *Service) stackDirsFor(ctx context.Context, names []string) map[string]string {
	dirs := map[string]string{}
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
		dirs[project] = dir
	}
	return dirs
}

// backupStackDir snapshots one project directory into the containers domain path
// under the stack tag, then applies the local retention to that tag. Nothing is
// stopped: the folder holds the compose file and shared files, and every member
// has been stopped and started around its own data by the time this runs.
func (s *Service) backupStackDir(ctx context.Context, project, dir string) error {
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
	tag := stackSnapshotTag(project)
	if _, err := s.engine.Backup(ctx, repo, []string{dir}, []string{tag, "p1"}, mode); err != nil {
		return fmt.Errorf("stack %s: %w", project, err)
	}
	s.applyRetention(ctx, repo, settings, mode, tag, "containers")
	return nil
}

// BackupStacks backs up the project directory of every compose stack present in
// names, once each. Errors are collected rather than fatal: a stack whose folder
// cannot be read must not cost the round its other stacks, and the member
// backups have already succeeded by the time this runs.
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
		log.Printf("api: stack backup: %s (%s) done", p, dirs[p])
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

// RestoreStackDir restores a compose project's working directory in place, from
// its own stack snapshot.
//
// Returns ok=false with no error when there is no stack snapshot: that is the
// normal state for a project backed up before this change, where the folder
// still lives inside each member's snapshot and comes back with the member. It
// must not read as a failure, or every restore of an older backup would report
// one.
func (s *Service) RestoreStackDir(ctx context.Context, project, source string) (ok bool, err error) {
	d, err := s.latestStackDir(ctx, project, source)
	if err != nil {
		return false, err
	}
	if !d.found || len(d.snap.Paths) == 0 {
		return false, nil
	}
	if err := s.engine.RestorePath(ctx, d.repo, d.snap.ID, d.snap.Paths[0], d.mode); err != nil {
		return false, fmt.Errorf("restore stack %s: %w", project, err)
	}
	log.Printf("api: stack restore: %s from %s", project, d.snap.ID[:8])
	return true, nil
}

// projectOfMember reports the compose project a container belongs to, or "" if
// it is not part of one. Reads the live container first and falls back to the
// stored definition, so a stack restore onto a box where the containers no
// longer exist can still find the project name.
func (s *Service) projectOfMember(ctx context.Context, name string) string {
	if in, err := s.docker.Inspect(ctx, name); err == nil {
		if p := composeProject(in.Config.Labels); p != "" {
			return p
		}
	}
	tg, err := s.store.GetTargetByContainer(name)
	if err != nil || len(tg.Definition) == 0 {
		return ""
	}
	var def struct {
		Inspect model.Inspect `json:"inspect"`
	}
	if err := json.Unmarshal([]byte(tg.Definition), &def); err != nil {
		return ""
	}
	return composeProject(def.Inspect.Config.Labels)
}
