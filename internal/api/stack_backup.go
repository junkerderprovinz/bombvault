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

// A Docker Compose project's working directory is backed up ONCE per project,
// as its own snapshot, instead of once per member container.
// ---------------------------------------------------------------------------
// Until now it rode along in every member's snapshot (see resolveAppdataPaths'
// comment where it used to be added). restic deduplicates the stored bytes, so
// a five-service stack cost one copy on disk and looked free. What it does not
// deduplicate is the work: every member walked, chunked and hashed the whole
// project directory on every run. The stored size stayed flat while the CPU
// cost scaled with the number of services, which is why this was invisible in
// the size column and very visible in a fan curve.
//
// The snapshot carries `stack:<project>` where a container's carries
// `container:<ref>`, so restic's own per-tag retention applies to it unchanged
// and a stack restore can find it without a second index.

// stackSnapshotTag is the snapshot tag identifying a stack's project directory.
// Its shape mirrors the container tag deliberately: one tag, one owner.
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

// backupStackDir snapshots one project directory into the containers repo under
// the stack tag. It deliberately does NOT stop anything: the project directory
// holds the compose file and the stack's shared files, not a running database,
// and every member has already been stopped and started around its own data by
// the time this runs.
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
	tags := []string{stackSnapshotTag(project), "p1"}
	if _, err := s.engine.Backup(ctx, repo, []string{dir}, tags, mode); err != nil {
		return fmt.Errorf("stack %s: %w", project, err)
	}
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

// latestStackSnapshot returns the newest snapshot carrying this project's stack
// tag, or ok=false when the project has never been backed up as a stack (every
// installation before this change, and any project whose folder was never
// reachable).
func (s *Service) latestStackSnapshot(ctx context.Context, repo string, mode restic.Mode, project string) (restic.Snapshot, bool) {
	snaps, err := s.engine.Snapshots(ctx, repo, mode)
	if err != nil {
		log.Printf("api: stack restore: list snapshots: %v", err)
		return restic.Snapshot{}, false
	}
	want := stackSnapshotTag(project)
	var best restic.Snapshot
	var found bool
	for _, sn := range snaps {
		if !slices.Contains(sn.Tags, want) {
			continue
		}
		// Times are RFC3339 from restic, so a string compare orders them; taking
		// the max avoids depending on the listing order.
		if !found || sn.Time > best.Time {
			best, found = sn, true
		}
	}
	return best, found
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
	settings, repo, err := s.domainRepoSource("containers", source)
	if err != nil {
		return false, err
	}
	mode := s.primaryModeFor(settings, "containers", repo)
	sn, found := s.latestStackSnapshot(ctx, repo, mode, project)
	if !found || len(sn.Paths) == 0 {
		return false, nil
	}
	if err := s.engine.RestorePath(ctx, repo, sn.ID, sn.Paths[0], mode); err != nil {
		return false, fmt.Errorf("restore stack %s: %w", project, err)
	}
	log.Printf("api: stack restore: %s from %s", project, sn.ID[:8])
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
