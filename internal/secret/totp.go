package secret

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // G505: RFC 6238 fixes SHA-1 for TOTP; every authenticator app implements that and nothing else.
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"
)

// Time-based one-time passwords (RFC 6238), the second factor for the login.
//
// WRITTEN OUT RATHER THAN IMPORTED, on purpose. The whole algorithm is the
// forty lines below: an HMAC over a 30-second counter, truncated to six digits.
// Pulling a module in to get that would add a supply-chain dependency to the one
// code path whose whole job is to be trustworthy, and the spec has not moved
// since 2011.
//
// SHA-1 is not a mistake here. RFC 6238 names it, every authenticator app
// implements it, and the construction is HMAC, where SHA-1's collision weakness
// does not apply. An install using SHA-256 would simply fail to enrol in Google
// Authenticator.

const (
	totpDigits = 6
	totpPeriod = 30 * time.Second
	// totpSkew is how many steps either side of "now" are accepted. One step
	// each way covers the ordinary case of a phone clock a few seconds off and
	// a code typed just as it rolls over. Wider windows buy an attacker time
	// and buy the operator nothing.
	totpSkew = 1
	// totpSecretLen is 20 bytes, the length RFC 4226 recommends and the length
	// authenticator apps expect from a base32 secret.
	totpSecretLen = 20
)

var totpEnc = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewTOTPSecret returns a fresh base32 secret suitable for an authenticator app.
func NewTOTPSecret() (string, error) {
	buf := make([]byte, totpSecretLen)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", fmt.Errorf("secret: totp secret: %w", err)
	}
	return totpEnc.EncodeToString(buf), nil
}

// TOTPCode returns the six-digit code for secret at time t.
func TOTPCode(secret string, t time.Time) (string, error) {
	key, err := totpEnc.DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil {
		return "", fmt.Errorf("secret: totp: invalid base32 secret: %w", err)
	}
	return totpAt(key, totpStep(t)), nil
}

// totpStep converts a wall-clock time into an RFC 6238 counter.
//
// The clamp is the point: Unix() is an int64 and the counter is a uint64, so a
// time before the epoch would wrap to an enormous step and hand out codes from
// a window no verifier will ever reach, silently. A box whose clock has not
// been set yet is the realistic way to get there, and it is exactly the moment
// a second factor must not start producing nonsense.
func totpStep(t time.Time) uint64 {
	sec := t.Unix()
	if sec < 0 {
		return 0
	}
	return uint64(sec) / uint64(totpPeriod.Seconds())
}

func totpAt(key []byte, counter uint64) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(buf[:])
	sum := mac.Sum(nil)

	// Dynamic truncation (RFC 4226 §5.3): the low nibble of the last byte picks
	// the 4-byte window, and the top bit is masked off so the value is positive.
	offset := sum[len(sum)-1] & 0x0f
	code := (uint32(sum[offset])&0x7f)<<24 |
		uint32(sum[offset+1])<<16 |
		uint32(sum[offset+2])<<8 |
		uint32(sum[offset+3])

	mod := uint32(1)
	for range totpDigits {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", totpDigits, code%mod)
}

// ValidTOTP reports whether code is valid for secret around time t, allowing one
// step of clock skew either side.
//
// The comparison is constant-time. That matters less for a six-digit code than
// for a password, but a timing oracle on the FIRST digits would let an attacker
// find each digit independently, which turns a million guesses into sixty.
func ValidTOTP(secret, code string, t time.Time) bool {
	code = strings.TrimSpace(code)
	// Some apps and some people insert a space in the middle.
	code = strings.ReplaceAll(code, " ", "")
	if len(code) != totpDigits {
		return false
	}
	key, err := totpEnc.DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil || len(key) == 0 {
		return false
	}
	step := totpStep(t)
	ok := false
	for d := -totpSkew; d <= totpSkew; d++ {
		c := step
		switch {
		case d < 0:
			if c < uint64(-d) {
				continue
			}
			c -= uint64(-d)
		case d > 0:
			c += uint64(d)
		}
		// No early return: every window is compared, so the time taken does not
		// reveal WHICH window matched.
		if subtle.ConstantTimeCompare([]byte(totpAt(key, c)), []byte(code)) == 1 {
			ok = true
		}
	}
	return ok
}

// TOTPURI builds the otpauth:// URI an authenticator app scans. Issuer appears
// both as the label prefix and as a parameter, which is what the apps that
// disagree about the format each need.
func TOTPURI(issuer, account, secret string) string {
	label := url.PathEscape(issuer + ":" + account)
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprint(totpDigits))
	q.Set("period", fmt.Sprint(int(totpPeriod.Seconds())))
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// ---------------------------------------------------------------------------
// Recovery codes
// ---------------------------------------------------------------------------

const (
	// recoveryCodeCount is how many single-use codes are handed out when 2FA is
	// switched on. Eight is enough to survive a lost phone and few enough that
	// people actually write them down.
	recoveryCodeCount = 8
	recoveryHalfLen   = 5 // characters per half, "abcde-fghij"
)

// recoveryAlphabet omits the characters people misread off a printed sheet:
// 0/O, 1/l/I, and the pairs that look alike in a condensed font.
const recoveryAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"

// NewRecoveryCodes returns fresh single-use codes in plain text (shown to the
// operator once) together with their stored hashes.
//
// The stored form is a plain HMAC, NOT Argon2id, and that is deliberate: unlike a
// human-chosen password these codes carry about 50 bits of entropy each, so
// there is no dictionary to slow down. Argon2id would only make the login slower
// by eight verifications.
func NewRecoveryCodes(appKey string) (plain []string, hashed []string, err error) {
	plain = make([]string, 0, recoveryCodeCount)
	hashed = make([]string, 0, recoveryCodeCount)
	for range recoveryCodeCount {
		code, cErr := randomRecoveryCode()
		if cErr != nil {
			return nil, nil, cErr
		}
		plain = append(plain, code)
		hashed = append(hashed, HashRecoveryCode(appKey, code))
	}
	return plain, hashed, nil
}

func randomRecoveryCode() (string, error) {
	buf := make([]byte, recoveryHalfLen*2)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", fmt.Errorf("secret: recovery code: %w", err)
	}
	var b strings.Builder
	for i, v := range buf {
		if i == recoveryHalfLen {
			b.WriteByte('-')
		}
		// Modulo bias over a 31-character alphabet drawn from 256 values is
		// under half a bit per character and irrelevant next to the 50 bits the
		// code carries. Rejection sampling here would be ceremony.
		b.WriteByte(recoveryAlphabet[int(v)%len(recoveryAlphabet)])
	}
	return b.String(), nil
}

// HashRecoveryCode returns the stored form of a recovery code. Case and dashes
// are normalised first so that someone typing "ABCDE FGHIJ" still gets in.
func HashRecoveryCode(appKey, code string) string {
	return hmacHex(appKey, "bombvault:recovery:"+NormalizeRecoveryCode(code))
}

// NormalizeRecoveryCode strips everything that only exists for legibility.
func NormalizeRecoveryCode(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	code = strings.ReplaceAll(code, "-", "")
	code = strings.ReplaceAll(code, " ", "")
	return code
}

// MatchRecoveryCode returns the index of the stored hash that code matches, or
// -1. Every entry is compared so the time taken does not reveal which code was
// used, or how many are left.
func MatchRecoveryCode(appKey, code string, stored []string) int {
	if NormalizeRecoveryCode(code) == "" {
		return -1
	}
	want := HashRecoveryCode(appKey, code)
	found := -1
	for i, h := range stored {
		if subtle.ConstantTimeCompare([]byte(want), []byte(h)) == 1 {
			found = i
		}
	}
	return found
}
