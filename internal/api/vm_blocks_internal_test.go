package api

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/sshconn"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

var _ unixForwarder = (*sshconn.Conn)(nil)

func TestBlockRestoreImagesMatchesByPathThenByName(t *testing.T) {
	m := backup.BlocksManifest{Disks: []backup.BlocksDisk{
		{Dev: "vda", Path: "/host/user/domains/win/vdisk1.qcow2"},
		{Dev: "vdb", Path: "/host/user/domains/old-name/data.qcow2"},
	}}
	def := []string{"/host/user/domains/win/data.qcow2", "/host/user/domains/win/vdisk1.qcow2"}
	idx, err := blockRestoreImages(m, def)
	if err != nil {
		t.Fatal(err)
	}
	if idx[0] != 1 || idx[1] != 0 {
		t.Fatalf("idx = %v, want [1 0]", idx)
	}
}

func TestBlockRestoreImagesRefusesDiskTheVMNoLongerHas(t *testing.T) {
	m := backup.BlocksManifest{Disks: []backup.BlocksDisk{{Dev: "vdb", Path: "/host/user/domains/win/gone.qcow2"}}}
	if _, err := blockRestoreImages(m, []string{"/host/user/domains/win/vdisk1.qcow2"}); err == nil {
		t.Fatal("a disk missing from the definition was matched")
	}
	m.Disks[0].Path = "/host/user/domains/old/vdisk1.qcow2"
	two := []string{"/host/user/domains/a/vdisk1.qcow2", "/host/user/domains/b/vdisk1.qcow2"}
	if _, err := blockRestoreImages(m, two); err == nil {
		t.Fatal("an ambiguous file name was matched")
	}
}

func TestPlanBlockBackupNeedsTheBackupAPI(t *testing.T) {
	s := &Service{virsh: &scriptedVirsh{}}
	domain := virshcli.DomainInfo{Disks: []virshcli.DiskRef{{Dev: "vda", Source: "/mnt/user/domains/win/vdisk1.qcow2", Format: "qcow2"}}}
	if got := s.planBlockBackup(t.Context(), "win", domain, []string{"/host/user/domains/win/vdisk1.qcow2"}, 0); got.reason != blocksReasonUnsupported || got.host != nil {
		t.Fatalf("plan = %+v, want the classic path", got)
	}
}
