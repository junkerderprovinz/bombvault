package api_test

// Tests for wiring the zvol (block-device VM disk) backup path, TPM state
// capture, and per-identity-tag retention into the REAL BackupVM/RestoreVM —
// v8.0.0 VM service-layer integration, Task 2 (the design notes
// ). Phase B (Task 10/11) built the
// zvol backup mechanism and TPM path parsing, but internal/api/service.go's
// actual BackupVM never populated VMBackupDeps.BlockDisks/ZFSHost/ZvolRestic
// and never captured TPM state at all — this file proves the real caller now
// does, and that the file-only-VM path (every Unraid VM, and most VMs in
// production generally) is completely unaffected.
//
// ⚠ Same caveat as the rest of this VM work: none of this is exercised
// against real TrueNAS hardware — see internal/backup/vm_orchestrator.go's
// "Zvol-aware VM disk backup/restore" section header comment.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// zvolTPMVirsh is a scriptable virshcli.Virsh serving a fixed domain XML and
// a shut-off, non-running VM — so BackupVMGraceful's shutdown/restart dance
// never engages and these tests can focus purely on the restic call/tag/
// retention plumbing (already covered elsewhere for the shutdown sequencing
// itself).
type zvolTPMVirsh struct {
	fakeVirsh
	domainXML string
}

func (v zvolTPMVirsh) DumpXML(context.Context, string) (string, error) { return v.domainXML, nil }
func (v zvolTPMVirsh) DumpXMLInactive(context.Context, string) (string, error) {
	return v.domainXML, nil
}
func (v zvolTPMVirsh) IsActive(context.Context, string) (bool, error) { return false, nil }

// zvolTPMSSH is a scriptable api.HostSSH fake covering the NVRAM/TPM SSH
// read/write AND (via StreamCommand/RunWithStdin) the zvol ZFS-over-SSH
// surface.
type zvolTPMSSH struct {
	files   map[string][]byte // path -> bytes ReadFile returns; missing = error
	written map[string][]byte // path -> bytes recorded by WriteFile
}

var _ api.HostSSH = (*zvolTPMSSH)(nil)

func (s *zvolTPMSSH) ReadFile(_ context.Context, path string) ([]byte, error) {
	if b, ok := s.files[path]; ok {
		return b, nil
	}
	return nil, errors.New("fake ssh: no such file: " + path)
}
func (s *zvolTPMSSH) WriteFile(_ context.Context, path string, data []byte) error {
	if s.written == nil {
		s.written = map[string][]byte{}
	}
	s.written[path] = data
	return nil
}
func (s *zvolTPMSSH) PublicKey() (string, error)                     { return "", nil }
func (s *zvolTPMSSH) Test(context.Context) error                     { return nil }
func (s *zvolTPMSSH) Run(context.Context, ...string) (string, error) { return "", nil }
func (s *zvolTPMSSH) EnsureKnownHost(context.Context) error          { return nil }
func (s *zvolTPMSSH) StreamCommand(context.Context, ...string) (io.ReadCloser, func() error, error) {
	return io.NopCloser(bytes.NewReader([]byte("zvol-send-stream"))), func() error { return nil }, nil
}
func (s *zvolTPMSSH) RunWithStdin(_ context.Context, rd io.Reader, _ ...string) error {
	_, err := io.Copy(io.Discard, rd)
	return err
}

const fileOnlyVMDomainXML = `<domain type='kvm'>
  <devices>
    <disk type='file' device='disk'>
      <source file='/mnt/user/domains/plainvm/vdisk0.qcow2'/>
      <target dev='vda'/>
    </disk>
  </devices>
</domain>`

const mixedVMDomainXML = `<domain type='kvm'>
  <devices>
    <disk type='file' device='disk'>
      <source file='/mnt/user/domains/mixedvm/vdisk0.qcow2'/>
      <target dev='vda'/>
    </disk>
    <disk type='block' device='disk'>
      <source dev='/dev/zvol/tank/vms/mixedvm/disk1'/>
      <target dev='vdb'/>
    </disk>
    <disk type='block' device='disk'>
      <source dev='/dev/zvol/tank/vms/mixedvm/disk2'/>
      <target dev='vdc'/>
    </disk>
  </devices>
</domain>`

const tpmVMDomainXML = `<domain type='kvm'>
  <devices>
    <disk type='file' device='disk'>
      <source file='/mnt/user/domains/tpmvm/vdisk0.qcow2'/>
      <target dev='vda'/>
    </disk>
    <tpm model='tpm-tis'>
      <backend type='passthrough'>
        <device path='/dev/tpm0'/>
      </backend>
    </tpm>
  </devices>
  <os><nvram>/etc/libvirt/qemu/nvram/tpmvm_VARS.fd</nvram></os>
</domain>`

// vmZvolTestService builds a Service wired for these tests: a temp store with
// retention configured (RetentionKeepLast > 0, so applyRetention's early
// "policy has nothing to do" return doesn't swallow every ForgetPolicy call),
// the given domain XML served by virsh, and the given SSH fake. The returned
// root is the HostMountRoot temp dir — restore-side tests (Task 3) need it to
// seed the vms repo's "config" marker file directly (snapshotsForTag's
// localRepoMissing check requires it on disk; BackupVM never needs it since
// the fake engine's Backup/BackupStdin calls don't touch the filesystem).
func vmZvolTestService(t *testing.T, domainXML string, ssh *zvolTPMSSH) (*api.Service, *fakeResticEngine, *store.Repo, string) {
	t.Helper()
	dir := t.TempDir()
	root := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: root, HostSourceRoot: "/mnt"}
	st := newMemStore(t)
	settings := mustSettings(t, st)
	settings.VMsPath = "backups/vms"
	settings.RetentionKeepLast = 5
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	eng := &fakeResticEngine{}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, zvolTPMVirsh{domainXML: domainXML}, eng)
	if ssh != nil {
		svc.SetHostSSH(ssh)
	}
	return svc, eng, st, root
}

// TestBackupVMFileOnlyIsByteIdenticalToBeforeZvolTPMWiring is the critical
// regression case: a VM with only file disks (every Unraid VM, and most VMs
// in production generally) must see NO behaviour change at all from this
// task's BlockDisks/ZFSHost/ZvolRestic/RunTag/TPM wiring — exactly one restic
// Backup call with its historical tags, no zvol stdin calls, and exactly one
// retention call for the plain "vm:<name>" tag.
func TestBackupVMFileOnlyIsByteIdenticalToBeforeZvolTPMWiring(t *testing.T) {
	svc, eng, _, _ := vmZvolTestService(t, fileOnlyVMDomainXML, &zvolTPMSSH{})

	if _, err := svc.BackupVM(context.Background(), "plainvm"); err != nil {
		t.Fatalf("BackupVM: %v", err)
	}

	if len(eng.backedUp) != 1 {
		t.Fatalf("expected exactly 1 restic Backup call, got %d: %v", len(eng.backedUp), eng.backedUp)
	}
	wantTags := "vm:plainvm,p2"
	if strings.Join(eng.lastTags, ",") != wantTags {
		t.Fatalf("tags = %v, want %q (byte-identical — no vmrun: tag for a file-only VM)", eng.lastTags, wantTags)
	}
	if len(eng.stdinBackups) != 0 {
		t.Fatalf("expected NO zvol stdin backup calls for a file-only VM, got %v", eng.stdinBackups)
	}
	if len(eng.forgetTags) != 1 || eng.forgetTags[0] != "vm:plainvm" {
		t.Fatalf("retention calls = %v, want exactly one call for tag \"vm:plainvm\"", eng.forgetTags)
	}
}

// TestBackupVMMixedFileAndZvolDisksTagsAndRetainsPerDisk is the core new
// behaviour: a VM with 1 file disk + 2 zvol disks makes 3 restic backup calls
// total (1 file-backed + 2 zvol stdin), every one of them carries the SAME
// "vmrun:<runID>" correlation tag, each zvol disk carries its OWN
// "vm:<name>:zvol:<dev>" identity tag, and retention is applied exactly once
// per identity tag actually produced (not once, not per-call).
func TestBackupVMMixedFileAndZvolDisksTagsAndRetainsPerDisk(t *testing.T) {
	svc, eng, _, _ := vmZvolTestService(t, mixedVMDomainXML, &zvolTPMSSH{})

	if _, err := svc.BackupVM(context.Background(), "mixedvm"); err != nil {
		t.Fatalf("BackupVM: %v", err)
	}

	// 3 restic backup calls total: 1 file-backed + 2 zvol.
	if len(eng.backedUp) != 1 {
		t.Fatalf("expected exactly 1 file-backed restic Backup call, got %d: %v", len(eng.backedUp), eng.backedUp)
	}
	if len(eng.stdinBackups) != 2 {
		t.Fatalf("expected exactly 2 zvol BackupStdin calls (1 per zvol disk), got %d: %v", len(eng.stdinBackups), eng.stdinBackups)
	}

	// The file-backed call's tags: "vm:mixedvm,p2,vmrun:<runID>".
	if len(eng.lastTags) != 3 || eng.lastTags[0] != "vm:mixedvm" || eng.lastTags[1] != "p2" {
		t.Fatalf("file-backed tags = %v, want [vm:mixedvm p2 vmrun:<id>]", eng.lastTags)
	}
	runTag := eng.lastTags[2]
	if !strings.HasPrefix(runTag, "vmrun:") || runTag == "vmrun:" {
		t.Fatalf("file-backed 3rd tag = %q, want a non-empty vmrun:<runID> tag", runTag)
	}

	// Each zvol disk carries its OWN "vm:mixedvm:zvol:<dev>" identity tag PLUS
	// the SAME shared runTag — never the file-backed disk's plain "vm:mixedvm"
	// tag.
	wantSuffix1 := ":vm:mixedvm:zvol:vdb,p2," + runTag
	wantSuffix2 := ":vm:mixedvm:zvol:vdc,p2," + runTag
	if !strings.HasSuffix(eng.stdinBackups[0], wantSuffix1) {
		t.Fatalf("disk1 zvol call = %q, want suffix %q", eng.stdinBackups[0], wantSuffix1)
	}
	if !strings.HasSuffix(eng.stdinBackups[1], wantSuffix2) {
		t.Fatalf("disk2 zvol call = %q, want suffix %q", eng.stdinBackups[1], wantSuffix2)
	}

	// Retention: once for "vm:mixedvm", once per distinct "vm:mixedvm:zvol:<dev>"
	// — exactly 3 calls, never once, never one-per-restic-call-with-duplicates.
	wantForget := []string{"vm:mixedvm", "vm:mixedvm:zvol:vdb", "vm:mixedvm:zvol:vdc"}
	if strings.Join(eng.forgetTags, ",") != strings.Join(wantForget, ",") {
		t.Fatalf("retention tags = %v, want %v (exactly once per identity tag)", eng.forgetTags, wantForget)
	}
}

// TestBackupVMCapturesTPMStateWhenPresent mirrors the existing NVRAM
// capture's shape exactly (see BackupVM's inline TPM read, sibling to its
// inline NVRAM read): when the domain's <tpm> element resolves to a usable
// path, its bytes are read over SSH and persisted as TPMBytes on the VM's
// stored definition, alongside (and independent of) NVRAMBytes.
func TestBackupVMCapturesTPMStateWhenPresent(t *testing.T) {
	ssh := &zvolTPMSSH{files: map[string][]byte{
		"/dev/tpm0":                             []byte("captured-tpm-state"),
		"/etc/libvirt/qemu/nvram/tpmvm_VARS.fd": []byte("captured-nvram"),
	}}
	svc, _, st, _ := vmZvolTestService(t, tpmVMDomainXML, ssh)

	if _, err := svc.BackupVM(context.Background(), "tpmvm"); err != nil {
		t.Fatalf("BackupVM: %v", err)
	}

	def := readVMDefinition(t, st, "tpmvm")
	if string(def.TPMBytes) != "captured-tpm-state" {
		t.Fatalf("TPMBytes = %q, want %q", def.TPMBytes, "captured-tpm-state")
	}
	if string(def.NVRAMBytes) != "captured-nvram" {
		t.Fatalf("NVRAMBytes = %q, want %q (must be unaffected by TPM capture)", def.NVRAMBytes, "captured-nvram")
	}
}

// TestBackupVMSkipsTPMWhenDomainHasNoTPMElement mirrors NVRAM's own
// empty-path skip exactly: a domain with no <tpm> element (domain.TPMPath ==
// "") never even attempts an SSH read — proven here by an SSH fake with NO
// files configured, so any unexpected ReadFile call would fail the backup.
func TestBackupVMSkipsTPMWhenDomainHasNoTPMElement(t *testing.T) {
	svc, _, st, _ := vmZvolTestService(t, fileOnlyVMDomainXML, &zvolTPMSSH{}) // no files configured at all

	if _, err := svc.BackupVM(context.Background(), "plainvm"); err != nil {
		t.Fatalf("BackupVM: %v", err)
	}

	def := readVMDefinition(t, st, "plainvm")
	if len(def.TPMBytes) != 0 {
		t.Fatalf("TPMBytes = %q, want empty for a domain with no <tpm> element", def.TPMBytes)
	}
}

// vmDefJSON mirrors the JSON shape of the unexported vmDefinition struct
// (internal/api/service.go) — enough of it for these tests to inspect the
// persisted NVRAM/TPM bytes via the store's own Definition JSON string,
// without needing package-internal access.
type vmDefJSON struct {
	NVRAMBytes []byte `json:"nvram_bytes,omitempty"`
	TPMBytes   []byte `json:"tpm_bytes,omitempty"`
}

func readVMDefinition(t *testing.T, st *store.Repo, name string) vmDefJSON {
	t.Helper()
	tg, err := st.GetVMTargetByName(name)
	if err != nil {
		t.Fatalf("GetVMTargetByName(%q): %v", name, err)
	}
	var def vmDefJSON
	if err := json.Unmarshal([]byte(tg.Definition), &def); err != nil {
		t.Fatalf("unmarshal definition: %v", err)
	}
	return def
}
