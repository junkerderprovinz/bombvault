package virshcli_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// A passthrough backend is the one <tpm> shape whose path ParseDomain trusts.
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
	if d.NVRAMPath != "/etc/libvirt/qemu/nvram/Win_VARS.fd" {
		t.Fatalf("NVRAMPath = %q (must be unaffected by TPM parsing)", d.NVRAMPath)
	}
}

// TrueNAS Scale provisions an emulated vTPM, whose state path libvirt does not
// put in the domain XML.
func TestParseDomainTPMEmulatorHasNoPath(t *testing.T) {
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

// An external backend's socket path is not the vTPM state.
func TestParseDomainTPMExternalBackendHasNoPath(t *testing.T) {
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
		t.Fatalf("TPMPath = %q, want empty (an external backend's socket path is not usable state, see tpm.go)", d.TPMPath)
	}
}

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
		t.Fatalf("TPMPath = %q, want empty; an unsafe device path must not be trusted", d.TPMPath)
	}
}

func TestTPMFixedPath(t *testing.T) {
	got := virshcli.TPMFixedPath("1", "myvm")
	want := "/var/db/system/vm/tpm/1_myvm_tpm_state"
	if got != want {
		t.Fatalf("TPMFixedPath(1, myvm) = %q, want %q", got, want)
	}
}
