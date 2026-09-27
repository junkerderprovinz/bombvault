package hostload

import (
	"bufio"
	"bytes"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Roles a disk can play in a backup.
const (
	RoleSource = "source"
	RoleTarget = "target"
	RoleBoth   = "both"
)

type mount struct {
	point, fsType string
	dev           string
}

// Resolver finds the block device under a path and gives it the name the
// user knows: on Unraid the mount under /mnt (disk1, cache) or parity.
type Resolver struct {
	mounts   []mount
	devs     map[string]string // "major:minor" to kernel name
	md       map[string]string // Unraid array device to the disk behind it
	parity   map[string]string
	hostRoot string
	exists   func(string) bool
}

// NewResolver reads the mount table, the device list and Unraid's array
// table below proc. hostRoot is where the host's /mnt is mounted.
func NewResolver(proc, hostRoot string) *Resolver {
	r := &Resolver{hostRoot: path.Clean(filepath.ToSlash(hostRoot)), exists: func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	}}
	if b, err := os.ReadFile(filepath.Join(proc, "self", "mountinfo")); err == nil { //nolint:gosec // G304: a fixed file below /proc
		r.mounts = parseMounts(b)
	}
	if b, err := os.ReadFile(filepath.Join(proc, "diskstats")); err == nil { //nolint:gosec // G304: a fixed file below /proc
		r.devs = parseDevNumbers(b)
	}
	if b, err := os.ReadFile(filepath.Join(proc, "mdstat")); err == nil { //nolint:gosec // G304: a fixed file below /proc
		r.md, r.parity = parseMdstat(b)
	}
	return r
}

func parseMounts(b []byte) []mount {
	var out []mount
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		sep := -1
		for i := 6; i < len(f); i++ {
			if f[i] == "-" {
				sep = i
				break
			}
		}
		if sep < 0 || sep+1 >= len(f) {
			continue
		}
		out = append(out, mount{point: path.Clean(unescape(f[4])), fsType: f[sep+1], dev: f[2]})
	}
	return out
}

// unescape decodes the octal escapes mountinfo writes for spaces and the like.
func unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			if v, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(v))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func parseDevNumbers(b []byte) map[string]string {
	out := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) >= 3 {
			out[f[0]+":"+f[1]] = f[2]
		}
	}
	return out
}

// parseMdstat reads Unraid's array table: diskName.N is the md device, and
// rdevName.N the disk behind it. Slots 0 and 29 are the parity disks.
func parseMdstat(b []byte) (md, parity map[string]string) {
	names, rdevs := map[string]string{}, map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), "=")
		if !ok || v == "" {
			continue
		}
		field, slot, ok := strings.Cut(k, ".")
		if !ok {
			continue
		}
		switch field {
		case "diskName":
			names[slot] = v
		case "rdevName":
			rdevs[slot] = v
		}
	}
	md, parity = map[string]string{}, map[string]string{}
	for slot, dev := range rdevs {
		switch slot {
		case "0":
			parity[dev] = "parity"
		case "29":
			parity[dev] = "parity2"
		default:
			if name := names[slot]; name != "" {
				md[name] = dev
			}
		}
	}
	return md, parity
}

var partitionSuffix = regexp.MustCompile(`^((?:sd|vd|xvd|hd)[a-z]+)\d+$|^((?:nvme\d+n\d+|mmcblk\d+))p\d+$`)

// wholeDisk maps a partition to its disk, the name the counters use.
func wholeDisk(name string) string {
	if m := partitionSuffix.FindStringSubmatch(name); m != nil {
		return m[1] + m[2]
	}
	return name
}

// mountOf is the deepest mount holding p.
func (r *Resolver) mountOf(p string) (mount, bool) {
	var best mount
	found := false
	for _, m := range r.mounts {
		if (p == m.point || strings.HasPrefix(p, strings.TrimSuffix(m.point, "/")+"/")) && (!found || len(m.point) > len(best.point)) {
			best, found = m, true
		}
	}
	return best, found
}

// DeviceOf returns the disk a path lives on. A path on an Unraid user share
// (shfs, no device of its own) counts for the one disk or pool that holds it,
// and for none when it is spread over several.
func (r *Resolver) DeviceOf(p string) (string, bool) {
	p = path.Clean(filepath.ToSlash(p))
	m, ok := r.mountOf(p)
	if !ok {
		return "", false
	}
	if strings.HasPrefix(m.fsType, "fuse.shfs") {
		rest := strings.TrimPrefix(strings.TrimPrefix(p, m.point), "/")
		var hit mount
		hits := 0
		for _, c := range r.mounts {
			if path.Dir(c.point) != path.Dir(m.point) || c.point == m.point || strings.HasPrefix(c.fsType, "fuse") || strings.HasPrefix(c.dev, "0:") {
				continue
			}
			if r.exists(path.Join(c.point, rest)) {
				hit = c
				hits++
			}
		}
		if hits != 1 {
			return "", false
		}
		m = hit
	}
	name, ok := r.devs[m.dev]
	if !ok {
		return "", false
	}
	if disk, ok := r.md[name]; ok {
		name = disk
	}
	return wholeDisk(name), true
}

// Label is the name a user knows the disk by.
func (r *Resolver) Label(disk string) string {
	if p, ok := r.parity[disk]; ok {
		return p
	}
	for _, m := range r.mounts {
		if path.Dir(m.point) != r.hostRoot {
			continue
		}
		name, ok := r.devs[m.dev]
		if !ok {
			continue
		}
		if d, ok := r.md[name]; ok {
			name = d
		}
		if wholeDisk(name) == disk {
			return path.Base(m.point)
		}
	}
	return disk
}

// Annotate names the disks of s and marks the ones under the backup's
// sources and its local target. target is empty for a remote repository.
func (r *Resolver) Annotate(s *Summary, sources []string, target string) {
	src := map[string]bool{}
	for _, p := range sources {
		if d, ok := r.DeviceOf(p); ok {
			src[d] = true
		}
	}
	dst := ""
	if target != "" {
		dst, _ = r.DeviceOf(target)
	}
	for i := range s.Disks {
		d := &s.Disks[i]
		d.Label = r.Label(d.Name)
		switch {
		case src[d.Name] && d.Name == dst:
			d.Role = RoleBoth
		case src[d.Name]:
			d.Role = RoleSource
		case d.Name == dst:
			d.Role = RoleTarget
		}
	}
}
