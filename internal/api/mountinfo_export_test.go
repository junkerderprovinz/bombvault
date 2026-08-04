package api

import (
	"context"
	"io"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// Test hooks (white-box) exposing the unexported mountinfo seam to the
// black-box api_test package.

// ParseMountedDirs exposes parseMountedDirs for direct unit testing.
func ParseMountedDirs(r io.Reader) map[string]bool { return parseMountedDirs(r) }

// SetMountinfoPath points the mount-table source at p (a fixture) and returns a
// function that restores the previous value.
func SetMountinfoPath(p string) (restore func()) {
	prev := mountinfoPath
	mountinfoPath = p
	return func() { mountinfoPath = prev }
}

// DestinationMounted exposes the discriminator (now HostMountRoot-aware) for tests.
func (s *Service) DestinationMounted(repo string) bool { return s.destinationMounted(repo) }

// SnapshotsForTag exposes the explicit-repo snapshot lister (and its
// not-mounted vs empty gate) for tests.
func (s *Service) SnapshotsForTag(ctx context.Context, repo string, mode restic.Mode, tag string) ([]restic.Snapshot, error) {
	return s.snapshotsForTag(ctx, repo, mode, tag)
}
