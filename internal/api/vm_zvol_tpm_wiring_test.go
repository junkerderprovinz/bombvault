package api_test

// These tests run zvol disks, TPM capture and per-disk retention through the
// real BackupVM.

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

// zvolTPMVirsh serves a fixed domain XML for a VM that is shut off, so
// BackupVMGraceful never stops or restarts it.
type zvolTPMVirsh struct {
	fakeVirsh
	domainXML string
}

func (v zvolTPMVirsh) DumpXML(context.Context, string) (string, error) { return v.domainXML, nil }
func (v zvolTPMVirsh) DumpXMLInactive(context.Context, string) (string, error) {
	return v.domainXML, nil
}
func (v zvolTPMVirsh) IsActive(context.Context, string) (bool, error) { return false, nil }

// zvolTPMSSH fakes the host SSH file access used for NVRAM and TPM and the
// streamed ZFS commands used for zvols.
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

// vmZvolTestService returns a Service serving domainXML over virsh and using
// ssh as the host SSH. RetentionKeepLast is set because applyRetention skips
// ForgetPolicy without a policy. The returned root is HostMountRoot, where
// restore tests put the vms repository's config marker.
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

// TestBackupVMFileOnlySkipsZvolPath checks that a VM with only file disks, as
// on Unraid, gets one restic backup with the plain tags, no zvol stream and one
// retention call.
func TestBackupVMFileOnlySkipsZvolPath(t *testing.T) {
	svc, eng, _, _ := vmZvolTestService(t, fileOnlyVMDomainXML, &zvolTPMSSH{})

	if _, err := svc.BackupVM(context.Background(), "plainvm"); err != nil {
		t.Fatalf("BackupVM: %v", err)
	}

	if len(eng.backedUp) != 1 {
		t.Fatalf("expected exactly 1 restic Backup call, got %d: %v", len(eng.backedUp), eng.backedUp)
	}
	wantTags := "vm:plainvm,p2"
	if strings.Join(eng.lastTags, ",") != wantTags {
		t.Fatalf("tags = %v, want %q (no vmrun: tag for a file-only VM)", eng.lastTags, wantTags)
	}
	if len(eng.stdinBackups) != 0 {
		t.Fatalf("expected no zvol stdin backup calls for a file-only VM, got %v", eng.stdinBackups)
	}
	if len(eng.forgetTags) != 1 || eng.forgetTags[0] != "vm:plainvm" {
		t.Fatalf("retention calls = %v, want exactly one call for tag \"vm:plainvm\"", eng.forgetTags)
	}
}

// TestBackupVMMixedFileAndZvolDisksTagsAndRetainsPerDisk checks that a VM with
// one file disk and two zvol disks makes three restic backups sharing one
// vmrun:<runID> tag, that each zvol disk gets its own vm:<name>:zvol:<dev>
// tag, and that retention runs once per identity tag.
func TestBackupVMMixedFileAndZvolDisksTagsAndRetainsPerDisk(t *testing.T) {
	svc, eng, _, _ := vmZvolTestService(t, mixedVMDomainXML, &zvolTPMSSH{})

	if _, err := svc.BackupVM(context.Background(), "mixedvm"); err != nil {
		t.Fatalf("BackupVM: %v", err)
	}

	if len(eng.backedUp) != 1 {
		t.Fatalf("expected exactly 1 file-backed restic Backup call, got %d: %v", len(eng.backedUp), eng.backedUp)
	}
	if len(eng.stdinBackups) != 2 {
		t.Fatalf("expected exactly 2 zvol BackupStdin calls (1 per zvol disk), got %d: %v", len(eng.stdinBackups), eng.stdinBackups)
	}

	if len(eng.lastTags) != 3 || eng.lastTags[0] != "vm:mixedvm" || eng.lastTags[1] != "p2" {
		t.Fatalf("file-backed tags = %v, want [vm:mixedvm p2 vmrun:<id>]", eng.lastTags)
	}
	runTag := eng.lastTags[2]
	if !strings.HasPrefix(runTag, "vmrun:") || runTag == "vmrun:" {
		t.Fatalf("file-backed 3rd tag = %q, want a non-empty vmrun:<runID> tag", runTag)
	}

	// Zvol disks carry their own identity tag, not the plain "vm:mixedvm".
	wantSuffix1 := ":vm:mixedvm:zvol:vdb,p2," + runTag
	wantSuffix2 := ":vm:mixedvm:zvol:vdc,p2," + runTag
	if !strings.HasSuffix(eng.stdinBackups[0], wantSuffix1) {
		t.Fatalf("disk1 zvol call = %q, want suffix %q", eng.stdinBackups[0], wantSuffix1)
	}
	if !strings.HasSuffix(eng.stdinBackups[1], wantSuffix2) {
		t.Fatalf("disk2 zvol call = %q, want suffix %q", eng.stdinBackups[1], wantSuffix2)
	}

	wantForget := []string{"vm:mixedvm", "vm:mixedvm:zvol:vdb", "vm:mixedvm:zvol:vdc"}
	if strings.Join(eng.forgetTags, ",") != strings.Join(wantForget, ",") {
		t.Fatalf("retention tags = %v, want %v (exactly once per identity tag)", eng.forgetTags, wantForget)
	}
}

// TestBackupVMCapturesTPMStateWhenPresent checks that the TPM state is read
// over SSH and stored in the VM definition next to the NVRAM.
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

// TestBackupVMSkipsTPMWhenDomainHasNoTPMElement uses an SSH fake without
// files, so an attempted TPM read would fail the backup.
func TestBackupVMSkipsTPMWhenDomainHasNoTPMElement(t *testing.T) {
	svc, _, st, _ := vmZvolTestService(t, fileOnlyVMDomainXML, &zvolTPMSSH{})

	if _, err := svc.BackupVM(context.Background(), "plainvm"); err != nil {
		t.Fatalf("BackupVM: %v", err)
	}

	def := readVMDefinition(t, st, "plainvm")
	if len(def.TPMBytes) != 0 {
		t.Fatalf("TPMBytes = %q, want empty for a domain with no <tpm> element", def.TPMBytes)
	}
}

// vmDefJSON is the part of the unexported vmDefinition these tests inspect.
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
