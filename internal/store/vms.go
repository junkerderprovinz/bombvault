package store

import (
	"fmt"
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
		INSERT INTO vms (id, name, method, include_in_schedule, definition, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
		  method     = excluded.method,
		  definition = excluded.definition`,
		t.ID, t.Name, t.Method, boolInt(t.IncludeInSchedule), t.Definition, t.CreatedAt,
	)
	if err != nil {
		return VMTarget{}, fmt.Errorf("UpsertVMTarget: %w", err)
	}
	return r.GetVMTargetByName(t.Name)
}

// GetVMTargetByName returns the VM target for the named domain.
func (r *Repo) GetVMTargetByName(name string) (VMTarget, error) {
	row := r.db.QueryRow(`
		SELECT id, name, method, include_in_schedule, definition, created_at
		FROM vms WHERE name = ?`, name)
	return scanVMTarget(row)
}

// ListVMTargets returns all known VM targets ordered by name.
func (r *Repo) ListVMTargets() ([]VMTarget, error) {
	rows, err := r.db.Query(`
		SELECT id, name, method, include_in_schedule, definition, created_at
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
	err := s.Scan(&t.ID, &t.Name, &t.Method, &include, &t.Definition, &t.CreatedAt)
	if err != nil {
		return VMTarget{}, fmt.Errorf("scanVMTarget: %w", err)
	}
	t.IncludeInSchedule = include != 0
	return t, nil
}
