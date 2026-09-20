package api

import (
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// ownerContext is what deciding a snapshot's owner needs besides its tags.
type ownerContext struct {
	domain string
	known  map[string]bool // identities that exist as an item row or a copy rule
}

func (s *Service) ownerContextFor(domain string) (ownerContext, error) {
	c := ownerContext{domain: domain, known: map[string]bool{}}
	switch domain {
	case "containers":
		rows, err := s.store.ListTargets()
		if err != nil {
			return c, err
		}
		for _, t := range rows {
			c.known["container:"+t.ContainerName] = true
		}
	case "vms":
		rows, err := s.store.ListVMTargets()
		if err != nil {
			return c, err
		}
		for _, v := range rows {
			c.known["vm:"+v.Name] = true
		}
	case "files":
		rows, err := s.store.ListFileSets()
		if err != nil {
			return c, err
		}
		for _, fs := range rows {
			c.known["fileset:"+fs.Name] = true
		}
	default:
		return c, nil
	}
	rules, err := s.store.CopyRulesForDomain(domain)
	if err != nil {
		return c, err
	}
	for identity := range rules {
		c.known[identity] = true
	}
	return c, nil
}

// identityOf returns the identities a tag can belong to: none for a tag of another
// domain or a marker tag, one for an exact tag, two for vm:<a>:zvol:<b> when both
// vm:<a> and vm:<a>:zvol:<b> are known.
func (c ownerContext) identityOf(tag string) []string {
	switch c.domain {
	case "containers":
		for _, prefix := range []string{"container:", "stack:"} {
			if name, ok := strings.CutPrefix(tag, prefix); ok && name != "" {
				return []string{tag}
			}
		}
	case "vms":
		name, ok := strings.CutPrefix(tag, "vm:")
		if !ok || name == "" {
			return nil
		}
		vm, disk := splitDisk(name)
		switch {
		case !disk:
			return []string{tag}
		case c.known[tag] && c.known["vm:"+vm]:
			return []string{"vm:" + vm, tag}
		case c.known[tag]:
			return []string{tag}
		}
		return []string{"vm:" + vm}
	case "files":
		if name, ok := strings.CutPrefix(tag, "fileset:"); ok && name != "" {
			return []string{tag}
		}
	case "flash", "config":
		if tag == c.domain {
			return []string{tag}
		}
	}
	return nil
}

// splitDisk splits a VM tag's name at its last ":zvol:". Device names hold no
// colon, so anything else after that separator is part of a VM name.
func splitDisk(name string) (vm string, ok bool) {
	i := strings.LastIndex(name, ":zvol:")
	if i <= 0 {
		return "", false
	}
	dev := name[i+len(":zvol:"):]
	if dev == "" || strings.Contains(dev, ":") {
		return "", false
	}
	return name[:i], true
}

// snapshotOwner is identityOf applied to one snapshot of a listing.
type snapshotOwner struct {
	Possible []string // every identity the snapshot may belong to; the copy filter needs all of them to exclude
	Owner    string   // the settled owner, "" when the vmrun sibling does not decide
}

// owners settles every snapshot of one listing, keyed by snapshot id.
func (c ownerContext) owners(snaps []restic.Snapshot) map[string]snapshotOwner {
	runs := map[string][]restic.Snapshot{}
	for _, sn := range snaps {
		if run := runTag(sn); run != "" {
			runs[run] = append(runs[run], sn)
		}
	}
	out := make(map[string]snapshotOwner, len(snaps))
	for _, sn := range snaps {
		var o snapshotOwner
		for _, tag := range sn.Tags {
			for _, identity := range c.identityOf(tag) {
				if !slices.Contains(o.Possible, identity) {
					o.Possible = append(o.Possible, identity)
				}
			}
		}
		switch len(o.Possible) {
		case 0:
		case 1:
			o.Owner = o.Possible[0]
		default:
			o.Owner = settledBySibling(sn.ID, o.Possible, runs[runTag(sn)])
		}
		out[sn.ID] = o
	}
	return out
}

// runTag is the vmrun:<id> tag that ties the snapshots of one VM backup together.
func runTag(sn restic.Snapshot) string {
	for _, tag := range sn.Tags {
		if strings.HasPrefix(tag, "vmrun:") {
			return tag
		}
	}
	return ""
}

// settledBySibling picks the one candidate another snapshot of the same VM run
// names, as its own tag or as the VM of a disk tag. With none or several it
// returns "".
func settledBySibling(self string, possible []string, group []restic.Snapshot) string {
	found := ""
	for _, identity := range possible {
		if !namedBySibling(self, identity, group) {
			continue
		}
		if found != "" {
			return ""
		}
		found = identity
	}
	return found
}

func namedBySibling(self, identity string, group []restic.Snapshot) bool {
	for _, other := range group {
		if other.ID == self {
			continue
		}
		for _, tag := range other.Tags {
			if tag == identity {
				return true
			}
			if name, ok := strings.CutPrefix(tag, "vm:"); ok {
				if vm, disk := splitDisk(name); disk && "vm:"+vm == identity {
					return true
				}
			}
		}
	}
	return false
}
