package api

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A body large enough to clear minCompressSize, and repetitive enough that the
// compressed form is unmistakably smaller.
var longBody = strings.Repeat("BombVault settings payload. ", 200)

func serve(t *testing.T, h http.Handler, acceptEncoding string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if acceptEncoding != "" {
		req.Header.Set("Accept-Encoding", acceptEncoding)
	}
	w := httptest.NewRecorder()
	withCompression(h).ServeHTTP(w, req)
	return w
}

func TestCompressesLargeTextBodies(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, longBody)
	})
	w := serve(t, h, "gzip, deflate")

	if got := w.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if w.Body.Len() >= len(longBody) {
		t.Errorf("compressed body is %d bytes against %d raw - no saving", w.Body.Len(), len(longBody))
	}
	// It has to be READABLE again, which is the half a size check cannot see.
	zr, err := gzip.NewReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("body is not valid gzip: %v", err)
	}
	out, err := io.ReadAll(zr)
	if err != nil || string(out) != longBody {
		t.Errorf("round trip lost the body (err=%v, %d bytes back)", err, len(out))
	}
	// A wrong Content-Length is a truncated response; it must be gone.
	if got := w.Header().Get("Content-Length"); got != "" {
		t.Errorf("Content-Length = %q, want it dropped once the body is encoded", got)
	}
}

func TestNeverCompressesEventStreams(t *testing.T) {
	// The one that matters. gzip buffers, and a buffered event stream is a
	// live update that is no longer live - /api/progress is exactly that.
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, longBody)
	})
	w := serve(t, h, "gzip")

	if got := w.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, an event stream must go out uncompressed", got)
	}
	if w.Body.String() != longBody {
		t.Error("event-stream body was altered")
	}
}

func TestKeepsTheFlusherInterface(t *testing.T) {
	// Losing http.Flusher is how a compression layer silently disables SSE:
	// the handler asks for it, does not find it, and refuses to stream at all.
	var sawFlusher bool
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, sawFlusher = w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: hi\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	})
	serve(t, h, "gzip")
	if !sawFlusher {
		t.Error("the wrapped writer no longer implements http.Flusher")
	}
}

func TestLeavesBodiesAloneWhenNotAsked(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, longBody)
	})
	w := serve(t, h, "")
	if got := w.Header().Get("Content-Encoding"); got != "" {
		t.Errorf("Content-Encoding = %q without Accept-Encoding", got)
	}
	if w.Body.String() != longBody {
		t.Error("body altered for a client that never asked for gzip")
	}
	// Vary is set either way: a cache that stored the plain answer without it
	// would hand those bytes to a client that did ask.
	if !strings.Contains(w.Header().Get("Vary"), "Accept-Encoding") {
		t.Error("Vary: Accept-Encoding missing on an uncompressed response")
	}
}

func TestSkipsBinaryAndAlreadyEncoded(t *testing.T) {
	for _, tc := range []struct{ name, ctype, encoding string }{
		{"png", "image/png", ""},
		{"already encoded", "application/json", "br"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tc.ctype)
				if tc.encoding != "" {
					w.Header().Set("Content-Encoding", tc.encoding)
				}
				_, _ = io.WriteString(w, longBody)
			})
			w := serve(t, h, "gzip")
			if got := w.Header().Get("Content-Encoding"); got != tc.encoding {
				t.Errorf("Content-Encoding = %q, want %q", got, tc.encoding)
			}
		})
	}
}

func TestSkipsShortBodies(t *testing.T) {
	// Below the threshold gzip's own header costs more than it saves.
	const short = `{"ok":true}`
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "11")
		_, _ = io.WriteString(w, short)
	})
	w := serve(t, h, "gzip")
	if got := w.Header().Get("Content-Encoding"); got != "" {
		t.Errorf("Content-Encoding = %q on an 11-byte body", got)
	}
}
