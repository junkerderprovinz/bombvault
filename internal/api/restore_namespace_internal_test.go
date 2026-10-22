package api

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
)

// namespaceDocker knows containers by name and by ID, and the live recipe of
// the container being restored.
type namespaceDocker struct {
	dockercli.Docker
	names map[string]string // name or ID -> name
	live  map[string]model.Inspect
}

func (d namespaceDocker) InspectName(_ context.Context, ref string) (string, error) {
	return d.names[ref], nil
}

func (d namespaceDocker) Inspect(_ context.Context, name string) (model.Inspect, error) {
	in, ok := d.live[name]
	if !ok {
		return model.Inspect{}, errors.New("no such container")
	}
	return in, nil
}

var (
	oldGluetunID = strings.Repeat("a", 64)
	newGluetunID = strings.Repeat("b", 64)
)

// A definition from an older version names gluetun by an ID a recreate has
// replaced. The qbittorrent still running shares gluetun's network under its
// new ID, which names the container it shares with today.
func TestNamespacesByNameFindsTheContainerAnOldDefinitionMeant(t *testing.T) {
	s := &Service{docker: namespaceDocker{
		names: map[string]string{newGluetunID: "gluetun", "gluetun": "gluetun"},
		live: map[string]model.Inspect{"qbittorrent": {HostConfig: model.HostConfig{
			NetworkMode: "container:" + newGluetunID,
		}}},
	}}
	hc := s.namespacesByName(context.Background(), "qbittorrent", model.HostConfig{
		NetworkMode: "container:" + oldGluetunID,
		IpcMode:     "container:gluetun",
		PidMode:     "host",
	})
	if hc.NetworkMode != "container:gluetun" || hc.IpcMode != "container:gluetun" || hc.PidMode != "host" {
		t.Fatalf("modes = %q %q %q", hc.NetworkMode, hc.IpcMode, hc.PidMode)
	}
}

// With the container being restored gone too, nothing tells which container
// was meant, and the mode stays for the pre-flight to refuse.
func TestNamespacesByNameLeavesAModeNothingResolves(t *testing.T) {
	s := &Service{docker: namespaceDocker{names: map[string]string{}, live: map[string]model.Inspect{}}}
	hc := s.namespacesByName(context.Background(), "qbittorrent", model.HostConfig{NetworkMode: "container:" + oldGluetunID})
	if hc.NetworkMode != "container:"+oldGluetunID {
		t.Fatalf("network mode = %q, want it unchanged", hc.NetworkMode)
	}
}
