package api

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// handshakeLog is the HTTPS server's ErrorLog. Some clients fail the handshake
// on every reconnect: a browser that does not trust the self-signed
// certificate, or a monitor that polls the port over plain HTTP. net/http logs
// each failure, which buries everything else. The first failure of a kind from
// an address still reaches the log, so a certificate that stopped working
// stays visible. The repeats within the next hour are counted and reported in
// one summary line when the hour is over. Every other line passes unchanged.
type handshakeLog struct {
	out   func(string)
	now   func() time.Time
	after func(time.Duration, func())
	mu    sync.Mutex
	seen  map[handshakeKey]*handshakeSpan
}

type handshakeKey struct{ host, cause string }

type handshakeSpan struct {
	start   time.Time
	dropped int
}

const handshakeErrorPrefix = "http: TLS handshake error from "

const handshakeQuietSpan = time.Hour

// A browser that gives up on the certificate without an alert closes the
// connection, which reads as EOF.
const certRejected = "the client rejected the certificate or closed the connection"

// handshakeCauses maps the failure reasons that repeat for as long as the
// client keeps trying to the cause the summary line names.
var handshakeCauses = map[string]string{
	"remote error: tls: unknown certificate":               certRejected,
	"remote error: tls: bad certificate":                   certRejected,
	"remote error: tls: certificate unknown":               certRejected,
	"EOF":                                                  certRejected,
	"client sent an HTTP request to an HTTPS server":       "the client sent plain HTTP to the HTTPS port",
	"tls: first record does not look like a TLS handshake": "the client does not speak TLS",
}

// Socket errors carry both addresses in the reason, so they match on the end.
var handshakeCauseSuffixes = []struct{ suffix, cause string }{
	{"connection reset by peer", "the client reset the connection"},
	{"i/o timeout", "the client stalled until the handshake timed out"},
}

func newHandshakeLogger() *log.Logger {
	h := &handshakeLog{
		out:   func(line string) { log.Print(line) }, //nolint:gosec // G706: net/http's own server error, the line its default ErrorLog would write
		now:   time.Now,
		after: func(d time.Duration, f func()) { time.AfterFunc(d, f) },
		seen:  map[handshakeKey]*handshakeSpan{},
	}
	return log.New(h, "", 0)
}

func (h *handshakeLog) Write(p []byte) (int, error) {
	line := strings.TrimSuffix(string(p), "\n")
	key, ok := repeatingHandshakeFailure(line)
	if !ok {
		h.out(line)
		return len(p), nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	now := h.now()
	for k, span := range h.seen {
		if now.Sub(span.start) >= handshakeQuietSpan {
			h.close(k, span)
		}
	}
	span, ok := h.seen[key]
	if !ok {
		h.seen[key] = &handshakeSpan{start: now}
		h.out(line + " (repeats from this address are counted for an hour and logged as one line)")
		return len(p), nil
	}
	span.dropped++
	if span.dropped == 1 {
		h.after(span.start.Add(handshakeQuietSpan).Sub(now), func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			if h.seen[key] == span {
				h.close(key, span)
			}
		})
	}
	return len(p), nil
}

// close ends a span and reports what it held back. The caller holds h.mu.
func (h *handshakeLog) close(key handshakeKey, span *handshakeSpan) {
	delete(h.seen, key)
	if span.dropped == 0 {
		return
	}
	h.out(fmt.Sprintf("http: TLS handshake errors from %s: %d more in the hour after %s were left out (%s)",
		key.host, span.dropped, span.start.Format(time.TimeOnly), key.cause))
}

func repeatingHandshakeFailure(line string) (handshakeKey, bool) {
	rest, ok := strings.CutPrefix(line, handshakeErrorPrefix)
	if !ok {
		return handshakeKey{}, false
	}
	addr, reason, ok := strings.Cut(rest, ": ")
	if !ok {
		return handshakeKey{}, false
	}
	cause, ok := handshakeCauses[reason]
	if !ok {
		for _, s := range handshakeCauseSuffixes {
			if strings.HasSuffix(reason, s.suffix) {
				cause, ok = s.cause, true
				break
			}
		}
	}
	if !ok {
		return handshakeKey{}, false
	}
	host := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}
	return handshakeKey{host: host, cause: cause}, true
}
