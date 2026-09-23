package api

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// zfsProbeFixture builds a service whose only wiring is the host and the mount
// table, which is all the probes read.
func zfsProbeFixture(t *testing.T, cfg config.Config) (*Service, *store.Repo, *fakeZFSHost) {
	t.Helper()
	st := newTestStore(t)
	host := &fakeZFSHost{}
	cfg.DataDir = t.TempDir()
	cfg.AppKey = strings.Repeat("a", 64)
	if cfg.HostMountRoot == "" {
		cfg.HostMountRoot = "/host/user"
	}
	if cfg.HostSourceRoot == "" {
		cfg.HostSourceRoot = "/mnt"
	}
	s := &Service{
		store:  st,
		engine: &zfsFakeEngine{},
		zfs:    host,
		cfg:    cfg,
		repoMu: map[string]*sync.Mutex{"zfs": {}, "containers": {}},
	}
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	settings.ZFSEnabled = true
	settings.ZFSPath = "bombvault/zfs"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	zfsMountFixture(t, zfsTestRecords(t, zfsRunMountinfo), host, nil)
	return s, st, host
}

func TestZFSConnectionTestCodes(t *testing.T) {
	sshTarget := config.Config{LibvirtHost: "tower", LibvirtSSHUser: "root", LibvirtSSHPort: "22"}

	cases := []struct {
		name      string
		cfg       config.Config
		versionOn bool
		versionOf error
		noHost    bool
		wantCode  string
		wantTarg  string
		wantURI   string
	}{
		{
			name:     "no host is wired",
			cfg:      sshTarget,
			noHost:   true,
			wantCode: "ssh-missing",
		},
		{
			name:      "the host answers",
			cfg:       sshTarget,
			versionOn: true,
			wantCode:  "ok",
			wantTarg:  "root@tower:22",
		},
		{
			name:      "the template placeholder fell back to the gateway",
			cfg:       config.Config{LibvirtHost: "host.docker.internal", LibvirtSSHUser: "root", LibvirtSSHPort: "22", LibvirtHostWasPlaceholder: true},
			versionOn: true,
			wantCode:  "host-fallback",
			wantTarg:  "root@host.docker.internal:22",
		},
		{
			name:      "the template placeholder reaches nothing",
			cfg:       config.Config{LibvirtHost: "host.docker.internal", LibvirtSSHUser: "root", LibvirtSSHPort: "22", LibvirtHostWasPlaceholder: true},
			versionOf: &zfs.CmdError{Code: "ssh-unreachable", Stderr: "connection refused"},
			wantCode:  "host-placeholder",
		},
		{
			name:      "the key is not authorized",
			cfg:       sshTarget,
			versionOf: &zfs.CmdError{Code: "ssh-auth", Stderr: "Permission denied (publickey)."},
			wantCode:  "ssh-auth",
		},
		{
			name:      "the host has no zfs",
			cfg:       sshTarget,
			versionOf: &zfs.CmdError{Code: "zfs-not-found", Stderr: "zfs: command not found"},
			wantCode:  "zfs-not-found",
		},
		{
			name: "the URI names another machine",
			cfg: config.Config{
				LibvirtHost: "tower", LibvirtSSHUser: "root", LibvirtSSHPort: "22",
				LibvirtURI: "qemu+ssh://admin@truenas:2222/system", LibvirtURIHost: "truenas",
				LibvirtURIUser: "admin", LibvirtURIPort: "2222",
			},
			versionOn: true,
			wantCode:  "uri-mismatch",
			wantTarg:  "root@tower:22",
			wantURI:   "admin@truenas:2222",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, _, host := zfsProbeFixture(t, tc.cfg)
			if tc.noHost {
				s.zfs = nil
			}
			host.versionErr = tc.versionOf

			res := s.ZFSConnectionTest(context.Background())
			if res.Code != tc.wantCode {
				t.Fatalf("code = %q, want %q (detail %q)", res.Code, tc.wantCode, res.Detail)
			}
			if tc.wantTarg != "" && res.Target != tc.wantTarg {
				t.Errorf("target = %q, want %q", res.Target, tc.wantTarg)
			}
			if res.URITarget != tc.wantURI {
				t.Errorf("uri target = %q, want %q", res.URITarget, tc.wantURI)
			}
			if tc.versionOn && res.Version == "" {
				t.Error("a working connection must report the host's zfs version")
			}
			if tc.versionOf != nil && res.Detail == "" {
				t.Error("a failure must carry what the host wrote")
			}
		})
	}
}

// An old Host Data mapping passes no new mounts on, so the snapshot a run takes
// never appears in the container.
func TestZFSConnectionTestReportsMissingPropagation(t *testing.T) {
	const unpropagated = `
30 1 0:28 / / rw,relatime - overlay overlay rw
40 30 0:29 /mnt /host/user rw,relatime - rootfs rootfs rw
41 40 0:31 / /host/user/cache rw,relatime - zfs cache rw,xattr
`
	s, _, host := zfsProbeFixture(t, config.Config{LibvirtHost: "tower", LibvirtSSHUser: "root", LibvirtSSHPort: "22"})
	zfsMountFixture(t, zfsTestRecords(t, unpropagated), host, nil)

	res := s.ZFSConnectionTest(context.Background())
	if res.Code != "propagation-missing" {
		t.Fatalf("code = %q, want propagation-missing", res.Code)
	}
	if res.Propagation != "propagation-missing" {
		t.Errorf("propagation = %q, want propagation-missing", res.Propagation)
	}
	if len(res.Unpropagated) != 1 || res.Unpropagated[0] != "/host/user/cache" {
		t.Errorf("unpropagated = %v, want the one nested zfs record", res.Unpropagated)
	}
}

func TestZFSSpikeStatusOffWhileDomainDisabled(t *testing.T) {
	s, st, host := zfsProbeFixture(t, config.Config{LibvirtHost: "tower", LibvirtSSHUser: "root", LibvirtSSHPort: "22"})
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	settings.ZFSEnabled = false
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	detail, err := s.ZFSSpikeStatus()
	if err != nil {
		t.Fatalf("a switched-off domain is not a failed probe: %v", err)
	}
	if detail == "" {
		t.Fatal("the probe must say why it did not run")
	}
	if len(host.recorded()) != 0 {
		t.Fatalf("the dashboard probe reached the host while the domain is off: %v", host.recorded())
	}
}

func TestZFSSpikeStatusFailsOnAConnectionCode(t *testing.T) {
	s, _, host := zfsProbeFixture(t, config.Config{LibvirtHost: "tower", LibvirtSSHUser: "root", LibvirtSSHPort: "22"})
	host.versionErr = &zfs.CmdError{Code: "ssh-auth", Stderr: "Permission denied (publickey)."}

	if _, err := s.ZFSSpikeStatus(); err == nil {
		t.Fatal("a host the domain cannot reach must fail the probe")
	} else if !strings.Contains(err.Error(), "ssh-auth") {
		t.Fatalf("error = %q, want it to name the reason code", err)
	}
}

// The host listing fills the add dialog: what each dataset is, whether it can
// be read, and what already covers it.
func TestDiscoverZFSHostTreeFields(t *testing.T) {
	s, st, host := zfsProbeFixture(t, config.Config{})
	host.tree = []zfs.ListEntry{
		zfsEntry(zfsRoot, "/mnt/cache/appdata"),
		zfsEntry(zfsChild, "/mnt/cache/appdata/plex"),
		zfsEntry("cache/docker", "legacy"),
		zfsEntry("cache/vms", "/mnt/cache/vms"),
		{Name: "cache/win11", Type: "volume", Mountpoint: "-", Canmount: "-", Used: 4096},
		{Name: "cache/spare", Type: "volume", Mountpoint: "-", Canmount: "-", Used: 4096},
	}
	host.tree[3].Referenced = 1 << 20
	const layers = zfsMaxLegacyFilesystems + 1
	for i := range layers {
		host.tree = append(host.tree, zfsEntry("cache/docker/layer"+strconv.Itoa(i), "legacy"))
	}
	item := zfsSeedItem(t, st, zfsRoot)
	s.virsh = &zfsFakeVirsh{domains: map[string]string{
		"win11": `<domain><devices>` +
			`<disk type="file" device="disk"><source file="/mnt/cache/vms/win11/vdisk1.img"/><target dev="hdc"/></disk>` +
			`<disk type="block" device="disk"><source dev="/dev/zvol/cache/win11"/><target dev="hdd"/></disk>` +
			`</devices></domain>`,
	}}

	res := s.DiscoverZFSHost(context.Background())
	if !res.Available || res.Code != "ok" {
		t.Fatalf("available = %v code = %q, want a usable listing", res.Available, res.Code)
	}
	if res.HiddenLegacy != layers+1 {
		t.Errorf("hiddenLegacy = %d, want every legacy filesystem counted", res.HiddenLegacy)
	}
	if res.UnusedZvols != 1 {
		t.Errorf("unusedZvols = %d, want only the volume no VM references", res.UnusedZvols)
	}

	by := map[string]ZFSDiscoveredEntry{}
	for _, e := range res.Datasets {
		by[e.Dataset] = e
	}
	if got := by[zfsRoot]; got.ManagedID != item.ID || got.HostMountpoint != "/mnt/cache/appdata" {
		t.Errorf("the item root = %+v, want it marked as managed with its mountpoint", got)
	}
	if got := by[zfsChild]; got.CoveredBy != item.ID {
		t.Errorf("the child of an item = %+v, want it marked as covered", got)
	}
	if got := by["cache/docker"]; got.MemberCode != "legacy-mount" {
		t.Errorf("a legacy filesystem = %q, want legacy-mount", got.MemberCode)
	}
	if got := by["cache/vms"]; !got.VMDisk || got.Referenced != 1<<20 {
		t.Errorf("the VM disk folder = %+v, want vmDisk with its referenced size", got)
	}
	if got := by["cache/win11"]; !got.VMVolume || got.MemberCode != "zvol" {
		t.Errorf("a VM's volume = %+v, want vmVolume and the zvol code", got)
	}
	if got := by["cache/spare"]; got.VMVolume {
		t.Errorf("a volume no VM references = %+v, want it left unmarked", got)
	}
	if got := by[zfsChild]; len(got.Blockers) == 0 {
		t.Error("a dataset inside an item must be blocked as a root of its own")
	}
	if got := by["cache/docker"]; !containsString(got.Blockers, "docker-storage") {
		t.Errorf("blockers of Docker's own storage = %v, want docker-storage", got.Blockers)
	}
}

func TestDiscoverZFSHostUnavailable(t *testing.T) {
	s, _, host := zfsProbeFixture(t, config.Config{LibvirtHost: "tower", LibvirtSSHUser: "root", LibvirtSSHPort: "22"})
	host.listErr = &zfs.CmdError{Code: "zfs-permission", Stderr: "permission denied"}

	res := s.DiscoverZFSHost(context.Background())
	if res.Available {
		t.Fatal("a listing the host refused must not read as available")
	}
	if res.Code != "zfs-permission" {
		t.Fatalf("code = %q, want zfs-permission", res.Code)
	}
	if res.Target == "" {
		t.Error("the refusal must name the machine it was asked of")
	}

	s.zfs = nil
	if res := s.DiscoverZFSHost(context.Background()); res.Code != "ssh-missing" {
		t.Fatalf("code without a host = %q, want ssh-missing", res.Code)
	}
}

func TestCheckZFSDatasetReportsEveryMemberWithItsCode(t *testing.T) {
	s, _, host := zfsProbeFixture(t, config.Config{})
	host.tree = zfsTwoDatasetTree()
	host.tree = append(host.tree, zfsEntry("cache/appdata/enc", "/mnt/cache/appdata/enc"))
	host.tree[2].Keystatus = "unavailable"

	check := s.CheckZFSDataset(context.Background(), zfsRoot, []string{zfsChild})
	if check.Code != "ok" {
		t.Fatalf("code = %q, want ok (detail %q)", check.Code, check.Detail)
	}
	if check.ContainerPath != zfsRootPath {
		t.Errorf("container path = %q, want %q", check.ContainerPath, zfsRootPath)
	}
	if len(check.Members) != 3 {
		t.Fatalf("members = %d, want one per dataset of the tree", len(check.Members))
	}
	want := map[string]string{zfsRoot: "", zfsChild: "excluded", "cache/appdata/enc": "key-not-loaded"}
	for _, m := range check.Members {
		if m.Outcome != want[m.Dataset] {
			t.Errorf("%s outcome = %q, want %q", m.Dataset, m.Outcome, want[m.Dataset])
		}
	}
}

func TestCheckZFSDatasetRefusesWhatCannotBeAnItem(t *testing.T) {
	s, _, host := zfsProbeFixture(t, config.Config{})

	if got := s.CheckZFSDataset(context.Background(), "cache/../etc", nil); got.Code != "invalid-name" {
		t.Errorf("a name outside the charset = %q, want invalid-name", got.Code)
	}
	host.treeErr = &zfs.CmdError{Code: "not-found", Stderr: "dataset does not exist"}
	if got := s.CheckZFSDataset(context.Background(), "cache/gone", nil); got.Code != "not-found" {
		t.Errorf("a root that is gone = %q, want not-found", got.Code)
	}
	host.treeErr = nil
	host.tree = []zfs.ListEntry{{Name: "cache/win11", Type: "volume"}}
	if got := s.CheckZFSDataset(context.Background(), "cache/win11", nil); got.Code != "not-filesystem" {
		t.Errorf("a volume as a root = %q, want not-filesystem", got.Code)
	}
}

func TestProbeZFSSnapshotAccessCreatesVerifiesDestroysTree(t *testing.T) {
	s, st, host, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	d := zfsSeedItem(t, st, zfsRoot)

	check := s.ProbeZFSSnapshotAccess(context.Background(), d.ID)
	if check.Code != "ok" {
		t.Fatalf("code = %q, want ok (detail %q)", check.Code, check.Detail)
	}
	calls := host.recorded()
	var snap, destroy int
	for _, c := range calls {
		switch {
		case strings.HasPrefix(c, "snapshot -r "):
			snap++
		case strings.HasPrefix(c, "destroy -r "):
			destroy++
		}
	}
	if snap != 1 || destroy != 1 {
		t.Fatalf("calls = %v, want one recursive snapshot and one destroy", calls)
	}
	if len(host.snapshotNames()) != 0 {
		t.Fatalf("the probe left %v behind", host.snapshotNames())
	}
	for _, m := range check.Members {
		if m.Dataset == zfsRoot && m.Outcome != "" {
			t.Errorf("the root came back %q, want it readable", m.Outcome)
		}
	}
}

func TestProbeZFSSnapshotAccessReportsAnInvisibleSnapshot(t *testing.T) {
	s, st, host, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	d := zfsSeedItem(t, st, zfsRoot)
	// The snapshot is taken, but nothing of it ever appears in the container.
	zfsMountFixture(t, zfsTestRecords(t, zfsRunMountinfo), host, nil)

	check := s.ProbeZFSSnapshotAccess(context.Background(), d.ID)
	if check.Code != "snapshot-not-visible" {
		t.Fatalf("code = %q, want snapshot-not-visible", check.Code)
	}
	if !hostDid(host, "destroy -r ") {
		t.Fatal("a failed probe must still remove its snapshot")
	}
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// zfsFakeVirsh answers the two calls the host listing makes to find out which
// datasets a VM already owns.
type zfsFakeVirsh struct {
	domains map[string]string
	err     error
}

var _ virshcli.Virsh = (*zfsFakeVirsh)(nil)

func (v *zfsFakeVirsh) List(context.Context) ([]virshcli.VMInfo, error) {
	if v.err != nil {
		return nil, v.err
	}
	out := make([]virshcli.VMInfo, 0, len(v.domains))
	for name := range v.domains {
		out = append(out, virshcli.VMInfo{Name: name})
	}
	return out, nil
}

func (v *zfsFakeVirsh) DumpXML(_ context.Context, name string) (string, error) {
	xml, ok := v.domains[name]
	if !ok {
		return "", errors.New("no such domain")
	}
	return xml, nil
}

func (v *zfsFakeVirsh) DumpXMLInactive(ctx context.Context, name string) (string, error) {
	return v.DumpXML(ctx, name)
}

func (*zfsFakeVirsh) State(context.Context, string) (string, error)  { return "", nil }
func (*zfsFakeVirsh) Shutdown(context.Context, string) error         { return nil }
func (*zfsFakeVirsh) Destroy(context.Context, string) error          { return nil }
func (*zfsFakeVirsh) Start(context.Context, string) error            { return nil }
func (*zfsFakeVirsh) Define(context.Context, string) error           { return nil }
func (*zfsFakeVirsh) Undefine(context.Context, string) error         { return nil }
func (*zfsFakeVirsh) Autostart(context.Context, string, bool) error  { return nil }
func (*zfsFakeVirsh) IsActive(context.Context, string) (bool, error) { return false, nil }
func (*zfsFakeVirsh) GuestAgentPing(context.Context, string) bool    { return false }
func (*zfsFakeVirsh) BlockCommitActivePivot(context.Context, string, string) error {
	return nil
}
func (*zfsFakeVirsh) SnapshotCreateDiskOnly(context.Context, string, string, bool, []string) error {
	return nil
}
