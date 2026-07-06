package api

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
)

// excludeSvc builds a Service with only the Host Data mount config set — enough
// to exercise the exclude resolver/preview in isolation (they read cfg only via
// toContainerPath, no store/docker). Mirrors svcWithMount in path_internal_test.go
// with the box-gate mapping /mnt → /host/user, so a translated /config/... path
// picks up the DOUBLED "user" segment restic actually stored.
func excludeSvc() *Service {
	return &Service{cfg: config.Config{
		HostSourceRoot: "/mnt",
		HostMountRoot:  "/host/user",
	}}
}

func TestResolveExcludeLine(t *testing.T) {
	s := excludeSvc()
	in := model.Inspect{Mounts: []model.Mount{
		{Type: "bind", Source: "/mnt/user/appdata/plex", Destination: "/config"},
	}}

	// A container path under a mount Destination translates through the mount's
	// Source + toContainerPath into the exact anchored /host/user/user/... path
	// restic stored — note the DOUBLED "user" segment.
	pattern, status := s.resolveExcludeLine("/config/Library/Application Support/Plex Media Server/Cache", in)
	wantPattern := "/host/user/user/appdata/plex/Library/Application Support/Plex Media Server/Cache"
	if pattern != wantPattern || status != "translated" {
		t.Fatalf("translated: got (%q,%q), want (%q,translated)", pattern, status, wantPattern)
	}

	// A bare name (no slash) → verbatim basename, matches at any depth.
	if p, st := s.resolveExcludeLine(".git", in); p != ".git" || st != "basename" {
		t.Fatalf("basename: got (%q,%q), want (.git,basename)", p, st)
	}

	// A slash path under no mount → passthrough (verbatim, never dropped).
	if p, st := s.resolveExcludeLine("/data/x", in); p != "/data/x" || st != "passthrough" {
		t.Fatalf("passthrough: got (%q,%q), want (/data/x,passthrough)", p, st)
	}
}

func TestPreviewExcludes(t *testing.T) {
	s := excludeSvc()
	in := model.Inspect{Mounts: []model.Mount{
		{Type: "bind", Source: "/mnt/user/appdata/plex", Destination: "/config"},
		{Type: "bind", Source: "/mnt/user/media", Destination: "/media"},
	}}
	// Only the /config volume is actually backed up.
	effective := []string{"/host/user/user/appdata/plex"}

	raw := []string{
		"/config/Transcode/Cache", // translated, volume IS in effective → matches
		"/media/movies/tmp",       // translated, volume NOT in effective → no match
		".git",                    // basename → matches at any depth
		"   ",                     // blank → dropped entirely
	}
	got := s.previewExcludes(raw, in, effective)
	if len(got) != 3 {
		t.Fatalf("expected 3 previews (blank dropped), got %d: %+v", len(got), got)
	}

	// (a) Translated line whose volume IS selected for backup → Matches true.
	if got[0].Status != "translated" || !got[0].Matches ||
		got[0].Resolved != "/host/user/user/appdata/plex/Transcode/Cache" {
		t.Fatalf("config line should be a matching translation: %+v", got[0])
	}
	// (b) Translated line whose volume is NOT backed up → Matches false.
	if got[1].Status != "translated" || got[1].Matches {
		t.Fatalf("media line should not match (volume not backed up): %+v", got[1])
	}
	// (c) Basename → Matches true, raw text preserved verbatim.
	if got[2].Status != "basename" || !got[2].Matches || got[2].Raw != ".git" {
		t.Fatalf("basename line should match at any depth: %+v", got[2])
	}
}
