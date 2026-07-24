package api

import (
	"strings"
	"testing"
)

// TestRegistryHost pins the docker reference heuristic (#106): the part before
// the first "/" is a registry host only when it contains a "." or ":" or is
// "localhost"; everything else is a Docker Hub path.
func TestRegistryHost(t *testing.T) {
	cases := []struct {
		ref  string
		want string
	}{
		{"ghcr.io/x/y", "ghcr.io"},
		{"ghcr.io/x/y:latest", "ghcr.io"},
		{"docker.io/x/y", "docker.io"},
		{"x/y", "docker.io"},
		{"x/y:tag", "docker.io"},
		{"nginx", "docker.io"},
		{"nginx:latest", "docker.io"},
		{"library/nginx", "docker.io"},
		{"registry.example.com:5000/x", "registry.example.com:5000"},
		{"lscr.io/x/y", "lscr.io"},
		{"localhost/x", "localhost"},
		{"localhost:5000/x/y:tag", "localhost:5000"},
		// Docker Hub endpoint aliases collapse to the canonical host.
		{"index.docker.io/library/nginx", "docker.io"},
		{"GHCR.IO/x/y", "ghcr.io"},
	}
	for _, c := range cases {
		if got := registryHost(c.ref); got != c.want {
			t.Errorf("registryHost(%q) = %q, want %q", c.ref, got, c.want)
		}
	}
}

// TestMergeRegistryAuths pins the settings-PUT contract (#106): a blank token
// keeps the stored one for that host, a non-blank token replaces it, an entry
// absent from the submitted list is deleted, and invalid entries are rejected
// with user-facing errors.
func TestMergeRegistryAuths(t *testing.T) {
	stored := []RegistryAuth{
		{Host: "ghcr.io", Username: "old-user", Token: "ghcr-token"},
		{Host: "lscr.io", Username: "u", Token: "lscr-token"},
	}

	// Blank token keeps stored; non-blank replaces; lscr.io absent → deleted;
	// docker.io added new. Hosts normalize (case/scheme).
	got, err := mergeRegistryAuths([]registryAuthView{
		{Host: "https://GHCR.IO/", Username: "new-user", Token: ""},
		{Host: "docker.io", Username: "hubuser", Token: "hub-token"},
	}, stored)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 entries (lscr.io deleted), got %+v", got)
	}
	if got[0].Host != "ghcr.io" || got[0].Username != "new-user" || got[0].Token != "ghcr-token" {
		t.Fatalf("blank token must keep the stored one + apply the username edit: %+v", got[0])
	}
	if got[1].Host != "docker.io" || got[1].Token != "hub-token" {
		t.Fatalf("new entry must store its token: %+v", got[1])
	}

	// A NEW host with a blank token has nothing stored to keep → error.
	if _, err := mergeRegistryAuths([]registryAuthView{{Host: "quay.io"}}, stored); err == nil ||
		!strings.Contains(err.Error(), "token is required") {
		t.Fatalf("a new host without a token must be rejected, got %v", err)
	}

	// Blank host → error.
	if _, err := mergeRegistryAuths([]registryAuthView{{Host: " ", Token: "x"}}, nil); err == nil {
		t.Fatal("a blank host must be rejected")
	}

	// Duplicate hosts (after normalization) → error.
	if _, err := mergeRegistryAuths([]registryAuthView{
		{Host: "ghcr.io", Token: "a"},
		{Host: "https://ghcr.io", Token: "b"},
	}, nil); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate hosts must be rejected, got %v", err)
	}

	// A host carrying a path is a mistyped ref, not a host → error.
	if _, err := mergeRegistryAuths([]registryAuthView{{Host: "ghcr.io/owner/img", Token: "x"}}, nil); err == nil {
		t.Fatal("a host with a path must be rejected")
	}

	// Empty submitted list = delete everything.
	if got, err := mergeRegistryAuths(nil, stored); err != nil || len(got) != 0 {
		t.Fatalf("an empty list must clear all entries: %v %v", got, err)
	}
}
