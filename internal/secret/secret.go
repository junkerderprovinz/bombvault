// Package secret provides AES-256-GCM encryption keyed by a value derived from
// the APP_KEY. It is used to store container definitions on the backup storage,
// so they survive the loss of BombVault's own /config without exposing the env
// vars and other secrets they contain. The key derivation is domain-separated
// from the restic password derivation in package restickey.
//
// The package also holds the login helpers: password hashing, session tokens,
// TOTP and recovery codes.
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
	return mac.Sum(nil), nil
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

// hmacHex returns hex(HMAC-SHA256(hexDecode(appKey), message)). It panics on a
// non-hex appKey; callers validate the key first.
func hmacHex(appKey, message string) string {
	keyBytes, err := hex.DecodeString(appKey)
	if err != nil {
		panic(fmt.Sprintf("secret: invalid hex APP_KEY: %v", err))
	}
	mac := hmac.New(sha256.New, keyBytes)
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

// sessionMessage builds the message a session token signs. Rotating the epoch
// (POST /api/logout-all) changes the message for every token and so ends all
// sessions at once. An empty epoch leaves the epoch segment out, which keeps
// tokens issued before epochs existed valid until the first rotation. The two
// forms cannot collide because passwordHash never contains the ":" separator.
func sessionMessage(expiry, passwordHash, epoch string) string {
	if epoch == "" {
		return "bombvault:session:" + expiry + ":" + passwordHash
	}
	return "bombvault:session:" + expiry + ":" + epoch + ":" + passwordHash
}

// NewSessionToken creates a signed, time-limited session token of the form
// "<expiryUnix>.<hex-HMAC>". The MAC covers the expiry, passwordHash and epoch,
// so changing the password or rotating the epoch invalidates every existing
// session. It panics on a non-hex appKey.
func NewSessionToken(appKey, passwordHash, epoch string, ttl time.Duration) string {
	expiry := strconv.FormatInt(time.Now().Add(ttl).Unix(), 10)
	sig := hmacHex(appKey, sessionMessage(expiry, passwordHash, epoch))
	return expiry + "." + sig
}

// ValidSessionToken verifies a token produced by NewSessionToken. It returns
// false for any parse error, expired token, wrong APP_KEY, wrong epoch or
// tampered value. It panics on a non-hex appKey.
func ValidSessionToken(appKey, passwordHash, epoch, token string) bool {
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
		return false
	}

	wantSig := hmacHex(appKey, sessionMessage(expStr, passwordHash, epoch))
	a, err1 := hex.DecodeString(gotSig)
	b, err2 := hex.DecodeString(wantSig)
	if err1 != nil || err2 != nil {
		return false
	}
	return hmac.Equal(a, b)
}

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
