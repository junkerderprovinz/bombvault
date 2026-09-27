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

// Kinds of change in a restore plan. An extra path is there now and not in the
// backup; the restore leaves it alone.
const (
	changeAdded   = "added"
	changeChanged = "changed"
	changeRemoved = "removed"
	changeExtra   = "extra"
)

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
}

// PlanFile is one host path of the plan and how the restore changes it.
type PlanFile struct {
	Path   string `json:"path"`
	Change string `json:"change"`
}

// DefinitionChange is one difference between the definition a restore
// recreates and the one in use now. Backup and Now hold the two values; one
// is empty when the entry exists on one side only. An environment variable
// carries its name alone, because its value may be a password.
type DefinitionChange struct {
	Field  string `json:"field"`
	Name   string `json:"name,omitempty"`
	Change string `json:"change"`
	Backup string `json:"backup,omitempty"`
	Now    string `json:"now,omitempty"`
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
	add := func(p, change string) {
		if listed[change] >= planListCap {
			plan.ListCapped = true
			return
		}
		listed[change]++
		plan.Files = append(plan.Files, PlanFile{Path: s.toHostPath(p), Change: change})
	}
	budget := previewBudget
	if d, ok := ctx.Value(previewBudgetKey{}).(time.Duration); ok {
		budget = d
	}
	pctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	for _, st := range sc.steps {
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
				plan.Added++
				need[st.Target] += it.Size
				add(p, changeAdded)
			case "updated":
				plan.Changed++
				grow := it.Size
				if fi, err := os.Lstat(p); err == nil {
					grow -= fi.Size()
				}
				if grow > 0 {
					need[st.Target] += grow
				}
				add(p, changeChanged)
			case "unchanged":
				plan.Unchanged++
			case "deleted":
				plan.Extra++
				add(p, changeExtra)
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
	return plan, need, nil
}

// spaceLine compares what the restore writes with the free space of each
// volume it writes to. A measured shortfall fails even when the dry run
// stopped early, since the real need is only larger; an incomplete
// measurement that fits proves nothing and is left grey.
func (s *Service) spaceLine(sc restoreScope, need map[string]int64, incomplete bool) CheckLine {
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
	case len(sc.steps) == 0:
		line.Status, line.Reason = lineSkip, reasonNothing
		return line
	}
	type volume struct{ need, free int64 }
	volumes := map[string]*volume{}
	for target, n := range need {
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
		if v.need > v.free {
			line.Status, line.Reason = lineFail, reasonShort
			line.Need, line.Free = v.need, v.free
			return line
		}
		if v.need > line.Need {
			line.Need, line.Free = v.need, v.free
		}
	}
	if incomplete {
		line.Status, line.Reason = lineSkip, reasonUnmeasured
	}
	return line
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

func containerChanges(backup, now model.Inspect) []DefinitionChange {
	out := []DefinitionChange{}
	if a, b := containerImage(backup), containerImage(now); a != b {
		out = append(out, DefinitionChange{Field: "image", Change: changeChanged, Backup: a, Now: b})
	}
	out = append(out, setChanges("port", portList(backup.HostConfig.PortBindings), portList(now.HostConfig.PortBindings))...)
	out = append(out, envChanges(backup.Config.Env, now.Config.Env)...)
	out = append(out, setChanges("volume", backup.HostConfig.Binds, now.HostConfig.Binds)...)
	return out
}

func containerImage(in model.Inspect) string {
	if in.Config.Image != "" {
		return in.Config.Image
	}
	return in.Image
}

// portList spells each published port as "host -> container".
func portList(bindings map[string][]model.PortBinding) []string {
	var out []string
	for port, binds := range bindings {
		for _, b := range binds {
			host := b.HostPort
			if b.HostIP != "" && b.HostIP != "0.0.0.0" {
				host = b.HostIP + ":" + host
			}
			out = append(out, host+" -> "+port)
		}
	}
	return out
}

// setChanges lists what the restore adds (in the backup only) and removes (in
// use now only).
func setChanges(field string, backup, now []string) []DefinitionChange {
	out := []DefinitionChange{}
	for _, v := range sortedUnique(backup) {
		if !slices.Contains(now, v) {
			out = append(out, DefinitionChange{Field: field, Change: changeAdded, Backup: v})
		}
	}
	for _, v := range sortedUnique(now) {
		if !slices.Contains(backup, v) {
			out = append(out, DefinitionChange{Field: field, Change: changeRemoved, Now: v})
		}
	}
	return out
}

func envChanges(backup, now []string) []DefinitionChange {
	b, n := envMap(backup), envMap(now)
	names := make([]string, 0, len(b)+len(n))
	for k := range b {
		names = append(names, k)
	}
	for k := range n {
		if _, ok := b[k]; !ok {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	out := []DefinitionChange{}
	for _, k := range names {
		bv, inB := b[k]
		nv, inN := n[k]
		switch {
		case inB && !inN:
			out = append(out, DefinitionChange{Field: "env", Name: k, Change: changeAdded})
		case !inB && inN:
			out = append(out, DefinitionChange{Field: "env", Name: k, Change: changeRemoved})
		case bv != nv:
			out = append(out, DefinitionChange{Field: "env", Name: k, Change: changeChanged})
		}
	}
	return out
}

func envMap(env []string) map[string]string {
	m := make(map[string]string, len(env))
	for _, e := range env {
		k, v, _ := strings.Cut(e, "=")
		m[k] = v
	}
	return m
}

func sortedUnique(in []string) []string {
	out := slices.Clone(in)
	sort.Strings(out)
	return slices.Compact(out)
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
