// Package virshcli — concrete virsh CLI adapter. See types.go for the interface.
package virshcli

import (
	"context"
	"encoding/xml"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strings"
)

// Client is the real virsh adapter that shells out to the virsh CLI. It connects
// over the configured URI — qemu+ssh://root@<host>/system — so virsh runs ON the
// Unraid host (no libvirt socket is bind-mounted into the container).
type Client struct {
	bin string // "virsh" normally
	uri string // qemu+ssh://… ("" = virsh's local default; used only in tests)
}

// compile-time interface check.
var _ Virsh = (*Client)(nil)

// New returns a Client that connects via the given libvirt URI (qemu+ssh://…).
// An empty URI uses virsh's default (local) connection.
func New(uri string) *Client { return &Client{bin: "virsh", uri: uri} }

// baseArgs prefixes "-c <uri>" when a connection URI is configured.
func (c *Client) baseArgs(args ...string) []string {
	if c.uri == "" {
		return args
	}
	return append([]string{"-c", c.uri}, args...)
}

// absPathRe strips absolute paths from error messages so host paths do not
// leak to the caller (mirrors restic's lastReason scrubbing).
var absPathRe = regexp.MustCompile(`(/[^\s:'"]+)+`)

// run executes virsh with the given arguments. It returns the trimmed stdout
// on success. On failure it logs the full stderr server-side and returns a
// scrubbed error containing only the last non-empty stderr line (paths stripped).
func (c *Client) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, c.bin, c.baseArgs(args...)...) //nolint:gosec // G204: args are separate (never shell-interpolated); virsh name/path args come from libvirt, not raw user input
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		log.Printf("virshcli: %q failed: %s", args[0], stderr)
		return "", fmt.Errorf("virshcli: %s: %s", args[0], lastReason(stderr))
	}
	return strings.TrimSpace(string(out)), nil
}

// lastReason extracts the last non-empty line of virsh stderr and scrubs
// absolute paths so host filesystem layout does not reach the caller.
func lastReason(stderr string) string {
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			return absPathRe.ReplaceAllString(l, "[path]")
		}
	}
	return "unknown error"
}

// List returns all domains (running and stopped), one per name line.
func (c *Client) List(ctx context.Context) ([]VMInfo, error) {
	out, err := c.run(ctx, "list", "--all", "--name")
	if err != nil {
		return nil, err
	}
	var vms []VMInfo
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		state, stErr := c.State(ctx, name)
		if stErr != nil {
			state = "unknown"
		}
		vms = append(vms, VMInfo{Name: name, State: state})
	}
	return vms, nil
}

// State returns the domain state ("running", "shut off", …) or ("", nil) when
// the domain does not exist — mirrors dockercli.InspectName not-found tolerance.
func (c *Client) State(ctx context.Context, name string) (string, error) {
	out, err := c.run(ctx, "domstate", name)
	if err != nil {
		// "failed to get domain" / "Domain not found" → treat as absent.
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "failed to get domain") ||
			strings.Contains(msg, "domain not found") ||
			strings.Contains(msg, "no domain") {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// DumpXML returns the domain XML for the named VM.
func (c *Client) DumpXML(ctx context.Context, name string) (string, error) {
	out, err := c.run(ctx, "dumpxml", name)
	if err != nil {
		return "", err
	}
	return out, nil
}

// Shutdown sends an ACPI graceful-shutdown signal.
func (c *Client) Shutdown(ctx context.Context, name string) error {
	_, err := c.run(ctx, "shutdown", name)
	return err
}

// Destroy force-offs the domain. Tolerates already-off ("domain is not running").
func (c *Client) Destroy(ctx context.Context, name string) error {
	_, err := c.run(ctx, "destroy", name)
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "domain is not running") ||
			strings.Contains(msg, "not running") {
			return nil
		}
		return err
	}
	return nil
}

// Start boots the domain.
func (c *Client) Start(ctx context.Context, name string) error {
	_, err := c.run(ctx, "start", name)
	return err
}

// Define (re)defines a domain from an XML file on disk.
func (c *Client) Define(ctx context.Context, xmlPath string) error {
	_, err := c.run(ctx, "define", xmlPath)
	return err
}

// Undefine removes the domain definition including NVRAM. Tolerates not-defined.
func (c *Client) Undefine(ctx context.Context, name string) error {
	_, err := c.run(ctx, "undefine", "--nvram", name)
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "failed to undefine") ||
			strings.Contains(msg, "domain not found") ||
			strings.Contains(msg, "no domain") {
			return nil
		}
		return err
	}
	return nil
}

// Autostart sets (on=true) or clears (on=false) the domain autostart flag.
func (c *Client) Autostart(ctx context.Context, name string, on bool) error {
	args := []string{"autostart"}
	if !on {
		args = append(args, "--disable")
	}
	args = append(args, name)
	_, err := c.run(ctx, args...)
	return err
}

// IsActive reports whether the domain is currently running.
func (c *Client) IsActive(ctx context.Context, name string) (bool, error) {
	state, err := c.State(ctx, name)
	if err != nil {
		return false, err
	}
	return state == "running", nil
}

// ---------------------------------------------------------------------------
// Domain XML parsing
// ---------------------------------------------------------------------------

// domainXML is the minimal struct for parsing a libvirt domain XML document.
// Only the fields BombVault needs (disk sources + NVRAM) are decoded; the rest
// is discarded (xml.Unmarshal ignores unknown elements by default).
type domainXML struct {
	XMLName xml.Name `xml:"domain"`
	Devices struct {
		Disks []struct {
			Type   string `xml:"type,attr"`
			Device string `xml:"device,attr"`
			Source struct {
				File string `xml:"file,attr"`
			} `xml:"source"`
		} `xml:"disk"`
	} `xml:"devices"`
	OS struct {
		NVRAM string `xml:"nvram"`
	} `xml:"os"`
}

// ParseDomain parses a libvirt domain XML string and extracts the disk file
// paths (type="file", device="disk") and NVRAM path (empty for BIOS VMs).
// It is exported so the service layer can call it without importing virshcli
// internals (the result is plain strings; no libvirt types cross the boundary).
func ParseDomain(xmlStr string) (DomainInfo, error) {
	var d domainXML
	if err := xml.Unmarshal([]byte(xmlStr), &d); err != nil {
		return DomainInfo{}, fmt.Errorf("virshcli: parse domain xml: %w", err)
	}
	var disks []string
	for _, disk := range d.Devices.Disks {
		if disk.Type == "file" && disk.Device == "disk" && disk.Source.File != "" {
			disks = append(disks, disk.Source.File)
		}
	}
	nvram := strings.TrimSpace(d.OS.NVRAM)
	return DomainInfo{DiskPaths: disks, NVRAMPath: nvram}, nil
}
