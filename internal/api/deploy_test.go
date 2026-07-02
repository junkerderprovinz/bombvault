package api

import (
	"regexp"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// realIPv4Re matches a real dotted-quad IPv4 address. The 192.168.x.x placeholder
// does NOT match (the x octets are not digits), so any match is a leaked real IP.
var realIPv4Re = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)

// TestBuildDeploySnippet pins the rest-server deployment snippet: it enables
// append-only + private-repos, its htpasswd line verifies against the one-time
// password with bcrypt, and it leaks no real IP (only the 192.168.x.x placeholder).
func TestBuildDeploySnippet(t *testing.T) {
	snip, err := buildDeploySnippet("containers")
	if err != nil {
		t.Fatalf("buildDeploySnippet: %v", err)
	}

	// The append-only + private-repos flags are the whole point — the far side
	// enforces immutability. Both snippet flavours must carry them.
	for _, want := range []string{"--append-only", "--private-repos"} {
		if !strings.Contains(snip.DockerRun, want) {
			t.Errorf("docker run snippet missing %q", want)
		}
		if !strings.Contains(snip.Compose, want) {
			t.Errorf("compose snippet missing %q", want)
		}
	}

	// The user + htpasswd line are per-domain, and the hash verifies against the
	// returned plaintext password (proving it is a real bcrypt of THAT password).
	if snip.User != "bombvault-containers" {
		t.Errorf("user = %q, want bombvault-containers", snip.User)
	}
	if !strings.HasPrefix(snip.Htpasswd, "bombvault-containers:") {
		t.Fatalf("htpasswd line should start with bombvault-containers:, got %q", snip.Htpasswd)
	}
	hash := strings.TrimPrefix(snip.Htpasswd, "bombvault-containers:")
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(snip.Password)); err != nil {
		t.Fatalf("htpasswd hash does not verify against the returned password: %v", err)
	}
	if cost, cErr := bcrypt.Cost([]byte(hash)); cErr != nil || cost != 12 {
		t.Errorf("bcrypt cost = %d (err %v), want 12", cost, cErr)
	}
	if snip.Password == "" || len(snip.Password) != 24 {
		t.Errorf("password length = %d, want 24 url-safe chars", len(snip.Password))
	}
	// The plaintext password must never be embedded in the htpasswd line.
	if strings.Contains(snip.Htpasswd, snip.Password) {
		t.Errorf("htpasswd line must not embed the plaintext password")
	}

	// No real IPs — only the 192.168.x.x placeholder may appear in either snippet.
	for name, s := range map[string]string{"dockerRun": snip.DockerRun, "compose": snip.Compose} {
		if leak := realIPv4Re.FindString(s); leak != "" {
			t.Errorf("%s snippet contains a real IP %q (only 192.168.x.x placeholder allowed)", name, leak)
		}
		if !strings.Contains(s, "192.168.x.x") {
			t.Errorf("%s snippet should contain the 192.168.x.x placeholder hint", name)
		}
	}

	// Two invocations must yield different passwords (freshly random each time).
	snip2, err := buildDeploySnippet("containers")
	if err != nil {
		t.Fatalf("buildDeploySnippet (2): %v", err)
	}
	if snip2.Password == snip.Password {
		t.Errorf("passwords must be randomly generated per call")
	}
}

// TestBuildDeploySnippetUnknownDomain: a domain outside the fixed set is refused.
func TestBuildDeploySnippetUnknownDomain(t *testing.T) {
	if _, err := buildDeploySnippet("../etc"); err == nil {
		t.Fatal("expected an error for an unknown domain")
	}
}
