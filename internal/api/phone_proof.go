package api

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/relay"
)

// A phone of the group signs what it asks with an Ed25519 key of its own,
// which an instance pins the first time it asks the phone, as it does for
// another instance. The group's words alone let anybody name the phone, but
// not sign for it.

// phoneCalls admits each signed call of a phone once. A page loads many
// files at the same time, so the calls arrive in any order and a counter
// that only grows would refuse most of them; a call is taken instead while
// its time is recent and its signature new.
type phoneCalls struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

func (c *phoneCalls) admit(proof []byte, at, now time.Time) bool {
	if d := now.Sub(at); d > relay.ClockSkew || d < -relay.ClockSkew {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, t := range c.seen {
		if now.Sub(t) > 2*relay.ClockSkew {
			delete(c.seen, k)
		}
	}
	if _, dup := c.seen[string(proof)]; dup {
		return false
	}
	if c.seen == nil {
		c.seen = map[string]time.Time{}
	}
	c.seen[string(proof)] = now
	return true
}

// verifyPhone checks that a call in the name of phoneID is signed with the
// key pinned for it, is recent and has not been taken before.
func (s *Service) verifyPhone(ctx context.Context, phoneID, action string, payload []byte, p peerProof) error {
	if len(p.Key) == 0 || len(p.Proof) == 0 {
		return errUnsignedCall
	}
	key, err := s.memberKey(ctx, phoneID)
	if err != nil {
		return err
	}
	g, err := s.store.GetGroupState()
	if err != nil {
		return err
	}
	claim := keydeposit.Claim{Action: action, OwnerID: phoneID, KeeperID: g.InstanceID, Counter: p.Counter, Deposit: payload}
	if !bytes.Equal(key, p.Key) || !keydeposit.Proven(key, p.Proof, claim) {
		return errUnsignedCall
	}
	if !s.phoneCalls.admit(p.Proof, time.UnixMilli(p.Counter), time.Now()) {
		return errUnsignedCall
	}
	return nil
}
