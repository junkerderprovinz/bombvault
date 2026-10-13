package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// itemDomains are the kinds of entry whose backups can be marked, compared and
// exported one by one.
var itemDomains = []string{"containers", "vms", "files", "flash", "config", zfsDomain}

// backupSet is what one entry holds at a source: the backups a caller can name
// and the repository they are in.
type backupSet struct {
	domain string
	// name is what the entry is called: its container, VM or folder set name,
	// its root dataset, or the domain for flash and config.
	name string
	// noun is what a refusal calls the entry, empty where the repository holds
	// nothing else.
	noun  string
	repo  string
	mode  restic.Mode
	snaps []restic.Snapshot
	// zfsRoot is the root dataset of a ZFS entry, whose backups are one
	// snapshot per dataset of its tree.
	zfsRoot string
}

// backupsOf lists what the entry key of domain holds at source. key is what the
// routes of the domain take: a container or VM name, the id of a folder set or
// ZFS entry, and the domain itself for flash and config.
func (s *Service) backupsOf(ctx context.Context, domain, key, source string) (backupSet, error) {
	if !validItemSource(source) {
		return backupSet{}, errInvalidItemSource
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return backupSet{}, fmt.Errorf("read settings: %w", err)
	}
	set := backupSet{domain: domain, name: key}
	switch domain {
	case "containers":
		if !validResourceName(key) {
			return set, errors.New("invalid container name")
		}
		set.noun = "container"
		if set.repo, err = s.containerRepoForName(settings, key, source); err == nil {
			set.snaps, err = s.Snapshots(ctx, key, source)
		}
	case "vms":
		if !validVMName(key) {
			return set, errors.New("invalid VM name")
		}
		set.noun = "VM"
		if set.repo, err = s.vmRepoForName(settings, key, source); err == nil {
			set.snaps, err = s.SnapshotsVM(ctx, key, source)
		}
	case "files":
		fs, gErr := s.store.GetFileSet(key)
		if gErr != nil {
			return set, errFileSetNotFound
		}
		set.noun, set.name = "folder set", fs.Name
		if set.repo, err = s.fileSetRepoFor(settings, fs, source); err == nil {
			set.snaps, err = s.SnapshotsFileSet(ctx, key, source)
		}
	case "flash":
		if set.repo, err = s.repoFor(settings, domain, source); err == nil {
			set.snaps, err = s.SnapshotsFlash(ctx, source)
		}
	case "config":
		if set.repo, err = s.repoFor(settings, domain, source); err == nil {
			set.snaps, err = s.SnapshotsConfig(ctx, source)
		}
	case zfsDomain:
		d, gErr := s.store.GetZFSDataset(key)
		if gErr != nil {
			return set, fmt.Errorf("load dataset: %w", gErr)
		}
		set.noun, set.name, set.zfsRoot = "dataset", d.Dataset, d.Dataset
		if set.repo, err = s.zfsDatasetRepoFor(settings, d, source); err == nil {
			set.snaps, err = s.zfsItemSnapshots(ctx, d, source)
		}
	default:
		return set, fmt.Errorf("unknown domain %q", domain)
	}
	if err != nil {
		return set, err
	}
	set.mode = s.repoModeFor(settings, domain, source, set.repo)
	return set, nil
}

// find returns the backup id names, by its full id or a prefix of it.
func (set backupSet) find(id string) (restic.Snapshot, error) {
	for _, sn := range set.snaps {
		if strings.HasPrefix(sn.ID, id) {
			return sn, nil
		}
	}
	return restic.Snapshot{}, notInListing{id, set.noun}
}

// together returns the snapshots that form one backup with snap. A ZFS run
// writes one snapshot per dataset, and all of them carry the run's stamp.
func (set backupSet) together(snap restic.Snapshot) []restic.Snapshot {
	stamp, ok := zfsStampOf(snap)
	if set.zfsRoot == "" || !ok {
		return []restic.Snapshot{snap}
	}
	var out []restic.Snapshot
	for _, sn := range set.snaps {
		if other, ok := zfsStampOf(sn); ok && other == stamp {
			out = append(out, sn)
		}
	}
	return out
}

// backups counts the backups of the set. A ZFS run is one, however many
// datasets it wrote.
func (set backupSet) backups() int {
	if set.zfsRoot == "" {
		return len(set.snaps)
	}
	runs := map[string]bool{}
	for _, sn := range set.snaps {
		if stamp, ok := zfsStampOf(sn); ok {
			runs[stamp] = true
		}
	}
	return len(runs)
}

// runTargetID is the id the runs of an entry carry. It is empty for a
// container or VM without a stored row, which has no runs.
func (s *Service) runTargetID(domain, key string) (string, error) {
	var id string
	var err error
	switch domain {
	case "containers":
		var t store.Target
		t, err = s.store.GetTargetByContainer(key)
		id = t.ID
	case "vms":
		var vm store.VMTarget
		vm, err = s.store.GetVMTargetByName(key)
		id = vm.ID
	case "files", zfsDomain:
		return key, nil
	default:
		return domainRunTargetID(domain), nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}

// zfsStampOf is the host snapshot a ZFS member snapshot was read from.
func zfsStampOf(snap restic.Snapshot) (string, bool) {
	if len(snap.Paths) == 0 {
		return "", false
	}
	return zfs.StampFromPath(snap.Paths[0])
}

// TagBackup adds tags to one backup of an entry and removes others, in one
// rewrite. A ZFS run is tagged on every dataset it wrote, so the mark shows on
// the run whichever dataset is looked at.
func (s *Service) TagBackup(ctx context.Context, domain, key, source, snapID string, add, remove []string) error {
	if !backup.ValidSnapshotID(snapID) {
		return backup.ErrInvalidSnapshotID
	}
	add, err := sanitizeTags(add)
	if err != nil {
		return err
	}
	remove, err = sanitizeTags(remove)
	if err != nil {
		return err
	}
	if len(add)+len(remove) == 0 {
		return nil
	}
	set, err := s.backupsOf(ctx, domain, key, source)
	if err != nil {
		return err
	}
	snap, err := set.find(snapID)
	if err != nil {
		return err
	}
	// restic tag takes an exclusive lock, so it waits its turn behind a backup
	// or prune of the domain instead of colliding with one.
	unlock, ok := s.tryLockDomainFor(domain, "tag")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	// A lock an interrupted copy or check left behind would refuse the tag.
	s.unlockStale(ctx, set.repo, set.mode)
	var ids []string
	for _, sn := range set.together(snap) {
		ids = append(ids, sn.ID)
	}
	return s.engine.Tag(ctx, set.repo, ids, add, remove, set.mode)
}

// handleTagBackup serves POST /api/items/{domain}/{name}/tag?source= with the
// body {snapshotId, tags, remove}.
func (h *Handler) handleTagBackup(w http.ResponseWriter, r *http.Request) {
	domain, key, ok := h.itemParam(w, r, itemDomains...)
	if !ok {
		return
	}
	var body struct {
		SnapshotID string   `json:"snapshotId"`
		Tags       []string `json:"tags"`
		Remove     []string `json:"remove"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if err := h.svc.TagBackup(r.Context(), domain, key, sourceParam(r), body.SnapshotID, body.Tags, body.Remove); err != nil {
		placementFail(w, err, nil)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}
