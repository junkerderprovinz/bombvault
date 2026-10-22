package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path"
	"path/filepath"
	"slices"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// A disk that does not mount at boot leaves its mountpoint an ordinary folder,
// and Docker creates a container's bind source there, empty, when the
// container starts. Backed up as usual, that folder looks like a successful
// backup, and keep-last retention then ages the real history out. The two
// guards below catch it: one by the mount a folder sat on at its last backup,
// one by the folder having no files while earlier backups held data.

// pathsOnTheirOwnMount returns the paths that sit on a mount strictly below
// HostMountRoot, and false when the mount table could not be read.
func (s *Service) pathsOnTheirOwnMount(paths []string) ([]string, bool) {
	mounts, ok := mountPoints()
	if !ok {
		return nil, false
	}
	root := path.Clean(filepath.ToSlash(s.cfg.HostMountRoot))
	var out []string
	for _, p := range paths {
		for q := path.Clean(filepath.ToSlash(p)); ; q = path.Dir(q) {
			if mounts[q] && isStrictSubpath(root, q) {
				out = append(out, p)
				break
			}
			if path.Dir(q) == q {
				break
			}
		}
	}
	return out, true
}

// offTheirMount returns the paths of this backup that the last one recorded
// on a mount of their own and that are on none now. An unreadable mount table
// or definition refuses nothing.
func (s *Service) offTheirMount(name string, effective, mountedNow []string, known bool) []string {
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
		if slices.Contains(def.MountedPaths, p) && !slices.Contains(mountedNow, p) {
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
