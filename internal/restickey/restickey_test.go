package restickey_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restickey"
)

func TestDerive(t *testing.T) {
	a := restickey.Derive(strings.Repeat("a", 64))
	if a != restickey.Derive(strings.Repeat("a", 64)) {
		t.Fatal("not deterministic")
	}
	if a == restickey.Derive(strings.Repeat("b", 64)) {
		t.Fatal("collision")
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(a) {
		t.Fatal("format")
	}
	key := strings.Repeat("a", 64)
	if restickey.Derive(key) == key {
		t.Fatal("derived key must differ from APP_KEY")
	}
}

// TestDeriveKnownVector pins the digest for a fixed key. Any change to the key
// decoding or the message would change every repository password.
func TestDeriveKnownVector(t *testing.T) {
	const want = "f54d5f7bf68f232848e306cf52feb689e4302e4864de8a049b4389a3f2e595c4"
	got := restickey.Derive(strings.Repeat("a", 64))
	if got != want {
		t.Fatalf("known-vector mismatch:\n got  %s\n want %s", got, want)
	}
}
