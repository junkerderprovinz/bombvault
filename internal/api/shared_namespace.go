package api

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/notify"
)

// namespaceOwners names the containers whose network, PID or IPC namespace in
// joins. dockercli.Inspect turns the IDs Docker stores into names.
func namespaceOwners(in model.Inspect) []string {
	var owners []string
	for _, mode := range []string{in.HostConfig.NetworkMode, in.HostConfig.PidMode, in.HostConfig.IpcMode} {
		if owner, ok := strings.CutPrefix(mode, "container:"); ok && owner != "" && !slices.Contains(owners, owner) {
			owners = append(owners, owner)
		}
	}
	return owners
}

// stopContainerOf describes a running container for a stop and a restart in
// order: its compose identity and the namespaces it joins.
func stopContainerOf(name string, in model.Inspect) backup.StopContainer {
	return backup.StopContainer{
		Name:       name,
		ID:         in.ID,
		WasRunning: true,
		Service:    composeService(in.Config.Labels),
		DependsOn:  parseDependsOn(in.Config.Labels),
		Joins:      namespaceOwners(in),
	}
}

// withSharers adds the running containers that share a namespace of target or
// of a container in deps. Docker gives a restarted container a new namespace
// and leaves those sharing the old one running with only a loopback, so they go
// down and come back with it. BombVault's own container cannot stop itself and
// is returned in cutOff. A stopped sharer joins the new namespace by itself at
// its next start.
func (s *Service) withSharers(ctx context.Context, target string, in model.Inspect, deps []backup.StopContainer) (all []backup.StopContainer, cutOff []string) {
	down := map[string]bool{}
	if in.Running {
		down[target] = true
	}
	known := map[string]bool{target: true}
	for _, dep := range deps {
		known[dep.Name] = true
		if dep.WasRunning {
			down[dep.Name] = true
		}
	}
	if len(down) == 0 {
		return deps, nil
	}
	infos, err := s.docker.List(ctx)
	if err != nil {
		log.Printf("api: listing the containers that share a namespace of %q: %v", target, err) //nolint:gosec // G706: %q-quoted
		return deps, nil
	}
	sharers := map[string]model.Inspect{}
	for _, info := range infos {
		if info.State != "running" || known[info.Name] {
			continue
		}
		if si, err := s.docker.Inspect(ctx, info.ID); err == nil && len(namespaceOwners(si)) > 0 {
			sharers[info.Name] = si
		}
	}
	self := s.selfContainerName(ctx)
	for found := true; found; {
		found = false
		for _, info := range infos {
			si, ok := sharers[info.Name]
			if !ok || !slices.ContainsFunc(namespaceOwners(si), func(o string) bool { return down[o] }) {
				continue
			}
			delete(sharers, info.Name)
			if info.Name == self {
				cutOff = append(cutOff, self)
				continue
			}
			found = true
			down[info.Name] = true
			sc := stopContainerOf(info.Name, si)
			sc.Sharer = true
			deps = append(deps, sc)
		}
	}
	return deps, cutOff
}

// settleAroundTarget makes the start order around target possible: a container
// joining the target, directly or through another, starts after it whatever
// else says it should come first, and what a container that starts before the
// target joins has to start before it as well.
func settleAroundTarget(target string, deps []backup.StopContainer) {
	after := map[string]bool{target: true}
	for changed := true; changed; {
		changed = false
		for i := range deps {
			if !after[deps[i].Name] && slices.ContainsFunc(deps[i].Joins, func(o string) bool { return after[o] }) {
				after[deps[i].Name], deps[i].BeforeTarget = true, false
				changed = true
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for i := range deps {
			if !deps[i].BeforeTarget {
				continue
			}
			for j := range deps {
				if !deps[j].BeforeTarget && slices.Contains(deps[i].Joins, deps[j].Name) {
					deps[j].BeforeTarget = true
					changed = true
				}
			}
		}
	}
}

// notifyContainersNotBack tells the user which containers are down or cut off
// after a run. It goes out on its own, past the summary of a scheduled round,
// since a good backup says nothing about the app that went down for it.
func (s *Service) notifyContainersNotBack(ctx context.Context, after string, names []string) {
	c, err := s.NotifyConfig()
	if err != nil || c.On == "" || c.On == "never" {
		return
	}
	msg := fmt.Sprintf("After %s, these containers did not come back: %s", after, strings.Join(names, ", "))
	notify.Send(notify.WithHealthchecksSuppressed(context.Background()), c, "containers", notify.Event{Title: "BombVault", Message: msg})
	if s.unraidGate(c.Unraid) {
		if err := s.sendUnraidNotify(context.WithoutCancel(ctx), "BombVault: containers did not come back", msg, "warning"); err != nil {
			log.Printf("notify: unraid: %v", err)
		}
	}
}
