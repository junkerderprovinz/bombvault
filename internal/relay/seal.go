package relay

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

// ErrSealed is returned for every frame that does not open, whether the key
// was wrong or the frame truncated or tampered with, so a failed guess reveals
// nothing about how close it was.
var ErrSealed = errors.New("relay: frame could not be opened")

const nonceLen = 12

func aead(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("relay: frame key is %d bytes, want 32", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// seal encrypts plaintext under key and binds it to aad, the frame's clear
// routing fields. The nonce is random because both ends seal independently
// and a counter would have to survive reconnects.
func seal(key []byte, aad string, plaintext []byte) ([]byte, error) {
	gcm, err := aead(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, nonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("relay: no randomness for a frame nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, []byte(aad)), nil
}

func open(key []byte, aad string, sealed []byte) ([]byte, error) {
	gcm, err := aead(key)
	if err != nil {
		return nil, err
	}
	if len(sealed) < nonceLen {
		return nil, ErrSealed
	}
	plaintext, err := gcm.Open(nil, sealed[:nonceLen], sealed[nonceLen:], []byte(aad))
	if err != nil {
		return nil, ErrSealed
	}
	return plaintext, nil
}
