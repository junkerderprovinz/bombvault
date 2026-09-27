package api

import (
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/model"
)

// presetKinds are the entry kinds the interface has a sentence for. Each one
// says what the folder holds and how the app fills it again after a restore.
var presetKinds = map[string]bool{
	"cache":        true,
	"logs":         true,
	"crashReports": true,
	"previews":     true,
	"trickplay":    true,
	"metadata":     true,
	"covers":       true,
	"encodedVideo": true,
	"models":       true,
}

// presetEntry is one exclude line in the container's own view. Optional
// entries start unticked because the folder can also hold something the app
// cannot fetch again.
type presetEntry struct {
	line     string
	kind     string
	optional bool
}

// appPreset is the set of folders one image layout regenerates by itself.
// The paths come from each app's documentation or source; the design note
// names the source of every line.
type appPreset struct {
	app     string
	images  []string
	entries []presetEntry
}

const plexLSIO = "/config/Library/Application Support/Plex Media Server"

func plexEntries(root string) []presetEntry {
	return []presetEntry{
		{line: root + "/Cache", kind: "cache"},
		{line: root + "/Crash Reports", kind: "crashReports"},
		{line: root + "/Logs", kind: "logs"},
		{line: root + "/Media", kind: "previews"},
	}
}

func arrEntries() []presetEntry {
	return []presetEntry{
		{line: "/config/MediaCover", kind: "covers"},
		{line: "/config/logs", kind: "logs"},
	}
}

func arrImages(app string) []string {
	return []string{"linuxserver/" + app, "hotio/" + app, "binhex/arch-" + app}
}

var appPresets = []appPreset{
	{app: "plex", images: []string{"linuxserver/plex", "plexinc/pms-docker"}, entries: plexEntries(plexLSIO)},
	{app: "plex", images: []string{"hotio/plex"}, entries: plexEntries("/config")},
	{app: "plex", images: []string{"binhex/arch-plex", "binhex/arch-plexpass"}, entries: plexEntries("/config/Plex Media Server")},
	{app: "jellyfin", images: []string{"jellyfin/jellyfin"}, entries: []presetEntry{
		{line: "/cache", kind: "cache"},
		{line: "/config/log", kind: "logs"},
		{line: "/config/data/trickplay", kind: "trickplay"},
		{line: "/config/metadata", kind: "metadata", optional: true},
	}},
	{app: "jellyfin", images: []string{"linuxserver/jellyfin", "hotio/jellyfin"}, entries: []presetEntry{
		{line: "/config/cache", kind: "cache"},
		{line: "/config/log", kind: "logs"},
		{line: "/config/data/data/trickplay", kind: "trickplay"},
		{line: "/config/data/metadata", kind: "metadata", optional: true},
	}},
	{app: "jellyfin", images: []string{"binhex/arch-jellyfin"}, entries: []presetEntry{
		{line: "/config/cache", kind: "cache"},
		{line: "/config/logs", kind: "logs"},
		{line: "/config/data/data/trickplay", kind: "trickplay"},
		{line: "/config/data/metadata", kind: "metadata", optional: true},
	}},
	{app: "emby", images: []string{"emby/embyserver", "linuxserver/emby", "binhex/arch-emby"}, entries: []presetEntry{
		{line: "/config/cache", kind: "cache"},
	}},
	{app: "sonarr", images: arrImages("sonarr"), entries: arrEntries()},
	{app: "radarr", images: arrImages("radarr"), entries: arrEntries()},
	{app: "lidarr", images: arrImages("lidarr"), entries: arrEntries()},
	{app: "readarr", images: arrImages("readarr"), entries: arrEntries()},
	{app: "prowlarr", images: arrImages("prowlarr"), entries: []presetEntry{
		{line: "/config/logs", kind: "logs"},
	}},
	{app: "immich", images: []string{"immich-app/immich-server", "altran1502/immich-server"}, entries: []presetEntry{
		{line: "/data/thumbs", kind: "previews"},
		{line: "/data/encoded-video", kind: "encodedVideo"},
		{line: "/usr/src/app/upload/thumbs", kind: "previews"},
		{line: "/usr/src/app/upload/encoded-video", kind: "encodedVideo"},
	}},
	{app: "immich", images: []string{"imagegenius/immich"}, entries: []presetEntry{
		{line: "/photos/thumbs", kind: "previews"},
		{line: "/photos/encoded-video", kind: "encodedVideo"},
		{line: "/config/machine-learning/models", kind: "models"},
	}},
	{app: "immich", images: []string{"immich-app/immich-machine-learning"}, entries: []presetEntry{
		{line: "/cache", kind: "models"},
	}},
	{app: "nextcloud", images: []string{"linuxserver/nextcloud"}, entries: []presetEntry{
		{line: "/data/appdata_*/preview", kind: "previews"},
	}},
	{app: "nextcloud", images: []string{"nextcloud"}, entries: []presetEntry{
		{line: "/var/www/html/data/appdata_*/preview", kind: "previews"},
	}},
	{app: "photoprism", images: []string{"photoprism/photoprism"}, entries: []presetEntry{
		{line: "/photoprism/storage/cache/thumbnails", kind: "previews"},
	}},
	{app: "tautulli", images: []string{"tautulli/tautulli", "linuxserver/tautulli"}, entries: []presetEntry{
		{line: "/config/cache", kind: "cache"},
		{line: "/config/logs", kind: "logs"},
	}},
}

// ExcludePreset is what the assistant offers for a recognised app: the lines
// that land inside this container's backup and are not excluded yet.
type ExcludePreset struct {
	App     string                `json:"app"`
	Entries []ExcludePresetOption `json:"entries"`
}

// ExcludePresetOption is one offered line. Kind picks the sentence the
// interface shows; Optional entries start unticked.
type ExcludePresetOption struct {
	Line     string `json:"line"`
	Kind     string `json:"kind"`
	Optional bool   `json:"optional"`
}

// appPresetFor finds the layout for an image. A pattern with a slash names a
// publisher and matches at a path boundary, so every registry mirror of it
// matches too; one without names a Docker library image and matches only
// that. The longest match wins.
func appPresetFor(image string) (appPreset, bool) {
	repo := normalizeImageRepo(image)
	if repo == "" {
		return appPreset{}, false
	}
	library := strings.TrimPrefix(strings.TrimPrefix(repo, "docker.io/"), "library/")
	var best appPreset
	bestLen := 0
	for _, p := range appPresets {
		for _, m := range p.images {
			hit := false
			if strings.Contains(m, "/") {
				hit = repo == m || strings.HasSuffix(repo, "/"+m)
			} else {
				hit = library == m
			}
			if hit && len(m) > bestLen {
				best, bestLen = p, len(m)
			}
		}
	}
	return best, bestLen > 0
}

// excludePresetFor offers the preset lines that resolve through one of the
// container's mounts into a folder it backs up and are not stored yet. A
// layout with two possible paths (an older and a newer mount point) thereby
// shows only the one this container uses.
func (s *Service) excludePresetFor(image string, in model.Inspect, backedUp, stored []string) *ExcludePreset {
	p, ok := appPresetFor(image)
	if !ok {
		return nil
	}
	have := make(map[string]bool, len(stored))
	for _, l := range stored {
		have[strings.TrimSpace(l)] = true
	}
	out := &ExcludePreset{App: p.app}
	for _, e := range p.entries {
		if have[e.line] {
			continue
		}
		pattern, status := s.resolveExcludeLine(e.line, in)
		if status != "translated" || !isUnderAny(pattern, backedUp) {
			continue
		}
		out.Entries = append(out.Entries, ExcludePresetOption{Line: e.line, Kind: e.kind, Optional: e.optional})
	}
	if len(out.Entries) == 0 {
		return nil
	}
	return out
}
