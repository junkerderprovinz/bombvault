package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// doLogin posts a password to handleLogin from remoteAddr and returns the
// status code and the decoded envelope.
func doLogin(t *testing.T, h *Handler, remoteAddr, password string) (code int, ok bool, errMsg string) {
	t.Helper()
	body, err := json.Marshal(map[string]string{"password": password})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	r := jsonReq(http.MethodPost, "/api/login", bytes.NewReader(body))
	r.RemoteAddr = remoteAddr
	w := httptest.NewRecorder()
	h.handleLogin(w, r)

	var resp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if w.Code == http.StatusOK || w.Code == http.StatusTooManyRequests {
		if decErr := json.NewDecoder(w.Result().Body).Decode(&resp); decErr != nil && w.Code == http.StatusOK {
			t.Fatalf("decode response: %v", decErr)
		}
	}
	return w.Code, resp.OK, resp.Error
}

// An attacker hammering bad passwords from one address must not lock out
// another address that has the right password.
func TestLoginThrottleIsPerIP(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	enableAuth(t, h, repo) // password is "hunter2"

	attacker := "203.0.113.9:51000"
	admin := "198.51.100.7:52000"

	for i := 0; i < loginMaxFails; i++ {
		code, ok, _ := doLogin(t, h, attacker, "wrong-guess")
		if code != http.StatusOK || ok {
			t.Fatalf("attacker fail #%d: want 200/ok=false (not yet throttled), got code=%d ok=%v", i, code, ok)
		}
	}

	code, ok, errMsg := doLogin(t, h, attacker, "hunter2")
	if code != http.StatusTooManyRequests || ok {
		t.Fatalf("attacker after %d fails: want 429, got code=%d ok=%v err=%q", loginMaxFails, code, ok, errMsg)
	}

	code, ok, errMsg = doLogin(t, h, admin, "hunter2")
	if code != http.StatusOK || !ok {
		t.Fatalf("admin from different IP with correct password: want 200/ok=true, got code=%d ok=%v err=%q", code, ok, errMsg)
	}
}

// A successful login clears the failures of its own address only.
func TestLoginThrottleClearsOnlyThatIPOnSuccess(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	enableAuth(t, h, repo)

	attacker := "203.0.113.9:51000"
	other := "198.51.100.7:52000"

	for i := 0; i < loginMaxFails; i++ {
		if code, ok, _ := doLogin(t, h, attacker, "wrong-guess"); code != http.StatusOK || ok {
			t.Fatalf("attacker fail #%d: got code=%d ok=%v", i, code, ok)
		}
	}

	if code, ok, errMsg := doLogin(t, h, other, "hunter2"); code != http.StatusOK || !ok {
		t.Fatalf("other IP correct password: want 200/ok=true, got code=%d ok=%v err=%q", code, ok, errMsg)
	}

	code, ok, errMsg := doLogin(t, h, attacker, "hunter2")
	if code != http.StatusTooManyRequests || ok {
		t.Fatalf("attacker still throttled after unrelated IP's success: want 429, got code=%d ok=%v err=%q", code, ok, errMsg)
	}
}

// Failures older than loginWindow stop throttling, and the emptied entry is
// deleted.
func TestLoginThrottleRecoversAfterWindow(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	enableAuth(t, h, repo)

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

// A throttled attacker is stopped before recordLoginFail, so its timestamps
// stop advancing and it soon looks least recently touched. A flood of one-off
// keys that pushes the map past loginMaxTracked must not evict it. The test
// goes through handleLogin because the order of the throttle check and the
// flood is what matters.
func TestLoginThrottleEvictionNeverUnthrottlesAnActiveAttacker(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	// The fast legacy hash keeps thousands of logins inside loginWindow;
	// Argon2id at 19 MiB per attempt would not. The password is "hunter2".
	enableAuthLegacyHash(t, h, repo)

	attacker := "203.0.113.9:51000"

	for i := 0; i < loginMaxFails; i++ {
		code, ok, _ := doLogin(t, h, attacker, "wrong-guess")
		if code != http.StatusOK || ok {
			t.Fatalf("attacker fail #%d: want 200/ok=false (not yet throttled), got code=%d ok=%v", i, code, ok)
		}
	}
	if code, ok, _ := doLogin(t, h, attacker, "hunter2"); code != http.StatusTooManyRequests || ok {
		t.Fatalf("attacker after %d fails: want 429, got code=%d ok=%v", loginMaxFails, code, ok)
	}

	// Every flood entry is newer than the attacker's last failure. Eviction
	// runs on every call once the map is over the cap, so a small margin past
	// loginMaxTracked is enough.
	const flood = loginMaxTracked + 200
	for i := 0; i < flood; i++ {
		// loginClientKey ignores the port, so each entry needs its own host.
		addr := fmt.Sprintf("10.0.%d.%d:1234", i/256, i%256)
		if code, ok, _ := doLogin(t, h, addr, "wrong-guess"); code != http.StatusOK || ok {
			t.Fatalf("flood entry #%d: want 200/ok=false (a brand-new key must never be pre-throttled), got code=%d ok=%v", i, code, ok)
		}
	}

	// TestLoginThrottleCapsMapSize checks the exact cap. Some slack is expected
	// here: the throttled attacker is exempt from eviction, and the last flood
	// request adds its entry after that call's eviction ran.
	h.loginMu.Lock()
	mapSize := len(h.loginFails)
	h.loginMu.Unlock()
	if mapSize > loginMaxTracked+50 {
		t.Fatalf("loginFails held %d entries after a %d-entry flood, want roughly <= loginMaxTracked=%d; the hard cap did not evict", mapSize, flood, loginMaxTracked)
	}

	code, ok, errMsg := doLogin(t, h, attacker, "hunter2")
	if code != http.StatusTooManyRequests || ok {
		t.Fatalf("attacker still throttled after the flood triggered eviction: want 429, got code=%d ok=%v err=%q; eviction un-throttled an active attacker", code, ok, errMsg)
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
