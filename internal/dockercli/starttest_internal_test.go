package dockercli

import (
	"slices"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/model"
)

func TestIsolatedConfigTakesAwayEveryWayOut(t *testing.T) {
	from := model.Inspect{
		Name: "/nextcloud",
		Config: model.Config{
			Image: "lscr.io/linuxserver/nextcloud",
			Env:   []string{"PUID=99"},
			Labels: map[string]string{
				"com.docker.compose.project":            "cloud",
				"net.unraid.docker.managed":             "dockerman",
				"org.opencontainers.version":            "1",
				"traefik.enable":                        "true",
				"com.centurylinklabs.watchtower.enable": "true",
			},
		},
		HostConfig: model.HostConfig{
			Binds:         []string{"/mnt/user/appdata/nextcloud:/config", "/mnt/user/data:/data"},
			PortBindings:  map[string][]model.PortBinding{"443/tcp": {{HostPort: "8443"}}},
			RestartPolicy: model.RestartPolicy{Name: "unless-stopped"},
			Privileged:    true,
			NetworkMode:   "br0.20",
			PidMode:       "host",
			Devices:       []model.DeviceMapping{{PathOnHost: "/dev/dri"}},
			CapAdd:        []string{"SYS_ADMIN", "NET_ADMIN"},
			CapDrop:       []string{"MKNOD"},
			SecurityOpt:   []string{"apparmor=unconfined", "seccomp=unconfined", "systempaths=unconfined", "no-new-privileges:true"},
			Sysctls:       map[string]string{"net.ipv4.ip_forward": "1"},
			CgroupParent:  "/system.slice",
		},
		Network: model.NetworkEndpoint{Name: "br0.20", IPv4Address: "192.168.20.5"},
	}
	spec := IsolatedSpec{
		Name:        StartTestPrefix + "nextcloud-1",
		Network:     StartTestPrefix + "net-1",
		From:        from,
		Binds:       []string{"/mnt/user/restore/sandbox/host/user/appdata/nextcloud:/config"},
		NanoCPUs:    1_000_000_000,
		MemoryBytes: 2 << 30,
		PidsLimit:   512,
		Labels:      map[string]string{"bombvault.starttest.of": "nextcloud"},
		Instance:    "aaaa",
	}
	cfg, hc := isolatedConfig(spec)

	if len(hc.PortBindings) != 0 || hc.PublishAllPorts {
		t.Fatalf("no port may be published: %+v", hc.PortBindings)
	}
	if _, ok := cfg.ExposedPorts["443/tcp"]; !ok {
		t.Fatal("the recipe's port stays exposed inside the network, for the port check")
	}
	if string(hc.NetworkMode) != spec.Network {
		t.Fatalf("network = %q, want the test network", hc.NetworkMode)
	}
	if len(hc.Binds) != 1 || hc.Binds[0] != spec.Binds[0] {
		t.Fatalf("binds = %v, want only the sandbox copy", hc.Binds)
	}
	if hc.Privileged || len(hc.Devices) != 0 || hc.PidMode != "" {
		t.Fatalf("host access must be gone: privileged=%v devices=%v pid=%q", hc.Privileged, hc.Devices, hc.PidMode)
	}
	if hc.RestartPolicy.Name != "no" {
		t.Fatalf("restart policy = %q", hc.RestartPolicy.Name)
	}
	if hc.NanoCPUs != spec.NanoCPUs || hc.Memory != spec.MemoryBytes || hc.MemorySwap != spec.MemoryBytes || hc.PidsLimit == nil || *hc.PidsLimit != 512 {
		t.Fatalf("limits not applied: %+v", hc.Resources)
	}
	if cfg.Labels[StartTestLabel] != "1" || cfg.Labels["bombvault.starttest.of"] != "nextcloud" {
		t.Fatalf("labels = %v", cfg.Labels)
	}
	if len(cfg.Labels) != 3 {
		t.Fatalf("labels = %v, want only BombVault's own: another tool would act on the copy", cfg.Labels)
	}
	if len(hc.CapAdd) != 0 || len(hc.Sysctls) != 0 || hc.CgroupParent != "" {
		t.Fatalf("added privileges must be gone: caps=%v sysctls=%v cgroup=%q", hc.CapAdd, hc.Sysctls, hc.CgroupParent)
	}
	if len(hc.SecurityOpt) != 1 || hc.SecurityOpt[0] != "no-new-privileges:true" {
		t.Fatalf("security options = %v, want only the one that restricts", hc.SecurityOpt)
	}
	if len(hc.CapDrop) != 1 {
		t.Fatalf("dropped capabilities = %v, want them kept", hc.CapDrop)
	}
	if from.HostConfig.Privileged != true || len(from.HostConfig.Binds) != 2 {
		t.Fatal("the recipe itself must stay untouched")
	}
}

func TestStartTestLeftoversAreThisInstancesAndSafeOldOnes(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	mine := map[string]string{StartTestLabel: "1", StartTestInstanceLabel: "aaaa"}
	theirs := map[string]string{StartTestLabel: "1", StartTestInstanceLabel: "bbbb"}
	old := map[string]string{StartTestLabel: "1"}
	containers := []testContainer{
		{name: StartTestPrefix + "aaaa-app-1", labels: mine, running: true, created: now},
		{name: StartTestPrefix + "bbbb-app-2", labels: theirs, running: true, created: now.Add(-48 * time.Hour)},
		{name: StartTestPrefix + "app-3", labels: old, running: false, created: now},
		{name: StartTestPrefix + "app-4", labels: old, running: true, created: now.Add(-2 * time.Hour)},
		{name: StartTestPrefix + "app-5", labels: old, running: true, created: now.Add(-5 * time.Minute)},
		{name: "nextcloud", labels: mine, running: true, created: now},
	}
	networks := []testNetwork{
		{name: StartTestPrefix + "aaaa-net-1", labels: mine, attached: 1},
		{name: StartTestPrefix + "bbbb-net-2", labels: theirs},
		{name: StartTestPrefix + "net-3", labels: old},
		{name: StartTestPrefix + "net-4", labels: old, attached: 1},
	}
	gotC, gotN := pickStartTestLeftovers(containers, networks, "aaaa", now)
	if want := []string{StartTestPrefix + "aaaa-app-1", StartTestPrefix + "app-3", StartTestPrefix + "app-4"}; !slices.Equal(gotC, want) {
		t.Fatalf("containers = %v, want %v", gotC, want)
	}
	if want := []string{StartTestPrefix + "aaaa-net-1", StartTestPrefix + "net-3"}; !slices.Equal(gotN, want) {
		t.Fatalf("networks = %v, want %v", gotN, want)
	}
}

func TestStartTestLeftoversWithoutAnIDTakeNothingLabelledForAnInstance(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	labels := map[string]string{StartTestLabel: "1", StartTestInstanceLabel: "bbbb"}
	gotC, gotN := pickStartTestLeftovers(
		[]testContainer{{name: StartTestPrefix + "bbbb-app-1", labels: labels, created: now.Add(-48 * time.Hour)}},
		[]testNetwork{{name: StartTestPrefix + "bbbb-net-1", labels: labels}},
		"", now)
	if len(gotC) != 0 || len(gotN) != 0 {
		t.Fatalf("got %v %v, want nothing while this instance does not know its id", gotC, gotN)
	}
}

func TestIsolatedConfigNamesTheInstance(t *testing.T) {
	cfg, _ := isolatedConfig(IsolatedSpec{Name: StartTestPrefix + "aaaa-x-1", Network: StartTestPrefix + "aaaa-net-1", Instance: "aaaa"})
	if cfg.Labels[StartTestInstanceLabel] != "aaaa" {
		t.Fatalf("labels = %v", cfg.Labels)
	}
}

func TestIsolatedConfigReplacesTheOriginalsLimits(t *testing.T) {
	noOOMKill := true
	from := model.Inspect{
		Name:   "/plex",
		Config: model.Config{Image: "plexinc/pms-docker"},
		HostConfig: model.HostConfig{
			Memory:            8 << 30,
			MemoryReservation: 4 << 30,
			CPUPeriod:         100_000,
			CPUQuota:          400_000,
			OomKillDisable:    &noOOMKill,
			OomScoreAdj:       -500,
			LogConfig:         &model.LogConfig{Type: "gelf", Config: map[string]string{"gelf-address": "udp://10.0.0.9:12201"}},
		},
	}
	spec := IsolatedSpec{Name: StartTestPrefix + "plex-1", Network: StartTestPrefix + "net-1", From: from, NanoCPUs: 1_000_000_000, MemoryBytes: 2 << 30, PidsLimit: 512}
	_, hc := isolatedConfig(spec)

	if hc.Memory != spec.MemoryBytes || hc.MemoryReservation != 0 {
		t.Fatalf("memory = %d, reservation = %d", hc.Memory, hc.MemoryReservation)
	}
	if hc.CPUPeriod != 0 || hc.CPUQuota != 0 || hc.NanoCPUs != spec.NanoCPUs {
		t.Fatalf("cpu = period %d quota %d nano %d", hc.CPUPeriod, hc.CPUQuota, hc.NanoCPUs)
	}
	if hc.OomKillDisable != nil || hc.OomScoreAdj != 0 {
		t.Fatalf("oom = %v/%d, want Docker's defaults", hc.OomKillDisable, hc.OomScoreAdj)
	}
	if hc.LogConfig.Type != "" {
		t.Fatalf("log driver = %q, want the daemon's default", hc.LogConfig.Type)
	}
}

// A link names a container the test network does not have, a host UTS or
// cgroup namespace reaches out of the copy, and a storage option or an I/O
// limit can refuse the copy for reasons that are not the app's.
func TestIsolatedConfigLeavesOutLinksNamespacesAndIOLimits(t *testing.T) {
	from := model.Inspect{
		Name:   "/wordpress",
		Config: model.Config{Image: "wordpress"},
		HostConfig: model.HostConfig{
			DNS:                []string{"192.168.20.2"},
			Links:              []string{"/db:/wordpress/db"},
			UTSMode:            "host",
			CgroupnsMode:       "host",
			StorageOpt:         map[string]string{"size": "20G"},
			BlkioWeight:        300,
			BlkioWeightDevice:  []model.WeightDevice{{Path: "/dev/sda", Weight: 200}},
			BlkioDeviceReadBps: []model.ThrottleDevice{{Path: "/dev/sda", Rate: 1 << 20}},
		},
	}
	spec := IsolatedSpec{Name: StartTestPrefix + "wordpress-1", Network: StartTestPrefix + "net-1", From: from, NanoCPUs: 1_000_000_000, MemoryBytes: 2 << 30, PidsLimit: 512}
	_, hc := isolatedConfig(spec)

	if len(hc.Links) != 0 || hc.UTSMode != "" || hc.CgroupnsMode != "" || len(hc.StorageOpt) != 0 {
		t.Fatalf("links = %v, uts = %q, cgroupns = %q, storage = %v", hc.Links, hc.UTSMode, hc.CgroupnsMode, hc.StorageOpt)
	}
	if hc.BlkioWeight != 0 || len(hc.BlkioWeightDevice) != 0 || len(hc.BlkioDeviceReadBps) != 0 {
		t.Fatalf("io limits = %d %v %v", hc.BlkioWeight, hc.BlkioWeightDevice, hc.BlkioDeviceReadBps)
	}
	if !slices.Equal(hc.DNS, from.HostConfig.DNS) {
		t.Fatalf("dns = %v, want the recipe's", hc.DNS)
	}
}
