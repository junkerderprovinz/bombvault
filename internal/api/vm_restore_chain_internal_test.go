package api

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// labDomain is a domain whose one disk is at source, as virsh shows it on a
// VM that keeps its chain in the definition.
func labDomain(format, source string, chain ...string) string {
	var backing strings.Builder
	for _, f := range chain {
		backing.WriteString("<backingStore type='file'><format type='qcow2'/><source file='" + f + "'/>")
	}
	if len(chain) > 0 {
		backing.WriteString("<backingStore/>")
	}
	backing.WriteString(strings.Repeat("</backingStore>", len(chain)))
	return `<domain type='kvm'>
  <name>lab</name>
  <uuid>0b6c1a52-8d0e-4c1b-9f0e-3a1d2c4b5e6f</uuid>
  <devices>
    <disk type='file' device='disk'>
      <driver name='qemu' type='` + format + `'/>
      <source file='` + source + `'/>
      ` + backing.String() + `
      <target dev='vda' bus='virtio'/>
    </disk>
  </devices>
</domain>`
}

const (
	labBaseHost    = "/mnt/user/domains/lab/vdisk1.qcow2"
	labOverlayHost = "/mnt/user/domains/lab/vdisk1.S1.qcow2"
)

// backupLab backs the VM up as it stands and returns the snapshot.
func (r *restoreRig) backupLab(t *testing.T) (store.VMTarget, string) {
	t.Helper()
	if _, err := r.svc.BackupVM(context.Background(), "lab"); err != nil {
		t.Fatalf("backup: %v", err)
	}
	tg, err := r.st.GetVMTargetByName("lab")
	if err != nil {
		t.Fatal(err)
	}
	return tg, r.lastRun(t, tg.ID, "backup").SnapshotID
}

// restoreLab restores and returns the run's outcome.
func (r *restoreRig) restoreLab(t *testing.T, tg store.VMTarget, snap string, opts RestoreOptions) store.Run {
	t.Helper()
	started, err := r.svc.StartRestoreVM(context.Background(), "lab", snap, "local", opts)
	if err != nil || !started {
		t.Fatalf("StartRestoreVM: started=%v err=%v", started, err)
	}
	r.wait(t)
	return r.lastRun(t, tg.ID, "restore")
}

func (r *restoreRig) read(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(r.root, rel)) //nolint:gosec // G304: a test file below the rig's root
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// definedDisk is the one disk of the domain the restore defined.
func (r *restoreRig) definedDisk(t *testing.T) virshcli.DiskRef {
	t.Helper()
	r.virsh.mu.Lock()
	xml := r.virsh.xml
	r.virsh.mu.Unlock()
	if strings.Contains(xml, "backingStore") {
		t.Fatalf("the definition still lists a chain:\n%s", xml)
	}
	d, err := virshcli.ParseDomain(xml)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Disks) != 1 {
		t.Fatalf("disks = %+v", d.Disks)
	}
	return d.Disks[0]
}

// seedSnapshotOverBase backs the VM up on its base, then again on an overlay
// taken later, and returns the first backup.
func (r *restoreRig) seedSnapshotOverBase(t *testing.T, stateA string) (store.VMTarget, string) {
	t.Helper()
	r.write(t, map[string]string{"domains/lab/vdisk1.qcow2": stateA})
	r.virsh.name, r.virsh.state, r.virsh.xml = "lab", "shut off", labDomain("qcow2", labBaseHost)
	tg, snapA := r.backupLab(t)

	r.write(t, map[string]string{"domains/lab/vdisk1.S1.qcow2": qcow2Header("vdisk1.qcow2") + "later"})
	r.virsh.xml = labDomain("qcow2", labOverlayHost, labBaseHost)
	r.backupLab(t)
	return tg, snapA
}

func TestRestoreVMFromBeforeASnapshotRunsOnTheDiskOfThatBackup(t *testing.T) {
	r := newRestoreRig(t)
	stateA := qcow2Header("") + "state A"
	tg, snapA := r.seedSnapshotOverBase(t, stateA)
	overlay := r.read(t, "domains/lab/vdisk1.S1.qcow2")
	r.write(t, map[string]string{"domains/lab/vdisk1.qcow2": qcow2Header("") + "base written since"})

	if run := r.restoreLab(t, tg, snapA, RestoreOptions{}); run.Status != "success" {
		t.Fatalf("run = %+v", run)
	}
	if d := r.definedDisk(t); d.Source != labBaseHost {
		t.Fatalf("the VM runs on %s, a file the backup does not hold", d.Source)
	}
	if got := r.read(t, "domains/lab/vdisk1.qcow2"); got != stateA {
		t.Fatalf("the base was not restored: %q", got[512:])
	}
	if got := r.read(t, "domains/lab/vdisk1.S1.qcow2"); got != overlay {
		t.Fatal("the overlay of the later state was changed")
	}
}

func TestRestoreVMBesideFromBeforeASnapshotCopiesTheDiskOfThatBackup(t *testing.T) {
	r := newRestoreRig(t)
	stateA := qcow2Header("") + "state A"
	tg, snapA := r.seedSnapshotOverBase(t, stateA)

	if run := r.restoreLab(t, tg, snapA, RestoreOptions{Beside: true}); run.Status != "success" {
		t.Fatalf("run = %+v", run)
	}
	if got := r.read(t, "domains/lab-restored/vdisk1.qcow2"); got != stateA {
		t.Fatalf("the copy holds %q", got[512:])
	}
	if _, err := os.Stat(filepath.Join(r.root, "domains/lab-restored/vdisk1.S1.qcow2")); err == nil {
		t.Fatal("a file the backup does not hold turned up in the copy")
	}
}

func TestRestoreVMInPlaceFromBeforeASnapshotNeedsNoOverlayOnTheHost(t *testing.T) {
	r := newRestoreRig(t)
	tg, snapA := r.seedSnapshotOverBase(t, qcow2Header("")+"state A")
	if err := os.Remove(filepath.Join(r.root, "domains/lab/vdisk1.S1.qcow2")); err != nil {
		t.Fatal(err)
	}

	if run := r.restoreLab(t, tg, snapA, RestoreOptions{}); run.Status != "success" {
		t.Fatalf("run = %+v", run)
	}
	if d := r.definedDisk(t); d.Source != labBaseHost {
		t.Fatalf("the VM runs on %s", d.Source)
	}
}

func TestRestoreVMWithAChainAfterTheChainWasMergedRunsOnTheOverlayOfThatBackup(t *testing.T) {
	r := newRestoreRig(t)
	base, overlay := qcow2Header("")+"base", qcow2Header("vdisk1.qcow2")+"overlay"
	r.write(t, map[string]string{"domains/lab/vdisk1.qcow2": base, "domains/lab/vdisk1.S1.qcow2": overlay})
	r.virsh.name, r.virsh.state, r.virsh.xml = "lab", "shut off", labDomain("qcow2", labOverlayHost, labBaseHost)
	tg, snap := r.backupLab(t)

	r.write(t, map[string]string{"domains/lab/vdisk1.qcow2": qcow2Header("") + "merged"})
	if err := os.Remove(filepath.Join(r.root, "domains/lab/vdisk1.S1.qcow2")); err != nil {
		t.Fatal(err)
	}
	r.virsh.xml = labDomain("qcow2", labBaseHost)
	r.backupLab(t)

	if run := r.restoreLab(t, tg, snap, RestoreOptions{}); run.Status != "success" {
		t.Fatalf("run = %+v", run)
	}
	if d := r.definedDisk(t); d.Source != labOverlayHost || d.Format != "qcow2" {
		t.Fatalf("the VM runs on %+v, without the overlay the backup holds", d)
	}
	if r.read(t, "domains/lab/vdisk1.qcow2") != base || r.read(t, "domains/lab/vdisk1.S1.qcow2") != overlay {
		t.Fatal("the chain did not come back as it was backed up")
	}
}

func TestRestoreVMFromBeforeASnapshotOfARawDiskDefinesItAsRaw(t *testing.T) {
	r := newRestoreRig(t)
	const raw = "/mnt/user/domains/lab/vdisk1.img"
	stateA := strings.Repeat("raw state A ", 100)
	r.write(t, map[string]string{"domains/lab/vdisk1.img": stateA})
	r.virsh.name, r.virsh.state, r.virsh.xml = "lab", "shut off", labDomain("raw", raw)
	tg, snapA := r.backupLab(t)

	r.write(t, map[string]string{"domains/lab/vdisk1.S1.qcow2": qcow2Header("vdisk1.img")})
	r.virsh.xml = labDomain("qcow2", labOverlayHost, raw)
	r.backupLab(t)

	if run := r.restoreLab(t, tg, snapA, RestoreOptions{}); run.Status != "success" {
		t.Fatalf("run = %+v", run)
	}
	if d := r.definedDisk(t); d.Source != raw || d.Format != "raw" {
		t.Fatalf("the VM runs on %+v", d)
	}
	if r.read(t, "domains/lab/vdisk1.img") != stateA {
		t.Fatal("the disk was not restored")
	}
}

func TestRestoreVMOfTheNewestBackupLeavesItsChainToTheImages(t *testing.T) {
	r := newRestoreRig(t)
	r.write(t, map[string]string{
		"domains/lab/vdisk1.qcow2":    qcow2Header("") + "base",
		"domains/lab/vdisk1.S1.qcow2": qcow2Header("vdisk1.qcow2") + "overlay",
	})
	r.virsh.name, r.virsh.state, r.virsh.xml = "lab", "shut off", labDomain("qcow2", labOverlayHost, labBaseHost)
	tg, snap := r.backupLab(t)

	if run := r.restoreLab(t, tg, snap, RestoreOptions{}); run.Status != "success" {
		t.Fatalf("run = %+v", run)
	}
	if d := r.definedDisk(t); d.Source != labOverlayHost {
		t.Fatalf("the VM runs on %s", d.Source)
	}
	if got := r.backingOf(t, "domains/lab/vdisk1.S1.qcow2"); got != "vdisk1.qcow2" {
		t.Fatalf("the overlay reads through to %q", got)
	}
}

func TestBackupVMNamesWhatItLeavesOutOnItsGreenRun(t *testing.T) {
	r := newRestoreRig(t)
	r.write(t, map[string]string{"domains/lab/vdisk1.img": "disk"})
	r.virsh.name, r.virsh.state = "lab", "shut off"
	r.virsh.xml = `<domain type='kvm'>
  <name>lab</name>
  <devices>
    <disk type='file' device='disk'><driver type='raw'/><source file='/mnt/user/domains/lab/vdisk1.img'/><target dev='vda'/></disk>
    <disk type='network' device='disk'><source protocol='nbd' name='export'/><target dev='vdb'/></disk>
    <disk type='file' device='disk'><source file='/mnt/user/domains/lab/ro.img'/><target dev='vdc'/><readonly/></disk>
    <disk type='file' device='cdrom'><source file='/mnt/user/isos/install.iso'/><target dev='hda'/><readonly/></disk>
    <tpm model='tpm-tis'><backend type='emulator' version='2.0' persistent_state='yes'/></tpm>
  </devices>
</domain>`
	tg, _ := r.backupLab(t)

	run := r.lastRun(t, tg.ID, "backup")
	want := store.NoteVMTPMLeftOut + "; " + store.NoteVMDisksLeftOut + ": vdb (network), vdc (read-only)"
	if run.Status != "success" || run.Error != want {
		t.Fatalf("run status %q, note %q, want %q", run.Status, run.Error, want)
	}
}

// headerEngine answers a dump with the qcow2 header of the file asked for.
type headerEngine struct {
	ResticEngine
	headers map[string]string
}

func (e headerEngine) DumpRaw(_ context.Context, _, _, file string, w io.Writer, _ restic.Mode) error {
	_, err := io.WriteString(w, e.headers[file])
	return err
}

const (
	backedDisk    = "/host/user/domains/lab/vdisk1.qcow2"
	backedOverlay = "/host/user/domains/lab/vdisk1.S1.qcow2"
	backedTop     = "/host/user/domains/lab/vdisk1.S2.qcow2"
)

func chainService(headers map[string]string) *Service {
	return &Service{
		cfg:    config.Config{HostMountRoot: "/host/user", HostSourceRoot: "/mnt/user"},
		engine: headerEngine{headers: headers},
	}
}

func TestDefinitionAsBackedUpRefusesAnOverlayWithoutItsBase(t *testing.T) {
	s := chainService(map[string]string{backedOverlay: qcow2Header("/mnt/user/domains/lab/vdisk1.qcow2")})
	def := vmDefinition{DomainXML: labDomain("qcow2", labBaseHost), DiskPaths: []string{backedDisk}}

	_, err := s.definitionAsBackedUp(t.Context(), repoRef{}, "abc", def, snapshotFiles{backedOverlay}, nil)
	var refusal *restoreDestErr
	if !errors.As(err, &refusal) || !strings.Contains(err.Error(), "vdisk1.S1.qcow2") {
		t.Fatalf("err = %v, want a refusal that names the overlay", err)
	}
}

func TestDefinitionAsBackedUpRefusesTwoOverlaysOnOneDisk(t *testing.T) {
	other := "/host/user/domains/lab/vdisk1.S9.qcow2"
	s := chainService(map[string]string{
		backedOverlay: qcow2Header("vdisk1.qcow2"),
		other:         qcow2Header("vdisk1.qcow2"),
	})
	def := vmDefinition{DomainXML: labDomain("qcow2", labBaseHost), DiskPaths: []string{backedDisk}}

	_, err := s.definitionAsBackedUp(t.Context(), repoRef{}, "abc", def, snapshotFiles{backedDisk, backedOverlay, other}, nil)
	var refusal *restoreDestErr
	if !errors.As(err, &refusal) {
		t.Fatalf("err = %v, want a refusal", err)
	}
}

func TestDefinitionAsBackedUpOneLevelUpKeepsTheLowerLink(t *testing.T) {
	s := chainService(map[string]string{backedOverlay: qcow2Header("vdisk1.qcow2")})
	def := vmDefinition{
		DomainXML:  labDomain("qcow2", "/mnt/user/domains/lab/vdisk1.S2.qcow2", labOverlayHost, labBaseHost),
		DiskPaths:  []string{backedTop},
		Backing:    []vmBacking{{Image: backedTop, File: backedOverlay}, {Image: backedOverlay, File: backedDisk}},
		SnapshotDB: []byte("the list of the newest backup"),
	}

	got, err := s.definitionAsBackedUp(t.Context(), repoRef{}, "abc", def, snapshotFiles{backedOverlay, backedDisk}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.DiskPaths, []string{backedOverlay}) {
		t.Fatalf("disks = %v", got.DiskPaths)
	}
	if !slices.Equal(got.Backing, []vmBacking{{Image: backedOverlay, File: backedDisk}}) {
		t.Fatalf("links = %v", got.Backing)
	}
	if got.SnapshotDB != nil {
		t.Fatal("the list of the newest backup would go back with an older chain")
	}
}

func TestDefinitionAsBackedUpFollowsAChangedBlockImageDownTheChain(t *testing.T) {
	s := chainService(nil)
	def := vmDefinition{
		DomainXML: labDomain("qcow2", labOverlayHost, labBaseHost),
		DiskPaths: []string{backedOverlay},
		Backing:   []vmBacking{{Image: backedOverlay, File: backedDisk}},
	}
	m := backup.BlocksManifest{Disks: []backup.BlocksDisk{{Dev: "vda", Path: backedDisk, Format: "qcow2"}}}

	got, err := s.definitionAsBackedUp(t.Context(), repoRef{}, "abc", def, snapshotFiles{backedDisk}, &m)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.DiskPaths, []string{backedDisk}) || len(got.Backing) != 0 {
		t.Fatalf("disks = %v, links = %v", got.DiskPaths, got.Backing)
	}
	if strings.Contains(got.DomainXML, "backingStore") || !strings.Contains(got.DomainXML, "file='"+labBaseHost+"'") {
		t.Fatalf("definition:\n%s", got.DomainXML)
	}
}

func TestDefinitionAsBackedUpFindsAChangedBlockImageAfterAMergeByItsDevice(t *testing.T) {
	s := chainService(nil)
	def := vmDefinition{DomainXML: labDomain("qcow2", labBaseHost), DiskPaths: []string{backedDisk}}
	m := backup.BlocksManifest{Disks: []backup.BlocksDisk{{Dev: "vda", Path: backedOverlay, Format: "qcow2"}}}

	got, err := s.definitionAsBackedUp(t.Context(), repoRef{}, "abc", def, snapshotFiles{backedOverlay}, &m)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.DiskPaths, []string{backedOverlay}) {
		t.Fatalf("disks = %v", got.DiskPaths)
	}
}
