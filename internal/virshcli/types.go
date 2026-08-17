// Package virshcli wraps the virsh CLI behind the Virsh interface so the
// VM backup orchestrator is unit-testable without a real libvirt socket.
// The concrete Client lives in virshcli.go and is wired only in cmd/bombvault.
package virshcli

import "context"

// VMInfo is a summary of a KVM/libvirt domain as returned by List.
type VMInfo struct {
	Name  string
	State string // "running", "shut off", "paused", ...
	// FriendlyName is Name normalized for DISPLAY/MATCHING purposes only —
	// see normalizeDomainName (virshcli.go) for the classifier, and
	// vmInfoFromNames/Client.titleFromXML for how List resolves it. On a
	// plain libvirt host it is always identical to Name (neither TrueNAS
	// pattern matches an ordinary hand-chosen domain name). On TrueNAS 25.10
	// "Goldeye" it strips the "{id}_" prefix TrueNAS's own libvirt naming
	// convention adds (e.g. Name "1_debian" → FriendlyName "debian"). On
	// TrueNAS 26, where Name becomes the VM's bare UUID, FriendlyName is the
	// domain XML's <title> when List could recover one (one extra DumpXML
	// call per UUID-named domain), or the UUID itself as a fallback when it
	// couldn't. Name — never FriendlyName — stays the identifier every
	// virsh command in this package/interface takes; nothing in this
	// package uses FriendlyName for that.
	//
	// ⚠ On Unraid, "always identical to Name" is true in PRACTICE, not by
	// CONSTRUCTION: normalizeDomainName classifies by shape alone (regex on
	// the raw string), with no platform check. A hand-named Unraid VM that
	// happens to collide with the TrueNAS 25.10 shape — e.g. literally named
	// "10_Windows" — gets misclassified the same way a real TrueNAS domain
	// would, and FriendlyName comes back "Windows", not "10_Windows". This
	// is inert today because nothing consumes FriendlyName yet (see below),
	// but a future caller MUST NOT trust it unconditionally: gate on the
	// detected platform (platform.KindTrueNAS) before relying on
	// FriendlyName over Name, rather than assuming the shape match alone
	// means "this is TrueNAS".
	//
	// NOT YET CONSUMED by internal/api/service.go's ListVMs today (confirmed
	// by reading the real code, not assumed): it builds VMView{Name:
	// vm.Name, ...} and matches DB targets via byName[vm.Name], never
	// touching FriendlyName. Wiring it into that layer — so a TrueNAS 26 VM
	// shows its real name instead of a bare UUID in the UI, and matches its
	// stored VM record by friendly name instead of the volatile UUID — is a
	// deliberately separate, not-yet-done follow-up; this task's scope is
	// internal/virshcli only. Mirrors VMBackupDeps.TPMPath's doc comment
	// (internal/backup/vm_orchestrator.go) and the zvol section of
	// docs/vm-backup-ssh-setup.md, which flag the exact same "wired at this
	// layer only, one layer up is a separate task" gap for Tasks 10/11.
	FriendlyName string
}

// DiskRef pairs a writable disk's target device with its current source. It
// lets the service spot (and target) a leftover BombVault snapshot overlay
// precisely — i.e. the exact device whose source is "*.bombvault-tmp".
//
// IsBlockDevice reports whether Source is a raw block device path
// (libvirt <source dev="...">, e.g. a TrueNAS Scale zvol at
// /dev/zvol/<pool>/<dataset>) rather than a regular file
// (<source file="...">). It is always false for entries in DomainInfo.Disks
// (file-backed disks only, unchanged since before Task 10) and always true
// for entries in DomainInfo.BlockDisks (see that field's doc comment) — the
// two lists are never mixed, so a caller can tell which backup mechanism
// applies purely from which list an entry came from; the field itself exists
// so a single DiskRef value remains self-describing.
type DiskRef struct {
	Dev           string // target dev, e.g. "hdc"
	Source        string // current source file path OR block device path
	IsBlockDevice bool
}

// DomainInfo contains the artifacts parsed from a libvirt domain XML:
// the disk image path(s), the NVRAM path (empty for BIOS VMs), and the first
// disk's target device (e.g. "vda") used as the live-backup blockcommit target.
type DomainInfo struct {
	DiskPaths []string
	// Disks pairs each writable FILE-backed disk's target dev with its source
	// (parallels DiskPaths). Used to detect/commit a leftover live-snapshot
	// overlay. NEVER includes a block-device disk — see BlockDisks.
	Disks     []DiskRef
	NVRAMPath string
	// TPMPath is the vTPM device's discoverable state/device path, parsed
	// from the domain XML's <tpm> element — empty in exactly two cases that
	// are deliberately indistinguishable to a caller: no <tpm> element at
	// all, or a <tpm> element present whose shape doesn't carry a path
	// BombVault recognizes as safe to trust (see ParseDomain's doc comment
	// for exactly which shapes are recognized). Both degrade to "nothing to
	// capture" — mirrors NVRAMPath's own "empty = nothing to do" contract
	// exactly, so a caller checking `if domain.TPMPath != ""` behaves
	// correctly without needing to know which of the two cases it was.
	TPMPath string
	// Title is the domain XML's <title> element, trimmed — libvirt's own
	// free-form display-name field, a direct child of <domain> (NOT nested
	// under <devices> or <os>). Empty when the domain has no <title>
	// element — the common case on Unraid and TrueNAS 25.10, where the
	// friendly name lives in the domain NAME itself, not this element.
	// TrueNAS 26 is the one platform BombVault knows of that relies on this:
	// its libvirt domain name becomes the VM's bare UUID, with the
	// user-chosen name moved here instead — see normalizeDomainName
	// (virshcli.go) for the classifier that decides when a caller should
	// bother reading this field, and Client.titleFromXML for the one caller
	// that does. Mirrors NVRAMPath/TPMPath's own "empty = nothing to
	// report" convention exactly.
	Title      string
	DiskDevice string
	// SkipSnapshotDevs are target devices that must NOT be snapshotted in a live
	// backup (cdrom / read-only / source-less disks, AND block-device disks —
	// see BlockDisks) — snapshotting them fails with "external snapshot file
	// ... already exists and is not a block device".
	SkipSnapshotDevs []string
	// BlockDisks are writable disks whose backing store is a raw block device
	// (libvirt <source dev="...">, e.g. a TrueNAS Scale zvol at
	// /dev/zvol/<pool>/<dataset> — see internal/virshcli/zvol.go) rather than a
	// regular file. Populated ADDITIVELY to (never overlapping with) DiskPaths/
	// Disks/DiskDevice, which stay file-backed-only and byte-identical to their
	// pre-Task-10 behavior. A block-device disk needs a fundamentally different
	// backup mechanism (ZFS snapshot + `zfs send` streamed into restic, see
	// internal/backup/vm_orchestrator.go's BackupZvolDisk/RestoreZvolDisk) since
	// restic cannot back up a block device by path the way it backs up a file.
	//
	// ⚠ UNVERIFIED AGAINST REAL HARDWARE — see zvol.go's package doc comment.
	BlockDisks []DiskRef
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
	// DumpXML returns the domain XML for the named VM (the LIVE config for a
	// running VM — includes hot-plugged/transient devices and current disk paths).
	DumpXML(ctx context.Context, name string) (string, error)
	// DumpXMLInactive returns the PERSISTENT (inactive) domain XML (virsh dumpxml
	// --inactive): the VM's defined config without runtime-only/hot-plugged
	// devices. Used to capture the restore definition so a live-snapshot restore
	// doesn't pin transient devices a guest re-adds itself.
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
