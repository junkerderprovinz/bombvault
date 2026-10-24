package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// gateDocker is the traffic fake with a way into the container, which
// answers the probe's curl with out.
type gateDocker struct {
	*trafficDocker
	out  string
	code int
	err  error
	argv [][]string
}

func (d *gateDocker) ExecOutput(_ context.Context, _ string, argv []string, _ int) (string, int, error) {
	d.argv = append(d.argv, argv)
	return d.out, d.code, d.err
}

const twoPlexSessions = `<MediaContainer size="2">
<Video ratingKey="1" viewOffset="10"><Player machineIdentifier="tv" state="playing"/><Session id="s1"/></Video>
<Track ratingKey="2" viewOffset="20"><Player machineIdentifier="phone" state="paused"/><Session id="s2"/></Track>
</MediaContainer>`

func newGateService(t *testing.T) (*Service, *store.Repo, *gateDocker) {
	t.Helper()
	s, st, d := newTrafficService(t)
	s.cfg.AppKey = strings.Repeat("a", 64)
	g := &gateDocker{trafficDocker: d}
	s.docker = g
	return s, st, g
}

func plexInspect(t *testing.T, s *Service) model.Inspect {
	t.Helper()
	dir := filepath.ToSlash(t.TempDir())
	s.cfg.HostSourceRoot, s.cfg.HostMountRoot = dir, dir
	prefs := filepath.Join(dir, "appdata", "plex", "Library", "Application Support", "Plex Media Server")
	if err := os.MkdirAll(prefs, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(prefs, "Preferences.xml"), []byte(`<Preferences PlexOnlineToken="tok-1" />`), 0o600); err != nil {
		t.Fatal(err)
	}
	return model.Inspect{Name: "/plex", Running: true, Mounts: []model.Mount{{Type: "bind", Source: dir + "/appdata/plex", Destination: "/config"}}}
}

func TestPlexIsAskedWithTheTokenFromItsPreferences(t *testing.T) {
	s, st, d := newGateService(t)
	tg := deferralTarget(t, st, "plex")
	setPolicy(t, st, tg, func(p *store.DeferPolicy) { p.Preset = "plex" })
	d.out = twoPlexSessions
	busy := s.busyCheck(tg, plexInspect(t, s))
	if busy == nil {
		t.Fatal("a container with a preset has no check")
	}
	if got := busy(context.Background()); got != "Plex has 2 active sessions" {
		t.Fatalf("reason = %q", got)
	}
	if len(d.argv) != 1 || d.argv[0][6] != "tok-1" {
		t.Fatalf("argv = %q, want the token from Preferences.xml", d.argv)
	}
}

func TestAPausedSessionStopsCountingAfterItsMinutes(t *testing.T) {
	s, st, d := newGateService(t)
	tg := deferralTarget(t, st, "plex")
	setPolicy(t, st, tg, func(p *store.DeferPolicy) { p.Preset, p.PausedMin = "plex", 30 })
	d.out = `<MediaContainer><Video ratingKey="1" viewOffset="10"><Player state="paused"/><Session id="s1"/></Video></MediaContainer>`
	in := plexInspect(t, s)
	p, _ := st.DeferPolicyOf(tg.ID)
	t0 := time.Now()
	if n, err := s.askSessions(context.Background(), tg, in, p, "plex", t0); err != nil || n != 1 {
		t.Fatalf("first look: %d, %v", n, err)
	}
	if n, err := s.askSessions(context.Background(), tg, in, p, "plex", t0.Add(31*time.Minute)); err != nil || n != 0 {
		t.Fatalf("after 31 minutes: %d, %v; want the pause ignored", n, err)
	}
}

func TestAnIdleServerLetsTheBackupThrough(t *testing.T) {
	s, st, d := newGateService(t)
	tg := deferralTarget(t, st, "plex")
	setPolicy(t, st, tg, func(p *store.DeferPolicy) { p.Preset = "plex" })
	d.out = `<MediaContainer size="0"></MediaContainer>`
	if got := s.busyCheck(tg, plexInspect(t, s))(context.Background()); got != "" {
		t.Fatalf("reason = %q", got)
	}
}

func TestAServerThatCannotBeAskedFailsOpenOrClosed(t *testing.T) {
	for _, open := range []bool{true, false} {
		s, st, d := newGateService(t)
		tg := deferralTarget(t, st, "jf")
		key, err := secret.Encrypt(s.cfg.AppKey, []byte("k"))
		if err != nil {
			t.Fatal(err)
		}
		setPolicy(t, st, tg, func(p *store.DeferPolicy) { p.Preset, p.APIKeyEnc, p.FailOpen = "jellyfin", key, open })
		d.err = errors.New("container is not running")
		got := s.busyCheck(tg, model.Inspect{Name: "/jf", Running: true})(context.Background())
		switch {
		case open && got != "":
			t.Fatalf("fail-open deferred with %q", got)
		case !open && !strings.Contains(got, "could not ask Jellyfin"):
			t.Fatalf("fail-closed gave %q", got)
		}
	}
}

func TestJellyfinWithoutAKeyCannotBeAsked(t *testing.T) {
	s, st, d := newGateService(t)
	tg := deferralTarget(t, st, "jf")
	setPolicy(t, st, tg, func(p *store.DeferPolicy) { p.Preset, p.FailOpen = "jellyfin", false })
	got := s.busyCheck(tg, model.Inspect{Name: "/jf", Running: true})(context.Background())
	if !strings.Contains(got, "no API key") || len(d.argv) != 0 {
		t.Fatalf("reason = %q, exec calls %d", got, len(d.argv))
	}
}

func TestAContainerWithNoCheckGetsNone(t *testing.T) {
	s, st, _ := newGateService(t)
	tg := deferralTarget(t, st, "db")
	if s.busyCheck(tg, model.Inspect{Running: true}) != nil {
		t.Fatal("a container without a preset or a traffic limit got a check")
	}
}

func TestTheTrafficGateDefersWhileTheContainerSends(t *testing.T) {
	for _, tc := range []struct {
		name  string
		polls int
		tx    uint64
		want  string
	}{
		{"after 20 seconds of readings", 3, 10, "has not yet watched"},
		{"while it sends", 12, 50_000_000, "sent 100 KiB/s or more within the last 1 minutes"},
		{"after a quiet minute", 12, 1_000, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, st, g := newGateService(t)
			tg := deferralTarget(t, st, "db")
			setPolicy(t, st, tg, func(p *store.DeferPolicy) { p.NetKiB, p.NetMin = 100, 1 })
			load(s, g.trafficDocker, "db", tc.polls, 0, tc.tx)
			if g.reads["db"] != tc.polls {
				t.Fatalf("reads = %d, the traffic watch does not read a container with a traffic limit", g.reads["db"])
			}
			got := s.busyCheck(tg, model.Inspect{Name: "/db", Running: true})(context.Background())
			if (tc.want == "" && got != "") || !strings.Contains(got, tc.want) {
				t.Fatalf("reason = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTheTrafficGateLetsAContainerOnTheHostNetworkThrough(t *testing.T) {
	s, st, _ := newGateService(t)
	tg := deferralTarget(t, st, "jf")
	setPolicy(t, st, tg, func(p *store.DeferPolicy) { p.NetKiB = 100 })
	in := model.Inspect{Name: "/jf", Running: true, HostConfig: model.HostConfig{NetworkMode: "host"}}
	if got := s.busyCheck(tg, in)(context.Background()); got != "" {
		t.Fatalf("reason = %q", got)
	}
}
