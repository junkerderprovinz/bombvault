package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
)

func TestTheAnnouncementNamesTheSchemeAndPortTheWebInterfaceUses(t *testing.T) {
	h, _, repo, _ := newMCPGateHandler(t)
	h.cfg.HTTPSPort, h.cfg.Port = 3443, 3000
	svc := h.mdnsService()
	if svc.Type != "_https._tcp" || svc.Port != 3443 || svc.Instance != "BombVault" || svc.Host != "bombvault" {
		t.Fatalf("service %+v", svc)
	}
	settings, err := repo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.InstanceName = "tower"
	if err := repo.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	h.cfg.HTTPOnly = true
	svc = h.mdnsService()
	if svc.Type != "_http._tcp" || svc.Port != 3000 || svc.Instance != "BombVault (tower)" {
		t.Fatalf("service %+v", svc)
	}
	if len(svc.TXT) != 2 || svc.TXT[0] != "version="+Version || svc.TXT[1] != "path=/" {
		t.Fatalf("TXT %v", svc.TXT)
	}
}

func TestSwitchingTheAnnouncementOffIsStored(t *testing.T) {
	_, router, repo, _ := newMCPGateHandler(t)
	r := httptest.NewRequest(http.MethodPut, "/api/mdns", strings.NewReader(`{"enabled":false}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"running":false`) {
		t.Fatalf("PUT answered %d %s", w.Code, w.Body)
	}
	if on, err := repo.MDNSEnabled(); err != nil || on {
		t.Fatalf("the switch reads %v (%v) after switching off", on, err)
	}
}

// networkDocker is BombVault's own container on the networks given, each
// with its driver.
type networkDocker struct {
	dockercli.Docker
	mode     string
	drivers  map[string]string
	inspects int
}

func (d *networkDocker) Self(context.Context) (string, error) { return "bombvault", nil }

func (d *networkDocker) Inspect(context.Context, string) (model.Inspect, error) {
	d.inspects++
	in := model.Inspect{HostConfig: model.HostConfig{NetworkMode: d.mode}}
	for name := range d.drivers {
		in.Networks = append(in.Networks, model.NetworkEndpoint{Name: name})
	}
	return in, nil
}

func (d *networkDocker) NetworkDriver(_ context.Context, name string) (string, error) {
	driver, ok := d.drivers[name]
	if !ok {
		return "", errors.New("no such network")
	}
	return driver, nil
}

func TestTheMDNSRowSaysWhenTheAnnouncementStaysOnTheHost(t *testing.T) {
	cases := []struct {
		name     string
		docker   *networkDocker
		hostOnly bool
	}{
		{"default bridge", &networkDocker{mode: "bridge", drivers: map[string]string{"bridge": "bridge"}}, true},
		{"TrueNAS app network", &networkDocker{mode: "ix-bombvault_default", drivers: map[string]string{"ix-bombvault_default": "bridge"}}, true},
		{"Unraid br0", &networkDocker{mode: "br0", drivers: map[string]string{"br0": "macvlan"}}, false},
		{"bridge and macvlan", &networkDocker{mode: "app", drivers: map[string]string{"app": "bridge", "br0.20": "ipvlan"}}, false},
		{"host network", &networkDocker{mode: "host"}, false},
		{"another container's network", &networkDocker{mode: "container:vpn"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := newInstance(t, "cellar", strings.Repeat("a1", 32))
			in.svc.docker = c.docker
			for range 2 {
				_, out := in.do(t, http.MethodGet, "/api/mdns", nil)
				if out["hostOnly"] != c.hostOnly {
					t.Fatalf("hostOnly = %v, want %v", out["hostOnly"], c.hostOnly)
				}
			}
			if c.docker.inspects > 1 && c.docker.mode != "container:vpn" {
				t.Fatalf("Docker was asked %d times, want once", c.docker.inspects)
			}
		})
	}
}
