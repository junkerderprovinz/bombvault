package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/traffic"
)

// trafficDocker lists fixed containers and serves each one's counters from a
// map the test advances.
type trafficDocker struct {
	dockercli.Docker
	infos []dockercli.ContainerInfo
	tx    map[string]uint64
	cpu   map[string]uint64
	down  map[string]bool
	reads map[string]int
}

func (d *trafficDocker) List(context.Context) ([]dockercli.ContainerInfo, error) {
	return d.infos, nil
}

func (d *trafficDocker) Self(context.Context) (string, error) { return "bombvault", nil }

func (d *trafficDocker) Stats(_ context.Context, name string) (traffic.Sample, error) {
	d.reads[name]++
	if d.down[name] {
		return traffic.Sample{}, errors.New("no such container")
	}
	return traffic.Sample{Running: true, HasNet: true, TxBytes: d.tx[name], CPUNanos: d.cpu[name]}, nil
}

func newTrafficService(t *testing.T) (*Service, *store.Repo, *trafficDocker) {
	t.Helper()
	s, st := newSyncTestService(t)
	d := &trafficDocker{
		infos: []dockercli.ContainerInfo{
			{Name: "plex", Image: "plexinc/pms-docker:latest"},
			{Name: "jf", Image: "lscr.io/linuxserver/jellyfin", NetworkMode: "host"},
			{Name: "db", Image: "postgres:16"},
			{Name: "bombvault", Image: "junkerderprovinz/bombvault"},
		},
		tx:    map[string]uint64{},
		cpu:   map[string]uint64{},
		down:  map[string]bool{},
		reads: map[string]int{},
	}
	s.docker = d
	return s, st, d
}

func enableThrottle(t *testing.T, st *store.Repo, servers []string) {
	t.Helper()
	cfg := store.DefaultTrafficSettings()
	cfg.StreamThrottle = true
	cfg.MediaServers = servers
	if err := st.SetTrafficSettings(cfg); err != nil {
		t.Fatal(err)
	}
}

// stream advances plex by 5 MB per poll, well above the 2 Mbit/s default.
func stream(s *Service, d *trafficDocker, from time.Time, polls int) time.Time {
	at := from
	for i := range polls {
		at = from.Add(time.Duration(i) * trafficPoll)
		s.pollTraffic(context.Background(), d, at)
		d.tx["plex"] += 5_000_000
	}
	return at
}

func TestMediaServersArePreselectedByImageName(t *testing.T) {
	s, _, _ := newTrafficService(t)
	got := s.mediaServers(context.Background(), store.DefaultTrafficSettings(), time.Now())
	if !slices.Equal(got, []string{"plex", "jf"}) {
		t.Fatalf("auto media servers = %v, want plex and jf", got)
	}
	chosen := store.DefaultTrafficSettings()
	chosen.MediaServers = []string{"db"}
	if got := s.mediaServers(context.Background(), chosen, time.Now()); !slices.Equal(got, []string{"db"}) {
		t.Fatalf("a chosen list was not kept: %v", got)
	}
}

func TestAStreamLowersTheGateAndItsEndLiftsIt(t *testing.T) {
	s, st, d := newTrafficService(t)
	enableThrottle(t, st, []string{"plex"})
	t0 := time.Unix(1_700_000_000, 0)
	last := stream(s, d, t0, 4)
	if got := s.streamingServer(); got != "plex" {
		t.Fatalf("streaming server = %q, want plex", got)
	}
	if got := s.trafficState().limit.Load(); got != 512*1024 {
		t.Fatalf("gate limit = %d, want 512 KiB/s", got)
	}
	// No traffic after the stream, polled past the five-minute hold.
	for i := 1; i <= 32; i++ {
		s.pollTraffic(context.Background(), d, last.Add(time.Duration(i)*trafficPoll))
	}
	if got := s.streamingServer(); got != "" {
		t.Fatalf("still streaming after the hold: %q", got)
	}
	if got := s.trafficState().limit.Load(); got != 0 {
		t.Fatalf("gate limit = %d after the stream, want 0", got)
	}
}

func TestAStreamIsIgnoredWhileTheThrottleIsOff(t *testing.T) {
	s, st, d := newTrafficService(t)
	cfg := store.DefaultTrafficSettings()
	cfg.MediaServers = []string{"plex"}
	if err := st.SetTrafficSettings(cfg); err != nil {
		t.Fatal(err)
	}
	stream(s, d, time.Unix(1_700_000_000, 0), 4)
	if s.streamingServer() != "" || s.trafficState().limit.Load() != 0 {
		t.Fatal("a stream slowed uploads with the throttle off")
	}
}

func TestCopyIsUntouchedWhileTheThrottleIsOff(t *testing.T) {
	s, _, _ := newTrafficService(t)
	lim, mode, release := s.throttleCopy("containers", "s3:bucket/x", restic.Limits{UploadKBps: 900}, restic.Mode{Env: []string{"A=1"}})
	defer release()
	if lim.UploadKBps != 900 || !slices.Equal(mode.Env, []string{"A=1"}) {
		t.Fatalf("limits %+v, env %v changed with the throttle off", lim, mode.Env)
	}
	if got := s.offsiteThrottle("containers"); got != "" {
		t.Fatalf("offsiteThrottle = %q", got)
	}
}

func TestHTTPBackendsGoThroughTheProxy(t *testing.T) {
	s, st, d := newTrafficService(t)
	enableThrottle(t, st, []string{"plex"})
	base := restic.Mode{Env: []string{"AWS_ACCESS_KEY_ID=k"}}
	lim, mode, release := s.throttleCopy("containers", "s3:bucket/x", restic.Limits{UploadKBps: 900}, base)
	if lim.UploadKBps != 900 {
		t.Fatalf("static limit changed to %d; the proxy is the one that slows", lim.UploadKBps)
	}
	env := strings.Join(mode.Env, "\n")
	if !strings.Contains(env, "HTTPS_PROXY=http://bombvault:") || !strings.Contains(env, "AWS_ACCESS_KEY_ID=k") {
		t.Fatalf("env = %s", env)
	}
	if len(base.Env) != 1 {
		t.Fatal("the caller's env slice was changed")
	}
	if got := s.offsiteThrottle("containers"); got != "" {
		t.Fatalf("no stream yet, offsiteThrottle = %q", got)
	}
	stream(s, d, time.Unix(1_700_000_000, 0), 3)
	if got := s.offsiteThrottle("containers"); got != throttleNow {
		t.Fatalf("streaming through the proxy, offsiteThrottle = %q, want now", got)
	}
	release()
	if got := s.offsiteThrottle("containers"); got != "" {
		t.Fatalf("after the copy, offsiteThrottle = %q", got)
	}
}

func TestOtherBackendsStartAtTheStreamingLimit(t *testing.T) {
	s, st, d := newTrafficService(t)
	enableThrottle(t, st, []string{"plex"})
	stream(s, d, time.Unix(1_700_000_000, 0), 3)
	lim, mode, release := s.throttleCopy("vms", "sftp:user@host:/repo", restic.Limits{UploadKBps: 4096}, restic.Mode{})
	defer release()
	if lim.UploadKBps != 512 {
		t.Fatalf("upload limit %d, want the 512 KiB/s streaming limit", lim.UploadKBps)
	}
	if len(mode.Env) != 0 {
		t.Fatalf("an sftp copy got proxy env: %v", mode.Env)
	}
	if got := s.offsiteThrottle("vms"); got != throttleNow {
		t.Fatalf("offsiteThrottle = %q, want now", got)
	}
}

func TestAStreamDuringALocalCopyTakesEffectAtTheNextStep(t *testing.T) {
	s, st, d := newTrafficService(t)
	enableThrottle(t, st, []string{"plex"})
	lim, _, release := s.throttleCopy("files", "/mnt/remote/repo", restic.Limits{}, restic.Mode{})
	defer release()
	if lim.UploadKBps != 0 {
		t.Fatalf("no stream, yet limit %d", lim.UploadKBps)
	}
	stream(s, d, time.Unix(1_700_000_000, 0), 3)
	if got := s.offsiteThrottle("files"); got != throttleNext {
		t.Fatalf("offsiteThrottle = %q, want next", got)
	}
}

func TestAnAmbientProxyKeepsTheCopyOffOurs(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://corp:3128")
	s, st, _ := newTrafficService(t)
	enableThrottle(t, st, []string{"plex"})
	_, mode, release := s.throttleCopy("containers", "rest:https://h/r", restic.Limits{}, restic.Mode{})
	defer release()
	if strings.Contains(strings.Join(mode.Env, "\n"), "bombvault:") {
		t.Fatal("our proxy replaced the one the container routes through")
	}
}

func TestUnreadableMediaServerNeverCountsAsStreaming(t *testing.T) {
	s, st, d := newTrafficService(t)
	enableThrottle(t, st, []string{"plex"})
	d.down["plex"] = true
	stream(s, d, time.Unix(1_700_000_000, 0), 4)
	if s.streamingServer() != "" {
		t.Fatal("a container that could not be read streams")
	}
}

func TestStreamingSettingsHandlersRoundTrip(t *testing.T) {
	s, st, _ := newTrafficService(t)
	h := &Handler{store: st, svc: s}

	rec := httptest.NewRecorder()
	h.handleGetStreaming(rec, httptest.NewRequest(http.MethodGet, "/api/settings/streaming", nil))
	env := decodeEnvelope(t, rec)
	set := env["settings"].(map[string]any)
	if set["mediaServersAuto"] != true || set["thresholdMbit"] != float64(2) {
		t.Fatalf("defaults = %v", set)
	}
	cands := env["candidates"].([]any)
	if len(cands) != 3 {
		t.Fatalf("candidates = %v, want three without BombVault itself", cands)
	}
	jf := cands[1].(map[string]any)
	if jf["name"] != "jf" || jf["hostNetwork"] != true {
		t.Fatalf("jf = %v, want marked as host network", jf)
	}

	body, _ := json.Marshal(streamingView{Enabled: true, MediaServers: []string{"db"}, ThresholdMbit: 5, LimitKiB: 300, HoldMin: 2})
	rec = httptest.NewRecorder()
	h.handleSetStreaming(rec, jsonReq(http.MethodPut, "/api/settings/streaming", bytes.NewReader(body)))
	if env := decodeEnvelope(t, rec); env["ok"] != true {
		t.Fatalf("save failed: %v", env)
	}
	cfg, err := st.TrafficSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.StreamThrottle || !slices.Equal(cfg.MediaServers, []string{"db"}) || cfg.StreamMbit != 5 || cfg.StreamLimitKiB != 300 || cfg.StreamHoldMin != 2 {
		t.Fatalf("stored %+v", cfg)
	}
}

func TestStreamingSettingsRejectOutOfRangeValues(t *testing.T) {
	s, st, _ := newTrafficService(t)
	h := &Handler{store: st, svc: s}
	for _, v := range []streamingView{
		{ThresholdMbit: 0, LimitKiB: 10, HoldMin: 1},
		{ThresholdMbit: 1, LimitKiB: 0, HoldMin: 1},
		{ThresholdMbit: 1, LimitKiB: 10, HoldMin: 0},
		{ThresholdMbit: 1, LimitKiB: 10, HoldMin: 1, MediaServers: []string{" "}},
	} {
		body, _ := json.Marshal(v)
		rec := httptest.NewRecorder()
		h.handleSetStreaming(rec, jsonReq(http.MethodPut, "/api/settings/streaming", bytes.NewReader(body)))
		if env := decodeEnvelope(t, rec); env["ok"] != false || env["error"] == "" {
			t.Fatalf("%+v was accepted: %v", v, env)
		}
	}
}

// envEngine records the environment each copy starts restic with.
type envEngine struct {
	*hookFakeEngine
	envs [][]string
}

func (e *envEngine) Copy(ctx context.Context, dest, src string, ids []string, lim restic.Limits, m restic.Mode) error {
	e.envs = append(e.envs, m.Env)
	return e.hookFakeEngine.Copy(ctx, dest, src, ids, lim, m)
}

func TestReplicationToARestServerRunsThroughTheProxy(t *testing.T) {
	inner := &hookFakeEngine{snapsByRepo: map[string][]restic.Snapshot{}}
	eng := &envEngine{hookFakeEngine: inner}
	svc, st, own, _ := hookSvc(t, inner)
	svc.engine = eng
	enableThrottle(t, st, []string{"plex"})
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}

	svc.replicateOffsite(context.Background(), "containers", settings, restic.Mode{}, own)

	if len(eng.envs) == 0 {
		t.Fatal("nothing was copied")
	}
	var proxyURL string
	for _, kv := range eng.envs[0] {
		if v, ok := strings.CutPrefix(kv, "HTTPS_PROXY="); ok {
			proxyURL = v
		}
	}
	if proxyURL == "" {
		t.Fatalf("the copy to a REST server did not get the proxy: %v", eng.envs[0])
	}
	addr := proxyURL[strings.LastIndex(proxyURL, "@")+1:]
	if c, err := (&net.Dialer{Timeout: time.Second}).Dial("tcp", addr); err == nil {
		_ = c.Close()
		t.Fatal("the proxy still listens after the copy ended")
	}
}
