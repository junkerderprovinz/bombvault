package api

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// SetInclude sets the include_in_schedule flag for a container, creating the
// target row first if it does not exist yet (the first backup has not run).
// It inspects the container to resolve appdata paths exactly like Backup does,
// so the target is fully populated from the start. If docker inspect fails the
// operation is still completed: a placeholder target is upserted with a
// conventional appdata path so the toggle is never silently lost.
func (s *Service) SetInclude(ctx context.Context, name string, include bool) error {
	if _, err := s.store.GetTargetByContainer(name); err != nil {
		// Target does not exist yet: find or create it before calling SetInclude.
		var appdata []string
		if in, inspErr := s.docker.Inspect(ctx, name); inspErr == nil {
			appdata = s.resolveAppdataPaths(name, in)
		} else {
			log.Printf("api: SetInclude: inspect %q failed (checking fallback path): %v", name, inspErr) //nolint:gosec // G706: name is %q-quoted; no raw user bytes reach the log formatter
			// Fall back to the conventional appdata dir, but only if it exists on
			// disk (the same os.Stat guard as resolveAppdataPaths). A phantom
			// placeholder would show as a selected folder that backs up nothing
			// (#115); leave AppdataPaths empty (definition-only) until the user points
			// it at a real folder.
			cand := path.Join(s.cfg.HostMountRoot, "appdata", name)
			if _, statErr := os.Stat(cand); statErr == nil { //nolint:gosec // G703: cand is HostMountRoot + "appdata" + a validated container name, not raw user input
				appdata = []string{cand}
			}
		}
		if _, upsertErr := s.store.UpsertTarget(store.Target{
			ContainerName: name,
			AppdataPaths:  appdata,
		}); upsertErr != nil {
			return fmt.Errorf("ensure target: %w", upsertErr)
		}
	}
	return s.store.SetInclude(name, include)
}

// SetScheduleCadence sets a container's per-item schedule override (#121).
// It finds or creates the target row (like SetInclude) so an override can
// be set before the first backup. The cadence is validated with the
// grammar the domain schedules use; an empty string clears the override
// (back to the domain default). everyN is rejected because a per-item entry
// has no per-item last-run gate to enforce the interval, the same
// restriction the off-site and drill schedules carry.
func (s *Service) SetScheduleCadence(ctx context.Context, name, cadence string) error {
	cadence = strings.TrimSpace(cadence)
	if cadence != "" {
		cad, err := schedule.ParseCadence(cadence)
		if err != nil {
			return fmt.Errorf("invalid schedule: %w", err)
		}
		if cad.IntervalDays > 0 {
			return fmt.Errorf("per-item schedules do not support 'everyN': use 'off', 'daily HH:MM', 'weekly DOW HH:MM', or a cron expression")
		}
	}
	if _, err := s.store.GetTargetByContainer(name); err != nil {
		// Target does not exist yet: find or create it (same path as SetInclude).
		var appdata []string
		if in, inspErr := s.docker.Inspect(ctx, name); inspErr == nil {
			appdata = s.resolveAppdataPaths(name, in)
		}
		if _, upsertErr := s.store.UpsertTarget(store.Target{
			ContainerName: name,
			AppdataPaths:  appdata,
		}); upsertErr != nil {
			return fmt.Errorf("ensure target: %w", upsertErr)
		}
	}
	return s.store.SetScheduleCadence(name, cadence)
}

// SetIncludeAll sets the include_in_schedule flag for every installed
// container in one call, the one-click "include all in schedule" and
// "exclude all" action. It iterates the installed-container source the
// containers list uses (docker.List) and ensures a target row exists for
// each (find or create, as SetInclude does), so the flag is never silently
// lost on a container that has not been backed up yet. BombVault's own
// container is skipped: it can never be backed up (ErrSelfBackup), so
// scheduling it would only add a failing job. A single container's
// inspect or upsert failure aborts the batch with that error rather than
// leaving a partial, ambiguous result.
//
// Excluding also reaches every target whose container is gone from the
// host (#232). Such a row stays scheduled until somebody switches it off,
// and every run records a skip for it, so "Exclude all" has to end that
// too. Including never reaches them: it would put every removed container
// straight back into the skip loop.
func (s *Service) SetIncludeAll(ctx context.Context, include bool) error {
	infos, err := s.docker.List(ctx)
	if err != nil {
		return fmt.Errorf("list containers: %w", err)
	}
	self := s.selfContainerName(ctx)
	live := make(map[string]bool, len(infos))
	for _, c := range infos {
		live[c.Name] = true
		if self != "" && c.Name == self {
			continue // never schedule BombVault's own container
		}
		if err := s.SetInclude(ctx, c.Name, include); err != nil {
			return err
		}
	}
	if include {
		return nil
	}
	targets, err := s.store.ListTargets()
	if err != nil {
		return fmt.Errorf("list targets: %w", err)
	}
	for _, t := range targets {
		if live[t.ContainerName] || !t.IncludeInSchedule {
			continue
		}
		if err := s.store.SetInclude(t.ContainerName, false); err != nil {
			return err
		}
	}
	return nil
}

// SetVMMethod updates the backup method for a VM, creating the target if absent.
func (s *Service) SetVMMethod(_ context.Context, name, method string) error {
	if _, err := s.store.GetVMTargetByName(name); err != nil {
		if _, uErr := s.store.UpsertVMTarget(store.VMTarget{Name: name, Method: method}); uErr != nil {
			return fmt.Errorf("ensure vm target: %w", uErr)
		}
		return nil
	}
	return s.store.SetVMMethod(name, method)
}

// SetVMInclude updates the include_in_schedule flag for a VM, creating the
// target if absent.
func (s *Service) SetVMInclude(_ context.Context, name string, include bool) error {
	if _, err := s.store.GetVMTargetByName(name); err != nil {
		if _, uErr := s.store.UpsertVMTarget(store.VMTarget{Name: name, Method: "graceful"}); uErr != nil {
			return fmt.Errorf("ensure vm target: %w", uErr)
		}
	}
	return s.store.SetVMInclude(name, include)
}

// SetFileSetScheduleCadence writes a folder set's per-item schedule
// override (#199), validating it the same way the container and VM
// setters do.
//
// "everyN" is refused for the same reason it is refused there:
// classifyItemOverride maps an interval cadence back to the domain default
// rather than giving the item its own entry, so accepting one here would
// store a value that silently does nothing. Better to say so at the point
// of entry than to have somebody discover it from a backup that never ran.
func (s *Service) SetFileSetScheduleCadence(_ context.Context, id, cadence string) error {
	cadence = strings.TrimSpace(cadence)
	if cadence != "" {
		cad, err := schedule.ParseCadence(cadence)
		if err != nil {
			return fmt.Errorf("invalid schedule: %w", err)
		}
		if cad.IntervalDays > 0 {
			return fmt.Errorf("per-item schedules do not support 'everyN': use 'off', 'daily HH:MM', 'weekly DOW HH:MM', or a cron expression")
		}
	}
	return s.store.SetFileSetScheduleCadence(id, cadence)
}

// SetVMScheduleCadence sets a VM's per-item schedule override (#121),
// creating the target if absent. The cadence is validated with the
// domain-schedule grammar; an empty string clears the override. everyN is
// rejected (no per-item last-run gate), like SetScheduleCadence.
func (s *Service) SetVMScheduleCadence(_ context.Context, name, cadence string) error {
	cadence = strings.TrimSpace(cadence)
	if cadence != "" {
		cad, err := schedule.ParseCadence(cadence)
		if err != nil {
			return fmt.Errorf("invalid schedule: %w", err)
		}
		if cad.IntervalDays > 0 {
			return fmt.Errorf("per-item schedules do not support 'everyN': use 'off', 'daily HH:MM', 'weekly DOW HH:MM', or a cron expression")
		}
	}
	if _, err := s.store.GetVMTargetByName(name); err != nil {
		if _, uErr := s.store.UpsertVMTarget(store.VMTarget{Name: name, Method: "graceful"}); uErr != nil {
			return fmt.Errorf("ensure vm target: %w", uErr)
		}
	}
	return s.store.SetVMScheduleCadence(name, cadence)
}

// SetVMIncludeAll sets the include_in_schedule flag for every VM on the
// host in one call, the VM counterpart to SetIncludeAll. It iterates the
// live VMs reported by virsh and ensures a target row exists for each
// (find or create, as SetVMInclude does). Excluding then applies to every
// already-known VM target too, so an orphan VM that still has backups
// comes off the schedule. De-duplicated so a VM that is both live and a
// known target is only set once.
//
// Including stops at the live VMs (#232): putting every deleted VM back on
// the schedule would make each run try it again and log a skip. Same rule
// as SetIncludeAll on containers.
func (s *Service) SetVMIncludeAll(ctx context.Context, include bool) error {
	infos, err := s.virsh.List(ctx)
	if err != nil {
		return fmt.Errorf("list vms: %w", err)
	}
	live := make(map[string]bool, len(infos))
	for _, vm := range infos {
		live[vm.Name] = true
		if err := s.SetVMInclude(ctx, vm.Name, include); err != nil {
			return err
		}
	}
	if include {
		return nil
	}
	// Known targets whose VM is no longer defined on the host (orphans with
	// backups). The find-or-create in SetVMInclude already handles existing
	// rows, so a plain store update is enough here.
	targets, err := s.store.ListVMTargets()
	if err != nil {
		return fmt.Errorf("list vm targets: %w", err)
	}
	for _, t := range targets {
		if live[t.Name] || !t.IncludeInSchedule {
			continue
		}
		if err := s.store.SetVMInclude(t.Name, false); err != nil {
			return err
		}
	}
	return nil
}

// SetContainerHooks stores the pre/post-backup hook commands for a container.
func (s *Service) SetContainerHooks(_ context.Context, name, preHook, postHook string) error {
	return s.store.SetHooks(name, preHook, postHook)
}

// SetUpdateAfterBackup toggles the post-backup image update for a container (#52).
func (s *Service) SetUpdateAfterBackup(_ context.Context, name string, updateAfterBackup bool) error {
	return s.store.SetUpdateAfterBackup(name, updateAfterBackup)
}

// SetStopContainers stores the other container names to stop during this
// container's backup. Names are trimmed + de-duplicated; blanks are dropped.
func (s *Service) SetStopContainers(_ context.Context, name string, stop []string) error {
	var clean []string
	seen := map[string]bool{}
	for _, c := range stop {
		c = strings.TrimSpace(c)
		if c == "" || c == name || seen[c] {
			continue // skip blanks, self, and duplicates
		}
		seen[c] = true
		clean = append(clean, c)
	}
	return s.store.SetStopContainers(name, clean)
}

// BackupOrders returns the current explicit manual backup ordering (#119): the
// containers with a positive order, sorted by order ascending.
func (s *Service) BackupOrders(_ context.Context) ([]store.ContainerOrder, error) {
	return s.store.BackupOrders()
}

// SetBackupOrders authoritatively replaces the manual backup ordering (#119) from
// an ordered list of container names: the first name gets order 1, the next 2, and
// so on, and every container not in the list is returned to unordered. Blanks and
// duplicates (first occurrence wins) are dropped so the positions stay dense.
func (s *Service) SetBackupOrders(_ context.Context, names []string) error {
	orders := make([]store.ContainerOrder, 0, len(names))
	seen := map[string]bool{}
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" || seen[n] {
			continue // skip blanks and duplicates
		}
		seen[n] = true
		orders = append(orders, store.ContainerOrder{Container: n, Order: len(orders) + 1})
	}
	return s.store.SetBackupOrders(orders)
}

// VMBackupOrders returns the current explicit VM backup ordering (#119, VMs): the
// VMs with a positive order, sorted by order ascending.
func (s *Service) VMBackupOrders(_ context.Context) ([]store.VMOrder, error) {
	return s.store.VMBackupOrders()
}

// SetVMBackupOrders authoritatively replaces the VM backup ordering (#119, VMs)
// from an ordered list of VM names: the first name gets order 1, the next 2, and so
// on, and every VM not in the list is returned to unordered. Blanks and duplicates
// (first occurrence wins) are dropped so the positions stay dense.
func (s *Service) SetVMBackupOrders(_ context.Context, names []string) error {
	orders := make([]store.VMOrder, 0, len(names))
	seen := map[string]bool{}
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" || seen[n] {
			continue // skip blanks and duplicates
		}
		seen[n] = true
		orders = append(orders, store.VMOrder{VM: n, Order: len(orders) + 1})
	}
	return s.store.SetVMBackupOrders(orders)
}
