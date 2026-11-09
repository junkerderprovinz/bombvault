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

func TestHostIsStableForAKeyAndDiffersBetweenKeys(t *testing.T) {
	a := restickey.Host(strings.Repeat("a", 64))
	if a != restickey.Host(strings.Repeat("a", 64)) {
		t.Fatal("the same key gave two hosts")
	}
	if a == restickey.Host(strings.Repeat("b", 64)) {
		t.Fatal("two keys gave the same host")
	}
	if !regexp.MustCompile(`^bv-[0-9a-f]{12}$`).MatchString(a) {
		t.Fatalf("host %q is not bv- and twelve hex characters", a)
	}
	if strings.Contains(restickey.Derive(strings.Repeat("a", 64)), strings.TrimPrefix(a, "bv-")) {
		t.Fatal("the host gives away part of the repository password")
	}
}

// The documentation shows how to work the host out with openssl, and a change
// here would leave a server's history under another host.
func TestHostKnownVector(t *testing.T) {
	if got := restickey.Host(strings.Repeat("a", 64)); got != "bv-6f81fda40ac6" {
		t.Fatalf("Host = %q, want bv-6f81fda40ac6", got)
	}
}
