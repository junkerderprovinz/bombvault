package api

import (
	"regexp"

	"github.com/junkerderprovinz/bombvault/internal/compose"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
)

// orphanEntry is a not-installed store entry considered as a rename match: the
// container name plus the stored definition's Inspect and TemplateXML.
// AppdataPaths is left out because it holds BombVault's own container-visible
// paths, not the host paths Docker reports for a live container, so the
// appdata signal reads the raw bind mounts off Inspect instead.
type orphanEntry struct {
	Name        string
	Inspect     model.Inspect
	TemplateXML string
}

// templateLineage is the Unraid user-template directory as read by
// readTemplateLineage, which keeps the matcher free of filesystem access. Both
// maps are keyed by container name, not by template file name.
type templateLineage struct {
	// Present marks the names that have a template on the flash.
	Present map[string]bool
	// XMLByName holds the content of each present template that could be read.
	XMLByName map[string]string
}

// RenameCandidate is a live container's one not-installed match: the entry it
// looks renamed from, and why. Reason is a machine-readable code; the frontend
// owns the wording.
type RenameCandidate struct {
	OldName string
	Reason  string
}

// Reason codes for RenameCandidate, one per hard signal. When a pair agrees on
// several, matchRenames reports the first in its order.
const (
	reasonDockerID        = "docker-id"
	reasonAppdataBind     = "appdata-bind"
	reasonComposeService  = "compose-service"
	reasonTemplateLineage = "template-lineage"
	// reasonLibvirtUUID is the VM signal: Unraid keeps a domain's libvirt UUID
	// across a rename, so VMs need none of the container heuristics.
	reasonLibvirtUUID = "libvirt-uuid"
)

// dateInstalledRe matches the template's <DateInstalled> element, which, like
// <Name>, differs between a stored template and its renamed container's.
var dateInstalledRe = regexp.MustCompile(`<DateInstalled>[^<]*</DateInstalled>`)

// matchRenames finds, for each live container, the not-installed entry it was
// renamed from. It returns one-to-one matches only: a live container with
// several candidate entries, or an entry with several candidate containers, is
// left to the manual picker.
//
// A pair matches on one hard signal, checked in this order (the order only
// picks the reported reason):
//
//  1. the same Docker ID, which `docker rename` keeps;
//  2. the same non-empty set of appdata bind mounts (a bind whose host source
//     contains one of dataRootSegments as a full path segment), compared as
//     (source, destination) pairs, where each source is bound by exactly one
//     live container and one entry in the whole input, so a shared folder or
//     several instances of one image never count;
//  3. the same compose project and service;
//  4. the Unraid template lineage: the entry's template is gone from the
//     flash, the live container's is present, and the two match apart from
//     <Name> and <DateInstalled>.
//
// Image, static IP or MAC, host ports and creation time are left out: they may
// support a suggestion but never decide one.
func matchRenames(live []dockercli.ContainerInfo, orphans []orphanEntry, tmpl templateLineage, dataRootSegments []string) map[string]RenameCandidate {
	liveAppdata := make([]map[bindPair]bool, len(live))
	liveBySource := make(map[string]int, len(live))
	for i, c := range live {
		liveAppdata[i] = appdataBindsLive(c.Mounts, dataRootSegments)
		for src := range sourcesOf(liveAppdata[i]) {
			liveBySource[src]++
		}
	}

	orphanAppdata := make([]map[bindPair]bool, len(orphans))
	orphanBySource := make(map[string]int, len(orphans))
	for j, o := range orphans {
		orphanAppdata[j] = appdataBindsOrphan(o.Inspect.Mounts, dataRootSegments)
		for src := range sourcesOf(orphanAppdata[j]) {
			orphanBySource[src]++
		}
	}

	type candidate struct {
		orphan int
		reason string
	}
	candidatesForLive := make([][]candidate, len(live))
	matchCountForOrphan := make([]int, len(orphans))

	for i, c := range live {
		for j, o := range orphans {
			reason, ok := hardMatch(c, o, liveAppdata[i], orphanAppdata[j], liveBySource, orphanBySource, tmpl)
			if !ok {
				continue
			}
			candidatesForLive[i] = append(candidatesForLive[i], candidate{orphan: j, reason: reason})
			matchCountForOrphan[j]++
		}
	}

	out := make(map[string]RenameCandidate)
	for i, c := range live {
		if len(candidatesForLive[i]) != 1 {
			continue // no candidate, or more than one: manual picker
		}
		m := candidatesForLive[i][0]
		if matchCountForOrphan[m.orphan] != 1 {
			continue // this entry also matches another live container
		}
		out[c.Name] = RenameCandidate{OldName: orphans[m.orphan].Name, Reason: m.reason}
	}
	return out
}

// hardMatch reports which hard signal, if any, live container c and entry o
// share. cAppdata and oAppdata are their appdata bind sets; liveBySource and
// orphanBySource count each source across the input for signal 2.
func hardMatch(c dockercli.ContainerInfo, o orphanEntry, cAppdata, oAppdata map[bindPair]bool, liveBySource, orphanBySource map[string]int, tmpl templateLineage) (string, bool) {
	if c.ID != "" && o.Inspect.ID != "" && c.ID == o.Inspect.ID {
		return reasonDockerID, true
	}
	if appdataBindsMatch(cAppdata, oAppdata, liveBySource, orphanBySource) {
		return reasonAppdataBind, true
	}
	if composeMatch(c.Labels, o.Inspect.Config.Labels) {
		return reasonComposeService, true
	}
	if templateLineageMatch(o, c.Name, tmpl) {
		return reasonTemplateLineage, true
	}
	return "", false
}

// bindPair is one bind mount reduced to host source and container destination.
// The appdata set is keyed by the pair because Docker can mount one host path
// at two destinations, and a source-keyed map would drop one of them.
type bindPair struct {
	Source      string
	Destination string
}

// appdataBindsLive returns a live container's bind mounts under one of the
// configured data-root segments. ContainerInfo.Mounts holds bind mounts only.
func appdataBindsLive(mounts []dockercli.MountPoint, dataRootSegments []string) map[bindPair]bool {
	out := make(map[bindPair]bool, len(mounts))
	for _, m := range mounts {
		if m.Source == "" || m.Destination == "" || !matchesAnyDataRootSegment(m.Source, dataRootSegments) {
			continue
		}
		out[bindPair{Source: m.Source, Destination: m.Destination}] = true
	}
	return out
}

// appdataBindsOrphan is appdataBindsLive for an entry's stored Inspect, whose
// mounts include volumes.
func appdataBindsOrphan(mounts []model.Mount, dataRootSegments []string) map[bindPair]bool {
	out := make(map[bindPair]bool, len(mounts))
	for _, m := range mounts {
		if m.Type != "bind" || m.Source == "" || m.Destination == "" || !matchesAnyDataRootSegment(m.Source, dataRootSegments) {
			continue
		}
		out[bindPair{Source: m.Source, Destination: m.Destination}] = true
	}
	return out
}

// sourcesOf returns the distinct sources in pairs, so a container that mounts
// one path twice counts once in the exclusivity check.
func sourcesOf(pairs map[bindPair]bool) map[string]bool {
	out := make(map[string]bool, len(pairs))
	for p := range pairs {
		out[p.Source] = true
	}
	return out
}

// appdataBindsMatch is hard signal 2. Equality is per pair but exclusivity is
// per source: two containers mounting one folder at different destinations
// still share it.
func appdataBindsMatch(live, orphan map[bindPair]bool, liveBySource, orphanBySource map[string]int) bool {
	if len(live) == 0 || !bindSetsEqual(live, orphan) {
		return false
	}
	for src := range sourcesOf(live) {
		if liveBySource[src] != 1 || orphanBySource[src] != 1 {
			return false // shared by another live container and/or another entry
		}
	}
	return true
}

// bindSetsEqual reports whether a and b hold exactly the same bind pairs.
func bindSetsEqual(a, b map[bindPair]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for p := range a {
		if !b[p] {
			return false
		}
	}
	return true
}

// composeMatch is hard signal 3. An empty project or service never matches,
// or every container outside compose would match every other.
func composeMatch(liveLabels, orphanLabels map[string]string) bool {
	project := compose.Project(liveLabels)
	service := compose.Service(liveLabels)
	if project == "" || service == "" {
		return false
	}
	return project == compose.Project(orphanLabels) && service == compose.Service(orphanLabels)
}

// templateLineageMatch is hard signal 4. Unraid deletes a user template when
// its container is renamed but not when it is removed, so the old template's
// absence is itself evidence.
func templateLineageMatch(o orphanEntry, liveName string, tmpl templateLineage) bool {
	if o.TemplateXML == "" {
		return false // nothing stored to compare against
	}
	if tmpl.Present[o.Name] || !tmpl.Present[liveName] {
		return false
	}
	newXML, ok := tmpl.XMLByName[liveName]
	if !ok {
		return false
	}
	return normalizeTemplate(o.TemplateXML) == normalizeTemplate(newXML)
}

// normalizeTemplate strips the two elements a rename legitimately changes, so
// the rest of the template can be compared byte for byte.
func normalizeTemplate(xml string) string {
	xml = templateNameRe.ReplaceAllString(xml, "")
	xml = dateInstalledRe.ReplaceAllString(xml, "")
	return xml
}
