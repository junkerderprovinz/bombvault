package dockercli

import (
	"encoding/json"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"

	"github.com/junkerderprovinz/bombvault/internal/model"
)

// TestHostConfigRoundTripPreservesIsolationFields pins SEC-parity for restore:
// the namespace / isolation and resource fields a container was created with must
// survive backup→recreate, so a restored container keeps its original security
// posture and limits (dropping e.g. PidMode=host silently changes isolation).
func TestHostConfigRoundTripPreservesIsolationFields(t *testing.T) {
	src := &container.HostConfig{
		PidMode:    "host",
		IpcMode:    "host",
		UsernsMode: "host",
		GroupAdd:   []string{"users", "audio"},
		ExtraHosts: []string{"db:10.0.0.5"},
		CapAdd:     []string{"NET_ADMIN"},
	}
	src.Sysctls = map[string]string{"net.ipv4.ip_forward": "1"}
	src.Tmpfs = map[string]string{"/run": "rw,noexec"}
	src.CgroupParent = "/bombvault"
	src.Ulimits = []*container.Ulimit{{Name: "nofile", Soft: 1024, Hard: 2048}}

	m := mapHostConfig(src)
	if m.PidMode != "host" || m.IpcMode != "host" || m.UsernsMode != "host" {
		t.Fatalf("namespace modes not captured: %+v", m)
	}
	if len(m.GroupAdd) != 2 || len(m.ExtraHosts) != 1 || m.Sysctls["net.ipv4.ip_forward"] != "1" ||
		m.Tmpfs["/run"] != "rw,noexec" || m.CgroupParent != "/bombvault" || len(m.Ulimits) != 1 {
		t.Fatalf("isolation/resource fields not captured: %+v", m)
	}

	_, hc := buildCreateConfig(model.Inspect{HostConfig: m})
	if string(hc.PidMode) != "host" || string(hc.IpcMode) != "host" || string(hc.UsernsMode) != "host" {
		t.Fatalf("namespace modes not reproduced on recreate: %+v", hc)
	}
	if len(hc.GroupAdd) != 2 || len(hc.ExtraHosts) != 1 || hc.Sysctls["net.ipv4.ip_forward"] != "1" ||
		hc.Tmpfs["/run"] != "rw,noexec" || hc.CgroupParent != "/bombvault" {
		t.Fatalf("isolation/resource fields not reproduced: %+v", hc)
	}
	if len(hc.Ulimits) != 1 || hc.Ulimits[0].Name != "nofile" || hc.Ulimits[0].Hard != 2048 {
		t.Fatalf("ulimit not reproduced: %+v", hc.Ulimits)
	}
}

// TestNetworkingConfigPreservesMACWithoutStaticIP pins the fix: a container with a
// pinned MAC but no static IPv4 (DHCP) must still have its MAC reproduced on
// recreate (previously the whole endpoint config was dropped when IPv4 was empty).
func TestNetworkingConfigPreservesMACWithoutStaticIP(t *testing.T) {
	in := model.Inspect{Network: model.NetworkEndpoint{
		Name:       "br0.20",
		MACAddress: "02:42:ac:11:00:02",
		Aliases:    []string{"plex"},
	}}
	cfg := buildNetworkingConfig(in)
	if cfg == nil {
		t.Fatal("networking config dropped despite a pinned MAC")
	}
	ep := cfg.EndpointsConfig["br0.20"]
	if ep == nil || ep.MacAddress != "02:42:ac:11:00:02" {
		t.Fatalf("MAC not preserved: %+v", ep)
	}
}

// TestNetworkingConfigNilWhenNothingToPreserve keeps the default-network case a
// no-op: no static IP and no MAC => let Docker attach normally.
func TestNetworkingConfigNilWhenNothingToPreserve(t *testing.T) {
	if buildNetworkingConfig(model.Inspect{Network: model.NetworkEndpoint{Name: "bridge"}}) != nil {
		t.Fatal("expected nil networking config when there is no static IP or MAC")
	}
}

func TestContainerSummaryMappingCarriesMountsAndLabels(t *testing.T) {
	t.Run("running container with bind mount and named volume", func(t *testing.T) {
		summary := container.Summary{
			ID:      "abc123def456",
			Names:   []string{"/myapp"},
			Image:   "ubuntu:22.04",
			ImageID: "sha256:deadbeefcafe1234",
			State:   "running",
			Status:  "Up 2 hours",
			Created: 1609459200,
			Labels: map[string]string{
				"com.docker.compose.project": "myproject",
				"version":                    "1.0",
			},
			Mounts: []container.MountPoint{
				{
					Type:        mount.TypeBind,
					Source:      "/host/appdata",
					Destination: "/app/data",
				},
				{
					Type:        mount.TypeVolume,
					Name:        "myapp_cache",
					Source:      "/var/lib/docker/volumes/myapp_cache/_data",
					Destination: "/app/cache",
				},
			},
			NetworkSettings: &container.NetworkSettingsSummary{
				Networks: map[string]*network.EndpointSettings{
					"bridge": {IPAddress: "172.17.0.2"},
				},
			},
		}

		info := mapContainerSummary(summary)

		if info.ID != "abc123def456" {
			t.Errorf("ID = %q, want abc123def456", info.ID)
		}
		if info.Name != "myapp" {
			t.Errorf("Name = %q, want myapp", info.Name)
		}
		if info.Image != "ubuntu:22.04" {
			t.Errorf("Image = %q, want ubuntu:22.04", info.Image)
		}
		if info.ImageID != "sha256:deadbeefcafe1234" {
			t.Errorf("ImageID = %q, want sha256:deadbeefcafe1234", info.ImageID)
		}
		if info.Created != 1609459200 {
			t.Errorf("Created = %d, want 1609459200", info.Created)
		}
		if info.State != "running" {
			t.Errorf("State = %q, want running", info.State)
		}
		if info.IP != "172.17.0.2" {
			t.Errorf("IP = %q, want 172.17.0.2", info.IP)
		}
		if info.Stack != "myproject" {
			t.Errorf("Stack = %q, want myproject", info.Stack)
		}
		if info.Labels["version"] != "1.0" {
			t.Errorf("Labels[version] = %q, want 1.0", info.Labels["version"])
		}
		if info.Labels["com.docker.compose.project"] != "myproject" {
			t.Errorf("Labels[com.docker.compose.project] = %q, want myproject", info.Labels["com.docker.compose.project"])
		}
		if len(info.Mounts) != 1 {
			t.Fatalf("len(Mounts) = %d, want 1 (bind mount only, not named volume)", len(info.Mounts))
		}
		if info.Mounts[0].Source != "/host/appdata" {
			t.Errorf("Mounts[0].Source = %q, want /host/appdata", info.Mounts[0].Source)
		}
		if info.Mounts[0].Destination != "/app/data" {
			t.Errorf("Mounts[0].Destination = %q, want /app/data", info.Mounts[0].Destination)
		}
	})

	t.Run("stopped container carries all facts", func(t *testing.T) {
		summary := container.Summary{
			ID:      "stoppedabc123",
			Names:   []string{"/oldname"},
			Image:   "nginx:latest",
			ImageID: "sha256:stoppedimg99",
			State:   "exited",
			Status:  "Exited (0) 1 day ago",
			Created: 1609372800,
			Labels: map[string]string{
				"env": "test",
			},
			Mounts: []container.MountPoint{
				{
					Type:        mount.TypeBind,
					Source:      "/host/config",
					Destination: "/etc/nginx/conf.d",
				},
			},
			NetworkSettings: &container.NetworkSettingsSummary{
				Networks: map[string]*network.EndpointSettings{},
			},
		}

		info := mapContainerSummary(summary)

		if info.ID != "stoppedabc123" {
			t.Errorf("ID = %q, want stoppedabc123", info.ID)
		}
		if info.ImageID != "sha256:stoppedimg99" {
			t.Errorf("ImageID = %q, want sha256:stoppedimg99", info.ImageID)
		}
		if info.Created != 1609372800 {
			t.Errorf("Created = %d, want 1609372800", info.Created)
		}
		if info.State != "exited" {
			t.Errorf("State = %q, want exited", info.State)
		}
		if info.Labels["env"] != "test" {
			t.Errorf("Labels[env] = %q, want test", info.Labels["env"])
		}
		if info.IP != "" {
			t.Errorf("IP = %q, want empty for stopped container", info.IP)
		}
		if len(info.Mounts) != 1 {
			t.Fatalf("len(Mounts) = %d, want 1", len(info.Mounts))
		}
	})
}

func TestRecreateKeepsTheContainersOwnHealthcheck(t *testing.T) {
	resp := container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{ID: "abc", Name: "/db"},
		Config: &container.Config{
			Image:       "postgres:16",
			Healthcheck: &container.HealthConfig{Test: []string{"CMD-SHELL", "pg_isready"}, Retries: 5},
		},
	}
	in := mapInspect(resp)
	cfg, _ := buildCreateConfig(in)
	if cfg.Healthcheck == nil || cfg.Healthcheck.Test[1] != "pg_isready" || cfg.Healthcheck.Retries != 5 {
		t.Fatalf("healthcheck = %+v", cfg.Healthcheck)
	}

	resp.Config.Healthcheck = nil
	if cfg, _ := buildCreateConfig(mapInspect(resp)); cfg.Healthcheck != nil {
		t.Fatal("without a healthcheck of its own, the image's applies")
	}
}

// restoredHostConfig takes inspect data the way a backup stores it and a
// restore reads it back: through the definition's JSON.
func restoredHostConfig(t *testing.T, resp container.InspectResponse) *container.HostConfig {
	t.Helper()
	raw, err := json.Marshal(mapInspect(resp))
	if err != nil {
		t.Fatal(err)
	}
	var in model.Inspect
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatal(err)
	}
	_, hc := buildCreateConfig(in)
	return hc
}

func TestRestoreKeepsResourceLimitsAndLogDriver(t *testing.T) {
	swappiness, pids, noOOMKill, withInit := int64(10), int64(200), true, true
	src := &container.HostConfig{
		LogConfig:   container.LogConfig{Type: "none"},
		ShmSize:     128 << 20,
		OomScoreAdj: 300,
		Init:        &withInit,
	}
	src.Memory = 16 << 20
	src.MemoryReservation = 8 << 20
	src.MemorySwap = 32 << 20
	src.MemorySwappiness = &swappiness
	src.NanoCPUs = 50_000_000
	src.CPUShares = 512
	src.CpusetCpus = "0-1"
	src.CpusetMems = "0"
	src.PidsLimit = &pids
	src.OomKillDisable = &noOOMKill
	hc := restoredHostConfig(t, container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{Name: "/tiny", HostConfig: src},
		Config:            &container.Config{Image: "alpine"},
	})

	if hc.Memory != 16<<20 || hc.MemoryReservation != 8<<20 || hc.MemorySwap != 32<<20 {
		t.Fatalf("memory limits = %d/%d/%d", hc.Memory, hc.MemoryReservation, hc.MemorySwap)
	}
	if hc.MemorySwappiness == nil || *hc.MemorySwappiness != 10 {
		t.Fatalf("swappiness = %v", hc.MemorySwappiness)
	}
	if hc.NanoCPUs != 50_000_000 || hc.CPUShares != 512 || hc.CpusetCpus != "0-1" || hc.CpusetMems != "0" {
		t.Fatalf("cpu limits = %+v", hc.Resources)
	}
	if hc.PidsLimit == nil || *hc.PidsLimit != 200 {
		t.Fatalf("pids limit = %v", hc.PidsLimit)
	}
	if hc.OomKillDisable == nil || !*hc.OomKillDisable || hc.OomScoreAdj != 300 {
		t.Fatalf("oom settings = %v/%d", hc.OomKillDisable, hc.OomScoreAdj)
	}
	if hc.LogConfig.Type != "none" {
		t.Fatalf("log driver = %q", hc.LogConfig.Type)
	}
	if hc.ShmSize != 128<<20 || hc.Init == nil || !*hc.Init {
		t.Fatalf("shm size = %d, init = %v", hc.ShmSize, hc.Init)
	}
}

func TestRestoreKeepsLogOptions(t *testing.T) {
	src := &container.HostConfig{LogConfig: container.LogConfig{Type: "json-file", Config: map[string]string{"max-size": "50m", "max-file": "1"}}}
	hc := restoredHostConfig(t, container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{Name: "/app", HostConfig: src},
		Config:            &container.Config{Image: "alpine"},
	})
	if hc.LogConfig.Type != "json-file" || hc.LogConfig.Config["max-size"] != "50m" || hc.LogConfig.Config["max-file"] != "1" {
		t.Fatalf("log config = %+v", hc.LogConfig)
	}
}

// A definition stored before limits were recorded must restore with Docker's
// defaults, not with a zero limit or an empty log driver.
func TestRestoreFromAnOlderDefinitionLeavesLimitsUnset(t *testing.T) {
	var in model.Inspect
	old := `{"Name":"/plex","Config":{"Image":"plexinc/pms-docker"},"HostConfig":{"Binds":["/mnt/user/appdata/plex:/config"],"RestartPolicy":{"Name":"unless-stopped"},"NetworkMode":"bridge"}}`
	if err := json.Unmarshal([]byte(old), &in); err != nil {
		t.Fatal(err)
	}
	_, hc := buildCreateConfig(in)
	if hc.Memory != 0 || hc.MemorySwap != 0 || hc.NanoCPUs != 0 || hc.CPUShares != 0 || hc.ShmSize != 0 {
		t.Fatalf("limits = %+v, want none", hc.Resources)
	}
	if hc.PidsLimit != nil || hc.MemorySwappiness != nil || hc.OomKillDisable != nil || hc.Init != nil {
		t.Fatalf("optional settings must stay unset: %+v", hc)
	}
	if hc.LogConfig.Type != "" || hc.LogConfig.Config != nil {
		t.Fatalf("log config = %+v, want the daemon's default", hc.LogConfig)
	}
}
