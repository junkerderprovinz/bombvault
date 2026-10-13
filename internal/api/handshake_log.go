package api

import (
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// handshakeLog is the HTTPS server's ErrorLog. A browser that does not trust
// the self-signed certificate rejects it on every reconnect, and net/http logs
// each rejection. The first one from an address in an hour still reaches the
// log, so a certificate that stopped working stays visible; the repeats do not.
// Every other line passes unchanged.
type handshakeLog struct {
	out  func(string)
	now  func() time.Time
	mu   sync.Mutex
	seen map[string]time.Time
}

const handshakeErrorPrefix = "http: TLS handshake error from "

const handshakeQuietSpan = time.Hour

// certRejections are the handshake failures a client causes by not accepting
// the certificate. A browser that gives up without an alert closes the
// connection, which reads as EOF.
var certRejections = map[string]bool{
	"remote error: tls: unknown certificate": true,
	"remote error: tls: bad certificate":     true,
	"remote error: tls: certificate unknown": true,
	"EOF":                                    true,
}

func newHandshakeLogger() *log.Logger {
	h := &handshakeLog{
		out:  func(line string) { log.Print(line) }, //nolint:gosec // G706: net/http's own server error, the line its default ErrorLog would write
		now:  time.Now,
		seen: map[string]time.Time{},
	}
	return log.New(h, "", 0)
}

func (h *handshakeLog) Write(p []byte) (int, error) {
	line := strings.TrimSuffix(string(p), "\n")
	if host, ok := rejectedCertHost(line); ok {
		if !h.firstInSpan(host) {
			return len(p), nil
		}
		line += " (further rejections from this address are left out for an hour)"
	}
	h.out(line)
	return len(p), nil
}

func rejectedCertHost(line string) (string, bool) {
	rest, ok := strings.CutPrefix(line, handshakeErrorPrefix)
	if !ok {
		return "", false
	}
	addr, reason, ok := strings.Cut(rest, ": ")
	if !ok || !certRejections[reason] {
		return "", false
	}
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host, true
	}
	return addr, true
}

func (h *handshakeLog) firstInSpan(host string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := h.now()
	for k, at := range h.seen {
		if now.Sub(at) >= handshakeQuietSpan {
			delete(h.seen, k)
		}
	}
	if _, ok := h.seen[host]; ok {
		return false
	}
	h.seen[host] = now
	return true
}
