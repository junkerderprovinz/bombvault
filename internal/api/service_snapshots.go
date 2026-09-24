package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// LatestContainerBackupTimes returns, per container name, the unix time of the
// newest backup that name owns. A card's date is read from here rather than
// from the run history, so it agrees with the list of backups under it: an
// entry rebuilt by Discover has no run at all (#44), and a run stays with the
// entry while a backup stays with the name it was written under.
func (s *Service) LatestContainerBackupTimes(ctx context.Context) (map[string]int64, error) {
	// When the targets cannot be read nothing folds, and each old name keeps
	// its own date.
	idToName := map[string]string{}
	if targets, tErr := s.store.ListTargets(); tErr != nil {
		log.Printf("api: last-backup times: listing targets for alias fold: %v; leaving every tag as its own identity", tErr)
	} else {
		for _, t := range targets {
			idToName[t.ID] = t.ContainerName
		}
	}
	return s.latestBackupTimes(ctx, "containers", "container", idToName)
}

// LatestFileSetBackupTimes is LatestContainerBackupTimes for the folder sets.
// A set's name is fixed once it has backups, so no former name folds into it;
// backups left under a name no set carries any more come back as their own set
// through Discover.
func (s *Service) LatestFileSetBackupTimes(ctx context.Context) (map[string]int64, error) {
	return s.latestBackupTimes(ctx, "files", "fileset", nil)
}

// LatestVMBackupTimes is LatestContainerBackupTimes for the VMs domain.
func (s *Service) LatestVMBackupTimes(ctx context.Context) (map[string]int64, error) {
	idToName := map[string]string{}
	if targets, tErr := s.store.ListVMTargets(); tErr != nil {
		log.Printf("api: last-backup times: listing VM targets for alias fold: %v; leaving every tag as its own identity", tErr)
	} else {
		for _, t := range targets {
			idToName[t.ID] = t.Name
		}
	}
	return s.latestBackupTimes(ctx, "vms", "vm", idToName)
}

// latestBackupTimes reads one snapshot listing per repository of domain and
// keeps, per name, the newest time under that name's tag. idToName carries the
// current name of every entry, for the alias fold.
func (s *Service) latestBackupTimes(ctx context.Context, domain, aliasDomain string, idToName map[string]string) (map[string]int64, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	// Every repository this domain's items write to, not just the domain's own
	// (#204). A container pointed at a named repository keeps its snapshots
	// there, and reading only the domain repo would tell the dashboard it has
	// never been backed up.
	// The skip list is ignored here: this is a read-only overview, a repository
	// it cannot open has no times to contribute, and the loop below logs one it
	// cannot read. Every caller that changes something reports its skips; the
	// two that do not are this one and repoSharedWithAnotherDomain, which asks
	// a yes/no question and answers "shared" when it cannot tell.
	repos, _, err := s.domainReposInUse(settings, domain)
	if err != nil {
		return nil, err
	}
	prefix := aliasDomain + ":"
	out := make(map[string]int64)
	for _, repo := range repos {
		if localRepoMissing(repo.Loc) {
			continue
		}
		// Per repository, like every other reader: with the shared mode, a
		// container on a remote named repository with its own credentials could
		// not be listed, and the dashboard would say it had never been backed up.
		all, lErr := s.listSnapshots(ctx, repo.Loc, s.primaryModeFor(settings, domain, repo.Loc))
		if lErr != nil {
			// One unreachable repository must not blank the whole column: the
			// other entries' times are still true. The item whose repo this
			// is shows as never backed up, which is all that can be said while
			// its repository cannot be read.
			log.Printf("api: last-backup times: repository unreadable, skipping: %v", lErr)
			continue
		}
		for _, snap := range all {
			ts, perr := time.Parse(time.RFC3339Nano, snap.Time)
			if perr != nil {
				continue
			}
			unix := ts.Unix()
			for _, tag := range snap.Tags {
				name, ok := strings.CutPrefix(tag, prefix)
				if !ok || name == "" {
					continue
				}
				// A rename leaves the old tag on snapshots already written. One
				// from before the link is the renamed entry's and counts for its
				// current name, whoever holds the old name today; a later one
				// stays under the old name.
				if a, aErr := s.store.AliasByOldName(aliasDomain, name); aErr == nil {
					if cur := idToName[a.TargetID]; cur != "" && newAliasClaim(prefix, a).claims(snap) {
						name = cur
					}
				}
				if unix > out[name] {
					out[name] = unix
				}
			}
		}
	}
	return out, nil
}

// Snapshots lists the snapshots of a single container. The containers
// repository is shared, so the listing is filtered by the container:<name> tag
// the backup writes: otherwise the restore panel of one container would list,
// and could restore, another's snapshots.
func (s *Service) Snapshots(ctx context.Context, name, source string) ([]restic.Snapshot, error) {
	return s.containerSnapshotsOf(ctx, name, source, s.containerIdentity(name))
}

// containerSnapshotsOf is Snapshots for an identity the caller has already
// built, so a gate lists with the one it checked.
func (s *Service) containerSnapshotsOf(ctx context.Context, name, source string, id entryIdentity) ([]restic.Snapshot, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.containerRepoForName(settings, name, source)
	if err != nil {
		return nil, err
	}
	return s.snapshotsOwnedBy(ctx, repo, s.repoModeFor(settings, "containers", source, repo), id)
}

// containerIdentity is the container entry that answers to name, with every
// name it had before. A rename records an alias but never rewrites the
// snapshots under the old tag, so readers list the old tags too; which of
// those snapshots the entry owns is entryIdentity's rule. A name with no row
// has no aliases, and a name that is another entry's old name cedes that
// entry's pre-link snapshots (cededClaimOn).
func (s *Service) containerIdentity(name string) entryIdentity {
	ownID := ""
	tg, err := s.store.GetTargetByContainer(name)
	if err == nil {
		ownID = tg.ID
	}
	return withRowReadErr(s.aliasedIdentity("container", "container:", name, ownID), err)
}

// vmIdentity is containerIdentity for VMs. name is the libvirt name: on
// TrueNAS the display name is one virsh does not know, so it never appears in
// a tag.
func (s *Service) vmIdentity(name string) entryIdentity {
	ownID := ""
	tg, err := s.store.GetVMTargetByName(name)
	if err == nil {
		ownID = tg.ID
	}
	return withRowReadErr(s.aliasedIdentity("vm", "vm:", name, ownID), err)
}

// withRowReadErr marks id partial when reading its row failed for any reason
// other than there being no row, since the aliases were then never read.
func withRowReadErr(id entryIdentity, err error) entryIdentity {
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		id.readErr = errors.Join(id.readErr, err)
	}
	return id
}

// aliasedIdentity reads targetID's aliases with their link times ("" for a
// name with no row: no aliases). A failed read leaves the entry with its own
// tag alone: its pre-rename history is hidden until a read succeeds, nothing
// is claimed that is not provably its own, and readErr records the failure.
func (s *Service) aliasedIdentity(domain, prefix, name, targetID string) entryIdentity {
	id := tagIdentity(prefix + name)
	id.ceded, id.readErr = s.cededClaimOn(domain, prefix, name, targetID)
	if targetID == "" {
		return id
	}
	aliases, err := s.store.TargetAliases(domain, targetID)
	if err != nil {
		log.Printf("api: %s aliases for %q: %v", domain, name, err) //nolint:gosec // G706: domain is a fixed literal, name %q-quoted
		id.readErr = errors.Join(id.readErr, err)
		return id
	}
	for _, a := range aliases {
		id.aliases = append(id.aliases, newAliasClaim(prefix, a))
	}
	return id
}

// cededClaimOn returns the alias another entry holds on name, if any: a
// machine that took an old name up again must not own the renamed entry's
// snapshots from before the link. An alias on the entry's own current name
// cedes nothing. A failed read cannot rule another entry out, so it cedes the
// whole name until a read succeeds, and the error comes back with it.
func (s *Service) cededClaimOn(domain, prefix, name, targetID string) (*cededClaim, error) {
	a, err := s.store.AliasByOldName(domain, name)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		log.Printf("api: %s alias on %q: %v; none of its snapshots count as its own until this reads", domain, name, err) //nolint:gosec // G706: domain is a fixed literal, name %q-quoted
		return &cededClaim{aliasClaim: aliasClaim{tag: prefix + name}, unknown: true}, err
	case a.TargetID == targetID:
		return nil, nil
	}
	c := &cededClaim{aliasClaim: newAliasClaim(prefix, a)}
	if owner, oErr := s.entryNameByID(domain, a.TargetID); oErr == nil {
		c.owner = prefix + owner
	}
	return c, nil
}

// entryNameByID is the current name of the container or VM row id.
func (s *Service) entryNameByID(domain, id string) (string, error) {
	if domain == "vm" {
		tg, err := s.store.GetVMTargetByID(id)
		return tg.Name, err
	}
	tg, err := s.store.GetTargetByID(id)
	return tg.ContainerName, err
}

// entryIDByName is the id of the container or VM row on name.
func (s *Service) entryIDByName(domain, name string) (string, error) {
	if domain == "vm" {
		tg, err := s.store.GetVMTargetByName(name)
		return tg.ID, err
	}
	tg, err := s.store.GetTargetByContainer(name)
	return tg.ID, err
}

// snapshotsForTag lists an explicit repo (no settings resolution) and returns
// the snapshots carrying tag, oldest first. It reads one literal tag, not an
// entry's history: for the files and config domains, the zvol per-disk tags,
// and the checks that ask about one name's own snapshots. An entry's history
// is read through snapshotsOwnedBy.
func (s *Service) snapshotsForTag(ctx context.Context, repo string, mode restic.Mode, tag string) ([]restic.Snapshot, error) {
	return s.snapshotsForTags(ctx, repo, mode, []string{tag})
}

// snapshotsOwnedBy lists repo and keeps the snapshots id owns: those under its
// current tag that no other entry's alias claims, plus each alias's from
// before that alias was linked. Every reader of an entry's history goes
// through here, so neither a renamed entry nor a machine that took its old
// name up again can reach the other's snapshots.
func (s *Service) snapshotsOwnedBy(ctx context.Context, repo string, mode restic.Mode, id entryIdentity) ([]restic.Snapshot, error) {
	listed, err := s.snapshotsForTags(ctx, repo, mode, id.listTags())
	if err != nil {
		return nil, err
	}
	return id.owned(listed), nil
}

// snapshotsForTags is snapshotsForTag for several tags: a snapshot is included
// if it carries any of them. It applies no ownership rule; snapshotsOwnedBy
// narrows it to what an entry may claim, and retention reads it unfiltered. A
// missing local repo is "no snapshots yet", not an error, unless the repo was
// established before (share not mounted, #55). Remote repos skip that local
// check (see localRepoMissing).
func (s *Service) snapshotsForTags(ctx context.Context, repo string, mode restic.Mode, tags []string) ([]restic.Snapshot, error) {
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
	wanted := make(map[string]bool, len(tags))
	for _, t := range tags {
		wanted[t] = true
	}
	out := make([]restic.Snapshot, 0, len(all))
	for _, snap := range all {
		for _, t := range snap.Tags {
			if wanted[t] {
				out = append(out, snap)
				break
			}
		}
	}
	return out, nil
}

// DiffSnapshots compares two of a container's snapshots (restic diff) and
// returns the summary of what changed between them (files added/removed/changed,
// bytes added/removed).
//
// Security: both snapshot ids pass the strict hex guard
// (backup.ValidSnapshotID), and both must belong to the named container
// (tag-scoped via Snapshots, like RestoreContainerToPath and
// ListSnapshotFiles), so one container's snapshots can't be diffed through
// another's route. The repo and mode are resolved for the source.
func (s *Service) DiffSnapshots(ctx context.Context, name, source, snap1, snap2 string) (restic.DiffResult, error) {
	if !validResourceName(name) {
		return restic.DiffResult{}, errors.New("invalid container name")
	}
	if source != "local" && !isOffsiteSource(source) {
		return restic.DiffResult{}, errors.New("invalid source (must be local or offsite)")
	}
	if !backup.ValidSnapshotID(snap1) || !backup.ValidSnapshotID(snap2) {
		return restic.DiffResult{}, backup.ErrInvalidSnapshotID
	}

	// Scope to the named container: both snapshots must be among its snapshots.
	snaps, err := s.Snapshots(ctx, name, source)
	if err != nil {
		return restic.DiffResult{}, err
	}
	if !snapshotBelongs(snaps, snap1) {
		return restic.DiffResult{}, fmt.Errorf("snapshot %s does not belong to this container", snap1)
	}
	if !snapshotBelongs(snaps, snap2) {
		return restic.DiffResult{}, fmt.Errorf("snapshot %s does not belong to this container", snap2)
	}

	settings, err := s.store.GetSettings()
	if err != nil {
		return restic.DiffResult{}, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.containerRepoForName(settings, name, source)
	if err != nil {
		return restic.DiffResult{}, err
	}
	return s.engine.Diff(ctx, repo, snap1, snap2, s.repoModeFor(settings, "containers", source, repo))
}

// TagSnapshot adds tags to one of a container's snapshots (restic tag --add).
//
// Security: the snapshot id passes the strict hex guard and must belong to
// the named container (tag-scoped via Snapshots). Tags are sanitised:
// trimmed, empties dropped, and any tag with a comma or control character
// rejected (restic tags are comma-separated, so a comma would silently
// split into two tags). An empty resulting tag set is a no-op.
func (s *Service) TagSnapshot(ctx context.Context, name, source, snapID string, addTags []string) error {
	if !validResourceName(name) {
		return errors.New("invalid container name")
	}
	if source != "local" && !isOffsiteSource(source) {
		return errors.New("invalid source (must be local or offsite)")
	}
	if !backup.ValidSnapshotID(snapID) {
		return backup.ErrInvalidSnapshotID
	}
	tags, err := sanitizeTags(addTags)
	if err != nil {
		return err
	}
	if len(tags) == 0 {
		return nil // nothing to add
	}

	// Scope to the named container: the snapshot must be among its snapshots.
	snaps, err := s.Snapshots(ctx, name, source)
	if err != nil {
		return err
	}
	if !snapshotBelongs(snaps, snapID) {
		return fmt.Errorf("snapshot %s does not belong to this container", snapID)
	}

	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.containerRepoForName(settings, name, source)
	if err != nil {
		return err
	}
	mode := s.repoModeFor(settings, "containers", source, repo)
	// Serialize against a live backup/prune on this repo: restic tag takes an
	// exclusive lock, so run it under the domain lock like the other maintenance
	// ops, and report a clean busy instead of colliding on restic's repo lock.
	unlock, ok := s.tryLockDomainFor("containers", "tag")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	// unlockStale clears a genuine stale orphan (dead PID on this host) left by
	// a crashed run before restic takes its exclusive tag lock: the off-site
	// repo can carry a lock left by an interrupted off-site op (replication
	// copy or integrity check), and `restic tag` would otherwise fail with
	// "repository is already locked" (#29). Every other repo-mutating path
	// (backups, PruneDomain, DeleteSnapshot) does the same.
	s.unlockStale(ctx, repo, mode)
	return s.engine.TagAdd(ctx, repo, snapID, tags, mode)
}

// snapshotBelongs reports whether id (exact or unique prefix) is present in
// the already tag-scoped snapshot list, the access check shared by the
// diff, tag and restore-to-path routes.
func snapshotBelongs(snaps []restic.Snapshot, id string) bool {
	for _, sn := range snaps {
		if sn.ID == id || strings.HasPrefix(sn.ID, id) {
			return true
		}
	}
	return false
}

// chosenSnapshot returns the snapshot in snaps matching id (exact or
// unambiguous prefix, like snapshotBelongs/snapshotSubtree), or nil when
// there is no match. The restore path mapping reads the chosen snapshot's
// full recorded Paths from it, the source of truth for which selectors are
// valid in this snapshot (a recompute from the stored list would miss
// after the selection changed).
func chosenSnapshot(snaps []restic.Snapshot, id string) *restic.Snapshot {
	for i := range snaps {
		if snaps[i].ID == id || strings.HasPrefix(snaps[i].ID, id) {
			return &snaps[i]
		}
	}
	return nil
}

// sanitizeTags trims each tag, drops empties, and rejects any tag containing a
// comma or a control character. restic stores tags as a comma-separated list, so
// a comma would split one tag into two; control characters could corrupt argv or
// the snapshot metadata. Returns an error naming the offending tag.
func sanitizeTags(in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	for _, raw := range in {
		tag := strings.TrimSpace(raw)
		if tag == "" {
			continue
		}
		if strings.ContainsRune(tag, ',') {
			return nil, fmt.Errorf("invalid tag %q: tags cannot contain a comma", tag)
		}
		for _, r := range tag {
			if r < 0x20 || r == 0x7f {
				return nil, fmt.Errorf("invalid tag %q: tags cannot contain control characters", tag)
			}
		}
		out = append(out, tag)
	}
	return out, nil
}

// DeleteSnapshot forgets a single snapshot by id from a domain's repo
// (restic forget without prune, so it is fast). The space is reclaimed
// later by PruneDomain, so deleting several snapshots then pruning once is
// far cheaper than pruning per delete. The snapshot id is validated
// (argument-injection guard) and stale locks are cleared first.
func (s *Service) DeleteSnapshot(ctx context.Context, domain, snapshotID, source string) error {
	if !backup.ValidSnapshotID(snapshotID) {
		return backup.ErrInvalidSnapshotID
	}
	// The snapshot is deleted from the repository it is in, which with named
	// repositories (#204) need not be the domain's own. The id comes from a
	// list the interface built out of every one of them, so resolving the
	// domain repository alone would answer "no matching ID" for a snapshot
	// shown right beside the button, and the snapshot would stay.
	settings, repos, skipped, err := s.domainReposForOp(domain, source)
	if err != nil {
		return err
	}
	repos, missing, err := s.reposThatExist(repos, "no backups to delete yet")
	if err != nil {
		return err
	}
	skipped = append(skipped, missing...)
	unlock, ok := s.tryLockDomainFor(domain, "delete")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	repo, mode, err := s.repoHoldingSnapshot(ctx, settings, domain, source, repos, snapshotID)
	if err != nil {
		// A snapshot not found while part of the domain was unreachable is "I
		// could not look everywhere", not "it is not there", and the difference
		// decides whether somebody goes looking for it by hand.
		if sErr := skippedError("this delete", skipped); sErr != nil {
			return fmt.Errorf("%w; %w", err, sErr)
		}
		return err
	}
	// Deleting snapshots from an immutable repo is refused (same gate as
	// pruneDomain): append-only means credentials on this box cannot erase that
	// history. Asked of the repository the snapshot is actually in, so a named
	// append-only archive protects itself even though the domain's own repo is
	// maintainable. An off-site source asks the target it names.
	if isOffsiteSource(source) {
		immutable, err := s.offsiteSourceImmutable(settings, domain, source)
		if err != nil {
			return err
		}
		if immutable {
			return errAppendOnlyOffsiteTarget
		}
	}
	// The same refusal applies when the "local" source is a remote primary
	// flagged append-only in its saved safety settings (#152, same gate as
	// pruneDomain): there is no separate off-site copy in that shape, so
	// refusing here is the only thing standing between an on-box credential
	// and deleting backup history.
	if f := s.primaryAppendOnly(domain, repo); !isOffsiteSource(source) && f != appendOnlyNone {
		return appendOnlyRefusal(f)
	}
	s.unlockStale(ctx, repo, mode)
	return s.engine.Forget(ctx, repo, []string{snapshotID}, false, mode)
}

// repoHoldingSnapshot finds which of a domain's repositories a snapshot id
// lives in, and returns it with the mode that repository needs.
//
// A short id is matched by prefix, because that is how restic prints ids
// and therefore what the interface passes back. A repository that cannot
// be listed is skipped rather than failing the search: one unreachable
// location must not stop a deletion from a reachable one.
//
// Every repository is searched even after a hit, and an id that matches in
// two of them is refused rather than guessed. A short id is eight hex
// characters; two repositories of one domain hold snapshots written by the
// same BombVault, so a collision is likelier here than restic's own odds
// within one repository, and a wrong guess deletes the wrong backup.
//
// One repository short-circuits without listing: there is nothing to be
// ambiguous with, restic's own "no matching ID" is the better message for
// an id that is not there, and the caller's append-only refusal has to be
// reachable without first reading a repository that may be remote,
// unreachable or write-protected.
func (s *Service) repoHoldingSnapshot(ctx context.Context, settings store.Settings, domain, source string, repos []domainRepoRef, snapshotID string) (string, restic.Mode, error) {
	if len(repos) == 1 {
		return repos[0].Loc, s.repoModeFor(settings, domain, source, repos[0].Loc), nil
	}
	var (
		found     []domainRepoRef
		foundMode restic.Mode
		unread    []repoSkip
	)
	for _, r := range repos {
		mode := s.repoModeFor(settings, domain, source, r.Loc)
		snaps, err := s.listSnapshots(ctx, r.Loc, mode)
		if err != nil {
			// Remembered, not swallowed. A repository that could not be read is not a
			// repository that does not hold the snapshot, and answering "no repository
			// of this domain holds it" for a share that is merely unmounted sends
			// somebody looking for a backup that is right there.
			unread = append(unread, repoSkip{Name: s.refName(r), Reason: scrubError(err), Unreachable: true})
			continue
		}
		for _, sn := range snaps {
			if strings.HasPrefix(sn.ID, snapshotID) {
				found = append(found, r)
				foundMode = mode
				break
			}
		}
	}
	switch len(found) {
	case 0:
		if sErr := skippedError("the search for this backup", unread); sErr != nil {
			return "", restic.Mode{}, fmt.Errorf("no repository of this domain that could be read holds the backup %s; %w", snapshotID, sErr)
		}
		return "", restic.Mode{}, fmt.Errorf("no repository of this domain holds the backup %s", snapshotID)
	case 1:
		return found[0].Loc, foundMode, nil
	default:
		names := make([]string, 0, len(found))
		for _, r := range found {
			names = append(names, s.refName(r))
		}
		return "", restic.Mode{}, fmt.Errorf("the backup id %s matches a snapshot in more than one repository of this domain (%s); use the full id",
			snapshotID, strings.Join(names, ", "))
	}
}
