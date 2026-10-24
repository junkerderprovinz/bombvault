package activity

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testdata(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name)) //nolint:gosec // G304: a fixture of this package
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPlexCountsEveryVideoTrackAndPhoto(t *testing.T) {
	sessions, err := ParsePlex(testdata(t, "plex_sessions.xml"))
	if err != nil {
		t.Fatal(err)
	}
	want := []Session{
		{ID: "abc123session", Position: 1834000},
		{ID: "def456session", Paused: true, Position: 64000},
		{ID: "tablet-carol/99"},
	}
	if len(sessions) != len(want) {
		t.Fatalf("sessions = %+v", sessions)
	}
	for i := range want {
		if sessions[i] != want[i] {
			t.Errorf("session %d = %+v, want %+v", i, sessions[i], want[i])
		}
	}
}

func TestAnIdlePlexHasNoSessions(t *testing.T) {
	sessions, err := ParsePlex(testdata(t, "plex_idle.xml"))
	if err != nil || len(sessions) != 0 {
		t.Fatalf("sessions = %+v, %v", sessions, err)
	}
}

func TestAnAnswerThatIsNoSessionListIsAnError(t *testing.T) {
	if _, err := ParsePlex([]byte("<html><body>401 Unauthorized</body></html>")); err == nil {
		t.Fatal("a login page read as a session list")
	}
	if _, err := ParseSessions([]byte("<html>")); err == nil {
		t.Fatal("HTML read as a session list")
	}
}

func TestJellyfinCountsOnlySessionsThatPlaySomething(t *testing.T) {
	sessions, err := Parse(Jellyfin, testdata(t, "jellyfin_sessions.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 {
		t.Fatalf("sessions = %+v, want the two with a NowPlayingItem", sessions)
	}
	if sessions[0].Paused || sessions[0].Position != 18340000000 {
		t.Errorf("first = %+v", sessions[0])
	}
	paused := time.Date(2026, time.October, 24, 20, 0, 0, 0, time.UTC)
	if !sessions[1].Paused || !sessions[1].PausedSince.Equal(paused) {
		t.Errorf("second = %+v, want paused since 20:00 UTC", sessions[1])
	}
}

func TestEmbyReadsPausedSessionsAndSkipsANullItem(t *testing.T) {
	sessions, err := Parse(Emby, testdata(t, "emby_sessions.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || !sessions[0].Paused || sessions[0].ID != "b71c5f6a2c3e4d9b8a7f6e5d4c3b2a19" {
		t.Fatalf("sessions = %+v", sessions)
	}
}

func TestPlexTokenComesFromPreferences(t *testing.T) {
	if got := PlexToken(testdata(t, "Preferences.xml")); got != "xyzToken-123_abc" {
		t.Fatalf("token = %q", got)
	}
	if got := PlexToken([]byte(`<Preferences FriendlyName="Tower" />`)); got != "" {
		t.Fatalf("a server never signed in gave %q", got)
	}
}

func TestEveryPausedSessionCountsWithoutAnAge(t *testing.T) {
	var p Pauses
	now := time.Now()
	s := []Session{{ID: "a", Paused: true, Position: 5}, {ID: "b"}}
	if got := p.Busy("plex", s, 0, now); len(got) != 2 {
		t.Fatalf("busy = %+v", got)
	}
	if got := p.Busy("plex", s, 0, now.Add(10*time.Hour)); len(got) != 2 {
		t.Fatalf("after ten hours busy = %+v", got)
	}
}

func TestAPauseStopsCountingOnceItStoodLongEnough(t *testing.T) {
	var p Pauses
	t0 := time.Date(2026, time.October, 24, 23, 0, 0, 0, time.UTC)
	paused := []Session{{ID: "movie", Paused: true, Position: 100}}
	if got := p.Busy("plex", paused, 30*time.Minute, t0); len(got) != 1 {
		t.Fatalf("a pause seen for the first time does not count: %+v", got)
	}
	if got := p.Busy("plex", paused, 30*time.Minute, t0.Add(20*time.Minute)); len(got) != 1 {
		t.Fatalf("a pause of 20 minutes stopped counting: %+v", got)
	}
	if got := p.Busy("plex", paused, 30*time.Minute, t0.Add(31*time.Minute)); len(got) != 0 {
		t.Fatalf("a pause of 31 minutes still counts: %+v", got)
	}
	// Played on and paused again: the clock starts afresh.
	moved := []Session{{ID: "movie", Paused: true, Position: 900}}
	if got := p.Busy("plex", moved, 30*time.Minute, t0.Add(40*time.Minute)); len(got) != 1 {
		t.Fatalf("a pause at a new position was taken for the old one: %+v", got)
	}
}

func TestAPauseTheServerDatesCountsFromThatDate(t *testing.T) {
	var p Pauses
	now := time.Date(2026, time.October, 24, 23, 0, 0, 0, time.UTC)
	s := []Session{{ID: "x", Paused: true, PausedSince: now.Add(-2 * time.Hour)}}
	if got := p.Busy("jellyfin", s, time.Hour, now); len(got) != 0 {
		t.Fatalf("a pause the server dates two hours back still counts: %+v", got)
	}
}

func TestPausesOfTwoServersAreKeptApart(t *testing.T) {
	var p Pauses
	t0 := time.Now()
	s := []Session{{ID: "same", Paused: true}}
	p.Busy("plex", s, time.Hour, t0)
	if got := p.Busy("emby", s, time.Hour, t0.Add(2*time.Hour)); len(got) != 1 {
		t.Fatalf("emby took over the pause plex saw: %+v", got)
	}
}
