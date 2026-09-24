package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Discover rebuilds BombVault's target list from the backup storage after a
// fresh install or loss of /config. It lists the containers repo's
// snapshots (tagged container:<name>), reads and decrypts each container's
// mirrored definition, and upserts a target so the container can be
// restored. The result counts the containers discovered. Containers whose
// definition is missing or undecryptable are skipped and logged.
//
// dryRun makes it read-only: it opens the repo and decrypts the definitions
// (proving the repo is reachable and the APP_KEY is correct) and returns
// the same count, but writes no targets. The Recovery tab's readability
// probe uses this, so checking "is my backup readable?" never resurrects
// orphan entries; only the explicit "Discover backups" action rebuilds
// targets (#44).
func (s *Service) Discover(ctx context.Context, dryRun bool) (DiscoverResult, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return DiscoverResult{}, fmt.Errorf("read settings: %w", err)
	}
	// The distinct container names from the container:<name> tags, across every
	// repository this domain writes to, each with the named repository (#204) it
	// was found in. A not-yet-created repo yields nothing. A read failure comes
	// back as readErr together with whatever the named repositories yielded, so
	// an install whose domain repository is unreadable is still rebuilt as far
	// as it can be; the Recovery wizard classifies on that error.
	names, formerNames, skipped, directRows, readErr := s.discoverNamesAcrossRepos(ctx, settings, "containers", "container:")
	findings, fErr := s.directFindings("containers", directRows)
	if fErr != nil {
		log.Printf("api: discover containers: could not match direct repositories to targets: %v", fErr)
	}

	dir, err := s.defsDir(settings)
	if err != nil {
		return DiscoverResult{Direct: findings}, err
	}
	legacyDir, err := s.legacyDefsDir(settings)
	if err != nil {
		return DiscoverResult{Direct: findings}, err
	}
	res := DiscoverResult{Skipped: skipped, LeftOpen: []string{}, Direct: findings}
	unlock, locked := s.discoverLock("containers", dryRun)
	defer unlock()
	// storedDef reads name's mirrored definition. The item's own mirror comes
	// first: a container on a named repository has its definition beside its
	// snapshots, and the domain's mirror never had it.
	storedDef := func(name, repoID string) ([]byte, containerDefinition, error) {
		fn, err := defFileName(name)
		if err != nil {
			return nil, containerDefinition{}, err
		}
		lookIn := dir
		if own := s.itemDefsDir(repoID, false); own != "" {
			lookIn = own
		}
		enc, err := readStoredDef(lookIn, legacyDir, fn)
		if err != nil && lookIn != dir {
			enc, err = readStoredDef(dir, legacyDir, fn) // older backups mirrored to the domain
		}
		if err != nil {
			return nil, containerDefinition{}, fmt.Errorf("no stored definition: %w", err)
		}
		plain, err := secret.Decrypt(s.cfg.AppKey, enc)
		if err != nil {
			return nil, containerDefinition{}, fmt.Errorf("the definition does not decrypt (wrong APP_KEY?): %w", err)
		}
		var def containerDefinition
		if err := json.Unmarshal(plain, &def); err != nil {
			return nil, containerDefinition{}, fmt.Errorf("the definition is corrupt: %w", err)
		}
		return plain, def, nil
	}
	// rebuildOne reports whether name's stored definition is present,
	// decryptable and parseable, whatever dryRun says; dryRun only gates the
	// write.
	rebuildOne := func(name, repoID string) bool {
		plain, def, err := storedDef(name, repoID)
		if err != nil {
			log.Printf("api: discover: skipping %q, which cannot be recreated: %v", name, err) //nolint:gosec // G706: %q-quoted
			return false
		}
		if !dryRun {
			// A row this call is about to create takes its home from the pass only
			// while the domain lock was taken; otherwise it starts open, the same
			// as an existing row the lock refused, so discoverHome below reports it
			// in LeftOpen instead of the insert setting it straight through.
			write := store.HomeWrite{Choice: store.RepoOpen}
			if locked {
				write = s.discoverWrite("containers", name, repoID, readErr)
			}
			if _, uErr := s.store.UpsertTarget(store.Target{
				ContainerName: name,
				AppdataPaths:  def.AppdataPaths,
				Definition:    string(plain),
				Repo:          write.Repo,
				RepoChosen:    write.Choice,
			}); uErr != nil {
				log.Printf("api: discover: could not upsert target %q: %v", name, uErr) //nolint:gosec // G706: %q-quoted
				return false
			}
			left, hErr := s.discoverHome(ctx, store.ItemRef{Domain: "containers", Key: name}, repoID, readErr, locked)
			if hErr != nil {
				log.Printf("api: discover: could not restore the repository of %q: %v", name, hErr) //nolint:gosec // G706: %q-quoted
			}
			if left {
				res.LeftOpen = append(res.LeftOpen, name)
			}
		}
		return true
	}
	// A name that a stored definition records as a former name is left to
	// foldFormerNames, which runs once every other name has been rebuilt.
	claims := recordedFormerNames(names, func(name, repoID string) []definitionAlias {
		_, def, _ := storedDef(name, repoID) // rebuildOne logs why one does not read
		return def.Aliases
	})
	for name, repoID := range names {
		if len(claims[name]) > 0 {
			continue
		}
		if rebuildOne(name, repoID) {
			res.Found++
		}
	}
	// The fold runs once the loop above has rebuilt what it could, and never on
	// a dry run, which writes nothing.
	if !dryRun {
		logUnrecordedFormerNames("containers", formerNames, claims)
		res.Found += s.foldFormerNames(ctx, settings, "container", names, claims, rebuildOne,
			func(old, targetID string, record definitionAlias) error {
				_, err := s.store.AddAliasAt("container", old, targetID, record.LinkedAt)
				return err
			})
	}
	if !dryRun && res.Found > 0 {
		if err := s.pauseAfterDiscover(ctx, "containers", &res); err != nil {
			readErr = errors.Join(readErr, err)
		}
	}
	// The result first, the read failure last: a caller that branches on err
	// still sees it, and one that shows a partial rebuild has something to show.
	return res, readErr
}

// foldFormerNames links each name a rebuilt name's stored definition records
// as a former name to the first claimant with a row, with that claimant's
// record, and returns how many of those names it rebuilt as rows of their own:
// a name no claimant could be rebuilt for, and a name another machine has been
// backed up under since the link, which keeps its own row beside the link.
// A copy rule on a name it links moves to the claimant, as a takeover moves
// it, unless a container or VM installed under that name follows it, or the
// installed ones cannot be listed. Names and claimants are taken in sorted
// order, so a repository always rebuilds the same way.
func (s *Service) foldFormerNames(ctx context.Context, settings store.Settings, domain string, names map[string]string, claims map[string][]formerNameClaim,
	rebuild func(name, repoID string) bool, link func(old, targetID string, record definitionAlias) error) int {
	settingsDomain, _, _ := aliasDomain(domain)
	if len(claims) == 0 {
		return 0
	}
	var installed map[string]bool
	var listErr error
	if domain == "vm" {
		installed, listErr = s.definedVMs(ctx)
	} else {
		installed, listErr = s.installedContainers(ctx)
	}
	if listErr != nil {
		log.Printf("api: discover %s: the installed %s could not be listed, so a copy rule on a former name stays there: %v", settingsDomain, settingsDomain, listErr)
	}
	rebuilt := 0
	for _, old := range slices.Sorted(maps.Keys(claims)) {
		byOwner := claims[old]
		slices.SortFunc(byOwner, func(a, b formerNameClaim) int { return strings.Compare(a.owner, b.owner) })
		var owner, targetID string
		var record definitionAlias
		for _, c := range byOwner {
			if id, err := s.entryIDByName(domain, c.owner); err == nil {
				owner, targetID, record = c.owner, id, c.record
				break
			}
		}
		if owner == "" {
			curs := make([]string, 0, len(byOwner))
			for _, c := range byOwner {
				curs = append(curs, c.owner)
			}
			log.Printf("api: discover %s: none of %q, which record %q as a former name, could be rebuilt, so it is rebuilt on its own", settingsDomain, curs, old) //nolint:gosec // G706: names %q-quoted, the domain a fixed literal
			if rebuild(old, names[old]) {
				rebuilt++
			}
			continue
		}
		linked := false
		if a, err := s.store.AliasByOldName(domain, old); err == nil {
			if a.TargetID != targetID {
				log.Printf("api: discover %s: %q is already linked to another entry; leaving it alone", settingsDomain, old) //nolint:gosec // G706: name %q-quoted, the domain a fixed literal
				continue
			}
		} else {
			if _, err := s.entryIDByName(domain, old); err == nil {
				log.Printf("api: discover %s: %q already has its own row, so it is not linked to %q", settingsDomain, old, owner) //nolint:gosec // G706: names %q-quoted, the domain a fixed literal
				continue
			}
			if !validFormerName(domain, old) {
				log.Printf("api: discover %s: %q is not linked to %q: it cannot be written as a backup tag", settingsDomain, old, owner) //nolint:gosec // G706: names %q-quoted, the domain a fixed literal
				continue
			}
			if err := link(old, targetID, record); err != nil {
				log.Printf("api: discover %s: could not link %q to %q: %v", settingsDomain, old, owner, err) //nolint:gosec // G706: names %q-quoted, the domain a fixed literal
				continue
			}
			linked = true
		}
		if s.formerNameReusedSince(ctx, settings, domain, old, names[old], record.LinkedAt) && rebuild(old, names[old]) {
			rebuilt++
		}
		// After the rebuild, so a reused name that got its row back keeps its rule.
		if linked && listErr == nil {
			s.placementMu.Lock()
			err := s.store.CarryFormerNameRule(domain, old, installed)
			s.placementMu.Unlock()
			if err != nil {
				log.Printf("api: discover %s: the copy rule of %q could not move to %q, so it stays on the former name: %v", settingsDomain, old, owner, err) //nolint:gosec // G706: names %q-quoted, the domain a fixed literal
			}
		}
	}
	return rebuilt
}

// formerNameReusedSince reports whether old's backups, in the repository
// Discover found them in, hold one that a link at linkedAt does not claim:
// another machine has been backed up under the name since. A listing that
// fails counts too, so such a machine's backups never lose their entry.
func (s *Service) formerNameReusedSince(ctx context.Context, settings store.Settings, domain, old, repoID string, linkedAt int64) bool {
	settingsDomain, prefix, _ := aliasDomain(domain)
	claim := newAliasClaim(prefix, store.Alias{OldName: old, LinkedAt: linkedAt})
	repo, err := s.itemRepoPath(repoID, func() (string, error) { return s.repoFor(settings, settingsDomain, "local") })
	var snaps []restic.Snapshot
	if err == nil {
		snaps, err = s.snapshotsForTag(ctx, repo, s.primaryModeFor(settings, settingsDomain, repo), claim.tag)
	}
	if err != nil {
		log.Printf("api: discover %s: the backups of %q could not be listed, so it is rebuilt as its own entry beside the link: %v", settingsDomain, old, scrubError(err)) //nolint:gosec // G706: name %q-quoted, the domain a fixed literal and the error scrubbed
		return true
	}
	return !claim.claimsEvery(snaps)
}

// validFormerName reports whether name may become an alias in domain. Every
// later backup writes it into a formerly: tag, and restic splits a tag at a
// comma.
func validFormerName(domain, name string) bool {
	if domain == "vm" {
		return validVMName(name) && !strings.Contains(name, ",")
	}
	return validResourceName(name)
}

// DiscoverVMs rebuilds the VM target list from backup storage, the VM
// counterpart of Discover, after a fresh install or database loss, so a VM
// that was deleted from the host (or whose target is gone) becomes
// restorable again. It lists the vms repo's snapshots (tagged vm:<name>),
// reads and decrypts each VM's mirrored definition, and upserts a target.
// VMs whose definition is missing or undecryptable are skipped. The result
// counts the VMs discovered. dryRun makes it read-only: it opens the repo
// and decrypts the definitions to prove readability and the APP_KEY, and
// returns the same count, but writes no targets. The Recovery readability
// probe uses this so it never resurrects orphan VM entries (#44). A name
// that another VM's stored definition records as a former name is linked to
// that VM, and rebuilt beside the link only when a later VM has been backed
// up under it; see foldFormerNames.
func (s *Service) DiscoverVMs(ctx context.Context, dryRun bool) (DiscoverResult, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return DiscoverResult{}, fmt.Errorf("read settings: %w", err)
	}
	// Every repository this domain writes to (#204), with the one each name was
	// found in; a not-yet-created repo yields nothing. A read failure comes back
	// as readErr together with whatever the named repositories yielded, so an
	// install whose domain repository is unreadable is still rebuilt as far as
	// it can be; the Recovery wizard classifies on that error.
	names, formerNames, skipped, directRows, readErr := s.discoverNamesAcrossRepos(ctx, settings, "vms", "vm:")
	findings, fErr := s.directFindings("vms", directRows)
	if fErr != nil {
		log.Printf("api: discover vms: could not match direct repositories to targets: %v", fErr)
	}

	dir, err := s.vmDefsDir(settings)
	if err != nil {
		return DiscoverResult{Direct: findings}, err
	}
	legacyDir, err := s.legacyVMDefsDir(settings)
	if err != nil {
		return DiscoverResult{Direct: findings}, err
	}
	res := DiscoverResult{Skipped: skipped, LeftOpen: []string{}, Direct: findings}
	unlock, locked := s.discoverLock("vms", dryRun)
	defer unlock()
	// storedDef reads name's definition file, from beside its own snapshots
	// first as in Discover.
	storedDef := func(name, repoID string) ([]byte, vmDefinition, error) {
		fn, err := defFileName(name)
		if err != nil {
			return nil, vmDefinition{}, err
		}
		lookIn := dir
		if own := s.itemDefsDir(repoID, true); own != "" {
			lookIn = own
		}
		enc, err := readStoredDef(lookIn, legacyDir, fn)
		if err != nil && lookIn != dir {
			enc, err = readStoredDef(dir, legacyDir, fn)
		}
		if err != nil {
			return nil, vmDefinition{}, fmt.Errorf("no stored definition: %w", err)
		}
		plain, err := secret.Decrypt(s.cfg.AppKey, enc)
		if err != nil {
			return nil, vmDefinition{}, fmt.Errorf("the definition does not decrypt (wrong APP_KEY?): %w", err)
		}
		var def vmDefinition
		if err := json.Unmarshal(plain, &def); err != nil {
			return nil, vmDefinition{}, fmt.Errorf("the definition is corrupt: %w", err)
		}
		return plain, def, nil
	}
	rebuildOne := func(name, repoID string) bool {
		plain, def, err := storedDef(name, repoID)
		if err != nil {
			log.Printf("api: discover vms: skipping %q, which cannot be recreated: %v", name, err) //nolint:gosec // G706: %q-quoted
			return false
		}
		method := def.Method
		if method == "" {
			method = "graceful"
		}
		if !dryRun {
			// See Discover: a row this call is about to create starts open unless
			// the domain lock was taken, so discoverHome reports it in LeftOpen
			// instead of the insert setting its home straight through.
			write := store.HomeWrite{Choice: store.RepoOpen}
			if locked {
				write = s.discoverWrite("vms", name, repoID, readErr)
			}
			if _, uErr := s.store.UpsertVMTarget(store.VMTarget{
				Name:       name,
				Method:     method,
				Definition: string(plain),
				Repo:       write.Repo,
				RepoChosen: write.Choice,
			}); uErr != nil {
				log.Printf("api: discover vms: could not upsert target %q: %v", name, uErr) //nolint:gosec // G706: %q-quoted
				return false
			}
			left, hErr := s.discoverHome(ctx, store.ItemRef{Domain: "vms", Key: name}, repoID, readErr, locked)
			if hErr != nil {
				log.Printf("api: discover vms: could not restore the repository of %q: %v", name, hErr) //nolint:gosec // G706: %q-quoted
			}
			if left {
				res.LeftOpen = append(res.LeftOpen, name)
			}
		}
		return true
	}
	// As in Discover, a recorded former name waits for the fold.
	claims := recordedFormerNames(names, func(name, repoID string) []definitionAlias {
		_, def, _ := storedDef(name, repoID) // rebuildOne logs why one does not read
		return def.Aliases
	})
	for name, repoID := range names {
		if len(claims[name]) > 0 {
			continue
		}
		if rebuildOne(name, repoID) {
			res.Found++
		}
	}
	// As in Discover, the fold runs after the loop and never on a dry run.
	if !dryRun {
		logUnrecordedFormerNames("vms", formerNames, claims)
		res.Found += s.foldFormerNames(ctx, settings, "vm", names, claims, rebuildOne,
			func(old, targetID string, record definitionAlias) error {
				if record.PrevDefinition == "" {
					log.Printf("api: discover vms: the link record of %q carries no definition, so it is linked without one and cannot be unlinked", old) //nolint:gosec // G706: %q-quoted
				}
				_, err := s.store.AddVMAliasAt(old, targetID, record.LinkedAt, record.PrevDefinition)
				return err
			})
	}
	if !dryRun && res.Found > 0 {
		if err := s.pauseAfterDiscover(ctx, "vms", &res); err != nil {
			readErr = errors.Join(readErr, err)
		}
	}
	// The result first, the read failure last: a caller that branches on err
	// still sees it, and one that shows a partial rebuild has something to show.
	return res, readErr
}

// discoverNamesAcrossRepos collects the item names a domain's snapshots
// carry (from the tagPrefix tag, e.g. "container:") across every repository
// the domain writes to, and remembers which repository each name was found
// in: the value is the named repository's id (#204), or "" for the domain's
// own.
//
// Discover is the path back from a lost /config: it rebuilds items out of
// the snapshots that still exist. An item pointed at a named repository
// must be found there, or its intact backups would never be looked at
// again, and it must be put back on that repository, or its next backup
// would go somewhere else and show an empty history.
//
// A named repository that cannot be listed is skipped rather than failing
// the whole discovery. The domain's own listing failure is returned as an
// error: a wrong APP_KEY, an unmounted share or a corrupt repository all
// surface there, and the Recovery wizard classifies on exactly that error
// to tell "nothing found" from "this box cannot open its own repository".
// As a skip it would show a yellow "not reachable" pill with no message on
// the one screen an operator reaches on their worst day.
//
// The repository list is built here rather than taken from
// domainReposInUse, which derives it from the item rows discovery exists
// to rebuild. After a /config loss those tables are empty, so every named
// repository would drop out of the pass meant to find them; the same
// happens without any loss once the last item using a repository is
// removed. So every enabled named repository is searched, in use or not.
//
// A name is attributed to the repository holding its newest snapshot, not
// to "a named repository wins". If a retired containers folder is
// registered as a named repository so one item can still read it, that
// rule would re-home every container with old snapshots there onto it, and
// its next backup's retention would prune the archive under the domain
// keep-policy while its real history in the domain repository was
// orphaned. The newest snapshot is the only evidence available here of
// where an item is currently sent.
//
// The named repositories are searched before the domain's own: a failure
// on the domain's own ends the pass, so it has to come after everything
// else has been searched, and the error still reaches the caller that
// classifies on it.
//
// The second result maps a discovered name to the formerly: names on its
// own snapshots, so Discover can name each one no stored definition records
// as a link. Only container and VM backups write that tag. The fourth is
// every plain named repository holding bv:direct snapshots of the domain: a
// direct repository that lost its link.
func (s *Service) discoverNamesAcrossRepos(ctx context.Context, settings store.Settings, domain, tagPrefix string) (map[string]string, map[string][]string, []repoSkip, []store.OffsiteTarget, error) {
	own, err := s.repoFor(settings, domain, "local")
	if err != nil {
		return nil, nil, nil, nil, err
	}
	// Every enabled named repository that resolves somewhere else first, the
	// domain's own last: its listing failure ends the pass, so by then the
	// named repositories have already been searched.
	var refs []domainRepoRef
	var skipped []repoSkip
	var direct []store.OffsiteTarget
	named, nErr := s.store.ListNamedRepos()
	if nErr != nil {
		skipped = append(skipped, repoSkip{Name: "the named repositories", Reason: "their list could not be read", Unreachable: true})
		log.Printf("api: discover %s: could not list the named repositories (searching the domain repository only): %v", domain, nErr) //nolint:gosec // G706: domain is a fixed literal
	}
	for _, n := range named {
		if !n.Enabled {
			// Marked Note: switching a repository off is a first-class state, not something
			// that went wrong. It is still said, because after a /config loss the
			// operator needs to know which repositories were left out of the search,
			// but it must not colour the readability pill or fail the pass, or retiring
			// one share would leave Recovery permanently amber and swallow the
			// confirmation toast behind it.
			skipped = append(skipped, repoSkip{Name: scrubSafeName(n.Name), Reason: "switched off", Note: true})
			continue
		}
		loc, rErr := s.resolveRepo(n.Repo)
		if rErr != nil {
			skipped = append(skipped, repoSkip{Name: scrubSafeName(n.Name), Reason: "its location does not resolve", Unreachable: true})
			continue
		}
		if sameRepoLocation(loc, own) {
			continue
		}
		refs = append(refs, namedRef(loc, n))
	}
	refs = append(refs, ownRef(own))
	// best[name] is the newest snapshot seen for that name so far, and the
	// repository id it was in.
	type candidate struct {
		id   string
		when int64
	}
	best := map[string]candidate{}
	// formerly[name] holds the former names on name's snapshots in every
	// repository, not only the one best settles on: a formerly: tag holds
	// wherever it was written.
	formerly := map[string]map[string]bool{}
	// id → display name, so the duplicate-name line below can actually name both
	// sides instead of only saying that two exist. "" is the domain's own.
	refNames := map[string]string{}
	for _, ref := range refs {
		id := ""
		if !ref.Own {
			id = ref.Named.ID
		}
		refNames[id] = s.refName(ref)
	}
	for _, ref := range refs {
		if localRepoMissing(ref.Loc) {
			// The same three-way split the other two repository loops make: a location
			// that was never a repository holds nothing and is dropped silently, one
			// that was a working repository and is now unreachable is the most
			// important thing this pass can report, and "could not tell" goes to the
			// reported side.
			//
			// For the domain's own repository the two reported cases end the pass,
			// like the listing failure below. This branch fires before listSnapshots,
			// and Discover's attribution gate keys on the returned error: an unmounted
			// share takes this path, so without the error the newest-wins comparison
			// would be decided without the repository most items are in, and the
			// result would stick.
			est := s.repoEstablishmentOf(ref.Loc)
			if ref.Own && (est == repoWasEstablished || est == repoEstablishmentUnknown) {
				reason := "it was there before and is not reachable now"
				if est == repoEstablishmentUnknown {
					reason = "it is not reachable now, and whether it ever held backups could not be read"
				}
				log.Printf("api: discover %s: the domain's own repository is not there (%s)", domain, reason) //nolint:gosec // G706: domain is a fixed literal and the reason a fixed string
				skipped = append(skipped, repoSkip{Name: s.refName(ref), Reason: reason, Unreachable: true})
				out := make(map[string]string, len(best))
				for name, c := range best {
					out[name] = c.id
				}
				return out, formerlyOut(formerly), skipped, direct, errors.New("the " + domain + " repository is not reachable: " + reason)
			}
			switch est {
			case repoWasEstablished:
				skipped = append(skipped, repoSkip{Name: s.refName(ref), Reason: "it was there before and is not reachable now", Unreachable: true})
			case repoEstablishmentUnknown:
				skipped = append(skipped, repoSkip{Name: s.refName(ref), Reason: "it is not reachable now, and whether it ever held backups could not be read", Unreachable: true})
			case repoNeverEstablished:
				// Never created. For a named repository that is silence.
				//
				// For the domain's own it is a Note, not an error. An install with every
				// item on a named repository (#204) never creates the domain repository
				// at all, so ending the pass here would break exactly that configuration.
				//
				// The limit: after a /config loss the established marker is gone with the
				// database, so a repository that really exists on an unmounted share also
				// answers "never" here. That combination (configuration lost, share
				// unmounted, and a named repository holding older snapshots of the same
				// item) is what this branch cannot separate. It is named rather than
				// silent so the wizard shows it before anybody trusts the result, and it
				// is a Note rather than a skip because an all-named install would
				// otherwise report a permanent fault.
				if ref.Own {
					skipped = append(skipped, repoSkip{
						Name:   s.refName(ref),
						Reason: "it has not been created yet; if its share is simply not mounted, mount it and search again before trusting this result",
						Note:   true,
					})
				}
			}
			continue
		}
		mode := s.primaryModeFor(settings, domain, ref.Loc)
		snaps, sErr := s.listSnapshots(ctx, ref.Loc, mode)
		if sErr != nil {
			// The domain's own failure ends the pass. This is the error the Recovery
			// wizard classifies on (a wrong APP_KEY, an unmounted share, a corrupt
			// repository); as a skip nobody reads it would turn a red "the APP_KEY
			// differs from when this repo was first created" panel into a silent "0
			// found".
			//
			// It ends the pass with what the pass already has. The named repositories
			// are searched first and are already in `best`, and throwing them away
			// would turn "rebuilt three of five items, and here is why the rest are
			// missing" into "rebuilt nothing". A caller that only wants the error
			// still gets it.
			if ref.Own {
				log.Printf("api: discover %s: could not read the domain's own repository: %v", domain, scrubError(sErr)) //nolint:gosec // G706: domain is a fixed literal and the error scrubbed
				skipped = append(skipped, repoSkip{Name: s.refName(ref), Reason: scrubError(sErr), Unreachable: true})
				out := make(map[string]string, len(best))
				for name, c := range best {
					out[name] = c.id
				}
				return out, formerlyOut(formerly), skipped, direct, sErr
			}
			skipped = append(skipped, repoSkip{Name: s.refName(ref), Reason: scrubError(sErr), Unreachable: true})
			log.Printf("api: discover %s: could not read %s (continuing): %v", domain, s.refName(ref), scrubError(sErr)) //nolint:gosec // G706: domain is a fixed literal, the name is the row's own and the error scrubbed
			continue
		}
		if !ref.Own && ref.Named.CompanionOf == "" && holdsDirect(snaps, tagPrefix) {
			direct = append(direct, ref.Named)
		}
		id := ""
		if !ref.Own {
			id = ref.Named.ID
		}
		for _, snap := range snaps {
			when := int64(0)
			if ts, pErr := time.Parse(time.RFC3339Nano, snap.Time); pErr == nil {
				when = ts.Unix()
			}
			// A backup after a rename carries its identity tag and one formerly:
			// tag per former name, so each former name here belongs to the
			// identity tags on the same snapshot.
			var formerHere []string
			for _, tag := range snap.Tags {
				if old, ok := strings.CutPrefix(tag, "formerly:"); ok && old != "" {
					formerHere = append(formerHere, old)
				}
			}
			for _, tag := range snap.Tags {
				rest, ok := strings.CutPrefix(tag, tagPrefix)
				if !ok || rest == "" {
					continue
				}
				if len(formerHere) > 0 {
					set, ok := formerly[rest]
					if !ok {
						set = map[string]bool{}
						formerly[rest] = set
					}
					for _, old := range formerHere {
						set[old] = true
					}
				}
				prev, seen := best[rest]
				if seen && prev.id != id && prev.when != 0 && when != 0 {
					// Logged at the decision, naming both repositories: "it is in two places"
					// without saying which two is not something anybody can act on. It fires
					// once per pair of differing snapshots rather than once per name, because
					// the decision is re-made whenever a newer snapshot turns up in the other
					// place.
					log.Printf("api: discover %s: %q has backups in both %s and %s; taking the one with the newest snapshot", domain, rest, refNames[prev.id], refNames[id]) //nolint:gosec // G706: domain is a fixed literal, the name is %q-quoted and the repository names are shortened
				}
				// Strictly newer wins, and on a tie the domain's own repository keeps the
				// name. The tie is real: a repository duplicated by `restic copy` or a
				// plain folder copy carries identical snapshot times, and the paths differ
				// so sameRepoLocation does not catch it. The domain's own is the safe
				// answer there, and since the named repositories are listed first, the
				// first-seen rule alone would hand the name to whichever of them came
				// first.
				ownWinsTie := seen && when == prev.when && ref.Own && prev.id != ""
				if !seen || when > prev.when || ownWinsTie {
					best[rest] = candidate{id: id, when: when}
				}
			}
		}
	}
	out := make(map[string]string, len(best))
	for name, c := range best {
		out[name] = c.id
	}
	return out, formerlyOut(formerly), skipped, direct, nil
}

// formerlyOut flattens formerly into sorted lists, so Discover writes the same
// aliases on every run.
func formerlyOut(formerly map[string]map[string]bool) map[string][]string {
	if len(formerly) == 0 {
		return nil
	}
	out := make(map[string][]string, len(formerly))
	for name, set := range formerly {
		olds := make([]string, 0, len(set))
		for old := range set {
			olds = append(olds, old)
		}
		sort.Strings(olds)
		out[name] = olds
	}
	return out
}

// DiscoverFileSets rebuilds the file-set list from backup storage, the
// files counterpart of Discover and DiscoverVMs, after a fresh install or
// database loss. Unlike containers and VMs the files domain mirrors no
// definitions to the repo (there is nothing to recreate beyond the
// folder's content), so discovery works from the fileset:<Name> snapshot
// tags alone: every unknown name is stored as a disabled, path-less set,
// because the original source path cannot be known from tags; the UI flags
// "set path before backup" while restore to a folder already works.
//
// An existing set keeps its path, excludes and enabled state: those are
// the operator's own configuration. Its repository is the one exception,
// and only while it is still open or left unread by an earlier pass: such
// a set with no backups where it is pointed now is put back on the
// repository its snapshots were found in, the same repair Discover and
// DiscoverVMs make. The result counts the file sets found. dryRun makes it
// read-only: it lists and counts but writes nothing. The Recovery
// readability probe uses this so it never resurrects orphan entries (#44).
func (s *Service) DiscoverFileSets(ctx context.Context, dryRun bool) (DiscoverResult, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return DiscoverResult{}, fmt.Errorf("read settings: %w", err)
	}
	// Every repository this domain writes to (#204), with the one each name was
	// found in; a not-yet-created repo yields nothing. A read failure comes back
	// as readErr together with whatever the named repositories yielded, so an
	// install whose domain repository is unreadable is still rebuilt as far as
	// it can be; the Recovery wizard classifies on that error.
	names, _, skipped, directRows, readErr := s.discoverNamesAcrossRepos(ctx, settings, "files", "fileset:")
	findings, fErr := s.directFindings("files", directRows)
	if fErr != nil {
		log.Printf("api: discover files: could not match direct repositories to targets: %v", fErr)
	}

	res := DiscoverResult{Skipped: skipped, LeftOpen: []string{}, Direct: findings}
	unlock, locked := s.discoverLock("files", dryRun)
	defer unlock()
	for name, repoID := range names {
		// Defense-in-depth: only BombVault's own backups write fileset: tags, but
		// a name that fails the boundary charset (it feeds tags and progress keys)
		// is skipped rather than stored.
		if !validResourceName(name) {
			log.Printf("api: discover files: skipping unsafe file set name %q", name) //nolint:gosec // G706: %q-quoted
			continue
		}
		if dryRun {
			res.Found++ // probe: count what a real discover would surface, write nothing
			continue
		}
		// A set this pass is about to create starts open unless the domain lock
		// was taken, the same rule the existing-row branch below gets from
		// discoverHome, so a create under a running backup is reported in
		// LeftOpen instead of getting its home straight through the insert.
		write := store.HomeWrite{Choice: store.RepoOpen}
		if locked {
			write = s.discoverWrite("files", name, repoID, readErr)
		}
		existing, gErr := s.store.GetFileSetByName(name)
		switch {
		case errors.Is(gErr, sql.ErrNoRows):
			if _, cErr := s.store.CreateFileSet(store.FileSet{Name: name, Enabled: false, Repo: write.Repo, RepoChosen: write.Choice}); cErr != nil {
				log.Printf("api: discover files: could not create set %q: %v", name, cErr) //nolint:gosec // G706: %q-quoted
				continue
			}
			if !locked {
				res.LeftOpen = append(res.LeftOpen, name)
			}
		case gErr != nil:
			log.Printf("api: discover files: could not read set %q: %v", name, gErr) //nolint:gosec // G706: %q-quoted
			continue
		default:
			left, hErr := s.discoverHome(ctx, store.ItemRef{Domain: "files", Key: existing.ID}, repoID, readErr, locked)
			if hErr != nil {
				log.Printf("api: discover files: could not restore the repository of %q: %v", name, hErr) //nolint:gosec // G706: %q-quoted
			}
			if left {
				res.LeftOpen = append(res.LeftOpen, name)
			}
		}
		res.Found++
	}
	if !dryRun && res.Found > 0 {
		if err := s.pauseAfterDiscover(ctx, "files", &res); err != nil {
			readErr = errors.Join(readErr, err)
		}
	}
	return res, readErr
}
