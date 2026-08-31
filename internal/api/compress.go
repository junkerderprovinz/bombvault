package api

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// compressible answers whether a response body is worth gzipping, from its
// Content-Type.
//
// Deciding by type rather than by path is what keeps this correct as the app
// grows: a new endpoint gets the right treatment by saying what it returns,
// which it has to do anyway.
//
// text/event-stream is the one that MUST stay out, and not for efficiency:
// gzip buffers, and an event stream that arrives in buffered chunks is a live
// update that is no longer live. /api/progress is exactly that endpoint.
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

// Below this many bytes gzip costs more than it saves: the header alone is 18
// bytes, and a short JSON answer often comes out LARGER compressed.
const minCompressSize = 1024

var gzipPool = sync.Pool{
	New: func() any {
		// BestSpeed, not BestCompression. Measured on this app's own bundle,
		// the difference between the two is a few percent of size and a large
		// multiple of CPU, and this runs on a NAS that is also busy running
		// backups.
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		return w
	},
}

// gzipResponseWriter defers the decision until the handler has actually set a
// Content-Type, because that is the first moment the answer is knowable.
//
// It implements http.Flusher unconditionally. Dropping that interface is how a
// compression middleware silently breaks server-sent events: the SSE handler
// asks `w.(http.Flusher)` and, finding nothing, refuses to stream at all -
// which looks like a broken feature rather than a broken wrapper.
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
	// Never double-encode something a handler already compressed itself.
	if h.Get("Content-Encoding") != "" || !compressible(h.Get("Content-Type")) {
		return
	}
	// A known, small body is not worth it; an unknown length is, because the
	// bodies that matter here are the ones too large to have been measured.
	if n := h.Get("Content-Length"); n != "" && len(n) <= 4 {
		if size := atoiSafe(n); size > 0 && size < minCompressSize {
			return
		}
	}
	g.compress = true
	h.Set("Content-Encoding", "gzip")
	// The length of the ORIGINAL body is wrong once it is compressed, and a
	// wrong Content-Length is a truncated response.
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

// Flush passes through in both directions. For a compressed body it has to
// flush the gzip writer FIRST, or the bytes sit in its window and the client
// waits for data the server believes it already sent.
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

// withCompression gzips responses for clients that asked for it ([364]).
//
// The app shipped 5,172 kB of JavaScript and 473 kB of CSS uncompressed, and
// asking with `Accept-Encoding: gzip` returned exactly the same bytes: nothing
// in the chain compressed anything. `gzip -9` on that same bundle produces
// 1,409 kB, a factor of 3.7.
//
// On a LAN that transfer takes 0.13s and nobody notices. It is the remote case
// this is for - over a VPN, or from a phone - which is how this appliance is
// actually reached from outside the house.
//
// Vary is set on every response, not only compressed ones: a cache that stored
// the uncompressed answer without it would serve those bytes to a client that
// asked for gzip, and vice versa.
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
