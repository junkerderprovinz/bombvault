// Package virshcli wraps the virsh CLI behind the Virsh interface so the VM
// backup orchestrator can be tested without a libvirt host.
package virshcli

import "context"

// VMInfo is a summary of a KVM/libvirt domain as returned by List.
type VMInfo struct {
	Name  string
	State string // "running", "shut off", "paused", ...
	// FriendlyName is the display name of a TrueNAS domain: "debian" for
	// "1_debian" on 25.10, the <title> of a UUID-named domain on 26 (or the
	// UUID when there is none). It is guessed from the name's shape alone, so
	// an Unraid VM named "10_Windows" also comes back as "Windows"; callers
	// use it only on a detected TrueNAS host and only for display. Name stays
	// the identifier for virsh calls, target matching, tags and restore.
	FriendlyName string
}

// DiskRef pairs a writable disk's target device with its current source, so
// the service can find the exact device still running on a leftover
// "*.bombvault-tmp" overlay. IsBlockDevice is true for the entries of
// DomainInfo.BlockDisks and false for those of DomainInfo.Disks.
type DiskRef struct {
	Dev           string // target dev, e.g. "hdc"
	Source        string // file path or block device path
	IsBlockDevice bool
}

// DomainInfo holds what ParseDomain reads from a domain XML. DiskDevice is
// the first writable disk's target, used as the blockcommit target of a live
// backup; the path fields are empty when there is nothing to capture.
type DomainInfo struct {
	DiskPaths []string
	// Disks pairs each file-backed writable disk with its source, in the
	// order of DiskPaths. Block devices are in BlockDisks instead.
	Disks     []DiskRef
	NVRAMPath string
	// TPMPath is the vTPM state path from the <tpm> element. It is empty both
	// without a <tpm> element and for a shape tpm.go does not trust, since
	// either way there is nothing safe to capture.
	TPMPath string
	// Title is the trimmed <title> element. Only TrueNAS 26 relies on it,
	// where the domain name is a UUID and the VM's own name lives here.
	Title string
	// UUID is the domain XML's <uuid>, lower-cased and trimmed so two
	// spellings of one UUID never read as two VMs. Unraid keeps it across a
	// rename, which makes it the rename signal for VMs. Empty only when the
	// XML has no <uuid>.
	UUID       string
	DiskDevice string
	// SkipSnapshotDevs are target devices that must not be snapshotted in a live
	// backup (cdrom, read-only or source-less disks, and block-device disks;
	// see BlockDisks): snapshotting them fails with "external snapshot file
	// ... already exists and is not a block device".
	SkipSnapshotDevs []string
	// BlockDisks are writable disks backed by a raw block device, such as a
	// TrueNAS zvol at /dev/zvol/<pool>/<dataset>, and never appear in
	// DiskPaths, Disks or DiskDevice. restic cannot read a block device by
	// path, so these go through a ZFS snapshot and zfs send instead (see
	// BackupZvolDisk in internal/backup and zvol.go).
	BlockDisks []DiskRef
}

// Virsh is the host-control surface the VM backup orchestrator depends on.
type Virsh interface {
	// List returns all domains (running and stopped).
	List(ctx context.Context) ([]VMInfo, error)
	// State returns the domain state string ("running", "shut off", ...), or
	// ("", nil) when the domain does not exist.
	State(ctx context.Context, name string) (string, error)
	// DumpXML returns the domain XML for the named VM. For a running VM that is
	// the live config, including hot-plugged devices and current disk paths.
	DumpXML(ctx context.Context, name string) (string, error)
	// DumpXMLInactive returns the persistent domain XML (virsh dumpxml
	// --inactive). The restore definition is taken from it so a restore does
	// not pin transient devices the guest re-adds itself.
	DumpXMLInactive(ctx context.Context, name string) (string, error)
	// Shutdown sends an ACPI graceful-shutdown signal (virsh shutdown).
	Shutdown(ctx context.Context, name string) error
	// Destroy force-offs the domain (virsh destroy). Tolerates already-off.
	Destroy(ctx context.Context, name string) error
	// Start boots the domain (virsh start).
	Start(ctx context.Context, name string) error
	// Define (re)defines a domain from an XML file (virsh define <xmlPath>).
	Define(ctx context.Context, xmlPath string) error
	// Undefine removes the domain definition, including NVRAM if present
	// (virsh undefine --nvram). Tolerates not-defined.
	Undefine(ctx context.Context, name string) error
	// Autostart sets or clears the autostart flag (virsh autostart [--disable]).
	Autostart(ctx context.Context, name string, on bool) error
	// IsActive reports whether the domain is in the "running" state.
	IsActive(ctx context.Context, name string) (bool, error)
	// SnapshotCreateDiskOnly creates an external, atomic, disk-only snapshot
	// (the VM keeps running, writing to a fresh overlay). skipDevs lists target
	// devices to exclude (cdrom / read-only) via --diskspec <dev>,snapshot=no.
	SnapshotCreateDiskOnly(ctx context.Context, name, snapName string, quiesce bool, skipDevs []string) error
	// BlockCommitActivePivot merges the active overlay back into its base and
	// pivots the running VM onto the base (blockcommit --active --pivot --wait).
	BlockCommitActivePivot(ctx context.Context, name, device string) error
	// GuestAgentPing reports whether the qemu guest agent answers in the VM.
	GuestAgentPing(ctx context.Context, name string) bool
}
