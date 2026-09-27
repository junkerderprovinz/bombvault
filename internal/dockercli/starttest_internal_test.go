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
				"com.docker.compose.project": "cloud",
				"net.unraid.docker.managed":  "dockerman",
				"org.opencontainers.version": "1",
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
	if _, ok := cfg.Labels["com.docker.compose.project"]; ok {
		t.Fatal("the copy must not join the compose project")
	}
	if _, ok := cfg.Labels["net.unraid.docker.managed"]; ok {
		t.Fatal("the copy must not show as a managed Unraid app")
	}
	if cfg.Labels["org.opencontainers.version"] != "1" {
		t.Fatal("other labels stay")
	}
	if from.HostConfig.Privileged != true || len(from.HostConfig.Binds) != 2 {
		t.Fatal("the recipe itself must stay untouched")
	}
}
