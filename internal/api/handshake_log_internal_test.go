package api

import (
	"log"
	"strings"
	"testing"
	"time"
)

func newTestHandshakeLog() (*log.Logger, *[]string, *time.Time) {
	var lines []string
	clock := time.Date(2026, 10, 13, 15, 51, 5, 0, time.UTC)
	h := &handshakeLog{
		out:  func(line string) { lines = append(lines, line) },
		now:  func() time.Time { return clock },
		seen: map[string]time.Time{},
	}
	return log.New(h, "", 0), &lines, &clock
}

func TestRejectedCertificateIsLoggedOncePerAddressAndHour(t *testing.T) {
	l, lines, clock := newTestHandshakeLog()

	l.Printf("http: TLS handshake error from 192.168.50.119:56451: remote error: tls: unknown certificate")
	l.Printf("http: TLS handshake error from 192.168.50.119:56452: remote error: tls: unknown certificate")
	l.Printf("http: TLS handshake error from 192.168.50.119:56453: remote error: tls: bad certificate")
	l.Printf("http: TLS handshake error from 192.168.50.119:56454: EOF")
	if len(*lines) != 1 || !strings.HasPrefix((*lines)[0], "http: TLS handshake error from 192.168.50.119:56451") {
		t.Fatalf("want the first rejection only, got %q", *lines)
	}

	l.Printf("http: TLS handshake error from [fe80::1]:443: remote error: tls: certificate unknown")
	if len(*lines) != 2 {
		t.Fatalf("another address must get its own line, got %q", *lines)
	}

	*clock = clock.Add(59 * time.Minute)
	l.Printf("http: TLS handshake error from 192.168.50.119:56900: remote error: tls: unknown certificate")
	if len(*lines) != 2 {
		t.Fatalf("a repeat within the hour must stay out, got %q", *lines)
	}

	*clock = clock.Add(time.Minute)
	l.Printf("http: TLS handshake error from 192.168.50.119:57000: remote error: tls: unknown certificate")
	if len(*lines) != 3 {
		t.Fatalf("a rejection an hour later must show again, got %q", *lines)
	}
}

func TestOtherServerErrorsPassUnchanged(t *testing.T) {
	l, lines, _ := newTestHandshakeLog()
	keep := []string{
		"http: TLS handshake error from 192.168.50.119:56451: tls: first record does not look like a TLS handshake",
		"http: TLS handshake error from 192.168.50.119:56452: tls: client offered only unsupported versions: [301]",
		"http: Accept error: accept tcp [::]:8443: too many open files; retrying in 5ms",
		"http: panic serving 192.168.50.119:56453: boom",
	}
	for _, line := range keep {
		l.Print(line)
		l.Print(line)
	}
	if len(*lines) != 2*len(keep) {
		t.Fatalf("every other error must pass each time, got %q", *lines)
	}
	for i, line := range keep {
		if (*lines)[2*i] != line {
			t.Fatalf("line %d changed: got %q, want %q", i, (*lines)[2*i], line)
		}
	}
}
