package api

import (
	"context"
	"maps"
	"net/http"
	"path"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// diffChangeLimit caps the paths one comparison lists. The totals count every
// change, listed or not.
const diffChangeLimit = 500

// BackupChange is one path that differs between two backups. Part names the
// dataset of a ZFS run or the zvol disk of a VM the path lies in.
type BackupChange struct {
	Part     string `json:"part,omitempty"`
	Path     string `json:"path"`
	Modifier string `json:"modifier"`
}

// BackupDiff is what changed from one backup of an entry to a later one.
type BackupDiff struct {
	restic.DiffResult
	Changes   []BackupChange `json:"changes"`
	Truncated bool           `json:"truncated,omitempty"`
	// OnlyBefore and OnlyAfter name the datasets or disks one of the two
	// backups lacks. Nothing in them is compared.
	OnlyBefore []string `json:"onlyBefore,omitempty"`
	OnlyAfter  []string `json:"onlyAfter,omitempty"`
	// Disks marks the comparison of a VM. Its paths are disk images, a
	// changed-block backup counts segments where others count files, and the
	// bytes say how much of the disks differs. What changed inside the guest
	// is not known.
	Disks bool `json:"disks,omitempty"`
}

// partsOf returns the snapshots of the backup snap is one of, by what each one
// holds: a dataset for a ZFS run, the device for a zvol disk of a VM run, and ""
// for the backup's own snapshot.
func (s *Service) partsOf(ctx context.Context, set backupSet, snap restic.Snapshot) (map[string]restic.Snapshot, error) {
	parts := map[string]restic.Snapshot{}
	if set.zfsRoot != "" {
		for _, sn := range set.together(snap) {
			if dataset, ok := zfsSnapshotDataset(sn, set.zfsRoot); ok {
				parts[dataset] = sn
			}
		}
		return parts, nil
	}
	parts[""] = snap
	if set.domain != "vms" {
		return parts, nil
	}
	all, err := s.listRepo(ctx, set.repo, set.mode)
	if err != nil {
		return nil, err
	}
	run := vmRunTag(all, snap.ID)
	if run == "" {
		return parts, nil
	}
	for _, sn := range all {
		if !slices.Contains(sn.Tags, run) {
			continue
		}
		for _, tag := range sn.Tags {
			if _, dev, ok := strings.Cut(tag, ":zvol:"); ok {
				parts[dev] = sn
			}
		}
	}
	return parts, nil
}

// isSegment reports whether p is a segment file of a changed-block VM backup:
// eight digits inside the folder of its disk.
func isSegment(p string) bool {
	name := path.Base(p)
	return len(name) == 8 && strings.Trim(name, "0123456789") == ""
}

// DiffBackups compares two backups of one entry. A backup made of several
// snapshots is compared dataset by dataset or disk by disk, and the totals are
// summed.
func (s *Service) DiffBackups(ctx context.Context, domain, key, source, from, to string) (BackupDiff, error) {
	out := BackupDiff{Changes: []BackupChange{}, Disks: domain == "vms"}
	if !backup.ValidSnapshotID(from) || !backup.ValidSnapshotID(to) {
		return out, backup.ErrInvalidSnapshotID
	}
	set, err := s.backupsOf(ctx, domain, key, source)
	if err != nil {
		return out, err
	}
	older, err := set.find(from)
	if err != nil {
		return out, err
	}
	newer, err := set.find(to)
	if err != nil {
		return out, err
	}
	before, err := s.partsOf(ctx, set, older)
	if err != nil {
		return out, err
	}
	after, err := s.partsOf(ctx, set, newer)
	if err != nil {
		return out, err
	}
	for _, part := range slices.Sorted(maps.Keys(after)) {
		prev, ok := before[part]
		if !ok {
			out.OnlyAfter = append(out.OnlyAfter, part)
			continue
		}
		cur := after[part]
		// A changed-block backup stores a disk as thousands of segment files,
		// so its changes are listed once per disk.
		segments := slices.Contains(cur.Tags, backup.BlocksTag)
		disks := map[string]bool{}
		total, err := s.engine.DiffStream(ctx, set.repo, prev.ID, cur.ID, set.mode, func(c restic.DiffChange) {
			change := BackupChange{Part: part, Path: c.Path, Modifier: c.Modifier}
			if segments && isSegment(c.Path) {
				change.Path, change.Modifier = path.Dir(c.Path)+"/", "M"
				if disks[change.Path] {
					return
				}
				disks[change.Path] = true
			}
			if len(out.Changes) >= diffChangeLimit {
				out.Truncated = true
				return
			}
			out.Changes = append(out.Changes, change)
		})
		if err != nil {
			return out, err
		}
		out.AddedFiles += total.AddedFiles
		out.RemovedFiles += total.RemovedFiles
		out.ChangedFiles += total.ChangedFiles
		out.AddedBytes += total.AddedBytes
		out.RemovedBytes += total.RemovedBytes
	}
	for _, part := range slices.Sorted(maps.Keys(before)) {
		if _, ok := after[part]; !ok {
			out.OnlyBefore = append(out.OnlyBefore, part)
		}
	}
	return out, nil
}

// handleDiffBackups serves GET /api/items/{domain}/{name}/diff?from=&to=&source=.
func (h *Handler) handleDiffBackups(w http.ResponseWriter, r *http.Request) {
	domain, key, ok := h.itemParam(w, r, itemDomains...)
	if !ok {
		return
	}
	q := r.URL.Query()
	d, err := h.svc.DiffBackups(r.Context(), domain, key, sourceParam(r), q.Get("from"), q.Get("to"))
	if err != nil {
		placementFail(w, err, nil)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"diff": d}))
}
