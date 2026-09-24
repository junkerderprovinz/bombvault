package api

import (
	"log"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// ownerContext is what deciding a snapshot's owner needs besides its tags.
type ownerContext struct {
	domain         string
	known          map[string]bool       // identities that exist as an item row or a copy rule
	aliasPrefix    string                // container: or vm:, empty where no aliases exist
	aliases        map[string]aliasOwner // by the former name's identity
	aliasesUnknown bool                  // the alias table could not be read
	entries        []string              // every row's identity, all possible owners while aliasesUnknown
}

// ownerContextByName is the domain's rows and copy rules, enough to settle a
// snapshot by the name its tag carries today.
func (s *Service) ownerContextByName(domain string) (ownerContext, error) {
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

// identityOf returns the identities a tag can belong to by name: none for a tag
// of another domain or a marker tag, one for an exact tag, two for
// vm:<a>:zvol:<b> when both vm:<a> and vm:<a>:zvol:<b> are known. owners adds
// what the link of a former name decides.
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

// ownersByName settles every snapshot of one listing, keyed by snapshot id.
func (c ownerContext) ownersByName(snaps []restic.Snapshot) map[string]snapshotOwner {
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

// aliasOwner is the entry that a former name's snapshots from before the link
// belong to.
type aliasOwner struct {
	identity string
	linkedAt time.Time
}

// ownerContextFor is ownerContextByName with the domain's former names, which
// owners settles by the snapshot's time against the link.
func (s *Service) ownerContextFor(domain string) (ownerContext, error) {
	c, err := s.ownerContextByName(domain)
	if err != nil {
		return c, err
	}
	aliasDomain, ok := map[string]string{"containers": "container", "vms": "vm"}[domain]
	if !ok {
		return c, nil
	}
	names, err := s.rowNamesByID(domain)
	if err != nil {
		return c, err
	}
	c.aliasPrefix = aliasDomain + ":"
	for _, name := range names {
		c.entries = append(c.entries, c.aliasPrefix+name)
	}
	slices.Sort(c.entries)
	aliases, err := s.store.ListAliases(aliasDomain)
	if err != nil {
		log.Printf("api: %s aliases: %v; any entry may own a snapshot under a former name until this reads", aliasDomain, err) //nolint:gosec // G706: aliasDomain is a fixed literal
		c.aliasesUnknown = true
		return c, nil
	}
	c.aliases = make(map[string]aliasOwner, len(aliases))
	for _, a := range aliases {
		// A row deleted between the two reads took its aliases with it.
		name, ok := names[a.TargetID]
		if !ok {
			continue
		}
		id := c.aliasPrefix + a.OldName
		c.aliases[id] = aliasOwner{identity: c.aliasPrefix + name, linkedAt: time.Unix(a.LinkedAt, 0)}
		c.known[id] = true
	}
	return c, nil
}

// rowNamesByID maps the container or VM rows of domain from id to name.
func (s *Service) rowNamesByID(domain string) (map[string]string, error) {
	names := map[string]string{}
	if domain == "vms" {
		vms, err := s.store.ListVMTargets()
		if err != nil {
			return nil, err
		}
		for _, v := range vms {
			names[v.ID] = v.Name
		}
		return names, nil
	}
	targets, err := s.store.ListTargets()
	if err != nil {
		return nil, err
	}
	for _, t := range targets {
		names[t.ID] = t.ContainerName
	}
	return names, nil
}

// owners settles each snapshot by name, then gives a snapshot under a former
// name to the renamed entry when it was taken before the link.
func (c ownerContext) owners(snaps []restic.Snapshot) map[string]snapshotOwner {
	out := c.ownersByName(snaps)
	if len(c.aliases) == 0 && !c.aliasesUnknown {
		return out
	}
	for _, snap := range snaps {
		if o, ok := out[snap.ID]; ok {
			out[snap.ID] = c.throughLinks(snap, o)
		}
	}
	return out
}

// throughLinks applies the links of former names to an owner settled by name.
func (c ownerContext) throughLinks(snap restic.Snapshot, o snapshotOwner) snapshotOwner {
	ts, timed := snapshotTime(snap)
	var out snapshotOwner
	for _, id := range o.Possible {
		possible, _ := c.throughLink(id, ts, timed)
		out.Possible = append(out.Possible, possible...)
	}
	if o.Owner != "" {
		_, out.Owner = c.throughLink(o.Owner, ts, timed)
	}
	slices.Sort(out.Possible)
	out.Possible = slices.Compact(out.Possible)
	return out
}

// throughLink resolves one identity: a former name belongs to the renamed entry
// before its link and to whoever carries the name from the link on. owner is ""
// when the time or the alias table cannot tell.
func (c ownerContext) throughLink(id string, ts time.Time, timed bool) (possible []string, owner string) {
	if c.aliasesUnknown && strings.HasPrefix(id, c.aliasPrefix) {
		return append([]string{id}, c.entries...), ""
	}
	a, ok := c.aliases[id]
	switch {
	case !ok:
		return []string{id}, id
	case !timed:
		return []string{a.identity, id}, ""
	case ts.Before(a.linkedAt):
		return []string{a.identity}, a.identity
	}
	return []string{id}, id
}

// namesItem reports whether snap carries identity, or a former name linked to
// it, as a tag of its own. A disk image carries neither, only a tag derived
// from one.
func (c ownerContext) namesItem(snap restic.Snapshot, identity string) bool {
	for _, tag := range snap.Tags {
		if tag == identity || c.aliases[tag].identity == identity {
			return true
		}
	}
	return false
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
