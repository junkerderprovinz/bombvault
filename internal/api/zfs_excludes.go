package api

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

const (
	// zfsPreviewWalkCap bounds one preview. A media share holds more entries
	// than anyone needs counted to see whether a line works, and the page waits
	// for the answer.
	zfsPreviewWalkCap = 50000
	// zfsPreviewSamples is how many matching paths a row names.
	zfsPreviewSamples = 3
)

// ZFSExcludePreviewRow is what one exclude line would leave out of the next
// backup, with a few of the paths it hits. Everything marks a line that would
// exclude the whole tree and Root one that would leave a whole dataset out.
// The backup ignores both.
type ZFSExcludePreviewRow struct {
	Pattern    string   `json:"pattern"`
	Matches    int      `json:"matches"`
	Sample     []string `json:"sample"`
	Everything bool     `json:"everything,omitempty"`
	Root       bool     `json:"root,omitempty"`
}

// PreviewZFSExcludes resolves each exclude line against the item's live
// datasets. It globs the same way the backup does: an anchored pattern belongs
// to the member whose logical path is its longest prefix, an unanchored one
// matches a basename at any depth in every member.
func (s *Service) PreviewZFSExcludes(ctx context.Context, id string, patterns []string) ([]ZFSExcludePreviewRow, error) {
	d, err := s.store.GetZFSDataset(id)
	if err != nil {
		return nil, fmt.Errorf("zfs excludes preview: load dataset: %w", err)
	}
	members, err := s.store.ListZFSMembers(d.ID)
	if err != nil {
		return nil, fmt.Errorf("zfs excludes preview: load the tree: %w", err)
	}
	dirs := make(map[string]string, len(members))
	// mounts are the members' directories, where the walk of the dataset
	// above stops: each member is counted by its own walk.
	mounts := make(map[string]bool, len(members))
	relPaths := make([]string, 0, len(members))
	for _, m := range members {
		rel := zfsRelPath(d.Dataset, m.Dataset)
		relPaths = append(relPaths, rel)
		if cpath, ok := s.toContainerPath(m.HostMountpoint); ok {
			dirs[rel] = cpath
			mounts[filepath.Clean(filepath.FromSlash(cpath))] = true
		}
	}
	// The backup reads each member from a snapshot directory and ignores a
	// line that leaves that directory out whole, also one that names a part
	// of its path such as .zfs or the snapshot's name.
	snap := zfs.SnapshotName(time.Now())

	out := make([]ZFSExcludePreviewRow, 0, len(patterns))
	for _, pattern := range patterns {
		if strings.TrimSpace(pattern) == "" {
			continue
		}
		row := ZFSExcludePreviewRow{Pattern: pattern, Sample: []string{}, Everything: backup.ExcludesEverything(pattern)}
		for rel, rerooted := range backup.SplitExcludes([]string{pattern}, relPaths) {
			for _, p := range rerooted {
				if dirs[rel] != "" && backup.LeavesMemberOut(dirs[rel]+"/.zfs/snapshot/"+snap, p) {
					row.Root = true
					continue
				}
				countZFSMatches(ctx, dirs[rel], rel, p, mounts, &row)
			}
		}
		out = append(out, row)
	}
	return out, nil
}

// countZFSMatches walks one member's live directory and adds every entry the
// pattern hits to the row. A directory that cannot be read contributes
// nothing: the preview is guidance, and a permission error on one folder must
// not turn into a refusal for the whole line. The walk leaves out the
// directories in mounts below dir, which are members of their own.
func countZFSMatches(ctx context.Context, dir, rel, pattern string, mounts map[string]bool, row *ZFSExcludePreviewRow) {
	if dir == "" {
		return
	}
	anchored := strings.HasPrefix(pattern, "/")
	top := filepath.Clean(filepath.FromSlash(dir))
	seen := 0
	_ = filepath.WalkDir(dir, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			if entry != nil && entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if entry.IsDir() && filepath.Clean(p) != top && mounts[filepath.Clean(p)] {
			return fs.SkipDir
		}
		seen++
		if seen > zfsPreviewWalkCap {
			return fs.SkipAll
		}
		inner := strings.TrimPrefix(filepath.ToSlash(p), filepath.ToSlash(dir))
		if inner == "" {
			return nil
		}
		hit := false
		if anchored {
			hit, _ = path.Match(pattern, inner)
		} else {
			hit, _ = path.Match(pattern, path.Base(inner))
		}
		if !hit {
			return nil
		}
		row.Matches++
		if len(row.Sample) < zfsPreviewSamples {
			row.Sample = append(row.Sample, path.Join(strings.TrimSuffix(rel, "/"), inner))
		}
		if entry.IsDir() {
			return fs.SkipDir
		}
		return nil
	})
}
