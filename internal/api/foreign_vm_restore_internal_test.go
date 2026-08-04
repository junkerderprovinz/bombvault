package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// The cross-instance VM restore (#122) must (1) NEVER write a disk onto an
// unmounted path (the RAM rootfs) and brick the host, (2) place disks on a chosen
// destination and rewrite the domain XML to match, (3) never autostart a
// foreign-restored VM, and (4) leave the same-instance restore byte-for-byte
// unchanged. These tests pin all four. They reuse foreignRecordingEngine (this
// package's white-box ResticEngine fake) rather than the api_test fakes.

// vmRestoreSvc builds a bare Service for the prepare-level VM restore tests:
// HostMountRoot=/host/user (pure slash, so containment + the mount discriminator
// are OS-independent) and HostSourceRoot=/mnt (so toHostPath round-trips).
func vmRestoreSvc(t *testing.T, eng ResticEngine) *Service {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return &Service{
		store:  store.New(db),
		engine: eng,
		cfg: config.Config{
			HostMountRoot:  "/host/user",
			HostSourceRoot: "/mnt",
			DataDir:        t.TempDir(),
			AppKey:         strings.Repeat("a", 64),
		},
	}
}

// seedResticRepoDir writes restic's config marker into dir so localRepoMissing
// reads the repo as present (the snapshot listing then hits the fake engine).
func seedResticRepoDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config"), []byte("cfg"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// writeMountFixture points the mount-table source at a fixture listing mountPoints
// as present mounts (mountinfo field 5), for the destination-mounted guard.
func writeMountFixture(t *testing.T, mountPoints ...string) {
	t.Helper()
	var b strings.Builder
	for i, mp := range mountPoints {
		fmt.Fprintf(&b, "%d 1 0:%d / %s rw - xfs /dev/sd%c rw\n", 36+i, 10+i, mp, 'a'+i)
	}
	p := filepath.Join(t.TempDir(), "mountinfo")
	if err := os.WriteFile(p, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(SetMountinfoPath(p))
}

// vmTargetJSON builds a store.VMTarget whose Definition carries the given XML,
// disk paths and nvram host path — the in-memory recipe prepareRestoreVMForTarget
// consumes without any store read.
func vmTargetJSON(t *testing.T, name, xml string, diskPaths []string, nvramHost string) store.VMTarget {
	t.Helper()
	def := vmDefinition{
		DomainXML:     xml,
		DiskPaths:     diskPaths,
		NVRAMHostPath: nvramHost,
		Method:        "graceful",
		WasAutostart:  true,
	}
	raw, err := json.Marshal(def)
	if err != nil {
		t.Fatalf("marshal def: %v", err)
	}
	return store.VMTarget{Name: name, Method: "graceful", Definition: string(raw)}
}

const vmDomainXML = `<domain type='kvm'><name>win10</name>` +
	`<os><nvram>/etc/libvirt/qemu/nvram/win10_VARS.fd</nvram></os>` +
	`<devices><disk type='file' device='disk'><source file='/mnt/pool/domains/win10/win10.qcow2'/><target dev='vda'/></disk>` +
	`<disk type='file' device='cdrom'><source file='/mnt/iso/boot.iso'/><target dev='hdc'/></disk></devices></domain>`

// TestPrepareRestoreVMRemapsDisksAndXML pins requirement 2: with a destination
// base the disks are remapped to <destBase>/<vm>/<basename>, the restore is a
// subtree->target remap, and the domain XML <disk> source + <nvram> are rewritten
// to the destination (the untouched cdrom ISO source proves only the VM's own
// disks move).
func TestPrepareRestoreVMRemapsDisksAndXML(t *testing.T) {
	eng := &foreignRecordingEngine{snaps: []restic.Snapshot{{ID: "deadbeef12345678", Tags: []string{"vm:win10"}}}}
	s := vmRestoreSvc(t, eng)
	repoDir := filepath.Join(t.TempDir(), "repo")
	seedResticRepoDir(t, repoDir)
	// The destination /host/user/vmrestore is a mounted share → the guard passes.
	writeMountFixture(t, "/", "/host/user", "/host/user/vmrestore")

	tg := vmTargetJSON(t, "win10", vmDomainXML,
		[]string{"/host/user/pool/domains/win10/win10.qcow2"},
		"/etc/libvirt/qemu/nvram/win10_VARS.fd")

	plan, err := s.prepareRestoreVMForTarget(context.Background(),
		repoRef{repo: repoDir}, "win10", "latest", tg, "/host/user/vmrestore")
	if err != nil {
		t.Fatalf("prepareRestoreVMForTarget: %v", err)
	}

	if len(plan.diskPaths) != 1 || plan.diskPaths[0] != "/host/user/vmrestore/win10/win10.qcow2" {
		t.Fatalf("disk must be remapped under the destination, got %v", plan.diskPaths)
	}
	wantDir := backup.VMRestoreDir{Subtree: "/host/user/pool/domains/win10", Target: "/host/user/vmrestore/win10"}
	if len(plan.restoreDirs) != 1 || plan.restoreDirs[0] != wantDir {
		t.Fatalf("restoreDirs must remap source subtree -> dest, got %v", plan.restoreDirs)
	}
	if !strings.Contains(plan.domainXML, "file='/mnt/vmrestore/win10/win10.qcow2'") {
		t.Fatalf("XML disk source must be rewritten to the destination, got %q", plan.domainXML)
	}
	if strings.Contains(plan.domainXML, "/mnt/pool/domains/win10/win10.qcow2") {
		t.Fatalf("the source disk path must be gone from the XML, got %q", plan.domainXML)
	}
	if !strings.Contains(plan.domainXML, "<nvram") || !strings.Contains(plan.domainXML, "/mnt/vmrestore/win10/win10_VARS.fd") {
		t.Fatalf("nvram must be rewritten to the destination, got %q", plan.domainXML)
	}
	// The cdrom ISO is NOT one of the VM's disks and must be left in place.
	if !strings.Contains(plan.domainXML, "file='/mnt/iso/boot.iso'") {
		t.Fatalf("a non-disk source (cdrom ISO) must be untouched, got %q", plan.domainXML)
	}
}

// TestPrepareRestoreVMGuardAbortsWhenNotMounted pins requirement 1: when the
// destination is NOT on a real mounted pool the restore aborts BEFORE any restic
// call, so a multi-GB disk can never be written into the host's RAM rootfs.
func TestPrepareRestoreVMGuardAbortsWhenNotMounted(t *testing.T) {
	eng := &foreignRecordingEngine{snaps: []restic.Snapshot{{ID: "deadbeef12345678", Tags: []string{"vm:win10"}}}}
	s := vmRestoreSvc(t, eng)
	repoDir := filepath.Join(t.TempDir(), "repo")
	seedResticRepoDir(t, repoDir)
	// Only "/" and the broad bind are mounted — the destination share is NOT.
	writeMountFixture(t, "/", "/host/user")

	tg := vmTargetJSON(t, "win10", vmDomainXML,
		[]string{"/host/user/pool/domains/win10/win10.qcow2"},
		"/etc/libvirt/qemu/nvram/win10_VARS.fd")

	_, err := s.prepareRestoreVMForTarget(context.Background(),
		repoRef{repo: repoDir}, "win10", "latest", tg, "/host/user/vmrestore")
	if err == nil || !strings.Contains(err.Error(), "not on a mounted") {
		t.Fatalf("want the not-mounted brick-guard abort, got %v", err)
	}
	if len(eng.restores) != 0 {
		t.Fatalf("nothing may be restored when the guard aborts, got %v", eng.restores)
	}
}

// TestPrepareRestoreVMGuardAbortsWhenTooSmall pins requirement 1's free-space
// half: a mounted destination that cannot fit the restore size aborts before any
// write.
func TestPrepareRestoreVMGuardAbortsWhenTooSmall(t *testing.T) {
	eng := &foreignRecordingEngine{
		snaps:             []restic.Snapshot{{ID: "deadbeef12345678", Tags: []string{"vm:win10"}}},
		statsRestoreBytes: 50 << 30, // the restore needs 50 GiB
	}
	s := vmRestoreSvc(t, eng)
	s.diskFree = func(string) (uint64, error) { return 1 << 30, nil } // only 1 GiB free
	repoDir := filepath.Join(t.TempDir(), "repo")
	seedResticRepoDir(t, repoDir)
	writeMountFixture(t, "/", "/host/user", "/host/user/vmrestore")

	tg := vmTargetJSON(t, "win10", vmDomainXML,
		[]string{"/host/user/pool/domains/win10/win10.qcow2"},
		"/etc/libvirt/qemu/nvram/win10_VARS.fd")

	_, err := s.prepareRestoreVMForTarget(context.Background(),
		repoRef{repo: repoDir}, "win10", "latest", tg, "/host/user/vmrestore")
	if err == nil || !strings.Contains(err.Error(), "free space") {
		t.Fatalf("want the free-space brick-guard abort, got %v", err)
	}
	if len(eng.restores) != 0 {
		t.Fatalf("nothing may be restored when the guard aborts, got %v", eng.restores)
	}
}

// TestPrepareRestoreVMSameInstanceUnchanged pins requirement 4: with no
// destination base (the same-instance restore) the disks return to their own
// paths, the XML is used verbatim (only EnsureNVRAMTemplate applied), no remap
// dirs are produced, and the captured autostart flag is preserved — and NO mount
// guard runs (proven by the absence of any mount fixture: it would otherwise
// abort). Byte-for-byte the historical behaviour.
func TestPrepareRestoreVMSameInstanceUnchanged(t *testing.T) {
	eng := &foreignRecordingEngine{snaps: []restic.Snapshot{{ID: "deadbeef12345678", Tags: []string{"vm:win10"}}}}
	s := vmRestoreSvc(t, eng)
	repoDir := filepath.Join(t.TempDir(), "repo")
	seedResticRepoDir(t, repoDir)

	disks := []string{"/host/user/pool/domains/win10/win10.qcow2"}
	tg := vmTargetJSON(t, "win10", vmDomainXML, disks, "/etc/libvirt/qemu/nvram/win10_VARS.fd")

	plan, err := s.prepareRestoreVMForTarget(context.Background(),
		repoRef{repo: repoDir}, "win10", "latest", tg, "")
	if err != nil {
		t.Fatalf("prepareRestoreVMForTarget: %v", err)
	}
	if !reflect.DeepEqual(plan.diskPaths, disks) {
		t.Fatalf("same-instance disks must be unchanged, got %v", plan.diskPaths)
	}
	if plan.restoreDirs != nil {
		t.Fatalf("same-instance restore must produce no remap dirs, got %v", plan.restoreDirs)
	}
	if plan.domainXML != virshcli.EnsureNVRAMTemplate(vmDomainXML) {
		t.Fatalf("same-instance XML must be verbatim (only EnsureNVRAMTemplate), got %q", plan.domainXML)
	}
	if !plan.wasAutostart {
		t.Fatal("same-instance restore must preserve the captured autostart flag")
	}
}

// ---------------------------------------------------------------------------
// Foreign VM restore round trip (requirement 3): remap + leave stopped + no
// autostart, end to end through StartForeignRestore. Linux-only: it seeds the
// foreign repo + def on disk under a slash-absolute host mount, which the path
// containment (paths.Within needs a leading "/") requires.
// ---------------------------------------------------------------------------

// recordingVirsh records the lifecycle calls the round trip must (or must not)
// make. It implements the full virshcli.Virsh surface (no-op except where noted)
// so it can drive executeRestoreVM.
type recordingVirsh struct {
	mu           sync.Mutex
	autostartOn  *bool
	started      bool
	defineCalled bool
}

var _ virshcli.Virsh = (*recordingVirsh)(nil)

func (v *recordingVirsh) List(context.Context) ([]virshcli.VMInfo, error) { return nil, nil }
func (v *recordingVirsh) State(context.Context, string) (string, error)   { return "", nil } // absent
func (v *recordingVirsh) DumpXML(context.Context, string) (string, error) { return "<domain/>", nil }
func (v *recordingVirsh) DumpXMLInactive(context.Context, string) (string, error) {
	return "<domain/>", nil
}
func (v *recordingVirsh) Shutdown(context.Context, string) error { return nil }
func (v *recordingVirsh) Destroy(context.Context, string) error  { return nil }
func (v *recordingVirsh) Start(context.Context, string) error {
	v.mu.Lock()
	v.started = true
	v.mu.Unlock()
	return nil
}
func (v *recordingVirsh) Define(context.Context, string) error {
	v.mu.Lock()
	v.defineCalled = true
	v.mu.Unlock()
	return nil
}
func (v *recordingVirsh) Undefine(context.Context, string) error { return nil }
func (v *recordingVirsh) Autostart(_ context.Context, _ string, on bool) error {
	v.mu.Lock()
	b := on
	v.autostartOn = &b
	v.mu.Unlock()
	return nil
}
func (v *recordingVirsh) IsActive(context.Context, string) (bool, error) { return false, nil }
func (v *recordingVirsh) SnapshotCreateDiskOnly(context.Context, string, string, bool, []string) error {
	return nil
}
func (v *recordingVirsh) BlockCommitActivePivot(context.Context, string, string) error { return nil }
func (v *recordingVirsh) GuestAgentPing(context.Context, string) bool                  { return false }

// TestForeignRestoreVMLeavesStoppedAndRemaps pins requirement 3 on the FOREIGN
// path: a cross-instance VM restore remaps its disk onto the chosen destination
// (RestoreSubtreeTo), clears autostart, and never starts the VM.
func TestForeignRestoreVMLeavesStoppedAndRemaps(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("foreign round trip seeds a slash-absolute host mount on disk (paths.Within needs a leading /)")
	}
	eng := &foreignRecordingEngine{
		opens: opensEncrypted,
		snaps: []restic.Snapshot{{ID: "dddddddd44444444", Time: "2026-07-04T10:00:00Z", Tags: []string{"vm:win10"}}},
	}
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	mountRoot := t.TempDir()
	vm := &recordingVirsh{}
	s := &Service{
		store:  store.New(db),
		engine: eng,
		virsh:  vm,
		cfg: config.Config{
			HostMountRoot:  mountRoot,
			HostSourceRoot: "/mnt",
			DataDir:        t.TempDir(),
			AppKey:         strings.Repeat("a", 64),
		},
	}
	t.Cleanup(s.stopForeignJanitor)

	// The disk lives under the host mount so paths.Within accepts it.
	diskPath := filepath.ToSlash(filepath.Join(mountRoot, "pool/domains/win10/win10.qcow2"))
	xml := `<domain type='kvm'><name>win10</name><devices>` +
		`<disk type='file' device='disk'><source file='` + s.toHostPath(diskPath) + `'/><target dev='vda'/></disk>` +
		`</devices></domain>`
	def := vmDefinition{DomainXML: xml, DiskPaths: []string{diskPath}, Method: "graceful", WasAutostart: true}
	defRaw, err := json.Marshal(def)
	if err != nil {
		t.Fatalf("marshal def: %v", err)
	}

	// Seed the foreign repo: config marker + the encrypted vm-def for "win10".
	repoDir := filepath.Join(mountRoot, "backups", "other")
	if err := os.MkdirAll(filepath.Join(repoDir, "vm-def"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "config"), []byte("cfg"), 0o600); err != nil {
		t.Fatal(err)
	}
	enc, err := secret.Encrypt(foreignTestKey, defRaw)
	if err != nil {
		t.Fatalf("encrypt def: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "vm-def", "win10.def"), enc, 0o600); err != nil {
		t.Fatal(err)
	}

	// The destination folder "vmrestore" is a mounted share, so the guard passes.
	destBase := filepath.ToSlash(filepath.Join(mountRoot, "vmrestore"))
	writeMountFixture(t, "/", filepath.ToSlash(mountRoot), destBase)

	id, _, err := s.OpenForeign(context.Background(), "backups/other", foreignTestKey)
	if err != nil {
		t.Fatalf("OpenForeign: %v", err)
	}
	started, err := s.StartForeignRestore(context.Background(), id, "vms", "win10", "latest", true, "vmrestore")
	if err != nil || !started {
		t.Fatalf("StartForeignRestore: started=%v err=%v", started, err)
	}
	waitForeignIdle(t, s)

	// Remapped restore happened (source subtree -> destination dir).
	eng.mu.Lock()
	restores := append([]string(nil), eng.restores...)
	eng.mu.Unlock()
	if len(restores) != 1 || !strings.HasPrefix(restores[0], "RestoreSubtreeTo|") || !strings.Contains(restores[0], "->"+destBase+"/win10") {
		t.Fatalf("expected one remapped RestoreSubtreeTo into %s/win10, got %v", destBase, restores)
	}

	// Left stopped + autostart cleared.
	vm.mu.Lock()
	autostartOn, startCalled, defined := vm.autostartOn, vm.started, vm.defineCalled
	vm.mu.Unlock()
	if !defined {
		t.Fatal("the VM must be defined on the destination")
	}
	if autostartOn == nil || *autostartOn {
		t.Fatalf("a foreign-restored VM must have autostart CLEARED, got %v", autostartOn)
	}
	if startCalled {
		t.Fatal("a foreign-restored VM must be left STOPPED (Start must not be called)")
	}
}
