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

func TestLoadLibvirtDefaults(t *testing.T) {
	c, err := config.Load(map[string]string{"APP_KEY": strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	if c.LibvirtHost != "host.docker.internal" {
		t.Errorf("LibvirtHost = %q, want host.docker.internal", c.LibvirtHost)
	}
	if c.LibvirtSSHUser != "root" {
		t.Errorf("LibvirtSSHUser = %q, want root", c.LibvirtSSHUser)
	}
}
