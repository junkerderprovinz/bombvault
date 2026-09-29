package relay

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// The group key's 128 bits keep strangers out. This backoff only limits
// clients that fail the handshake over and over, each attempt costing a TLS
// negotiation and a goroutine for up to helloTimeout.
const (
	// failWindow is how long a failure stays on an address's record.
	failWindow = 10 * time.Minute
	// failsBeforeBlock is how many failures inside failWindow start a block.
	// Somebody mistyping a relay address fails once or twice, not ten times.
	failsBeforeBlock = 10
	// baseBlock doubles with each further failure up to maxBlock.
	baseBlock = time.Minute
	maxBlock  = 50 * time.Minute
	// maxTrackedAddrs bounds the limiter's own memory.
	maxTrackedAddrs = 4096
	// sweepEvery is how often the request path may walk the whole map.
	sweepEvery = time.Minute
)

type attempts struct {
	fails        int
	last         time.Time
	blockedUntil time.Time
	blockFor     time.Duration
}

// limiter tracks failed handshakes per client address. now is a field so
// tests can drive the clock.
type limiter struct {
	mu        sync.Mutex
	addrs     map[string]*attempts
	now       func() time.Time
	lastSweep time.Time
}

func newLimiter() *limiter {
	return &limiter{addrs: map[string]*attempts{}, now: time.Now}
}

func (l *limiter) blocked(addr string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.maybeSweepLocked(now)
	a := l.addrs[addr]
	return a != nil && now.Before(a.blockedUntil)
}

func (l *limiter) maybeSweepLocked(now time.Time) {
	if now.Sub(l.lastSweep) < sweepEvery {
		return
	}
	l.lastSweep = now
	for k, a := range l.addrs {
		if now.Sub(lastActive(a)) > failWindow {
			delete(l.addrs, k)
		}
	}
}

// fail records one failed handshake and extends the block once the address
// has earned one. The gap is measured from the end of a block, so waiting out
// a long block does not reset the backoff.
func (l *limiter) fail(addr string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.maybeSweepLocked(now)

	a := l.addrs[addr]
	if a == nil {
		l.evictIfFullLocked()
		a = &attempts{}
		l.addrs[addr] = a
	}
	if !a.last.IsZero() && now.Sub(lastActive(a)) > failWindow {
		a.fails = 0
		a.blockFor = 0
	}
	a.fails++
	a.last = now
	if a.fails < failsBeforeBlock {
		return
	}
	switch {
	case a.blockFor == 0:
		a.blockFor = baseBlock
	case a.blockFor < maxBlock:
		a.blockFor = min(a.blockFor*2, maxBlock)
	}
	a.blockedUntil = now.Add(a.blockFor)
}

// succeed clears an address's record: a working handshake shows a real
// client, and its earlier failures should not count against it.
func (l *limiter) succeed(addr string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.addrs, addr)
}

func lastActive(a *attempts) time.Time {
	if a.blockedUntil.After(a.last) {
		return a.blockedUntil
	}
	return a.last
}

// evictIfFullLocked drops the least recently active record, so an address
// whose block is still running goes last.
func (l *limiter) evictIfFullLocked() {
	if len(l.addrs) < maxTrackedAddrs {
		return
	}
	var oldestKey string
	var oldest time.Time
	for k, a := range l.addrs {
		if t := lastActive(a); oldestKey == "" || t.Before(oldest) {
			oldestKey, oldest = k, t
		}
	}
	delete(l.addrs, oldestKey)
}

func defaultClientAddr(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) clientAddrOf(r *http.Request) string {
	if s.ClientAddr != nil {
		return s.ClientAddr(r)
	}
	return defaultClientAddr(r)
}
