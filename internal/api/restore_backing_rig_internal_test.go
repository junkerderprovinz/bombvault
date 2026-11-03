package api

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/qcow2"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// qcow2Header is a qcow2 header naming backing, as much of an image as the
// chain needs.
func qcow2Header(backing string) string {
	b := make([]byte, 512)
	be := binary.BigEndian
	be.PutUint32(b[0:], 0x514649fb)
	be.PutUint32(b[4:], 3)
	be.PutUint32(b[20:], 16)
	be.PutUint32(b[100:], 104)
	if backing != "" {
		be.PutUint64(b[8:], 112)
		be.PutUint32(b[16:], uint32(len(backing))) //nolint:gosec // G115: a short test string
		copy(b[112:], backing)
	}
	return string(b)
}

const labXML = `<domain type='kvm'>
  <name>lab</name>
  <uuid>0b6c1a52-8d0e-4c1b-9f0e-3a1d2c4b5e6f</uuid>
  <devices>
    <disk type='file' device='disk'>
      <driver name='qemu' type='qcow2'/>
      <source file='/mnt/user/domains/lab/vdisk1.snap.qcow2'/>
      <target dev='vda' bus='virtio'/>
    </disk>
  </devices>
</domain>`

// seedLab puts a shut-off VM on the rig whose disk is an Unraid snapshot on
// top of an overlay on top of a template in another folder, and has the
// service back it up.
func (r *restoreRig) seedLab(t *testing.T) (store.VMTarget, string) {
	t.Helper()
	r.write(t, map[string]string{
		"domains/templates/base.qcow2":  qcow2Header(""),
		"domains/lab/vdisk1.qcow2":      qcow2Header("/mnt/user/domains/templates/base.qcow2"),
		"domains/lab/vdisk1.snap.qcow2": qcow2Header("vdisk1.qcow2"),
	})
	r.virsh.name, r.virsh.state, r.virsh.xml = "lab", "shut off", labXML
	if _, err := r.svc.BackupVM(context.Background(), "lab"); err != nil {
		t.Fatalf("seed backup: %v", err)
	}
	tg, err := r.st.GetVMTargetByName("lab")
	if err != nil {
		t.Fatal(err)
	}
	return tg, r.lastRun(t, tg.ID, "backup").SnapshotID
}

func (r *restoreRig) backingOf(t *testing.T, rel string) string {
	t.Helper()
	h, err := qcow2.ReadHeader(filepath.Join(r.root, rel))
	if err != nil {
		t.Fatal(err)
	}
	return h.BackingFile
}

func TestRestoreBesideVMLeadsTheCopiedChainThroughTheCopies(t *testing.T) {
	r := newRestoreRig(t)
	_, snap := r.seedLab(t)
	domains := filepath.Join(r.root, "domains")
	lab, templates := treeSum(t, filepath.Join(domains, "lab"), true), treeSum(t, filepath.Join(domains, "templates"), true)

	res := r.check(t, RestoreCheckRequest{Kind: checkVM, Name: "lab", SnapshotID: snap, Target: targetBeside})
	wantFolders := []BesideFolder{
		{Original: "/mnt/user/domains/lab", Copy: "/mnt/user/domains/lab-restored"},
		{Original: "/mnt/user/domains/templates", Copy: "/mnt/user/domains/templates-restored"},
	}
	if p := res.Plan; !res.Ready || p.Added != 3 || p.Beside == nil || !slices.Equal(p.Beside.Folders, wantFolders) {
		t.Fatalf("ready=%v plan = %+v, beside %+v, want all three files of the chain", res.Ready, p, p.Beside)
	}

	started, err := r.svc.StartRestoreVM(context.Background(), "lab", snap, "local", RestoreOptions{Beside: true})
	if err != nil || !started {
		t.Fatalf("StartRestoreVM: started=%v err=%v", started, err)
	}
	r.wait(t)

	if got := r.backingOf(t, "domains/lab-restored/vdisk1.qcow2"); got != "/mnt/user/domains/templates-restored/base.qcow2" {
		t.Fatalf("the copied overlay reads through to %q", got)
	}
	if got := r.backingOf(t, "domains/lab-restored/vdisk1.snap.qcow2"); got != "vdisk1.qcow2" {
		t.Fatalf("the copied snapshot reads through to %q", got)
	}
	if _, err := os.Stat(filepath.Join(domains, "templates-restored", "base.qcow2")); err != nil {
		t.Fatalf("the template was not copied: %v", err)
	}
	if treeSum(t, filepath.Join(domains, "lab"), true) != lab || treeSum(t, filepath.Join(domains, "templates"), true) != templates {
		t.Fatal("the original chain changed")
	}
}

func TestRestoreVMInPlaceBringsBackTheBackingFilesWhereTheyWere(t *testing.T) {
	r := newRestoreRig(t)
	tg, snap := r.seedLab(t)
	base := filepath.Join(r.root, "domains", "templates", "base.qcow2")
	if err := os.Remove(base); err != nil {
		t.Fatal(err)
	}

	res := r.check(t, RestoreCheckRequest{Kind: checkVM, Name: "lab", SnapshotID: snap})
	back := PlanFile{Path: "/mnt/user/domains/templates/base.qcow2", Change: changeAdded}
	if p := res.Plan; !res.Ready || !slices.Contains(p.Files, back) {
		t.Fatalf("ready=%v plan = %+v, want the missing template to come back", res.Ready, p)
	}

	started, err := r.svc.StartRestoreVM(context.Background(), "lab", snap, "local", RestoreOptions{})
	if err != nil || !started {
		t.Fatalf("StartRestoreVM: started=%v err=%v", started, err)
	}
	r.wait(t)

	if _, err := os.Stat(base); err != nil {
		t.Fatalf("the template did not come back: %v", err)
	}
	if got := r.backingOf(t, "domains/lab/vdisk1.qcow2"); got != "/mnt/user/domains/templates/base.qcow2" {
		t.Fatalf("a restore in place repointed the overlay to %q", got)
	}
	if calls := r.virsh.recorded(); !slices.Contains(calls, "define") {
		t.Fatalf("libvirt calls = %v, want the domain defined", calls)
	}
	if run := r.lastRun(t, tg.ID, "restore"); run.Status != "success" {
		t.Fatalf("run = %+v", run)
	}
}
