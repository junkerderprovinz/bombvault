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

// TestResolveRepoAbsolutePathGuidance is the issue-#138 regression: entering the
// ABSOLUTE host path of a mounted remote share (/mnt/remotes/...) — the obvious
// thing to type — used to fail with the raw internal sentinel "paths: sub must
// be a relative path", which reads like "local paths are unsupported". It must
// now name the relative-path convention and spell out the exact value to enter.
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
	// The cause is still inspectable, and the scrubber-bypass tag is set.
	if !errors.Is(err, paths.ErrAbsoluteSub) {
		t.Fatal("guidance must still unwrap to paths.ErrAbsoluteSub")
	}
	if !errors.Is(err, errRepoPathGuidance) {
		t.Fatal("guidance must be tagged errRepoPathGuidance so scrubError keeps it intact")
	}
}

// TestResolveRepoGuidanceSurvivesScrubbing: the hint is worthless once the path
// scrubber has eaten it, so it must reach the client verbatim even through the
// caller's wrap ("resolve off-site repo: …") that the UI actually shows.
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

// TestHostRelativeSuggestion: a path under the host storage root loses exactly
// that prefix; one outside it just loses its leading slash (still the right
// shape for the field, even if the folder has to be reachable under the mount).
func TestHostRelativeSuggestion(t *testing.T) {
	s := repoPathSvc()
	cases := []struct{ in, want string }{
		{"/mnt/remotes/nas/bv", "remotes/nas/bv"},
		{"/mnt/user/backups/bv", "user/backups/bv"},
		{"/mnt", "mnt"}, // no trailing slash → not a prefix match
		{"/srv/backups", "srv/backups"},
		{"//mnt/x", "mnt/x"},
	}
	for _, c := range cases {
		if got := s.hostRelativeSuggestion(c.in); got != c.want {
			t.Errorf("hostRelativeSuggestion(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestResolveRepoTraversalGuidance: an escaping path gets its own guidance and
// still unwraps to the traversal sentinel.
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

// TestResolveRepoAcceptedForms pins that the guidance change did not narrow what
// resolveRepo accepts: a relative subpath still resolves under the mount, and a
// restic remote URL still passes through verbatim.
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
