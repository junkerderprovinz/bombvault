package api

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/model"
)

func TestPresetMatchesEveryPublisherOfAnApp(t *testing.T) {
	cases := map[string]string{
		"lscr.io/linuxserver/plex:latest":          "/config/Library/Application Support/Plex Media Server/Cache",
		"linuxserver/plex":                         "/config/Library/Application Support/Plex Media Server/Cache",
		"plexinc/pms-docker:1.40":                  "/config/Library/Application Support/Plex Media Server/Cache",
		"ghcr.io/hotio/plex:release":               "/config/Cache",
		"binhex/arch-plexpass":                     "/config/Plex Media Server/Cache",
		"ghcr.io/linuxserver/sonarr@sha256:abc123": "/config/MediaCover",
	}
	for image, want := range cases {
		p, ok := appPresetFor(image)
		if !ok {
			t.Errorf("%s: no preset", image)
			continue
		}
		if !presetHasLine(p, want) {
			t.Errorf("%s: preset %q lacks %q", image, p.app, want)
		}
	}
}

func TestOfficialImageNameDoesNotCatchAForeignPublisher(t *testing.T) {
	if p, ok := appPresetFor("nextcloud:29-apache"); !ok || !presetHasLine(p, "/var/www/html/data/appdata_*/preview") {
		t.Fatalf("the library image must get the official layout, got %+v", p)
	}
	if p, ok := appPresetFor("docker.io/library/nextcloud"); !ok || !presetHasLine(p, "/var/www/html/data/appdata_*/preview") {
		t.Fatalf("the fully qualified library image must get the official layout, got %+v", p)
	}
	if p, ok := appPresetFor("lscr.io/linuxserver/nextcloud"); !ok || !presetHasLine(p, "/data/appdata_*/preview") {
		t.Fatalf("linuxserver keeps its own layout, got %+v", p)
	}
	if p, ok := appPresetFor("someone/nextcloud"); ok {
		t.Fatalf("an unknown publisher must not get a layout it may not have, got %+v", p)
	}
}

func TestUnknownImageHasNoPreset(t *testing.T) {
	for _, image := range []string{"", "alpine:3.22", "example.com/team/app"} {
		if _, ok := appPresetFor(image); ok {
			t.Errorf("%q must have no preset", image)
		}
	}
}

func TestEveryPresetEntryHasAKnownKind(t *testing.T) {
	for _, p := range appPresets {
		if len(p.images) == 0 || len(p.entries) == 0 {
			t.Errorf("preset %q is empty", p.app)
		}
		for _, e := range p.entries {
			if !presetKinds[e.kind] {
				t.Errorf("preset %q: kind %q has no text in the interface", p.app, e.kind)
			}
		}
	}
}

// Only lines that land inside a folder this container backs up are offered, so
// an image with two possible layouts shows the one it really uses.
func TestPresetOffersOnlyLinesInsideBackedUpFolders(t *testing.T) {
	svc, _, _ := suggestFixture(t)
	dir := svc.cfg.DataDir
	svc.cfg.HostSourceRoot = "/mnt"
	svc.cfg.HostMountRoot = filepath.ToSlash(svc.cfg.DataDir)
	data := filepath.Join(dir, "user", "photos")
	if err := os.MkdirAll(data, 0o750); err != nil {
		t.Fatal(err)
	}
	svc.docker = &suggestFakeDocker{inspect: model.Inspect{
		Name:   "/immich",
		Config: model.Config{Image: "ghcr.io/immich-app/immich-server:v2.1.0"},
		Mounts: []model.Mount{{Source: "/mnt/user/photos", Destination: "/data"}},
	}}
	if err := svc.store.SetBackupPaths("immich", []string{filepath.ToSlash(data)}); err != nil {
		t.Fatal(err)
	}

	res, err := svc.SuggestExcludes(context.Background(), "immich", "live")
	if err != nil {
		t.Fatal(err)
	}
	if res.Preset == nil || res.Preset.App != "immich" {
		t.Fatalf("preset = %+v, want immich", res.Preset)
	}
	var lines []string
	for _, e := range res.Preset.Entries {
		lines = append(lines, e.Line)
	}
	if len(lines) != 2 || lines[0] != "/data/thumbs" || lines[1] != "/data/encoded-video" {
		t.Fatalf("lines = %v, want only the two under the mounted /data", lines)
	}
}

func TestPresetLeavesOutLinesAlreadyExcluded(t *testing.T) {
	svc, _, root := suggestFixture(t)
	svc.cfg.HostSourceRoot = "/mnt"
	svc.cfg.HostMountRoot = filepath.ToSlash(svc.cfg.DataDir)
	rel, err := filepath.Rel(svc.cfg.DataDir, filepath.FromSlash(root))
	if err != nil {
		t.Fatal(err)
	}
	svc.docker = &suggestFakeDocker{inspect: model.Inspect{
		Name:   "/plex",
		Config: model.Config{Image: "lscr.io/linuxserver/sonarr"},
		Mounts: []model.Mount{{Source: "/mnt/" + filepath.ToSlash(rel), Destination: "/config"}},
	}}
	if err := svc.store.SetExcludes("plex", []string{"/config/logs"}); err != nil {
		t.Fatal(err)
	}

	res, err := svc.SuggestExcludes(context.Background(), "plex", "live")
	if err != nil {
		t.Fatal(err)
	}
	if res.Preset == nil {
		t.Fatal("no preset for sonarr")
	}
	for _, e := range res.Preset.Entries {
		if e.Line == "/config/logs" {
			t.Fatal("a line that is already excluded is offered again")
		}
	}
	if !presetEntryOffered(res.Preset, "/config/MediaCover") {
		t.Fatalf("MediaCover missing from %+v", res.Preset.Entries)
	}
}

func presetHasLine(p appPreset, line string) bool {
	for _, e := range p.entries {
		if e.line == line {
			return true
		}
	}
	return false
}

func presetEntryOffered(p *ExcludePreset, line string) bool {
	for _, e := range p.Entries {
		if e.Line == line {
			return true
		}
	}
	return false
}
