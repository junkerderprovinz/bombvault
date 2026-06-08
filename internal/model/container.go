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
}
