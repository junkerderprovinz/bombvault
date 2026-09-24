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

// Client shells out to the virsh CLI. It connects over the configured URI
// (qemu+ssh://root@<host>/system), so virsh runs on the host and no libvirt
// socket has to be mounted into the container.
type Client struct {
	bin string
	uri string // empty means virsh's local default, which only tests use
}

var _ Virsh = (*Client)(nil)

// New returns a Client that connects through the given libvirt URI. An empty
// URI uses virsh's local default connection.
func New(uri string) *Client { return &Client{bin: "virsh", uri: uri} }

func (c *Client) baseArgs(args ...string) []string {
	if c.uri == "" {
		return args
	}
	return append([]string{"-c", c.uri}, args...)
}

// absPathRe strips absolute paths from error messages so host paths do not
// leak to the caller, as restic's lastReason does.
var absPathRe = regexp.MustCompile(`(/[^\s:'"]+)+`)

// credentialRe matches a "user:password@" URL userinfo segment. It is the
// same scrub internal/restic applies to its errors (its credentialRe comment
// covers the trade-offs). A libvirt URI rarely carries a password, so this is
// defence in depth.
var credentialRe = regexp.MustCompile(`[\w.+%-]+:[^\s/@"']+@`)

// run executes virsh and returns its trimmed stdout. On failure it logs the
// full stderr and returns only the scrubbed last line of it.
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

// lastReason returns the last non-empty line of virsh stderr with absolute
// paths and URL credentials scrubbed. Paths go first because the other order
// would also swallow the hostname.
func lastReason(stderr string) string {
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			l = absPathRe.ReplaceAllString(l, "[path]")
			return credentialRe.ReplaceAllString(l, "[redacted]@")
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
	var names []string
	for _, line := range strings.Split(out, "\n") {
		if name := strings.TrimSpace(line); name != "" {
			names = append(names, name)
		}
	}
	return vmInfoFromNames(ctx, names, c.State, c.titleFromXML), nil
}

// titleFromXML returns a domain's <title>. List calls it for UUID-named
// TrueNAS 26 domains, whose friendly name only lives there.
func (c *Client) titleFromXML(ctx context.Context, name string) (string, error) {
	x, err := c.DumpXML(ctx, name)
	if err != nil {
		return "", err
	}
	d, err := ParseDomain(x)
	if err != nil {
		return "", err
	}
	return d.Title, nil
}

// truenas2510DomainNameRe matches the "{id}_{name}" domain names of TrueNAS
// 25.10, such as "1_debian". It is anchored so that "my_2_vm" does not match.
var truenas2510DomainNameRe = regexp.MustCompile(`^\d+_(.+)$`)

// uuidDomainNameRe matches a bare UUID, which TrueNAS 26 uses as the domain
// name while the VM's own name moves to the <title> element.
var uuidDomainNameRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// normalizeDomainName derives a display name from a domain name by its shape
// alone. "{id}_{name}" (TrueNAS 25.10) yields the part after the underscore;
// a bare UUID (TrueNAS 26) comes back unchanged with isVersioned26Style set,
// telling the caller the real name has to come from <title>; anything else
// passes through. Because it cannot see the platform, an Unraid VM named
// "10_Windows" comes back as "Windows", so callers gate on the platform.
func normalizeDomainName(raw string) (friendlyName string, isVersioned26Style bool) {
	if m := truenas2510DomainNameRe.FindStringSubmatch(raw); m != nil {
		return m[1], false
	}
	if uuidDomainNameRe.MatchString(raw) {
		return raw, true
	}
	return raw, false
}

// vmInfoFromNames builds List's result from raw domain names. The lookups are
// passed in so the name resolution can be tested without a virsh binary.
func vmInfoFromNames(
	ctx context.Context,
	names []string,
	stateFn func(ctx context.Context, name string) (string, error),
	titleFn func(ctx context.Context, name string) (string, error),
) []VMInfo {
	var vms []VMInfo
	for _, name := range names {
		state, stErr := stateFn(ctx, name)
		if stErr != nil {
			state = "unknown"
		}
		friendly, versioned26 := normalizeDomainName(name)
		if versioned26 {
			// A missing or unreadable <title> keeps the UUID rather than
			// failing the whole list.
			if title, tErr := titleFn(ctx, name); tErr == nil && title != "" {
				friendly = title
			}
		}
		vms = append(vms, VMInfo{Name: name, State: state, FriendlyName: friendly})
	}
	return vms
}

// IsNotFound reports whether a virsh error means the domain is gone from the
// host, so a scheduled backup can skip a vanished VM instead of failing.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "failed to get domain") ||
		strings.Contains(msg, "domain not found") ||
		strings.Contains(msg, "no domain")
}

// State returns the domain state ("running", "shut off", ...), or ("", nil)
// when the domain does not exist, like dockercli.InspectName.
func (c *Client) State(ctx context.Context, name string) (string, error) {
	out, err := c.run(ctx, "domstate", name)
	if err != nil {
		if IsNotFound(err) {
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

// DumpXMLInactive returns the persistent domain XML (virsh dumpxml --inactive),
// the defined config without runtime-only or hot-plugged devices.
func (c *Client) DumpXMLInactive(ctx context.Context, name string) (string, error) {
	out, err := c.run(ctx, "dumpxml", "--inactive", name)
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

// SnapshotCreateDiskOnly creates an external, atomic, disk-only snapshot.
func (c *Client) SnapshotCreateDiskOnly(ctx context.Context, name, snapName string, quiesce bool, skipDevs []string) error {
	args := []string{"snapshot-create-as", "--domain", name, snapName,
		"--disk-only", "--atomic", "--no-metadata"}
	if quiesce {
		args = append(args, "--quiesce")
	}
	// Snapshotting a cdrom or read-only disk fails with "external snapshot file
	// ... already exists and is not a block device"; writable disks get an
	// external snapshot by default under --disk-only.
	for _, dev := range skipDevs {
		args = append(args, "--diskspec", dev+",snapshot=no")
	}
	_, err := c.run(ctx, args...)
	return err
}

// BlockCommitActivePivot merges the active overlay back into the base and pivots.
func (c *Client) BlockCommitActivePivot(ctx context.Context, name, device string) error {
	_, err := c.run(ctx, "blockcommit", name, device, "--active", "--pivot", "--wait")
	return err
}

// GuestAgentPing reports whether the qemu guest agent answers inside the VM.
func (c *Client) GuestAgentPing(ctx context.Context, name string) bool {
	_, err := c.run(ctx, "qemu-agent-command", name, `{"execute":"guest-ping"}`)
	return err == nil
}

// domainXML decodes the parts of a libvirt domain XML that BombVault reads.
type domainXML struct {
	XMLName xml.Name `xml:"domain"`
	Title   string   `xml:"title"`
	UUID    string   `xml:"uuid"`
	Devices struct {
		Disks []struct {
			Type   string `xml:"type,attr"`
			Device string `xml:"device,attr"`
			Source struct {
				File string `xml:"file,attr"`
				// Dev is set instead of File for a type="block" disk, such
				// as a TrueNAS zvol at /dev/zvol/<pool>/<dataset>.
				Dev string `xml:"dev,attr"`
			} `xml:"source"`
			Target struct {
				Dev string `xml:"dev,attr"`
			} `xml:"target"`
			ReadOnly *struct{} `xml:"readonly"`
		} `xml:"disk"`
		TPM *tpmXML `xml:"tpm"`
	} `xml:"devices"`
	OS struct {
		NVRAM string `xml:"nvram"`
	} `xml:"os"`
}

// ParseDomain extracts the disks, NVRAM and vTPM paths, title and UUID from a
// libvirt domain XML document.
func ParseDomain(xmlStr string) (DomainInfo, error) {
	var d domainXML
	if err := xml.Unmarshal([]byte(xmlStr), &d); err != nil {
		return DomainInfo{}, fmt.Errorf("virshcli: parse domain xml: %w", err)
	}
	var disks []string
	var diskRefs []DiskRef
	var blockDisks []DiskRef
	device := ""
	var skip []string
	for _, disk := range d.Devices.Disks {
		writable := disk.Type == "file" && disk.Device == "disk" && disk.Source.File != "" && disk.ReadOnly == nil
		blockWritable := disk.Type == "block" && disk.Device == "disk" && disk.Source.Dev != "" && disk.ReadOnly == nil
		switch {
		case writable:
			disks = append(disks, disk.Source.File)
			diskRefs = append(diskRefs, DiskRef{Dev: disk.Target.Dev, Source: disk.Source.File})
			if device == "" {
				device = disk.Target.Dev // the blockcommit target
			}
		case blockWritable:
			// The external-file live snapshot cannot target a raw block
			// device, so it is skipped there and backed up through BlockDisks.
			blockDisks = append(blockDisks, DiskRef{Dev: disk.Target.Dev, Source: disk.Source.Dev, IsBlockDevice: true})
			if disk.Target.Dev != "" {
				skip = append(skip, disk.Target.Dev)
			}
		case disk.Target.Dev != "":
			// A cdrom, read-only or source-less disk stays out of the live snapshot.
			skip = append(skip, disk.Target.Dev)
		}
	}
	nvram := strings.TrimSpace(d.OS.NVRAM)
	tpm := tpmPathFromXML(d.Devices.TPM)
	title := strings.TrimSpace(d.Title)
	uuid := strings.ToLower(strings.TrimSpace(d.UUID))
	return DomainInfo{
		DiskPaths:        disks,
		Disks:            diskRefs,
		NVRAMPath:        nvram,
		TPMPath:          tpm,
		Title:            title,
		UUID:             uuid,
		DiskDevice:       device,
		SkipSnapshotDevs: skip,
		BlockDisks:       blockDisks,
	}, nil
}
