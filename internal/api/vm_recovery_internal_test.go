package api

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// TestDomainLocks pins the per-repo serialisation: while a backup holds a
// domain's lock, maintenance (tryLockDomain) reports busy; other domains stay
// lockable; an unknown domain is a no-op success.
func TestDomainLocks(t *testing.T) {
	s := &Service{repoMu: map[string]*sync.Mutex{"containers": {}, "vms": {}}}

	release := s.lockDomain("containers") // simulate a backup holding the lock
	if _, ok := s.tryLockDomain("containers"); ok {
		t.Fatal("tryLockDomain must report busy while the domain is held")
	}
	if u, ok := s.tryLockDomain("vms"); !ok {
		t.Fatal("a different domain must remain lockable")
	} else {
		u()
	}
	release()

	u, ok := s.tryLockDomain("containers")
	if !ok {
		t.Fatal("domain must be lockable again after release")
	}
	u()

	if _, ok := s.tryLockDomain("unknown"); !ok {
		t.Fatal("an unknown domain must be a no-op success (never blocks)")
	}
}

// scriptedVirsh is a minimal virshcli.Virsh for the leftover-overlay recovery
// tests: it records blockcommit calls and serves a scripted DumpXML.
type scriptedVirsh struct {
	active    bool
	dumpXMLs  []string // returned per DumpXML call (last value repeats)
	dumpIdx   int
	commits   []string // devices passed to BlockCommitActivePivot
	commitErr error
}

var _ virshcli.Virsh = (*scriptedVirsh)(nil)

func (s *scriptedVirsh) List(context.Context) ([]virshcli.VMInfo, error) { return nil, nil }
func (s *scriptedVirsh) State(context.Context, string) (string, error)   { return "", nil }
func (s *scriptedVirsh) DumpXML(context.Context, string) (string, error) {
	if len(s.dumpXMLs) == 0 {
		return "<domain/>", nil
	}
	i := s.dumpIdx
	if i >= len(s.dumpXMLs) {
		i = len(s.dumpXMLs) - 1
	}
	s.dumpIdx++
	return s.dumpXMLs[i], nil
}
func (s *scriptedVirsh) Shutdown(context.Context, string) error          { return nil }
func (s *scriptedVirsh) Destroy(context.Context, string) error           { return nil }
func (s *scriptedVirsh) Start(context.Context, string) error             { return nil }
func (s *scriptedVirsh) Define(context.Context, string) error            { return nil }
func (s *scriptedVirsh) Undefine(context.Context, string) error          { return nil }
func (s *scriptedVirsh) Autostart(context.Context, string, bool) error   { return nil }
func (s *scriptedVirsh) IsActive(context.Context, string) (bool, error)  { return s.active, nil }
func (s *scriptedVirsh) SnapshotCreateDiskOnly(context.Context, string, string, bool, []string) error {
	return nil
}
func (s *scriptedVirsh) BlockCommitActivePivot(_ context.Context, _, dev string) error {
	s.commits = append(s.commits, dev)
	return s.commitErr
}
func (s *scriptedVirsh) GuestAgentPing(context.Context, string) bool { return false }

const overlayDomainXML = `<domain><devices>
  <disk type='file' device='disk'>
    <source file='/mnt/user/domains/WinSrv/vdisk1.bombvault-tmp'/>
    <target dev='hdc'/>
  </disk>
</devices></domain>`

const cleanDomainXML = `<domain><devices>
  <disk type='file' device='disk'>
    <source file='/mnt/user/domains/WinSrv/vdisk1.qcow2'/>
    <target dev='hdc'/>
  </disk>
</devices></domain>`

// TestLeftoverOverlayDevices: a writable disk whose source is a "*.bombvault-tmp"
// overlay (a leftover from an interrupted live backup) is detected by its device;
// a clean disk is not. Matching our own snapshot name is unambiguous.
func TestLeftoverOverlayDevices(t *testing.T) {
	clean := virshcli.DomainInfo{Disks: []virshcli.DiskRef{
		{Dev: "hdc", Source: "/mnt/user/domains/WinSrv/vdisk1.qcow2"},
	}}
	if got := leftoverOverlayDevices(clean); len(got) != 0 {
		t.Fatalf("clean domain should yield no overlay devices, got %v", got)
	}

	overlay := virshcli.DomainInfo{Disks: []virshcli.DiskRef{
		{Dev: "hdc", Source: "/mnt/user/domains/WinSrv/vdisk1.bombvault-tmp"},
		{Dev: "vdb", Source: "/mnt/user/domains/WinSrv/data.qcow2"},
	}}
	got := leftoverOverlayDevices(overlay)
	if len(got) != 1 || got[0] != "hdc" {
		t.Fatalf("expected [hdc], got %v", got)
	}
}

// TestRecoverLeftoverOverlayCommitsAndRedumps: a running VM on a "*.bombvault-tmp"
// overlay is committed back (blockcommit on the overlay device), then re-dumped so
// the returned domain points at the clean base disk.
func TestRecoverLeftoverOverlayCommitsAndRedumps(t *testing.T) {
	sv := &scriptedVirsh{active: true, dumpXMLs: []string{cleanDomainXML}}
	s := &Service{virsh: sv}
	dom, _ := virshcli.ParseDomain(overlayDomainXML)

	gotXML, gotDom, err := s.recoverLeftoverOverlay(context.Background(), "WinSrv", overlayDomainXML, dom)
	if err != nil {
		t.Fatalf("recoverLeftoverOverlay: %v", err)
	}
	if len(sv.commits) != 1 || sv.commits[0] != "hdc" {
		t.Fatalf("expected one blockcommit on hdc, got %v", sv.commits)
	}
	if strings.Contains(gotXML, "bombvault-tmp") || !strings.Contains(gotXML, "vdisk1.qcow2") {
		t.Fatalf("expected the clean re-dumped xml, got %s", gotXML)
	}
	if len(gotDom.DiskPaths) != 1 || gotDom.DiskPaths[0] != "/mnt/user/domains/WinSrv/vdisk1.qcow2" {
		t.Fatalf("expected the clean base path, got %v", gotDom.DiskPaths)
	}
}

// TestRecoverLeftoverOverlayShutOffErrors: a shut-off VM with a leftover overlay
// can't be active-committed; we must error (never silently start it), and never
// blockcommit.
func TestRecoverLeftoverOverlayShutOffErrors(t *testing.T) {
	sv := &scriptedVirsh{active: false}
	s := &Service{virsh: sv}
	dom, _ := virshcli.ParseDomain(overlayDomainXML)

	if _, _, err := s.recoverLeftoverOverlay(context.Background(), "WinSrv", overlayDomainXML, dom); err == nil {
		t.Fatal("expected an error for a shut-off VM left on an overlay")
	}
	if len(sv.commits) != 0 {
		t.Fatalf("must not blockcommit a shut-off VM, got %v", sv.commits)
	}
}

// TestRecoverLeftoverOverlayNoOpWhenClean: a clean VM is untouched (no commit, xml
// passes through unchanged).
func TestRecoverLeftoverOverlayNoOpWhenClean(t *testing.T) {
	sv := &scriptedVirsh{active: true}
	s := &Service{virsh: sv}
	dom, _ := virshcli.ParseDomain(cleanDomainXML)

	gotXML, _, err := s.recoverLeftoverOverlay(context.Background(), "WinSrv", cleanDomainXML, dom)
	if err != nil {
		t.Fatalf("recoverLeftoverOverlay: %v", err)
	}
	if len(sv.commits) != 0 {
		t.Fatalf("a clean VM must not be committed, got %v", sv.commits)
	}
	if gotXML != cleanDomainXML {
		t.Fatal("clean xml should pass through unchanged")
	}
}
