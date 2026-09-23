package api

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
)

// excludeSvc builds a Service with only the host mount config, which is all
// the exclude resolver reads. /mnt maps to /host/user, so a translated
// /config/... path gets the doubled "user" segment restic stores.
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

	// A path under a mount destination becomes the anchored path restic stored.
	pattern, status := s.resolveExcludeLine("/config/Library/Application Support/Plex Media Server/Cache", in)
	wantPattern := "/host/user/user/appdata/plex/Library/Application Support/Plex Media Server/Cache"
	if pattern != wantPattern || status != "translated" {
		t.Fatalf("translated: got (%q,%q), want (%q,translated)", pattern, status, wantPattern)
	}

	// A bare name stays a basename that matches at any depth.
	if p, st := s.resolveExcludeLine(".git", in); p != ".git" || st != "basename" {
		t.Fatalf("basename: got (%q,%q), want (.git,basename)", p, st)
	}

	// A path under no mount passes through verbatim.
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
		"/config/Transcode/Cache", // translated, volume backed up
		"/media/movies/tmp",       // translated, volume not backed up
		".git",                    // basename
		"   ",                     // blank, dropped
	}
	got := s.previewExcludes(raw, in, effective)
	if len(got) != 3 {
		t.Fatalf("expected 3 previews (blank dropped), got %d: %+v", len(got), got)
	}

	if got[0].Status != "translated" || !got[0].Matches ||
		got[0].Resolved != "/host/user/user/appdata/plex/Transcode/Cache" {
		t.Fatalf("config line should be a matching translation: %+v", got[0])
	}
	if got[1].Status != "translated" || got[1].Matches {
		t.Fatalf("media line should not match (volume not backed up): %+v", got[1])
	}
	if got[2].Status != "basename" || !got[2].Matches || got[2].Raw != ".git" {
		t.Fatalf("basename line should match at any depth: %+v", got[2])
	}
}
