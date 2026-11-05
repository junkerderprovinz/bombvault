package virshcli

import (
	"slices"
	"strings"
	"testing"
)

func TestRewriteBlockSourcesMovesOnlyTheMappedZvols(t *testing.T) {
	xml := `<disk type='block'><source dev='/dev/zvol/tank/vms/a'/></disk>` +
		`<disk type='block'><source dev="/dev/zvol/tank/vms/b"/></disk>` +
		`<disk type='file'><source file='/dev/zvol/tank/vms/a'/></disk>`
	got := RewriteBlockSources(xml, map[string]string{"/dev/zvol/tank/vms/a": "/dev/zvol/flash/vms/a"})
	want := `<disk type='block'><source dev='/dev/zvol/flash/vms/a'/></disk>` +
		`<disk type='block'><source dev="/dev/zvol/tank/vms/b"/></disk>` +
		`<disk type='file'><source file='/dev/zvol/tank/vms/a'/></disk>`
	if got != want {
		t.Fatalf("RewriteBlockSources =\n%s\nwant\n%s", got, want)
	}
}

const chainedDisk = `<domain>
  <devices>
    <disk type='file' device='disk'>
      <driver name='qemu' type='qcow2'/>
      <source file='/mnt/user/vm/vdisk1.S2.qcow2'/>
      <backingStore type='file'>
        <format type='qcow2'/>
        <source file='/mnt/user/vm/vdisk1.S1.qcow2'/>
        <backingStore type='file'>
          <format type='raw'/>
          <source file='/mnt/user/vm/vdisk1.img'/>
          <backingStore/>
        </backingStore>
      </backingStore>
      <target dev='vda'/>
    </disk>
    <disk type='file' device='disk'>
      <driver name='qemu' type='raw'/>
      <source file='/mnt/user/vm/data.img'/>
      <backingStore/>
      <target dev='vdb'/>
    </disk>
  </devices>
</domain>`

func TestStripBackingStoreRemovesTheWholeChainAndNothingElse(t *testing.T) {
	got := StripBackingStore(chainedDisk)
	if strings.Contains(got, "backingStore") || strings.Contains(got, "vdisk1.S1.qcow2") || strings.Contains(got, "vdisk1.img") {
		t.Fatalf("the chain is still there:\n%s", got)
	}
	d, err := ParseDomain(got)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"/mnt/user/vm/vdisk1.S2.qcow2", "/mnt/user/vm/data.img"}; !slices.Equal(d.DiskPaths, want) {
		t.Fatalf("disks = %v, want %v", d.DiskPaths, want)
	}
}

func TestRewriteDiskFormatChangesOnlyTheDiskAtThatSource(t *testing.T) {
	got := RewriteDiskFormat(StripBackingStore(chainedDisk), "/mnt/user/vm/data.img", "qcow2")
	d, err := ParseDomain(got)
	if err != nil {
		t.Fatal(err)
	}
	if d.Disks[0].Format != "qcow2" || d.Disks[1].Format != "qcow2" {
		t.Fatalf("formats = %+v", d.Disks)
	}
	if !strings.Contains(got, "name='qemu' type='qcow2'") {
		t.Fatalf("the driver name changed:\n%s", got)
	}
	back := RewriteDiskFormat(got, "/mnt/user/vm/vdisk1.S2.qcow2", "raw")
	if d, _ := ParseDomain(back); d.Disks[0].Format != "raw" || d.Disks[1].Format != "qcow2" {
		t.Fatalf("formats = %+v", d.Disks)
	}
}

func TestParseDomainNamesTheDisksAndTheTPMABackupLeavesOut(t *testing.T) {
	d, err := ParseDomain(`<domain><devices>
  <disk type='file' device='disk'><source file='/mnt/user/vm/vdisk1.img'/><target dev='vda'/></disk>
  <disk type='file' device='disk'><source file='/mnt/user/vm/ro.img'/><target dev='vdb'/><readonly/></disk>
  <disk type='network' device='disk'><source protocol='iscsi' name='iqn/1'/><target dev='vdc'/></disk>
  <disk type='volume' device='disk'><source pool='default' volume='v1'/><target dev='vdd'/></disk>
  <disk type='block' device='lun'><source dev='/dev/sdb'/><target dev='sda'/></disk>
  <disk type='file' device='cdrom'><source file='/mnt/user/isos/a.iso'/><target dev='hda'/><readonly/></disk>
  <disk type='file' device='floppy'><source file='/mnt/user/isos/a.img'/><target dev='fda'/></disk>
  <disk type='file' device='disk'><target dev='vde'/></disk>
  <tpm model='tpm-crb'><backend type='emulator' version='2.0'/></tpm>
</devices></domain>`)
	if err != nil {
		t.Fatal(err)
	}
	want := []LeftOutDisk{{"vdb", "read-only"}, {"vdc", "network"}, {"vdd", "volume"}, {"sda", "lun"}}
	if !slices.Equal(d.LeftOutDisks, want) {
		t.Fatalf("left out = %v, want %v", d.LeftOutDisks, want)
	}
	if !d.TPMLeftOut {
		t.Fatal("an emulated TPM has to count as left out")
	}
}

func TestParseDomainDoesNotCountAPassthroughTPMAsLeftOut(t *testing.T) {
	d, err := ParseDomain(`<domain><devices><tpm><backend type='passthrough'><device path='/dev/tpm0'/></backend></tpm></devices></domain>`)
	if err != nil {
		t.Fatal(err)
	}
	if d.TPMLeftOut || d.TPMPath != "/dev/tpm0" {
		t.Fatalf("left out = %v, path = %q", d.TPMLeftOut, d.TPMPath)
	}
}
