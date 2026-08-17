package virshcli_test

import (
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// TestParseDomainExcludesCDROM pins the live-backup fix: only writable file
// disks are snapshotted; a cdrom (and a read-only disk) go into SkipSnapshotDevs
// so the snapshot sets snapshot=no for them (snapshotting a cdrom fails with
// "external snapshot file ... already exists and is not a block device").
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
	// The cdrom and the read-only disk must be skipped in the snapshot.
	if !slices.Contains(d.SkipSnapshotDevs, "hdc") || !slices.Contains(d.SkipSnapshotDevs, "hdd") {
		t.Fatalf("SkipSnapshotDevs = %v (want hdc + hdd)", d.SkipSnapshotDevs)
	}
	if slices.Contains(d.SkipSnapshotDevs, "vda") {
		t.Fatalf("the writable disk must NOT be skipped: %v", d.SkipSnapshotDevs)
	}
	if d.NVRAMPath != "/etc/libvirt/qemu/nvram/Win_VARS.fd" {
		t.Fatalf("NVRAMPath = %q", d.NVRAMPath)
	}
}

// TestParseDomainExposesDiskDevSource pins the per-disk dev+source mapping used
// to detect a leftover BombVault overlay: a VM still running on a
// "*.bombvault-tmp" overlay (with the real qcow2 as its backingStore) must
// surface that disk's dev and the overlay source so the service can commit it.
// This mirrors manilx's Windows Server 2022 dump (writable disk on hdc).
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
	// The cdrom (no source) must not appear as a writable disk.
	if slices.Contains(d.SkipSnapshotDevs, "hdc") {
		t.Fatalf("the writable disk hdc must not be skipped: %v", d.SkipSnapshotDevs)
	}
}

// TestParseDomainDetectsBlockDeviceDisk pins Task 10's detection signal: a
// <disk> whose <source dev="..."> (not <source file="...">) is libvirt's OWN
// signal that the backing store is a raw block device — e.g. a TrueNAS Scale
// zvol, /dev/zvol/<pool>/<dataset>. It must surface in the new BlockDisks
// field (IsBlockDevice=true) WITHOUT appearing in DiskPaths/Disks/DiskDevice
// (those feed the existing file-copy backup path, which cannot handle a block
// device path) — this is the regression guard: file-backed (Unraid) disk
// parsing must stay byte-identical, so a block-device disk must be invisible
// to every field a pre-existing caller already reads. It IS added to
// SkipSnapshotDevs, preserving today's (pre-Task-10) behavior — before this
// field existed, a block-device disk already fell through to the
// "cdrom/read-only/source-less" skip branch below because it failed the
// file-only `writable` check; that side effect must be unchanged.
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
		t.Fatalf("DiskPaths = %v, want empty — a block-device disk must not enter the file-copy path", d.DiskPaths)
	}
	if len(d.Disks) != 0 {
		t.Fatalf("Disks = %v, want empty — a block-device disk must not enter the file-based live-snapshot bookkeeping", d.Disks)
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
	// Pre-existing behavior preserved: excluded from the live-snapshot overlay
	// (qemu's external-file snapshot cannot target a raw block device the same
	// way it does a qcow2 file).
	if !slices.Contains(d.SkipSnapshotDevs, "vda") {
		t.Fatalf("SkipSnapshotDevs = %v, want vda included", d.SkipSnapshotDevs)
	}
}

// TestParseDomainExtractsTitle pins the TrueNAS 26 support ParseDomain adds
// for Task 12: a domain XML carrying a <title> element (libvirt's own
// free-form display-name field, a direct child of <domain>) must surface it
// in DomainInfo.Title — the field Client.titleFromXML (virshcli.go) reads
// when a UUID-style domain name needs its friendly name resolved.
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

// TestParseDomainNoTitleElement is the explicit regression pin: a domain XML
// with no <title> element (every VM in production today, including TrueNAS
// 25.10's own id_name-named domains) must parse with Title empty — matching
// NVRAMPath/TPMPath's own "empty = nothing to report" convention — and every
// other field must stay unaffected by <title> parsing being added.
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

// TestParseDomainFileBackedDiskHasNoBlockDisks is the explicit regression pin:
// the existing file-backed fixtures (Unraid's own shape) must produce an
// EMPTY BlockDisks — the new field must never spuriously populate for a
// perfectly ordinary qcow2-backed VM.
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
