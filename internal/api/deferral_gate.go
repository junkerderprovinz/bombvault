package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/activity"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// sessionsTimeout bounds one look at a media server, the way through the
// container included.
const sessionsTimeout = 30 * time.Second

// busyCheck is the activity check of a container's backup, nil when its
// policy asks for none. It names why the app is busy, or returns "".
func (s *Service) busyCheck(tg store.Target, in model.Inspect) func(context.Context) string {
	p := s.policyOf(tg.ID)
	preset := activity.Preset(p.Preset)
	if preset == activity.None && p.NetKiB <= 0 {
		return nil
	}
	return func(ctx context.Context) string {
		if p.NetKiB > 0 {
			if why := s.sendingGate(tg.ContainerName, in, p, time.Now()); why != "" {
				return why
			}
		}
		if preset != activity.None {
			return s.sessionGate(ctx, tg, in, p, preset)
		}
		return ""
	}
}

// sendingGate defers the backup while the container sent at least the
// policy's rate within its window. A container on the host network has no
// counters of its own, so the gate lets it through.
func (s *Service) sendingGate(name string, in model.Inspect, p store.DeferPolicy, now time.Time) string {
	if in.HostConfig.NetworkMode == "host" {
		log.Printf("api: %s is on the host network, whose traffic Docker does not count; the traffic check lets its backup through", name) //nolint:gosec // G706: container names from Docker
		return ""
	}
	window := time.Duration(p.NetMin) * time.Minute
	sending, measured := s.trafficState().watch.Sending(name, float64(p.NetKiB)*1024, window, now)
	switch {
	case sending:
		return store.DeferredSending(p.NetKiB, p.NetMin)
	case !measured:
		return store.DeferredMeasuring(p.NetMin)
	}
	return ""
}

// netGateContainers names the containers whose backup waits while they send,
// so the traffic watch reads them ahead of their backup.
func (s *Service) netGateContainers() []string {
	policies, err := s.store.DeferPolicies()
	if err != nil || len(policies) == 0 {
		return nil
	}
	targets, err := s.store.ListTargets()
	if err != nil {
		return nil
	}
	var names []string
	for _, t := range targets {
		if p, ok := policies[t.ID]; ok && p.NetKiB > 0 {
			names = append(names, t.ContainerName)
		}
	}
	return names
}

// errNoKey is a Jellyfin or Emby with no API key to ask it with.
var errNoKey = errors.New("no API key is set")

// sessionGate asks the media server whether anybody is watching. A server
// that cannot be asked lets the backup through or defers it, as the policy
// says. Why it could not be asked goes to the log only: the error names
// addresses inside the host.
func (s *Service) sessionGate(ctx context.Context, tg store.Target, in model.Inspect, p store.DeferPolicy, preset activity.Preset) string {
	ctx, cancel := context.WithTimeout(ctx, sessionsTimeout)
	defer cancel()
	busy, err := s.askSessions(ctx, tg, in, p, preset, time.Now())
	if err != nil {
		if p.FailOpen {
			log.Printf("api: could not ask %s whether anybody watches on %s, backing up anyway: %v", preset.Name(), tg.ContainerName, err) //nolint:gosec // G706: container names from Docker
			return ""
		}
		log.Printf("api: could not ask %s whether anybody watches on %s, the backup waits: %v", preset.Name(), tg.ContainerName, err) //nolint:gosec // G706: container names from Docker
		if errors.Is(err, errNoKey) {
			return store.DeferredNoKey(preset.Name())
		}
		return store.DeferredNoAnswer(preset.Name())
	}
	if busy == 0 {
		return ""
	}
	return store.DeferredSessions(preset.Name(), busy)
}

// askSessions counts the sessions that keep the server busy.
func (s *Service) askSessions(ctx context.Context, tg store.Target, in model.Inspect, p store.DeferPolicy, preset activity.Preset, now time.Time) (int, error) {
	token, err := s.presetToken(ctx, tg.ContainerName, in, p, preset)
	if err != nil {
		return 0, err
	}
	probe := activity.Probe{
		Preset: preset,
		Token:  token,
		Addr:   s.directAddr(ctx, tg.ContainerName, in),
		Exec: func(ctx context.Context, argv []string) (string, int, error) {
			return s.docker.ExecOutput(ctx, tg.ContainerName, argv, 4<<20)
		},
	}
	body, _, err := probe.Fetch(ctx)
	if err != nil {
		return 0, err
	}
	sessions, err := activity.Parse(preset, body)
	if err != nil {
		return 0, err
	}
	busy := s.deferralState().pauses.Busy(tg.ID, sessions, time.Duration(p.PausedMin)*time.Minute, now)
	return len(busy), nil
}

// directAddr is the address BombVault reaches the container at, "" for one on
// the host network or without an address.
func (s *Service) directAddr(ctx context.Context, name string, in model.Inspect) string {
	if in.HostConfig.NetworkMode == "host" {
		return ""
	}
	infos, err := s.docker.List(ctx)
	if err != nil {
		return ""
	}
	for _, c := range infos {
		if c.Name == name {
			return c.IP
		}
	}
	return ""
}

// presetToken is what the server is asked with: Plex's own token from its
// Preferences.xml, or the API key stored for Jellyfin and Emby.
func (s *Service) presetToken(ctx context.Context, name string, in model.Inspect, p store.DeferPolicy, preset activity.Preset) (string, error) {
	if preset != activity.Plex {
		if len(p.APIKeyEnc) == 0 {
			return "", fmt.Errorf("%s: %w", preset.Name(), errNoKey)
		}
		key, err := secret.Decrypt(s.cfg.AppKey, p.APIKeyEnc)
		if err != nil {
			return "", fmt.Errorf("the API key for %s cannot be read: %w", preset.Name(), err)
		}
		return string(key), nil
	}
	if prefs := s.readPlexPreferences(in); prefs != nil {
		return activity.PlexToken(prefs), nil
	}
	// A server never signed in has no token and may still answer.
	return activity.PlexToken([]byte(s.execPlexPreferences(ctx, name))), nil
}

// readPlexPreferences reads Preferences.xml through the Host Data mount, from
// the folder the container has mounted at /config.
func (s *Service) readPlexPreferences(in model.Inspect) []byte {
	for _, m := range in.Mounts {
		if path.Clean(m.Destination) != "/config" {
			continue
		}
		local, ok := s.toContainerPath(m.Source)
		if !ok {
			continue
		}
		for _, rel := range activity.PlexPreferences {
			if b, err := os.ReadFile(filepath.Join(local, filepath.FromSlash(rel))); err == nil { //nolint:gosec // G304: a fixed name under the container's own mount
				return b
			}
		}
	}
	return nil
}

// execPlexPreferences reads Preferences.xml inside the container, for a
// /config that lies outside the Host Data mount.
func (s *Service) execPlexPreferences(ctx context.Context, name string) string {
	argv := []string{"sh", "-c", `for f in "$@"; do if [ -f "$f" ]; then exec cat "$f"; fi; done; exit 1`, "sh"}
	for _, rel := range activity.PlexPreferences {
		argv = append(argv, "/config/"+rel)
	}
	out, code, err := s.docker.ExecOutput(ctx, name, argv, 1<<20)
	if err != nil || code != 0 || !strings.Contains(out, "PlexOnlineToken") {
		return ""
	}
	return out
}
