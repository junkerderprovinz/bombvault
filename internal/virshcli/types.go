// Package virshcli wraps the virsh CLI behind the Virsh interface so the
// VM backup orchestrator is unit-testable without a real libvirt socket.
// The concrete Client lives in virshcli.go and is wired only in cmd/bombvault.
package virshcli

import "context"

// VMInfo is a summary of a KVM/libvirt domain as returned by List.
type VMInfo struct {
	Name  string
	State string // "running", "shut off", "paused", ...
}

// DomainInfo contains the artifacts parsed from a libvirt domain XML:
// the disk image path(s), the NVRAM path (empty for BIOS VMs), and the first
// disk's target device (e.g. "vda") used as the live-backup blockcommit target.
type DomainInfo struct {
	DiskPaths  []string
	NVRAMPath  string
	DiskDevice string
	// SkipSnapshotDevs are target devices that must NOT be snapshotted in a live
	// backup (cdrom / read-only / source-less disks) — snapshotting them fails
	// with "external snapshot file ... already exists and is not a block device".
	SkipSnapshotDevs []string
}

// Virsh is the host-control surface the VM backup orchestrator depends on.
// It is deliberately small and interface-shaped so orchestrators and the
// service layer can be unit-tested with fakes without a real libvirt socket.
type Virsh interface {
	// List returns all domains (running and stopped).
	List(ctx context.Context) ([]VMInfo, error)
	// State returns the domain state string ("running", "shut off", …), or
	// ("", nil) when the domain does not exist (mirror of dockercli.InspectName's
	// not-found tolerance).
	State(ctx context.Context, name string) (string, error)
	// DumpXML returns the domain XML for the named VM.
	DumpXML(ctx context.Context, name string) (string, error)
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
