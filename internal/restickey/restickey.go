// Package restickey derives a restic repository password from an APP_KEY.
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
	keyBytes, err := hex.DecodeString(appKey)
	if err != nil {
		// Callers validate appKey first, so bad hex here is a programming error.
		panic(fmt.Sprintf("restickey.Derive: invalid hex APP_KEY: %v", err))
	}
	mac := hmac.New(sha256.New, keyBytes)
	mac.Write([]byte("bombvault:restic-repo"))
	return hex.EncodeToString(mac.Sum(nil))
}
