package api

import (
	"log"
	"strings"
	"testing"
	"time"
)

type testHandshakeLog struct {
	*log.Logger
	lines  []string
	clock  time.Time
	timers []func()
	delays []time.Duration
}

func newTestHandshakeLog() *testHandshakeLog {
	tl := &testHandshakeLog{clock: time.Date(2026, 10, 13, 15, 51, 5, 0, time.UTC)}
	h := &handshakeLog{
		out: func(line string) { tl.lines = append(tl.lines, line) },
		now: func() time.Time { return tl.clock },
		after: func(d time.Duration, f func()) {
			tl.delays = append(tl.delays, d)
			tl.timers = append(tl.timers, f)
		},
		seen: map[handshakeKey]*handshakeSpan{},
	}
	tl.Logger = log.New(h, "", 0)
	return tl
}

func TestRejectedCertificateIsLoggedOncePerAddressAndHour(t *testing.T) {
	l := newTestHandshakeLog()

	l.Printf("http: TLS handshake error from 192.168.50.119:56451: remote error: tls: unknown certificate")
	l.Printf("http: TLS handshake error from 192.168.50.119:56452: remote error: tls: unknown certificate")
	l.Printf("http: TLS handshake error from 192.168.50.119:56453: remote error: tls: bad certificate")
	l.Printf("http: TLS handshake error from 192.168.50.119:56454: EOF")
	if len(l.lines) != 1 || !strings.HasPrefix(l.lines[0], "http: TLS handshake error from 192.168.50.119:56451") {
		t.Fatalf("want the first rejection only, got %q", l.lines)
	}

	l.Printf("http: TLS handshake error from [fe80::1]:443: remote error: tls: certificate unknown")
	if len(l.lines) != 2 {
		t.Fatalf("another address must get its own line, got %q", l.lines)
	}

	l.clock = l.clock.Add(59 * time.Minute)
	l.Printf("http: TLS handshake error from 192.168.50.119:56900: remote error: tls: unknown certificate")
	if len(l.lines) != 2 {
		t.Fatalf("a repeat within the hour must stay out, got %q", l.lines)
	}

	l.clock = l.clock.Add(time.Minute)
	l.Printf("http: TLS handshake error from 192.168.50.119:57000: remote error: tls: unknown certificate")
	if len(l.lines) != 4 {
		t.Fatalf("an hour later the held-back count and the new rejection must show, got %q", l.lines)
	}
	if !strings.Contains(l.lines[2], "192.168.50.119: 4 more") || !strings.HasPrefix(l.lines[3], "http: TLS handshake error from 192.168.50.119:57000") {
		t.Fatalf("want the summary before the new rejection, got %q", l.lines[2:])
	}
}

func TestPlainHTTPToTheHTTPSPortIsSummedUpPerAddress(t *testing.T) {
	l := newTestHandshakeLog()
	for i := range 1334 {
		l.Printf("http: TLS handshake error from 192.168.20.87:%d: client sent an HTTP request to an HTTPS server", 40000+i)
	}
	if len(l.lines) != 1 {
		t.Fatalf("want one line for the whole run, got %d", len(l.lines))
	}
	if len(l.timers) != 1 || l.delays[0] != time.Hour {
		t.Fatalf("want one summary scheduled an hour after the first line, got %v", l.delays)
	}

	l.timers[0]()
	want := "http: TLS handshake errors from 192.168.20.87: 1333 more in the hour after 15:51:05 were left out (the client sent plain HTTP to the HTTPS port)"
	if len(l.lines) != 2 || l.lines[1] != want {
		t.Fatalf("summary = %q, want %q", l.lines[1:], want)
	}

	l.Printf("http: TLS handshake error from 192.168.20.87:50000: client sent an HTTP request to an HTTPS server")
	if len(l.lines) != 3 {
		t.Fatalf("after the summary the next failure must start a new hour, got %q", l.lines)
	}
}

func TestSocketErrorsDuringTheHandshakeAreSummedUp(t *testing.T) {
	l := newTestHandshakeLog()
	for i := range 3 {
		l.Printf("http: TLS handshake error from 192.168.20.87:%d: read tcp 172.17.0.2:3443->192.168.20.87:%d: read: connection reset by peer", 51000+i, 51000+i)
		l.Printf("http: TLS handshake error from 192.168.20.87:%d: read tcp 172.17.0.2:3443->192.168.20.87:%d: i/o timeout", 52000+i, 52000+i)
	}
	if len(l.lines) != 2 {
		t.Fatalf("want one line per cause, got %q", l.lines)
	}
	for _, f := range l.timers {
		f()
	}
	got := strings.Join(l.lines[2:], "\n")
	for _, want := range []string{"2 more in the hour after 15:51:05 were left out (the client reset the connection)", "(the client stalled until the handshake timed out)"} {
		if !strings.Contains(got, want) {
			t.Fatalf("summaries %q miss %q", got, want)
		}
	}
}

func TestALateSummaryTimerLeavesTheNextHourAlone(t *testing.T) {
	l := newTestHandshakeLog()
	line := "http: TLS handshake error from 10.0.0.5:%d: EOF"
	l.Printf(line, 1)
	l.Printf(line, 2)
	l.clock = l.clock.Add(time.Hour)
	l.Printf(line, 3)
	l.Printf(line, 4)
	if len(l.lines) != 3 {
		t.Fatalf("want first line, summary, first line of the next hour, got %q", l.lines)
	}

	l.timers[0]()
	if len(l.lines) != 3 {
		t.Fatalf("the first hour's timer must not close the second hour, got %q", l.lines)
	}
	l.timers[1]()
	if len(l.lines) != 4 || !strings.Contains(l.lines[3], "1 more in the hour after 16:51:05") {
		t.Fatalf("want the second hour's summary, got %q", l.lines)
	}
}

func TestASingleFailureGetsNoSummary(t *testing.T) {
	l := newTestHandshakeLog()
	l.Printf("http: TLS handshake error from 10.0.0.5:1: EOF")
	l.clock = l.clock.Add(2 * time.Hour)
	l.Printf("http: TLS handshake error from 10.0.0.6:1: EOF")
	if len(l.lines) != 2 || len(l.timers) != 0 {
		t.Fatalf("want two plain lines and no summary, got %q", l.lines)
	}
}

func TestOtherServerErrorsPassUnchanged(t *testing.T) {
	l := newTestHandshakeLog()
	keep := []string{
		"http: TLS handshake error from 192.168.50.119:56452: tls: client offered only unsupported versions: [301]",
		"http: TLS handshake error from 192.168.50.119:56453: tls: no cipher suite supported by both client and server",
		"http: Accept error: accept tcp [::]:8443: too many open files; retrying in 5ms",
		"http: panic serving 192.168.50.119:56453: boom",
	}
	for _, line := range keep {
		l.Print(line)
		l.Print(line)
	}
	if len(l.lines) != 2*len(keep) {
		t.Fatalf("every other error must pass each time, got %q", l.lines)
	}
	for i, line := range keep {
		if l.lines[2*i] != line {
			t.Fatalf("line %d changed: got %q, want %q", i, l.lines[2*i], line)
		}
	}
}
