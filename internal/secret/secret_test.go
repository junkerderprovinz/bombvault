package secret

import (
	"bytes"
	"strings"
	"testing"
)

const appKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestEncryptDecryptRoundtrip(t *testing.T) {
	plain := []byte(`{"inspect":{"Image":"x"},"template_xml":"<xml/>"}`)
	ct, err := Encrypt(appKey, plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if bytes.Contains(ct, plain) {
		t.Fatal("ciphertext must not contain the plaintext")
	}
	got, err := Decrypt(appKey, ct)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("roundtrip mismatch: %q", got)
	}
}

func TestDecryptWrongKeyFails(t *testing.T) {
	ct, err := Encrypt(appKey, []byte("secret env vars"))
	if err != nil {
		t.Fatal(err)
	}
	other := strings.Repeat("a", 64)
	if _, err := Decrypt(other, ct); err == nil {
		t.Fatal("decrypt with wrong APP_KEY must fail")
	}
}

func TestDecryptShortCiphertextFails(t *testing.T) {
	if _, err := Decrypt(appKey, []byte("xx")); err == nil {
		t.Fatal("decrypt of too-short data must fail")
	}
}

func TestEncryptInvalidAppKeyFails(t *testing.T) {
	if _, err := Encrypt("not-hex", []byte("x")); err == nil {
		t.Fatal("encrypt with non-hex APP_KEY must fail")
	}
}
