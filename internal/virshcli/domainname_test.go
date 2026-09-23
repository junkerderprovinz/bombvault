package virshcli

import "testing"

// Unraid domain names carry neither an id prefix nor a UUID and pass through.
func TestNormalizeDomainNameUnraidStyle(t *testing.T) {
	friendly, versioned26 := normalizeDomainName("Windows10")
	if friendly != "Windows10" {
		t.Fatalf("friendlyName = %q, want unchanged %q", friendly, "Windows10")
	}
	if versioned26 {
		t.Fatalf("isVersioned26Style = true, want false for a plain Unraid-style name")
	}
}

// TrueNAS 25.10 names domains "{id}_{name}"; the id prefix is stripped.
func TestNormalizeDomainNameTrueNAS2510Style(t *testing.T) {
	friendly, versioned26 := normalizeDomainName("1_debian")
	if friendly != "debian" {
		t.Fatalf("friendlyName = %q, want %q", friendly, "debian")
	}
	if versioned26 {
		t.Fatalf("isVersioned26Style = true, want false for the 25.10 id_name style")
	}
}

// TrueNAS 26 names domains by UUID and keeps the friendly name in the XML's
// <title>, which vmInfoFromNames looks up. normalizeDomainName only reports
// the shape and returns the UUID as the friendly name.
func TestNormalizeDomainNameTrueNAS26UUIDStyle(t *testing.T) {
	const uuid = "550e8400-e29b-41d4-a716-446655440000"
	friendly, versioned26 := normalizeDomainName(uuid)
	if !versioned26 {
		t.Fatalf("isVersioned26Style = false, want true for a UUID-shaped domain name")
	}
	if friendly != uuid {
		t.Fatalf("friendlyName = %q, want the UUID itself (%q) as the cheap-path fallback", friendly, uuid)
	}
}

// The id prefix must start the name; digits and an underscore further in are
// not one.
func TestNormalizeDomainNameIgnoresEmbeddedDigits(t *testing.T) {
	friendly, versioned26 := normalizeDomainName("my_2_vm")
	if friendly != "my_2_vm" {
		t.Fatalf("friendlyName = %q, want unchanged %q (must not false-positive as 25.10-style)", friendly, "my_2_vm")
	}
	if versioned26 {
		t.Fatalf("isVersioned26Style = true, want false for %q", "my_2_vm")
	}
}

func TestNormalizeDomainNameEmptyString(t *testing.T) {
	friendly, versioned26 := normalizeDomainName("")
	if friendly != "" {
		t.Fatalf("friendlyName = %q, want empty", friendly)
	}
	if versioned26 {
		t.Fatalf("isVersioned26Style = true, want false for an empty raw name")
	}
}
