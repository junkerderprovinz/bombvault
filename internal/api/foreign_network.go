package api

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/model"
)

// missingNetworkErr refuses a foreign container restore whose primary network
// does not exist on this host when no network to use instead was chosen.
// Without it the restore would write the appdata and then fail to create the
// container.
type missingNetworkErr struct{ name string }

func (e *missingNetworkErr) Error() string {
	return fmt.Sprintf("network %q does not exist on this host; choose a network to restore the container onto", e.name)
}

// missingNetworkName returns the network err refuses over, or "".
func missingNetworkName(err error) string {
	var m *missingNetworkErr
	if errors.As(err, &m) {
		return m.name
	}
	return ""
}

// customNetwork returns the user-defined network a container's primary
// attachment names, or "" for Docker's built-in modes and a shared namespace.
func customNetwork(in model.Inspect) string {
	mode := in.HostConfig.NetworkMode
	switch {
	case mode == "", mode == "default", mode == "bridge", mode == "host", mode == "none",
		strings.HasPrefix(mode, "container:"):
		return ""
	}
	return mode
}

// restoreNetworks are the networks a container can be moved onto. host and
// none are left out: neither takes the aliases and secondary networks a moved
// container keeps.
func restoreNetworks(have []string) []string {
	return slices.DeleteFunc(slices.Clone(have), func(n string) bool { return n == "host" || n == "none" })
}

// missingNetwork returns the custom primary network of in when this host
// lacks it, and the networks this host has.
func (s *Service) missingNetwork(ctx context.Context, in model.Inspect) (string, []string, error) {
	name := customNetwork(in)
	if name == "" {
		return "", nil, nil
	}
	have, err := s.docker.Networks(ctx)
	if err != nil {
		return "", nil, err
	}
	if slices.Contains(have, name) {
		return "", have, nil
	}
	return name, have, nil
}

// placeOnNetwork refuses a foreign container whose primary network this host
// lacks, unless network names one of this host's networks to use instead.
func (s *Service) placeOnNetwork(ctx context.Context, plan *containerRestorePlan, network string) error {
	missing, have, err := s.missingNetwork(ctx, plan.inspect)
	if err != nil || missing == "" {
		return err
	}
	if network == "" {
		return &missingNetworkErr{name: missing}
	}
	if !slices.Contains(restoreNetworks(have), network) {
		return fmt.Errorf("network %q does not exist on this host", network)
	}
	moveToNetwork(plan, missing, network)
	return nil
}

// moveToNetwork puts the restored container on to in place of its primary
// network from. The static IP and MAC address belonged to the old network and
// are dropped; an ipvlan network refuses a set MAC address outright.
func moveToNetwork(plan *containerRestorePlan, from, to string) {
	in := &plan.inspect
	in.HostConfig.NetworkMode = to
	ep := model.NetworkEndpoint{Name: to, Aliases: in.Network.Aliases}
	in.Network = ep
	nets := []model.NetworkEndpoint{ep}
	for _, n := range in.Networks {
		if n.Name != from && n.Name != to {
			nets = append(nets, n)
		}
	}
	in.Networks = nets
	plan.templateXML = strings.Replace(plan.templateXML, "<Network>"+from+"</Network>", "<Network>"+to+"</Network>", 1)
}

// ForeignContainerNetwork returns the primary network of a foreign container
// when this host lacks it, together with the networks it can be restored onto
// instead. Both are empty when the container's network exists here.
func (s *Service) ForeignContainerNetwork(ctx context.Context, sessionID, item string) (string, []string, error) {
	def, _, err := s.foreignContainerDefinition(sessionID, item)
	if err != nil {
		return "", nil, err
	}
	missing, have, err := s.missingNetwork(ctx, def.Inspect)
	if err != nil || missing == "" {
		return "", nil, err
	}
	return missing, restoreNetworks(have), nil
}
