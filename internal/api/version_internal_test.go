package api

import "testing"

// A stamped build shows its version after the app name, an unstamped one "(dev)".
func TestVersionTag(t *testing.T) {
	orig := Version
	defer func() { Version = orig }()

	Version = "v2.7.1"
	if got := versionTag(); got != " v2.7.1" {
		t.Fatalf("stamped versionTag = %q, want %q", got, " v2.7.1")
	}
	for _, dev := range []string{"", "dev"} {
		Version = dev
		if got := versionTag(); got != " (dev)" {
			t.Fatalf("versionTag(%q) = %q, want %q", dev, got, " (dev)")
		}
	}
}
