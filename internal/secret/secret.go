// Package secret provides AES-256-GCM encryption keyed by a value derived from
// the APP_KEY. It is used to store container definitions on the backup storage
// (so they survive a loss of BombVault's own /config) without leaking the
// container env vars and other secrets they contain. The key derivation is
// domain-separated from the restic-password derivation in package restickey.
//
// The package also contains authentication helpers (HashPassword, VerifyPassword,
// NewSessionToken, ValidSessionToken) that follow the same HMAC-SHA256/APP_KEY
// pattern.
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// deriveKey returns a 32-byte AES-256 key from appKey, domain-separated from the
// restic password derivation ("bombvault:def-encryption").
func deriveKey(appKey string) ([]byte, error) {
	keyBytes, err := hex.DecodeString(appKey)
	if err != nil {
		return nil, fmt.Errorf("secret: invalid hex APP_KEY: %w", err)
	}
	mac := hmac.New(sha256.New, keyBytes)
	mac.Write([]byte("bombvault:def-encryption"))
	return mac.Sum(nil), nil // 32 bytes → AES-256
}

// Encrypt seals plaintext with AES-256-GCM and returns nonce||ciphertext.
func Encrypt(appKey string, plaintext []byte) ([]byte, error) {
	gcm, err := newGCM(appKey)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("secret: nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt opens data produced by Encrypt (nonce||ciphertext). A wrong APP_KEY or
// tampered ciphertext fails the GCM auth check.
func Decrypt(appKey string, data []byte) ([]byte, error) {
	gcm, err := newGCM(appKey)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(data) < ns {
		return nil, errors.New("secret: ciphertext too short")
	}
	nonce, ct := data[:ns], data[ns:]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("secret: decrypt (wrong APP_KEY or corrupt data): %w", err)
	}
	return pt, nil
}

// ---------------------------------------------------------------------------
// Authentication helpers
// ---------------------------------------------------------------------------

// hmacHex returns hex(HMAC-SHA256(hexDecode(appKey), message)).
// It panics on an invalid (non-hex) appKey — the caller must have validated it.
func hmacHex(appKey, message string) string {
	keyBytes, err := hex.DecodeString(appKey)
	if err != nil {
		panic(fmt.Sprintf("secret: invalid hex APP_KEY: %v", err))
	}
	mac := hmac.New(sha256.New, keyBytes)
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

// HashPassword derives a stored password hash from appKey and password using
// HMAC-SHA256.  The result is deterministic and domain-separated so that an
// offline brute-force also requires knowledge of APP_KEY.
//
// It panics on an invalid (non-hex) appKey.
func HashPassword(appKey, password string) string {
	return hmacHex(appKey, "bombvault:auth:"+password)
}

// VerifyPassword returns true when password hashes to storedHash under appKey.
// The comparison is constant-time to resist timing attacks.
//
// It panics on an invalid (non-hex) appKey.
func VerifyPassword(appKey, password, storedHash string) bool {
	got := HashPassword(appKey, password)
	a, _ := hex.DecodeString(got)
	b, _ := hex.DecodeString(storedHash)
	return hmac.Equal(a, b)
}

// sessionMessage builds the HMAC message a session token signs. The epoch is a
// server-side revocation value: rotating it (POST /api/logout-all) changes the
// message for every token, invalidating all outstanding sessions at once. An
// EMPTY epoch reproduces the pre-epoch legacy message format (no epoch segment),
// so cookies minted before the epoch existed keep validating until the first
// rotation. This is unambiguous because passwordHash is hex (it can never
// contain the ":" separator), so a legacy message can't collide with an
// epoch-bearing one.
func sessionMessage(expiry, passwordHash, epoch string) string {
	if epoch == "" {
		return "bombvault:session:" + expiry + ":" + passwordHash
	}
	return "bombvault:session:" + expiry + ":" + epoch + ":" + passwordHash
}

// NewSessionToken creates a signed, time-limited session token.
//
// Format: "<expiryUnix>.<hex-HMAC>"
//
// The MAC is bound to the expiry timestamp, the current passwordHash and the
// session epoch, so that changing or clearing the password — or rotating the
// epoch ("log out everywhere") — instantly invalidates all existing sessions.
// An empty epoch is a valid (legacy) value; see sessionMessage.
//
// It panics on an invalid (non-hex) appKey.
func NewSessionToken(appKey, passwordHash, epoch string, ttl time.Duration) string {
	expiry := strconv.FormatInt(time.Now().Add(ttl).Unix(), 10)
	sig := hmacHex(appKey, sessionMessage(expiry, passwordHash, epoch))
	return expiry + "." + sig
}

// ValidSessionToken verifies a token produced by NewSessionToken.  It returns
// false for any parse error, expired token, wrong APP_KEY, wrong epoch, or
// tampered value.
//
// It panics on an invalid (non-hex) appKey.
func ValidSessionToken(appKey, passwordHash, epoch, token string) bool {
	// Split on the LAST "." so that the expiry part can never contain a dot.
	dot := strings.LastIndex(token, ".")
	if dot < 0 {
		return false
	}
	expStr, gotSig := token[:dot], token[dot+1:]

	expiry, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix() > expiry {
		return false // expired
	}

	wantSig := hmacHex(appKey, sessionMessage(expStr, passwordHash, epoch))
	// Constant-time hex comparison.
	a, err1 := hex.DecodeString(gotSig)
	b, err2 := hex.DecodeString(wantSig)
	if err1 != nil || err2 != nil {
		return false
	}
	return hmac.Equal(a, b)
}

// ---------------------------------------------------------------------------
// AES-256-GCM encryption
// ---------------------------------------------------------------------------

func newGCM(appKey string) (cipher.AEAD, error) {
	key, err := deriveKey(appKey)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("secret: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secret: new gcm: %w", err)
	}
	return gcm, nil
}
