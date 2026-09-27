package secret

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
)

// Prefixes of what the OAuth side of the MCP endpoint hands out. They make a
// leaked value recognisable to people and to secret scanners, and they let the
// endpoint tell an access token from an MCP key before it looks anything up.
const (
	OAuthAccessPrefix  = "bvat_"
	OAuthRefreshPrefix = "bvrt_"
	OAuthCodePrefix    = "bvac_"
	OAuthSecretPrefix  = "bvcs_"
	OAuthClientPrefix  = "bvc_"
)

// NewOAuthSecret returns prefix followed by 32 random bytes in unpadded
// base64url.
func NewOAuthSecret(prefix string) (string, error) {
	buf := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", fmt.Errorf("secret: oauth: %w", err)
	}
	return prefix + base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashOAuthSecret returns the HMAC-SHA256 of value under appKey, separated by
// kind, so an access token's digest can never stand in for a refresh token's.
// Like an MCP key, a 256-bit random value needs no slow derivation.
func HashOAuthSecret(appKey, kind, value string) string {
	return mcpHMAC(appKey, "bombvault-oauth-"+kind+":"+value)
}

// ValidPKCEVerifier reports whether v has the shape RFC 7636 section 4.1 gives
// a code verifier: 43 to 128 unreserved characters.
func ValidPKCEVerifier(v string) bool {
	if len(v) < 43 || len(v) > 128 {
		return false
	}
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		case c == '-', c == '.', c == '_', c == '~':
		default:
			return false
		}
	}
	return true
}

// PKCEMatches reports whether verifier hashes to challenge under the S256
// method, the only one the server accepts.
func PKCEMatches(verifier, challenge string) bool {
	sum := sha256.Sum256([]byte(verifier))
	got := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(got), []byte(challenge)) == 1
}
