package api

import (
	"errors"
	"fmt"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/paths"
)

// ---------------------------------------------------------------------------
// Repo-location guidance — turning paths.Resolve's internal sentinels into an
// answer the operator can act on.
//
// A backup/off-site LOCATION is either a restic remote URL (rest:, sftp:, s3:,
// b2:, rclone:, …) or a path RELATIVE to the container's "Host Data" mount
// (the host storage root, /mnt by default, mounted at /host/user). Typing the
// absolute host path a share is mounted at — /mnt/remotes/<nas>/<share>, the
// obvious thing to do — used to fail with the raw internal sentinel
// "paths: sub must be a relative path", which never mentions the relative-path
// convention and reads like "local paths are not supported" (issue #138).
// ---------------------------------------------------------------------------

// errRepoPathGuidance tags a repo-location refusal whose message is only
// actionable WITH the operator's own path in it (same pattern, and same
// reasoning, as errRestoreDestination): it echoes the location they typed into
// Settings plus the relative form they should have typed instead. That is their
// own storage layout, never a credential or a secret — so it bypasses the path
// scrubber, which would otherwise reduce the whole hint to "[path]".
var errRepoPathGuidance = errors.New("repository location refused")

// repoPathErr carries the ready-to-show guidance while still unwrapping to the
// paths sentinel that caused it, so errors.Is holds for BOTH
// errRepoPathGuidance (the scrubber bypass) and paths.ErrAbsoluteSub /
// paths.ErrTraversal (the cause).
type repoPathErr struct {
	msg   string
	cause error
}

func (e *repoPathErr) Error() string { return e.msg }

func (e *repoPathErr) Unwrap() error { return e.cause }

func (e *repoPathErr) Is(target error) bool { return target == errRepoPathGuidance }

// remoteURLHint lists the accepted restic remote prefixes, appended to both
// guidance messages so "a local path is wrong here" never reads as "only local
// paths exist".
const remoteURLHint = "or a restic remote URL (rest:, sftp:, s3:, b2:, rclone:)"

// hostRelativeSuggestion turns the absolute host path an operator typed into the
// relative form BombVault expects: the same path minus the host storage root
// that is mounted as "Host Data" (cfg.HostSourceRoot, /mnt by default).
// "/mnt/remotes/nas/bombvault" → "remotes/nas/bombvault". A path outside that
// root just loses its leading slash, which is still the right shape.
func (s *Service) hostRelativeSuggestion(loc string) string {
	root := strings.TrimSuffix(s.cfg.HostSourceRoot, "/")
	if root != "" && strings.HasPrefix(loc, root+"/") {
		loc = strings.TrimPrefix(loc, root+"/")
	}
	return strings.TrimLeft(loc, "/")
}

// hostRootName is how the "Host Data" mount is referred to in operator-facing
// text: the Unraid template's mount label plus the host path it points at.
func (s *Service) hostRootName() string {
	if root := strings.TrimSuffix(s.cfg.HostSourceRoot, "/"); root != "" {
		return `the "Host Data" mount (` + root + ")"
	}
	return `the "Host Data" mount`
}

// repoPathError converts a paths.Resolve failure for a LOCAL repo location into
// the error resolveRepo returns. The two known sentinels become a complete,
// operator-facing sentence naming the relative-path convention and the exact
// value to enter instead; anything else keeps the previous plain wrap so an
// unexpected cause is never dressed up as advice.
func (s *Service) repoPathError(loc string, err error) error {
	switch {
	case errors.Is(err, paths.ErrAbsoluteSub):
		return &repoPathErr{cause: err, msg: fmt.Sprintf(
			"%q is an absolute host path. Backup and off-site locations are relative to %s, so enter %q instead — %s.",
			loc, s.hostRootName(), s.hostRelativeSuggestion(loc), remoteURLHint)}
	case errors.Is(err, paths.ErrTraversal):
		return &repoPathErr{cause: err, msg: fmt.Sprintf(
			"%q points outside %s. Enter a path inside it — %s.",
			loc, s.hostRootName(), remoteURLHint)}
	}
	return fmt.Errorf("resolve repo path: %w", err)
}
