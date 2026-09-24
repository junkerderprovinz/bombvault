package api

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// DeleteBackups removes every backup of a container. From the local source it
// also forgets the container's entry; from an off-site source it deletes at that
// target only and the entry stays.
//
// Forgetting the entry takes its aliases with it, so the pre-link part of an
// old name falls to whichever entry uses that name next. The user asked for
// these backups to go, so that is the answer they want.
func (s *Service) DeleteBackups(ctx context.Context, name, source string) error {
	if isOffsiteSource(source) {
		if err := refuseDeleteWithPartialIdentity(name, s.containerIdentity(name)); err != nil {
			return err
		}
		_, err := s.forgetAtTarget(ctx, "containers", "container:"+name, source, taggedForItem, nil)
		return err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.containerRepoForName(settings, name, "local")
	if err != nil {
		return err
	}
	mode := s.repoModeFor(settings, "containers", "local", repo)

	// Serialize against a live backup on this repo: a container Backup holds
	// the domain lock for its whole run (potentially hours), so without this
	// an unlocked bulk delete could race a concurrent `restic forget --prune`
	// against the same repo files. DeleteBackupsVM and DeleteBackupsFileSet
	// use the same guard. (No requireExistingRepo here, unlike those two: a
	// never-backed-up container's target row must still be cleaned up below.)
	//
	// The append-only question is asked before the domain lock and before
	// anything is written. It cannot refuse outright, because a row with no
	// backups must stay clearable, so the refusal itself sits after the
	// listing; but nothing on the way there may touch a repository the
	// interface promises nothing on this box may delete from: not the domain
	// lock (a scheduled containers backup would get errDomainBusy from a call
	// that can only be refused), and not a stale unlock, which removes lock
	// files and on a local append-only folder succeeds.
	protection := s.primaryAppendOnly("containers", repo)
	if protection != appendOnlyNone {
		// A read, and it answers the only question left: is there anything in here
		// for the flag to protect?
		snaps, sErr := s.Snapshots(ctx, name, "")
		if sErr != nil {
			return sErr
		}
		if len(snaps) > 0 {
			return appendOnlyRefusal(protection)
		}
	}

	unlock, ok := s.tryLockDomainFor("containers", "delete")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	id := s.containerIdentity(name)
	if err := refuseDeleteWithPartialIdentity(name, id); err != nil {
		return err
	}
	// The entry's former names keep its copy rule unless a container installed
	// under one follows its own.
	held, err := s.heldContainerNames(ctx)
	if err != nil {
		return fmt.Errorf("nothing was deleted: %w", err)
	}
	if protection == appendOnlyNone {
		s.unlockStale(ctx, repo, mode)
	}

	snaps, err := s.containerSnapshotsOf(ctx, name, "", id)
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(snaps))
	for _, snap := range snaps {
		ids = append(ids, snap.ID)
	}
	if len(ids) > 0 {
		// The append-only refusal is asked here rather than at the top because the
		// flag protects snapshots and this function does two things: it forgets
		// them, and it removes the target row. With nothing to forget there is
		// nothing to protect, and refusing anyway would leave a container with no
		// backups stuck in the "not installed (backups only)" list, with no way
		// out but switching the whole repository's protection off, which drops it
		// for every other item sharing that repository.
		//
		// While an entry has backups, this is the only removal the not-installed
		// card offers (#232 added a row-only route, handleForgetContainer, for an
		// entry without any), which is what makes this the difference between an
		// inconvenience and a dead end.
		//
		// It is asked again rather than replayed. The check above runs outside the
		// domain lock and answers "is there anything to protect"; this one runs
		// inside it and answers "may this delete happen", and between the two an
		// operator can have turned the protection on, which is exactly when they
		// most mean it. A flag honoured only if it was already set when the button
		// was pressed is not a protection. Turning it off mid-flight costs one
		// refused delete and a second press.
		if f := s.primaryAppendOnly("containers", repo); f != appendOnlyNone {
			return appendOnlyRefusal(f)
		}
		if err := s.engine.Forget(ctx, repo, ids, true, mode); err != nil {
			return fmt.Errorf("forget snapshots: %w", err)
		}
	}

	// Remove the target row + its run history so the container disappears from
	// the "not installed" list once its backups are gone.
	if err := s.store.DeleteTarget(name, held); err != nil {
		return fmt.Errorf("delete target: %w", err)
	}
	return nil
}

// DeleteBackupsVM removes every backup of a VM. From the local source it also
// forgets the VM's entry; from an off-site source it deletes the snapshots the
// VM's backup list shows at that target and the entry stays. Its disk images
// live under their own tags and go through the window that shows them first.
// It is serialised against VM backups by the domain lock and clears stale locks
// first, so a leftover lock cannot fail it.
func (s *Service) DeleteBackupsVM(ctx context.Context, name, source string) error {
	if isOffsiteSource(source) {
		if err := refuseDeleteWithPartialIdentity(name, s.vmIdentity(name)); err != nil {
			return err
		}
		_, err := s.forgetAtTarget(ctx, "vms", "vm:"+name, source, taggedForItem, nil)
		return err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	// The VM's own repository (#204), not the domain's: deleting a VM's
	// backups has to reach the repository they were written to, or the call
	// succeeds against the domain repo and leaves the real snapshots behind.
	repo, err := s.vmRepoForName(settings, name, source)
	if err != nil {
		return err
	}
	// The same refusal applies when the local source is a remote primary flagged
	// append-only in its safety settings (#152). There is no separate off-site
	// copy in that shape, so this is the only thing between an on-box credential
	// and a Forget with prune against the sole backup.
	if f := s.primaryAppendOnly("vms", repo); f != appendOnlyNone {
		return appendOnlyRefusal(f)
	}
	if err := s.requireExistingRepo(repo, "no backups to delete yet"); err != nil {
		// A repository that was never created holds no backups, and nothing is
		// left to delete but the entry (#232); refusing here would leave a
		// not-installed card whose one removal button can never succeed.
		// snapshotsForTag tells that case apart from an established repository
		// whose share is not mounted (#55), where every snapshot still exists and
		// the entry is the only thing pointing at them: that one is refused, as
		// DeleteBackups does for a container. As this only removes the entry, it
		// keeps the entry of a VM still defined, like ForgetVMTarget.
		//
		// snapshotsForTag reads "could not tell whether it was established" as
		// "never", which is right for a list and wrong for a removal: the entry
		// could be all that points at a repository on an unmounted share. So that
		// answer is asked first and refused, the way reposThatExist words it.
		if s.repoEstablishmentOf(repo) == repoEstablishmentUnknown {
			return errors.New("this VM's repository is not reachable now, and whether it ever held backups could not be read, so its entry stays")
		}
		if _, sErr := s.snapshotsForTag(ctx, repo, s.repoModeFor(settings, "vms", source, repo), "vm:"+name); sErr != nil {
			return sErr
		}
		installed, lErr := s.installedVMs(ctx, settings)
		if lErr != nil {
			return fmt.Errorf("%q keeps its entry until the VMs on the host can be listed: %w", name, lErr)
		}
		if dErr := refuseDefinedVM(installed, name); dErr != nil {
			return dErr
		}
		unlock, ok := s.tryLockDomainFor("vms", "delete")
		if !ok {
			return errDomainBusy
		}
		defer unlock()
		if err := refuseDeleteWithPartialIdentity(name, s.vmIdentity(name)); err != nil {
			return err
		}
		held, err := s.heldVMNames(ctx)
		if err != nil {
			return fmt.Errorf("%q keeps its entry: %w", name, err)
		}
		if err := s.store.DeleteVMTarget(name, held); err != nil {
			return fmt.Errorf("delete vm target: %w", err)
		}
		return nil
	}
	unlock, ok := s.tryLockDomainFor("vms", "delete")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	mode := s.repoModeFor(settings, "vms", source, repo)
	s.unlockStale(ctx, repo, mode)

	id := s.vmIdentity(name)
	if err := refuseDeleteWithPartialIdentity(name, id); err != nil {
		return err
	}
	// The entry's former names keep its copy rule unless a VM defined under
	// one follows its own.
	held, err := s.heldVMNames(ctx)
	if err != nil {
		return fmt.Errorf("nothing was deleted: %w", err)
	}
	snaps, err := s.vmSnapshotsOf(ctx, name, source, id)
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(snaps))
	for _, snap := range snaps {
		ids = append(ids, snap.ID)
	}
	if len(ids) > 0 {
		if err := s.engine.Forget(ctx, repo, ids, true, mode); err != nil {
			return fmt.Errorf("forget snapshots: %w", err)
		}
	}

	if err := s.store.DeleteVMTarget(name, held); err != nil {
		return fmt.Errorf("delete vm target: %w", err)
	}
	return nil
}

// ForgetVMTarget removes a VM's target row and run history without
// touching any repo, to clear a stale "Not installed" entry that has no
// backups (which also stops the scheduler from retrying a deleted VM).
// Deleting actual backups is DeleteBackupsVM; this is only the bookkeeping.
//
// Refused for a VM that is defined on the host, asked the way ListVMs asks
// (libvirt only while VMs are enabled), so a card left open while the VM came
// back cannot wipe a live VM's settings and history. Serialised against VM
// backups and restores like DeleteBackupsVM: a restore of this entry writes
// the run row this would delete. Refused too while the entry owns a backup
// anywhere it replicates to (refuseRowRemovalWithBackups).
func (s *Service) ForgetVMTarget(ctx context.Context, name string) error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	installed, err := s.installedVMs(ctx, settings)
	if err != nil {
		return fmt.Errorf("%q keeps its entry until the VMs on the host can be listed: %w", name, err)
	}
	if err := refuseDefinedVM(installed, name); err != nil {
		return err
	}
	unlock, ok := s.tryLockDomainFor("vms", "delete")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	repo, err := s.vmRepoForName(settings, name, "")
	if err != nil {
		return fmt.Errorf("%q keeps its entry: its repository could not be resolved to check for backups: %w", name, err)
	}
	if err := s.refuseRowRemovalWithBackups(ctx, settings, "vms", name, repo, s.vmIdentity(name)); err != nil {
		return err
	}
	held, err := s.heldVMNames(ctx)
	if err != nil {
		return fmt.Errorf("%q keeps its entry: %w", name, err)
	}
	if err := s.store.DeleteVMTarget(name, held); err != nil {
		return fmt.Errorf("forget vm target: %w", err)
	}
	return nil
}

// refuseRowRemovalWithBackups refuses to remove the row of name while its
// identity id owns a snapshot in repo or any off-site target of domain, or
// while one of them cannot be read. The row carries the aliases that make its
// older backups its own; without them those backups would fall to whichever
// entry takes the name next.
func (s *Service) refuseRowRemovalWithBackups(ctx context.Context, settings store.Settings, domain, name, repo string, id entryIdentity) error {
	if id.readErr != nil {
		return fmt.Errorf("%q keeps its entry: its backups could not be checked: %w", name, id.readErr)
	}
	places, err := s.backupPlaces(settings, domain, []string{repo})
	if err != nil {
		return fmt.Errorf("%q keeps its entry until its backups can be ruled out: %w", name, err)
	}
	for _, p := range places {
		owned, err := s.snapshotsOwnedBy(ctx, p.repo, p.mode, id)
		if err != nil {
			return fmt.Errorf("%q keeps its entry until its backups can be ruled out: %s could not be read: %w", name, p.name, err)
		}
		if len(owned) > 0 {
			return fmt.Errorf("%q still has backups in %s; delete its backups instead", name, p.name)
		}
	}
	return nil
}

// refuseDeleteWithPartialIdentity refuses delete-all while id is partial,
// because it would forget only part of the entry's backups and then drop the
// aliases that make the rest its own.
func refuseDeleteWithPartialIdentity(name string, id entryIdentity) error {
	if id.readErr == nil {
		return nil
	}
	return fmt.Errorf("nothing was deleted: the backups of %q could not be checked: %w", name, id.readErr)
}

// refuseDefinedVM answers an error when name is among installed, for the routes
// that remove only a VM's entry, so none of them can drop the settings and
// history of a live VM.
func refuseDefinedVM(installed map[string]bool, name string) error {
	if installed[name] {
		return fmt.Errorf("VM %q is defined on the host, so its entry stays", name)
	}
	return nil
}

// installedContainers is the set of containers Docker lists, the ones the
// container list shows as installed.
func (s *Service) installedContainers(ctx context.Context) (map[string]bool, error) {
	infos, err := s.docker.List(ctx)
	if err != nil {
		return nil, err
	}
	installed := make(map[string]bool, len(infos))
	for _, c := range infos {
		installed[c.Name] = true
	}
	return installed, nil
}

// heldContainerNames is what the store is told Docker lists when it decides on
// the copy rule of a former container name. Docker is asked only while some
// container has a former name, so nothing else waits for it.
func (s *Service) heldContainerNames(ctx context.Context) (map[string]bool, error) {
	aliases, err := s.store.ListAliases("container")
	if err != nil {
		return nil, err
	}
	if len(aliases) == 0 {
		return nil, nil
	}
	installed, err := s.installedContainers(ctx)
	if err != nil {
		return nil, fmt.Errorf("the installed containers could not be listed: %w", err)
	}
	return installed, nil
}

// installedVMs is the set of VMs the VM list shows as installed: the ones
// libvirt defines while VMs are enabled, and none while they are off, when
// every entry is listed as not installed.
func (s *Service) installedVMs(ctx context.Context, settings store.Settings) (map[string]bool, error) {
	if !settings.VMsEnabled {
		return map[string]bool{}, nil
	}
	return s.definedVMs(ctx)
}

// definedVMs is the set of VMs libvirt defines, whatever the VMs setting says.
func (s *Service) definedVMs(ctx context.Context) (map[string]bool, error) {
	infos, err := s.virsh.List(ctx)
	if err != nil {
		return nil, err
	}
	defined := make(map[string]bool, len(infos))
	for _, vm := range infos {
		defined[vm.Name] = true
	}
	return defined, nil
}

// heldVMNames is what the store is told libvirt defines when it decides on the
// copy rule of a former VM name. libvirt is asked whatever the VMs setting says,
// since a rule written while VMs are off applies once they are on again, and
// when it cannot answer every former name counts as held.
func (s *Service) heldVMNames(ctx context.Context) (map[string]bool, error) {
	aliases, err := s.store.ListAliases("vm")
	if err != nil {
		return nil, err
	}
	if len(aliases) == 0 {
		return nil, nil
	}
	defined, err := s.definedVMs(ctx)
	if err == nil {
		return defined, nil
	}
	log.Printf("api: the VMs on the host could not be listed, so every former VM name keeps the copy rule it has: %v", err)
	held := make(map[string]bool, len(aliases))
	for _, a := range aliases {
		held[a.OldName] = true
	}
	return held, nil
}

// ForgetTarget removes a container's target row and run history without
// touching any repo, the container twin of ForgetVMTarget (#232). It
// clears a "Not installed" entry that has no backups, which also takes it
// off the schedule. Deleting actual backups is DeleteBackups; this is only
// the bookkeeping. Refused for an installed container and serialised like
// ForgetVMTarget, for the same reasons. Refused too while the entry owns a
// backup anywhere it replicates to (refuseRowRemovalWithBackups).
func (s *Service) ForgetTarget(ctx context.Context, name string) error {
	installed, err := s.installedContainers(ctx)
	if err != nil {
		return fmt.Errorf("list containers: %w", err)
	}
	if installed[name] {
		return fmt.Errorf("container %q is installed, so its entry stays", name)
	}
	unlock, ok := s.tryLockDomainFor("containers", "delete")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.containerRepoForName(settings, name, "")
	if err != nil {
		return fmt.Errorf("%q keeps its entry: its repository could not be resolved to check for backups: %w", name, err)
	}
	if err := s.refuseRowRemovalWithBackups(ctx, settings, "containers", name, repo, s.containerIdentity(name)); err != nil {
		return err
	}
	if err := s.store.DeleteTarget(name, installed); err != nil {
		return fmt.Errorf("forget target: %w", err)
	}
	return nil
}
