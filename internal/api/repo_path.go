package api

import (
	"errors"
	"fmt"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/store"
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

// ErrNoLocation refuses a backup or an off-site copy of a section whose own
// repository has no location, before anything reaches restic.
var ErrNoLocation = errors.New("section has no storage location")

// codeNoLocation is the code a refusal by ErrNoLocation carries, which the page
// turns into a sentence in its language.
const codeNoLocation = "no-location"

// sectionNames are the labels the settings page gives the sections.
var sectionNames = map[string]string{
	"containers": "Containers",
	"vms":        "VMs",
	"flash":      "Flash",
	"config":     "Self-backup",
	"files":      "Folders",
	zfsDomain:    "ZFS datasets",
}

type noLocationErr struct{ domain string }

func (e *noLocationErr) Error() string {
	name := sectionNames[e.domain]
	return fmt.Sprintf("%s has no storage location yet. Open Settings, Storage locations and switch on a line for %s.", name, name)
}

func (e *noLocationErr) Is(target error) bool { return target == ErrNoLocation }

// sectionRepo resolves a section's own repository location, refusing an empty
// one: path.Clean would turn it into the mount root and call that a traversal.
func (s *Service) sectionRepo(domain, loc string) (string, error) {
	if strings.TrimSpace(loc) == "" {
		return "", &noLocationErr{domain: domain}
	}
	return s.resolveRepo(loc)
}

// requireOwnLocation refuses a section without placement whose own folder is
// in use but has no location.
func (s *Service) requireOwnLocation(settings store.Settings, domain string) error {
	if strings.TrimSpace(domainPathRaw(domain, settings)) != "" {
		return nil
	}
	if off, err := s.localOff(domain); err == nil && off {
		return nil
	}
	return &noLocationErr{domain: domain}
}

// ownRepoFor resolves a domain's own repository, "" for a switched-off own
// folder without a location: that one has none, and its items' named
// repositories are still in use.
func (s *Service) ownRepoFor(settings store.Settings, domain string) (string, error) {
	own, err := s.repoFor(settings, domain, "local")
	if errors.Is(err, ErrNoLocation) && s.requireOwnLocation(settings, domain) == nil {
		return "", nil
	}
	return own, err
}
