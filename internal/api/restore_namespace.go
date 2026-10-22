package api

import (
	"context"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/model"
)

// namespacesByName points the shared namespaces of a recipe at the containers
// they belong to by name. A definition from an older version holds the other
// container's ID, which is gone once that container was recreated. The
// container being restored, while it still exists, then tells which container
// it currently shares with. A mode neither resolves stays as it is, and the
// restore's pre-flight refuses it.
func (s *Service) namespacesByName(ctx context.Context, name string, hc model.HostConfig) model.HostConfig {
	var live *model.HostConfig
	liveRead := false
	owner := func(mode string) string {
		ref, ok := strings.CutPrefix(mode, "container:")
		if !ok || ref == "" {
			return ""
		}
		got, err := s.docker.InspectName(ctx, ref)
		if err != nil {
			return ""
		}
		return got
	}
	resolve := func(mode string, of func(model.HostConfig) string) string {
		if !strings.HasPrefix(mode, "container:") {
			return mode
		}
		if o := owner(mode); o != "" {
			return "container:" + o
		}
		if !liveRead {
			liveRead = true
			if in, err := s.docker.Inspect(ctx, name); err == nil {
				live = &in.HostConfig
			}
		}
		if live != nil {
			if o := owner(of(*live)); o != "" {
				return "container:" + o
			}
		}
		return mode
	}
	hc.NetworkMode = resolve(hc.NetworkMode, func(h model.HostConfig) string { return h.NetworkMode })
	hc.PidMode = resolve(hc.PidMode, func(h model.HostConfig) string { return h.PidMode })
	hc.IpcMode = resolve(hc.IpcMode, func(h model.HostConfig) string { return h.IpcMode })
	return hc
}
