package api

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/paths"
)

// repoPathSvc is a Service configured like a stock Unraid install: the host's
// /mnt is mounted as "Host Data" at /host/user.
func repoPathSvc() *Service {
	return &Service{cfg: config.Config{HostMountRoot: "/host/user", HostSourceRoot: "/mnt"}}
}

// An absolute host path is refused with the relative value to enter instead,
// not with the raw paths sentinel.
func TestResolveRepoAbsolutePathGuidance(t *testing.T) {
	s := repoPathSvc()
	const abs = "/mnt/remotes/192.168.2.53_backup/bombvault"
	const want = "remotes/192.168.2.53_backup/bombvault"

	repo, err := s.resolveRepo(abs)
	if err == nil {
		t.Fatalf("an absolute host path must still be rejected, got repo %q", repo)
	}
	msg := err.Error()
	if !strings.Contains(msg, want) {
		t.Fatalf("guidance must offer the relative form %q, got: %s", want, msg)
	}
	if !strings.Contains(msg, "Host Data") {
		t.Fatalf("guidance must name the Host Data mount, got: %s", msg)
	}
	if strings.Contains(msg, "sub must be a relative path") {
		t.Fatalf("the raw paths sentinel must not reach the operator, got: %s", msg)
	}
	if !errors.Is(err, paths.ErrAbsoluteSub) {
		t.Fatal("guidance must still unwrap to paths.ErrAbsoluteSub")
	}
	if !errors.Is(err, errRepoPathGuidance) {
		t.Fatal("guidance must be tagged errRepoPathGuidance so scrubError keeps it intact")
	}
}

// The hint has to reach the client unscrubbed, including through the caller's
// wrap that the UI shows.
func TestResolveRepoGuidanceSurvivesScrubbing(t *testing.T) {
	s := repoPathSvc()
	_, err := s.resolveRepo("/mnt/remotes/nas/bombvault")
	if err == nil {
		t.Fatal("expected a rejection")
	}
	got := scrubError(fmt.Errorf("resolve off-site repo: %w", err))
	if !strings.Contains(got, "remotes/nas/bombvault") {
		t.Fatalf("scrubError swallowed the relative-path hint: %s", got)
	}
	if strings.Contains(got, "[path]") {
		t.Fatalf("scrubError must not placeholder-ise this message: %s", got)
	}
}

func TestHostRelativeSuggestion(t *testing.T) {
	s := repoPathSvc()
	cases := []struct{ in, want string }{
		{"/mnt/remotes/nas/bv", "remotes/nas/bv"},
		{"/mnt/user/backups/bv", "user/backups/bv"},
		{"/mnt", "mnt"}, // only "/mnt/" counts as the root prefix
		{"/srv/backups", "srv/backups"},
		{"//mnt/x", "mnt/x"},
	}
	for _, c := range cases {
		if got := s.hostRelativeSuggestion(c.in); got != c.want {
			t.Errorf("hostRelativeSuggestion(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestResolveRepoTraversalGuidance(t *testing.T) {
	s := repoPathSvc()
	_, err := s.resolveRepo("backups/../../etc")
	if err == nil {
		t.Fatal("a traversal path must be rejected")
	}
	if !errors.Is(err, paths.ErrTraversal) {
		t.Fatalf("expected a traversal cause, got %v", err)
	}
	if !errors.Is(err, errRepoPathGuidance) {
		t.Fatal("traversal guidance must also bypass the scrubber")
	}
	if !strings.Contains(err.Error(), "Host Data") {
		t.Fatalf("traversal guidance must name the Host Data mount, got: %v", err)
	}
}

// A relative subpath resolves under the mount and a restic remote URL passes
// through unchanged.
func TestResolveRepoAcceptedForms(t *testing.T) {
	s := repoPathSvc()

	got, err := s.resolveRepo("remotes/192.168.2.53_backup/bombvault")
	if err != nil {
		t.Fatalf("a relative subpath must still be accepted: %v", err)
	}
	if want := "/host/user/remotes/192.168.2.53_backup/bombvault"; got != want {
		t.Fatalf("resolveRepo = %q, want %q", got, want)
	}

	const remote = "rest:http://192.168.1.2:8000/bombvault-containers/containers"
	if got, err := s.resolveRepo(remote); err != nil || got != remote {
		t.Fatalf("a remote URL must pass through verbatim, got (%q, %v)", got, err)
	}
}
