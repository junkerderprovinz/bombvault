package api

import (
	"slices"
	"sort"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/model"
)

// Kinds of change between a backed-up definition and the one in use now.
// Added is in the backup only, removed in use now only.
const (
	changeAdded   = "added"
	changeChanged = "changed"
	changeRemoved = "removed"
)

// DefinitionChange is one difference between the definition a restore
// recreates and the one in use now. Backup and Now hold the two values; one
// is empty when the entry exists on one side only. An environment variable
// carries its name alone, because its value may be a password.
type DefinitionChange struct {
	Field  string `json:"field"`
	Name   string `json:"name,omitempty"`
	Change string `json:"change"`
	Backup string `json:"backup,omitempty"`
	Now    string `json:"now,omitempty"`
}

func containerChanges(backup, now model.Inspect) []DefinitionChange {
	out := []DefinitionChange{}
	if a, b := containerImage(backup), containerImage(now); a != b {
		out = append(out, DefinitionChange{Field: "image", Change: changeChanged, Backup: a, Now: b})
	}
	out = append(out, setChanges("port", portList(backup.HostConfig.PortBindings), portList(now.HostConfig.PortBindings))...)
	out = append(out, envChanges(backup.Config.Env, now.Config.Env)...)
	out = append(out, setChanges("volume", backup.HostConfig.Binds, now.HostConfig.Binds)...)
	return out
}

func containerImage(in model.Inspect) string {
	if in.Config.Image != "" {
		return in.Config.Image
	}
	return in.Image
}

// portList spells each published port as "host -> container".
func portList(bindings map[string][]model.PortBinding) []string {
	var out []string
	for port, binds := range bindings {
		for _, b := range binds {
			host := b.HostPort
			if b.HostIP != "" && b.HostIP != "0.0.0.0" {
				host = b.HostIP + ":" + host
			}
			out = append(out, host+" -> "+port)
		}
	}
	return out
}

// setChanges lists what the restore adds (in the backup only) and removes (in
// use now only).
func setChanges(field string, backup, now []string) []DefinitionChange {
	out := []DefinitionChange{}
	for _, v := range sortedUnique(backup) {
		if !slices.Contains(now, v) {
			out = append(out, DefinitionChange{Field: field, Change: changeAdded, Backup: v})
		}
	}
	for _, v := range sortedUnique(now) {
		if !slices.Contains(backup, v) {
			out = append(out, DefinitionChange{Field: field, Change: changeRemoved, Now: v})
		}
	}
	return out
}

func envChanges(backup, now []string) []DefinitionChange {
	b, n := envMap(backup), envMap(now)
	names := make([]string, 0, len(b)+len(n))
	for k := range b {
		names = append(names, k)
	}
	for k := range n {
		if _, ok := b[k]; !ok {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	out := []DefinitionChange{}
	for _, k := range names {
		bv, inB := b[k]
		nv, inN := n[k]
		switch {
		case inB && !inN:
			out = append(out, DefinitionChange{Field: "env", Name: k, Change: changeAdded})
		case !inB && inN:
			out = append(out, DefinitionChange{Field: "env", Name: k, Change: changeRemoved})
		case bv != nv:
			out = append(out, DefinitionChange{Field: "env", Name: k, Change: changeChanged})
		}
	}
	return out
}

func envMap(env []string) map[string]string {
	m := make(map[string]string, len(env))
	for _, e := range env {
		k, v, _ := strings.Cut(e, "=")
		m[k] = v
	}
	return m
}

func sortedUnique(in []string) []string {
	out := slices.Clone(in)
	sort.Strings(out)
	return slices.Compact(out)
}
