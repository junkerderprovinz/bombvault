package zfs

import (
	"bufio"
	"io"
	"path"
	"strings"
)

// MountRecord is one line of /proc/self/mountinfo with its paths decoded.
// Root is the mount's root inside the source filesystem, so a bind of a
// subdirectory is recognisable and never mistaken for a dataset's own top.
type MountRecord struct {
	MountPoint string
	Root       string
	FSType     string
	Source     string
	Options    []string
	Optional   []string
}

// ParseMountinfo reads the mount table and skips lines it cannot make sense of.
func ParseMountinfo(r io.Reader) []MountRecord {
	var out []MountRecord
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 7 {
			continue
		}
		sep := -1
		for i := 6; i < len(fields); i++ {
			if fields[i] == "-" {
				sep = i
				break
			}
		}
		if sep < 0 || sep+2 >= len(fields) {
			continue
		}
		out = append(out, MountRecord{
			Root:       unescapeOctal(fields[3]),
			MountPoint: path.Clean(unescapeOctal(fields[4])),
			Options:    strings.Split(fields[5], ","),
			Optional:   fields[6:sep],
			FSType:     fields[sep+1],
			Source:     unescapeOctal(fields[sep+2]),
		})
	}
	return out
}

// FindDatasetMount picks the container path where dataset is mounted whole.
// The preferred path is the translated host mountpoint; an Unraid exclusive
// share adds a second record under the user share, and writing into that one
// would go through shfs. anywhere covers an identity mapping such as TrueNAS's
// /mnt/pool:/mnt/pool and is only given for backups: a restore writes, so it
// stays below hostMountRoot.
func FindDatasetMount(recs []MountRecord, dataset, preferred, hostMountRoot string, anywhere bool) (MountRecord, bool) {
	var below, outside MountRecord
	var haveBelow, haveOutside bool
	for _, r := range recs {
		if r.FSType != "zfs" || r.Source != dataset || r.Root != "/" {
			continue
		}
		if preferred != "" && r.MountPoint == preferred {
			return r, true
		}
		if under(r.MountPoint, hostMountRoot) {
			if !haveBelow || len(r.MountPoint) < len(below.MountPoint) {
				below, haveBelow = r, true
			}
			continue
		}
		if !haveOutside || len(r.MountPoint) < len(outside.MountPoint) {
			outside, haveOutside = r, true
		}
	}
	if haveBelow {
		return below, true
	}
	if anywhere && haveOutside {
		return outside, true
	}
	return MountRecord{}, false
}

// SnapshotMounted reports whether the snapshot itself, not the empty control
// directory above it, is what a run would read at cpath.
func SnapshotMounted(recs []MountRecord, dataset, snap, cpath string) bool {
	source := dataset + "@" + snap
	point := path.Clean(cpath + "/.zfs/snapshot/" + snap)
	for _, r := range recs {
		if r.FSType == "zfs" && r.Source == source && r.MountPoint == point {
			return true
		}
	}
	return false
}

// ShfsOnly reports that cpath is covered by a fuse.shfs mount and by no zfs
// mount. Unraid's user shares never expose .zfs, so a dataset reachable only
// that way cannot be backed up this way.
func ShfsOnly(recs []MountRecord, cpath string) bool {
	shfs := false
	for _, r := range recs {
		if cpath != r.MountPoint && !under(cpath, r.MountPoint) {
			continue
		}
		switch r.FSType {
		case "zfs":
			return false
		case "fuse.shfs":
			shfs = true
		}
	}
	return shfs
}

// Writable reports whether rec was mounted read-write.
func Writable(rec MountRecord) bool {
	for _, o := range rec.Options {
		if o == "ro" {
			return false
		}
	}
	return true
}

// Propagation reports whether the Host Data mapping receives new mounts from
// the host, and names the zfs mounts below it that do not. A snapshot
// automount appears below one of those, so without propagation the lookup ends
// in ELOOP rather than in the snapshot.
func Propagation(recs []MountRecord, hostMountRoot string) (top bool, nestedWithout []string) {
	for _, r := range recs {
		switch {
		case r.MountPoint == hostMountRoot:
			top = hasMaster(r)
		case r.FSType == "zfs" && under(r.MountPoint, hostMountRoot) && !hasMaster(r):
			nestedWithout = append(nestedWithout, r.MountPoint)
		}
	}
	return top, nestedWithout
}

func hasMaster(r MountRecord) bool {
	for _, o := range r.Optional {
		if strings.HasPrefix(o, "master:") {
			return true
		}
	}
	return false
}

// under reports whether p lies strictly below root.
func under(p, root string) bool {
	if root == "" || root == "/" {
		return root == "/" && p != "/"
	}
	return strings.HasPrefix(p, root+"/")
}

// unescapeOctal decodes the \NNN escapes the kernel writes into mountinfo
// paths for space, tab, newline and backslash.
func unescapeOctal(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) &&
			s[i+1] >= '0' && s[i+1] <= '7' &&
			s[i+2] >= '0' && s[i+2] <= '7' &&
			s[i+3] >= '0' && s[i+3] <= '7' {
			b.WriteByte((s[i+1]-'0')<<6 | (s[i+2]-'0')<<3 | (s[i+3] - '0'))
			i += 3
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
