package api

import (
	"context"
	"log"
	"net"
	"net/url"
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

// maxStreamHoldMin is the longest wait after a stream the card accepts. The
// watch keeps its rates that long and one poll more, so the last reading of a
// stream is still there when the wait ends.
const maxStreamHoldMin = 120

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
			watch: traffic.NewWatch(maxStreamHoldMin*time.Minute+trafficPoll, 3*trafficPoll),
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

// proxiedBackends are the restic backends that speak HTTP to a cloud service,
// whose upload the proxy can slow while it runs.
var proxiedBackends = []string{"b2:", "azure:", "gs:", "swift:"}

// rcloneHTTPTypes are the rclone backends that reach their storage over HTTP.
// rclone sends only that traffic through the proxy in its environment.
var rcloneHTTPTypes = []string{
	"azureblob", "azurefiles", "b2", "box", "drive", "dropbox", "fichier", "filefabric",
	"gofile", "google cloud storage", "hidrive", "http", "iclouddrive", "internetarchive",
	"jottacloud", "koofr", "linkbox", "mailru", "mega", "netstorage", "onedrive",
	"opendrive", "oracleobjectstorage", "pcloud", "pikpak", "pixeldrain", "premiumizeme",
	"protondrive", "putio", "qingstor", "quatrix", "s3", "seafile", "sharefile",
	"sugarsync", "swift", "uptobox", "webdav", "yandex", "zoho",
}

// rcloneWrapperTypes are the rclone backends that store through the one remote
// their "remote" key names.
var rcloneWrapperTypes = []string{"alias", "cache", "chunker", "compress", "crypt", "hasher"}

// proxiedBackend reports whether restic's upload to repo goes through the
// proxy in its environment.
func (s *Service) proxiedBackend(repo string) bool {
	switch {
	case strings.HasPrefix(repo, "rest:"):
		return proxyReaches(strings.TrimPrefix(repo, "rest:"))
	case strings.HasPrefix(repo, "s3:"):
		endpoint := strings.TrimPrefix(repo, "s3:")
		if !strings.Contains(endpoint, "://") {
			endpoint = "https://" + endpoint
		}
		return proxyReaches(endpoint)
	case strings.HasPrefix(repo, "rclone:"):
		conf, err := os.ReadFile(s.rcloneConfPath())
		return err == nil && rcloneOverHTTP(string(conf), strings.TrimPrefix(repo, "rclone:"))
	}
	return slices.ContainsFunc(proxiedBackends, func(p string) bool { return strings.HasPrefix(repo, p) })
}

// proxyReaches reports whether Go's HTTP client, which restic and rclone use,
// sends a request for rawURL through a proxy set in the environment. It never
// does for a loopback host.
func proxyReaches(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	host := u.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		return !ip.IsLoopback()
	}
	return host != "" && host != "localhost"
}

// rcloneOverHTTP reports whether rclone stores to remote, such as
// "cloud:bucket/path", over HTTP, following a wrapping remote to the one it
// stores through. A remote that is not in conf counts as not over HTTP.
func rcloneOverHTTP(conf, remote string) bool {
	sections := map[string]map[string]string{}
	for _, b := range splitRcloneSections(conf) {
		sections[b.name] = rcloneKeys(b.text)
	}
	// The bound stops a chain of remotes that wrap each other.
	for range len(sections) + 1 {
		name, _, ok := strings.Cut(remote, ":")
		if !ok {
			return false
		}
		name, _, _ = strings.Cut(name, ",")
		keys, ok := sections[name]
		if !ok {
			return false
		}
		if !slices.Contains(rcloneWrapperTypes, keys["type"]) {
			return slices.Contains(rcloneHTTPTypes, keys["type"])
		}
		remote = keys["remote"]
	}
	return false
}

// rcloneKeys reads the "key = value" lines of one config section.
func rcloneKeys(section string) map[string]string {
	keys := map[string]string{}
	for _, line := range strings.Split(section, "\n") {
		k, v, ok := strings.Cut(line, "=")
		if ok {
			keys[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return keys
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
	if s.proxiedBackend(dest) && !ambientProxy() {
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
