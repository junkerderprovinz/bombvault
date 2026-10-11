package keydeposit

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"fmt"
)

// A group shares one secret, so a member can speak in any other member's
// name. An install therefore signs what only it may say with an Ed25519 key:
// its deposits to the keeper that saw the key with the first one, and its
// answers about roles, the passwords and logins those give, and requests for
// a deposit to every member, which learns the key from the install itself.
// Whoever else names the install has no such key.

// The actions a proof covers.
const (
	ProofKeep = "keep"
	ProofDrop = "drop"
	// ProofKey is a member's answer to the question which key it signs with.
	ProofKey           = "key"
	ProofRoleAnswer    = "role-answer"
	ProofRoleWithdraw  = "role-withdraw"
	ProofPairing       = "pairing"
	ProofReceiverLogin = "receiver-login"
	ProofRelease       = "release"
	ProofZFSReceive    = "zfs-receive"
)

// Claim is what an install says and signs. Counter only ever grows, so a call
// the other side has seen cannot be sent to it again. Deposit and Salt are a
// deposit's; another call puts what it says into Deposit.
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
