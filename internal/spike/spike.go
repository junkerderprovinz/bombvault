// Package spike runs host-integration probes. Probes are plain functions over
// Deps, so tests can inject stubs instead of a Docker socket or restic binary.
package spike

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
)

// Check is the result of a single probe.
type Check struct {
	Name       string
	OK         bool
	Detail     string
	BestEffort bool // does not count against Run's allOK
}

// ProbeFn runs one probe. It returns a detail line for the report, or an error
// that marks the probe as failed.
type ProbeFn func(deps Deps) (detail string, err error)

// Probe pairs a display name with its implementation.
type Probe struct {
	Name string
	Fn   ProbeFn
	// BestEffort probes are reported but do not count against allOK when they
	// fail.
	BestEffort bool
}

// Deps carries what the real probes need. The zero value works for tests that
// inject stub probes.
type Deps struct {
	Docker dockercli.Docker
	// ContainerPath is the container backup path. Empty skips the path probe.
	ContainerPath string
	// LibvirtTest checks libvirt over SSH (qemu+ssh). It is nil until SSH is
	// set up.
	LibvirtTest func() error
	// ZFSTest reaches the pool owner over SSH and reports what it found, or why
	// the ZFS domain cannot read it. Nil while the domain is off.
	ZFSTest func() (string, error)
}

// Run executes the probes in order. A panicking probe becomes a failed check.
// allOK is false when any probe that is not BestEffort failed.
func Run(deps Deps, probes []Probe) (checks []Check, allOK bool) {
	allOK = true
	checks = make([]Check, 0, len(probes))

	for _, p := range probes {
		c := runProbe(deps, p)
		checks = append(checks, c)
		if !c.OK && !p.BestEffort {
			allOK = false
		}
	}
	return checks, allOK
}

func runProbe(deps Deps, p Probe) (c Check) {
	c.Name = p.Name
	c.BestEffort = p.BestEffort

	defer func() {
		if r := recover(); r != nil {
			c.OK = false
			c.Detail = fmt.Sprintf("probe panicked: %v", r)
		}
	}()

	detail, err := p.Fn(deps)
	if err != nil {
		c.OK = false
		c.Detail = err.Error()
	} else {
		c.OK = true
		c.Detail = detail
	}
	return c
}

// DefaultProbes returns the probes used in production. docker, restic and
// path-writable gate allOK. qemu-img, rclone and libvirt are only needed for VM
// and flash backups, so they are best-effort.
func DefaultProbes() []Probe {
	return []Probe{
		{Name: "docker", Fn: probeDocker},
		{Name: "restic", Fn: probeRestic},
		{Name: "qemu-img", Fn: probeQemuImg, BestEffort: true},
		{Name: "rclone", Fn: probeRclone, BestEffort: true},
		{Name: "path-writable", Fn: probePathWritable},
		{Name: "libvirt", Fn: probeLibvirt, BestEffort: true},
		{Name: "zfs", Fn: probeZFS, BestEffort: true},
	}
}

// probeDocker checks that the Docker socket is reachable by listing containers.
func probeDocker(deps Deps) (string, error) {
	if deps.Docker == nil {
		return "", fmt.Errorf("docker client not configured")
	}
	containers, err := deps.Docker.List(context.Background())
	if err != nil {
		return "", fmt.Errorf("docker not reachable: %w", err)
	}
	return fmt.Sprintf("reachable (%d containers)", len(containers)), nil
}

var resticVersionRe = regexp.MustCompile(`restic\s+(\d+)\.(\d+)`)

// probeRestic checks that restic is on PATH and at least version 0.17.
func probeRestic(deps Deps) (string, error) {
	//nolint:gosec // G204: restic is a known binary, no user input in args
	out, err := exec.Command("restic", "version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("restic not found or failed: %w", err)
	}
	version := strings.TrimSpace(string(out))
	m := resticVersionRe.FindStringSubmatch(version)
	if m == nil {
		return "", fmt.Errorf("could not parse restic version from: %q", version)
	}
	// The regex only matches digits, so Atoi cannot fail.
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	if major == 0 && minor < 17 {
		return "", fmt.Errorf("restic version too old (%s); need ≥0.17", version)
	}
	return version, nil
}

// probeQemuImg checks that qemu-img is on PATH.
func probeQemuImg(_ Deps) (string, error) {
	//nolint:gosec // G204: qemu-img is a known binary, no user input
	out, err := exec.Command("qemu-img", "--version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("qemu-img not found: %w", err)
	}
	first := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
	return first, nil
}

// probeRclone checks that rclone is on PATH.
func probeRclone(_ Deps) (string, error) {
	//nolint:gosec // G204: rclone is a known binary, no user input
	out, err := exec.Command("rclone", "version", "--check").CombinedOutput()
	if err != nil {
		// rclone version --check exits non-zero when an update is available but
		// the binary is present. Fall back to plain "rclone version".
		out2, err2 := exec.Command("rclone", "version").CombinedOutput() //nolint:gosec
		if err2 != nil {
			return "", fmt.Errorf("rclone not found: %w", err2)
		}
		first := strings.SplitN(strings.TrimSpace(string(out2)), "\n", 2)[0]
		return first, nil
	}
	first := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
	return first, nil
}

// probePathWritable checks that the container backup path is writable without
// creating it. On Unraid a new top-level directory under /mnt/user becomes a
// share the user cannot easily delete, so the folder is left to the first
// backup and the probe tests the nearest existing ancestor instead.
func probePathWritable(deps Deps) (string, error) {
	p := deps.ContainerPath
	if p == "" {
		return "skipped (no path configured)", nil
	}
	// A remote (rclone/SFTP) repo has no local dir to probe.
	if strings.Contains(p, ":") && !filepath.IsAbs(p) {
		return fmt.Sprintf("remote repo (%s), not probed", p), nil
	}
	dir := filepath.Clean(p)
	for {
		if _, err := os.Stat(dir); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no existing parent directory to probe for %q", p)
		}
		dir = parent
	}
	//nolint:gosec // G304: dir is an existing ancestor of a path validated by paths.Resolve under the mount root; not a user-supplied HTTP value
	f, err := os.CreateTemp(dir, ".spike-probe-*")
	if err != nil {
		return "", fmt.Errorf("path not writable: %w", err)
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name) //nolint:gosec // G104 best-effort cleanup of a temp file we just created
	if dir == filepath.Clean(p) {
		return fmt.Sprintf("writable (%s)", p), nil
	}
	return fmt.Sprintf("writable (%s will be created on first backup)", p), nil
}

// probeLibvirt checks that libvirt is reachable over SSH (qemu+ssh). VM backup
// has no local libvirt socket, so the SSH connection is the check. Without an
// authorized SSH key it reports "not configured".
func probeLibvirt(d Deps) (string, error) {
	if d.LibvirtTest == nil {
		return "", fmt.Errorf("VM backup over SSH not configured: authorize the key in Settings → VM Backup over SSH")
	}
	if err := d.LibvirtTest(); err != nil {
		return "", fmt.Errorf("libvirt not reachable over SSH: %v", err)
	}
	return "reachable over SSH (qemu+ssh)", nil
}

// probeZFS reports how the ZFS domain reaches the pool owner and whether the
// container still receives the host's new mounts. Both break silently: a Host
// Data mapping without mount propagation only shows up when the first backup
// cannot read its snapshot.
func probeZFS(d Deps) (string, error) {
	if d.ZFSTest == nil {
		return "ZFS datasets are switched off", nil
	}
	return d.ZFSTest()
}
