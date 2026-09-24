package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

const (
	zfsConnectTimeout  = 20 * time.Second
	zfsDiscoverTimeout = 60 * time.Second
	// zfsMaxDiscoverEntries bounds the add dialog. A host with Docker's ZFS
	// storage driver carries thousands of layer datasets, and the dialog builds
	// a tree out of every line it gets.
	zfsMaxDiscoverEntries = 5000
	// zfsDetailLimit is how much host output a details block keeps.
	zfsDetailLimit = 2000
)

// ZFSCheck is what one look at a dataset tree found: whether it can be an item
// at all, and what each of its members would be.
type ZFSCheck struct {
	Dataset        string          `json:"dataset"`
	Code           string          `json:"code"`
	Detail         string          `json:"detail"`
	HostMountpoint string          `json:"hostMountpoint"`
	ContainerPath  string          `json:"containerPath"`
	Max            int             `json:"max,omitempty"`
	Names          []string        `json:"names,omitempty"`
	Members        []ZFSMemberView `json:"members"`
}

// ZFSConnectionResult is what the connection card shows: which machine the
// domain talks to, what its zfs answered, and whether the container still
// receives the mounts a snapshot makes.
type ZFSConnectionResult struct {
	Code         string   `json:"code"`
	Target       string   `json:"target"`
	URITarget    string   `json:"uriTarget"`
	Version      string   `json:"version"`
	Detail       string   `json:"detail"`
	ZFSBinary    string   `json:"zfsBinary"`
	Propagation  string   `json:"propagation"`
	Unpropagated []string `json:"unpropagated"`
}

// ZFSDiscoveredEntry is one dataset of the host listing, with everything the
// add dialog needs to decide what it may become.
type ZFSDiscoveredEntry struct {
	Dataset        string   `json:"dataset"`
	Type           string   `json:"type"`
	HostMountpoint string   `json:"hostMountpoint"`
	Referenced     int64    `json:"referenced"`
	Used           int64    `json:"used"`
	UsedByDataset  int64    `json:"usedByDataset"`
	Mounted        bool     `json:"mounted"`
	Encrypted      bool     `json:"encrypted"`
	KeyLoaded      bool     `json:"keyLoaded"`
	Visible        bool     `json:"visible"`
	Writable       bool     `json:"writable"`
	MemberCode     string   `json:"memberCode"`
	ManagedID      string   `json:"managedId"`
	CoveredBy      string   `json:"coveredBy"`
	VMDisk         bool     `json:"vmDisk"`
	System         bool     `json:"system"`
	VMVolume       bool     `json:"vmVolume"`
	Blockers       []string `json:"blockers"`
}

// ZFSDiscoverResult is the whole host listing plus the counts the page states
// without claiming anything about them.
type ZFSDiscoverResult struct {
	Available    bool                 `json:"available"`
	Code         string               `json:"code"`
	Target       string               `json:"target"`
	Datasets     []ZFSDiscoveredEntry `json:"datasets"`
	HiddenLegacy int                  `json:"hiddenLegacy"`
	UnusedZvols  int                  `json:"unusedZvols"`
	NotInItem    int                  `json:"notInItem"`
	Truncated    bool                 `json:"truncated"`
	// ListedAt is when the host was listed, in Unix seconds.
	ListedAt int64 `json:"listedAt"`
}

// ZFSConnectionTest reaches the pool owner and reports what stands between the
// domain and a working backup. It never returns a Go error: the code is the
// answer, and every code has a sentence and a fix on the page.
func (s *Service) ZFSConnectionTest(ctx context.Context) ZFSConnectionResult {
	res := ZFSConnectionResult{
		Target:       s.zfsSSHTarget(),
		URITarget:    s.zfsURITarget(),
		Propagation:  "ok",
		Unpropagated: []string{},
	}
	top, nested := zfs.Propagation(zfsMountRecords(), s.cfg.HostMountRoot)
	if !top || len(nested) > 0 {
		res.Propagation = "propagation-missing"
		res.Unpropagated = nested
	}
	if s.zfs == nil {
		res.Code = "ssh-missing"
		return res
	}
	cctx, cancel := context.WithTimeout(ctx, zfsConnectTimeout)
	defer cancel()
	version, err := s.zfs.Version(cctx)
	res.ZFSBinary = s.zfs.Binary()
	if err != nil {
		res.Code = zfsErrCode(err)
		res.Detail = zfsDetail(err.Error())
		if s.cfg.LibvirtHostWasPlaceholder {
			// The template ships a placeholder address, so the fallback is what
			// was actually tried and naming it is the only useful advice.
			res.Code = "host-placeholder"
		}
		return res
	}
	res.Version = strings.TrimSpace(strings.SplitN(strings.TrimSpace(version), "\n", 2)[0])
	switch {
	case s.zfsURIMismatch():
		res.Code = "uri-mismatch"
	case s.cfg.LibvirtHostWasPlaceholder:
		res.Code = "host-fallback"
	case res.Propagation != "ok":
		res.Code = "propagation-missing"
	default:
		res.Code = "ok"
	}
	return res
}

// ZFSSpikeStatus is the connection test as the host-integration panel reads it.
// A switched-off domain is not a failure and reaches no host, so the panel of a
// server that has no pools stays quiet.
func (s *Service) ZFSSpikeStatus() (string, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return "", fmt.Errorf("read settings: %w", err)
	}
	if !settings.ZFSEnabled {
		return "ZFS datasets are switched off", nil
	}
	res := s.ZFSConnectionTest(context.Background())
	switch res.Code {
	case "ok", "host-fallback":
		return res.Version + " on " + res.Target, nil
	case "propagation-missing":
		return "", fmt.Errorf("the container does not receive the host's new mounts (%s) [propagation-missing]",
			strings.Join(res.Unpropagated, ", "))
	}
	if res.Detail != "" {
		return "", fmt.Errorf("%s: %s [%s]", res.Target, res.Detail, res.Code)
	}
	return "", fmt.Errorf("%s [%s]", res.Target, res.Code)
}

// zfsHostListing is what one listing of the host found. The last one that
// worked is kept, so the page can show its counts without listing every pool
// over SSH each time it opens.
type zfsHostListing struct {
	list      []zfs.ListEntry
	truncated bool
	vmDirs    map[string]bool
	vmVolumes map[string]bool
	at        time.Time
}

// DiscoverZFSHost lists every dataset and volume of the host, with what each
// one would be as an item and what already covers it. Unless fresh is set, the
// last listing that worked answers, joined with the items as they are now.
func (s *Service) DiscoverZFSHost(ctx context.Context, fresh bool) ZFSDiscoverResult {
	res := ZFSDiscoverResult{Target: s.zfsSSHTarget(), Datasets: []ZFSDiscoveredEntry{}}
	s.zfsHostMu.Lock()
	last := s.zfsHostLast
	s.zfsHostMu.Unlock()
	if fresh || last == nil {
		listing, code := s.listZFSHost(ctx)
		if code != "" {
			res.Code = code
			return res
		}
		last = &listing
	}
	res.Available = true
	res.Code = "ok"
	res.Truncated = last.truncated
	res.ListedAt = last.at.Unix()
	list := last.list

	rows, err := s.store.ListZFSDatasets()
	if err != nil {
		log.Printf("api: zfs: the host listing could not read which datasets are already items: %v", err)
	}
	vmDisks := zfsVMDiskDatasets(list, last.vmDirs)
	recs := zfsMountRecords()
	legacyBelow := zfsLegacyCounts(list)

	for _, e := range list {
		entry := ZFSDiscoveredEntry{
			Dataset:        e.Name,
			Type:           e.Type,
			HostMountpoint: e.Mountpoint,
			Referenced:     e.Referenced,
			Used:           e.Used,
			UsedByDataset:  e.UsedByDataset,
			Mounted:        e.Mounted,
			Encrypted:      e.Encryption != "" && e.Encryption != "off",
			KeyLoaded:      e.Keystatus != "unavailable",
			MemberCode:     zfs.MemberCode(e, nil),
			VMVolume:       last.vmVolumes[e.Name],
			Blockers:       []string{},
		}
		if rec, code := s.resolveDatasetMount(recs, e, false); code == "" {
			entry.Visible = true
			entry.Writable = zfs.Writable(rec)
			entry.System = zfsHoldsSystemImage(rec.MountPoint)
		} else if entry.MemberCode == "" {
			entry.MemberCode = code
		}
		if e.Type == "filesystem" && e.Mountpoint == "legacy" {
			res.HiddenLegacy++
			entry.System = true
		}
		if e.Type == "volume" && !entry.VMVolume {
			res.UnusedZvols++
		}
		entry.VMDisk = vmDisks[e.Name]
		for _, d := range rows {
			switch {
			case d.Dataset == e.Name:
				entry.ManagedID = d.ID
			case zfs.DescendantOf(e.Name, d.Dataset):
				entry.CoveredBy = d.ID
			}
		}
		entry.Blockers = zfsRootBlockers(e, entry, rows, legacyBelow[e.Name])
		if e.Type == "filesystem" && e.Mounted && entry.ManagedID == "" && entry.CoveredBy == "" {
			res.NotInItem++
		}
		res.Datasets = append(res.Datasets, entry)
	}
	return res
}

// listZFSHost asks the host for its datasets and libvirt for its VMs' disks,
// and keeps what it found. It returns the reason code when the host refused.
func (s *Service) listZFSHost(ctx context.Context) (zfsHostListing, string) {
	if s.zfs == nil {
		return zfsHostListing{}, "ssh-missing"
	}
	lctx, cancel := context.WithTimeout(ctx, zfsDiscoverTimeout)
	defer cancel()
	list, err := s.zfs.List(lctx)
	if err != nil {
		return zfsHostListing{}, zfsErrCode(err)
	}
	listing := zfsHostListing{list: list, at: time.Now()}
	if len(list) > zfsMaxDiscoverEntries {
		listing.list, listing.truncated = list[:zfsMaxDiscoverEntries], true
	}
	listing.vmDirs, listing.vmVolumes = s.zfsVMOwned(ctx)
	s.zfsHostMu.Lock()
	s.zfsHostLast = &listing
	s.zfsHostMu.Unlock()
	return listing, ""
}

// CheckZFSDataset looks at a tree the way a run would, without taking a
// snapshot. It never returns a Go error; the code is the answer.
func (s *Service) CheckZFSDataset(ctx context.Context, root string, excluded []string) ZFSCheck {
	check, _ := s.checkZFSTree(ctx, root, excluded)
	return check
}

// checkZFSTree is CheckZFSDataset with the resolved members, which the snapshot
// probe needs to know where the container sees each one.
func (s *Service) checkZFSTree(ctx context.Context, root string, excluded []string) (ZFSCheck, []zfsMember) {
	check := ZFSCheck{Dataset: root, Members: []ZFSMemberView{}}
	if err := zfs.ValidateDatasetName(root); err != nil {
		var ne *zfs.NameError
		check.Code = "invalid-name"
		if errors.As(err, &ne) {
			check.Code = ne.Code
		}
		check.Detail = err.Error()
		if check.Code == "name-too-long" {
			check.Max = zfs.MaxDatasetNameLen
		}
		return check, nil
	}
	if s.zfs == nil {
		check.Code = "ssh-missing"
		return check, nil
	}
	lctx, cancel := context.WithTimeout(ctx, zfsListTimeout)
	defer cancel()
	tree, err := s.zfs.Tree(lctx, root)
	if err != nil {
		check.Code = zfsErrCode(err)
		check.Detail = zfsDetail(err.Error())
		return check, nil
	}
	if ref := zfsTreeRefusal(root, tree); ref != nil {
		check.Code = ref.Code
		check.Detail = ref.Detail
		if ref.Code == "name-too-long" {
			check.Max = zfs.MaxSnapshotNameLen
			check.Names = []string{ref.Detail}
		}
		return check, nil
	}

	members := s.zfsResolveMembers(tree, root, excluded, nil)
	readable := 0
	for _, m := range members {
		if m.Code == "" {
			readable++
		}
		check.Members = append(check.Members, ZFSMemberView{
			Dataset:        m.Entry.Name,
			RelPath:        m.RelPath,
			HostMountpoint: m.Entry.Mountpoint,
			Outcome:        m.Code,
			UsedByDataset:  m.Entry.UsedByDataset,
		})
	}
	check.HostMountpoint = tree[0].Mountpoint
	check.ContainerPath = members[0].Mount.MountPoint
	if readable == 0 {
		check.Code = "nothing-readable"
		return check, members
	}
	check.Code = "ok"
	return check, members
}

// ProbeZFSSnapshotAccess proves the whole read path on a real snapshot: the
// recursive snapshot, each readable member appearing in the container, and the
// destroy that takes it away again.
func (s *Service) ProbeZFSSnapshotAccess(ctx context.Context, id string) ZFSCheck {
	d, err := s.store.GetZFSDataset(id)
	if err != nil {
		return ZFSCheck{Code: "not-found", Detail: id, Members: []ZFSMemberView{}}
	}
	defer s.lockDomain(zfsDomain)()

	check, members := s.checkZFSTree(ctx, d.Dataset, d.ExcludedChildren)
	s.recordZFSCheck(d, check)
	if members != nil {
		s.recordZFSMembers(d, members)
	}
	if check.Code != "ok" || s.zfs == nil {
		return check
	}

	snap := zfs.SnapshotName(time.Now().UTC())
	sctx, cancel := context.WithTimeout(ctx, zfsSnapshotTimeout)
	defer cancel()
	if err := s.zfs.SnapshotRecursive(sctx, d.Dataset, snap); err != nil {
		check.Code = "snapshot-failed"
		check.Detail = zfsDetail(err.Error())
		s.recordZFSCheck(d, check)
		return check
	}
	defer func() {
		if err := backup.DestroyRecursiveWithRetry(context.WithoutCancel(ctx), s.zfs, d.Dataset, snap,
			zfsDestroyBudget, time.Now, sleepFor, s.IsShuttingDown()); err != nil {
			s.recordZFSLeftover(d, d.Dataset, snap, err)
		}
	}()

	for i, m := range members {
		if m.Code != "" {
			continue
		}
		if err := s.zfs.Prime(ctx, m.Entry.Mountpoint, snap); err != nil {
			log.Printf("api: zfs: priming the snapshot of %s failed: %v", m.Entry.Name, err)
		}
		vErr := s.visibleSnapshot(ctx, m.Entry.Name, snap, m.Mount.MountPoint+"/.zfs/snapshot/"+snap)
		if vErr == nil {
			continue
		}
		var ref *backup.ZFSRefusal
		if errors.As(vErr, &ref) {
			check.Members[i].Outcome = ref.Code
			if err := s.store.SetZFSMemberOutcome(d.ID, m.Entry.Name, ref.Code, 0); err != nil {
				log.Printf("api: zfs: recording the probe of %s failed: %v", m.Entry.Name, err)
			}
			if check.Code == "ok" {
				check.Code = ref.Code
				check.Detail = ref.Detail
			}
		}
	}
	s.recordZFSCheck(d, check)
	return check
}

// recordZFSCheck puts the newest verdict on the row, so the page shows it
// without asking the host again.
func (s *Service) recordZFSCheck(d store.ZFSDataset, check ZFSCheck) {
	mountpoint := check.HostMountpoint
	if mountpoint == "" {
		mountpoint = d.LastHostMountpoint
	}
	if _, err := s.store.SetZFSCheck(d.ID, check.Code, check.Detail, mountpoint, time.Now().Unix()); err != nil {
		log.Printf("api: zfs: recording the check of %s failed: %v", d.Dataset, err)
	}
}

// zfsVMOwned reads the defined VMs once and reports which directories hold
// their disk images and which volumes they use. A dataset of either kind is
// already backed up on the VMs page, so the add dialog leaves it out.
func (s *Service) zfsVMOwned(ctx context.Context) (dirs map[string]bool, volumes map[string]bool) {
	dirs, volumes = map[string]bool{}, map[string]bool{}
	if s.virsh == nil {
		return dirs, volumes
	}
	vms, err := s.virsh.List(ctx)
	if err != nil {
		log.Printf("api: zfs: the host listing could not ask libvirt which datasets belong to a VM: %v", err)
		return dirs, volumes
	}
	for _, vm := range vms {
		xml, xErr := s.virsh.DumpXML(ctx, vm.Name)
		if xErr != nil {
			continue
		}
		info, pErr := virshcli.ParseDomain(xml)
		if pErr != nil {
			continue
		}
		for _, disk := range info.DiskPaths {
			dirs[path.Dir(disk)] = true
		}
		for _, disk := range info.BlockDisks {
			if dataset, ok := virshcli.ZvolDatasetFromDevPath(disk.Source); ok {
				volumes[dataset] = true
			}
		}
	}
	return dirs, volumes
}

// zfsVMDiskDatasets names the datasets a VM keeps its disk images on: for each
// disk directory the mounted filesystem with the deepest mountpoint above it.
// Its ancestors only contain that dataset, and flagging them would leave their
// other children out of the add dialog's default.
func zfsVMDiskDatasets(list []zfs.ListEntry, dirs map[string]bool) map[string]bool {
	out := map[string]bool{}
	for dir := range dirs {
		owner, depth := "", -1
		for _, e := range list {
			if e.Type != "filesystem" || !e.Mounted || !strings.HasPrefix(e.Mountpoint, "/") {
				continue
			}
			if (dir == e.Mountpoint || strings.HasPrefix(dir, e.Mountpoint+"/")) && len(e.Mountpoint) > depth {
				owner, depth = e.Name, len(e.Mountpoint)
			}
		}
		if owner != "" {
			out[owner] = true
		}
	}
	return out
}

// zfsHoldsSystemImage reports whether a dataset carries Docker's or libvirt's
// image file, which belong to those domains rather than to a file backup.
func zfsHoldsSystemImage(containerPath string) bool {
	if containerPath == "" {
		return false
	}
	for _, name := range []string{"docker.img", "libvirt.img"} {
		if _, err := zfsStat(containerPath + "/" + name); err == nil {
			return true
		}
	}
	return false
}

// zfsLegacyCounts counts, per dataset, how many legacy filesystems lie in its
// subtree. Docker's ZFS storage driver is what a large count means.
func zfsLegacyCounts(list []zfs.ListEntry) map[string]int {
	counts := map[string]int{}
	for _, e := range list {
		if e.Type != "filesystem" || e.Mountpoint != "legacy" {
			continue
		}
		for name := e.Name; ; {
			counts[name]++
			cut := strings.LastIndex(name, "/")
			if cut <= 0 {
				break
			}
			name = name[:cut]
		}
	}
	return counts
}

// zfsRootBlockers names what keeps a dataset from becoming an item of its own.
// An empty list means the add dialog may offer it.
func zfsRootBlockers(e zfs.ListEntry, entry ZFSDiscoveredEntry, rows []store.ZFSDataset, legacy int) []string {
	out := []string{}
	if e.Type != "filesystem" {
		out = append(out, "not-filesystem")
	}
	if err := zfs.ValidateDatasetName(e.Name); err != nil {
		var ne *zfs.NameError
		code := "invalid-name"
		if errors.As(err, &ne) {
			code = ne.Code
		}
		out = append(out, code)
	}
	if entry.ManagedID != "" || entry.CoveredBy != "" {
		out = append(out, "overlaps-item")
	} else {
		for _, d := range rows {
			if zfs.DescendantOf(d.Dataset, e.Name) {
				out = append(out, "overlaps-item")
				break
			}
		}
	}
	if legacy > zfsMaxLegacyFilesystems {
		out = append(out, "docker-storage")
	}
	return out
}

// zfsSSHTarget is the machine the domain talks to, as the card prints it.
func (s *Service) zfsSSHTarget() string {
	return s.cfg.LibvirtSSHUser + "@" + s.cfg.LibvirtHost + ":" + s.cfg.LibvirtSSHPort
}

// zfsURITarget is the target LIBVIRT_URI names, empty when none is set.
func (s *Service) zfsURITarget() string {
	if s.cfg.LibvirtURIHost == "" {
		return ""
	}
	port := s.cfg.LibvirtURIPort
	if port == "" {
		port = "22"
	}
	user := s.cfg.LibvirtURIUser
	if user == "" {
		user = "root"
	}
	return user + "@" + s.cfg.LibvirtURIHost + ":" + port
}

// zfsURIMismatch reports that the SSH target and the libvirt URI name different
// machines, which means one of the two domains is talking to the wrong host.
func (s *Service) zfsURIMismatch() bool {
	if s.cfg.LibvirtURIHost == "" {
		return false
	}
	if s.cfg.LibvirtURIHost != s.cfg.LibvirtHost {
		return true
	}
	return s.cfg.LibvirtURIUser != "" && s.cfg.LibvirtURIUser != s.cfg.LibvirtSSHUser
}

// zfsErrCode is the reason code a failed host call carries.
func zfsErrCode(err error) string {
	var ce *zfs.CmdError
	if errors.As(err, &ce) && ce.Code != "" {
		return ce.Code
	}
	return "zfs-error"
}

// zfsDetail prepares host output for a details block and a refusal, which the
// scrubber lets through: control characters and paths out, cut to what one is
// worth reading.
func zfsDetail(text string) string {
	clean := scrubSecrets(strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || r >= 0x20 {
			return r
		}
		return -1
	}, text))
	if len(clean) > zfsDetailLimit {
		clean = clean[:zfsDetailLimit]
	}
	return strings.TrimSpace(clean)
}
