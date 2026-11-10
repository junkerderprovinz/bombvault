package api

import (
	"log"
	"regexp"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// kitSharedSection is the heading of the recovery kit's chapter on shared
// repositories.
const kitSharedSection = "Shared repositories"

// kitSharedFieldRe is a line under a shared repository of the kit that a new
// server reads back.
var kitSharedFieldRe = regexp.MustCompile(`^  (owner|sections|other): (.+)$`)

// sharedKitSection writes the kit's chapter on the shared repositories this
// server copies into or holds. Each names the host this server writes there,
// which every restic command has to select by, and the other servers whose
// backups the same password opens.
func (s *Service) sharedKitSection(w func(string, ...any), settings store.Settings, key string) {
	repos, err := s.store.ListSharedRepos()
	if err != nil {
		log.Printf("api: recovery kit: the shared repositories could not be read: %v", err)
		return
	}
	if len(repos) == 0 {
		return
	}
	targets, err := s.store.ListOffsiteTargets()
	if err != nil {
		log.Printf("api: recovery kit: the copy targets could not be read: %v", err)
	}
	joins, err := s.store.ListSharedJoins(store.SharedJoinOut)
	if err != nil {
		log.Printf("api: recovery kit: the shared repositories this server joined could not be read: %v", err)
	}
	host := restickey.Host(key)
	w("## " + kitSharedSection + "\n\n")
	w("These restic repositories are shared with other BombVault servers of the group.\n")
	w("Every server writes its backups there under a restic host of its own, this one\n")
	w("under %s, so name that host in every restic command, as shown below each\n", host)
	w("repository. The password above opens the backups of the other servers there too,\n")
	w("because every key of a restic repository opens the same master key. Their\n")
	w("containers and VMs come back as files only, since each server seals its own\n")
	w("definitions.\n\n")
	for _, sr := range repos {
		owner := s.ownsSharedRepo(sr)
		name, loc := s.sharedKitName(sr, owner, settings, joins), sr.Repo
		if owner {
			if resolved, err := s.resolveRepo(sr.Repo); err == nil {
				loc = resolved
			}
		}
		w("- %s: %s\n", name, loc)
		w("  owner: %s\n", sr.Owner)
		var sections []string
		for _, domain := range offsiteConfigDomains {
			if slices.ContainsFunc(targets, func(t store.OffsiteTarget) bool { return t.Domain == domain && sameRepoLocation(t.Repo, sr.Repo) }) {
				sections = append(sections, domain)
			}
		}
		if len(sections) > 0 {
			w("  sections: %s\n", strings.Join(sections, ", "))
		}
		for _, m := range sr.Members {
			if m.Host != host {
				w("  other: %s, host %s\n", m.Name, m.Host)
			}
		}
		if owner {
			w("  This server holds it and is the only one that forgets and prunes there.\n")
		}
		w("    restic -r %s snapshots --host %s\n\n", loc, host)
	}
}

// sharedKitName is what the kit calls a shared repository: by the server that
// holds it.
func (s *Service) sharedKitName(sr store.SharedRepo, owner bool, settings store.Settings, joins []store.SharedJoin) string {
	if owner {
		return instanceDisplayName(settings)
	}
	if i := slices.IndexFunc(joins, func(j store.SharedJoin) bool { return j.MemberID == sr.Owner && j.MemberName != "" }); i >= 0 {
		return joins[i].MemberName
	}
	if i := slices.IndexFunc(sr.Members, func(m store.SharedMember) bool { return m.Instance == sr.Owner && m.Name != "" }); i >= 0 {
		return sr.Members[i].Name
	}
	return "Shared repository"
}

// readSharedKitField takes one line under a shared repository of the kit.
func (loc *kitLocation) readSharedKitField(field, value string) {
	switch field {
	case "owner":
		loc.Owner = value
	case "sections":
		for d := range strings.SplitSeq(value, ", ") {
			if slices.Contains(offsiteConfigDomains, d) && !slices.Contains(loc.Sections, d) {
				loc.Sections = append(loc.Sections, d)
			}
		}
	case "other":
		if i := strings.LastIndex(value, ", host "); i >= 0 {
			loc.Others = append(loc.Others, store.SharedMember{Name: value[:i], Host: value[i+len(", host "):]})
		}
	}
}

// attachSharedPlace records a shared repository a recovery kit names, so this
// server, now writing under the host of the one that wrote the kit, lists and
// restores only what that one wrote there. Each section that copied there gets
// its target again. It says which record it made, empty when there was one,
// and which targets.
func (s *Service) attachSharedPlace(p emergencyPlace, ref string) (string, []string, error) {
	recorded := ""
	_, known, err := s.store.SharedRepoAt(p.Location)
	if err != nil {
		return "", nil, err
	}
	if !known {
		g, err := s.store.GetGroupState()
		if err != nil {
			return "", nil, err
		}
		settings, err := s.store.GetSettings()
		if err != nil {
			return "", nil, err
		}
		here := s.resticHost()
		members := slices.DeleteFunc(slices.Clone(p.Others), func(m store.SharedMember) bool { return m.Host == here })
		members = append(members, store.SharedMember{Host: here, Instance: g.InstanceID, Name: instanceDisplayName(settings)})
		sr, err := s.store.PutSharedRepo(store.SharedRepo{Repo: p.Location, Owner: p.Owner, Members: members})
		if err != nil {
			return "", nil, err
		}
		recorded = sr.ID
	}
	if !restic.IsRemoteRepo(p.Location) {
		return recorded, nil, nil
	}
	var made []string
	for _, domain := range p.Sections {
		at := p
		at.Domain, at.Name = domain, "Shared with "+p.Name
		id, err := s.attachOffsiteTarget(at, ref)
		if err != nil {
			return recorded, made, err
		}
		if id != "" {
			made = append(made, id)
		}
	}
	return recorded, made, nil
}
