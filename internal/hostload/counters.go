// Package hostload measures how busy the CPU, the disks and the network were
// while a backup ran, from the kernel's counters in /proc. Inside a container
// /proc/stat and /proc/diskstats are the host's, and /proc/net/dev is the
// container's own network, which is where restic sends its data.
package hostload

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Counters is one reading of the kernel counters. A file that could not be
// read leaves its part empty, and every later figure built on it is left out.
type Counters struct {
	CPUBusy, CPUTotal uint64
	// DiskTicks is the milliseconds each block device spent doing I/O, by
	// kernel name.
	DiskTicks map[string]uint64
	// TxBytes is what every interface but loopback sent. HasNet is false when
	// /proc/net/dev could not be read.
	TxBytes uint64
	HasNet  bool
	// CgroupUsec is the CPU time this container has used, and CPULimit how
	// many CPUs it may use: 0 when it may use every CPU of the host, whose
	// busy share then says all there is.
	CgroupUsec uint64
	CPULimit   float64
}

// Read takes one reading below proc and the container's cgroup, which are
// /proc and /sys/fs/cgroup outside of tests.
func Read(proc, cgroup string) Counters {
	var c Counters
	hostCPUs := 0
	if b, err := os.ReadFile(filepath.Join(proc, "stat")); err == nil { //nolint:gosec // G304: a fixed file below /proc
		c.CPUBusy, c.CPUTotal, hostCPUs = parseStat(b)
	}
	c.CgroupUsec, c.CPULimit = readCgroup(cgroup, hostCPUs)
	if b, err := os.ReadFile(filepath.Join(proc, "diskstats")); err == nil { //nolint:gosec // G304: a fixed file below /proc
		c.DiskTicks = parseDiskstats(b)
	}
	if b, err := os.ReadFile(filepath.Join(proc, "net", "dev")); err == nil { //nolint:gosec // G304: a fixed file below /proc
		c.TxBytes, c.HasNet = parseNetDev(b), true
	}
	return c
}

// parseStat reads the aggregate cpu line and counts the CPUs. Idle and
// iowait count as not busy; guest time is already part of user time.
func parseStat(b []byte) (busy, total uint64, cpus int) {
	lines := strings.Split(string(b), "\n")
	for _, l := range lines[1:] {
		if strings.HasPrefix(l, "cpu") {
			cpus++
		}
	}
	f := strings.Fields(lines[0])
	if len(f) < 5 || f[0] != "cpu" {
		return 0, 0, cpus
	}
	var idle uint64
	for i, s := range f[1:min(len(f), 9)] {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return 0, 0, cpus
		}
		total += v
		if i == 3 || i == 4 {
			idle += v
		}
	}
	return total - idle, total, cpus
}

// readCgroup reads the container's CPU time and its limit from cgroup v2:
// the quota in cpu.max or the CPUs cpuset.cpus.effective allows, whichever
// is lower. Without cgroup v2 both stay zero.
func readCgroup(dir string, hostCPUs int) (usec uint64, limit float64) {
	b, err := os.ReadFile(filepath.Join(dir, "cpu.stat")) //nolint:gosec // G304: a fixed file below /sys/fs/cgroup
	if err != nil {
		return 0, 0
	}
	for _, l := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(l, "usage_usec "); ok {
			usec, _ = strconv.ParseUint(strings.TrimSpace(v), 10, 64)
		}
	}
	limit = float64(hostCPUs)
	if b, err := os.ReadFile(filepath.Join(dir, "cpu.max")); err == nil { //nolint:gosec // G304: a fixed file below /sys/fs/cgroup
		if f := strings.Fields(string(b)); len(f) == 2 && f[0] != "max" {
			quota, qErr := strconv.ParseFloat(f[0], 64)
			period, pErr := strconv.ParseFloat(f[1], 64)
			if qErr == nil && pErr == nil && period > 0 {
				limit = min(limit, quota/period)
			}
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, "cpuset.cpus.effective")); err == nil { //nolint:gosec // G304: a fixed file below /sys/fs/cgroup
		if n := countCPUs(strings.TrimSpace(string(b))); n > 0 {
			limit = min(limit, float64(n))
		}
	}
	if hostCPUs == 0 || limit >= float64(hostCPUs) {
		return usec, 0
	}
	return usec, limit
}

// countCPUs counts a cpuset list such as "0-3,8".
func countCPUs(list string) int {
	n := 0
	for _, part := range strings.Split(list, ",") {
		lo, hi, isRange := strings.Cut(part, "-")
		a, err := strconv.Atoi(lo)
		if err != nil {
			return 0
		}
		b := a
		if isRange {
			if b, err = strconv.Atoi(hi); err != nil {
				return 0
			}
		}
		n += b - a + 1
	}
	return n
}

// virtualDisk matches devices that are never a backup's brake: loop files,
// RAM disks and optical drives.
var virtualDisk = regexp.MustCompile(`^(loop|ram|zram|sr|fd)\d`)

// partition matches a partition of a whole disk, which the disk already
// counts.
var partition = regexp.MustCompile(`^((sd|vd|xvd|hd)[a-z]+\d+|(nvme\d+n\d+|mmcblk\d+)p\d+)$`)

func parseDiskstats(b []byte) map[string]uint64 {
	out := map[string]uint64{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 13 {
			continue
		}
		name := f[2]
		if virtualDisk.MatchString(name) || partition.MatchString(name) {
			continue
		}
		ticks, err := strconv.ParseUint(f[12], 10, 64)
		if err != nil {
			continue
		}
		out[name] = ticks
	}
	return out
}

func parseNetDev(b []byte) uint64 {
	var tx uint64
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		name, rest, ok := strings.Cut(sc.Text(), ":")
		if !ok || strings.TrimSpace(name) == "lo" {
			continue
		}
		f := strings.Fields(rest)
		if len(f) < 9 {
			continue
		}
		if v, err := strconv.ParseUint(f[8], 10, 64); err == nil {
			tx += v
		}
	}
	return tx
}
