package store

import (
	"fmt"
	"sort"
	"time"
)

// VMTarget represents a KVM/libvirt VM that BombVault can back up.
type VMTarget struct {
	ID                string
	Name              string
	Method            string // "graceful" (default) or "live"
	IncludeInSchedule bool
	// Definition is an opaque JSON blob persisted at backup time containing
	// the domain XML, disk paths, NVRAM path, and method so restore works even
	// after the VM has been deleted or BombVault's /config is lost (full DR).
	Definition string
	CreatedAt  int64
	// ScheduleCadence is this VM's OPTIONAL per-item schedule override (#121, same
	// cadence grammar as the domain schedules). Empty (the default) means "use the
	// VMs domain schedule exactly as today"; only consulted when the per-item-
	// schedules feature toggle is on. Owned by SetVMScheduleCadence (never reset by
	// UpsertVMTarget).
	ScheduleCadence string
	// BackupOrder is the explicit position this VM takes in a scheduled VM run
	// (#119, mirrors targets.backup_order): VMs with an explicit order (>0) run
	// first in ascending order; 0 (the default) is unordered and keeps the
	// name-order tiebreak. Owned by SetVMBackupOrders (never reset by UpsertVMTarget).
	BackupOrder int
}

// UpsertVMTarget inserts or updates a VM target by name.
// On conflict, method and definition are refreshed; id, created_at, and
// include_in_schedule are preserved (include_in_schedule is owned by SetVMInclude).
// Returns the authoritative VMTarget (original ID when a conflict fires).
func (r *Repo) UpsertVMTarget(t VMTarget) (VMTarget, error) {
	if t.ID == "" {
		t.ID = newID()
	}
	if t.CreatedAt == 0 {
		t.CreatedAt = time.Now().Unix()
	}
	if t.Method == "" {
		t.Method = "graceful"
	}

	_, err := r.db.Exec(`
		INSERT INTO vms (id, name, method, include_in_schedule, definition, created_at, schedule_cadence, backup_order)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
		  method     = excluded.method,
		  definition = excluded.definition`,
		t.ID, t.Name, t.Method, boolInt(t.IncludeInSchedule), t.Definition, t.CreatedAt, t.ScheduleCadence, t.BackupOrder,
	)
	if err != nil {
		return VMTarget{}, fmt.Errorf("UpsertVMTarget: %w", err)
	}
	return r.GetVMTargetByName(t.Name)
}

// GetVMTargetByName returns the VM target for the named domain.
func (r *Repo) GetVMTargetByName(name string) (VMTarget, error) {
	row := r.db.QueryRow(`
		SELECT id, name, method, include_in_schedule, definition, created_at, schedule_cadence, backup_order
		FROM vms WHERE name = ?`, name)
	return scanVMTarget(row)
}

// ListVMTargets returns all known VM targets ordered by name.
func (r *Repo) ListVMTargets() ([]VMTarget, error) {
	rows, err := r.db.Query(`
		SELECT id, name, method, include_in_schedule, definition, created_at, schedule_cadence, backup_order
		FROM vms ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("ListVMTargets: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []VMTarget
	for rows.Next() {
		t, err := scanVMTarget(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// SetVMMethod updates the backup method for the named VM.
func (r *Repo) SetVMMethod(name, method string) error {
	res, err := r.db.Exec(`UPDATE vms SET method = ? WHERE name = ?`, method, name)
	if err != nil {
		return fmt.Errorf("SetVMMethod: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("SetVMMethod: vm %q not found", name)
	}
	return nil
}

// SetVMScheduleCadence sets this VM's per-item schedule override (#121). An empty
// string clears the override so the VM falls back to the VMs domain schedule. Plain
// UPDATE (no create-on-miss): a VM row is created by Discover/Upsert before it can
// be scheduled. Owned by this setter; never reset by UpsertVMTarget.
func (r *Repo) SetVMScheduleCadence(name, cadence string) error {
	res, err := r.db.Exec(`UPDATE vms SET schedule_cadence = ? WHERE name = ?`, cadence, name)
	if err != nil {
		return fmt.Errorf("SetVMScheduleCadence: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("SetVMScheduleCadence: vm %q not found", name)
	}
	return nil
}

// SetVMInclude updates the include_in_schedule flag for the named VM.
func (r *Repo) SetVMInclude(name string, include bool) error {
	res, err := r.db.Exec(`UPDATE vms SET include_in_schedule = ? WHERE name = ?`,
		boolInt(include), name)
	if err != nil {
		return fmt.Errorf("SetVMInclude: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("SetVMInclude: vm %q not found", name)
	}
	return nil
}

// DeleteVMTarget removes a VM target and ALL its run history by name, in a
// single transaction. It is a no-op (no error) if the target does not exist.
func (r *Repo) DeleteVMTarget(name string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("DeleteVMTarget begin: %w", err)
	}
	if _, err := tx.Exec(
		`DELETE FROM runs WHERE target_id IN (SELECT id FROM vms WHERE name = ?)`, name,
	); err != nil {
		tx.Rollback() //nolint:errcheck,gosec // best-effort rollback; original error takes priority
		return fmt.Errorf("DeleteVMTarget runs: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM vms WHERE name = ?`, name); err != nil {
		tx.Rollback() //nolint:errcheck,gosec // best-effort rollback; original error takes priority
		return fmt.Errorf("DeleteVMTarget: %w", err)
	}
	return tx.Commit()
}

func scanVMTarget(s scanner) (VMTarget, error) {
	var t VMTarget
	var include int
	err := s.Scan(&t.ID, &t.Name, &t.Method, &include, &t.Definition, &t.CreatedAt, &t.ScheduleCadence, &t.BackupOrder)
	if err != nil {
		return VMTarget{}, fmt.Errorf("scanVMTarget: %w", err)
	}
	t.IncludeInSchedule = include != 0
	return t, nil
}

// VMOrder pairs a VM name with its explicit backup order (#119, VMs). Mirrors
// ContainerOrder; used by the get/set VM backup-order API to move a whole
// ordering in one call.
type VMOrder struct {
	VM    string `json:"vm"`
	Order int    `json:"order"`
}

// SortVMTargetsForRun stably orders VM targets for a scheduled VM run (#119, VMs),
// mirroring SortTargetsForRun: VMs with an explicit BackupOrder (>0) run first in
// ascending order; the rest (0 = unordered) keep their incoming order (which the
// caller passes name-sorted). Mutates vms in place.
func SortVMTargetsForRun(vms []VMTarget) {
	sort.SliceStable(vms, func(i, j int) bool {
		oi, oj := vms[i].BackupOrder, vms[j].BackupOrder
		iExplicit, jExplicit := oi > 0, oj > 0
		if iExplicit != jExplicit {
			return iExplicit // an explicit order (>0) always runs before an unordered one
		}
		if !iExplicit {
			return false // both unordered: preserve the incoming (name-sorted) order
		}
		return oi < oj // both explicit: the lower order runs earlier
	})
}

// VMBackupOrders returns the current explicit VM backup ordering (#119, VMs): the
// VMs with a positive order, sorted by order ascending.
func (r *Repo) VMBackupOrders() ([]VMOrder, error) {
	rows, err := r.db.Query(`
		SELECT name, backup_order
		FROM vms
		WHERE backup_order > 0
		ORDER BY backup_order ASC, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("VMBackupOrders: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	out := []VMOrder{}
	for rows.Next() {
		var o VMOrder
		if err := rows.Scan(&o.VM, &o.Order); err != nil {
			return nil, fmt.Errorf("VMBackupOrders scan: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// SetVMBackupOrders authoritatively replaces the whole VM backup ordering (#119,
// VMs) in one transaction, mirroring SetBackupOrders: every VM is first reset to
// unordered (0), then each listed VM is stamped with its order. A VM omitted from
// orders is returned to the name-order tiebreak. Negative orders are clamped to 0.
// A listed name with no VM row (a VM must be discovered before it can be ordered)
// simply updates nothing. Owned by this setter; never reset by UpsertVMTarget.
func (r *Repo) SetVMBackupOrders(orders []VMOrder) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("SetVMBackupOrders begin: %w", err)
	}
	if _, err := tx.Exec(`UPDATE vms SET backup_order = 0`); err != nil {
		tx.Rollback() //nolint:errcheck,gosec // best-effort rollback; original error takes priority
		return fmt.Errorf("SetVMBackupOrders reset: %w", err)
	}
	for _, o := range orders {
		order := o.Order
		if order < 0 {
			order = 0
		}
		if _, err := tx.Exec(
			`UPDATE vms SET backup_order = ? WHERE name = ?`, order, o.VM,
		); err != nil {
			tx.Rollback() //nolint:errcheck,gosec // best-effort rollback; original error takes priority
			return fmt.Errorf("SetVMBackupOrders update %q: %w", o.VM, err)
		}
	}
	return tx.Commit()
}
