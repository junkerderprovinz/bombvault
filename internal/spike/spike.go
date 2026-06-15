// Package spike implements host-integration probes. Each probe is a function
// that returns a human-readable detail string and an error. Probes are
// dependency-injected so the package is fully unit-testable without a real
// Docker socket or restic binary.
package spike

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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
	BestEffort bool // true when this probe does not gate AllOK
}

// ProbeFn is a probe implementation. It receives the shared Deps and returns a
// human-readable detail (shown on success or failure) and an error that marks
// the probe as failed when non-nil.
type ProbeFn func(deps Deps) (detail string, err error)

// Probe pairs a display name with its implementation.
type Probe struct {
	Name string
	Fn   ProbeFn
	// BestEffort marks optional probes (e.g. libvirt, qemu-img, rclone) that
	// are reported but do NOT count against AllOK when they fail. Only the
	// gating probes (docker, restic, path-writable) gate AllOK.
	BestEffort bool
}

// Deps carries the shared dependencies that real probes use. All fields are
// optional so the zero value is safe for unit tests that inject stub probes.
type Deps struct {
	// Docker is used by the docker-reachable probe. May be nil in tests.
	Docker dockercli.Docker
	// ContainerPath is the resolved absolute backup path for the path-writable
	// probe. May be empty; the probe skips the write if it is.
	ContainerPath string
	// LibvirtTest checks that libvirt is reachable over SSH (qemu+ssh). nil when
	// SSH is not wired; the libvirt probe then reports "not configured".
	LibvirtTest func() error
}

// Run executes each probe in order, collects results, and returns them along
// with an overall AllOK flag. Panics inside a probe are caught and converted
// into a failed check — Run itself never panics.
//
// A failing best-effort probe is included in checks with OK=false but does NOT
// lower AllOK. Only gating probes (BestEffort=false) affect AllOK.
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

// runProbe executes a single probe, catching any panic.
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

// ---------------------------------------------------------------------------
// Default probes
// ---------------------------------------------------------------------------

// DefaultProbes returns the standard set of host-integration probes used in
// production. Each probe is independently injectable in tests via Run.
//
// Gating probes (BestEffort=false) — their failure lowers AllOK:
//   - docker reachable
//   - restic ≥0.17
//   - path-writable
//
// Best-effort probes (BestEffort=true) — reported but do not gate AllOK; they
// are needed only for later VM/Flash phases, not Phase-1 container backup:
//   - qemu-img, rclone, libvirt
func DefaultProbes() []Probe {
	return []Probe{
		{Name: "docker", Fn: probeDocker},
		{Name: "restic", Fn: probeRestic},
		{Name: "qemu-img", Fn: probeQemuImg, BestEffort: true},
		{Name: "rclone", Fn: probeRclone, BestEffort: true},
		{Name: "path-writable", Fn: probePathWritable},
		{Name: "libvirt", Fn: probeLibvirt, BestEffort: true},
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

// resticMinVersion is the minimum acceptable restic version.
var resticVersionRe = regexp.MustCompile(`restic\s+(\d+)\.(\d+)`)

// probeRestic verifies that restic is on PATH and is version ≥0.17.
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
	// m[1]/m[2] are guaranteed to be digit-only by the regex above.
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

// probePathWritable verifies the chosen container backup path is writable by
// writing and immediately removing a small probe file.
func probePathWritable(deps Deps) (string, error) {
	p := deps.ContainerPath
	if p == "" {
		return "skipped (no path configured)", nil
	}
	if err := os.MkdirAll(p, 0o700); err != nil {
		return "", fmt.Errorf("cannot create backup dir %q: %w", p, err)
	}
	//nolint:gosec // G304: caller must pass a path already validated by paths.Resolve under the mount root; this is not a user-supplied HTTP value
	f, err := os.CreateTemp(p, ".spike-probe-*")
	if err != nil {
		return "", fmt.Errorf("path not writable: %w", err)
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name) //nolint:gosec // G104 best-effort cleanup of a temp file we just created
	return fmt.Sprintf("writable (%s)", p), nil
}

// probeLibvirt checks that libvirt is reachable over SSH (qemu+ssh). VM backup
// uses NO local libvirt mount, so there is no socket file to look for — the
// check is the SSH connection itself (best-effort; not gating). "not configured"
// means the SSH public key has not been authorized on the host yet (Settings →
// VM Backup over SSH).
func probeLibvirt(d Deps) (string, error) {
	if d.LibvirtTest == nil {
		return "", fmt.Errorf("VM backup over SSH not configured — authorize the key in Settings → VM Backup over SSH")
	}
	if err := d.LibvirtTest(); err != nil {
		return "", fmt.Errorf("libvirt not reachable over SSH: %v", err)
	}
	return "reachable over SSH (qemu+ssh)", nil
}
