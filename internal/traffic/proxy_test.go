package traffic

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// sink counts what the far side received.
func sink(t *testing.T, got *atomic.Int64) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		n, _ := io.Copy(io.Discard, r.Body)
		got.Add(n)
		_, _ = w.Write([]byte("ok"))
	}
}

func proxyClient(t *testing.T, p *Proxy) *http.Client {
	t.Helper()
	u, err := url.Parse(p.url)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(u)}, Timeout: 30 * time.Second}
}

func TestUploadsPassUnthrottledWhileTheRateIsZero(t *testing.T) {
	var got atomic.Int64
	far := httptest.NewServer(sink(t, &got))
	defer far.Close()
	p, err := StartProxy(NewGate(func() int { return 0 }))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()

	start := time.Now()
	resp, err := proxyClient(t, p).Post(far.URL, "application/octet-stream", bytes.NewReader(make([]byte, 4<<20)))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if got.Load() != 4<<20 {
		t.Fatalf("far side got %d bytes", got.Load())
	}
	if time.Since(start) > 5*time.Second {
		t.Fatalf("an unthrottled 4 MiB upload took %s", time.Since(start))
	}
}

func TestPlainHTTPUploadIsHeldToTheRate(t *testing.T) {
	var got atomic.Int64
	far := httptest.NewServer(sink(t, &got))
	defer far.Close()
	p, err := StartProxy(NewGate(func() int { return 128 << 10 }))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()

	start := time.Now()
	resp, err := proxyClient(t, p).Post(far.URL, "application/octet-stream", bytes.NewReader(make([]byte, 320<<10)))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	// 320 KiB at 128 KiB/s with a 64 KiB burst: at least two seconds.
	if d := time.Since(start); d < 1800*time.Millisecond {
		t.Fatalf("320 KiB at 128 KiB/s took only %s", d)
	}
	if got.Load() != 320<<10 {
		t.Fatalf("far side got %d bytes", got.Load())
	}
}

func TestTunnelledUploadFollowsARateChangeMidTransfer(t *testing.T) {
	var got atomic.Int64
	far := httptest.NewTLSServer(sink(t, &got))
	defer far.Close()
	var limit atomic.Int64
	limit.Store(64 << 10)
	p, err := StartProxy(NewGate(func() int { return int(limit.Load()) }))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()

	c := proxyClient(t, p)
	c.Transport.(*http.Transport).TLSClientConfig = far.Client().Transport.(*http.Transport).TLSClientConfig
	go func() {
		time.Sleep(time.Second)
		limit.Store(0)
	}()
	start := time.Now()
	resp, err := c.Post(far.URL, "application/octet-stream", bytes.NewReader(make([]byte, 8<<20)))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	// 8 MiB at 64 KiB/s would take two minutes; lifting the limit after a
	// second has to finish it in a few.
	if d := time.Since(start); d > 10*time.Second {
		t.Fatalf("the lifted limit did not reach the running upload: %s", d)
	}
	if got.Load() != 8<<20 {
		t.Fatalf("far side got %d bytes", got.Load())
	}
}

func TestProxyRefusesRequestsWithoutItsPassword(t *testing.T) {
	far := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer far.Close()
	p, err := StartProxy(NewGate(func() int { return 0 }))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()

	u, _ := url.Parse(p.url)
	u.User = nil
	c := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(u)}}
	resp, err := c.Get(far.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusProxyAuthRequired {
		t.Fatalf("status %d, want 407", resp.StatusCode)
	}
}

func TestProxyEnvPointsBothSchemesAtTheProxyAndClearsExemptions(t *testing.T) {
	p, err := StartProxy(NewGate(func() int { return 0 }))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	env := strings.Join(p.Env(), "\n")
	for _, want := range []string{"HTTPS_PROXY=http://bombvault:", "HTTP_PROXY=http://bombvault:", "NO_PROXY=\n", "no_proxy="} {
		if !strings.Contains(env+"\n", want) {
			t.Fatalf("env lacks %q:\n%s", want, env)
		}
	}
	if !strings.Contains(env, "@127.0.0.1:") {
		t.Fatalf("proxy is not on loopback:\n%s", env)
	}
}
