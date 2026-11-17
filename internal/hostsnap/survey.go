package hostsnap

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// Survey lists the snapshots of every kind the host has. The disks of a
// pool that sleeps are left alone unless wake is set.
func (h *Host) Survey(ctx context.Context, wake bool) Survey {
	s := Survey{Kinds: map[Kind]KindState{}, Pools: []Pool{}, Sources: []Source{}, Snapshots: []Snapshot{}}
	var wg sync.WaitGroup
	var vm vmPart
	var unraid map[string]string
	if h.vms != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			vm = h.surveyVMs(ctx)
		}()
	} else {
		s.Kinds[KindVM] = KindState{Code: "missing"}
	}

	out, stderr, err := h.sh.RunCapture(ctx, "sh", "-c", surveyScript, "bombvault-snapshots", boolArg(wake), "full")
	if err != nil {
		code := zfs.Classify(stderr, err)
		s.Kinds[KindZFS] = KindState{Code: code, Detail: clip(stderr)}
		s.Kinds[KindBtrfs] = KindState{Code: code, Detail: clip(stderr)}
	} else {
		raw := parseSurvey(out)
		raw.fill(&s, wake)
		h.addZFSDetails(ctx, &s)
		unraid = raw.unraid
	}
	wg.Wait()
	if h.vms != nil {
		vm.unraid = unraid
		vm.fill(&s)
	}
	sortSnapshots(s.Snapshots)
	return s
}

func boolArg(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// clip keeps a host message short enough for a details block.
func clip(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 500 {
		s = s[:500]
	}
	return s
}

// rawSurvey is the survey script's output sorted by record.
type rawSurvey struct {
	offset    *time.Location
	tools     map[Kind]string
	zfsPools  []string
	devices   map[string][]devState // by kind and pool, "zfs\x00tank"
	after     map[string][]devState
	asleep    map[string]bool
	datasets  []zfsDataset
	snaps     []zfsSnap
	errs      map[string]string
	btrfs     []*btrfsFS
	btrfsByID map[string]*btrfsFS
	unraid    map[string]string
}

type devState struct{ dev, state string }

type zfsDataset struct {
	name, typ, mountpoint string
	used, written         int64
}

type zfsSnap struct {
	dataset, name         string
	creation, used, refer int64
	userrefs              int
	clones                []string
}

func poolKey(k Kind, name string) string { return string(k) + "\x00" + name }

func parseSurvey(out string) *rawSurvey {
	r := &rawSurvey{
		offset: time.UTC, tools: map[Kind]string{}, devices: map[string][]devState{}, after: map[string][]devState{},
		asleep: map[string]bool{}, errs: map[string]string{}, btrfsByID: map[string]*btrfsFS{}, unraid: map[string]string{},
	}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(f) < 2 {
			continue
		}
		switch f[0] {
		case "Z":
			if loc, ok := parseOffset(f[1]); ok {
				r.offset = loc
			}
		case "T":
			if len(f) == 3 {
				r.tools[Kind(f[1])] = f[2]
			}
		case "D", "A", "BD", "BA":
			r.addDevice(f)
		case "P":
			if len(f) == 4 {
				r.zfsPools = append(r.zfsPools, f[2])
				r.asleep[poolKey(KindZFS, f[2])] = f[3] == "1"
			}
		case "BP":
			if len(f) == 3 {
				r.asleep[poolKey(KindBtrfs, mountUnescape(f[1]))] = f[2] == "1"
			}
		case "F":
			if d, ok := parseDataset(f[1:]); ok {
				r.datasets = append(r.datasets, d)
			}
		case "S":
			if sn, ok := parseZFSSnap(f[1:]); ok {
				r.snaps = append(r.snaps, sn)
			}
		case "E":
			if len(f) >= 3 {
				r.errs[strings.Join(f[1:len(f)-1], "\t")] = f[len(f)-1]
			}
		case "BF":
			if len(f) == 4 {
				fs := &btrfsFS{esc: f[1], mount: mountUnescape(f[1]), device: f[2], top: f[3] == "1", subvols: map[string]btrfsSubvol{}, readOnly: map[string]bool{}, qgroups: map[string][2]int64{}}
				r.btrfs = append(r.btrfs, fs)
				r.btrfsByID[f[1]] = fs
			}
		case "BT", "BL", "BS", "BR", "BQ":
			if len(f) >= 3 {
				if fs := r.btrfsByID[f[1]]; fs != nil {
					fs.add(f[0], strings.Join(f[2:], "\t"))
				}
			}
		case "U":
			if len(f) == 3 {
				r.unraid[f[1]] = f[2]
			}
		}
	}
	return r
}

func (r *rawSurvey) addDevice(f []string) {
	var kind Kind
	var pool, dev, state string
	switch f[0] {
	case "D", "A":
		if len(f) != 5 {
			return
		}
		kind, pool, dev, state = Kind(f[1]), f[2], f[3], f[4]
	default:
		if len(f) != 4 {
			return
		}
		kind, pool, dev, state = KindBtrfs, mountUnescape(f[1]), f[2], f[3]
	}
	key := poolKey(kind, pool)
	if f[0] == "A" || f[0] == "BA" {
		r.after[key] = append(r.after[key], devState{dev, state})
		return
	}
	r.devices[key] = append(r.devices[key], devState{dev, state})
}

func parseOffset(s string) (*time.Location, bool) {
	t, err := time.Parse("-0700", strings.TrimSpace(s))
	if err != nil {
		return nil, false
	}
	return t.Location(), true
}

func parseDataset(f []string) (zfsDataset, bool) {
	if len(f) != 5 {
		return zfsDataset{}, false
	}
	d := zfsDataset{name: f[0], typ: f[1], mountpoint: f[2]}
	d.used = num(f[3])
	d.written = num(f[4])
	return d, true
}

func parseZFSSnap(f []string) (zfsSnap, bool) {
	if len(f) != 6 {
		return zfsSnap{}, false
	}
	ds, name, ok := strings.Cut(f[0], "@")
	if !ok || ds == "" || name == "" {
		return zfsSnap{}, false
	}
	s := zfsSnap{dataset: ds, name: name, creation: num(f[1]), used: num(f[2]), refer: num(f[3])}
	s.userrefs = int(num(f[4]))
	if c := strings.TrimSpace(f[5]); c != "" && c != "-" {
		s.clones = strings.Split(c, ",")
	}
	return s, true
}

// num reads a parsable zfs number, -1 for "-" or anything else.
func num(s string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return -1
	}
	return n
}

// fill turns the records into the page's survey.
func (r *rawSurvey) fill(s *Survey, wake bool) {
	s.Kinds[KindZFS] = r.kindState(KindZFS, []string{"F", "S"})
	var btrfsLists []string
	for _, fs := range r.btrfs {
		btrfsLists = append(btrfsLists, "BL\t"+fs.esc, "BS\t"+fs.esc)
	}
	s.Kinds[KindBtrfs] = r.kindState(KindBtrfs, btrfsLists)
	for _, pool := range r.zfsPools {
		s.Pools = append(s.Pools, r.pool(KindZFS, pool, wake))
	}
	legacy := map[string]bool{}
	for _, d := range r.datasets {
		if d.typ == "filesystem" && d.mountpoint == "legacy" {
			legacy[d.name] = true
			continue
		}
		pool, _, _ := strings.Cut(d.name, "/")
		s.Sources = append(s.Sources, Source{
			Kind: KindZFS, Name: d.name, Group: pool, Used: d.used, NewSinceLatest: d.written,
			Recursive: true, Create: true,
		})
	}
	for _, sn := range r.snaps {
		if legacy[sn.dataset] {
			s.HiddenSystem++
			continue
		}
		s.Snapshots = append(s.Snapshots, zfsSnapshot(sn))
	}
	for _, fs := range r.btrfs {
		s.Pools = append(s.Pools, r.pool(KindBtrfs, fs.mount, wake))
		if !r.read(KindBtrfs, fs.mount, wake) {
			continue
		}
		hidden := fs.fill(s, r.offset)
		s.HiddenSystem += hidden
	}
}

func (r *rawSurvey) kindState(k Kind, errKeys []string) KindState {
	switch r.tools[k] {
	case "ok":
	case "":
		return KindState{Code: "missing"}
	default:
		return KindState{Code: r.tools[k]}
	}
	for _, key := range errKeys {
		if msg, ok := r.errs[key]; ok {
			return KindState{Code: zfs.Classify(msg, nil), Detail: clip(msg)}
		}
	}
	return KindState{Code: "ok"}
}

func (r *rawSurvey) read(k Kind, name string, wake bool) bool {
	return wake || !r.asleep[poolKey(k, name)]
}

func (r *rawSurvey) pool(k Kind, name string, wake bool) Pool {
	key := poolKey(k, name)
	p := Pool{Kind: k, Name: name, State: StateAwake, Read: r.read(k, name, wake)}
	unknown := false
	for _, d := range r.devices[key] {
		switch d.state {
		case "asleep":
			p.State = StateAsleep
		case "unknown":
			unknown = true
		}
	}
	if p.State != StateAsleep && unknown {
		p.State = StateUnknown
	}
	if p.State == StateAsleep && wake {
		before := map[string]string{}
		for _, d := range r.devices[key] {
			before[d.dev] = d.state
		}
		for _, d := range r.after[key] {
			if before[d.dev] == "asleep" && d.state == "awake" {
				p.Woke = true
			}
		}
	}
	return p
}

func zfsSnapshot(sn zfsSnap) Snapshot {
	s := Snapshot{
		Ref:       Ref{Kind: KindZFS, Source: sn.dataset, Name: sn.name},
		Label:     sn.name,
		CreatedAt: sn.creation, Used: sn.used, Referenced: sn.refer, NewSince: -1,
		Holds: []string{}, Clones: sn.clones, Owner: Owner(sn.name), ReadOnly: true,
	}
	if s.Clones == nil {
		s.Clones = []string{}
	}
	if s.Owner == "plan" {
		s.PlanID, _ = PlanOf(sn.name)
	}
	if sn.userrefs > 0 {
		// The tags themselves come from the detail pass; until then the
		// count alone stands for them.
		for i := 0; i < sn.userrefs; i++ {
			s.Holds = append(s.Holds, "")
		}
	}
	if !managedByRuns(sn.name) {
		s.Caps = Caps{Rename: true, Hold: true, Delete: true, Estimate: true}
	}
	return s
}

// addZFSDetails reads what the listing leaves out, in one more round trip:
// how much was written to each dataset since each of its snapshots, and the
// tags of the holds. It is best effort; a value it cannot read stays unknown.
func (h *Host) addZFSDetails(ctx context.Context, s *Survey) {
	var script strings.Builder
	script.WriteString(preamble)
	byDataset := map[string][]string{}
	var order []string
	var held []string
	for _, sn := range s.Snapshots {
		if sn.Kind != KindZFS {
			continue
		}
		if _, ok := byDataset[sn.Source]; !ok {
			order = append(order, sn.Source)
		}
		byDataset[sn.Source] = append(byDataset[sn.Source], "written@"+sn.Name)
		if len(sn.Holds) > 0 {
			held = append(held, sn.Source+"@"+sn.Name)
		}
	}
	if len(order) == 0 {
		return
	}
	for _, ds := range order {
		script.WriteString(`"$ZFS" get -H -p -o name,property,value ` + shQuote(strings.Join(byDataset[ds], ",")) + " " + shQuote(ds) + ` 2>/dev/null | awk '{ print "W\t" $0 }'` + "\n")
	}
	if len(held) > 0 {
		script.WriteString(`"$ZFS" holds -H`)
		for _, n := range held {
			script.WriteString(" " + shQuote(n))
		}
		script.WriteString(` 2>/dev/null | awk '{ print "H\t" $0 }'` + "\n")
	}
	out, _, err := h.sh.RunCapture(ctx, "sh", "-c", script.String(), "bombvault-snapshots")
	if err != nil && out == "" {
		return
	}
	written := map[string]int64{}
	holds := map[string][]string{}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Split(line, "\t")
		switch {
		case len(f) == 4 && f[0] == "W":
			if name, ok := strings.CutPrefix(f[2], "written@"); ok {
				written[f[1]+"@"+name] = num(f[3])
			}
		case len(f) >= 3 && f[0] == "H":
			holds[f[1]] = append(holds[f[1]], f[2])
		}
	}
	for i := range s.Snapshots {
		sn := &s.Snapshots[i]
		if sn.Kind != KindZFS {
			continue
		}
		full := sn.Source + "@" + sn.Name
		if v, ok := written[full]; ok {
			sn.NewSince = v
		}
		if tags, ok := holds[full]; ok {
			sn.Holds = tags
		}
	}
}

// btrfsFS is one btrfs filesystem as the survey read it through the mount of
// its top level.
type btrfsFS struct {
	esc, mount string
	device     string
	top        bool
	topUUID    string
	subvols    map[string]btrfsSubvol // by uuid
	snaps      []btrfsSubvol
	readOnly   map[string]bool
	qgroups    map[string][2]int64 // "0/<id>" -> referenced, exclusive
	quotas     bool
}

type btrfsSubvol struct {
	id, parentUUID, uuid, path string
	otime                      string
}

var (
	btrfsKeyRe = regexp.MustCompile(`(?:^|\s)(otime \S+|ID|parent_uuid|uuid|top level|parent|gen|cgen|received_uuid) (\S+)`)
	btrfsPath  = regexp.MustCompile(`\spath (.*)$`)
	btrfsUUID  = regexp.MustCompile(`^\s*UUID:\s*(\S+)`)
)

func (fs *btrfsFS) add(rec, line string) {
	switch rec {
	case "BT":
		if m := btrfsUUID.FindStringSubmatch(line); m != nil {
			fs.topUUID = m[1]
		}
	case "BL", "BS", "BR":
		sv, ok := parseBtrfsLine(line)
		if !ok {
			return
		}
		switch rec {
		case "BL":
			fs.subvols[sv.uuid] = sv
		case "BS":
			fs.snaps = append(fs.snaps, sv)
		case "BR":
			fs.readOnly[sv.uuid] = true
		}
	case "BQ":
		f := strings.Fields(line)
		if len(f) >= 3 && strings.Contains(f[0], "/") {
			rfer, excl := num(f[1]), num(f[2])
			if rfer >= 0 && excl >= 0 {
				fs.qgroups[f[0]] = [2]int64{rfer, excl}
				fs.quotas = true
			}
		}
	}
}

func parseBtrfsLine(line string) (btrfsSubvol, bool) {
	pm := btrfsPath.FindStringSubmatch(line)
	if pm == nil {
		return btrfsSubvol{}, false
	}
	head := strings.TrimSuffix(line, pm[0])
	var sv btrfsSubvol
	sv.path = strings.TrimPrefix(pm[1], "<FS_TREE>/")
	for _, m := range btrfsKeyRe.FindAllStringSubmatch(head, -1) {
		key, val := m[1], m[2]
		switch {
		case key == "ID":
			sv.id = val
		case key == "uuid":
			sv.uuid = val
		case key == "parent_uuid":
			sv.parentUUID = val
		case strings.HasPrefix(key, "otime "):
			sv.otime = strings.TrimPrefix(key, "otime ") + " " + val
		}
	}
	if sv.id == "" {
		return btrfsSubvol{}, false
	}
	if sv.uuid == "" {
		sv.uuid = "id:" + sv.id
	}
	return sv, true
}

// systemSubvol reports whether a subvolume belongs to Docker's storage
// driver, whose layers would bury everything else.
func systemSubvol(p string) bool {
	return strings.Contains("/"+p, "/docker/btrfs/subvolumes/") || strings.Contains("/"+p, "/docker/btrfs/snapshots/")
}

// fill adds the filesystem's sources and snapshots and returns how many
// system snapshots it left out.
func (fs *btrfsFS) fill(s *Survey, loc *time.Location) int {
	if !fs.top {
		// Only a mounted top level makes every subvolume reachable by a path
		// this package can build.
		return 0
	}
	snapUUID := map[string]bool{}
	for _, sn := range fs.snaps {
		snapUUID[sn.uuid] = true
	}
	s.Sources = append(s.Sources, Source{Kind: KindBtrfs, FS: fs.mount, Name: "", Group: fs.mount, Used: -1, NewSinceLatest: -1, Create: true})
	for _, sv := range fs.subvols {
		if snapUUID[sv.uuid] || systemSubvol(sv.path) || strings.HasPrefix(sv.path, BtrfsFolder+"/") {
			continue
		}
		s.Sources = append(s.Sources, Source{Kind: KindBtrfs, FS: fs.mount, Name: sv.path, Group: fs.mount, Used: fs.size(sv.id, 0), NewSinceLatest: -1, Create: true})
	}
	hidden := 0
	for _, sn := range fs.snaps {
		if systemSubvol(sn.path) {
			hidden++
			continue
		}
		// A source that is gone shows as "?"; the top level has no path.
		source := "?"
		if p, ok := fs.subvols[sn.parentUUID]; ok {
			source = p.path
		} else if fs.topUUID != "" && fs.topUUID != "-" && sn.parentUUID == fs.topUUID {
			source = ""
		}
		label := path.Base(sn.path)
		snap := Snapshot{
			Ref:       Ref{Kind: KindBtrfs, FS: fs.mount, Source: source, Name: sn.path},
			Label:     label,
			CreatedAt: btrfsTime(sn.otime, loc),
			Used:      fs.size(sn.id, 1), Referenced: fs.size(sn.id, 0), NewSince: -1,
			Holds: []string{}, Clones: []string{}, Owner: Owner(label), ReadOnly: fs.readOnly[sn.uuid],
			Caps: Caps{Rename: true, Delete: true, Estimate: fs.quotas},
		}
		if snap.Owner == "plan" {
			snap.PlanID, _ = PlanOf(label)
		}
		s.Snapshots = append(s.Snapshots, snap)
	}
	return hidden
}

func (fs *btrfsFS) size(id string, i int) int64 {
	if q, ok := fs.qgroups["0/"+id]; ok {
		return q[i]
	}
	return -1
}

// btrfsTime reads an otime, which btrfs prints in the host's local time
// without a zone.
func btrfsTime(s string, loc *time.Location) int64 {
	t, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(s), loc)
	if err != nil {
		return 0
	}
	return t.Unix()
}

// vmPart is what the survey of the VMs found.
type vmPart struct {
	state   KindState
	domains []string
	snaps   []Snapshot
	unraid  map[string]string
}

// vmListWorkers bounds the virsh calls in flight. Over qemu+ssh each one is a
// handshake of its own.
const vmListWorkers = 4

func (h *Host) surveyVMs(ctx context.Context) vmPart {
	names, err := h.vms.DomainNames(ctx)
	if err != nil {
		return vmPart{state: KindState{Code: "vm-unreachable", Detail: clip(err.Error())}}
	}
	part := vmPart{state: KindState{Code: "ok"}, domains: names}
	results := make([][]Snapshot, len(names))
	sem := make(chan struct{}, vmListWorkers)
	var wg sync.WaitGroup
	for i, name := range names {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			table, err := h.vms.SnapshotTable(ctx, name)
			if err == nil {
				results[i] = parseVMSnapshots(name, table)
			}
		}()
	}
	wg.Wait()
	for _, r := range results {
		part.snaps = append(part.snaps, r...)
	}
	return part
}

func (p vmPart) fill(s *Survey) {
	if p.state.Code == "" {
		p.state.Code = "ok"
	}
	s.Kinds[KindVM] = p.state
	for _, d := range p.domains {
		s.Sources = append(s.Sources, Source{Kind: KindVM, Name: d, Group: "vm", Used: -1, NewSinceLatest: -1, Create: true})
	}
	known := map[string]bool{}
	for _, sn := range p.snaps {
		known[sn.Source+"\x00"+sn.Name] = true
		s.Snapshots = append(s.Snapshots, sn)
	}
	for vm, b64 := range p.unraid {
		for _, sn := range parseUnraidDB(vm, b64) {
			if !known[sn.Source+"\x00"+sn.Name] {
				s.Snapshots = append(s.Snapshots, sn)
			}
		}
	}
}

var vmRowRe = regexp.MustCompile(`^\s*(.+?)\s+(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} [+-]\d{4})\s+(\S.*?)\s*$`)

// parseVMSnapshots reads virsh's snapshot-list table. The creation time is
// the anchor, so a name with spaces in it stays whole.
func parseVMSnapshots(domain, table string) []Snapshot {
	var out []Snapshot
	for _, line := range strings.Split(table, "\n") {
		m := vmRowRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		t, err := time.Parse("2006-01-02 15:04:05 -0700", m[2])
		if err != nil {
			continue
		}
		out = append(out, Snapshot{
			Ref:       Ref{Kind: KindVM, Source: domain, Name: m[1]},
			Label:     m[1],
			CreatedAt: t.Unix(), Used: -1, Referenced: -1, NewSince: -1,
			Holds: []string{}, Clones: []string{}, Owner: Owner(m[1]), ReadOnly: true,
			Caps: Caps{Delete: true},
		})
	}
	return out
}

// parseUnraidDB reads the snapshot list Unraid keeps for a VM. Its VM
// manager takes those snapshots without libvirt metadata and merges them back
// itself, so they are listed here and changed there.
func parseUnraidDB(vm, b64 string) []Snapshot {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil
	}
	var db map[string]map[string]any
	if err := json.Unmarshal(raw, &db); err != nil {
		return nil
	}
	var out []Snapshot
	for name, entry := range db {
		var created int64
		for _, key := range []string{"creationtime", "creationTime", "created"} {
			switch v := entry[key].(type) {
			case float64:
				created = int64(v)
			case string:
				created = num(v)
			}
			if created > 0 {
				break
			}
		}
		if created < 0 {
			created = 0
		}
		out = append(out, Snapshot{
			Ref:       Ref{Kind: KindVM, Source: vm, Name: name},
			Label:     name,
			CreatedAt: created, Used: -1, Referenced: -1, NewSince: -1,
			Holds: []string{}, Clones: []string{}, ReadOnly: true, ManagedBy: "unraid",
		})
	}
	return out
}
