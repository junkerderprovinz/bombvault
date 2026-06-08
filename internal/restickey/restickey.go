// Package restickey derives a restic repository password from an APP_KEY.
package restickey

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Derive returns a 64-character lowercase hex string derived from appKey using
// HMAC-SHA256 keyed by the hex-decoded APP_KEY bytes with the message
// "bombvault:restic-repo".  The result is deterministic and domain-separated.
func Derive(appKey string) string {
	keyBytes, err := hex.DecodeString(appKey)
	if err != nil {
		// appKey must have been validated before reaching here; panic is
		// appropriate for a programming error at this layer.
		panic(fmt.Sprintf("restickey.Derive: invalid hex APP_KEY: %v", err))
	}
	mac := hmac.New(sha256.New, keyBytes)
	mac.Write([]byte("bombvault:restic-repo"))
	return hex.EncodeToString(mac.Sum(nil))
}
