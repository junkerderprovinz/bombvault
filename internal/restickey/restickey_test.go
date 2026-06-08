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
	// derived key must not equal the raw APP_KEY
	key := strings.Repeat("a", 64)
	if restickey.Derive(key) == key {
		t.Fatal("derived key must differ from APP_KEY")
	}
}
