package virshcli_test

import (
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// Snapshotting a cdrom fails with "external snapshot file ... already exists
// and is not a block device", so only writable file disks are snapshotted.
func TestParseDomainExcludesCDROM(t *testing.T) {
	const xml = `
<domain type='kvm'>
  <devices>
    <disk type='file' device='disk'>
      <source file='/mnt/cache/vms/Win/vdisk1.img'/>
      <target dev='vda'/>
    </disk>
    <disk type='file' device='cdrom'>
      <source file='/mnt/cache/iso/virtio.iso'/>
      <target dev='hdc'/>
      <readonly/>
    </disk>
    <disk type='file' device='disk'>
      <source file='/mnt/cache/iso/windows.iso'/>
      <target dev='hdd'/>
      <readonly/>
    </disk>
  </devices>
  <os><nvram>/etc/libvirt/qemu/nvram/Win_VARS.fd</nvram></os>
</domain>`

	d, err := virshcli.ParseDomain(xml)
	if err != nil {
		t.Fatalf("ParseDomain: %v", err)
	}
	if len(d.DiskPaths) != 1 || d.DiskPaths[0] != "/mnt/cache/vms/Win/vdisk1.img" {
		t.Fatalf("DiskPaths = %v (want only the writable disk)", d.DiskPaths)
	}
	if d.DiskDevice != "vda" {
		t.Fatalf("DiskDevice = %q (want vda)", d.DiskDevice)
	}
	if !slices.Contains(d.SkipSnapshotDevs, "hdc") || !slices.Contains(d.SkipSnapshotDevs, "hdd") {
		t.Fatalf("SkipSnapshotDevs = %v (want hdc + hdd)", d.SkipSnapshotDevs)
	}
	if slices.Contains(d.SkipSnapshotDevs, "vda") {
		t.Fatalf("the writable disk must not be skipped: %v", d.SkipSnapshotDevs)
	}
	if d.NVRAMPath != "/etc/libvirt/qemu/nvram/Win_VARS.fd" {
		t.Fatalf("NVRAMPath = %q", d.NVRAMPath)
	}
}

// A VM still running on a "*.bombvault-tmp" overlay must surface that disk's
// dev and the overlay source so the service can commit it. The fixture is a
// Windows Server 2022 domain with its writable disk on hdc.
func TestParseDomainExposesDiskDevSource(t *testing.T) {
	const xml = `
<domain type='kvm'>
  <devices>
    <disk type='file' device='disk'>
      <driver name='qemu' type='qcow2'/>
      <source file='/mnt/user/domains/WinSrv/vdisk1.bombvault-tmp'/>
      <backingStore type='file'>
        <format type='qcow2'/>
        <source file='/mnt/user/domains/WinSrv/vdisk1.qcow2'/>
        <backingStore/>
      </backingStore>
      <target dev='hdc' bus='virtio'/>
    </disk>
    <disk type='file' device='cdrom'>
      <target dev='hdb' bus='sata'/>
      <readonly/>
    </disk>
  </devices>
</domain>`

	d, err := virshcli.ParseDomain(xml)
	if err != nil {
		t.Fatalf("ParseDomain: %v", err)
	}
	if len(d.Disks) != 1 {
		t.Fatalf("Disks = %v (want exactly the one writable disk)", d.Disks)
	}
	if d.Disks[0].Dev != "hdc" {
		t.Fatalf("Disks[0].Dev = %q (want hdc)", d.Disks[0].Dev)
	}
	if d.Disks[0].Source != "/mnt/user/domains/WinSrv/vdisk1.bombvault-tmp" {
		t.Fatalf("Disks[0].Source = %q (want the live overlay file)", d.Disks[0].Source)
	}
	if slices.Contains(d.SkipSnapshotDevs, "hdc") {
		t.Fatalf("the writable disk hdc must not be skipped: %v", d.SkipSnapshotDevs)
	}
}

// A disk with <source dev="..."> is a raw block device such as a TrueNAS zvol.
// It belongs in BlockDisks only, since the file-copy backup that reads
// DiskPaths, Disks and DiskDevice cannot handle a device path, and it stays
// out of the live snapshot.
func TestParseDomainDetectsBlockDeviceDisk(t *testing.T) {
	const xmlStr = `
<domain type='kvm'>
  <devices>
    <disk type='block' device='disk'>
      <driver name='qemu' type='raw'/>
      <source dev='/dev/zvol/tank/vms/truenasvm/disk0'/>
      <target dev='vda' bus='virtio'/>
    </disk>
  </devices>
</domain>`

	d, err := virshcli.ParseDomain(xmlStr)
	if err != nil {
		t.Fatalf("ParseDomain: %v", err)
	}
	if len(d.DiskPaths) != 0 {
		t.Fatalf("DiskPaths = %v, want empty: a block-device disk must not enter the file-copy path", d.DiskPaths)
	}
	if len(d.Disks) != 0 {
		t.Fatalf("Disks = %v, want empty: a block-device disk must not enter the file-based live-snapshot bookkeeping", d.Disks)
	}
	if d.DiskDevice != "" {
		t.Fatalf("DiskDevice = %q, want empty (no file-backed writable disk in this fixture)", d.DiskDevice)
	}
	if len(d.BlockDisks) != 1 {
		t.Fatalf("BlockDisks = %v, want exactly one block-device disk", d.BlockDisks)
	}
	bd := d.BlockDisks[0]
	if !bd.IsBlockDevice {
		t.Fatalf("BlockDisks[0].IsBlockDevice = false, want true")
	}
	if bd.Dev != "vda" {
		t.Fatalf("BlockDisks[0].Dev = %q, want vda", bd.Dev)
	}
	if bd.Source != "/dev/zvol/tank/vms/truenasvm/disk0" {
		t.Fatalf("BlockDisks[0].Source = %q, want the zvol dev path", bd.Source)
	}
	// qemu's external-file snapshot cannot target a raw block device.
	if !slices.Contains(d.SkipSnapshotDevs, "vda") {
		t.Fatalf("SkipSnapshotDevs = %v, want vda included", d.SkipSnapshotDevs)
	}
}

// TrueNAS 26 keeps the VM's own name in <title>, which List reads for a
// UUID-named domain.
func TestParseDomainExtractsTitle(t *testing.T) {
	const xml = `
<domain type='kvm'>
  <title>my-debian-vm</title>
  <devices>
    <disk type='file' device='disk'>
      <source file='/mnt/cache/vms/Win/vdisk1.img'/>
      <target dev='vda'/>
    </disk>
  </devices>
</domain>`

	d, err := virshcli.ParseDomain(xml)
	if err != nil {
		t.Fatalf("ParseDomain: %v", err)
	}
	if d.Title != "my-debian-vm" {
		t.Fatalf("Title = %q, want my-debian-vm", d.Title)
	}
}

func TestParseDomainNoTitleElement(t *testing.T) {
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
	if d.Title != "" {
		t.Fatalf("Title = %q, want empty for a domain with no <title> element", d.Title)
	}
	if d.NVRAMPath != "/etc/libvirt/qemu/nvram/Win_VARS.fd" {
		t.Fatalf("NVRAMPath = %q (must be unaffected by <title> parsing)", d.NVRAMPath)
	}
	if len(d.DiskPaths) != 1 || d.DiskPaths[0] != "/mnt/cache/vms/Win/vdisk1.img" {
		t.Fatalf("DiskPaths = %v (must be unaffected)", d.DiskPaths)
	}
}

// Two spellings of one UUID must never read as two VMs.
func TestParseDomainExtractsUUID(t *testing.T) {
	const xml = `
<domain type='kvm'>
  <uuid>  4A9B3FA1-4E77-4F2A-9C1F-2B6E9D1A7C33  </uuid>
  <devices>
    <disk type='file' device='disk'>
      <source file='/mnt/cache/vms/Win/vdisk1.img'/>
      <target dev='vda'/>
    </disk>
  </devices>
</domain>`

	d, err := virshcli.ParseDomain(xml)
	if err != nil {
		t.Fatalf("ParseDomain: %v", err)
	}
	if d.UUID != "4a9b3fa1-4e77-4f2a-9c1f-2b6e9d1a7c33" {
		t.Fatalf("UUID = %q, want lower-cased and trimmed", d.UUID)
	}
}

func TestParseDomainNoUUIDElement(t *testing.T) {
	const xml = `
<domain type='kvm'>
  <devices>
    <disk type='file' device='disk'>
      <source file='/mnt/cache/vms/Win/vdisk1.img'/>
      <target dev='vda'/>
    </disk>
  </devices>
</domain>`

	d, err := virshcli.ParseDomain(xml)
	if err != nil {
		t.Fatalf("ParseDomain: %v", err)
	}
	if d.UUID != "" {
		t.Fatalf("UUID = %q, want empty for a domain with no <uuid> element", d.UUID)
	}
}

func TestParseDomainFileBackedDiskHasNoBlockDisks(t *testing.T) {
	const xmlStr = `
<domain type='kvm'>
  <devices>
    <disk type='file' device='disk'>
      <source file='/mnt/cache/vms/Win/vdisk1.img'/>
      <target dev='vda'/>
    </disk>
  </devices>
</domain>`

	d, err := virshcli.ParseDomain(xmlStr)
	if err != nil {
		t.Fatalf("ParseDomain: %v", err)
	}
	if len(d.BlockDisks) != 0 {
		t.Fatalf("BlockDisks = %v, want empty for an all-file-backed domain", d.BlockDisks)
	}
}
