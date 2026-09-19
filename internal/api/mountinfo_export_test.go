package api

import (
	"context"
	"io"
	"io/fs"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

func ParseMountedDirs(r io.Reader) map[string]bool { return parseMountedDirs(r) }

// SetMountinfoPath points the mount table at a fixture and returns a function
// that restores the previous path.
func SetMountinfoPath(p string) (restore func()) {
	prev := mountinfoPath
	mountinfoPath = p
	return func() { mountinfoPath = prev }
}

func (s *Service) DestinationMounted(repo string) bool { return s.destinationMounted(repo) }

func (s *Service) SnapshotsForTag(ctx context.Context, repo string, mode restic.Mode, tag string) ([]restic.Snapshot, error) {
	return s.snapshotsForTag(ctx, repo, mode, tag)
}

// SetFileSetSourceWalk swaps the directory walk fileSetSourceEmpty uses, so a
// test can force a read error without an unreadable directory. The returned
// function restores the previous walk.
func SetFileSetSourceWalk(w func(root string, fn fs.WalkDirFunc) error) (restore func()) {
	prev := walkFileSetSource
	walkFileSetSource = w
	return func() { walkFileSetSource = prev }
}
