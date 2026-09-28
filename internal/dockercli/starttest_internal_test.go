package dockercli

import (
	"testing"

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
	if len(cfg.Labels) != 2 {
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
