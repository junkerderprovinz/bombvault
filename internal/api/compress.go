package api

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// compressible reports whether a body of the given Content-Type is worth
// gzipping. text/event-stream stays out because gzip buffers, and a buffered
// event stream such as /api/progress stops being live.
func compressible(contentType string) bool {
	ct := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	switch ct {
	case "text/event-stream":
		return false
	case "text/html", "text/css", "text/plain", "text/markdown",
		"application/json", "application/javascript", "text/javascript",
		"image/svg+xml", "application/manifest+json":
		return true
	}
	return false
}

// minCompressSize is the smallest known length worth compressing. Below it the
// 18-byte gzip header often makes a short answer larger.
const minCompressSize = 1024

var gzipPool = sync.Pool{
	New: func() any {
		// On the app bundle BestCompression saves a few percent more at several
		// times the CPU, on a NAS that is busy running backups.
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		return w
	},
}

// gzipResponseWriter decides whether to compress once the handler has set a
// Content-Type. It always implements http.Flusher, since an SSE handler that
// cannot find one refuses to stream.
type gzipResponseWriter struct {
	http.ResponseWriter
	gz       *gzip.Writer
	decided  bool
	compress bool
	status   int
}

func (g *gzipResponseWriter) WriteHeader(status int) {
	g.status = status
	g.decide()
	g.ResponseWriter.WriteHeader(status)
}

func (g *gzipResponseWriter) decide() {
	if g.decided {
		return
	}
	g.decided = true
	h := g.Header()
	if h.Get("Content-Encoding") != "" || !compressible(h.Get("Content-Type")) {
		return
	}
	// An unknown length is compressed: the bodies worth it are the ones too
	// large to have been measured up front.
	if n := h.Get("Content-Length"); n != "" && len(n) <= 4 {
		if size := atoiSafe(n); size > 0 && size < minCompressSize {
			return
		}
	}
	g.compress = true
	h.Set("Content-Encoding", "gzip")
	h.Del("Content-Length")
	gz := gzipPool.Get().(*gzip.Writer)
	gz.Reset(g.ResponseWriter)
	g.gz = gz
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	g.decide()
	if g.compress {
		return g.gz.Write(b)
	}
	return g.ResponseWriter.Write(b)
}

// Flush flushes the gzip writer before the underlying one, or the compressed
// bytes stay in gzip's buffer.
func (g *gzipResponseWriter) Flush() {
	if g.compress && g.gz != nil {
		_ = g.gz.Flush()
	}
	if f, ok := g.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (g *gzipResponseWriter) close() {
	if g.compress && g.gz != nil {
		_ = g.gz.Close()
		gzipPool.Put(g.gz)
		g.gz = nil
	}
}

// atoiSafe parses a non-negative decimal and returns -1 for anything else.
func atoiSafe(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// withCompression gzips responses for clients that accept it. It matters for
// remote access over a VPN or from a phone: the JavaScript bundle is about 5 MB
// raw and 1.4 MB compressed.
//
// Vary is set on every response, compressed or not, so a cache never hands the
// plain bytes to a client that asked for gzip, or the reverse.
func withCompression(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept-Encoding")
		if !strings.Contains(strings.ToLower(r.Header.Get("Accept-Encoding")), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		gw := &gzipResponseWriter{ResponseWriter: w}
		defer gw.close()
		next.ServeHTTP(gw, r)
	})
}
