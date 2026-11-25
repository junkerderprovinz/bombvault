package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// Failures older than loginWindow stop throttling, and the emptied entry is
// deleted.
func TestLoginThrottleRecoversAfterWindow(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	_ = repo

	addr := "203.0.113.9:51000"
	key := loginClientKey(&http.Request{RemoteAddr: addr})

	h.loginMu.Lock()
	if h.loginFails == nil {
		h.loginFails = make(map[string][]time.Time)
	}
	for i := 0; i < loginMaxFails; i++ {
		h.loginFails[key] = append(h.loginFails[key], time.Now().Add(-2*loginWindow))
	}
	h.loginMu.Unlock()

	if h.loginThrottled(key) {
		t.Fatalf("stale failures outside loginWindow must not throttle")
	}
	h.loginMu.Lock()
	_, stillPresent := h.loginFails[key]
	h.loginMu.Unlock()
	if stillPresent {
		t.Fatalf("loginThrottled must delete the map entry once its window empties, to bound memory growth")
	}
}

// Keys that fail once and are never queried again, as from a botnet or an
// attacker rotating through an IPv6 /64, are swept once loginThrottled has
// been called loginSweepEvery times for some other key.
func TestLoginThrottleSweepsStaleOneOffKeys(t *testing.T) {
	h := &Handler{}
	stale := time.Now().Add(-2 * loginWindow)
	h.loginMu.Lock()
	h.loginFails = make(map[string][]time.Time)
	const oneOffKeys = loginSweepEvery + 10
	for i := 0; i < oneOffKeys; i++ {
		h.loginFails[fmt.Sprintf("one-off-%d", i)] = []time.Time{stale}
	}
	h.loginMu.Unlock()

	for i := 0; i < loginSweepEvery; i++ {
		h.loginThrottled("driver")
	}

	h.loginMu.Lock()
	remaining := len(h.loginFails)
	h.loginMu.Unlock()
	// "driver" has no failures, so its own prune removes it; anything left is a
	// one-off key the sweep missed.
	if remaining != 0 {
		t.Fatalf("loginFails still holds %d stale one-off entries after %d loginThrottled calls (>= loginSweepEvery=%d); the periodic sweep did not run", remaining, loginSweepEvery, loginSweepEvery)
	}
}

// Keys with failures inside the window cannot be pruned, so a single
// loginThrottled call on an over-full map has to evict the least recently
// touched ones down to loginMaxTracked.
func TestLoginThrottleCapsMapSize(t *testing.T) {
	h := &Handler{}
	now := time.Now()
	h.loginMu.Lock()
	h.loginFails = make(map[string][]time.Time)
	const extra = 50
	for i := 0; i < loginMaxTracked+extra; i++ {
		h.loginFails[fmt.Sprintf("flood-%d", i)] = []time.Time{now.Add(time.Duration(i) * time.Millisecond)}
	}
	h.loginMu.Unlock()

	h.loginThrottled("flood-0")

	h.loginMu.Lock()
	size := len(h.loginFails)
	h.loginMu.Unlock()
	if size > loginMaxTracked {
		t.Fatalf("loginFails held %d entries after exceeding loginMaxTracked=%d, want <= %d; the hard cap did not evict", size, loginMaxTracked, loginMaxTracked)
	}
}
