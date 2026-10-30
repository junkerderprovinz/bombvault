package api

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// locationFault is one place where the storage location model breaks a rule
// the rest of BombVault takes for granted.
type locationFault struct {
	problem string
	// fix names the repair, so faults that share one run it once. A fault
	// without a fix that is safe without asking has none.
	fix    string
	repair func() (string, error)
}

// locationModel is the storage location model read once.
type locationModel struct {
	settings     store.Settings
	targets      []store.OffsiteTarget
	targetByID   map[string]store.OffsiteTarget
	named        map[string]store.OffsiteTarget
	destinations map[string]bool
	defaults     []store.PlacementDefault
	rules        []store.CopyRule
	formerRepos  []string
}

func (s *Service) readLocationModel() (locationModel, error) {
	var m locationModel
	var err error
	if m.settings, err = s.store.GetSettings(); err != nil {
		return m, err
	}
	if m.targets, err = s.store.ListOffsiteTargets(); err != nil {
		return m, err
	}
	m.targetByID = map[string]store.OffsiteTarget{}
	for _, t := range m.targets {
		m.targetByID[t.ID] = t
	}
	if m.named, err = s.namedRepoIndex(); err != nil {
		return m, err
	}
	destinations, err := s.store.ListDestinations()
	if err != nil {
		return m, err
	}
	m.destinations = destinationIDs(destinations)
	if m.defaults, err = s.store.ListPlacementDefaults(); err != nil {
		return m, err
	}
	if m.rules, err = s.store.ListCopyRules(); err != nil {
		return m, err
	}
	m.formerRepos, err = s.store.FormerHomeRepos()
	return m, err
}

// locationFaults reads the whole storage location model and returns every
// fault in it.
func (s *Service) locationFaults() ([]locationFault, error) {
	m, err := s.readLocationModel()
	if err != nil {
		return nil, err
	}
	var out []locationFault
	for _, domain := range offsiteConfigDomains {
		if domainEnabled(m.settings, domain) && strings.TrimSpace(domainPathRaw(domain, m.settings)) == "" {
			out = append(out, locationFault{problem: fmt.Sprintf("the %s section is switched on and has no folder of its own", domain)})
		}
		out = append(out, s.ownFolderFaults(m, domain)...)
	}
	for _, t := range m.targets {
		if t.DestinationID != "" && !m.destinations[t.DestinationID] {
			out = append(out, locationFault{problem: fmt.Sprintf("the %s copy %q follows the storage location %s, which is gone",
				t.Domain, placementTargetName(t), t.DestinationID)})
		}
	}
	for _, d := range m.defaults {
		out = append(out, s.skipFaults(m, d.Domain, "the "+d.Domain+" default", d.Skip)...)
		out = append(out, s.homeFaults(m, d.Domain, "the "+d.Domain+" default", d.Home)...)
	}
	for _, r := range m.rules {
		out = append(out, s.skipFaults(m, r.Domain, "the copy rule of "+r.Identity, r.Skip)...)
	}
	for _, domain := range store.PlacementDomains {
		items, err := s.domainItems(domain)
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			out = append(out, s.homeFaults(m, domain, it.identity, it.home.Repo)...)
		}
	}
	datasets, err := s.store.ListZFSDatasets()
	if err != nil {
		return nil, err
	}
	for _, d := range datasets {
		out = append(out, s.homeFaults(m, zfsDomain, "zfs:"+d.Dataset, d.Repo)...)
	}
	for _, repo := range m.formerRepos {
		if _, ok := m.named[repo]; !ok {
			out = append(out, s.goneRepoFault(fmt.Sprintf("an entry lists the repository %s among its former homes, which is gone", repo), repo))
		}
	}
	out = append(out, s.directFaults(m)...)
	out = append(out, s.repoOnOwnFolderFaults(m)...)
	return out, nil
}

// repoOnOwnFolderFaults finds a repository at a section's own folder. Entries
// there would write into the section's repository with sign-in and settings
// of their own, and removing either place would take the other's backups.
func (s *Service) repoOnOwnFolderFaults(m locationModel) []locationFault {
	var out []locationFault
	for _, domain := range offsiteConfigDomains {
		own, err := s.repoFor(m.settings, domain, "local")
		if err != nil {
			continue
		}
		for _, r := range m.named {
			if loc, err := s.resolveRepo(r.Repo); err == nil && sameRepoLocation(own, loc) {
				out = append(out, locationFault{problem: fmt.Sprintf("the repository %q lies at the own folder of the %s section", r.Name, domain)})
			}
		}
	}
	return out
}

// ownFolderFaults finds a copy and an off-site field of the section that name
// its own folder. A copy there lands on itself and fails each run, and an
// off-site field there brings that copy back as the primary target.
func (s *Service) ownFolderFaults(m locationModel, domain string) []locationFault {
	own, err := s.repoFor(m.settings, domain, "local")
	if err != nil {
		return nil
	}
	onOwn := func(loc string) bool {
		repo, err := s.resolveRepo(loc)
		return err == nil && sameRepoLocation(own, repo)
	}
	var out []locationFault
	for _, t := range m.targets {
		if t.Domain != domain || !onOwn(t.Repo) {
			continue
		}
		out = append(out, locationFault{
			problem: fmt.Sprintf("the %s copy %q sits on the section's own folder", domain, placementTargetName(t)),
			fix:     "copy " + t.ID,
			repair: func() (string, error) {
				err := s.store.RemoveCopyOnOwnFolder(t.ID, func(st *store.Settings, t store.OffsiteTarget) bool {
					if !onOwn(offsiteRepoFromSettings(t.Domain, *st)) {
						return false
					}
					clearOffsiteFields(st, []string{t.Domain})
					return true
				})
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("offsite: removed the %s copy %q, which sat on the section's own folder", domain, placementTargetName(t)), nil
			},
		})
	}
	if field := strings.TrimSpace(offsiteRepoFromSettings(domain, m.settings)); field != "" && onOwn(field) {
		out = append(out, locationFault{
			problem: fmt.Sprintf("the %s off-site field names the section's own folder", domain),
			fix:     "field " + domain,
			repair: func() (string, error) {
				cleared := false
				_, err := s.store.MutateSettings(func(st *store.Settings) error {
					if onOwn(offsiteRepoFromSettings(domain, *st)) {
						clearOffsiteFields(st, []string{domain})
						cleared = true
					}
					return nil
				})
				if err != nil || !cleared {
					return "", err
				}
				return fmt.Sprintf("offsite: cleared the %s off-site field, which named the section's own folder", domain), nil
			},
		})
	}
	return out
}

// skipFaults finds the ids in a skip list that name no copy of the section.
// Saving a list like that is refused, and so is importing a file that carries
// it.
func (s *Service) skipFaults(m locationModel, domain, owner string, skip []string) []locationFault {
	var out []locationFault
	for _, id := range skip {
		if id == store.SkipAll {
			continue
		}
		if t, ok := m.targetByID[id]; ok && t.Domain == domain {
			continue
		}
		out = append(out, locationFault{
			problem: fmt.Sprintf("%s skips %s, which is no copy of the %s section", owner, id, domain),
			fix:     "skip " + domain + " " + id,
			repair: func() (string, error) {
				if err := s.store.ForgetSkippedTarget(domain, id); err != nil {
					return "", err
				}
				return fmt.Sprintf("placement: took %s out of the %s skip lists, since no copy of the section has that id", id, domain), nil
			},
		})
	}
	return out
}

// homeFaults finds a home that names a repository that is gone, or the direct
// repository of another section's copy.
func (s *Service) homeFaults(m locationModel, domain, owner, home string) []locationFault {
	if home == "" || home == store.HomeNowhere {
		return nil
	}
	r, ok := m.named[home]
	if !ok {
		return []locationFault{s.goneRepoFault(fmt.Sprintf("%s backs up to the repository %s, which is gone", owner, home), home)}
	}
	if t, ok := m.targetByID[r.CompanionOf]; ok && t.Domain != domain {
		return []locationFault{{problem: fmt.Sprintf("%s backs up to the direct repository %q of a %s copy", owner, r.Name, t.Domain)}}
	}
	return nil
}

// goneRepoFault sends what still names a repository that is gone back to its
// section, as removing the repository does.
func (s *Service) goneRepoFault(problem, repo string) locationFault {
	return locationFault{
		problem: problem,
		fix:     "repo " + repo,
		repair: func() (string, error) {
			if err := s.store.ForgetGoneRepo(repo); err != nil {
				return "", err
			}
			return fmt.Sprintf("placement: sent what backed up to the repository %s, which is gone, back to its section", repo), nil
		},
	}
}

// directFaults finds a direct repository whose copy is gone, and a copy with
// more than one.
func (s *Service) directFaults(m locationModel) []locationFault {
	var out []locationFault
	count := map[string]int{}
	for _, r := range m.named {
		if r.CompanionOf == "" {
			continue
		}
		count[r.CompanionOf]++
		if _, ok := m.targetByID[r.CompanionOf]; ok {
			continue
		}
		out = append(out, locationFault{
			problem: fmt.Sprintf("the direct repository %q belongs to the copy %s, which is gone", r.Name, r.CompanionOf),
			fix:     "direct " + r.CompanionOf,
			repair: func() (string, error) {
				if err := s.store.ReleaseDirectOfGoneTarget(r.CompanionOf); err != nil {
					return "", err
				}
				return fmt.Sprintf("offsite: the direct repository %q stays as a repository of its own, since its copy is gone", r.Name), nil
			},
		})
	}
	for id, n := range count {
		if n > 1 {
			out = append(out, locationFault{problem: fmt.Sprintf("the copy %s has %d direct repositories", id, n)})
		}
	}
	return out
}

// RepairLocationModel puts right every fault in the storage location model
// that has a fix safe without asking, logs each fix and returns their lines.
// A fault without one is logged and left as it is.
func (s *Service) RepairLocationModel() ([]string, error) {
	faults, err := s.locationFaults()
	if err != nil {
		return nil, err
	}
	var fixed []string
	var errs []error
	done := map[string]bool{}
	for _, f := range faults {
		if f.repair == nil {
			log.Printf("storage: %s, which is left as it is", f.problem) //nolint:gosec // G706: names and ids in it are %q-quoted or come from the database
			continue
		}
		if done[f.fix] {
			continue
		}
		done[f.fix] = true
		line, err := f.repair()
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", f.problem, err))
			continue
		}
		if line != "" {
			log.Print(line)
			fixed = append(fixed, line)
		}
	}
	return fixed, errors.Join(errs...)
}
