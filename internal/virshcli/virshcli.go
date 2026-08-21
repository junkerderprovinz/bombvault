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

// credentialRe matches a "user:password@" URL-userinfo segment — the same
// defense-in-depth scrub internal/restic/restic.go and internal/api/handlers.go
// apply to their own surfaced errors (see restic.go's credentialRe doc
// comment for the full reasoning: the username class, why it covers a
// fully-numeric username, the known Docker-digest-style false-positive
// tradeoff, the known unencoded-"/"-in-password false-negative tradeoff, and
// why it runs AFTER absPathRe rather than before). Applied here too, for the
// parity this package's own doc comment above already claims, even though a
// libvirt "qemu+ssh://user@host/system" URI doesn't normally carry a
// password.
var credentialRe = regexp.MustCompile(`[\w.+%-]+:[^\s/@"']+@`)

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
// absolute paths and URL-embedded credentials so host filesystem layout and
// secrets do not reach the caller. Paths are scrubbed BEFORE credentials —
// not a correctness requirement, both orders fully redact a password, but
// this order avoids also destroying the hostname; see
// internal/restic/restic.go's scrubSecrets doc comment for the full
// reasoning.
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

// titleFromXML fetches a domain's <title> via DumpXML + ParseDomain — the
// one extra virsh call vmInfoFromNames makes per UUID-style (TrueNAS 26)
// domain name, to try to recover a friendly name. Wrapping DumpXML+
// ParseDomain here (rather than passing both separately) keeps
// vmInfoFromNames's fake surface down to a single func value, matching
// stateFn (c.State).
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

// truenas2510DomainNameRe matches TrueNAS 25.10 "Goldeye"'s libvirt
// domain-naming convention: "{id}_{name}" (id = TrueNAS's own numeric VM id,
// name = the user-chosen VM name), e.g. "1_debian". Anchored at both ends so
// a name that merely CONTAINS a digits-then-underscore run somewhere (e.g.
// "my_2_vm", an ordinary Unraid-style name) never false-positives — the run
// must be the very start of the string.
var truenas2510DomainNameRe = regexp.MustCompile(`^\d+_(.+)$`)

// uuidDomainNameRe matches a standard 8-4-4-4-12 hex UUID — TrueNAS 26's
// libvirt domain-naming convention, where the domain name IS the VM's UUID
// and the user-friendly name moves to the domain XML's <title> element
// instead (see normalizeDomainName's doc comment).
var uuidDomainNameRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// normalizeDomainName is a cheap, pure, XML-free classifier for TrueNAS
// Scale's version-dependent libvirt domain-naming schemes. It never talks to
// virsh or reads a domain XML — it only pattern-matches the raw string
// already in hand. Three outcomes:
//
//   - TrueNAS 25.10 "Goldeye" style, "{id}_{name}" (e.g. "1_debian"): fully
//     resolved from the string alone — friendlyName is the part after the
//     first underscore ("debian"), isVersioned26Style is false.
//   - TrueNAS 26 style, a bare UUID (e.g.
//     "550e8400-e29b-41d4-a716-446655440000"): the raw string carries NO
//     friendly-name information — the user-chosen name lives only in the
//     domain XML's <title> element, which requires a separate `virsh
//     dumpxml` call this function deliberately does not make (it stays pure
//     and XML-free). friendlyName falls back to the UUID itself, and
//     isVersioned26Style is true — the caller's signal to try to resolve a
//     better name from <title> if it has (or is willing to fetch) the
//     domain XML. See Client.List/vmInfoFromNames below for the one caller
//     that does this.
//   - Anything else (an ordinary Unraid-style name, or an empty string):
//     passed through unchanged, isVersioned26Style false. This is the no-op
//     path — Unraid domain names never match either TrueNAS pattern, so
//     Unraid behavior is byte-identical to before this function existed.
//
// This classification is purely SHAPE-based (regex on raw) — it is not, and
// cannot be, platform-gated in here, since it never sees which platform it's
// running on. A domain name that merely happens to look like one of the two
// TrueNAS shapes (e.g. an Unraid VM literally named "10_Windows") is
// misclassified exactly like a real TrueNAS domain would be. See
// VMInfo.FriendlyName's doc comment (types.go) for the exact scenario and
// the mitigation a future caller must apply before trusting the result.
func normalizeDomainName(raw string) (friendlyName string, isVersioned26Style bool) {
	if m := truenas2510DomainNameRe.FindStringSubmatch(raw); m != nil {
		return m[1], false
	}
	if uuidDomainNameRe.MatchString(raw) {
		return raw, true
	}
	return raw, false
}

// vmInfoFromNames builds List's result from already-split raw domain names,
// given the state and (versioned-domain) title lookups to use. Split out of
// List so the friendly-name resolution logic — including the one extra
// virsh call a TrueNAS 26 (UUID-named) domain needs — is unit-testable with
// plain fake func values, without a real virsh binary.
func vmInfoFromNames(
	ctx context.Context,
	names []string,
	stateFn func(ctx context.Context, name string) (string, error),
	titleFn func(ctx context.Context, name string) (string, error),
) []VMInfo {
	var vms []VMInfo
	for _, name := range names {
		state, stErr := stateFn(ctx, name)
		if stErr != nil { // mirrors List's own pre-existing State-failure tolerance
			state = "unknown"
		}
		friendly, versioned26 := normalizeDomainName(name)
		if versioned26 {
			// One extra virsh call, only for a UUID-style name, to try to
			// recover the friendly name from <title>. A failure here (or an
			// absent/empty <title>) must not fail List — fall back to the
			// UUID normalizeDomainName already returned, the same way a
			// failed State lookup falls back to "unknown" above rather than
			// failing the whole call.
			if title, tErr := titleFn(ctx, name); tErr == nil && title != "" {
				friendly = title
			}
		}
		vms = append(vms, VMInfo{Name: name, State: state, FriendlyName: friendly})
	}
	return vms
}

// IsNotFound reports whether a virsh error means the domain does not exist on the
// host (it was deleted/undefined, or is a template that is no longer defined).
// Exported so callers — e.g. a scheduled backup — can skip a vanished VM instead
// of treating libvirt's "failed to get domain" as a hard failure.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "failed to get domain") ||
		strings.Contains(msg, "domain not found") ||
		strings.Contains(msg, "no domain")
}

// State returns the domain state ("running", "shut off", …) or ("", nil) when
// the domain does not exist — mirrors dockercli.InspectName not-found tolerance.
func (c *Client) State(ctx context.Context, name string) (string, error) {
	out, err := c.run(ctx, "domstate", name)
	if err != nil {
		if IsNotFound(err) { // a missing domain has no state, not an error
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

// DumpXMLInactive returns the persistent (inactive) domain XML (virsh dumpxml
// --inactive) — the defined config without runtime-only/hot-plugged devices.
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
	// Exclude non-writable disks (cdrom / read-only). Snapshotting them fails with
	// "external snapshot file ... already exists and is not a block device". The
	// writable disks default to an external snapshot under --disk-only.
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

// ---------------------------------------------------------------------------
// Domain XML parsing
// ---------------------------------------------------------------------------

// domainXML is the minimal struct for parsing a libvirt domain XML document.
// Only the fields BombVault needs (disk sources + NVRAM) are decoded; the rest
// is discarded (xml.Unmarshal ignores unknown elements by default).
type domainXML struct {
	XMLName xml.Name `xml:"domain"`
	// Title is libvirt's own free-form display-name element — a direct
	// child of <domain>, not nested under <devices> or <os>. See
	// DomainInfo.Title's doc comment (types.go) for what it's used for
	// (TrueNAS 26's friendly-name recovery).
	Title   string `xml:"title"`
	Devices struct {
		Disks []struct {
			Type   string `xml:"type,attr"`
			Device string `xml:"device,attr"`
			Source struct {
				File string `xml:"file,attr"`
				// Dev is libvirt's own signal that the backing store is a raw
				// block device (type="block") rather than a regular file
				// (type="file", which uses Source.File instead) — e.g. a
				// TrueNAS Scale zvol at /dev/zvol/<pool>/<dataset>. See
				// zvol.go's package doc comment for the "reasoned from
				// documentation, unverified against real hardware" caveat.
				Dev string `xml:"dev,attr"`
			} `xml:"source"`
			Target struct {
				Dev string `xml:"dev,attr"`
			} `xml:"target"`
			ReadOnly *struct{} `xml:"readonly"`
		} `xml:"disk"`
		// TPM is nil when the domain has no <tpm> element at all — see
		// tpmPathFromXML (tpm.go) for how its presence/shape is turned into
		// DomainInfo.TPMPath.
		TPM *tpmXML `xml:"tpm"`
	} `xml:"devices"`
	OS struct {
		NVRAM string `xml:"nvram"`
	} `xml:"os"`
}

// ParseDomain parses a libvirt domain XML string and extracts the disk file
// paths (type="file", device="disk"), the NVRAM path (empty for BIOS VMs),
// and the vTPM device path (empty for a domain with no <tpm> element, or one
// whose shape doesn't carry a path this package trusts — see tpm.go's
// package doc comment for exactly which <tpm> shapes are recognized). It is
// exported so the service layer can call it without importing virshcli
// internals (the result is plain strings; no libvirt types cross the boundary).
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
		// blockWritable: libvirt's own signal (<source dev="..."> under
		// type="block") that this disk's backing store is a raw block device —
		// e.g. a TrueNAS Scale zvol — not a regular file. See zvol.go.
		blockWritable := disk.Type == "block" && disk.Device == "disk" && disk.Source.Dev != "" && disk.ReadOnly == nil
		switch {
		case writable:
			disks = append(disks, disk.Source.File)
			diskRefs = append(diskRefs, DiskRef{Dev: disk.Target.Dev, Source: disk.Source.File})
			if device == "" {
				device = disk.Target.Dev // first writable disk's target dev = blockcommit target
			}
		case blockWritable:
			// Recorded ADDITIVELY in BlockDisks (see its doc comment) — never in
			// DiskPaths/Disks/DiskDevice, which stay file-backed-only. PRESERVES
			// pre-Task-10 behavior for SkipSnapshotDevs: before BlockDisks existed,
			// this disk already fell through to the skip branch below (it failed
			// the file-only `writable` check), since qemu's external-file live
			// snapshot cannot target a raw block device the way it does a qcow2.
			blockDisks = append(blockDisks, DiskRef{Dev: disk.Target.Dev, Source: disk.Source.Dev, IsBlockDevice: true})
			if disk.Target.Dev != "" {
				skip = append(skip, disk.Target.Dev)
			}
		case disk.Target.Dev != "":
			// cdrom, read-only, or source-less disk → exclude from the live snapshot.
			skip = append(skip, disk.Target.Dev)
		}
	}
	nvram := strings.TrimSpace(d.OS.NVRAM)
	tpm := tpmPathFromXML(d.Devices.TPM)
	title := strings.TrimSpace(d.Title)
	return DomainInfo{
		DiskPaths:        disks,
		Disks:            diskRefs,
		NVRAMPath:        nvram,
		TPMPath:          tpm,
		Title:            title,
		DiskDevice:       device,
		SkipSnapshotDevs: skip,
		BlockDisks:       blockDisks,
	}, nil
}
