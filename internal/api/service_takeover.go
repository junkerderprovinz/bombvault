package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// templateNameRe matches the template's <Name> element, the container's
// display name. A <Config Name="..."> attribute is a setting's label and does
// not match.
var templateNameRe = regexp.MustCompile(`<Name>[^<]*</Name>`)

// rewriteTemplateXMLName sets the template's <Name> element to newName. A
// template without one, including an empty template, is returned unchanged.
func rewriteTemplateXMLName(xml, newName string) string {
	if xml == "" {
		return xml
	}
	return templateNameRe.ReplaceAllString(xml, "<Name>"+newName+"</Name>")
}

// rewriteDefinitionJSON returns definition, a target's stored recreate recipe,
// with Inspect.Name and the template's <Name> set to name, so a restore before
// the next backup recreates the container under its new name. It does no
// store I/O because the takeover and the unlink write its result in the same
// transaction as the rename; a restore can then never pair the new name with
// the old definition. An empty definition (never backed up) is returned
// unchanged.
func rewriteDefinitionJSON(definition, name string) (string, error) {
	if definition == "" {
		return "", nil
	}
	var def containerDefinition
	if err := json.Unmarshal([]byte(definition), &def); err != nil {
		return "", fmt.Errorf("unmarshal: %w", err)
	}
	if strings.HasPrefix(def.Inspect.Name, "/") {
		def.Inspect.Name = "/" + name
	} else {
		def.Inspect.Name = name
	}
	def.TemplateXML = rewriteTemplateXMLName(def.TemplateXML, name)
	defBytes, err := json.Marshal(def)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}
	return string(defBytes), nil
}

// rewriteStopLists points every other entry's stop list from oldName to
// newName, so a container that stops the renamed one during its own backup
// keeps stopping it.
func (s *Service) rewriteStopLists(oldName, newName string) error {
	targets, err := s.store.ListTargets()
	if err != nil {
		return fmt.Errorf("rewrite stop lists: list targets: %w", err)
	}
	for _, t := range targets {
		if t.ContainerName == newName {
			continue
		}
		changed := false
		stop := make([]string, len(t.StopContainers))
		for i, n := range t.StopContainers {
			if n == oldName {
				n = newName
				changed = true
			}
			stop[i] = n
		}
		if !changed {
			continue
		}
		if err := s.store.SetStopContainers(t.ContainerName, stop); err != nil {
			return fmt.Errorf("rewrite stop lists: %q: %w", t.ContainerName, err)
		}
	}
	return nil
}

// configuredStateLabels lists the operator-set fields t carries, the ones a
// takeover may not discard just because the row has no backups yet.
func configuredStateLabels(t store.Target) []string {
	var labels []string
	if t.IncludeInSchedule {
		labels = append(labels, "scheduled")
	}
	if t.PreHook != "" || t.PostHook != "" {
		labels = append(labels, "hooks")
	}
	if len(t.Excludes) > 0 {
		labels = append(labels, "excludes")
	}
	if len(t.ExcludeCaches) > 0 {
		labels = append(labels, "exclude-caches")
	}
	if len(t.StopContainers) > 0 {
		labels = append(labels, "stop list")
	}
	if len(t.SelectedPaths) > 0 {
		labels = append(labels, "selected paths")
	}
	if t.Repo != "" {
		labels = append(labels, "repository override")
	}
	if t.ScheduleCadence != "" {
		labels = append(labels, "schedule cadence")
	}
	if t.BackupOrder != 0 {
		labels = append(labels, "backup order")
	}
	if t.UpdateAfterBackup {
		labels = append(labels, "update-after-backup")
	}
	return labels
}

// targetOccupyingNewNameIsEmpty reports whether t, the row already on a
// takeover's new name, may be deleted to make way for the entry on from: it has
// no backups, no operator-set configuration and no copy rule other than the
// entry's own. labels names what it found, so the refusal can say what would
// be lost.
func (s *Service) targetOccupyingNewNameIsEmpty(ctx context.Context, t store.Target, from string) (empty bool, labels []string, err error) {
	labels, err = s.withCopyRule(configuredStateLabels(t), "containers", "container:"+from, "container:"+t.ContainerName)
	if err != nil {
		return false, nil, err
	}
	if len(labels) > 0 {
		return false, labels, nil
	}
	hasBackups, err := s.containerHasBackups(ctx, t.ContainerName)
	if err != nil {
		return false, nil, err
	}
	return !hasBackups, nil, nil
}

// ownNamesFor returns the target's current name and every alias recorded for
// it.
func (s *Service) ownNamesFor(domain, targetID, currentName string) (map[string]bool, error) {
	own := map[string]bool{currentName: true}
	names, err := s.store.AliasNames(domain, targetID)
	if err != nil {
		return nil, fmt.Errorf("read alias names for %q: %w", currentName, err)
	}
	for _, n := range names {
		own[n] = true
	}
	return own, nil
}

// namesAFormerName reports whether snap carries a formerly: tag naming one of
// ownNames, which marks it as a backup of that entry made after a takeover.
func namesAFormerName(snap restic.Snapshot, ownNames map[string]bool) bool {
	for _, t := range snap.Tags {
		if n, ok := strings.CutPrefix(t, "formerly:"); ok && ownNames[n] {
			return true
		}
	}
	return false
}

// reposToCheckForTakeover returns the repositories an entry of domain
// ("container" or "vm") could read newName's snapshots from after a takeover:
// the one newName resolves to, and ownRepo, the entry's own, which the rename
// keeps. A stranger's history in either would become the entry's.
func (s *Service) reposToCheckForTakeover(settings store.Settings, domain, newName, ownRepo string) ([]string, error) {
	repoForName := s.containerRepoForName
	if domain == "vm" {
		repoForName = s.vmRepoForName
	}
	newRepo, err := repoForName(settings, newName, "")
	if err != nil {
		return nil, err
	}
	if ownRepo == newRepo {
		return []string{newRepo}, nil
	}
	return []string{newRepo, ownRepo}, nil
}

// refuseForeignBackups refuses to move entry targetID of domain from
// currentName onto newName while a backup under newName in repos or an
// off-site target is not the entry's, or while one of them cannot be read,
// since retention would then age a stranger's snapshots with the entry's. A
// backup naming one of the entry's names as a former name is its own, made
// before an unlink.
func (s *Service) refuseForeignBackups(ctx context.Context, settings store.Settings, domain, targetID, currentName, newName string, repos []string) error {
	settingsDomain, prefix, _ := aliasDomain(domain)
	own, err := s.ownNamesFor(domain, targetID, currentName)
	if err != nil {
		return fmt.Errorf("%q cannot be checked for backups: %w", newName, err)
	}
	places, err := s.backupPlaces(settings, settingsDomain, repos)
	if err != nil {
		return fmt.Errorf("%q cannot be checked for backups: %w", newName, err)
	}
	for _, p := range places {
		// The literal tag: an identity widened by aliases would pull in the
		// history of whatever row holds newName.
		snaps, err := s.snapshotsForTag(ctx, p.repo, p.mode, prefix+newName)
		if err != nil {
			return fmt.Errorf("%q cannot be checked for backups: %s could not be read: %w", newName, p.name, err)
		}
		for _, snap := range snaps {
			if namesAFormerName(snap, own) {
				continue
			}
			// Nothing on screen explains an orphaned backup, so the refusal says
			// whether an entry still owns the name.
			owner := "a different, unrelated entry"
			if _, err := s.entryIDByName(domain, newName); err == nil {
				owner = "its own entry"
			}
			return fmt.Errorf("%q already has backups in %s that belong to %s; delete them, then take over again", newName, p.name, owner)
		}
	}
	return nil
}

// moveDRDrillTargetTo repoints the DR drill from oldName to newName when it
// names oldName. The drill verifies by the literal container:<name> tag, so
// left alone it would keep checking an ever older snapshot under a name
// nothing answers to.
func (s *Service) moveDRDrillTargetTo(oldName, newName string) error {
	_, err := s.store.MutateSettings(func(cur *store.Settings) error {
		if cur.DRDrillTarget == oldName {
			cur.DRDrillTarget = newName
		}
		return nil
	})
	return err
}

// TakeOverContainer moves a not-installed entry onto the name its container was
// renamed to. Nothing in the repository is touched: the row keeps its id, so
// history and settings follow, and the old name becomes an alias the readers
// include. It is refused while the new name has backups that are not this
// entry's own, since adopting a stranger's history cannot be undone.
//
// Another entry's former name is refused on either side (takeoverAliasCheck).
// Onto the entry's own former name the takeover is a rename back: the store
// drops that alias and links the name the entry leaves, once the former name
// is shown to hold nothing from after its link (refuseTakeBackWhileNameReused).
//
// Both names are validated before anything is written, because the store does
// not validate them and the old name ends up in a formerly: tag on every later
// backup, which restic would split at a comma. The rewritten definition goes
// into the same transaction as the rename, so a corrupt stored definition
// fails the takeover before anything changes.
func (s *Service) TakeOverContainer(ctx context.Context, oldName, newName string) error {
	if oldName == newName {
		return errors.New("an entry cannot take over itself")
	}
	if !validResourceName(oldName) || !validResourceName(newName) {
		return errors.New("invalid container name")
	}
	// Locked before anything is checked: a backup or a restore finishing
	// between a lock-free check and the rename would slip past it.
	unlock, ok := s.tryLockDomainFor("containers", "takeover")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	live, err := s.installedContainers(ctx)
	if err != nil {
		return fmt.Errorf("list containers: %w", err)
	}
	if !live[newName] {
		return fmt.Errorf("container %q is not installed", newName)
	}
	if live[oldName] {
		return fmt.Errorf("container %q is installed again, so nothing was taken over", oldName)
	}
	// A restore from the entry would stop BombVault halfway.
	if self := s.selfContainerName(ctx); self != "" && newName == self {
		return fmt.Errorf("%q is BombVault's own container, so no entry can move onto it", newName)
	}
	oldTg, err := s.store.GetTargetByContainer(oldName)
	if err != nil {
		return fmt.Errorf("%q has no entry to take over", oldName)
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	back, err := s.takeoverAliasCheck("container", oldTg.ID, oldName, newName)
	if err != nil {
		return err
	}
	ownRepo, err := s.containerRepoPath(settings, oldTg)
	if err != nil {
		return fmt.Errorf("resolve the entry's repository: %w", err)
	}
	repos, err := s.reposToCheckForTakeover(settings, "container", newName, ownRepo)
	if err != nil {
		return fmt.Errorf("%q cannot be checked for backups: %w", newName, err)
	}
	if back != nil {
		err = s.refuseTakeBackWhileNameReused(ctx, settings, *back, repos)
	} else {
		err = s.refuseForeignBackups(ctx, settings, "container", oldTg.ID, oldName, newName, repos)
	}
	if err != nil {
		return err
	}
	newDefinition, err := rewriteDefinitionJSON(oldTg.Definition, newName)
	if err != nil {
		return fmt.Errorf("rewrite definition for %q: %w", newName, err)
	}
	if existing, err := s.store.GetTargetByContainer(newName); err == nil {
		// RenameTargetWithAlias refuses an occupied name, so an empty row on it
		// is cleared first; one with backups or settings is kept.
		empty, labels, err := s.targetOccupyingNewNameIsEmpty(ctx, existing, oldName)
		if err != nil {
			return fmt.Errorf("check the existing entry of %q: %w", newName, err)
		}
		if !empty {
			if onlyACopyRule(labels) {
				return fmt.Errorf("%q: %w", newName, store.ErrCopyRuleTaken)
			}
			return fmt.Errorf("%q already has its own configured entry (%s), refusing to delete it; unlink or remove it yourself first", newName, strings.Join(labels, ", "))
		}
		if err := s.store.DeleteTarget(newName, live); err != nil {
			return fmt.Errorf("remove the empty entry of %q: %w", newName, err)
		}
	}
	// The store refuses such a rule too, but only after the mirrors below
	// have dropped their link records.
	if err := s.store.CheckCopyRuleMove("containers", "container:"+oldName, "container:"+newName); err != nil {
		return err
	}
	// Discover rebuilds a link only from the definition mirrors, so the name the
	// entry leaves stops recording links before the rename and the new name
	// records them after it.
	if err := s.dropLinkRecords("container", settings, oldName, ownRepo, oldTg.Definition); err != nil {
		return err
	}
	if err := s.store.RenameTargetWithAlias(oldName, newName, newDefinition); err != nil {
		return err
	}
	s.recordLinks("container", settings, oldTg.ID, newName, ownRepo, newDefinition)
	// Best-effort from here on: the rename, alias and definition are committed,
	// and other rows' stop lists and the DR drill in Settings cannot share that
	// transaction. A stop list still naming the old name stops nothing, which
	// is harmless.
	if err := s.rewriteStopLists(oldName, newName); err != nil {
		log.Printf("api: takeover %q -> %q succeeded, but rewriting sibling stop lists failed; some entries may still try to stop %q by its old name until fixed: %v", oldName, newName, oldName, err) //nolint:gosec // G706: both %q-quoted
	}
	if err := s.moveDRDrillTargetTo(oldName, newName); err != nil {
		log.Printf("api: takeover %q -> %q succeeded, but moving the DR-drill target failed; a drill may keep verifying the stale name %q until fixed: %v", oldName, newName, oldName, err) //nolint:gosec // G706: both %q-quoted
	}
	return nil
}

// takeoverAliasCheck refuses moving entry targetID from oldName to newName in
// domain ("container" or "vm") while either name is another entry's former
// name, because a former name and its older backups belong to one entry. When
// newName is the entry's own former name the move is a rename back, and that
// alias is returned.
func (s *Service) takeoverAliasCheck(domain, targetID, oldName, newName string) (*store.Alias, error) {
	_, _, kind := aliasDomain(domain)
	a, err := s.store.AliasByOldName(domain, oldName)
	switch {
	case err == nil:
		return nil, fmt.Errorf("%q is also a former name of %s, so its entry cannot move to %q; back up %q as a new entry instead", oldName, s.formerNameOwner(domain, a), newName, newName)
	case !errors.Is(err, sql.ErrNoRows):
		return nil, fmt.Errorf("read the former names: %w", err)
	}
	a, err = s.store.AliasByOldName(domain, newName)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("read the former names: %w", err)
	case a.TargetID == targetID:
		return &a, nil
	}
	return nil, fmt.Errorf("%q is a former name of %s and still holds its older backups; give the %s a name of its own, then take over", newName, s.formerNameOwner(domain, a), kind)
}

// formerNameOwner is how a message names the entry that alias a belongs to.
func (s *Service) formerNameOwner(domain string, a store.Alias) string {
	if name, err := s.entryNameByID(domain, a.TargetID); err == nil {
		return fmt.Sprintf("%q", name)
	}
	return "another entry"
}

// refuseTakeBackWhileNameReused refuses to rename an entry back onto one of its
// former names, back.OldName, while that name holds a backup from the link on
// in repos or an off-site target. Dropping the alias lifts the name's time
// bound, so this is the check unlink makes.
func (s *Service) refuseTakeBackWhileNameReused(ctx context.Context, settings store.Settings, back store.Alias, repos []string) error {
	_, _, kind := aliasDomain(back.Domain)
	reused, err := s.oldNameReused(ctx, settings, back, repos)
	if err != nil {
		return fmt.Errorf("%q cannot be taken back until newer backups under it can be ruled out: %w", back.OldName, err)
	}
	if reused {
		return fmt.Errorf("%q cannot be taken back: another %s has been backed up under that name since this entry left it. Delete those backups, then take over again", back.OldName, kind)
	}
	return nil
}

// UnlinkContainerAlias reverses a takeover: the entry goes back to its old
// name, the alias is removed and the stored definition is rewritten back, all
// in one transaction as in TakeOverContainer. It is refused when oldName is no
// alias, so a stale or mistyped name changes nothing, while another container
// is installed under oldName next to the entry's own, since the entry would
// move onto it, and while oldName holds another machine's backups from after
// the link (refuseUnlinkWhileOldNameReused).
func (s *Service) UnlinkContainerAlias(ctx context.Context, oldName string) error {
	if !validResourceName(oldName) {
		return errors.New("invalid container name")
	}
	alias, err := s.store.AliasByOldName("container", oldName)
	if err != nil {
		return fmt.Errorf("%q is not a taken-over name", oldName)
	}
	tg, err := s.store.GetTargetByID(alias.TargetID)
	if err != nil {
		return fmt.Errorf("read the linked entry: %w", err)
	}
	currentName := tg.ContainerName
	newDefinition, err := rewriteDefinitionJSON(tg.Definition, oldName)
	if err != nil {
		return fmt.Errorf("rewrite definition for %q: %w", oldName, err)
	}
	unlock, ok := s.tryLockDomainFor("containers", "unlink")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	// Under the lock, like the takeover gate: a backup of a machine under
	// oldName landing between these checks and the unlink would slip past them.
	live, err := s.installedContainers(ctx)
	if err != nil {
		return fmt.Errorf("%q stays linked until the installed containers can be listed: %w", oldName, err)
	}
	if live[oldName] && live[currentName] {
		return fmt.Errorf("%q stays linked: another container is installed under that name; rename that container first", oldName)
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	ownRepo, err := s.containerRepoPath(settings, tg)
	if err != nil {
		return fmt.Errorf("resolve the linked entry's repository: %w", err)
	}
	if err := s.refuseUnlinkWhileOldNameReused(ctx, settings, alias, ownRepo); err != nil {
		return err
	}
	// The rule check and the definition mirrors follow as in TakeOverContainer.
	if err := s.store.CheckCopyRuleMove("containers", "container:"+currentName, "container:"+oldName); err != nil {
		return err
	}
	if err := s.dropLinkRecords("container", settings, currentName, ownRepo, tg.Definition); err != nil {
		return err
	}
	if err := s.store.UnlinkAlias(oldName, newDefinition, live); err != nil {
		return err
	}
	s.relistAfterUnlink("containers", "container:"+currentName)
	s.recordLinks("container", settings, tg.ID, oldName, ownRepo, newDefinition)
	// Best-effort, as in TakeOverContainer.
	if err := s.rewriteStopLists(currentName, oldName); err != nil {
		log.Printf("api: unlink %q succeeded, but rewriting sibling stop lists failed; some entries may still try to stop %q by its former name until fixed: %v", oldName, currentName, err) //nolint:gosec // G706: both %q-quoted
	}
	if err := s.moveDRDrillTargetTo(currentName, oldName); err != nil {
		log.Printf("api: unlink %q succeeded, but moving the DR-drill target back failed; a drill may keep verifying the stale name %q until fixed: %v", oldName, currentName, err) //nolint:gosec // G706: both %q-quoted
	}
	return nil
}

// relistAfterUnlink lists again, in the background, each target that counted
// copies under identity, the name an entry has just left. Some of them are the
// entry's again and some stay with the name, and only a listing tells which.
func (s *Service) relistAfterUnlink(domain, identity string) {
	held, err := s.store.ItemCopiesFor(domain, identity)
	if err != nil {
		log.Printf("api: unlink: the copies counted under %q stay there until the next replication: %v", identity, err) //nolint:gosec // G706: identity is %q-quoted
		return
	}
	for _, c := range held {
		s.listTargetInBackground(domain, c.TargetID)
	}
}

// refuseUnlinkWhileOldNameReused refuses to unlink a while its old name holds
// a snapshot a does not claim in ownRepo or any off-site target, or while one
// of them cannot be read. Unlinking lifts the name's time bound, so a
// later machine's backups under it would become the entry's own; the way out
// is renaming that machine or deleting its backups.
func (s *Service) refuseUnlinkWhileOldNameReused(ctx context.Context, settings store.Settings, a store.Alias, ownRepo string) error {
	_, _, kind := aliasDomain(a.Domain)
	reused, err := s.oldNameReused(ctx, settings, a, []string{ownRepo})
	if err != nil {
		return fmt.Errorf("%q stays linked until newer backups under that name can be ruled out: %w", a.OldName, err)
	}
	if reused {
		return fmt.Errorf("%q stays linked: another %s has been backed up under that name since it was linked here, and unlinking would make those backups this entry's. Rename that %s, or delete its backups, then unlink", a.OldName, kind, kind)
	}
	return nil
}

// aliasDomain maps an alias domain ("container" or "vm") to its settings
// domain, its identity tag prefix and the word a message uses for it.
func aliasDomain(domain string) (settingsDomain, prefix, kind string) {
	if domain == "vm" {
		return "vms", "vm:", "VM"
	}
	return "containers", "container:", "container"
}

// oldNameReused reports whether a's old name holds a snapshot a does not
// claim, one taken at or after the link or at a time that does not parse, in
// repos or in any off-site target of the alias's domain. The error names the
// place that could not be read.
func (s *Service) oldNameReused(ctx context.Context, settings store.Settings, a store.Alias, repos []string) (bool, error) {
	domain, prefix, _ := aliasDomain(a.Domain)
	places, err := s.backupPlaces(settings, domain, repos)
	if err != nil {
		return false, err
	}
	claim := newAliasClaim(prefix, a)
	for _, p := range places {
		snaps, err := s.snapshotsForTag(ctx, p.repo, p.mode, claim.tag)
		if err != nil {
			return false, fmt.Errorf("%s could not be read: %w", p.name, err)
		}
		if !claim.claimsEvery(snaps) {
			return true, nil
		}
	}
	return false, nil
}

// backupPlace is a repository an entry's snapshots may sit in, with the name
// a message gives it.
type backupPlace struct {
	name string
	repo string
	mode restic.Mode
}

// backupPlaces is repos plus every off-site target domain replicates to. An
// off-site target list that cannot be read is an error, because a check that
// skips a copy it cannot see passes on nothing.
func (s *Service) backupPlaces(settings store.Settings, domain string, repos []string) ([]backupPlace, error) {
	places := make([]backupPlace, 0, len(repos)+1)
	for _, repo := range repos {
		places = append(places, backupPlace{"the repository (" + shortRepoName(repo) + ")", repo, s.repoModeFor(settings, domain, "", repo)})
	}
	targets, err := s.enabledOffsiteTargets(domain)
	if err != nil {
		return nil, fmt.Errorf("the off-site target list could not be read: %w", err)
	}
	for _, t := range orSettingsOffsiteTarget(targets, domain, settings) {
		repo, err := s.resolveRepo(t.Repo)
		if err != nil {
			return nil, fmt.Errorf("off-site target %q could not be resolved: %w", t.Name, err)
		}
		places = append(places, backupPlace{fmt.Sprintf("off-site target %q", t.Name), repo, s.offsiteModeForTarget(settings, t)})
	}
	return places, nil
}
