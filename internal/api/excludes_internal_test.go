package api

import (
	"context"
	"os"
	"path/filepath"
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
		"   ",                     // blank, dropped
	}
	got := s.previewExcludes(context.Background(), raw, in, effective)
	if len(got) != 2 {
		t.Fatalf("expected 2 previews (blank dropped), got %d: %+v", len(got), got)
	}

	if got[0].Status != "translated" || !got[0].Matches ||
		got[0].Resolved != "/host/user/user/appdata/plex/Transcode/Cache" {
		t.Fatalf("config line should be a matching translation: %+v", got[0])
	}
	if got[1].Status != "translated" || got[1].Matches {
		t.Fatalf("media line should not match (volume not backed up): %+v", got[1])
	}
}

// bareNameVolume is a backed-up volume holding plex/.git/config and
// plex/logs/server.log.
func bareNameVolume(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "plex")
	for _, dir := range []string{filepath.Join(root, ".git"), filepath.Join(root, "logs")} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{filepath.Join(root, ".git", "config"), filepath.Join(root, "logs", "server.log")} {
		if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestPreviewExcludesMatchesABareNameOnlyWhenTheVolumeHasIt(t *testing.T) {
	effective := []string{bareNameVolume(t)}
	cases := []struct {
		line string
		want bool
	}{
		{".git", true},
		{"*.log", true},
		{"server.log", true},
		{"nomatch.zzz", false},
		{"*.zzz", false},
	}
	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			got := excludeSvc().previewExcludes(context.Background(), []string{tc.line}, model.Inspect{}, effective)
			if len(got) != 1 || got[0].Status != "basename" || got[0].Matches != tc.want {
				t.Fatalf("preview of %q = %+v, want basename with matches=%v", tc.line, got, tc.want)
			}
		})
	}
}

func TestPreviewExcludesDoesNotWarnWhenTheSearchCannotFinish(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := excludeSvc().previewExcludes(ctx, []string{"nomatch.zzz"}, model.Inspect{}, []string{bareNameVolume(t)})
	if len(got) != 1 || !got[0].Matches {
		t.Fatalf("preview = %+v, want a match assumed when the search stopped early", got)
	}
}
