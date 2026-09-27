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
}

// Read takes one reading below proc, which is /proc outside of tests.
func Read(proc string) Counters {
	var c Counters
	if b, err := os.ReadFile(filepath.Join(proc, "stat")); err == nil { //nolint:gosec // G304: a fixed file below /proc
		c.CPUBusy, c.CPUTotal = parseStat(b)
	}
	if b, err := os.ReadFile(filepath.Join(proc, "diskstats")); err == nil { //nolint:gosec // G304: a fixed file below /proc
		c.DiskTicks = parseDiskstats(b)
	}
	if b, err := os.ReadFile(filepath.Join(proc, "net", "dev")); err == nil { //nolint:gosec // G304: a fixed file below /proc
		c.TxBytes, c.HasNet = parseNetDev(b), true
	}
	return c
}

// parseStat reads the aggregate cpu line. Idle and iowait count as not busy;
// guest time is already part of user time.
func parseStat(b []byte) (busy, total uint64) {
	line, _, _ := bytes.Cut(b, []byte("\n"))
	f := strings.Fields(string(line))
	if len(f) < 5 || f[0] != "cpu" {
		return 0, 0
	}
	var idle uint64
	for i, s := range f[1:min(len(f), 9)] {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return 0, 0
		}
		total += v
		if i == 3 || i == 4 {
			idle += v
		}
	}
	return total - idle, total
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
