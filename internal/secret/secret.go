// Package secret provides AES-256-GCM encryption keyed by a value derived from
// the APP_KEY. It is used to store container definitions on the backup storage
// (so they survive a loss of BombVault's own /config) without leaking the
// container env vars and other secrets they contain. The key derivation is
// domain-separated from the restic-password derivation in package restickey.
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
