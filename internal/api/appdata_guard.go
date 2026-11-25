package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// A disk that does not mount at boot leaves its mountpoint an ordinary folder,
// and Docker creates a container's bind source there, empty, when the
// container starts. Backed up as usual, that folder looks like a successful
// backup, and keep-last retention then ages the real history out. The two
// guards below catch it: one by the mount a folder sat on at its last backup,
// one by the folder having no files while earlier backups held data.

// pathMounts returns, for each path on storage of its own below HostMountRoot,
// the mount point it sits on, and false when the mount table could not be
// read.
func (s *Service) pathMounts(paths []string) (map[string]string, bool) {
	mounts, ok := mountTable()
	if !ok {
		return nil, false
	}
	out := map[string]string{}
	for _, p := range paths {
		if point, on := s.storageMountOf(mounts, p); on {
			out[p] = point
		}
	}
	return out, true
}

// offTheirMount returns the paths of this backup that the last one recorded
// on a mount of their own and that are not on that mount now: on none, or on
// one further up such as the folder a disk is mounted into. One on a mount
// further down, a dataset made for the folder for example, is still on its
// storage. An unreadable mount table or definition refuses nothing.
func (s *Service) offTheirMount(name string, effective []string, mountsNow map[string]string, known bool) []string {
	if !known {
		return nil
	}
	tg, err := s.store.GetTargetByContainer(name)
	if err != nil || tg.Definition == "" {
		return nil
	}
	var def containerDefinition
	if err := json.Unmarshal([]byte(tg.Definition), &def); err != nil {
		return nil
	}
	var left []string
	for _, p := range effective {
		now, on := mountsNow[p]
		switch was, recorded := def.PathMounts[p]; {
		case recorded && now != was && !isStrictSubpath(was, now):
			left = append(left, p)
		case !recorded && slices.Contains(def.MountedPaths, p) && !on:
			left = append(left, p)
		}
	}
	return left
}

// refuseEmptiedAppdata refuses a backup whose folders hold no files at all
// while an earlier backup of the container held data, the rule a folder set
// follows for its source. A folder that cannot be walked is left to restic,
// which reports it. A history that cannot be read counts as one that held data.
func (s *Service) refuseEmptiedAppdata(ctx context.Context, repo string, mode restic.Mode, name string, effective []string) error {
	if len(effective) == 0 {
		return nil
	}
	for _, p := range effective {
		if empty, err := fileSetSourceEmpty(p); err != nil || !empty {
			return nil
		}
	}
	snaps, err := s.snapshotsOwnedBy(ctx, repo, mode, s.containerIdentity(name))
	if err != nil {
		log.Printf("api: backup: %q: could not check earlier backups: %v", name, err) //nolint:gosec // G706: name is %q-quoted
		return fmt.Errorf("backup %q: its backup folders hold no files, and earlier backups could not be read to tell whether they held data. "+
			"Refusing an empty backup that retention would count as a real one", name)
	}
	// Only a snapshot with a recorded size counts, so an app whose folder was
	// always empty keeps backing up. Restic before 0.17 recorded none.
	if !slices.ContainsFunc(snaps, func(sn restic.Snapshot) bool {
		return sn.Summary != nil && sn.Summary.TotalBytesProcessed > 0
	}) {
		return nil
	}
	return fmt.Errorf("backup %q: its backup folders hold no files, while earlier backups held data (did a disk not mount?). "+
		"Refusing an empty backup that retention would count as a real one", name)
}

// mountedAfterStart returns the host paths of the folders that are empty here
// while the container sees files in them. Without mount propagation on Host
// Data, a disk the host mounts after BombVault started never shows up in this
// container, and a first backup of such a folder would hold nothing.
func (s *Service) mountedAfterStart(ctx context.Context, in model.Inspect, effective []string) []string {
	peeker, ok := s.docker.(dockercli.FilePeeker)
	if !ok || in.ID == "" {
		return nil
	}
	var hidden []string
	for _, p := range effective {
		if empty, err := fileSetSourceEmpty(p); err != nil || !empty {
			continue
		}
		host := s.toHostPath(p)
		for _, m := range in.Mounts {
			rel, err := filepath.Rel(m.Source, host)
			if m.Source == "" || err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				continue
			}
			holds, err := peeker.HoldsFiles(ctx, in.ID, path.Join(m.Destination, filepath.ToSlash(rel)))
			if err != nil {
				log.Printf("api: backup: could not look at %s through Docker: %v", host, err) //nolint:gosec // G706: host is a path from the container's own inspect
			}
			if holds {
				hidden = append(hidden, host)
			}
			break
		}
	}
	return hidden
}

// LateMountNote says, once at startup, that Host Data is bound without mount
// propagation, and is empty when it is not or when Host Data is no mount here.
// TrueNAS's app catalog binds Host Data that way, so this is a log line and no error.
func (s *Service) LateMountNote() string {
	recs := zfsMountRecords()
	if !slices.ContainsFunc(recs, func(r zfs.MountRecord) bool { return r.MountPoint == s.cfg.HostMountRoot }) {
		return ""
	}
	if top, _ := zfs.Propagation(recs, s.cfg.HostMountRoot); top {
		return ""
	}
	return fmt.Sprintf("Host Data %s is bound without mount propagation. A disk the host mounts below it after BombVault started "+
		"shows up here after a restart, and backups of its folders are refused until then", s.cfg.HostMountRoot)
}
