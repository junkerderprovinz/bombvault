package secret

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
	"time"
)

// A test fixture, not a real credential. Its shape alone matches the gitleaks
// generic-api-key rule, hence the annotation.
const appKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" //gitleaks:allow

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

const otherKey = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"

func mustHash(t *testing.T, key, password string) string {
	t.Helper()
	h, err := HashPassword(key, password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	return h
}

func TestHashPasswordVerify(t *testing.T) {
	hash := mustHash(t, appKey, "hunter2")
	if !VerifyPassword(appKey, "hunter2", hash) {
		t.Fatal("VerifyPassword: correct password must return true")
	}
}

func TestVerifyPasswordWrongPassword(t *testing.T) {
	hash := mustHash(t, appKey, "hunter2")
	if VerifyPassword(appKey, "wrong", hash) {
		t.Fatal("VerifyPassword: wrong password must return false")
	}
}

func TestVerifyPasswordWrongAppKey(t *testing.T) {
	hash := mustHash(t, appKey, "hunter2")
	if VerifyPassword(otherKey, "hunter2", hash) {
		t.Fatal("VerifyPassword: wrong APP_KEY must return false")
	}
}

func TestSessionTokenRoundtrip(t *testing.T) {
	hash := mustHash(t, appKey, "s3cret")
	tok := NewSessionToken(appKey, hash, "", time.Hour)
	if !ValidSessionToken(appKey, hash, "", tok) {
		t.Fatal("ValidSessionToken: fresh token must be valid")
	}
}

func TestSessionTokenRoundtripWithEpoch(t *testing.T) {
	hash := mustHash(t, appKey, "s3cret")
	const epoch = "0011223344556677"
	tok := NewSessionToken(appKey, hash, epoch, time.Hour)
	if !ValidSessionToken(appKey, hash, epoch, tok) {
		t.Fatal("ValidSessionToken: fresh token must be valid under its own epoch")
	}
}

func TestSessionTokenExpired(t *testing.T) {
	hash := mustHash(t, appKey, "s3cret")
	tok := NewSessionToken(appKey, hash, "", -time.Second)
	if ValidSessionToken(appKey, hash, "", tok) {
		t.Fatal("ValidSessionToken: expired token must be invalid")
	}
}

func TestSessionTokenTampered(t *testing.T) {
	hash := mustHash(t, appKey, "s3cret")
	tok := NewSessionToken(appKey, hash, "", time.Hour)
	b := []byte(tok)
	b[len(b)-1] ^= 0x01
	if ValidSessionToken(appKey, hash, "", string(b)) {
		t.Fatal("ValidSessionToken: tampered token must be invalid")
	}
}

func TestSessionTokenPasswordHashChanged(t *testing.T) {
	hash := mustHash(t, appKey, "s3cret")
	tok := NewSessionToken(appKey, hash, "", time.Hour)

	newHash := mustHash(t, appKey, "newpassword")
	if ValidSessionToken(appKey, newHash, "", tok) {
		t.Fatal("ValidSessionToken: token must be invalid after password change")
	}
}

func TestSessionTokenEpochChanged(t *testing.T) {
	hash := mustHash(t, appKey, "s3cret")
	tok := NewSessionToken(appKey, hash, "epochA", time.Hour)
	if ValidSessionToken(appKey, hash, "epochB", tok) {
		t.Fatal("ValidSessionToken: token minted under epoch A must be invalid under epoch B")
	}
	// Moving from the empty epoch to a set one revokes as well.
	legacyTok := NewSessionToken(appKey, hash, "", time.Hour)
	if ValidSessionToken(appKey, hash, "epochB", legacyTok) {
		t.Fatal("ValidSessionToken: empty-epoch token must be invalid after epoch rotation")
	}
}

// TestSessionTokenLegacyFormatValidUnderEmptyEpoch checks that a token signed
// without an epoch segment stays valid under the empty epoch.
func TestSessionTokenLegacyFormatValidUnderEmptyEpoch(t *testing.T) {
	hash := mustHash(t, appKey, "s3cret")
	expiry := strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)
	legacy := expiry + "." + hmacHex(appKey, "bombvault:session:"+expiry+":"+hash)
	if !ValidSessionToken(appKey, hash, "", legacy) {
		t.Fatal("ValidSessionToken: pre-epoch legacy token must be valid under the empty epoch")
	}
}

func TestSessionTokenBadFormat(t *testing.T) {
	hash := mustHash(t, appKey, "x")
	if ValidSessionToken(appKey, hash, "", "nodot") {
		t.Fatal("ValidSessionToken: token without dot must be invalid")
	}
	if ValidSessionToken(appKey, hash, "", "notanumber.abc") {
		t.Fatal("ValidSessionToken: non-numeric expiry must be invalid")
	}
}
