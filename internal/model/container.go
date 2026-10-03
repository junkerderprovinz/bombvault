// Package model holds the plain container types shared by the backup
// orchestrator and the dockercli adapter, so neither has to import the other.
package model

import "time"

// PortBinding maps a published host endpoint for a container port.
type PortBinding struct {
	HostIP   string
	HostPort string
}

// Allocation describes the network resources a live container holds. Restore
// checks it against the other containers for a taken static IP or host port
// before it stops and removes anything.
type Allocation struct {
	// Name is the container name without the leading slash.
	Name string
	// IPv4 is empty when the container holds no address (DHCP not yet
	// assigned, host networking, or stopped).
	IPv4 string
	// HostPorts are the published host ports as "<port>/<proto>", e.g. "8080/tcp".
	HostPorts []string
}

// DeviceMapping is a single HostConfig.Devices entry.
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
	// User is the process user, e.g. "1000:1000".
	User string
	// Labels must survive a recreate: Unraid reads the net.unraid.docker.*
	// labels (managed, icon, webui, shell) to show the container as a managed
	// app instead of a third-party one.
	Labels map[string]string
	// Healthcheck is the container's own healthcheck, or the image's. A
	// definition captured before it was recorded leaves it nil, and the image's
	// own then applies.
	Healthcheck *Healthcheck `json:",omitempty"`
}

// Healthcheck mirrors Docker's HealthConfig.
type Healthcheck struct {
	Test        []string
	Interval    time.Duration
	Timeout     time.Duration
	StartPeriod time.Duration
	Retries     int
}

// NetworkEndpoint is a network attachment, kept so a recreated container gets
// its original IP and MAC back (e.g. an Unraid br0.x static IP).
type NetworkEndpoint struct {
	// Name is the docker network name, e.g. "br0.20" or "bridge".
	Name string
	// IPv4Address is the requested static IPv4, empty for DHCP.
	IPv4Address string
	// MACAddress is empty when docker assigns one.
	MACAddress string
	Aliases    []string
}

// HostConfig holds the host-side configuration preserved on recreate. The
// capability, privilege, namespace and limit fields must round-trip exactly:
// dropping one could give the restored container more privilege than the
// original (PidMode=host, say) or remove a hardening limit.
type HostConfig struct {
	Binds          []string
	PortBindings   map[string][]PortBinding
	RestartPolicy  RestartPolicy
	CapAdd         []string
	CapDrop        []string
	Privileged     bool
	SecurityOpt    []string
	ReadonlyRootfs bool
	NetworkMode    string
	Devices        []DeviceMapping
	PidMode        string
	IpcMode        string
	UsernsMode     string
	GroupAdd       []string
	Sysctls        map[string]string
	Tmpfs          map[string]string
	ExtraHosts     []string
	CgroupParent   string
	Ulimits        []Ulimit

	// The limits below are absent from definitions stored before they were
	// recorded. Their zero values and nil pointers leave Docker's defaults in
	// place, as on a container started without the flag.
	Memory            int64  `json:",omitempty"`
	MemoryReservation int64  `json:",omitempty"`
	MemorySwap        int64  `json:",omitempty"`
	MemorySwappiness  *int64 `json:",omitempty"`
	NanoCPUs          int64  `json:",omitempty"`
	CPUShares         int64  `json:",omitempty"`
	CPUPeriod         int64  `json:",omitempty"`
	CPUQuota          int64  `json:",omitempty"`
	CpusetCpus        string `json:",omitempty"`
	CpusetMems        string `json:",omitempty"`
	PidsLimit         *int64 `json:",omitempty"`
	OomKillDisable    *bool  `json:",omitempty"`
	OomScoreAdj       int    `json:",omitempty"`
	ShmSize           int64  `json:",omitempty"`
	Init              *bool  `json:",omitempty"`
	// LogConfig is nil when the daemon's default log driver applies.
	LogConfig *LogConfig `json:",omitempty"`

	DNS          []string          `json:"Dns,omitempty"`
	DNSSearch    []string          `json:"DnsSearch,omitempty"`
	DNSOptions   []string          `json:"DnsOptions,omitempty"`
	UTSMode      string            `json:",omitempty"`
	CgroupnsMode string            `json:",omitempty"`
	Links        []string          `json:",omitempty"`
	StorageOpt   map[string]string `json:",omitempty"`

	BlkioWeight          uint16           `json:",omitempty"`
	BlkioWeightDevice    []WeightDevice   `json:",omitempty"`
	BlkioDeviceReadBps   []ThrottleDevice `json:",omitempty"`
	BlkioDeviceWriteBps  []ThrottleDevice `json:",omitempty"`
	BlkioDeviceReadIOps  []ThrottleDevice `json:",omitempty"`
	BlkioDeviceWriteIOps []ThrottleDevice `json:",omitempty"`

	// Runtime and DeviceRequests are how a container gets a GPU
	// (--runtime=nvidia, --gpus). A host without that runtime or driver
	// refuses the container with a MissingRuntimeError.
	Runtime        string          `json:",omitempty"`
	DeviceRequests []DeviceRequest `json:",omitempty"`
}

// DeviceRequest mirrors Docker's request for devices from a driver, the form
// --gpus takes.
type DeviceRequest struct {
	Driver       string            `json:",omitempty"`
	Count        int               `json:",omitempty"`
	DeviceIDs    []string          `json:",omitempty"`
	Capabilities [][]string        `json:",omitempty"`
	Options      map[string]string `json:",omitempty"`
}

// UsesExtraRuntime reports whether the container asks for a GPU or for a
// runtime other than Docker's own.
func (h HostConfig) UsesExtraRuntime() bool {
	return (h.Runtime != "" && h.Runtime != "runc") || len(h.DeviceRequests) > 0
}

// MissingRuntimeError is Docker refusing a container because this host lacks
// the runtime or device driver its recipe asks for. Err is Docker's refusal.
type MissingRuntimeError struct{ Err error }

func (e *MissingRuntimeError) Error() string { return e.Err.Error() }

func (e *MissingRuntimeError) Unwrap() error { return e.Err }

// WeightDevice mirrors Docker's per-device block I/O weight.
type WeightDevice struct {
	Path   string
	Weight uint16
}

// ThrottleDevice mirrors Docker's per-device block I/O rate limit.
type ThrottleDevice struct {
	Path string
	Rate uint64
}

// LogConfig mirrors Docker's log driver and its options.
type LogConfig struct {
	Type   string
	Config map[string]string `json:",omitempty"`
}

// Ulimit mirrors docker's units.Ulimit.
type Ulimit struct {
	Name string
	Soft int64
	Hard int64
}

// Inspect is the part of a container's inspect data captured at backup time
// and used to recreate the container on restore.
type Inspect struct {
	ID    string
	Name  string // may carry a leading slash, e.g. "/plex"
	Image string
	// Running is recorded so backup and restore leave a stopped container
	// stopped.
	Running    bool
	Config     Config
	HostConfig HostConfig
	Mounts     []Mount
	// Network is the primary attachment. Restore also uses it for the IP and
	// port conflict check.
	Network NetworkEndpoint
	// Networks lists every attached network, Network included. Recreate
	// creates the container on the primary one and reconnects the rest.
	Networks []NetworkEndpoint
}

// Health is a live container's readiness, used when containers are restarted
// in dependency order after a backup.
type Health struct {
	Running bool
	// HasHealthcheck reports whether State.Health is present. Without one the
	// caller treats Running plus a short grace period as ready.
	HasHealthcheck bool
	// Healthy is State.Health.Status == "healthy", and false without a
	// healthcheck.
	Healthy bool
}
