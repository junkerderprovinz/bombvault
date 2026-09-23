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

func TestLoadDataRootSegmentsDefault(t *testing.T) {
	c, err := config.Load(map[string]string{"APP_KEY": strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"appdata"}
	if len(c.DataRootSegments) != len(want) || c.DataRootSegments[0] != want[0] {
		t.Fatalf("DataRootSegments = %v, want %v", c.DataRootSegments, want)
	}
}

func TestLoadPlatformOverrideDefault(t *testing.T) {
	c, err := config.Load(map[string]string{"APP_KEY": strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	if c.PlatformOverride != "" {
		t.Fatalf("default PlatformOverride = %q, want \"\" (auto-detect)", c.PlatformOverride)
	}
}

// TestLoadPlatformOverridePassthrough checks that Load passes PLATFORM through
// as given; platform.Detect validates it.
func TestLoadPlatformOverridePassthrough(t *testing.T) {
	c, err := config.Load(map[string]string{
		"APP_KEY":  strings.Repeat("a", 64),
		"PLATFORM": "truenas",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.PlatformOverride != "truenas" {
		t.Fatalf("PlatformOverride = %q, want %q", c.PlatformOverride, "truenas")
	}
}

func TestLoadLibvirtURIDefault(t *testing.T) {
	c, err := config.Load(map[string]string{"APP_KEY": strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	if c.LibvirtURI != "" {
		t.Fatalf("default LibvirtURI = %q, want \"\" (build from LIBVIRT_HOST/USER/PORT)", c.LibvirtURI)
	}
}

func TestLoadLibvirtURIPassthrough(t *testing.T) {
	const uri = "qemu+ssh://root@truenas.local/system?socket=/run/truenas_libvirt/libvirt-sock"
	c, err := config.Load(map[string]string{
		"APP_KEY":     strings.Repeat("a", 64),
		"LIBVIRT_URI": uri,
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.LibvirtURI != uri {
		t.Fatalf("LibvirtURI = %q, want %q", c.LibvirtURI, uri)
	}
}

// TestLoadDataRootSegmentsParsesCommaSeparatedList covers trimming,
// lower-casing and dropping empty entries.
func TestLoadDataRootSegmentsParsesCommaSeparatedList(t *testing.T) {
	c, err := config.Load(map[string]string{
		"APP_KEY":            strings.Repeat("a", 64),
		"DATA_ROOT_SEGMENTS": " Appdata, Config ,,data",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"appdata", "config", "data"}
	if len(c.DataRootSegments) != len(want) {
		t.Fatalf("DataRootSegments = %v, want %v", c.DataRootSegments, want)
	}
	for i := range want {
		if c.DataRootSegments[i] != want[i] {
			t.Fatalf("DataRootSegments = %v, want %v", c.DataRootSegments, want)
		}
	}
}

func TestLoadDerivesSSHTargetFromLibvirtURIWhenUnset(t *testing.T) {
	c, err := config.Load(map[string]string{
		"APP_KEY":     strings.Repeat("a", 64),
		"LIBVIRT_URI": "qemu+ssh://truenas_admin@nas.lan:2222/system?socket=/run/truenas_libvirt/libvirt-sock",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.LibvirtHost != "nas.lan" || c.LibvirtSSHUser != "truenas_admin" || c.LibvirtSSHPort != "2222" {
		t.Fatalf("target = %s@%s:%s, want truenas_admin@nas.lan:2222", c.LibvirtSSHUser, c.LibvirtHost, c.LibvirtSSHPort)
	}
}

func TestLoadDerivesEachUnsetVariableSeparately(t *testing.T) {
	c, err := config.Load(map[string]string{
		"APP_KEY":      strings.Repeat("a", 64),
		"LIBVIRT_URI":  "qemu+ssh://truenas_admin@nas.lan:2222/system",
		"LIBVIRT_HOST": "10.0.0.5",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.LibvirtHost != "10.0.0.5" {
		t.Errorf("LibvirtHost = %q, want the value from the environment", c.LibvirtHost)
	}
	if c.LibvirtSSHUser != "truenas_admin" || c.LibvirtSSHPort != "2222" {
		t.Errorf("user/port = %s:%s, want them from the URI", c.LibvirtSSHUser, c.LibvirtSSHPort)
	}
}

func TestLoadExplicitLibvirtHostWinsOverURI(t *testing.T) {
	c, err := config.Load(map[string]string{
		"APP_KEY":          strings.Repeat("a", 64),
		"LIBVIRT_URI":      "qemu+ssh://truenas_admin@nas.lan:2222/system",
		"LIBVIRT_HOST":     "10.0.0.5",
		"LIBVIRT_SSH_USER": "root",
		"LIBVIRT_SSH_PORT": "1004",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.LibvirtHost != "10.0.0.5" || c.LibvirtSSHUser != "root" || c.LibvirtSSHPort != "1004" {
		t.Fatalf("target = %s@%s:%s, want the environment's own values", c.LibvirtSSHUser, c.LibvirtHost, c.LibvirtSSHPort)
	}
}

func TestLoadNonSSHURIKeepsDefaults(t *testing.T) {
	c, err := config.Load(map[string]string{
		"APP_KEY":     strings.Repeat("a", 64),
		"LIBVIRT_URI": "qemu:///system",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.LibvirtHost != "host.docker.internal" || c.LibvirtSSHUser != "root" || c.LibvirtSSHPort != "22" {
		t.Fatalf("target = %s@%s:%s, want the defaults", c.LibvirtSSHUser, c.LibvirtHost, c.LibvirtSSHPort)
	}
	if c.LibvirtURIHost != "" || c.LibvirtURIUser != "" {
		t.Errorf("URI target = %s@%s, want nothing", c.LibvirtURIUser, c.LibvirtURIHost)
	}
}

func TestLoadRecordsURITarget(t *testing.T) {
	c, err := config.Load(map[string]string{
		"APP_KEY":      strings.Repeat("a", 64),
		"LIBVIRT_URI":  "qemu+ssh://truenas_admin@nas.lan:2222/system",
		"LIBVIRT_HOST": "10.0.0.5",
	})
	if err != nil {
		t.Fatal(err)
	}
	// The probe reports uri-mismatch from these, so they stay even when the
	// environment wins.
	if c.LibvirtURIHost != "nas.lan" || c.LibvirtURIUser != "truenas_admin" {
		t.Fatalf("URI target = %s@%s, want truenas_admin@nas.lan", c.LibvirtURIUser, c.LibvirtURIHost)
	}
}

func TestLoadTreatsTemplatePlaceholderAsUnset(t *testing.T) {
	c, err := config.Load(map[string]string{
		"APP_KEY":      strings.Repeat("a", 64),
		"LIBVIRT_HOST": "192.168.x.x",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.LibvirtHost != "host.docker.internal" {
		t.Errorf("LibvirtHost = %q, want the default", c.LibvirtHost)
	}
	if !c.LibvirtHostWasPlaceholder {
		t.Error("LibvirtHostWasPlaceholder is false")
	}

	c, err = config.Load(map[string]string{
		"APP_KEY":      strings.Repeat("a", 64),
		"LIBVIRT_HOST": "192.168.x.x",
		"LIBVIRT_URI":  "qemu+ssh://truenas_admin@nas.lan:2222/system",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.LibvirtHost != "nas.lan" {
		t.Errorf("LibvirtHost = %q, want the URI's host", c.LibvirtHost)
	}

	c, err = config.Load(map[string]string{
		"APP_KEY":      strings.Repeat("a", 64),
		"LIBVIRT_HOST": "192.168.10.10",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.LibvirtHost != "192.168.10.10" || c.LibvirtHostWasPlaceholder {
		t.Errorf("a real address was taken for the placeholder: %q", c.LibvirtHost)
	}
}
