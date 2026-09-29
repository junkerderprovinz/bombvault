package relay

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

// testLimiter returns a limiter whose clock the test drives, so a backoff
// can be waited out in nanoseconds instead of minutes.
func testLimiter() (*Limiter, func(time.Duration)) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	l := NewLimiter()
	l.now = func() time.Time { return now }
	return l, func(d time.Duration) { now = now.Add(d) }
}

// A person who mistypes a phrase and corrects it is never blocked.
func TestLimiterLetsOccasionalFailuresThrough(t *testing.T) {
	l, _ := testLimiter()
	for i := 0; i < failsBeforeBlock-1; i++ {
		l.Fail("198.51.100.7")
		if l.Blocked("198.51.100.7") {
			t.Fatalf("blocked after only %d failures, threshold is %d", i+1, failsBeforeBlock)
		}
	}
}

func TestLimiterBlocksAfterThreshold(t *testing.T) {
	l, _ := testLimiter()
	for i := 0; i < failsBeforeBlock; i++ {
		l.Fail("198.51.100.7")
	}
	if !l.Blocked("198.51.100.7") {
		t.Fatalf("not blocked after %d failures", failsBeforeBlock)
	}
}

func TestLimiterIsPerAddress(t *testing.T) {
	l, _ := testLimiter()
	for i := 0; i < failsBeforeBlock*3; i++ {
		l.Fail("198.51.100.7")
	}
	if !l.Blocked("198.51.100.7") {
		t.Fatal("the offending address is not blocked")
	}
	if l.Blocked("203.0.113.9") {
		t.Fatal("an unrelated address was blocked")
	}
}

func TestLimiterBlockExpires(t *testing.T) {
	l, advance := testLimiter()
	for i := 0; i < failsBeforeBlock; i++ {
		l.Fail("198.51.100.7")
	}
	if !l.Blocked("198.51.100.7") {
		t.Fatal("not blocked")
	}
	advance(baseBlock + time.Second)
	if l.Blocked("198.51.100.7") {
		t.Fatal("still blocked after the first backoff elapsed")
	}
}

// Each further failure over the threshold costs more than the last, or a
// caller could wait out a fixed penalty forever.
func TestLimiterBackoffGrows(t *testing.T) {
	l, advance := testLimiter()
	for i := 0; i < failsBeforeBlock; i++ {
		l.Fail("198.51.100.7")
	}
	first := l.addrs["198.51.100.7"].blockFor

	advance(first + time.Second)
	l.Fail("198.51.100.7")
	second := l.addrs["198.51.100.7"].blockFor

	if second <= first {
		t.Fatalf("backoff did not grow: %s then %s", first, second)
	}
}

func TestLimiterBackoffIsCapped(t *testing.T) {
	l, advance := testLimiter()
	for i := 0; i < failsBeforeBlock+40; i++ {
		l.Fail("198.51.100.7")
		advance(time.Second)
	}
	if got := l.addrs["198.51.100.7"].blockFor; got > maxBlock {
		t.Fatalf("backoff grew to %s, cap is %s", got, maxBlock)
	}
}

// Otherwise an instance that reconnects a few times during a key change ends
// up locked out afterwards.
func TestLimiterSuccessClearsTheRecord(t *testing.T) {
	l, _ := testLimiter()
	for i := 0; i < failsBeforeBlock-1; i++ {
		l.Fail("198.51.100.7")
	}
	l.Succeed("198.51.100.7")

	for i := 0; i < failsBeforeBlock-1; i++ {
		l.Fail("198.51.100.7")
		if l.Blocked("198.51.100.7") {
			t.Fatal("old failures still counted after a successful handshake")
		}
	}
}

// Failures spread far apart do not accumulate into a block.
func TestLimiterFailuresAgeOut(t *testing.T) {
	l, advance := testLimiter()
	for i := 0; i < failsBeforeBlock*2; i++ {
		l.Fail("198.51.100.7")
		advance(failWindow + time.Minute)
		if l.Blocked("198.51.100.7") {
			t.Fatalf("blocked by failures spaced more than %s apart", failWindow)
		}
	}
}

func TestLimiterBoundsItsOwnMemory(t *testing.T) {
	l, advance := testLimiter()
	for i := 0; i < maxTrackedAddrs+500; i++ {
		l.Fail(strings.Repeat("a", 3) + string(rune('0'+i%10)) + "." + string(rune('a'+i%26)) + strconv.Itoa(i))
		advance(time.Millisecond)
	}
	if got := len(l.addrs); got > maxTrackedAddrs {
		t.Fatalf("limiter tracks %d addresses, cap is %d", got, maxTrackedAddrs)
	}
}

// Buckets are per IP, not per connection: every connection gets a fresh
// source port, so counting the pair would never block anyone.
func TestRequestAddrDropsThePort(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"198.51.100.7:54321", "198.51.100.7"},
		{"198.51.100.7:1", "198.51.100.7"},
		{"[2a01:4f8:c014:3544::1]:44444", "2a01:4f8:c014:3544::1"},
		{"no-port-here", "no-port-here"},
	} {
		got := RequestAddr(&http.Request{RemoteAddr: c.in})
		if got != c.want {
			t.Errorf("RequestAddr(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// An address that fails once and never comes back, the usual scanner, is
// forgotten once its failure has aged out. blocked runs the same sweep the
// request path does, since there is no exported sweep to call directly.
func TestLimiterForgetsAnAddressThatNeverReturns(t *testing.T) {
	l, advance := testLimiter()
	l.Fail("198.51.100.7")
	advance(failWindow + time.Second)
	l.Blocked("198.51.100.7")
	if n := len(l.addrs); n != 0 {
		t.Fatalf("%d records left after the failure aged out, want 0", n)
	}
}

func TestLimiterKeepsABlockedAddressUntilTheBlockEnds(t *testing.T) {
	l, advance := testLimiter()
	for i := 0; i < failsBeforeBlock+16; i++ {
		l.Fail("198.51.100.7")
	}
	advance(failWindow + time.Minute)
	if !l.Blocked("198.51.100.7") {
		t.Fatal("the sweep lifted a block that had not run out")
	}
}

// policyRetention is the worst case for how long a failed address is kept:
// the longest possible block, plus the failure window it must age past,
// plus one sweep interval to actually delete the record.
const policyRetention = 61 * time.Minute

// Whatever the history, a record has run out policyRetention minus sweepEvery
// after the last failure, so the next sweep deletes it within that bound.
func TestLimiterRetentionIsBoundedAfterTheLastFailure(t *testing.T) {
	for _, fails := range []int{1, failsBeforeBlock - 1, failsBeforeBlock, failsBeforeBlock + 3, failsBeforeBlock + 40} {
		l, advance := testLimiter()
		for i := 0; i < fails; i++ {
			l.Fail("198.51.100.7")
		}
		advance(policyRetention - sweepEvery + time.Second)
		l.Blocked("198.51.100.7")
		if n := len(l.addrs); n != 0 {
			t.Fatalf("after %d failures, %d records left %s past the last failure, want 0", fails, n, policyRetention-sweepEvery)
		}
	}
}

// A single caller never fails while blocked, because ServeHTTP answers 429
// before the handshake, so fail only runs once a block is over. The backoff
// still has to grow on that path, and the throttled sweep that blocked and
// fail both run must not wipe the record in between.
func TestLimiterBackoffGrowsForACallerThatWaitsOutEachBlock(t *testing.T) {
	for _, every := range []time.Duration{time.Second, 37 * time.Second} {
		l, advance := testLimiter()
		var longest time.Duration
		for elapsed := time.Duration(0); elapsed < 6*time.Hour; elapsed += every {
			if !l.Blocked("198.51.100.7") {
				l.Fail("198.51.100.7")
			}
			a := l.addrs["198.51.100.7"]
			if a == nil || a.blockFor < longest {
				t.Fatalf("a caller retrying every %s had its backoff reset at %s, after reaching %s", every, elapsed, longest)
			}
			longest = a.blockFor
			advance(every)
		}
		if longest != maxBlock {
			t.Fatalf("a caller retrying every %s reached a longest block of %s, want %s", every, longest, maxBlock)
		}
	}
}

// The request path sweeps on its own, at most once a minute, so a relay that
// only ever sees traffic still forgets without a separate timer.
func TestLimiterSweepsOnTheRequestPath(t *testing.T) {
	l, advance := testLimiter()
	l.Fail("198.51.100.7")
	advance(policyRetention)
	l.Blocked("203.0.113.9")
	if n := len(l.addrs); n != 0 {
		t.Fatalf("%d records left after a request came in past the retention bound, want 0", n)
	}
}
