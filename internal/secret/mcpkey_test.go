package secret

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

const mcpTestAppKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestNewMCPKeyFormat(t *testing.T) {
	const suffixChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		key, err := NewMCPKey()
		if err != nil {
			t.Fatalf("NewMCPKey: %v", err)
		}
		if !strings.HasPrefix(key, MCPKeyPrefix) {
			t.Fatalf("key %q has no %q prefix", key, MCPKeyPrefix)
		}
		if len(key) != 49 {
			t.Fatalf("len(%q) = %d, want 49", key, len(key))
		}
		for _, c := range strings.TrimPrefix(key, MCPKeyPrefix) {
			if !strings.ContainsRune(suffixChars, c) {
				t.Fatalf("key %q contains %q, which is not base64url", key, c)
			}
		}
		if seen[key] {
			t.Fatalf("NewMCPKey returned %q twice", key)
		}
		seen[key] = true
	}
}

func TestHashMCPKeyIsPepperedAndStable(t *testing.T) {
	const key = "bvmcp_5cVRBGBFbGnHmb2DPuBoDdM9zJkVtsvWMMpcNaq1Ozc"

	digest := HashMCPKey(mcpTestAppKey, key)
	if len(digest) != 64 {
		t.Fatalf("len(%q) = %d, want 64", digest, len(digest))
	}
	if strings.ToLower(digest) != digest {
		t.Fatalf("digest %q is not lowercase", digest)
	}
	if _, err := hex.DecodeString(digest); err != nil {
		t.Fatalf("digest %q is not hex: %v", digest, err)
	}
	if again := HashMCPKey(mcpTestAppKey, key); again != digest {
		t.Fatalf("second call gave %q, want %q", again, digest)
	}
	if other := HashMCPKey("f00dcafe", key); other == digest {
		t.Fatal("another app key gave the same digest")
	}
	if other := HashMCPKey(mcpTestAppKey, key+"x"); other == digest {
		t.Fatal("another key gave the same digest")
	}

	plain := sha256.Sum256([]byte(key))
	if digest == hex.EncodeToString(plain[:]) {
		t.Fatal("digest is the unpeppered SHA-256 of the key")
	}
}

func TestMCPKeyHint(t *testing.T) {
	cases := []struct {
		key  string
		want string
	}{
		{"bvmcp_5cVRBGBFbGnHmb2DPuBoDdM9zJkVtsvWMMpcNaq1Ozc", "1Ozc"},
		{"abcd", "abcd"},
		{"abc", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := MCPKeyHint(c.key); got != c.want {
			t.Fatalf("MCPKeyHint(%q) = %q, want %q", c.key, got, c.want)
		}
	}
}

func TestMCPKeyCheckDependsOnAppKeyAndIDOnly(t *testing.T) {
	const id = "4b1d0f4b1d0f4b1d0f4b1d0f4b1d0f4b"

	check := MCPKeyCheck(mcpTestAppKey, id)
	if len(check) != 64 {
		t.Fatalf("len(%q) = %d, want 64", check, len(check))
	}
	if _, err := hex.DecodeString(check); err != nil {
		t.Fatalf("check %q is not hex: %v", check, err)
	}
	if again := MCPKeyCheck(mcpTestAppKey, id); again != check {
		t.Fatalf("second call gave %q, want %q", again, check)
	}
	if other := MCPKeyCheck("f00dcafe", id); other == check {
		t.Fatal("another app key gave the same check")
	}
	if other := MCPKeyCheck(mcpTestAppKey, id+"0"); other == check {
		t.Fatal("another id gave the same check")
	}
	if HashMCPKey(mcpTestAppKey, id) == check {
		t.Fatal("check and digest share their domain separator")
	}
}
