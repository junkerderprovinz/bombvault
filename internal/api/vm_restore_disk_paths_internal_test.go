package api

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// A disk the restore cannot reach stops it, the way the backup stops for the
// same disk. Leaving it out and defining the VM from an XML that still lists it
// would hand back a machine with a stale disk and say nothing.
func TestPrepareRestoreVMRefusesADiskOutsideTheHostMount(t *testing.T) {
	const stray = "/var/lib/libvirt/images/data.qcow2"
	eng := &foreignRecordingEngine{snaps: []restic.Snapshot{
		{ID: "deadbeef12345678", Tags: []string{"vm:win11"}, Paths: []string{win11Dir + "/vdisk1.img"}},
	}}
	s := vmRestoreSvc(t, eng)
	repoDir := filepath.Join(t.TempDir(), "repo")
	seedResticRepoDir(t, repoDir)
	tg := vmTargetJSON(t, "win11", win11DomainXML, []string{win11Dir + "/vdisk1.img", stray}, "")

	_, err := s.prepareRestoreVMForTarget(context.Background(),
		repoRef{repo: repoDir}, "win11", "latest", tg, tagIdentity("vm:win11"), "", "")
	if err == nil {
		t.Fatal("want a refusal naming the disk that cannot be reached")
	}
	if !strings.Contains(err.Error(), stray) {
		t.Fatalf("error = %v, want it to name %s", err, stray)
	}
	if len(eng.restores) != 0 {
		t.Fatalf("nothing may be restored when the refusal fires, got %v", eng.restores)
	}
}

// A cross-instance restore puts every disk in one folder, so two disks that
// share a file name would land on each other.
func TestPrepareRestoreVMCrossInstanceRefusesTwoDisksWithOneName(t *testing.T) {
	const other = "/host/user/domains/win11-extra/vdisk1.img"
	eng := &foreignRecordingEngine{snaps: []restic.Snapshot{
		{ID: "deadbeef12345678", Tags: []string{"vm:win11"}, Paths: []string{win11Dir + "/vdisk1.img", other}},
	}}
	s := vmRestoreSvc(t, eng)
	repoDir := filepath.Join(t.TempDir(), "repo")
	seedResticRepoDir(t, repoDir)
	tg := vmTargetJSON(t, "win11", win11DomainXML, []string{win11Dir + "/vdisk1.img", other}, "")

	_, err := s.prepareRestoreVMForTarget(context.Background(),
		repoRef{repo: repoDir}, "win11", "latest", tg, tagIdentity("vm:win11"), "/host/user/vmrestore", "")
	if err == nil {
		t.Fatal("want a refusal for two disks that would be restored over each other")
	}
	if !strings.Contains(err.Error(), "vdisk1.img") {
		t.Fatalf("error = %v, want it to name the disk both would be written as", err)
	}
	if len(eng.restores) != 0 {
		t.Fatalf("nothing may be restored when the refusal fires, got %v", eng.restores)
	}
}
