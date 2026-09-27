package virshcli

import (
	"strings"
	"testing"
)

func TestPullBackupXMLExportsOnlyBackedUpDisks(t *testing.T) {
	disks := []BackupDisk{
		{Dev: "vda", Scratch: "/mnt/user/domains/a & b/vda.bombvault-scratch"},
		{Dev: "sda", Skip: true},
	}
	got := PullBackupXML("/tmp/bombvault-nbd-x.sock", "bombvault-1", disks)
	for _, want := range []string{
		`<domainbackup mode="pull">`,
		"<incremental>bombvault-1</incremental>",
		`<server transport="unix" socket="/tmp/bombvault-nbd-x.sock"/>`,
		`<disk name="vda" backup="yes" type="file"><scratch file="/mnt/user/domains/a &amp; b/vda.bombvault-scratch"/></disk>`,
		`<disk name="sda" backup="no"/>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("backup xml lacks %s:\n%s", want, got)
		}
	}
	if strings.Contains(PullBackupXML("/s", "", disks), "incremental") {
		t.Error("a full backup names an incremental base")
	}
}

func TestCheckpointXMLTracksOnlyBackedUpDisks(t *testing.T) {
	got := CheckpointXML("bombvault-2", []BackupDisk{{Dev: "vda"}, {Dev: "hdc", Skip: true}})
	want := `<domaincheckpoint><name>bombvault-2</name><disks><disk name="vda" checkpoint="bitmap"/><disk name="hdc" checkpoint="no"/></disks></domaincheckpoint>`
	if got != want {
		t.Fatalf("got %s\nwant %s", got, want)
	}
}

func TestBackupJobSocketReadsTheServer(t *testing.T) {
	job := `<domainbackup mode='pull'>
  <incremental>bombvault-1</incremental>
  <server transport='unix' socket='/tmp/bombvault-nbd-x.sock'/>
  <disks><disk name='vda' backup='yes' type='file' exportname='vda' exportbitmap='backup-vda'/></disks>
</domainbackup>`
	if got := BackupJobSocket(job); got != "/tmp/bombvault-nbd-x.sock" {
		t.Fatalf("socket = %q", got)
	}
	if got := BackupJobSocket("<domainbackup mode='push'/>"); got != "" {
		t.Fatalf("push job socket = %q", got)
	}
}

func TestParseDomainKeepsDiskFormat(t *testing.T) {
	d, err := ParseDomain(`<domain><devices>
<disk type='file' device='disk'><driver name='qemu' type='qcow2'/><source file='/mnt/user/domains/x/vdisk1.qcow2'/><target dev='vda'/></disk>
<disk type='file' device='disk'><driver name='qemu' type='raw'/><source file='/mnt/user/domains/x/vdisk2.img'/><target dev='vdb'/></disk>
</devices></domain>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Disks) != 2 || d.Disks[0].Format != "qcow2" || d.Disks[1].Format != "raw" {
		t.Fatalf("disks = %+v", d.Disks)
	}
}
