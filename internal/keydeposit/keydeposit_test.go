package keydeposit_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
)

func TestACodeHasFourGroupsOfFiveAndIsNeverTheSame(t *testing.T) {
	shape := regexp.MustCompile(`^([0-9A-HJKMNP-TV-Z]{5}-){3}[0-9A-HJKMNP-TV-Z]{5}$`)
	seen := map[string]bool{}
	for range 50 {
		code, err := keydeposit.NewCode()
		if err != nil {
			t.Fatal(err)
		}
		if !shape.MatchString(code) {
			t.Fatalf("code %q is not four groups of five from the code alphabet", code)
		}
		if seen[code] {
			t.Fatalf("code %q came twice", code)
		}
		seen[code] = true
	}
}

func TestTheKeyPairComesFromCodeAndSaltAlone(t *testing.T) {
	code, err := keydeposit.NewCode()
	if err != nil {
		t.Fatal(err)
	}
	salt, err := keydeposit.NewSalt()
	if err != nil {
		t.Fatal(err)
	}
	other, err := keydeposit.NewSalt()
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	first, err := keydeposit.Recipient(code, salt)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("one derivation took %s", time.Since(start).Round(10*time.Millisecond))
	again, err := keydeposit.Recipient(code, salt)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, again) {
		t.Fatal("the same code and salt gave two recipients")
	}
	salted, err := keydeposit.Recipient(code, other)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first, salted) {
		t.Fatal("another salt gave the same recipient")
	}
	// The code as somebody types it off paper: lower case, spaces, and the
	// letters that look like digits.
	typed := strings.NewReplacer("-", " ", "0", "o", "1", "l").Replace(strings.ToLower(code))
	fromTyped, err := keydeposit.Recipient(typed, salt)
	if err != nil || !bytes.Equal(first, fromTyped) {
		t.Fatalf("the code typed as %q gives another recipient: %v", typed, err)
	}
}

func TestADepositOpensWithItsCodeAndNothingElse(t *testing.T) {
	code, _ := keydeposit.NewCode()
	wrong, _ := keydeposit.NewCode()
	salt, _ := keydeposit.NewSalt()
	recipient, err := keydeposit.Recipient(code, salt)
	if err != nil {
		t.Fatal(err)
	}
	secret := []byte("APP_KEY 0123456789abcdef")

	sealed, err := keydeposit.Seal(secret, recipient)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, secret) || bytes.Contains(sealed, []byte("0123456789abcdef")) {
		t.Fatal("the deposit carries its content in the clear")
	}
	twice, err := keydeposit.Seal(secret, recipient)
	if err != nil || bytes.Equal(sealed, twice) {
		t.Fatalf("sealing the same content twice gave the same bytes: %v", err)
	}

	plain, err := keydeposit.Open(sealed, code, salt)
	if err != nil || !bytes.Equal(plain, secret) {
		t.Fatalf("Open with the code = %q, %v", plain, err)
	}
	if _, err := keydeposit.Open(sealed, wrong, salt); !errors.Is(err, keydeposit.ErrWrongCode) {
		t.Fatalf("Open with another code = %v, want %v", err, keydeposit.ErrWrongCode)
	}
	if _, err := keydeposit.Open(sealed, "too short", salt); !errors.Is(err, keydeposit.ErrCode) {
		t.Fatalf("Open with a malformed code = %v, want %v", err, keydeposit.ErrCode)
	}
	sealed[len(sealed)-1] ^= 1
	if _, err := keydeposit.Open(sealed, code, salt); !errors.Is(err, keydeposit.ErrWrongCode) {
		t.Fatalf("Open of a changed deposit = %v, want %v", err, keydeposit.ErrWrongCode)
	}
}

// A deposit made under one build has to open under every later one, so the
// way a code becomes a key is held to a value worked out once.
func TestACodeGivesTheSameKeyInEveryVersion(t *testing.T) {
	recipient, err := keydeposit.Recipient("01234-56789-ABCDE-FGHJK", []byte("0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	const want = "744bbed16c0bfb49c017cd0c4025e66c98f4dc647db9e538529a139eb735e571"
	if got := hex.EncodeToString(recipient); got != want {
		t.Fatalf("the key of the code is %s, want %s, so deposits made before would stay shut", got, want)
	}
}
