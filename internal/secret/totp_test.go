package secret

import (
	"strings"
	"testing"
	"time"
)

// RFC 6238 ships test vectors, and they are the only real check that this
// implementation is the same algorithm every authenticator app runs. The secret
// in the RFC is the ASCII string "12345678901234567890"; here it is base32 as an
// app would receive it.
const rfcSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

func TestTOTPMatchesRFC6238Vectors(t *testing.T) {
	// The RFC's SHA-1 vectors are eight digits; BombVault uses six, so the
	// expected value is the last six of each.
	cases := []struct {
		unix int64
		want string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
	}
	for _, c := range cases {
		got, err := TOTPCode(rfcSecret, time.Unix(c.unix, 0))
		if err != nil {
			t.Fatalf("TOTPCode(%d): %v", c.unix, err)
		}
		if got != c.want {
			t.Fatalf("TOTPCode at %d = %s, want %s", c.unix, got, c.want)
		}
	}
}

func TestValidTOTPAcceptsTheCurrentCode(t *testing.T) {
	now := time.Unix(1111111109, 0)
	code, err := TOTPCode(rfcSecret, now)
	if err != nil {
		t.Fatal(err)
	}
	if !ValidTOTP(rfcSecret, code, now) {
		t.Fatal("the code for this moment must be accepted")
	}
	// A space in the middle is what a phone screen invites; it must not matter.
	if !ValidTOTP(rfcSecret, code[:3]+" "+code[3:], now) {
		t.Fatal("a code typed with a space must be accepted")
	}
}

// One step of skew each way, and no more. Both halves matter: too narrow locks
// out a phone whose clock drifted, too wide hands an attacker extra seconds.
func TestValidTOTPWindowIsOneStepEitherSide(t *testing.T) {
	now := time.Unix(1111111109, 0)
	code, err := TOTPCode(rfcSecret, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, delta := range []time.Duration{-30 * time.Second, 30 * time.Second} {
		if !ValidTOTP(rfcSecret, code, now.Add(delta)) {
			t.Fatalf("a code must survive %v of clock skew", delta)
		}
	}
	for _, delta := range []time.Duration{-90 * time.Second, 90 * time.Second} {
		if ValidTOTP(rfcSecret, code, now.Add(delta)) {
			t.Fatalf("a code must NOT be accepted %v away", delta)
		}
	}
}

func TestValidTOTPRejectsRubbish(t *testing.T) {
	now := time.Unix(1111111109, 0)
	for _, code := range []string{"", "12345", "1234567", "abcdef", "00000000"} {
		if ValidTOTP(rfcSecret, code, now) {
			t.Fatalf("must reject %q", code)
		}
	}
	// An unusable secret must refuse everything rather than error into acceptance.
	if ValidTOTP("not base32!", "123456", now) {
		t.Fatal("an invalid secret must accept nothing")
	}
	if ValidTOTP("", "123456", now) {
		t.Fatal("an empty secret must accept nothing")
	}
}

func TestNewTOTPSecretIsUsableAndFresh(t *testing.T) {
	a, err := NewTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("two secrets must differ")
	}
	now := time.Now()
	code, err := TOTPCode(a, now)
	if err != nil {
		t.Fatalf("a freshly minted secret must produce a code: %v", err)
	}
	if !ValidTOTP(a, code, now) {
		t.Fatal("a freshly minted secret must validate its own code")
	}
	if ValidTOTP(b, code, now) {
		t.Fatal("one secret's code must not validate under another")
	}
}

func TestTOTPURICarriesWhatAnAppNeeds(t *testing.T) {
	uri := TOTPURI("BombVault", "tower", "ABCDEFGH")
	for _, want := range []string{
		"otpauth://totp/",
		"secret=ABCDEFGH",
		"issuer=BombVault",
		"digits=6",
		"period=30",
		"algorithm=SHA1",
	} {
		if !strings.Contains(uri, want) {
			t.Fatalf("otpauth URI is missing %q: %s", want, uri)
		}
	}
}

// ---------------------------------------------------------------------------
// Recovery codes
// ---------------------------------------------------------------------------

func TestRecoveryCodesMatchOnlyThemselves(t *testing.T) {
	plain, hashed, err := NewRecoveryCodes(appKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(plain) != recoveryCodeCount || len(hashed) != recoveryCodeCount {
		t.Fatalf("want %d codes, got %d/%d", recoveryCodeCount, len(plain), len(hashed))
	}
	seen := map[string]bool{}
	for i, code := range plain {
		if seen[code] {
			t.Fatalf("duplicate recovery code %q", code)
		}
		seen[code] = true
		if got := MatchRecoveryCode(appKey, code, hashed); got != i {
			t.Fatalf("code %d matched index %d", i, got)
		}
	}
	if MatchRecoveryCode(appKey, "abcde-fghij", hashed) >= 0 {
		t.Fatal("an invented code must not match")
	}
	if MatchRecoveryCode(appKey, "", hashed) >= 0 {
		t.Fatal("an empty code must not match — every stored hash would be a target")
	}
}

// Somebody reading a code off paper types it as they see it. Case and dashes
// must not be the reason a lost phone becomes a lost instance.
func TestRecoveryCodeIsForgivingAboutTyping(t *testing.T) {
	plain, hashed, err := NewRecoveryCodes(appKey)
	if err != nil {
		t.Fatal(err)
	}
	code := plain[3]
	for _, typed := range []string{
		strings.ToUpper(code),
		strings.ReplaceAll(code, "-", ""),
		strings.ReplaceAll(code, "-", " "),
		"  " + code + "  ",
	} {
		if MatchRecoveryCode(appKey, typed, hashed) != 3 {
			t.Fatalf("must accept %q", typed)
		}
	}
}

func TestRecoveryCodesNeedTheAppKey(t *testing.T) {
	plain, hashed, err := NewRecoveryCodes(appKey)
	if err != nil {
		t.Fatal(err)
	}
	if MatchRecoveryCode(otherKey, plain[0], hashed) >= 0 {
		t.Fatal("a stolen database without the APP_KEY must not yield a usable code")
	}
}
