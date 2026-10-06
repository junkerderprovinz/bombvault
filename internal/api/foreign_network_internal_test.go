package api

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/template"
)

// unraidWeb is a container as Unraid runs it: on br0.20 with a static IP and
// a pinned MAC, also attached to a proxy network.
func unraidWeb() model.Inspect {
	primary := model.NetworkEndpoint{Name: "br0.20", IPv4Address: "192.168.20.5", MACAddress: "02:42:c0:a8:14:05", Aliases: []string{"web"}}
	return model.Inspect{
		Name:       "web",
		Config:     model.Config{Image: "nginx:latest"},
		HostConfig: model.HostConfig{NetworkMode: "br0.20"},
		Network:    primary,
		Networks:   []model.NetworkEndpoint{primary, {Name: "proxy"}},
	}
}

// openForeignWithDef opens a foreign session whose repository holds one
// container definition, and returns the session id.
func openForeignWithDef(t *testing.T, s *Service, in model.Inspect) string {
	t.Helper()
	repoDir := filepath.Join(s.cfg.HostMountRoot, "backups", "other")
	if err := os.MkdirAll(filepath.Join(repoDir, "def"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "config"), []byte("cfg"), 0o600); err != nil {
		t.Fatal(err)
	}
	defJSON, err := json.Marshal(containerDefinition{
		Inspect:     in,
		TemplateXML: "<Container><Network>br0.20</Network></Container>",
	})
	if err != nil {
		t.Fatal(err)
	}
	enc, err := secret.Encrypt(foreignTestKey, defJSON)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "def", in.Name+".def"), enc, 0o600); err != nil {
		t.Fatal(err)
	}
	id, _, err := s.OpenForeign(context.Background(), "backups/other", foreignTestKey, nil)
	if err != nil {
		t.Fatalf("OpenForeign: %v", err)
	}
	return id
}

func TestMoveToNetworkKeepsTheSecondaryNetworks(t *testing.T) {
	plan := containerRestorePlan{inspect: unraidWeb(), templateXML: "<Network>br0.20</Network>"}
	moveToNetwork(&plan, "br0.20", "bridge")

	in := plan.inspect
	if in.HostConfig.NetworkMode != "bridge" {
		t.Fatalf("network mode = %q, want bridge", in.HostConfig.NetworkMode)
	}
	want := model.NetworkEndpoint{Name: "bridge", Aliases: []string{"web"}}
	if !reflect.DeepEqual(in.Network, want) {
		t.Fatalf("primary = %+v, want %+v", in.Network, want)
	}
	if len(in.Networks) != 2 || in.Networks[0].Name != "bridge" || in.Networks[1].Name != "proxy" {
		t.Fatalf("networks = %+v, want bridge then proxy", in.Networks)
	}
	if plan.templateXML != "<Network>bridge</Network>" {
		t.Fatalf("template = %q", plan.templateXML)
	}
}

func TestMoveToNetworkDropsTheChosenNetworkAsSecondary(t *testing.T) {
	plan := containerRestorePlan{inspect: unraidWeb()}
	moveToNetwork(&plan, "br0.20", "proxy")
	if len(plan.inspect.Networks) != 1 || plan.inspect.Networks[0].Name != "proxy" {
		t.Fatalf("networks = %+v, want proxy once", plan.inspect.Networks)
	}
}

func TestCustomNetworkSkipsBuiltInModes(t *testing.T) {
	for _, mode := range []string{"", "default", "bridge", "host", "none", "container:vpn"} {
		if got := customNetwork(model.Inspect{HostConfig: model.HostConfig{NetworkMode: mode}}); got != "" {
			t.Errorf("customNetwork(%q) = %q, want none", mode, got)
		}
	}
	if got := customNetwork(model.Inspect{HostConfig: model.HostConfig{NetworkMode: "immich_default"}}); got != "immich_default" {
		t.Errorf("customNetwork(immich_default) = %q", got)
	}
}

func TestForeignRestoreRefusesAMissingNetworkBeforeWritingAnything(t *testing.T) {
	s := newForeignTestService(t, &foreignRecordingEngine{opens: opensEncrypted})
	d := &foreignFakeDocker{}
	s.docker = d
	id := openForeignWithDef(t, s, unraidWeb())

	started, err := s.StartForeignRestore(context.Background(), id, "containers", "web", "latest", true, "", nil, false, "", "")
	if started || missingNetworkName(err) != "br0.20" {
		t.Fatalf("started=%v err=%v, want a refusal naming br0.20", started, err)
	}
	if _, err := s.store.GetTargetByContainer("web"); err == nil {
		t.Fatal("a refused restore must not adopt the container")
	}
	if d.created != 0 {
		t.Fatalf("created %d containers, want none", d.created)
	}
}

func TestForeignRestoreRefusesANetworkThisHostLacks(t *testing.T) {
	s := newForeignTestService(t, &foreignRecordingEngine{opens: opensEncrypted})
	s.docker = &foreignFakeDocker{}
	id := openForeignWithDef(t, s, unraidWeb())

	for _, network := range []string{"br0.30", "host"} {
		started, err := s.StartForeignRestore(context.Background(), id, "containers", "web", "latest", true, "", nil, false, "", network)
		if started || err == nil {
			t.Fatalf("network %q: started=%v err=%v, want a refusal", network, started, err)
		}
	}
}

func TestForeignRestoreCreatesTheContainerOnTheChosenNetwork(t *testing.T) {
	s := newForeignTestService(t, &foreignRecordingEngine{opens: opensEncrypted})
	d := &foreignFakeDocker{networks: []string{"bridge", "host", "none", "proxy"}}
	s.docker = d
	s.cfg.FlashTemplatesDir = t.TempDir()
	id := openForeignWithDef(t, s, unraidWeb())

	started, err := s.StartForeignRestore(context.Background(), id, "containers", "web", "latest", true, "", nil, false, "", "bridge")
	if err != nil || !started {
		t.Fatalf("StartForeignRestore: started=%v err=%v", started, err)
	}
	waitForeignIdle(t, s)

	d.mu.Lock()
	created, in := d.created, d.createdIn
	d.mu.Unlock()
	if created != 1 {
		t.Fatalf("created %d containers, want one", created)
	}
	if in.HostConfig.NetworkMode != "bridge" || in.Network.Name != "bridge" || in.Network.IPv4Address != "" || in.Network.MACAddress != "" {
		t.Fatalf("recreated on %q with primary %+v, want bridge without the old IP and MAC", in.HostConfig.NetworkMode, in.Network)
	}
	xml, _, err := template.Read(s.cfg.FlashTemplatesDir, "web")
	if err != nil || !strings.Contains(xml, "<Network>bridge</Network>") {
		t.Fatalf("template = %q (err %v), want it on bridge", xml, err)
	}
}

func TestForeignContainerNetworkListsWhereTheContainerCanGo(t *testing.T) {
	s := newForeignTestService(t, &foreignRecordingEngine{opens: opensEncrypted})
	s.docker = &foreignFakeDocker{networks: []string{"bridge", "host", "none", "proxy"}}
	id := openForeignWithDef(t, s, unraidWeb())

	missing, networks, err := s.ForeignContainerNetwork(context.Background(), id, "web")
	if err != nil {
		t.Fatal(err)
	}
	if missing != "br0.20" || !reflect.DeepEqual(networks, []string{"bridge", "proxy"}) {
		t.Fatalf("missing=%q networks=%v, want br0.20 and [bridge proxy]", missing, networks)
	}

	s.docker = &foreignFakeDocker{networks: []string{"br0.20", "bridge"}}
	missing, networks, err = s.ForeignContainerNetwork(context.Background(), id, "web")
	if err != nil || missing != "" || networks != nil {
		t.Fatalf("missing=%q networks=%v err=%v, want nothing when br0.20 exists", missing, networks, err)
	}
}

func TestCheckRestoreFlagsAMissingNetwork(t *testing.T) {
	s := newForeignTestService(t, &foreignRecordingEngine{opens: opensEncrypted})
	s.docker = &foreignFakeDocker{}
	id := openForeignWithDef(t, s, unraidWeb())

	req := RestoreCheckRequest{Kind: checkForeign, Session: id, Domain: "containers", Name: "web", SnapshotID: "latest"}
	res, err := s.CheckRestore(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	last := res.Checks[len(res.Checks)-1]
	if res.Ready || last.ID != lineNetwork || last.Status != lineFail || last.Detail != "br0.20" {
		t.Fatalf("ready=%v checks=%+v, want a failed network line naming br0.20", res.Ready, res.Checks)
	}

	req.Network = "bridge"
	res, err = s.CheckRestore(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range res.Checks {
		if c.ID == lineNetwork {
			t.Fatalf("a chosen network must clear the line, got %+v", res.Checks)
		}
	}
}
