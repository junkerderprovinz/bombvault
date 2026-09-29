package relay

import (
	"sync"
	"time"
)

// ClockSkew is how far a call's timestamp may be from this instance's clock.
const ClockSkew = 2 * time.Minute

// ReplayGuard admits each sealed call once and only while its timestamp is
// recent. An instance keeps one guard for the relay and the direct transport,
// so a call captured on either cannot be run a second time on the other.
type ReplayGuard struct {
	mu   sync.Mutex
	seen map[string]time.Time
	now  func() time.Time
}

// NewReplayGuard returns a guard that has seen nothing.
func NewReplayGuard() *ReplayGuard {
	return &ReplayGuard{seen: map[string]time.Time{}, now: time.Now}
}

// Admit reports whether call is recent and new, and records it. Ids are kept
// for twice the skew, which covers every call whose timestamp would still
// pass.
func (g *ReplayGuard) Admit(call ProxyCall) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	if d := now.Sub(time.Unix(call.Sent, 0)); call.ID == "" || d > ClockSkew || d < -ClockSkew {
		return false
	}
	for id, at := range g.seen {
		if now.Sub(at) > 2*ClockSkew {
			delete(g.seen, id)
		}
	}
	if _, dup := g.seen[call.ID]; dup {
		return false
	}
	g.seen[call.ID] = now
	return true
}
