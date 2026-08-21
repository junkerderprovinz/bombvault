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

// doLogin POSTs {password} to handleLogin from the given RemoteAddr and
// returns the response code plus the decoded {ok, error} envelope.
func doLogin(t *testing.T, h *Handler, remoteAddr, password string) (code int, ok bool, errMsg string) {
	t.Helper()
	body, err := json.Marshal(map[string]string{"password": password})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	r := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body))
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

// TestLoginThrottleIsPerIP is the fix for the finding that loginFails was a
// single counter shared by every caller: an unauthenticated attacker hammering
// bad passwords from one address must not be able to lock out a DIFFERENT
// address that then supplies the correct password.
func TestLoginThrottleIsPerIP(t *testing.T) {
	h, repo, _ := newAuthGateHandler(t)
	enableAuth(t, h, repo) // password is "hunter2"

	attacker := "203.0.113.9:51000"
	admin := "198.51.100.7:52000"

	// The attacker exhausts the throttle for their own address with wrong
	// passwords — this alone must not touch anyone else's bucket.
	for i := 0; i < loginMaxFails; i++ {
		code, ok, _ := doLogin(t, h, attacker, "wrong-guess")
		if code != http.StatusOK || ok {
			t.Fatalf("attacker fail #%d: want 200/ok=false (not yet throttled), got code=%d ok=%v", i, code, ok)
		}
	}

	// The attacker's OWN next attempt is now throttled, correct password or not.
	code, ok, errMsg := doLogin(t, h, attacker, "hunter2")
	if code != http.StatusTooManyRequests || ok {
		t.Fatalf("attacker after %d fails: want 429, got code=%d ok=%v err=%q", loginMaxFails, code, ok, errMsg)
	}

	// The admin, logging in with the CORRECT password from a DIFFERENT address,
	// must succeed — the attacker's failures must not have locked them out.
	code, ok, errMsg = doLogin(t, h, admin, "hunter2")
	if code != http.StatusOK || !ok {
		t.Fatalf("admin from different IP with correct password: want 200/ok=true, got code=%d ok=%v err=%q", code, ok, errMsg)
	}
}

// TestLoginThrottleClearsOnlyThatIPOnSuccess exercises the other half of the
// same bug: recordLoginSuccess used to wipe the ONE global fail list, so a
// legitimate login from IP A would also reset an attacker's counter on IP B.
// A successful login must only clear its own key.
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

	// A successful login from a different address...
	if code, ok, errMsg := doLogin(t, h, other, "hunter2"); code != http.StatusOK || !ok {
		t.Fatalf("other IP correct password: want 200/ok=true, got code=%d ok=%v err=%q", code, ok, errMsg)
	}

	// ...must NOT clear the attacker's own throttle.
	code, ok, errMsg := doLogin(t, h, attacker, "hunter2")
	if code != http.StatusTooManyRequests || ok {
		t.Fatalf("attacker still throttled after unrelated IP's success: want 429, got code=%d ok=%v err=%q", code, ok, errMsg)
	}
}

// TestLoginThrottleRecoversAfterWindow confirms the per-key rolling window
// still self-heals: once the failures age out of loginWindow, the same
// address can try again.
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

// TestLoginThrottleSweepsStaleOneOffKeys is the fix for the finding that
// loginThrottled only pruned the ONE key it was asked about: a flood of
// one-off distinct keys that each fail exactly once and are never queried
// again — a botnet, or a single attacker rotating through an IPv6 /64's
// effectively unlimited addresses — used to leave a PERMANENT map entry per
// key, since nothing ever revisited or swept the others. Seeding
// loginSweepEvery+10 stale, never-again-queried keys directly (bypassing the
// HTTP path, which would need that many real requests) and then driving
// enough loginThrottled calls on a completely DIFFERENT key to cross the
// periodic-sweep threshold must clear every one of them out.
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

	// None of the one-off keys above is ever queried again — only this
	// unrelated "driver" key drives the calls that cross loginSweepEvery.
	for i := 0; i < loginSweepEvery; i++ {
		h.loginThrottled("driver")
	}

	h.loginMu.Lock()
	remaining := len(h.loginFails)
	h.loginMu.Unlock()
	// "driver" itself has 0 fails, so its own per-key prune deletes it too —
	// any survivor here must be one of the stale one-off keys the periodic
	// sweep was supposed to have cleared.
	if remaining != 0 {
		t.Fatalf("loginFails still holds %d stale one-off entries after %d loginThrottled calls (>= loginSweepEvery=%d) — the periodic sweep did not run", remaining, loginSweepEvery, loginSweepEvery)
	}
}

// TestLoginThrottleCapsMapSize is the backstop for a burst faster than
// loginSweepEvery calls apart: even loginMaxTracked+50 distinct keys, all with
// CURRENT (within-window) timestamps that a plain prune-empty-entries pass
// cannot remove, must not be allowed to grow loginFails past loginMaxTracked —
// a single loginThrottled call has to notice the map is already over the hard
// cap and evict the least-recently-touched entries down to it.
func TestLoginThrottleCapsMapSize(t *testing.T) {
	h := &Handler{}
	now := time.Now()
	h.loginMu.Lock()
	h.loginFails = make(map[string][]time.Time)
	const extra = 50
	for i := 0; i < loginMaxTracked+extra; i++ {
		// Distinct, strictly increasing timestamps, all well within
		// loginWindow — a full prune alone (which only deletes EMPTY
		// entries) cannot shrink this map; only least-recently-touched
		// eviction can.
		h.loginFails[fmt.Sprintf("flood-%d", i)] = []time.Time{now.Add(time.Duration(i) * time.Millisecond)}
	}
	h.loginMu.Unlock()

	h.loginThrottled("flood-0") // any single call must trigger the hard-cap check

	h.loginMu.Lock()
	size := len(h.loginFails)
	h.loginMu.Unlock()
	if size > loginMaxTracked {
		t.Fatalf("loginFails held %d entries after exceeding loginMaxTracked=%d, want <= %d — the hard cap did not evict", size, loginMaxTracked, loginMaxTracked)
	}
}
