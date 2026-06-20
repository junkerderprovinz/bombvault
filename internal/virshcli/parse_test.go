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
