package api

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// TakeOverVM moves the entry of a renamed VM onto its new libvirt name, keeping
// its id, history and settings, with the old name as an alias. The definition
// is rebuilt from the renamed VM because Unraid moves the disk folder with the
// name, and the one from before stays with the alias for an unlink or a rename
// back.
func (s *Service) TakeOverVM(ctx context.Context, oldName, newName string) error {
	if oldName == newName {
		return errors.New("an entry cannot take over itself")
	}
	if !validVMName(oldName) || !validVMName(newName) {
		return errors.New("invalid VM name")
	}
	// Past validVMName, a comma is all validFormerName refuses.
	if !validFormerName("vm", oldName) {
		return fmt.Errorf("%q cannot be taken over: backups cannot record a former name with a comma", oldName)
	}
	unlock, ok := s.tryLockDomainFor("vms", "takeover")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	infos, err := s.virsh.List(ctx)
	if err != nil {
		return fmt.Errorf("list vms: virsh: %w", err)
	}
	defined := make(map[string]bool, len(infos))
	for _, vm := range infos {
		defined[vm.Name] = true
	}
	if !defined[newName] {
		return fmt.Errorf("VM %q is not defined on the host", newName)
	}
	if defined[oldName] {
		return fmt.Errorf("VM %q is defined on the host again, so nothing was taken over", oldName)
	}
	oldTg, err := s.store.GetVMTargetByName(oldName)
	if err != nil {
		return fmt.Errorf("%q has no entry to take over", oldName)
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	back, err := s.takeoverAliasCheck("vm", oldTg.ID, oldName, newName)
	if err != nil {
		return err
	}
	ownRepo, err := s.vmRepoPath(settings, oldTg)
	if err != nil {
		return fmt.Errorf("resolve the entry's repository: %w", err)
	}
	repos, err := s.reposToCheckForTakeover(settings, "vm", newName, ownRepo)
	if err != nil {
		return fmt.Errorf("%q cannot be checked for backups: %w", newName, err)
	}
	if back != nil {
		err = s.refuseTakeBackWhileNameReused(ctx, settings, *back, repos)
	} else {
		err = s.refuseForeignBackups(ctx, settings, "vm", oldTg.ID, oldName, newName, repos)
	}
	if err != nil {
		return err
	}
	oldDef, err := vmDefinitionForTakeover(oldName, newName, oldTg.Definition)
	if err != nil {
		return err
	}
	var newDefinition string
	switch {
	case back == nil:
		// The firmware bytes were captured with the stored definition, so its
		// UUID decides; the column only stands in when the XML names none.
		uuid := cmp.Or(definitionUUID(oldTg.Definition), oldTg.UUID)
		if newDefinition, err = s.refreshedVMDefinition(ctx, oldDef, uuid, newName); err != nil {
			return err
		}
	case back.PrevDefinition == "":
		return fmt.Errorf("%q cannot be taken back: the definition the entry had under that name was not kept", newName)
	default:
		if err := s.refuseAnotherVMUnder(ctx, newName, back.PrevDefinition); err != nil {
			return fmt.Errorf("%q cannot be taken back: %w", newName, err)
		}
		newDefinition = back.PrevDefinition
	}
	if err := s.removeEmptyVMRow(ctx, newName); err != nil {
		return err
	}
	// The definition mirrors follow as in TakeOverContainer.
	if err := s.dropLinkRecords("vm", settings, oldName, ownRepo, oldTg.Definition); err != nil {
		return err
	}
	if err := s.store.RenameVMTargetWithAlias(oldName, newName, newDefinition, definitionUUID(newDefinition)); err != nil {
		return err
	}
	s.recordLinks("vm", settings, oldTg.ID, newName, ownRepo, newDefinition)
	if err := s.moveDRDrillTargetVMTo(oldName, newName); err != nil {
		log.Printf("api: takeover of VM %q -> %q succeeded, but moving the DR-drill pin failed; set it again in Settings: %v", oldName, newName, err) //nolint:gosec // G706: both %q-quoted
	}
	return nil
}

// UnlinkVMAlias reverses a VM takeover, moving the entry back onto oldName with
// the definition it had there. It refuses while that cannot be done safely:
// the alias kept no definition, oldName holds a backup from after the link, or
// another VM is defined under oldName, next to the entry's own or without the
// definition's UUID.
func (s *Service) UnlinkVMAlias(ctx context.Context, oldName string) error {
	if !validVMName(oldName) {
		return errors.New("invalid VM name")
	}
	unlock, ok := s.tryLockDomainFor("vms", "unlink")
	if !ok {
		return errDomainBusy
	}
	defer unlock()
	alias, err := s.store.AliasByOldName("vm", oldName)
	if err != nil {
		return fmt.Errorf("%q is not a taken-over name", oldName)
	}
	if alias.PrevDefinition == "" {
		return fmt.Errorf("%q cannot be unlinked: the definition the entry had under that name was not kept", oldName)
	}
	tg, err := s.store.GetVMTargetByID(alias.TargetID)
	if err != nil {
		return fmt.Errorf("read the linked entry: %w", err)
	}
	infos, err := s.virsh.List(ctx)
	if err != nil {
		return fmt.Errorf("%q stays linked until the VMs on the host can be listed: %w", oldName, err)
	}
	defined := make(map[string]bool, len(infos))
	for _, vm := range infos {
		defined[vm.Name] = true
	}
	if defined[oldName] {
		if defined[tg.Name] {
			return fmt.Errorf("%q stays linked: another VM is defined under that name; rename that VM first", oldName)
		}
		if err := s.refuseAnotherVMUnder(ctx, oldName, alias.PrevDefinition); err != nil {
			return fmt.Errorf("%q stays linked: %w", oldName, err)
		}
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	ownRepo, err := s.vmRepoPath(settings, tg)
	if err != nil {
		return fmt.Errorf("resolve the linked entry's repository: %w", err)
	}
	if err := s.refuseUnlinkWhileOldNameReused(ctx, settings, alias, ownRepo); err != nil {
		return err
	}
	if err := s.dropLinkRecords("vm", settings, tg.Name, ownRepo, tg.Definition); err != nil {
		return err
	}
	if err := s.store.UnlinkVMAlias(oldName, alias.PrevDefinition, definitionUUID(alias.PrevDefinition)); err != nil {
		return err
	}
	s.relistAfterUnlink("vms", "vm:"+tg.Name)
	s.recordLinks("vm", settings, tg.ID, oldName, ownRepo, alias.PrevDefinition)
	if err := s.moveDRDrillTargetVMTo(tg.Name, oldName); err != nil {
		log.Printf("api: unlink of VM %q succeeded, but moving the DR-drill pin back failed; set it again in Settings: %v", oldName, err) //nolint:gosec // G706: %q-quoted
	}
	return nil
}

// refuseAnotherVMUnder refuses moving an entry with definition onto name, a
// defined VM, unless that VM has the libvirt UUID the definition names: the
// definition's disks and firmware state are that VM's, not another's.
func (s *Service) refuseAnotherVMUnder(ctx context.Context, name, definition string) error {
	xmlStr, err := s.virsh.DumpXMLInactive(ctx, name)
	if err != nil {
		return fmt.Errorf("the VM defined under that name could not be read: %w", err)
	}
	domain, err := virshcli.ParseDomain(xmlStr)
	if err != nil {
		return fmt.Errorf("the VM defined under that name could not be read: %w", err)
	}
	if domain.UUID == "" || domain.UUID != definitionUUID(definition) {
		return errors.New("the VM defined under that name is not the one this entry backed up there; rename that VM first")
	}
	return nil
}

// vmDefinitionForTakeover reads the stored definition of the entry on oldName
// and refuses a disk layout a takeover cannot carry. Block-device (zvol) disks
// back up under a per-disk tag that no alias follows, and a definition that
// does not parse has an unknown layout.
func vmDefinitionForTakeover(oldName, newName, stored string) (vmDefinition, error) {
	var def vmDefinition
	if err := json.Unmarshal([]byte(stored), &def); err != nil {
		return vmDefinition{}, fmt.Errorf("%q cannot be taken over: its stored definition does not parse, so its disk layout is unknown", oldName)
	}
	info, err := virshcli.ParseDomain(def.DomainXML)
	if err != nil {
		return vmDefinition{}, fmt.Errorf("%q cannot be taken over: its stored domain XML does not parse, so its disk layout is unknown", oldName)
	}
	if len(info.BlockDisks) > 0 {
		return vmDefinition{}, fmt.Errorf("%q cannot be taken over: the backups of its block-device (zvol) disks do not follow a rename. Back up %q as a new entry instead", oldName, newName)
	}
	return def, nil
}

// refreshedVMDefinition is def with the domain XML, disk paths and NVRAM path
// of the VM called name, derived the way BackupVM derives them. The
// captured NVRAM and TPM state stay only when the VM has the entry's libvirt
// UUID, since another VM's firmware state would be restored into it.
func (s *Service) refreshedVMDefinition(ctx context.Context, def vmDefinition, uuid, name string) (string, error) {
	xmlStr, err := s.virsh.DumpXMLInactive(ctx, name)
	if err != nil {
		return "", fmt.Errorf("the definition of VM %q could not be read: %w", name, err)
	}
	domain, err := virshcli.ParseDomain(xmlStr)
	if err != nil {
		return "", fmt.Errorf("the definition of VM %q does not parse: %w", name, err)
	}
	if len(leftoverOverlayDevices(domain)) > 0 {
		return "", fmt.Errorf("VM %q is still on a snapshot overlay from an interrupted live backup. Start it and run one backup to merge the overlay, then delete that backup with \"Delete all backups\" on %q and link it again", name, name)
	}
	diskPaths, err := s.vmDiskContainerPaths(name, domain)
	if err != nil {
		return "", err
	}
	def.DomainXML, def.DiskPaths, def.NVRAMHostPath = xmlStr, diskPaths, domain.NVRAMPath
	if domain.UUID == "" || domain.UUID != uuid {
		def.NVRAMBytes, def.TPMBytes = nil, nil
	}
	b, _ := json.Marshal(def)
	return string(b), nil
}

// removeEmptyVMRow deletes the row on name when it has no backups, nothing
// configured and no copy rule, so the entry can move there, and refuses any
// other row.
func (s *Service) removeEmptyVMRow(ctx context.Context, name string) error {
	tg, err := s.store.GetVMTargetByName(name)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil
	case err != nil:
		return fmt.Errorf("read the entry of %q: %w", name, err)
	}
	labels, err := s.withCopyRule(vmConfiguredStateLabels(tg), "vms", "vm:"+name)
	if err != nil {
		return err
	}
	if len(labels) > 0 {
		return fmt.Errorf("%q already has its own configured entry (%s); remove it yourself first", name, strings.Join(labels, ", "))
	}
	has, err := s.vmHasBackups(ctx, name)
	if err != nil {
		return fmt.Errorf("check the existing entry of %q: %w", name, err)
	}
	if has {
		return fmt.Errorf("%q already has its own entry with backups", name)
	}
	if err := s.store.DeleteVMTarget(name); err != nil {
		return fmt.Errorf("remove the empty entry of %q: %w", name, err)
	}
	return nil
}

// vmConfiguredStateLabels lists the settings an operator made on a VM row,
// which a takeover must not drop just because the row has no backups yet.
func vmConfiguredStateLabels(t store.VMTarget) []string {
	var labels []string
	if t.IncludeInSchedule {
		labels = append(labels, "scheduled")
	}
	if t.Method != "graceful" {
		labels = append(labels, "backup method")
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
	return labels
}

// moveDRDrillTargetVMTo is moveDRDrillTargetTo for the VM drill pin.
func (s *Service) moveDRDrillTargetVMTo(oldName, newName string) error {
	_, err := s.store.MutateSettings(func(cur *store.Settings) error {
		if cur.DRDrillTargetVM == oldName {
			cur.DRDrillTargetVM = newName
		}
		return nil
	})
	return err
}
