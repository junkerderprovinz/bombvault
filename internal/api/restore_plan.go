package api

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// previewBudget bounds the dry runs of one check together. A tree too large
// to walk in that time gets a partial plan that says so.
var previewBudget = 30 * time.Second

// minMemberBudget is the least a stack member gets of the budget it shares.
const minMemberBudget = 5 * time.Second

type previewBudgetKey struct{}

// withPreviewBudget gives the dry runs of one check a budget other than the
// default, for a check that is one of several answering a single request.
func withPreviewBudget(ctx context.Context, d time.Duration) context.Context {
	return context.WithValue(ctx, previewBudgetKey{}, d)
}

// planListCap is how many paths of each kind of change the plan lists.
const planListCap = 200

// changeExtra is the plan's own kind of change next to the definition's: a
// path that is there now and not in the backup, which the restore leaves alone.
const changeExtra = "extra"

// RestorePlan is what a restore would do compared with what is there now.
// Partial means the dry run stopped at the time budget and the counts are a
// lower bound; ListCapped means Files holds fewer paths than the counts.
type RestorePlan struct {
	InPlace    bool               `json:"inPlace"`
	Added      int                `json:"added"`
	Changed    int                `json:"changed"`
	Unchanged  int                `json:"unchanged"`
	Extra      int                `json:"extra"`
	Files      []PlanFile         `json:"files"`
	ListCapped bool               `json:"listCapped"`
	Partial    bool               `json:"partial"`
	Error      string             `json:"error,omitempty"`
	Missing    bool               `json:"missing"`
	Definition []DefinitionChange `json:"definition"`
	Shared     []SharedFolder     `json:"shared"`
	// Beside is set for a restore next to the original and says where its
	// copy goes.
	Beside *BesidePlan `json:"beside,omitempty"`
	// BackupFirst is set when the restore backs up the current state before
	// it replaces it.
	BackupFirst *BackupFirstPlan `json:"backupFirst,omitempty"`
	// Zvols are the VM disks on ZFS volumes, which the restore writes whole.
	Zvols []PlanZvol `json:"zvols,omitempty"`
}

// PlanZvol is a zvol a VM restore writes whole. Change is "changed" for a zvol
// it replaces, "added" for one it creates and empty when the host could not
// be read. Safety starts the name of the snapshot taken before a replaced
// zvol is written. Free is the room ZFS has for the zvol, -1 when unknown, and
// NoPool says the pool it belongs on is not on this server.
type PlanZvol struct {
	Dataset string `json:"dataset"`
	Size    int64  `json:"size"`
	Change  string `json:"change"`
	Safety  string `json:"safety,omitempty"`
	Free    int64  `json:"free"`
	NoPool  bool   `json:"noPool,omitempty"`
}

// PlanFile is one host path of the plan and how the restore changes it.
type PlanFile struct {
	Path   string `json:"path"`
	Change string `json:"change"`
}

// SharedFolder names the other containers that bind a folder the restore
// writes into.
type SharedFolder struct {
	Path       string   `json:"path"`
	Containers []string `json:"containers"`
}

// previewRestore runs the dry run of every step and returns the plan and the
// bytes each target needs: a new file whole, a replaced file only by what it
// grows. Directories are left out; restic reports one after its contents,
// which is how they are recognised.
func (s *Service) previewRestore(ctx context.Context, sc restoreScope) (RestorePlan, map[string]int64, error) {
	plan := RestorePlan{InPlace: sc.inPlace, Files: []PlanFile{}}
	need := map[string]int64{}
	listed := map[string]int{}
	add := func(hostPath, change string) {
		if listed[change] >= planListCap {
			plan.ListCapped = true
			return
		}
		listed[change]++
		plan.Files = append(plan.Files, PlanFile{Path: hostPath, Change: change})
	}
	// A new dataset is empty until the restore fills it, so its dry run
	// compares against an empty folder instead of whatever lies where it will
	// be mounted.
	var empty string
	if sc.newDataset != "" {
		dir, err := os.MkdirTemp("", "bombvault-plan-")
		if err != nil {
			return RestorePlan{}, nil, fmt.Errorf("plan a new dataset: %w", err)
		}
		defer func() { _ = os.RemoveAll(dir) }()
		empty = filepath.ToSlash(dir)
	}
	budget := previewBudget
	if d, ok := ctx.Value(previewBudgetKey{}).(time.Duration); ok {
		budget = d
	}
	pctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	for _, st := range sc.steps {
		// A target the restore adds nothing to still gets its free space shown.
		target := st.Target
		if _, ok := need[target]; !ok {
			need[target] = 0
		}
		shownAs := s.toHostPath(target)
		if empty != "" {
			st.Target = empty
			if target == "" {
				shownAs = sc.newDataset
			}
		}
		show := func(p string) string {
			if empty != "" {
				return path.Join(shownAs, strings.TrimPrefix(p, empty))
			}
			return s.toHostPath(p)
		}
		dirs := map[string]bool{}
		err := s.engine.RestorePreview(pctx, sc.ref.repo, st, sc.ref.mode, func(it restic.PreviewItem) {
			p := path.Join(st.Target, filepath.ToSlash(it.Item))
			if dirs[p] {
				return
			}
			for d := path.Dir(p); d != "/" && d != "." && !dirs[d]; d = path.Dir(d) {
				dirs[d] = true
			}
			switch it.Action {
			case "restored":
				// restic reports every directory it passes as restored, and an
				// empty one has no contents to give it away.
				if fi, err := os.Lstat(p); err == nil && fi.IsDir() {
					return
				}
				plan.Added++
				need[target] += it.Size
				add(show(p), changeAdded)
			case "updated":
				plan.Changed++
				grow := it.Size
				if fi, err := os.Lstat(p); err == nil {
					grow -= fi.Size()
				}
				if grow > 0 {
					need[target] += grow
				}
				add(show(p), changeChanged)
			case "unchanged":
				plan.Unchanged++
			case "deleted":
				plan.Extra++
				add(show(p), changeExtra)
			}
		})
		switch {
		case err == nil:
			continue
		case ctx.Err() != nil:
			return RestorePlan{}, nil, ctx.Err()
		case errors.Is(err, context.DeadlineExceeded) || pctx.Err() != nil:
			plan.Partial = true
		default:
			plan.Error = scrubError(err)
		}
		break
	}
	for _, img := range sc.images {
		if _, err := os.Lstat(img.Target); err == nil {
			plan.Changed++
			add(s.toHostPath(img.Target), changeChanged)
		} else {
			plan.Added++
			add(s.toHostPath(img.Target), changeAdded)
		}
		need[path.Dir(img.Target)] += blockImageCeiling(img.Disk)
	}
	for _, f := range sc.extraFiles {
		plan.Added++
		add(s.toHostPath(f), changeAdded)
	}
	if len(sc.zvols) > 0 {
		plan.Zvols = s.planZvols(ctx, sc.zvols)
	}
	return plan, need, nil
}

// planZvols reads from the host whether each zvol is there to be replaced and
// how much room ZFS has for it. A zvol that is not there yet is created with
// its parents, so the room is that of its nearest existing ancestor.
func (s *Service) planZvols(ctx context.Context, disks []backup.VMRestoreBlockDisk) []PlanZvol {
	var space map[string]zfsSpace
	if s.ssh != nil {
		space, _ = sshZFSHost{ssh: s.ssh}.Space(ctx)
	}
	out := make([]PlanZvol, len(disks))
	for i, d := range disks {
		z := PlanZvol{Dataset: d.Dataset, Size: d.Size, Free: -1}
		if space != nil {
			placeZvol(&z, space)
		}
		out[i] = z
	}
	return out
}

func placeZvol(z *PlanZvol, space map[string]zfsSpace) {
	if own, ok := space[z.Dataset]; ok && own.volume {
		z.Change, z.Safety, z.Free = changeChanged, z.Dataset+"@"+backup.ZvolSafetySnapshotPrefix, own.available
		return
	}
	z.Change = changeAdded
	for p := path.Dir(z.Dataset); p != "."; p = path.Dir(p) {
		if parent, ok := space[p]; ok {
			z.Free = parent.available
			return
		}
	}
	z.NoPool = true
}

// blockImageCeiling is the most a rebuilt disk image takes beside the one it
// replaces: the raw image at the guest's full size, and a converted copy of
// the same size for any other format. Holes and qcow2's own packing usually
// leave it far below that.
func blockImageCeiling(d backup.BlocksDisk) int64 {
	if d.Format != "" && d.Format != "raw" {
		return 2 * d.Size
	}
	return d.Size
}

// spaceLine compares what the restore writes with the free space of each
// volume it writes to, and each zvol's size with the room ZFS has for it. A
// measured shortfall fails even when the dry run stopped early, since the real
// need is only larger; an incomplete measurement that fits proves nothing and
// is left grey.
func (s *Service) spaceLine(sc restoreScope, need map[string]int64, zvols []PlanZvol, incomplete bool) CheckLine {
	line := CheckLine{ID: lineSpace}
	switch {
	case sc.download:
		line.Status, line.Reason = lineSkip, reasonDownload
		return line
	case sc.fixedAt != "":
		if sc.fixedNeed < 0 {
			line.Status, line.Reason = lineSkip, reasonUnmeasured
			return line
		}
		need = map[string]int64{sc.fixedAt: sc.fixedNeed}
	case len(sc.steps) == 0 && len(sc.images) == 0 && len(zvols) == 0:
		line.Status, line.Reason = lineSkip, reasonNothing
		return line
	}
	// zfs create reserves the whole size of a new zvol, so one that does not
	// fit is refused. A replaced zvol is left to the rule for disk images
	// below: zeros that compress away can make it fit after all.
	for _, z := range zvols {
		switch {
		case z.NoPool:
			line.Status, line.Reason, line.Detail = lineFail, reasonNoPool, zvolPool(z.Dataset)
			return line
		case z.Change == changeAdded && z.Size > z.Free:
			line.Status, line.Reason, line.Detail = lineFail, reasonShort, z.Dataset
			line.Need, line.Free = z.Size, z.Free
			return line
		}
	}
	type volume struct{ need, free int64 }
	volumes := map[string]*volume{}
	for target, n := range need {
		// A new dataset whose mount cannot be told yet has no volume to ask.
		if target == "" {
			line.Status, line.Reason = lineSkip, reasonUnmeasured
			return line
		}
		st, err := s.diskStatFn()(nearestExistingDir(target))
		if err != nil {
			line.Status, line.Reason = lineSkip, reasonUnmeasured
			return line
		}
		v := volumes[st.Volume]
		if v == nil {
			v = &volume{free: int64(min(st.Free, math.MaxInt64))} //nolint:gosec // G115: capped at MaxInt64
			volumes[st.Volume] = v
		}
		v.need += n
	}
	line.Status = lineOK
	for _, v := range volumes {
		// A disk image counted at its ceiling that does not fit may still fit
		// once its holes are left out, so that shortfall does not block.
		if v.need > v.free && len(sc.images) > 0 {
			line.Status, line.Reason = lineSkip, reasonUnmeasured
			return line
		}
		if v.need > v.free {
			line.Status, line.Reason = lineFail, reasonShort
			line.Need, line.Free = v.need, v.free
			return line
		}
		if v.need > line.Need || line.Free == 0 {
			line.Need, line.Free = v.need, v.free
		}
	}
	for _, z := range zvols {
		if z.Free < 0 || z.Size > z.Free {
			line.Status, line.Reason, line.Need, line.Free = lineSkip, reasonUnmeasured, 0, 0
			return line
		}
		if z.Size > line.Need || line.Free == 0 {
			line.Need, line.Free = z.Size, z.Free
		}
	}
	if incomplete {
		line.Status, line.Reason = lineSkip, reasonUnmeasured
	}
	return line
}

// zvolPool is the pool a zvol belongs on, the first part of its name.
func zvolPool(dataset string) string {
	pool, _, _ := strings.Cut(dataset, "/")
	return pool
}

// definitionChanges compares the container or VM definition the restore
// recreates with the one in use now.
func (s *Service) definitionChanges(ctx context.Context, sc restoreScope, plan *RestorePlan) {
	plan.Definition = []DefinitionChange{}
	switch {
	case sc.container != nil:
		live, found, err := s.liveContainer(ctx, sc.owner)
		if err != nil {
			return
		}
		if !found {
			plan.Missing = true
			return
		}
		plan.Definition = containerChanges(*sc.container, live)
	case sc.vmXML != "" && s.virsh != nil:
		vms, err := s.virsh.List(ctx)
		if err != nil {
			return
		}
		if !slices.ContainsFunc(vms, func(v virshcli.VMInfo) bool { return v.Name == sc.vmName }) {
			plan.Missing = true
			return
		}
		live, err := s.virsh.DumpXMLInactive(ctx, sc.vmName)
		if err != nil {
			return
		}
		plan.Definition = vmChanges(sc.vmXML, live)
	}
}

// liveContainer inspects the container by name, and says whether there is one.
func (s *Service) liveContainer(ctx context.Context, name string) (model.Inspect, bool, error) {
	list, err := s.docker.List(ctx)
	if err != nil {
		return model.Inspect{}, false, err
	}
	for _, c := range list {
		if c.Name == name {
			in, err := s.docker.Inspect(ctx, name)
			return in, err == nil, err
		}
	}
	return model.Inspect{}, false, nil
}

// domainShape is the part of a libvirt domain the plan compares.
type domainShape struct {
	Memory struct {
		Value int64  `xml:",chardata"`
		Unit  string `xml:"unit,attr"`
	} `xml:"memory"`
	VCPU  string `xml:"vcpu"`
	Disks []struct {
		Device string `xml:"device,attr"`
		Source struct {
			File string `xml:"file,attr"`
			Dev  string `xml:"dev,attr"`
		} `xml:"source"`
		Target struct {
			Dev string `xml:"dev,attr"`
		} `xml:"target"`
	} `xml:"devices>disk"`
	Interfaces []struct {
		MAC struct {
			Address string `xml:"address,attr"`
		} `xml:"mac"`
		Source struct {
			Bridge  string `xml:"bridge,attr"`
			Network string `xml:"network,attr"`
		} `xml:"source"`
	} `xml:"devices>interface"`
}

func vmChanges(backupXML, liveXML string) []DefinitionChange {
	var b, n domainShape
	if xml.Unmarshal([]byte(backupXML), &b) != nil || xml.Unmarshal([]byte(liveXML), &n) != nil {
		return []DefinitionChange{}
	}
	out := []DefinitionChange{}
	if a, c := memoryText(b), memoryText(n); a != c {
		out = append(out, DefinitionChange{Field: "memory", Change: changeChanged, Backup: a, Now: c})
	}
	if a, c := strings.TrimSpace(b.VCPU), strings.TrimSpace(n.VCPU); a != c {
		out = append(out, DefinitionChange{Field: "vcpus", Change: changeChanged, Backup: a, Now: c})
	}
	out = append(out, setChanges("disk", diskList(b), diskList(n))...)
	out = append(out, setChanges("network", nicList(b), nicList(n))...)
	return out
}

// memoryText spells the domain's memory in MiB, whatever unit it was written in.
func memoryText(d domainShape) string {
	kib := d.Memory.Value
	switch strings.ToLower(d.Memory.Unit) {
	case "b", "bytes":
		kib /= 1024
	case "m", "mib":
		kib *= 1024
	case "g", "gib":
		kib *= 1024 * 1024
	}
	return strconv.FormatInt(kib/1024, 10) + " MiB"
}

func diskList(d domainShape) []string {
	var out []string
	for _, disk := range d.Disks {
		src := disk.Source.File
		if src == "" {
			src = disk.Source.Dev
		}
		if src == "" {
			continue
		}
		out = append(out, disk.Target.Dev+": "+src)
	}
	return out
}

func nicList(d domainShape) []string {
	var out []string
	for _, nic := range d.Interfaces {
		src := nic.Source.Bridge
		if src == "" {
			src = nic.Source.Network
		}
		out = append(out, fmt.Sprintf("%s (%s)", nic.MAC.Address, src))
	}
	return out
}

// sharedFolders names the other containers, running or not, whose bind
// mounts overlap a folder an in-place restore writes into. A bind of a whole
// share or pool, which file managers and backup tools hold, is no sharing and
// would name them in every plan.
func (s *Service) sharedFolders(ctx context.Context, sc restoreScope) []SharedFolder {
	out := []SharedFolder{}
	if !sc.inPlace || len(sc.written) == 0 || s.docker == nil {
		return out
	}
	list, err := s.docker.List(ctx)
	if err != nil {
		return out
	}
	self, _ := s.docker.Self(ctx)
	for _, w := range sc.written {
		host := s.toHostPath(w)
		var names []string
		for _, c := range list {
			if c.Name == sc.owner || c.Name == self {
				continue
			}
			for _, m := range c.Mounts {
				if bindShares(host, m.Source) {
					names = append(names, c.Name)
					break
				}
			}
		}
		if len(names) > 0 {
			sort.Strings(names)
			out = append(out, SharedFolder{Path: host, Containers: names})
		}
	}
	return out
}

// guardRecreateOverRunning refuses to bring a container that is gone from
// the host back in place while another running container binds a folder the
// restore writes, since restic would overwrite that container's live files.
func (s *Service) guardRecreateOverRunning(ctx context.Context, name string, plan containerRestorePlan) error {
	if s.docker == nil {
		return nil
	}
	list, err := s.docker.List(ctx)
	if err != nil {
		return fmt.Errorf("list containers: %w", err)
	}
	if slices.ContainsFunc(list, func(c dockercli.ContainerInfo) bool { return c.Name == name }) {
		return nil
	}
	self, _ := s.docker.Self(ctx)
	for _, w := range containerScope(plan, name).written {
		host := s.toHostPath(w)
		for _, c := range list {
			if c.Name == self || !strings.EqualFold(c.State, "running") {
				continue
			}
			for _, m := range c.Mounts {
				if bindShares(host, m.Source) {
					return destinationRefusal("the running container %s uses %s, so bringing %s back there would write into its files. Stop %s or restore into a folder instead", c.Name, host, name, c.Name)
				}
			}
		}
	}
	return nil
}

// bindShares reports whether a bind mount reaches into the folder a restore
// writes. On Unraid /mnt/user/<share> and /mnt/<pool>/<share> are the same
// folder, so two paths compare below the share when either is a user share.
func bindShares(target, bind string) bool {
	if shareRoot(bind) {
		return false
	}
	if nested(target, bind) {
		return true
	}
	rt, userT, okT := shareRel(target)
	rb, userB, okB := shareRel(bind)
	return okT && okB && (userT || userB) && nested(rt, rb)
}

// shareRel is a path below /mnt/<pool or user>/, and whether it is a user share.
func shareRel(p string) (string, bool, bool) {
	segs := strings.Split(strings.TrimPrefix(path.Clean(p), "/"), "/")
	if len(segs) < 3 || segs[0] != "mnt" {
		return "", false, false
	}
	return "/" + strings.Join(segs[2:], "/"), segs[1] == "user" || segs[1] == "user0", true
}

// shareRoot is /, /mnt, a pool or /mnt/user, or a share directly below one.
func shareRoot(p string) bool {
	segs := strings.Split(strings.TrimPrefix(path.Clean(p), "/"), "/")
	if segs[0] == "mnt" {
		return len(segs) <= 3
	}
	return len(segs) <= 1
}

func nested(a, b string) bool {
	a, b = path.Clean(a), path.Clean(b)
	return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}
