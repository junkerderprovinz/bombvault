package zfs

import (
	"path"
	"strings"
)

// Binding is one place a container backup reads from a snapshot: the part of
// Dataset at Rel ("" for its top) is shown at Target, the path the backup
// always reads.
type Binding struct {
	Dataset string
	Rel     string
	Target  string
}

// Unplaced says why a path cannot be read from a snapshot. Code is a reason
// code the page turns into a sentence, Path the container path it is about.
type Unplaced struct {
	Code string
	Path string
}

func (u *Unplaced) Error() string { return u.Code + ": " + u.Path }

// The reasons a container falls back to the ordinary stop because of where its
// data lies.
const (
	CodeNotOnZFS    = "not-on-zfs"
	CodeSplitShare  = "split-share"
	CodeNoBranch    = "share-not-found"
	CodeNestedMount = "nested-mount"
)

// Lookup is how PlaceOnZFS sees the filesystem: whether a path exists, and
// where it really lies once symlinks are followed.
type Lookup struct {
	Exists func(string) bool
	Real   func(string) string
}

// PlaceOnZFS finds the datasets that hold paths, as this container's mount
// table shows them. A path is followed through its symlinks first, so an
// Unraid exclusive share, which is a link from the user share into its pool,
// lands on the pool. A path left on a user share is followed to the one disk
// or pool below mountRoot that holds it. Every path has to lie on ZFS, and so
// does every mount below it; the first one that does not is returned instead,
// since then the whole container is stopped the ordinary way.
func PlaceOnZFS(recs []MountRecord, paths []string, mountRoot string, fs Lookup) ([]Binding, *Unplaced) {
	var out []Binding
	for _, p := range paths {
		p = path.Clean(p)
		source := path.Clean(fs.Real(p))
		top, ok := coveringMount(recs, source)
		if !ok {
			return nil, &Unplaced{Code: CodeNotOnZFS, Path: p}
		}
		if top.FSType == "fuse.shfs" {
			rel := strings.TrimPrefix(strings.TrimPrefix(source, top.MountPoint), "/")
			branch, u := shareBranch(recs, rel, mountRoot, fs.Exists)
			if u != nil {
				u.Path = p
				return nil, u
			}
			source = branch
			if top, ok = coveringMount(recs, source); !ok {
				return nil, &Unplaced{Code: CodeNotOnZFS, Path: p}
			}
		}
		if top.FSType != "zfs" || strings.Contains(top.Source, "@") {
			return nil, &Unplaced{Code: CodeNotOnZFS, Path: p}
		}
		out = append(out, Binding{Dataset: top.Source, Rel: inDataset(top, source), Target: p})

		for _, r := range recs {
			if !under(r.MountPoint, source) || strings.Contains(r.Source, "@") {
				continue
			}
			if r.FSType != "zfs" {
				return nil, &Unplaced{Code: CodeNestedMount, Path: p + strings.TrimPrefix(r.MountPoint, source)}
			}
			out = append(out, Binding{Dataset: r.Source, Rel: inDataset(r, r.MountPoint), Target: p + strings.TrimPrefix(r.MountPoint, source)})
		}
	}
	return out, nil
}

// inDataset is where p lies inside the dataset mounted by r, "" for its top.
// A mount of a subdirectory, such as an Unraid exclusive share bound from its
// pool, starts below the top by its root.
func inDataset(r MountRecord, p string) string {
	rel := strings.TrimPrefix(strings.TrimPrefix(p, r.MountPoint), "/")
	return strings.Trim(path.Join(r.Root, rel), "/")
}

// coveringMount returns the mount p is read through: the deepest one at or
// above it, and of several on one mountpoint the last, which hides the
// others.
func coveringMount(recs []MountRecord, p string) (MountRecord, bool) {
	var best MountRecord
	found := false
	for _, r := range recs {
		if r.MountPoint != p && !under(p, r.MountPoint) {
			continue
		}
		if !found || len(r.MountPoint) >= len(best.MountPoint) {
			best, found = r, true
		}
	}
	return best, found
}

// shareBranch finds the one disk or pool directly below mountRoot that holds
// rel of a user share. shfs merges them, and a share spread over several of
// them has no single dataset to snapshot.
func shareBranch(recs []MountRecord, rel, mountRoot string, exists func(string) bool) (string, *Unplaced) {
	var hits []string
	seen := map[string]bool{}
	for _, r := range recs {
		if path.Dir(r.MountPoint) != mountRoot || r.FSType == "fuse.shfs" || seen[r.MountPoint] {
			continue
		}
		seen[r.MountPoint] = true
		if cand := r.MountPoint + "/" + rel; exists(cand) {
			hits = append(hits, cand)
		}
	}
	switch len(hits) {
	case 0:
		return "", &Unplaced{Code: CodeNoBranch}
	case 1:
		return hits[0], nil
	}
	return "", &Unplaced{Code: CodeSplitShare}
}
