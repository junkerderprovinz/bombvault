// Package activity asks a media server whether anybody is watching, so a
// backup that would stop the server can wait until nobody is.
package activity

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Preset names a media server whose sessions BombVault knows how to read.
type Preset string

const (
	None     Preset = ""
	Plex     Preset = "plex"
	Jellyfin Preset = "jellyfin"
	Emby     Preset = "emby"
)

// Valid reports whether p is one of the presets.
func (p Preset) Valid() bool {
	switch p {
	case None, Plex, Jellyfin, Emby:
		return true
	}
	return false
}

// Name is how the server calls itself.
func (p Preset) Name() string {
	switch p {
	case Plex:
		return "Plex"
	case Jellyfin:
		return "Jellyfin"
	case Emby:
		return "Emby"
	}
	return ""
}

// Port is the port the server listens on inside its container.
func (p Preset) Port() int {
	if p == Plex {
		return 32400
	}
	return 8096
}

// Path is where the server lists its sessions.
func (p Preset) Path() string {
	if p == Plex {
		return "/status/sessions"
	}
	return "/Sessions"
}

// Header is the request header that carries token.
func (p Preset) Header(token string) (name, value string) {
	switch p {
	case Plex:
		return "X-Plex-Token", token
	case Jellyfin:
		return "Authorization", `MediaBrowser Token="` + token + `"`
	default:
		return "X-Emby-Token", token
	}
}

// Session is one client playing something.
type Session struct {
	ID     string
	Paused bool
	// Position is where the playback stands. A paused session whose position
	// moved between two looks was played in between.
	Position int64
	// PausedSince is when the server says the session paused, zero when it
	// does not say.
	PausedSince time.Time
}

// Parse reads the session list p answered with.
func Parse(p Preset, body []byte) ([]Session, error) {
	if p == Plex {
		return ParsePlex(body)
	}
	return ParseSessions(body)
}

type plexItem struct {
	XMLName    xml.Name
	Key        string `xml:"ratingKey,attr"`
	ViewOffset int64  `xml:"viewOffset,attr"`
	Player     struct {
		State   string `xml:"state,attr"`
		Machine string `xml:"machineIdentifier,attr"`
	} `xml:"Player"`
	Session struct {
		ID string `xml:"id,attr"`
	} `xml:"Session"`
}

// ParsePlex reads the answer of Plex's /status/sessions. Every Video, Track
// and Photo in it is somebody playing.
func ParsePlex(body []byte) ([]Session, error) {
	var mc struct {
		XMLName xml.Name   `xml:"MediaContainer"`
		Items   []plexItem `xml:",any"`
	}
	if err := xml.Unmarshal(body, &mc); err != nil {
		return nil, fmt.Errorf("read the Plex sessions: %w", err)
	}
	out := []Session{}
	for _, it := range mc.Items {
		switch it.XMLName.Local {
		case "Video", "Track", "Photo":
		default:
			continue
		}
		id := it.Session.ID
		if id == "" {
			id = it.Player.Machine + "/" + it.Key
		}
		out = append(out, Session{ID: id, Paused: it.Player.State == "paused", Position: it.ViewOffset})
	}
	return out, nil
}

type mbSession struct {
	ID             string          `json:"Id"`
	NowPlayingItem json.RawMessage `json:"NowPlayingItem"`
	PlayState      struct {
		IsPaused      bool  `json:"IsPaused"`
		PositionTicks int64 `json:"PositionTicks"`
	} `json:"PlayState"`
	LastPausedDate string `json:"LastPausedDate"`
}

// ParseSessions reads the answer of Jellyfin's and Emby's /Sessions. A session
// plays while it has a NowPlayingItem.
func ParseSessions(body []byte) ([]Session, error) {
	var list []mbSession
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("read the sessions: %w", err)
	}
	out := []Session{}
	for _, s := range list {
		item := strings.TrimSpace(string(s.NowPlayingItem))
		if item == "" || item == "null" {
			continue
		}
		one := Session{ID: s.ID, Paused: s.PlayState.IsPaused, Position: s.PlayState.PositionTicks}
		if s.PlayState.IsPaused && s.LastPausedDate != "" {
			if at, err := time.Parse(time.RFC3339Nano, s.LastPausedDate); err == nil && at.Year() > 1 {
				one.PausedSince = at
			}
		}
		out = append(out, one)
	}
	return out, nil
}

var plexTokenRe = regexp.MustCompile(`PlexOnlineToken="([^"]*)"`)

// PlexToken is the token in a Plex Preferences.xml, "" when the server was
// never signed in.
func PlexToken(prefs []byte) string {
	if m := plexTokenRe.FindSubmatch(prefs); m != nil {
		return string(m[1])
	}
	return ""
}

// PlexPreferences are where the images people run keep Preferences.xml,
// relative to the folder mounted at /config.
var PlexPreferences = []string{
	"Library/Application Support/Plex Media Server/Preferences.xml",
	"Plex Media Server/Preferences.xml",
	"Preferences.xml",
}

// Pauses remembers since when each paused session has stood still, so a movie
// paused for the night stops counting once it has stood long enough. Plex does
// not say when a session paused, so the first look that finds it paused is
// when the clock starts.
type Pauses struct {
	mu   sync.Mutex
	seen map[string]pauseMark
}

type pauseMark struct {
	position int64
	since    time.Time
	looked   time.Time
}

// forgetAfter is how long a pause nobody looked at is remembered.
const forgetAfter = 48 * time.Hour

// Busy returns the sessions that keep the server busy: every one that plays,
// and every paused one that has stood still for less than ignoreAfter. With
// ignoreAfter 0 every paused session counts. key tells servers apart.
func (p *Pauses) Busy(key string, sessions []Session, ignoreAfter time.Duration, now time.Time) []Session {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.seen == nil {
		p.seen = map[string]pauseMark{}
	}
	for k, m := range p.seen {
		if now.Sub(m.looked) > forgetAfter {
			delete(p.seen, k)
		}
	}
	var busy []Session
	for _, s := range sessions {
		id := key + "\x00" + s.ID
		if !s.Paused {
			delete(p.seen, id)
			busy = append(busy, s)
			continue
		}
		mark, ok := p.seen[id]
		if !ok || mark.position != s.Position {
			mark = pauseMark{position: s.Position, since: now}
		}
		if !s.PausedSince.IsZero() && s.PausedSince.Before(mark.since) {
			mark.since = s.PausedSince
		}
		mark.looked = now
		p.seen[id] = mark
		if ignoreAfter <= 0 || now.Sub(mark.since) < ignoreAfter {
			busy = append(busy, s)
		}
	}
	return busy
}
