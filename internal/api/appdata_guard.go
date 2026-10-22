package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"slices"

	"github.com/junkerderprovinz/bombvault/internal/restic"
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
// follows for its source. A folder that cannot be walked, and a history that
// cannot be read, are left to restic, which reports both.
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
		return nil
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
