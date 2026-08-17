package virshcli_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// TestParseDomainTPMPassthroughDiscoversPath pins the one <tpm> shape this
// package trusts: a passthrough backend's <device path='...'/> is a directly
// usable, safe absolute path, so ParseDomain must surface it in TPMPath —
// mirroring how NVRAMPath is read straight off <os><nvram>.
func TestParseDomainTPMPassthroughDiscoversPath(t *testing.T) {
	const xml = `
<domain type='kvm'>
  <devices>
    <disk type='file' device='disk'>
      <source file='/mnt/cache/vms/Win/vdisk1.img'/>
      <target dev='vda'/>
    </disk>
    <tpm model='tpm-tis'>
      <backend type='passthrough'>
        <device path='/dev/tpm0'/>
      </backend>
    </tpm>
  </devices>
  <os><nvram>/etc/libvirt/qemu/nvram/Win_VARS.fd</nvram></os>
</domain>`

	d, err := virshcli.ParseDomain(xml)
	if err != nil {
		t.Fatalf("ParseDomain: %v", err)
	}
	if d.TPMPath != "/dev/tpm0" {
		t.Fatalf("TPMPath = %q, want /dev/tpm0", d.TPMPath)
	}
	// NVRAM parsing must be completely unaffected by a <tpm> element being
	// present alongside it.
	if d.NVRAMPath != "/etc/libvirt/qemu/nvram/Win_VARS.fd" {
		t.Fatalf("NVRAMPath = %q (must be unaffected by TPM parsing)", d.NVRAMPath)
	}
}

// TestParseDomainTPMEmulatorDegradesCleanly is the realistic TrueNAS Scale
// vTPM shape: a software (emulated) TPM backend. Per libvirt's public
// documentation this backend type does not expose its swtpm state path as a
// domain-XML attribute, so BombVault must NOT guess one — TPMPath must come
// back empty (clean degrade to "no TPM to capture"), never an error and
// never an invented path.
func TestParseDomainTPMEmulatorDegradesCleanly(t *testing.T) {
	const xml = `
<domain type='kvm'>
  <devices>
    <disk type='file' device='disk'>
      <source file='/mnt/cache/vms/Win/vdisk1.img'/>
      <target dev='vda'/>
    </disk>
    <tpm model='tpm-crb'>
      <backend type='emulator' version='2.0'/>
    </tpm>
  </devices>
</domain>`

	d, err := virshcli.ParseDomain(xml)
	if err != nil {
		t.Fatalf("ParseDomain: %v (an unrecognized <tpm> shape must never error)", err)
	}
	if d.TPMPath != "" {
		t.Fatalf("TPMPath = %q, want empty (emulator backend carries no XML-discoverable path)", d.TPMPath)
	}
}

// TestParseDomainTPMExternalBackendDegradesCleanly covers the newer
// <backend type='external'> shape (an externally-managed swtpm connected via
// a UNIX socket) — plausibly how TrueNAS's own middleware wires its vTPM, but
// NOT confirmed against real hardware anywhere in this project (see tpm.go's
// package doc comment). Rather than guess at its exact attribute layout, this
// must degrade the same clean way the emulator case does.
func TestParseDomainTPMExternalBackendDegradesCleanly(t *testing.T) {
	const xml = `
<domain type='kvm'>
  <devices>
    <tpm model='tpm-crb'>
      <backend type='external'>
        <source type='unix' mode='connect'>
          <address type='unix' path='/var/db/system/vm/tpm/1_myvm_tpm_state/swtpm-sock'/>
        </source>
      </backend>
    </tpm>
  </devices>
</domain>`

	d, err := virshcli.ParseDomain(xml)
	if err != nil {
		t.Fatalf("ParseDomain: %v", err)
	}
	if d.TPMPath != "" {
		t.Fatalf("TPMPath = %q, want empty (external backend's socket path is not treated as usable state — see tpm.go)", d.TPMPath)
	}
}

// TestParseDomainNoTPMElement is the explicit regression pin: a domain with
// no <tpm> element at all (every VM in production today) must parse exactly
// as it did before TPM support existed — TPMPath empty, every other field
// unaffected.
func TestParseDomainNoTPMElement(t *testing.T) {
	const xml = `
<domain type='kvm'>
  <devices>
    <disk type='file' device='disk'>
      <source file='/mnt/cache/vms/Win/vdisk1.img'/>
      <target dev='vda'/>
    </disk>
  </devices>
  <os><nvram>/etc/libvirt/qemu/nvram/Win_VARS.fd</nvram></os>
</domain>`

	d, err := virshcli.ParseDomain(xml)
	if err != nil {
		t.Fatalf("ParseDomain: %v", err)
	}
	if d.TPMPath != "" {
		t.Fatalf("TPMPath = %q, want empty for a domain with no <tpm> element", d.TPMPath)
	}
	if d.NVRAMPath != "/etc/libvirt/qemu/nvram/Win_VARS.fd" {
		t.Fatalf("NVRAMPath = %q (must be unaffected)", d.NVRAMPath)
	}
	if len(d.DiskPaths) != 1 || d.DiskPaths[0] != "/mnt/cache/vms/Win/vdisk1.img" {
		t.Fatalf("DiskPaths = %v (must be unaffected)", d.DiskPaths)
	}
}

// TestParseDomainTPMPassthroughRejectsUnsafePath mirrors nvram.go's
// attribute-breaking-content discipline (TestEnsureNVRAMTemplate's "loader
// with an attribute-breaking char falls back" case): a device path carrying
// characters that would be unsafe to trust later (whitespace, quotes) must
// degrade to "" rather than being handed to a caller as a trusted path.
func TestParseDomainTPMPassthroughRejectsUnsafePath(t *testing.T) {
	const xml = `
<domain type='kvm'>
  <devices>
    <tpm model='tpm-tis'>
      <backend type='passthrough'>
        <device path='/dev/tpm0 &amp;&amp; rm -rf /'/>
      </backend>
    </tpm>
  </devices>
</domain>`

	d, err := virshcli.ParseDomain(xml)
	if err != nil {
		t.Fatalf("ParseDomain: %v", err)
	}
	if d.TPMPath != "" {
		t.Fatalf("TPMPath = %q, want empty — an unsafe device path must never be trusted", d.TPMPath)
	}
}

func TestTPMFixedPath(t *testing.T) {
	got := virshcli.TPMFixedPath("1", "myvm")
	want := "/var/db/system/vm/tpm/1_myvm_tpm_state"
	if got != want {
		t.Fatalf("TPMFixedPath(1, myvm) = %q, want %q", got, want)
	}
}
