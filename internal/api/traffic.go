package api

import (
	"context"
	"log"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/traffic"
)

// trafficPoll is how often the watched containers are read.
const trafficPoll = 10 * time.Second

// autoServersTTL is how long the media servers found by image name are reused
// before the container list is read again.
const autoServersTTL = time.Minute

// mediaServerImages are the image name parts that preselect a media server.
var mediaServerImages = []string{"plex", "jellyfin", "emby"}

// statsReader is the part of the Docker client the traffic watch needs.
type statsReader interface {
	Stats(ctx context.Context, name string) (traffic.Sample, error)
}

// Throttle states an off-site progress event carries.
const (
	throttleNow  = "now"
	throttleNext = "next"
)

// copyStep is how one running restic copy meets the streaming limit.
type copyStep struct {
	// live means the upload goes through the proxy, so the limit follows the
	// stream while the copy runs.
	live bool
	// held means the copy started at the streaming limit and keeps it.
	held bool
}

type trafficState struct {
	watch *traffic.Watch
	gate  *traffic.Gate
	// limit is the upload rate in bytes per second the gate holds uploads to,
	// 0 while nothing streams or the throttle is off.
	limit atomic.Int64

	mu        sync.Mutex
	streaming string
	auto      []string
	autoAt    time.Time
	steps     map[string]copyStep
}

func (s *Service) trafficState() *trafficState {
	s.trafficOnce.Do(func() {
		st := &trafficState{
			watch: traffic.NewWatch(time.Hour, 3*trafficPoll),
			steps: map[string]copyStep{},
		}
		st.gate = traffic.NewGate(func() int { return int(st.limit.Load()) })
		s.trafficSt = st
	})
	return s.trafficSt
}

// StartTrafficWatch reads the watched containers every trafficPoll until ctx
// ends. Without a Docker client that can read stats it does nothing.
func (s *Service) StartTrafficWatch(ctx context.Context) {
	sr, ok := s.docker.(statsReader)
	if !ok {
		return
	}
	go func() {
		t := time.NewTicker(trafficPoll)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.pollTraffic(ctx, sr, time.Now())
			}
		}
	}()
}

func (s *Service) pollTraffic(ctx context.Context, sr statsReader, now time.Time) {
	cfg, err := s.store.TrafficSettings()
	if err != nil {
		log.Printf("traffic: read settings: %v", err)
		return
	}
	st := s.trafficState()
	servers := s.mediaServers(ctx, cfg, now)
	watched := s.idleWaitContainers()
	if cfg.StreamThrottle {
		for _, n := range servers {
			if !slices.Contains(watched, n) {
				watched = append(watched, n)
			}
		}
	}
	for _, name := range watched {
		sctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		sample, err := sr.Stats(sctx, name)
		cancel()
		if err != nil {
			st.watch.RecordFailure(name, now)
			continue
		}
		sample.At = now
		st.watch.Record(name, sample)
	}
	st.watch.Retain(watched)
	who, on := st.watch.Streaming(streamRule(cfg, servers), now)
	st.setStreaming(who, on && cfg.StreamThrottle, cfg.StreamLimitKiB)
}

func streamRule(cfg store.TrafficSettings, servers []string) traffic.StreamRule {
	return traffic.StreamRule{
		Servers:   servers,
		Threshold: float64(cfg.StreamMbit) * 1e6 / 8,
		Hold:      time.Duration(cfg.StreamHoldMin) * time.Minute,
	}
}

func (st *trafficState) setStreaming(who string, on bool, limitKiB int) {
	st.mu.Lock()
	was := st.streaming
	if !on {
		who = ""
	}
	st.streaming = who
	st.mu.Unlock()
	if on {
		st.limit.Store(int64(limitKiB) * 1024)
	} else {
		st.limit.Store(0)
	}
	switch {
	case was == "" && who != "":
		log.Printf("traffic: %s is streaming, off-site uploads slowed to %d KiB/s", who, limitKiB)
	case was != "" && who == "":
		log.Print("traffic: streaming ended, off-site uploads back to their normal limit")
	}
}

// streamingServer names the media server whose stream holds off-site uploads
// back, "" when none does.
func (s *Service) streamingServer() string {
	st := s.trafficState()
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.streaming
}

// mediaServers are the containers watched for streams: the chosen ones, or
// until something was chosen, the ones whose image names a media server.
func (s *Service) mediaServers(ctx context.Context, cfg store.TrafficSettings, now time.Time) []string {
	if cfg.MediaServers != nil {
		return cfg.MediaServers
	}
	st := s.trafficState()
	st.mu.Lock()
	if st.autoAt.After(now.Add(-autoServersTTL)) {
		auto := st.auto
		st.mu.Unlock()
		return auto
	}
	st.mu.Unlock()
	infos, err := s.docker.List(ctx)
	if err != nil {
		log.Printf("traffic: list containers: %v", err)
		return nil
	}
	var auto []string
	for _, c := range infos {
		if isMediaServerImage(c.Image) {
			auto = append(auto, c.Name)
		}
	}
	st.mu.Lock()
	st.auto, st.autoAt = auto, now
	st.mu.Unlock()
	return auto
}

func isMediaServerImage(image string) bool {
	image = strings.ToLower(image)
	return slices.ContainsFunc(mediaServerImages, func(p string) bool { return strings.Contains(image, p) })
}

// proxiedBackends are the restic backends that speak HTTP, whose upload the
// proxy can slow while it runs. rclone passes the proxy on to its own HTTP
// remotes.
var proxiedBackends = []string{"rest:", "s3:", "b2:", "azure:", "gs:", "swift:", "rclone:"}

func proxiedBackend(repo string) bool {
	return slices.ContainsFunc(proxiedBackends, func(p string) bool { return strings.HasPrefix(repo, p) })
}

// ambientProxy reports a proxy the container already routes through. Putting
// ours in front of it would cut that route, so such a copy is throttled from
// its next step only.
func ambientProxy() bool {
	for _, k := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		if os.Getenv(k) != "" {
			return true
		}
	}
	return false
}

// throttleCopy prepares one restic copy of domain to dest for the streaming
// limit and returns the limits and mode to start it with, and a func to call
// once it ended. An HTTP backend goes through the proxy, where the limit
// changes while restic runs; any other backend starts at the streaming limit
// when a stream is on and keeps what it started with.
func (s *Service) throttleCopy(domain, dest string, lim restic.Limits, mode restic.Mode) (restic.Limits, restic.Mode, func()) {
	cfg, err := s.store.TrafficSettings()
	if err != nil || !cfg.StreamThrottle {
		return lim, mode, func() {}
	}
	st := s.trafficState()
	step := copyStep{}
	var proxy *traffic.Proxy
	if proxiedBackend(dest) && !ambientProxy() {
		p, perr := traffic.StartProxy(st.gate)
		if perr != nil {
			log.Printf("api: offsite %s: streaming limit only from the next step, proxy failed: %v", domain, perr) //nolint:gosec // G706: domain is a fixed literal
		} else {
			proxy = p
			step.live = true
			mode.Env = append(slices.Clone(mode.Env), p.Env()...)
		}
	}
	if !step.live && st.limit.Load() > 0 {
		lim.UploadKBps = lowerLimit(lim.UploadKBps, cfg.StreamLimitKiB)
		step.held = true
	}
	st.mu.Lock()
	st.steps[domain] = step
	st.mu.Unlock()
	return lim, mode, func() {
		st.mu.Lock()
		delete(st.steps, domain)
		st.mu.Unlock()
		if proxy != nil {
			_ = proxy.Close()
		}
	}
}

// lowerLimit is the stricter of two KiB/s limits where 0 means none.
func lowerLimit(a, b int) int {
	if a <= 0 {
		return b
	}
	if b <= 0 {
		return a
	}
	return min(a, b)
}

// offsiteThrottle is the throttle state the off-site progress of domain shows:
// throttleNow while its copy runs at the streaming limit, throttleNext while a
// stream runs that its copy can only meet from the next step.
func (s *Service) offsiteThrottle(domain string) string {
	st := s.trafficState()
	st.mu.Lock()
	step, ok := st.steps[domain]
	streaming := st.streaming != ""
	st.mu.Unlock()
	switch {
	case !ok:
		return ""
	case step.held, step.live && streaming:
		return throttleNow
	case streaming:
		return throttleNext
	}
	return ""
}
