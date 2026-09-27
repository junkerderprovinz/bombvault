// Package traffic measures how busy containers are and slows BombVault's own
// off-site uploads while a media server streams.
package traffic

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// chunk is the largest piece of an upload that passes the gate at once, so a
// lowered rate takes hold within a fraction of a second.
const chunk = 32 << 10

// Gate is the upload limiter every proxy of the process shares, so two domains
// copying at once stay under one limit together.
type Gate struct {
	rate func() int

	mu  sync.Mutex
	lim *rate.Limiter
	cur int
}

// NewGate returns a gate that asks bytesPerSec before every chunk. Zero or
// less lets the upload through unthrottled.
func NewGate(bytesPerSec func() int) *Gate {
	return &Gate{rate: bytesPerSec}
}

func (g *Gate) wait(ctx context.Context, n int) error {
	r := g.rate()
	if r <= 0 {
		return nil
	}
	g.mu.Lock()
	if g.lim == nil {
		g.lim = rate.NewLimiter(rate.Limit(r), 2*chunk)
	} else if r != g.cur {
		g.lim.SetLimit(rate.Limit(r))
	}
	g.cur = r
	lim := g.lim
	g.mu.Unlock()
	return lim.WaitN(ctx, n)
}

type gatedReader struct {
	ctx  context.Context
	r    io.Reader
	gate *Gate
}

func (t gatedReader) Read(p []byte) (int, error) {
	if len(p) > chunk {
		p = p[:chunk]
	}
	n, err := t.r.Read(p)
	if n > 0 {
		if werr := t.gate.wait(t.ctx, n); werr != nil {
			return n, werr
		}
	}
	return n, err
}

// Proxy is an HTTP proxy on the loopback interface that restic and the rclone
// it starts are pointed at through HTTPS_PROXY. restic reads --limit-upload
// once when it starts; the proxy is what lets the rate of a running copy change.
// Only the upload direction passes the gate.
type Proxy struct {
	ln   net.Listener
	srv  *http.Server
	gate *Gate
	auth string
	url  string
	tr   *http.Transport

	ctx    context.Context
	cancel context.CancelFunc

	mu    sync.Mutex
	conns map[net.Conn]struct{}
}

// StartProxy listens on a random loopback port. Every request has to carry a
// password made for this proxy alone, so nothing else in the container can use
// it as a way out.
func StartProxy(gate *Gate) (*Proxy, error) {
	secret := make([]byte, 16)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("traffic: proxy secret: %w", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("traffic: proxy listen: %w", err)
	}
	pass := hex.EncodeToString(secret)
	ctx, cancel := context.WithCancel(context.Background())
	p := &Proxy{
		ln:   ln,
		gate: gate,
		auth: "Basic " + base64.StdEncoding.EncodeToString([]byte("bombvault:"+pass)),
		url:  "http://bombvault:" + pass + "@" + ln.Addr().String(),
		tr: &http.Transport{
			Proxy:               nil,
			DialContext:         (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression:  true,
			MaxIdleConnsPerHost: 8,
		},
		ctx:    ctx,
		cancel: cancel,
		conns:  map[net.Conn]struct{}{},
	}
	p.srv = &http.Server{Handler: p, ReadHeaderTimeout: 30 * time.Second}
	go func() {
		if err := p.srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			cancel()
		}
	}()
	return p, nil
}

// Env is what a child process needs to send its HTTP traffic through the
// proxy. NO_PROXY is cleared so an exemption set for the container cannot let
// the copy slip past the gate.
func (p *Proxy) Env() []string {
	return []string{
		"HTTPS_PROXY=" + p.url, "HTTP_PROXY=" + p.url,
		"https_proxy=", "http_proxy=", "NO_PROXY=", "no_proxy=",
	}
}

// Close stops the proxy and cuts the tunnels still open through it.
func (p *Proxy) Close() error {
	p.cancel()
	err := p.srv.Close()
	p.mu.Lock()
	for c := range p.conns {
		_ = c.Close()
	}
	p.conns = map[net.Conn]struct{}{}
	p.mu.Unlock()
	p.tr.CloseIdleConnections()
	return err
}

func (p *Proxy) track(cs ...net.Conn) {
	p.mu.Lock()
	for _, c := range cs {
		p.conns[c] = struct{}{}
	}
	p.mu.Unlock()
}

func (p *Proxy) untrack(cs ...net.Conn) {
	p.mu.Lock()
	for _, c := range cs {
		delete(p.conns, c)
	}
	p.mu.Unlock()
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("Proxy-Authorization")), []byte(p.auth)) != 1 {
		w.Header().Set("Proxy-Authenticate", `Basic realm="bombvault"`)
		http.Error(w, "proxy authentication required", http.StatusProxyAuthRequired)
		return
	}
	if r.Method == http.MethodConnect {
		p.tunnel(w, r)
		return
	}
	p.forward(w, r)
}

func (p *Proxy) tunnel(w http.ResponseWriter, r *http.Request) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "tunnel not supported", http.StatusInternalServerError)
		return
	}
	var d net.Dialer
	dctx, cancel := context.WithTimeout(p.ctx, 30*time.Second)
	dst, err := d.DialContext(dctx, "tcp", r.Host)
	cancel()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	src, buf, err := hj.Hijack()
	if err != nil {
		_ = dst.Close()
		return
	}
	p.track(src, dst)
	defer func() {
		p.untrack(src, dst)
		_ = src.Close()
		_ = dst.Close()
	}()
	if _, err := src.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n")); err != nil {
		return
	}
	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(dst, gatedReader{ctx: p.ctx, r: buf.Reader, gate: p.gate})
		closeWrite(dst)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(src, dst)
		closeWrite(src)
		done <- struct{}{}
	}()
	<-done
	<-done
}

func closeWrite(c net.Conn) {
	if tc, ok := c.(interface{ CloseWrite() error }); ok {
		_ = tc.CloseWrite()
		return
	}
	_ = c.Close()
}

// hopHeaders belong to one connection and are not passed on.
var hopHeaders = []string{
	"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate",
	"Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade",
}

func (p *Proxy) forward(w http.ResponseWriter, r *http.Request) {
	if !r.URL.IsAbs() {
		http.Error(w, "absolute URL required", http.StatusBadRequest)
		return
	}
	out := r.Clone(r.Context())
	out.RequestURI = ""
	for _, h := range hopHeaders {
		out.Header.Del(h)
	}
	if r.Body != nil && r.Body != http.NoBody {
		out.Body = struct {
			io.Reader
			io.Closer
		}{gatedReader{ctx: p.ctx, r: r.Body, gate: p.gate}, r.Body}
	}
	resp, err := p.tr.RoundTrip(out)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	for _, h := range hopHeaders {
		resp.Header.Del(h)
	}
	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}
