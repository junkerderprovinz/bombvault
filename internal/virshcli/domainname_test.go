package virshcli

import "testing"

// TestNormalizeDomainNameUnraidStyle pins the no-op path: an ordinary
// Unraid-chosen domain name (never id-prefixed, never a UUID) must pass
// through completely unchanged — this is the regression guard that makes
// normalizeDomainName "harmless on Unraid" per its own doc comment.
func TestNormalizeDomainNameUnraidStyle(t *testing.T) {
	friendly, versioned26 := normalizeDomainName("Windows10")
	if friendly != "Windows10" {
		t.Fatalf("friendlyName = %q, want unchanged %q", friendly, "Windows10")
	}
	if versioned26 {
		t.Fatalf("isVersioned26Style = true, want false for a plain Unraid-style name")
	}
}

// TestNormalizeDomainNameTrueNAS2510Style pins TrueNAS 25.10 "Goldeye"'s
// libvirt domain-naming convention: "{id}_{name}". The numeric id prefix and
// its underscore must be stripped, leaving just the user-chosen name.
func TestNormalizeDomainNameTrueNAS2510Style(t *testing.T) {
	friendly, versioned26 := normalizeDomainName("1_debian")
	if friendly != "debian" {
		t.Fatalf("friendlyName = %q, want %q", friendly, "debian")
	}
	if versioned26 {
		t.Fatalf("isVersioned26Style = true, want false for the 25.10 id_name style")
	}
}

// TestNormalizeDomainNameTrueNAS26UUIDStyle pins TrueNAS 26's libvirt
// domain-naming convention: the domain name IS the VM's UUID, and the raw
// string carries no friendly-name information at all (that lives only in the
// domain XML's <title> element — see virshcli.go's Client.titleFromXML/
// vmInfoFromNames for the caller that resolves it). normalizeDomainName
// itself stays pure/XML-free, so its own contract is just: recognize the
// shape and signal isVersioned26Style=true, falling back to the UUID itself
// as friendlyName.
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

// TestNormalizeDomainNameDoesNotFalsePositiveOnEmbeddedDigits is the anchor
// regression pin: a name that merely CONTAINS a digit-run followed by an
// underscore somewhere is NOT the TrueNAS 25.10 "{id}_{name}" shape unless
// that run is the very START of the string — an ordinary Unraid-style name
// like "my_2_vm" must not be misparsed as id="my"... it simply doesn't start
// with digits, so it must pass through unchanged.
func TestNormalizeDomainNameDoesNotFalsePositiveOnEmbeddedDigits(t *testing.T) {
	friendly, versioned26 := normalizeDomainName("my_2_vm")
	if friendly != "my_2_vm" {
		t.Fatalf("friendlyName = %q, want unchanged %q (must not false-positive as 25.10-style)", friendly, "my_2_vm")
	}
	if versioned26 {
		t.Fatalf("isVersioned26Style = true, want false for %q", "my_2_vm")
	}
}

// TestNormalizeDomainNameEmptyString must not panic and must fall through to
// the Unraid-style (unchanged) branch — an empty raw name matches neither
// TrueNAS pattern.
func TestNormalizeDomainNameEmptyString(t *testing.T) {
	friendly, versioned26 := normalizeDomainName("")
	if friendly != "" {
		t.Fatalf("friendlyName = %q, want empty", friendly)
	}
	if versioned26 {
		t.Fatalf("isVersioned26Style = true, want false for an empty raw name")
	}
}
