package api

import (
	"errors"
	"fmt"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/paths"
)

// A backup or off-site location is either a restic remote URL or a path
// relative to the "Host Data" mount (the host's /mnt by default, mounted at
// /host/user).
// The functions below turn paths.Resolve's sentinels for a local location into
// a message that tells the operator what to enter instead.

// errRepoPathGuidance tags a repo-location refusal that has to show the
// operator's own path to be useful, like errRestoreDestination. The path is the
// location they typed, not a secret, so the error bypasses the path scrubber.
var errRepoPathGuidance = errors.New("repository location refused")

// repoPathErr is errRepoPathGuidance for errors.Is and unwraps to the paths
// sentinel that caused it.
type repoPathErr struct {
	msg   string
	cause error
}

func (e *repoPathErr) Error() string { return e.msg }

func (e *repoPathErr) Unwrap() error { return e.cause }

func (e *repoPathErr) Is(target error) bool { return target == errRepoPathGuidance }

// remoteURLHint ends both guidance messages so a refused local path does not
// read as "only local paths are supported".
const remoteURLHint = "or a restic remote URL (rest:, sftp:, s3:, b2:, rclone:)"

// hostRelativeSuggestion strips cfg.HostSourceRoot from an absolute host path,
// so "/mnt/remotes/nas/bombvault" becomes "remotes/nas/bombvault". A path
// outside that root only loses its leading slash.
func (s *Service) hostRelativeSuggestion(loc string) string {
	root := strings.TrimSuffix(s.cfg.HostSourceRoot, "/")
	if root != "" && strings.HasPrefix(loc, root+"/") {
		loc = strings.TrimPrefix(loc, root+"/")
	}
	return strings.TrimLeft(loc, "/")
}

// hostRootName names the mount in messages: the Unraid template's label plus the
// host path behind it.
func (s *Service) hostRootName() string {
	if root := strings.TrimSuffix(s.cfg.HostSourceRoot, "/"); root != "" {
		return `the "Host Data" mount (` + root + ")"
	}
	return `the "Host Data" mount`
}

// repoPathError converts a paths.Resolve failure for a local repo location into
// the error resolveRepo returns. The two known sentinels get guidance; any other
// cause is wrapped plainly rather than dressed up as advice.
func (s *Service) repoPathError(loc string, err error) error {
	switch {
	case errors.Is(err, paths.ErrAbsoluteSub):
		return &repoPathErr{cause: err, msg: fmt.Sprintf(
			"%q is an absolute host path. Backup and off-site locations are relative to %s, so enter %q instead, %s.",
			loc, s.hostRootName(), s.hostRelativeSuggestion(loc), remoteURLHint)}
	case errors.Is(err, paths.ErrTraversal):
		return &repoPathErr{cause: err, msg: fmt.Sprintf(
			"%q points outside %s. Enter a path inside it, %s.",
			loc, s.hostRootName(), remoteURLHint)}
	}
	return fmt.Errorf("resolve repo path: %w", err)
}
