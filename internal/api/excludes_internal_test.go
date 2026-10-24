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

// mountedExcludeSvc maps /mnt to a temp folder holding the plex appdata
// (Transcode/Cache, Logs/a.log, Logs/b.log) and a folder whose name holds glob
// characters.
func mountedExcludeSvc(t *testing.T) (s *Service, in model.Inspect, effective []string) {
	t.Helper()
	root := filepath.ToSlash(t.TempDir())
	for _, dir := range []string{"user/appdata/plex/Transcode/Cache", "user/appdata/plex/Logs", "user/appdata/Plex [Media]/Cache"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"user/appdata/plex/Logs/a.log", "user/appdata/plex/Logs/b.log"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	s = &Service{cfg: config.Config{HostSourceRoot: "/mnt", HostMountRoot: root}}
	in = model.Inspect{Mounts: []model.Mount{
		{Type: "bind", Source: "/mnt/user/appdata/plex", Destination: "/config"},
		{Type: "bind", Source: "/mnt/user/appdata/Plex [Media]", Destination: "/media"},
		{Type: "bind", Source: "/mnt/user/other", Destination: "/other"},
	}}
	effective = []string{root + "/user/appdata/plex", root + "/user/appdata/Plex [Media]"}
	return s, in, effective
}

func TestPreviewExcludesChecksAContainerPathOnDisk(t *testing.T) {
	s, in, effective := mountedExcludeSvc(t)
	cases := []struct {
		line          string
		matches, gone bool
	}{
		{"/config/Transcode/Cache", true, false},
		{"/config/Transcode", true, false},
		{"/config/Transcode/Nope", false, true},
		{"/config/Logs/*.log", true, false},
		{"/config/Logs/*.txt", false, true},
		{"/config/**/Cache", true, false},
		{"/config/**/Nope", false, true},
		{"/media/Cache", true, false},
		{"/media/Nope", false, true},
		{"/media/C*", true, false},
		{"/other/anything", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			got := s.previewExcludes(context.Background(), []string{tc.line}, in, effective)
			if len(got) != 1 || got[0].Status != "translated" || got[0].Matches != tc.matches || got[0].Missing != tc.gone {
				t.Fatalf("preview of %q = %+v, want matches=%v missing=%v", tc.line, got, tc.matches, tc.gone)
			}
		})
	}
}

func TestPreviewExcludesMarksALineThatSilencesDataOutsideHostData(t *testing.T) {
	s, in, effective := mountedExcludeSvc(t)
	in.Mounts = append(in.Mounts,
		model.Mount{Type: "volume", Source: anonVolume, Destination: "/var/lib/beszel-agent"},
		model.Mount{Type: "volume", Source: namedVolume, Destination: "/cache"},
	)
	cases := []struct{ line, status string }{
		{"/var/lib/beszel-agent", "outside"},
		{"/var/lib", "outside"},
		{"/cache/", "outside"},
		{"/other", "outside"},
		{"/other/anything", "translated"},
		{"/cache/transcodes", "passthrough"},
		{"/", "everything"},
		{"Cache", "basename"},
		{"/config/Transcode", "translated"},
	}
	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			got := s.previewExcludes(context.Background(), []string{tc.line}, in, effective)
			if len(got) != 1 || got[0].Status != tc.status || got[0].Missing {
				t.Fatalf("preview of %q = %+v, want status %s", tc.line, got, tc.status)
			}
		})
	}
}

func TestPreviewExcludesWarnsAboutALineThatWouldExcludeEverything(t *testing.T) {
	s, in, effective := mountedExcludeSvc(t)
	cases := []struct{ line, status string }{
		{"/", "everything"},
		{" // ", "everything"},
		{"/.", "everything"},
		{"/config/..", "everything"},
		{"*", "everything"},
		{"**", "everything"},
		{"*/", "everything"},
		{"/*", "everything"},
		{"/**", "everything"},
		{"**/*", "everything"},
		{"*.log", "basename"},
		{"/config/*", "translated"},
		{"/data/*", "passthrough"},
	}
	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			got := s.previewExcludes(context.Background(), []string{tc.line}, in, effective)
			if len(got) != 1 || got[0].Status != tc.status {
				t.Fatalf("preview of %q = %+v, want status %s", tc.line, got, tc.status)
			}
			if tc.status == "everything" && (got[0].Matches || got[0].Missing) {
				t.Fatalf("preview of %q = %+v, want neither a match nor a missing path", tc.line, got)
			}
			if kept := s.resolveExcludePatterns([]string{tc.line}, in); (tc.status == "everything") != (len(kept) == 0) {
				t.Fatalf("resolveExcludePatterns(%q) = %q", tc.line, kept)
			}
		})
	}
}

func TestPreviewExcludesDoesNotCallAPathMissingWhenTheSearchCannotFinish(t *testing.T) {
	s, in, effective := mountedExcludeSvc(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := s.previewExcludes(ctx, []string{"/config/**/Nope"}, in, effective)
	if len(got) != 1 || !got[0].Matches || got[0].Missing {
		t.Fatalf("preview = %+v, want a match assumed when the search stopped early", got)
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
