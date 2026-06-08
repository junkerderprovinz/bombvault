package config_test

import (
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
)

func TestLoadValidatesAppKey(t *testing.T) {
	_, err := config.Load(map[string]string{"APP_KEY": "short"})
	if err == nil {
		t.Fatal("expected error for short APP_KEY")
	}
	c, err := config.Load(map[string]string{"APP_KEY": strings.Repeat("a", 64)})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if c.HTTPSPort != 3443 {
		t.Fatalf("default HTTPSPort wrong: %d", c.HTTPSPort)
	}
}
