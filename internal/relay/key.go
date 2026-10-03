package relay

import (
	"crypto/sha256"
	"encoding/hex"
)

// DefaultURL is the project relay every instance dials unless its owner picks
// another. It is the same relay KnightLoader uses: the server groups by key
// and never reads what it forwards, and BombVault's key domains keep the two
// apps' groups apart on it.
const DefaultURL = "wss://parleyport.halleluja.design/relay/connect"

// keyDomain separates the relay key from every other value derived from the
// group secret.
const keyDomain = "bombvault/relay/group-key/v1"

// DeriveKey returns the relay key for a group secret, the value an instance
// puts in its hello frame. The relay learns this hash and never the secret.
func DeriveKey(secret []byte) string {
	h := sha256.New()
	h.Write([]byte(keyDomain))
	h.Write(secret)
	return hex.EncodeToString(h.Sum(nil))
}

// frameDomain is its own domain because the relay holds DeriveKey's output,
// and a frame key derivable from that would be one the relay could compute.
const frameDomain = "bombvault/relay/frame-key/v1"

// DeriveFrameKey returns the AES-256-GCM key that seals every call between
// group members. It is derived on each member and never stored or sent.
func DeriveFrameKey(secret []byte) []byte {
	h := sha256.New()
	h.Write([]byte(frameDomain))
	h.Write(secret)
	return h.Sum(nil)
}
