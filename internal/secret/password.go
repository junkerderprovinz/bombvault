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

// Login passwords are stored as Argon2id over an HMAC of the password keyed with
// APP_KEY. APP_KEY sits in the same /config as the database, so one copied volume
// yields both; Argon2id makes each guess cost memory and time instead of a single
// SHA-256 block. The HMAC pepper still makes a database worthless without
// APP_KEY, and the salt gives two installs with the same password different
// hashes.
//
// m=19 MiB, t=2, p=1 is OWASP's first recommended Argon2id profile. More memory
// is not worth it here: the container is often capped at 256 MB, and a login
// must not be what pushes the process into the OOM killer. The login throttle
// (5 attempts per minute per client) already bounds how often this runs.
const (
	argonMemoryKiB = 19 * 1024
	argonTime      = 2
	argonThreads   = 1
	argonKeyLen    = 32
	argonSaltLen   = 16

	// argonPrefix marks the stored format. A hash without it is a legacy
	// HMAC hex string; see VerifyPassword.
	argonPrefix = "argon2id$"
)

// hashPasswordLegacy returns the legacy storage format, a bare HMAC-SHA256 hex
// string. It verifies hashes that NeedsRehash has not replaced yet and serves as
// the pepper; new hashes are never stored in this form.
func hashPasswordLegacy(appKey, password string) string {
	return hmacHex(appKey, "bombvault:auth:"+password)
}

// pepper returns the APP_KEY-keyed value that Argon2id hashes. It uses the same
// domain-separated message as the legacy format.
func pepper(appKey, password string) []byte {
	return []byte(hashPasswordLegacy(appKey, password))
}

// HashPassword derives a stored password hash from appKey and password using
// Argon2id over an APP_KEY-keyed pepper. The returned string carries its own
// parameters and salt, so hashes written today still verify after a parameter
// change.
//
// Format: argon2id$<m>$<t>$<p>$<salt-b64>$<key-b64>
//
// It panics on a non-hex appKey.
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

// VerifyPassword reports whether password matches storedHash under appKey. A
// value starting with argonPrefix is checked with the parameters recorded in it;
// anything else is treated as a legacy HMAC hex string. Both comparisons are
// constant-time. It panics on a non-hex appKey.
func VerifyPassword(appKey, password, storedHash string) bool {
	if strings.HasPrefix(storedHash, argonPrefix) {
		return verifyArgon(appKey, password, storedHash)
	}
	got := hashPasswordLegacy(appKey, password)
	// Compare the hex strings rather than decoded bytes, so a stored value that
	// is not valid hex cannot decode to empty and match another empty.
	return hmac.Equal([]byte(got), []byte(storedHash))
}

func verifyArgon(appKey, password, storedHash string) bool {
	m, t, p, salt, want, ok := parseArgon(storedHash)
	if !ok {
		return false
	}
	// parseArgon refuses an empty key; the upper bound keeps the uint32
	// conversion from overflowing.
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
	// argon2.IDKey panics on a zero time or thread count, and an empty key
	// would match any password.
	if mv == 0 || tv == 0 || pv == 0 || len(salt) == 0 || len(key) == 0 {
		return 0, 0, 0, nil, nil, false
	}
	return uint32(mv), uint32(tv), uint8(pv), salt, key, true
}

// NeedsRehash reports whether storedHash should be replaced with a freshly
// derived one: every legacy HMAC value, and an Argon2id value whose recorded
// parameters are weaker than the current ones. The login handler rehashes after
// a successful login, the only moment the plaintext password is available.
func NeedsRehash(storedHash string) bool {
	if !strings.HasPrefix(storedHash, argonPrefix) {
		return true
	}
	m, t, p, _, key, ok := parseArgon(storedHash)
	if !ok {
		// An unparseable value never verifies, so no login could reach the
		// rehash anyway.
		return false
	}
	return m < argonMemoryKiB || t < argonTime || p < argonThreads || len(key) < argonKeyLen
}

// MinPasswordLen is the shortest password the settings page will store. The
// login throttle allows 5 attempts a minute, about 7200 a day, which exhausts a
// four-digit PIN in under two days; twelve characters put brute force out of
// reach regardless of the throttle.
//
// It applies when a password is set, not when one is verified, so an existing
// shorter password keeps working and its owner is asked to change it instead of
// being locked out.
const MinPasswordLen = 12
