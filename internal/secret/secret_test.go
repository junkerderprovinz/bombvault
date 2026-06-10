package secret

import (
	"bytes"
	"strings"
	"testing"
	"time"
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

// ---------------------------------------------------------------------------
// Auth helper tests
// ---------------------------------------------------------------------------

const otherKey = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"

func TestHashPasswordVerify(t *testing.T) {
	hash := HashPassword(appKey, "hunter2")
	if !VerifyPassword(appKey, "hunter2", hash) {
		t.Fatal("VerifyPassword: correct password must return true")
	}
}

func TestVerifyPasswordWrongPassword(t *testing.T) {
	hash := HashPassword(appKey, "hunter2")
	if VerifyPassword(appKey, "wrong", hash) {
		t.Fatal("VerifyPassword: wrong password must return false")
	}
}

func TestVerifyPasswordWrongAppKey(t *testing.T) {
	hash := HashPassword(appKey, "hunter2")
	// Same password, different APP_KEY — must not verify.
	if VerifyPassword(otherKey, "hunter2", hash) {
		t.Fatal("VerifyPassword: wrong APP_KEY must return false")
	}
}

func TestSessionTokenRoundtrip(t *testing.T) {
	hash := HashPassword(appKey, "s3cret")
	tok := NewSessionToken(appKey, hash, time.Hour)
	if !ValidSessionToken(appKey, hash, tok) {
		t.Fatal("ValidSessionToken: fresh token must be valid")
	}
}

func TestSessionTokenExpired(t *testing.T) {
	hash := HashPassword(appKey, "s3cret")
	// TTL of -1s gives an already-expired token.
	tok := NewSessionToken(appKey, hash, -time.Second)
	if ValidSessionToken(appKey, hash, tok) {
		t.Fatal("ValidSessionToken: expired token must be invalid")
	}
}

func TestSessionTokenTampered(t *testing.T) {
	hash := HashPassword(appKey, "s3cret")
	tok := NewSessionToken(appKey, hash, time.Hour)
	// Flip the last character of the signature.
	b := []byte(tok)
	b[len(b)-1] ^= 0x01
	if ValidSessionToken(appKey, hash, string(b)) {
		t.Fatal("ValidSessionToken: tampered token must be invalid")
	}
}

func TestSessionTokenPasswordHashChanged(t *testing.T) {
	hash := HashPassword(appKey, "s3cret")
	tok := NewSessionToken(appKey, hash, time.Hour)

	newHash := HashPassword(appKey, "newpassword")
	// Token was issued against old hash — must be invalid under new hash.
	if ValidSessionToken(appKey, newHash, tok) {
		t.Fatal("ValidSessionToken: token must be invalid after password change")
	}
}

func TestSessionTokenBadFormat(t *testing.T) {
	hash := HashPassword(appKey, "x")
	if ValidSessionToken(appKey, hash, "nodot") {
		t.Fatal("ValidSessionToken: token without dot must be invalid")
	}
	if ValidSessionToken(appKey, hash, "notanumber.abc") {
		t.Fatal("ValidSessionToken: non-numeric expiry must be invalid")
	}
}
