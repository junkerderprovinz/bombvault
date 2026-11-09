// Package restickey derives what a server's restic repositories are opened and
// written with from its backup key.
package restickey

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Derive returns the HMAC-SHA256 of "bombvault:restic-repo", keyed with the
// hex-decoded appKey, as 64 lowercase hex characters.
func Derive(appKey string) string {
	return mac(appKey, "bombvault:restic-repo")
}

// Host returns the restic host the backups under appKey are written with: bv-
// and the first twelve hex characters of the HMAC-SHA256 of "bombvault:host".
// It follows the key, so a server that takes over another one's backups goes
// on with the same host.
func Host(appKey string) string {
	return "bv-" + mac(appKey, "bombvault:host")[:12]
}

func mac(appKey, msg string) string {
	keyBytes, err := hex.DecodeString(appKey)
	if err != nil {
		// Callers validate appKey first, so bad hex here is a programming error.
		panic(fmt.Sprintf("restickey: invalid hex APP_KEY: %v", err))
	}
	m := hmac.New(sha256.New, keyBytes)
	m.Write([]byte(msg))
	return hex.EncodeToString(m.Sum(nil))
}
