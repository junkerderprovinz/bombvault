package keydeposit

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"fmt"
)

// A group shares one secret, so a member can speak in any other member's
// name. An install therefore proves to its keeper that a deposit is its own
// with an Ed25519 key the keeper saw with the first deposit. Whoever else
// names the install has no such key and can neither replace the deposit nor
// take it back.

// The actions a proof covers.
const (
	ProofKeep = "keep"
	ProofDrop = "drop"
)

// Claim is what an install says to its keeper and signs. Counter only ever
// grows, so a call the keeper has seen cannot be sent to it again.
type Claim struct {
	Action   string
	OwnerID  string
	KeeperID string
	Counter  int64
	Deposit  []byte
	Salt     []byte
}

// NewOwnerSeed returns the seed of an install's proof key.
func NewOwnerSeed() ([]byte, error) {
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		return nil, fmt.Errorf("keydeposit: proof key: %w", err)
	}
	return seed, nil
}

// OwnerKey returns the public half of a proof key, which the keeper holds.
func OwnerKey(seed []byte) []byte {
	return ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
}

// message spells a claim so that no two claims give the same bytes: every
// field goes in with its length.
func (c Claim) message() []byte {
	out := []byte("bombvault key deposit proof 1\x00")
	for _, field := range [][]byte{[]byte(c.Action), []byte(c.OwnerID), []byte(c.KeeperID), c.Deposit, c.Salt} {
		out = binary.BigEndian.AppendUint64(out, uint64(len(field)))
		out = append(out, field...)
	}
	return binary.BigEndian.AppendUint64(out, uint64(c.Counter)) //nolint:gosec // G115: a counter is never negative
}

// Prove signs a claim with the install's proof key.
func Prove(seed []byte, c Claim) []byte {
	return ed25519.Sign(ed25519.NewKeyFromSeed(seed), c.message())
}

// Proven reports whether proof is the claim signed by the holder of key.
func Proven(key, proof []byte, c Claim) bool {
	return len(key) == ed25519.PublicKeySize && ed25519.Verify(key, c.message(), proof)
}
