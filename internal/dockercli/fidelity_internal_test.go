package dockercli

import (
	"testing"

	"github.com/docker/docker/api/types/container"

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
