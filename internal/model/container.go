// Package model holds the behavior-free container types shared across the
// dependency-injection seam. The backup orchestrator and the dockercli adapter
// both depend on these types, but neither depends on the other — keeping the DI
// seam clean (no concrete-adapter import in the orchestrator). This package
// imports only the standard library.
package model

// PortBinding maps a published host endpoint for a container port.
type PortBinding struct {
	HostIP   string
	HostPort string
}

// Allocation describes the network resources a live container currently holds.
// The restore pre-flight conflict check compares a container being restored
// against the Allocations of all other containers to catch an in-use static IP
// or published host port BEFORE the destructive stop/remove.
type Allocation struct {
	// Name is the normalized container name (no leading slash).
	Name string
	// IPv4 is the container's current IPv4, empty when it holds none (DHCP not
	// yet assigned, host networking, or a stopped container).
	IPv4 string
	// HostPorts are the published host ports as "<port>/<proto>" (e.g. "8080/tcp").
	HostPorts []string
}

// DeviceMapping is a single HostConfig.Devices entry (host device → container device).
type DeviceMapping struct {
	PathOnHost        string
	PathInContainer   string
	CgroupPermissions string
}

// RestartPolicy mirrors the Docker container restart policy.
type RestartPolicy struct {
	Name              string
	MaximumRetryCount int
}

// Mount describes a single bind/volume mount as reported by inspect.
type Mount struct {
	Type        string
	Source      string
	Destination string
}

// Config holds the portable container configuration we preserve.
type Config struct {
	Image string
	Env   []string
	Cmd   []string
	// User is the process user (e.g. "1000:1000"). SEC: preserved on recreate.
	User string
	// Labels carries the container labels. Unraid uses net.unraid.docker.*
	// labels (managed, icon, webui, shell) to treat the container as a managed,
	// editable app rather than a "third-party" one — so they MUST be preserved.
	Labels map[string]string
}

// NetworkEndpoint captures the primary network attachment so a recreated
// container keeps its original IP/MAC (e.g. an Unraid br0.x static IP) instead
// of being reassigned a new one.
type NetworkEndpoint struct {
	// Name is the docker network name (e.g. "br0.20", "bridge").
	Name string
	// IPv4Address is the statically-requested IPv4 (empty for DHCP/auto).
	IPv4Address string
	// MACAddress is the requested MAC (empty for auto).
	MACAddress string
	// Aliases are the network-scoped aliases.
	Aliases []string
}

// HostConfig holds the host-side configuration we preserve on recreate.
// The Cap/Privileged/SecurityOpt/ReadonlyRootfs/NetworkMode/Devices fields are
// security-relevant: a recreated container must never gain privilege over the
// original (SEC parity with the TypeScript implementation).
type HostConfig struct {
	Binds         []string
	PortBindings  map[string][]PortBinding
	RestartPolicy RestartPolicy
	// SEC: security-relevant fields preserved on recreate.
	CapAdd         []string
	CapDrop        []string
	Privileged     bool
	SecurityOpt    []string
	ReadonlyRootfs bool
	NetworkMode    string
	Devices        []DeviceMapping
}

// Inspect is the subset of a container's inspect data that BombVault captures at
// backup time and uses to recreate the container on restore. It is the rich
// profile that flows through the DI seam so the recreated container preserves
// the original's security-relevant fields (SEC §8).
type Inspect struct {
	ID         string
	Name       string // dockerode-style, may carry a leading slash (e.g. "/plex")
	Image      string
	Config     Config
	HostConfig HostConfig
	Mounts     []Mount
	// Network is the primary network attachment, preserved so the recreated
	// container keeps its original (often static) IP.
	Network NetworkEndpoint
}
