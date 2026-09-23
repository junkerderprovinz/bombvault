package secret

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
)

// MCPKeyPrefix makes a BombVault MCP key recognisable to people and to secret
// scanners.
const MCPKeyPrefix = "bvmcp_"

// Domain separators keep the two digests of one key apart: a stolen check value
// must not let anyone recompute the digest that authenticates a request.
const (
	mcpKeyDomain   = "bombvault-mcp-key:"
	mcpCheckDomain = "bombvault-mcp-check:"
)

// NewMCPKey returns MCPKeyPrefix followed by 32 random bytes in unpadded
// base64url, 49 characters in all.
func NewMCPKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", fmt.Errorf("secret: mcp key: %w", err)
	}
	return MCPKeyPrefix + base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashMCPKey returns the lowercase hex HMAC-SHA256 of key under appKey. A 256-bit
// random key needs no slow derivation, and the pepper means a copied database
// alone cannot confirm a guess.
func HashMCPKey(appKey, key string) string {
	return mcpHMAC(appKey, mcpKeyDomain+key)
}

// MCPKeyHint returns the last four characters of key, empty for shorter input.
func MCPKeyHint(key string) string {
	if len(key) < 4 {
		return ""
	}
	return key[len(key)-4:]
}

// MCPKeyCheck returns the lowercase hex HMAC-SHA256 of id under appKey. It is
// derived from the id alone, so a changed APP_KEY can be told from a wrong key
// without the server ever seeing the key again.
func MCPKeyCheck(appKey, id string) string {
	return mcpHMAC(appKey, mcpCheckDomain+id)
}

func mcpHMAC(appKey, msg string) string {
	mac := hmac.New(sha256.New, []byte(appKey))
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}
