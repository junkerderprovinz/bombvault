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
