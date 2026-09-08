package secret

import (
	"crypto/hmac"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Password storage.
//
// WHY THIS EXISTS. Until v8.5.5 a login password was stored as
// HMAC-SHA256(APP_KEY, "bombvault:auth:"+password) — see hashPasswordLegacy
// below. That is a good MAC and a bad password hash, for one reason: it is fast
// on purpose. The only thing standing between a stolen /config and the operator's
// password was the secrecy of APP_KEY, and APP_KEY lives in the SAME /config
// directory as the database that holds the hash. One copied volume — an old
// backup, a pulled disk, a snapshot shared for support — hands an attacker both
// halves, and from there a commodity GPU walks the whole realistic keyspace in
// hours.
//
// Argon2id fixes the half that can be fixed in software: each guess now costs
// memory and time instead of a single SHA-256 block.
//
// THE PEPPER IS KEPT. The password is still HMAC'd with APP_KEY before it
// reaches Argon2id, so the old property survives: an attacker holding only the
// database and not APP_KEY cannot even begin. Argon2id's own salt handles the
// property HMAC never had (two installs with the same password now store
// different hashes). Belt and braces, and the braces are the cheap part.
//
// PARAMETERS. m=19 MiB, t=2, p=1 is OWASP's first recommended Argon2id profile.
// The temptation is to reach for 64 MiB, and it was resisted deliberately:
// BombVault ships as an Unraid container that people cap with --memory, often at
// 256 MB, and a login must not be the allocation that pushes the process into
// the OOM killer. The login throttle (5 attempts per minute per client) already
// bounds how often this runs, so the marginal value of more memory here is small
// next to the cost of a container that dies when someone signs in.
const (
	argonMemoryKiB = 19 * 1024 // 19 MiB
	argonTime      = 2
	argonThreads   = 1
	argonKeyLen    = 32
	argonSaltLen   = 16

	// argonPrefix marks the stored format. A hash without it is a legacy
	// HMAC hex string; see VerifyPassword.
	argonPrefix = "argon2id$"
)

// hashPasswordLegacy is the pre-v8.6.0 storage format: a bare HMAC-SHA256 hex
// string. Kept ONLY so that an install that has not yet seen a successful login
// since the upgrade can still verify (and then be migrated by NeedsRehash). Never
// call it to WRITE a new hash.
func hashPasswordLegacy(appKey, password string) string {
	return hmacHex(appKey, "bombvault:auth:"+password)
}

// pepper returns the APP_KEY-keyed input Argon2id actually hashes. Reusing the
// legacy message means the pepper is domain-separated exactly as before.
func pepper(appKey, password string) []byte {
	return []byte(hashPasswordLegacy(appKey, password))
}

// HashPassword derives a stored password hash from appKey and password using
// Argon2id over an APP_KEY-keyed pepper. The returned string carries its own
// parameters and salt, so a future parameter change can still verify hashes
// written today.
//
// Format: argon2id$<m>$<t>$<p>$<salt-b64>$<key-b64>
//
// It panics on an invalid (non-hex) appKey, like the rest of this package.
func HashPassword(appKey, password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("secret: password salt: %w", err)
	}
	return hashPasswordWithSalt(appKey, password, salt, argonMemoryKiB, argonTime, argonThreads), nil
}

func hashPasswordWithSalt(appKey, password string, salt []byte, m uint32, t uint32, p uint8) string {
	key := argon2.IDKey(pepper(appKey, password), salt, t, m, p, argonKeyLen)
	return fmt.Sprintf("%s%d$%d$%d$%s$%s",
		argonPrefix, m, t, p,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
}

// VerifyPassword returns true when password matches storedHash under appKey.
//
// It accepts BOTH storage formats. A stored value beginning with "argon2id$" is
// verified with the parameters recorded in the value itself; anything else is
// treated as the legacy HMAC hex string. Both comparisons are constant-time.
//
// It panics on an invalid (non-hex) appKey.
func VerifyPassword(appKey, password, storedHash string) bool {
	if strings.HasPrefix(storedHash, argonPrefix) {
		return verifyArgon(appKey, password, storedHash)
	}
	got := hashPasswordLegacy(appKey, password)
	// Compare the hex STRINGS, not decoded bytes: a stored value that is not
	// valid hex must fail, not decode to empty and match another empty.
	return hmac.Equal([]byte(got), []byte(storedHash))
}

func verifyArgon(appKey, password, storedHash string) bool {
	m, t, p, salt, want, ok := parseArgon(storedHash)
	if !ok {
		return false
	}
	// argon2 wants the key length as a uint32 and len() is an int. parseArgon
	// already refuses an empty key, and a stored hash is one base64 field in a
	// settings row, so this can never be near 4 GiB - but a bound costs nothing
	// and makes the conversion provably safe instead of safe by argument.
	n := len(want)
	if n <= 0 || n > math.MaxUint32 {
		return false
	}
	got := argon2.IDKey(pepper(appKey, password), salt, t, m, p, uint32(n))
	return hmac.Equal(got, want)
}

// parseArgon splits a stored argon2id value into its parameters. It returns
// ok=false for anything malformed rather than guessing at defaults: a corrupted
// hash must refuse every password, not accidentally accept one hashed with the
// current parameters.
func parseArgon(s string) (m uint32, t uint32, p uint8, salt, key []byte, ok bool) {
	parts := strings.Split(strings.TrimPrefix(s, argonPrefix), "$")
	if len(parts) != 5 {
		return 0, 0, 0, nil, nil, false
	}
	mv, err1 := strconv.ParseUint(parts[0], 10, 32)
	tv, err2 := strconv.ParseUint(parts[1], 10, 32)
	pv, err3 := strconv.ParseUint(parts[2], 10, 8)
	salt, err4 := base64.RawStdEncoding.DecodeString(parts[3])
	key, err5 := base64.RawStdEncoding.DecodeString(parts[4])
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil || err5 != nil {
		return 0, 0, 0, nil, nil, false
	}
	// Zero anywhere would make argon2.IDKey panic, and an empty key would make
	// hmac.Equal a coin flip against another empty result.
	if mv == 0 || tv == 0 || pv == 0 || len(salt) == 0 || len(key) == 0 {
		return 0, 0, 0, nil, nil, false
	}
	return uint32(mv), uint32(tv), uint8(pv), salt, key, true
}

// NeedsRehash reports whether storedHash should be replaced with a freshly
// derived one. True for every legacy HMAC value and for an Argon2id value whose
// recorded parameters are weaker than the current ones.
//
// The caller (the login handler) rehashes on a SUCCESSFUL login, which is the
// only moment the plaintext password is available. An install whose operator
// never signs in again keeps its old hash, which is exactly as safe as it was
// before the upgrade and no worse.
func NeedsRehash(storedHash string) bool {
	if !strings.HasPrefix(storedHash, argonPrefix) {
		return true
	}
	m, t, p, _, key, ok := parseArgon(storedHash)
	if !ok {
		// Unparseable: it can never verify anyway, so there is nothing to
		// preserve. Say false — rehashing happens only after a successful
		// verify, which this value cannot produce.
		return false
	}
	return m < argonMemoryKiB || t < argonTime || p < argonThreads || len(key) < argonKeyLen
}

// MinPasswordLen is the shortest password the settings page will store.
//
// There was no minimum at all before v8.6.0: handleSetPassword accepted any
// non-empty string, so "1234" was a valid password on an instance somebody had
// just published through a reverse proxy. The throttle allows 5 attempts a
// minute, which is 7200 a day, which walks a four-digit PIN in under two days.
// Twelve characters is the number that makes the throttle irrelevant instead of
// load-bearing.
//
// It is checked when SETTING a password, never when verifying one: an existing
// short password keeps working, and its owner is told to change it rather than
// locked out by an upgrade.
const MinPasswordLen = 12
